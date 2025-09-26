package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tradecaptain/api-gateway/internal/config"
	"tradecaptain/api-gateway/internal/handlers"
	"tradecaptain/api-gateway/internal/middleware"
	"tradecaptain/api-gateway/internal/services"
	"tradecaptain/api-gateway/internal/storage"
	"tradecaptain/api-gateway/internal/websocket"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title TradeCaptain API
// @version 1.0
// @description API for TradeCaptain - Modern Financial Data Terminal
// @author Tarun Thakur
// @contact.name Tarun Thakur
// @contact.email thakur[dot]cs[dot]tarun[at]gmail[dot]com

// @license.name Apache 2.0

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	// Initialize configuration
	cfg := config.Load()

	// Initialize storage
	db, err := storage.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	cache, err := storage.NewRedisCache(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer cache.Close()

	// Initialize Kafka consumer
	consumer, err := storage.NewKafkaConsumer(cfg.KafkaBootstrapServers, "api-gateway-group")
	if err != nil {
		log.Fatalf("Failed to initialize Kafka consumer: %v", err)
	}
	defer consumer.Close()

	// Initialize services
	marketDataService := services.NewMarketDataService(db, cache)
	portfolioService := services.NewPortfolioService(db)
	userService := services.NewUserService(db)
	newsService := services.NewNewsService(db, cache)

	// Initialize WebSocket hub
	wsHub := websocket.NewHub()
	go wsHub.Run()

	// Start Kafka consumer in background
	go func() {
		if err := consumer.StartConsuming(wsHub); err != nil {
			log.Printf("Kafka consumer error: %v", err)
		}
	}()

	// Initialize Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.CORS())
	router.Use(middleware.RateLimit(cfg.RateLimitPerSecond))

	// Initialize handlers
	marketHandler := handlers.NewMarketDataHandler(marketDataService)
	portfolioHandler := handlers.NewPortfolioHandler(portfolioService)
	userHandler := handlers.NewUserHandler(userService, cfg.JWTSecret)
	newsHandler := handlers.NewNewsHandler(newsService)
	wsHandler := handlers.NewWebSocketHandler(wsHub)

	// API routes
	v1 := router.Group("/api/v1")
	{
		// Public routes
		auth := v1.Group("/auth")
		{
			auth.POST("/register", userHandler.Register)
			auth.POST("/login", userHandler.Login)
			auth.POST("/refresh", userHandler.RefreshToken)
		}

		// Market data routes
		market := v1.Group("/market")
		{
			market.GET("/quote/:symbol", marketHandler.GetQuote)
			market.GET("/quotes", marketHandler.GetMultipleQuotes)
			market.GET("/historical/:symbol", marketHandler.GetHistoricalData)
			market.GET("/search", marketHandler.SearchSymbols)
		}

		// News routes
		news := v1.Group("/news")
		{
			news.GET("/", newsHandler.GetNews)
			news.GET("/search", newsHandler.SearchNews)
		}

		// Protected routes
		protected := v1.Group("/")
		protected.Use(middleware.AuthMiddleware(cfg.JWTSecret))
		{
			// Portfolio routes
			portfolio := protected.Group("/portfolio")
			{
				portfolio.GET("/", portfolioHandler.GetPortfolios)
				portfolio.POST("/", portfolioHandler.CreatePortfolio)
				portfolio.GET("/:id", portfolioHandler.GetPortfolio)
				portfolio.PUT("/:id", portfolioHandler.UpdatePortfolio)
				portfolio.DELETE("/:id", portfolioHandler.DeletePortfolio)
				portfolio.POST("/:id/positions", portfolioHandler.AddPosition)
				portfolio.PUT("/:id/positions/:positionId", portfolioHandler.UpdatePosition)
				portfolio.DELETE("/:id/positions/:positionId", portfolioHandler.DeletePosition)
			}

			// User profile routes
			user := protected.Group("/user")
			{
				user.GET("/profile", userHandler.GetProfile)
				user.PUT("/profile", userHandler.UpdateProfile)
				user.GET("/watchlist", userHandler.GetWatchlist)
				user.POST("/watchlist", userHandler.AddToWatchlist)
				user.DELETE("/watchlist/:symbol", userHandler.RemoveFromWatchlist)
			}
		}

		// WebSocket endpoint
		v1.GET("/ws", wsHandler.HandleWebSocket)
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Start server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	log.Printf("API Gateway started on port %s", cfg.Port)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}
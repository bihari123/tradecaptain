package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"tradecaptain/data-collector/internal/collector"
	"tradecaptain/data-collector/internal/config"
	"tradecaptain/data-collector/internal/storage"

	"github.com/joho/godotenv"
)

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

	// Initialize Kafka producer
	producer, err := storage.NewKafkaProducer(cfg.KafkaBootstrapServers)
	if err != nil {
		log.Fatalf("Failed to initialize Kafka producer: %v", err)
	}
	defer producer.Close()

	// Initialize data collector
	dataCollector := collector.New(db, cache, producer, cfg)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// Start data collection services
	wg.Add(1)
	go func() {
		defer wg.Done()
		dataCollector.StartMarketDataCollection(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		dataCollector.StartNewsCollection(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		dataCollector.StartEconomicDataCollection(ctx)
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Data collector service started...")
	<-sigChan

	log.Println("Shutting down data collector service...")
	cancel()

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Data collector service stopped gracefully")
	case <-time.After(30 * time.Second):
		log.Println("Force shutdown after timeout")
	}
}
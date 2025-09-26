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
	"tradecaptain/data-collector/internal/cache"

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

	// Initialize L1 embedded cache (BigCache - 25x faster than Redis for local data)
	l1Cache, err := cache.NewL1Cache()
	if err != nil {
		log.Fatalf("Failed to initialize L1 cache: %v", err)
	}
	defer l1Cache.Close()

	// Initialize L2 distributed cache (Dragonfly - Redis compatible)
	redisCache, err := storage.NewRedisCache(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Dragonfly: %v", err)
	}
	defer redisCache.Close()

	// Initialize BadgerDB WAL for ultra-fast local persistence
	walPath := os.Getenv("WAL_PATH")
	if walPath == "" {
		walPath = "./data/wal"
	}
	wal, err := storage.NewBadgerWAL(walPath, cfg.KafkaBootstrapServers)
	if err != nil {
		log.Fatalf("Failed to initialize WAL: %v", err)
	}
	defer wal.Close()

	// Initialize Kafka producer
	producer, err := storage.NewKafkaProducer(cfg.KafkaBootstrapServers)
	if err != nil {
		log.Fatalf("Failed to initialize Kafka producer: %v", err)
	}
	defer producer.Close()

	// Initialize data collector with optimized storage layers
	dataCollector := collector.NewWithOptimizations(db, l1Cache, redisCache, wal, producer, cfg)

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
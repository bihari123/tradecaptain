package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// Kafka
	KafkaBootstrapServers string

	// API Keys
	AlphaVantageAPIKey string
	IEXCloudAPIKey     string
	NewsAPIKey         string
	FREDAPIKey         string

	// Collection intervals
	MarketDataInterval    time.Duration
	NewsInterval          time.Duration
	EconomicDataInterval  time.Duration

	// Symbols to track
	StockSymbols  []string
	CryptoSymbols []string

	// Rate limiting
	MaxRequestsPerSecond int
}

func Load() *Config {
	return &Config{
		DatabaseURL:           getEnv("DATABASE_URL", "postgres://user:password@localhost/bloomberg_terminal?sslmode=disable"),
		RedisURL:              getEnv("REDIS_URL", "redis://localhost:6379"),
		KafkaBootstrapServers: getEnv("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092"),

		AlphaVantageAPIKey: getEnv("ALPHA_VANTAGE_API_KEY", ""),
		IEXCloudAPIKey:     getEnv("IEX_CLOUD_API_KEY", ""),
		NewsAPIKey:         getEnv("NEWS_API_KEY", ""),
		FREDAPIKey:         getEnv("FRED_API_KEY", ""),

		MarketDataInterval:   getDuration("MARKET_DATA_INTERVAL", 30*time.Second),
		NewsInterval:         getDuration("NEWS_INTERVAL", 5*time.Minute),
		EconomicDataInterval: getDuration("ECONOMIC_DATA_INTERVAL", 1*time.Hour),

		StockSymbols:  getStringSlice("STOCK_SYMBOLS", []string{"AAPL", "GOOGL", "MSFT", "TSLA", "AMZN"}),
		CryptoSymbols: getStringSlice("CRYPTO_SYMBOLS", []string{"BTC", "ETH", "ADA", "DOT"}),

		MaxRequestsPerSecond: getInt("MAX_REQUESTS_PER_SECOND", 10),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
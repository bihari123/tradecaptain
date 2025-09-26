package storage

import (
	"context"
	"testing"
	"time"

	"tradecaptain/data-collector/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresDB_SaveMarketData(t *testing.T) {
	// This would normally use testcontainers for a real PostgreSQL instance
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	marketData := &models.MarketData{
		Symbol:        "AAPL",
		Price:         150.25,
		Volume:        1000000,
		High:          151.0,
		Low:           149.5,
		Open:          150.0,
		Close:         150.25,
		Change:        0.25,
		ChangePercent: 0.17,
		MarketCap:     2500000000000,
		Timestamp:     time.Now().UTC(),
		Source:        "test",
	}

	err := db.SaveMarketData(ctx, marketData)
	require.NoError(t, err)

	// Verify data was saved
	saved, err := db.GetLatestMarketData(ctx, []string{"AAPL"})
	require.NoError(t, err)
	require.Len(t, saved, 1)

	assert.Equal(t, marketData.Symbol, saved[0].Symbol)
	assert.Equal(t, marketData.Price, saved[0].Price)
	assert.Equal(t, marketData.Volume, saved[0].Volume)
}

func TestPostgresDB_GetMarketData_TimeRange(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	symbol := "GOOGL"

	// Insert test data spanning multiple days
	baseTime := time.Now().UTC().Truncate(24 * time.Hour)
	for i := 0; i < 5; i++ {
		marketData := &models.MarketData{
			Symbol:    symbol,
			Price:     float64(100 + i),
			Volume:    1000000,
			High:      float64(101 + i),
			Low:       float64(99 + i),
			Open:      float64(100 + i),
			Close:     float64(100 + i),
			Timestamp: baseTime.Add(time.Duration(i) * 24 * time.Hour),
			Source:    "test",
		}
		require.NoError(t, db.SaveMarketData(ctx, marketData))
	}

	// Query for specific time range
	from := baseTime.Add(24 * time.Hour)
	to := baseTime.Add(72 * time.Hour)

	results, err := db.GetMarketData(ctx, symbol, from, to)
	require.NoError(t, err)
	assert.Len(t, results, 2) // Should get days 1 and 2 (inclusive)

	// Verify results are in chronological order
	for i := 1; i < len(results); i++ {
		assert.True(t, results[i-1].Timestamp.Before(results[i].Timestamp))
	}
}

func TestPostgresDB_BatchUpdate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Create batch data
	symbols := []string{"AAPL", "GOOGL", "MSFT", "TSLA", "AMZN"}
	var batchData []*models.MarketData

	for _, symbol := range symbols {
		marketData := &models.MarketData{
			Symbol:    symbol,
			Price:     150.0,
			Volume:    1000000,
			High:      151.0,
			Low:       149.0,
			Open:      150.0,
			Close:     150.0,
			Timestamp: time.Now().UTC(),
			Source:    "test",
		}
		batchData = append(batchData, marketData)
	}

	// Test batch insert
	err := db.UpdateMarketDataBatch(ctx, batchData)
	require.NoError(t, err)

	// Verify all records were inserted
	for _, symbol := range symbols {
		saved, err := db.GetLatestMarketData(ctx, []string{symbol})
		require.NoError(t, err)
		require.Len(t, saved, 1)
		assert.Equal(t, symbol, saved[0].Symbol)
	}
}

func TestPostgresDB_Upsert_Behavior(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Insert initial data
	marketData := &models.MarketData{
		Symbol:    "TEST",
		Price:     100.0,
		Volume:    1000,
		High:      101.0,
		Low:       99.0,
		Open:      100.0,
		Close:     100.0,
		Timestamp: time.Now().UTC().Truncate(time.Minute), // Truncate for consistent comparison
		Source:    "test",
	}

	err := db.SaveMarketData(ctx, marketData)
	require.NoError(t, err)

	// Update same record with different price
	marketData.Price = 105.0
	marketData.Volume = 2000

	err = db.SaveMarketData(ctx, marketData)
	require.NoError(t, err)

	// Verify update worked (should have same ID but updated values)
	saved, err := db.GetLatestMarketData(ctx, []string{"TEST"})
	require.NoError(t, err)
	require.Len(t, saved, 1)

	assert.Equal(t, 105.0, saved[0].Price)
	assert.Equal(t, int64(2000), saved[0].Volume)
}

func setupTestDB(t *testing.T) *PostgresDB {
	// In a real test, you would use testcontainers or a test database
	// For now, this is a placeholder that would connect to a test database

	// Example using testcontainers:
	// ctx := context.Background()
	// req := testcontainers.ContainerRequest{
	//     Image:        "postgres:15",
	//     ExposedPorts: []string{"5432/tcp"},
	//     Env: map[string]string{
	//         "POSTGRES_DB":       "testdb",
	//         "POSTGRES_USER":     "testuser",
	//         "POSTGRES_PASSWORD": "testpass",
	//     },
	//     WaitingFor: wait.ForListeningPort("5432/tcp"),
	// }

	// postgres, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
	//     ContainerRequest: req,
	//     Started:          true,
	// })
	// require.NoError(t, err)

	// host, err := postgres.Host(ctx)
	// require.NoError(t, err)

	// port, err := postgres.MappedPort(ctx, "5432")
	// require.NoError(t, err)

	// connStr := fmt.Sprintf("postgres://testuser:testpass@%s:%s/testdb?sslmode=disable", host, port.Port())
	// db, err := NewPostgresDB(connStr)
	// require.NoError(t, err)

	// // Run migrations
	// require.NoError(t, db.CreateTables(ctx))

	// t.Cleanup(func() {
	//     postgres.Terminate(ctx)
	// })

	// return db

	// For now, return a mock or skip test if no test DB available
	t.Skip("Test requires real PostgreSQL instance")
	return nil
}

func BenchmarkPostgresDB_BatchInsert(b *testing.B) {
	db := setupBenchmarkDB(b)
	defer db.Close()

	ctx := context.Background()

	// Prepare batch data
	batchSize := 1000
	var batchData []*models.MarketData

	for i := 0; i < batchSize; i++ {
		marketData := &models.MarketData{
			Symbol:    fmt.Sprintf("STOCK%d", i),
			Price:     float64(100 + i),
			Volume:    int64(1000 + i),
			High:      float64(101 + i),
			Low:       float64(99 + i),
			Open:      float64(100 + i),
			Close:     float64(100 + i),
			Timestamp: time.Now().UTC(),
			Source:    "benchmark",
		}
		batchData = append(batchData, marketData)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := db.UpdateMarketDataBatch(ctx, batchData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func setupBenchmarkDB(b *testing.B) *PostgresDB {
	// Similar setup as test but optimized for benchmarking
	b.Skip("Benchmark requires PostgreSQL instance")
	return nil
}
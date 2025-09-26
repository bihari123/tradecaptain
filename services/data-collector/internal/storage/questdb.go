package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	"tradecaptain/data-collector/internal/models"
)

// QuestDBClient provides ultra-fast time-series data ingestion
type QuestDBClient struct {
	db *sql.DB
}

// NewQuestDBClient creates a new QuestDB client using PostgreSQL wire protocol
func NewQuestDBClient(connectionString string) (*QuestDBClient, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to QuestDB: %w", err)
	}

	// Optimize connection for high-frequency writes
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping QuestDB: %w", err)
	}

	return &QuestDBClient{db: db}, nil
}

// InsertMarketData inserts market data using optimized batch operations
func (q *QuestDBClient) InsertMarketData(data *models.MarketData) error {
	query := `
		INSERT INTO market_data_realtime (
			symbol, price, volume, bid, ask, high, low, open, close,
			volatility_pct, risk_level, market_session, exchange, timestamp
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err := q.db.Exec(
		query,
		data.Symbol,
		data.Price,
		data.Volume,
		data.Bid,
		data.Ask,
		data.High,
		data.Low,
		data.Open,
		data.Close,
		calculateVolatility(data),
		classifyRisk(data),
		determineMarketSession(data.Timestamp),
		data.Exchange,
		data.Timestamp,
	)

	return err
}

// BatchInsertMarketData performs bulk inserts for maximum throughput
func (q *QuestDBClient) BatchInsertMarketData(dataSlice []*models.MarketData) error {
	if len(dataSlice) == 0 {
		return nil
	}

	// Use PostgreSQL COPY for maximum performance
	txn, err := q.db.Begin()
	if err != nil {
		return err
	}
	defer txn.Rollback()

	stmt, err := txn.Prepare(pq.CopyIn(
		"market_data_realtime",
		"symbol", "price", "volume", "bid", "ask", "high", "low", "open", "close",
		"volatility_pct", "risk_level", "market_session", "exchange", "timestamp",
	))
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, data := range dataSlice {
		_, err = stmt.Exec(
			data.Symbol,
			data.Price,
			data.Volume,
			data.Bid,
			data.Ask,
			data.High,
			data.Low,
			data.Open,
			data.Close,
			calculateVolatility(data),
			classifyRisk(data),
			determineMarketSession(data.Timestamp),
			data.Exchange,
			data.Timestamp,
		)
		if err != nil {
			return err
		}
	}

	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	return txn.Commit()
}

// GetLatestPrices retrieves the most recent price for each symbol
func (q *QuestDBClient) GetLatestPrices(symbols []string) (map[string]*models.MarketData, error) {
	if len(symbols) == 0 {
		return make(map[string]*models.MarketData), nil
	}

	// Use QuestDB's LATEST ON optimization for time-series queries
	query := `
		SELECT symbol, price, volume, bid, ask, high, low, open, close, timestamp
		FROM market_data_realtime
		WHERE symbol IN (%s)
		LATEST ON timestamp PARTITION BY symbol
	`

	// Build placeholders for symbols
	placeholders := make([]string, len(symbols))
	args := make([]interface{}, len(symbols))
	for i, symbol := range symbols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = symbol
	}

	formattedQuery := fmt.Sprintf(query, fmt.Sprintf("'%s'", symbols[0]))
	if len(symbols) > 1 {
		formattedQuery = fmt.Sprintf(query, "'"+fmt.Sprintf("%s'", symbols[0]))
		for i := 1; i < len(symbols); i++ {
			formattedQuery += fmt.Sprintf(", '%s'", symbols[i])
		}
	}

	rows, err := q.db.Query(formattedQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]*models.MarketData)
	for rows.Next() {
		var data models.MarketData
		err := rows.Scan(
			&data.Symbol,
			&data.Price,
			&data.Volume,
			&data.Bid,
			&data.Ask,
			&data.High,
			&data.Low,
			&data.Open,
			&data.Close,
			&data.Timestamp,
		)
		if err != nil {
			return nil, err
		}
		result[data.Symbol] = &data
	}

	return result, rows.Err()
}

// GetPriceHistory retrieves historical price data for backtesting
func (q *QuestDBClient) GetPriceHistory(symbol string, start, end time.Time, interval string) ([]*models.MarketData, error) {
	var query string

	switch interval {
	case "1m":
		query = `
			SELECT symbol, first(price) as open, max(price) as high, min(price) as low,
				   last(price) as close, sum(volume) as volume, timestamp
			FROM market_data_realtime
			WHERE symbol = $1 AND timestamp BETWEEN $2 AND $3
			SAMPLE BY 1m FILL(PREV)
			ORDER BY timestamp
		`
	case "5m":
		query = `
			SELECT symbol, first(price) as open, max(price) as high, min(price) as low,
				   last(price) as close, sum(volume) as volume, timestamp
			FROM market_data_realtime
			WHERE symbol = $1 AND timestamp BETWEEN $2 AND $3
			SAMPLE BY 5m FILL(PREV)
			ORDER BY timestamp
		`
	case "1h":
		query = `
			SELECT symbol, first(price) as open, max(price) as high, min(price) as low,
				   last(price) as close, sum(volume) as volume, timestamp
			FROM market_data_realtime
			WHERE symbol = $1 AND timestamp BETWEEN $2 AND $3
			SAMPLE BY 1h FILL(PREV)
			ORDER BY timestamp
		`
	default:
		return nil, fmt.Errorf("unsupported interval: %s", interval)
	}

	rows, err := q.db.Query(query, symbol, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*models.MarketData
	for rows.Next() {
		var data models.MarketData
		err := rows.Scan(
			&data.Symbol,
			&data.Open,
			&data.High,
			&data.Low,
			&data.Close,
			&data.Volume,
			&data.Timestamp,
		)
		if err != nil {
			return nil, err
		}
		data.Price = data.Close // Use close as current price
		result = append(result, &data)
	}

	return result, rows.Err()
}

// GetPerformanceStats returns database performance statistics
func (q *QuestDBClient) GetPerformanceStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get table sizes
	row := q.db.QueryRow("SELECT count(*) FROM market_data_realtime")
	var recordCount int64
	if err := row.Scan(&recordCount); err == nil {
		stats["total_records"] = recordCount
	}

	// Get ingestion rate (records per second in last minute)
	row = q.db.QueryRow(`
		SELECT count(*) / 60 as records_per_second
		FROM market_data_realtime
		WHERE timestamp > dateadd('m', -1, now())
	`)
	var recordsPerSecond float64
	if err := row.Scan(&recordsPerSecond); err == nil {
		stats["records_per_second"] = recordsPerSecond
	}

	return stats, nil
}

// Close closes the database connection
func (q *QuestDBClient) Close() error {
	return q.db.Close()
}

// Helper functions
func calculateVolatility(data *models.MarketData) float64 {
	if data.Price == 0 {
		return 0
	}
	return ((data.High - data.Low) / data.Price) * 100
}

func classifyRisk(data *models.MarketData) string {
	volatility := calculateVolatility(data)
	if volatility > 5 {
		return "high"
	} else if volatility > 2 {
		return "medium"
	}
	return "low"
}

func determineMarketSession(timestamp time.Time) string {
	hour := timestamp.Hour()
	if hour >= 9 && hour < 16 {
		return "market_hours"
	}
	return "after_hours"
}
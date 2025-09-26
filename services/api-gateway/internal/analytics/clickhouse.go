package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// ClickHouseClient provides ultra-fast analytical queries for financial data
type ClickHouseClient struct {
	conn driver.Conn
}

// NewClickHouseClient creates a new ClickHouse client
func NewClickHouseClient(host string, database string, username string, password string) (*ClickHouseClient, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{host},
		Auth: clickhouse.Auth{
			Database: database,
			Username: username,
			Password: password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
			"max_memory_usage":   "4000000000", // 4GB
		},
		DialTimeout:     time.Second * 30,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	// Test connection
	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	return &ClickHouseClient{conn: conn}, nil
}

// MarketAnalytics represents aggregated market data for analytics
type MarketAnalytics struct {
	Symbol          string    `ch:"symbol"`
	Date            time.Time `ch:"date"`
	Timestamp       time.Time `ch:"timestamp"`
	Open            float64   `ch:"open"`
	High            float64   `ch:"high"`
	Low             float64   `ch:"low"`
	Close           float64   `ch:"close"`
	Volume          uint64    `ch:"volume"`
	PriceChange     float64   `ch:"price_change"`
	PriceChangePct  float64   `ch:"price_change_pct"`
	Volatility      float64   `ch:"volatility"`
	VolatilityPct   float64   `ch:"volatility_pct"`
	MarketSession   string    `ch:"market_session"`
	Exchange        string    `ch:"exchange"`
	Sector          string    `ch:"sector"`
}

// PortfolioAnalytics represents portfolio performance metrics
type PortfolioAnalytics struct {
	PortfolioID       string    `ch:"portfolio_id"`
	Date              time.Time `ch:"date"`
	Timestamp         time.Time `ch:"timestamp"`
	TotalValue        float64   `ch:"total_value"`
	Cash              float64   `ch:"cash"`
	InvestedValue     float64   `ch:"invested_value"`
	DailyReturn       float64   `ch:"daily_return"`
	CumulativeReturn  float64   `ch:"cumulative_return"`
	Volatility        float64   `ch:"volatility"`
	SharpeRatio       *float64  `ch:"sharpe_ratio"`
	MaxDrawdown       float64   `ch:"max_drawdown"`
	VaR95             float64   `ch:"var_95"`
	Beta              float64   `ch:"beta"`
	PositionCount     uint32    `ch:"position_count"`
	ConcentrationTop5 float64   `ch:"concentration_top_5"`
}

// BatchInsertMarketAnalytics inserts market analytics data in batches
func (c *ClickHouseClient) BatchInsertMarketAnalytics(data []MarketAnalytics) error {
	ctx := context.Background()

	batch, err := c.conn.PrepareBatch(ctx, `
		INSERT INTO market_analytics (
			symbol, date, timestamp, open, high, low, close, volume,
			price_change, price_change_pct, volatility, volatility_pct,
			market_session, exchange, sector
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, item := range data {
		err := batch.Append(
			item.Symbol,
			item.Date,
			item.Timestamp,
			item.Open,
			item.High,
			item.Low,
			item.Close,
			item.Volume,
			item.PriceChange,
			item.PriceChangePct,
			item.Volatility,
			item.VolatilityPct,
			item.MarketSession,
			item.Exchange,
			item.Sector,
		)
		if err != nil {
			return fmt.Errorf("failed to append data: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	return nil
}

// GetTopPerformers returns top performing stocks by return percentage
func (c *ClickHouseClient) GetTopPerformers(limit int, timeframe string) ([]MarketAnalytics, error) {
	var query string
	var args []interface{}

	switch timeframe {
	case "1d":
		query = `
			SELECT symbol, date, max(timestamp) as timestamp,
				   argMax(close, timestamp) as close,
				   (argMax(close, timestamp) - argMin(open, timestamp)) / argMin(open, timestamp) * 100 as price_change_pct
			FROM market_analytics
			WHERE date = today()
			GROUP BY symbol, date
			ORDER BY price_change_pct DESC
			LIMIT ?
		`
		args = []interface{}{limit}
	case "7d":
		query = `
			SELECT symbol, max(date) as date, max(timestamp) as timestamp,
				   argMax(close, timestamp) as close,
				   (argMax(close, timestamp) - argMin(open, timestamp)) / argMin(open, timestamp) * 100 as price_change_pct
			FROM market_analytics
			WHERE date >= today() - INTERVAL 7 DAY
			GROUP BY symbol
			ORDER BY price_change_pct DESC
			LIMIT ?
		`
		args = []interface{}{limit}
	default:
		return nil, fmt.Errorf("unsupported timeframe: %s", timeframe)
	}

	rows, err := c.conn.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var results []MarketAnalytics
	for rows.Next() {
		var item MarketAnalytics
		err := rows.Scan(
			&item.Symbol,
			&item.Date,
			&item.Timestamp,
			&item.Close,
			&item.PriceChangePct,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, item)
	}

	return results, rows.Err()
}

// GetPortfolioPerformance returns portfolio performance analytics
func (c *ClickHouseClient) GetPortfolioPerformance(portfolioID string, days int) (*PortfolioAnalytics, error) {
	query := `
		SELECT
			portfolio_id,
			max(date) as date,
			max(timestamp) as timestamp,
			argMax(total_value, timestamp) as total_value,
			argMax(cumulative_return, timestamp) as cumulative_return,
			avg(volatility) as volatility,
			argMax(sharpe_ratio, timestamp) as sharpe_ratio,
			max(max_drawdown) as max_drawdown,
			argMax(var_95, timestamp) as var_95,
			argMax(beta, timestamp) as beta
		FROM portfolio_analytics
		WHERE portfolio_id = ? AND date >= today() - INTERVAL ? DAY
		GROUP BY portfolio_id
	`

	row := c.conn.QueryRow(context.Background(), query, portfolioID, days)

	var result PortfolioAnalytics
	err := row.Scan(
		&result.PortfolioID,
		&result.Date,
		&result.Timestamp,
		&result.TotalValue,
		&result.CumulativeReturn,
		&result.Volatility,
		&result.SharpeRatio,
		&result.MaxDrawdown,
		&result.VaR95,
		&result.Beta,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan portfolio performance: %w", err)
	}

	return &result, nil
}

// GetMarketVolatility calculates market-wide volatility metrics
func (c *ClickHouseClient) GetMarketVolatility(timeframe string) (map[string]float64, error) {
	query := `
		SELECT
			avg(volatility_pct) as avg_volatility,
			quantile(0.5)(volatility_pct) as median_volatility,
			quantile(0.95)(volatility_pct) as p95_volatility,
			max(volatility_pct) as max_volatility
		FROM market_analytics
		WHERE date >= today() - INTERVAL ? DAY
	`

	var days int
	switch timeframe {
	case "1d":
		days = 1
	case "7d":
		days = 7
	case "30d":
		days = 30
	default:
		return nil, fmt.Errorf("unsupported timeframe: %s", timeframe)
	}

	row := c.conn.QueryRow(context.Background(), query, days)

	var avgVol, medianVol, p95Vol, maxVol float64
	err := row.Scan(&avgVol, &medianVol, &p95Vol, &maxVol)
	if err != nil {
		return nil, fmt.Errorf("failed to scan volatility metrics: %w", err)
	}

	return map[string]float64{
		"average":    avgVol,
		"median":     medianVol,
		"percentile_95": p95Vol,
		"maximum":    maxVol,
	}, nil
}

// GetSectorPerformance returns performance by sector
func (c *ClickHouseClient) GetSectorPerformance() (map[string]float64, error) {
	query := `
		SELECT
			sector,
			avg(price_change_pct) as avg_return
		FROM market_analytics
		WHERE date = today() AND sector != ''
		GROUP BY sector
		ORDER BY avg_return DESC
	`

	rows, err := c.conn.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute sector query: %w", err)
	}
	defer rows.Close()

	results := make(map[string]float64)
	for rows.Next() {
		var sector string
		var avgReturn float64
		err := rows.Scan(&sector, &avgReturn)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sector data: %w", err)
		}
		results[sector] = avgReturn
	}

	return results, rows.Err()
}

// GetSystemMetrics returns database performance metrics
func (c *ClickHouseClient) GetSystemMetrics() (map[string]interface{}, error) {
	metrics := make(map[string]interface{})

	// Get table sizes
	row := c.conn.QueryRow(context.Background(), `
		SELECT
			sum(rows) as total_rows,
			sum(bytes_on_disk) as total_bytes
		FROM system.parts
		WHERE table IN ('market_analytics', 'portfolio_analytics')
	`)

	var totalRows uint64
	var totalBytes uint64
	if err := row.Scan(&totalRows, &totalBytes); err == nil {
		metrics["total_rows"] = totalRows
		metrics["total_size_mb"] = totalBytes / (1024 * 1024)
	}

	// Get query performance
	row = c.conn.QueryRow(context.Background(), `
		SELECT avg(query_duration_ms)
		FROM system.query_log
		WHERE event_time > now() - INTERVAL 1 HOUR
		AND type = 'QueryFinish'
	`)

	var avgQueryTime float64
	if err := row.Scan(&avgQueryTime); err == nil {
		metrics["avg_query_time_ms"] = avgQueryTime
	}

	return metrics, nil
}

// Close closes the ClickHouse connection
func (c *ClickHouseClient) Close() error {
	return c.conn.Close()
}
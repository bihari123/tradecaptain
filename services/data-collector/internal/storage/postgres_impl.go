package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"tradecaptain/data-collector/internal/models"
	"github.com/lib/pq"
)

// Actual implementation of key PostgreSQL functions
func NewPostgresDB(connectionString string) (*PostgresDB, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresDB{db: db}, nil
}

func (p *PostgresDB) SaveMarketData(ctx context.Context, data *models.MarketData) error {
	query := `
		INSERT INTO market_data (symbol, price, volume, high, low, open, close, change, change_percent, market_cap, timestamp, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (symbol, timestamp, source)
		DO UPDATE SET
			price = EXCLUDED.price,
			volume = EXCLUDED.volume,
			high = EXCLUDED.high,
			low = EXCLUDED.low,
			open = EXCLUDED.open,
			close = EXCLUDED.close,
			change = EXCLUDED.change,
			change_percent = EXCLUDED.change_percent,
			market_cap = EXCLUDED.market_cap
	`

	_, err := p.db.ExecContext(ctx, query,
		data.Symbol,
		data.Price,
		data.Volume,
		data.High,
		data.Low,
		data.Open,
		data.Close,
		data.Change,
		data.ChangePercent,
		data.MarketCap,
		data.Timestamp,
		data.Source,
	)

	if err != nil {
		return fmt.Errorf("failed to save market data: %w", err)
	}

	return nil
}

func (p *PostgresDB) GetMarketData(ctx context.Context, symbol string, from, to time.Time) ([]*models.MarketData, error) {
	query := `
		SELECT id, symbol, price, volume, high, low, open, close, change, change_percent, market_cap, timestamp, source
		FROM market_data
		WHERE symbol = $1 AND timestamp BETWEEN $2 AND $3
		ORDER BY timestamp ASC
	`

	rows, err := p.db.QueryContext(ctx, query, symbol, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to query market data: %w", err)
	}
	defer rows.Close()

	var results []*models.MarketData
	for rows.Next() {
		data := &models.MarketData{}
		err := rows.Scan(
			&data.ID,
			&data.Symbol,
			&data.Price,
			&data.Volume,
			&data.High,
			&data.Low,
			&data.Open,
			&data.Close,
			&data.Change,
			&data.ChangePercent,
			&data.MarketCap,
			&data.Timestamp,
			&data.Source,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan market data: %w", err)
		}
		results = append(results, data)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

func (p *PostgresDB) UpdateMarketDataBatch(ctx context.Context, data []*models.MarketData) error {
	if len(data) == 0 {
		return nil
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO market_data (symbol, price, volume, high, low, open, close, change, change_percent, market_cap, timestamp, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (symbol, timestamp, source)
		DO UPDATE SET
			price = EXCLUDED.price,
			volume = EXCLUDED.volume,
			high = EXCLUDED.high,
			low = EXCLUDED.low,
			open = EXCLUDED.open,
			close = EXCLUDED.close,
			change = EXCLUDED.change,
			change_percent = EXCLUDED.change_percent,
			market_cap = EXCLUDED.market_cap
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, item := range data {
		_, err := stmt.ExecContext(ctx,
			item.Symbol,
			item.Price,
			item.Volume,
			item.High,
			item.Low,
			item.Open,
			item.Close,
			item.Change,
			item.ChangePercent,
			item.MarketCap,
			item.Timestamp,
			item.Source,
		)
		if err != nil {
			return fmt.Errorf("failed to execute batch insert for %s: %w", item.Symbol, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
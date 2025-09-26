package models

import "time"

type MarketData struct {
	ID        int       `json:"id" db:"id"`
	Symbol    string    `json:"symbol" db:"symbol"`
	Price     float64   `json:"price" db:"price"`
	Volume    int64     `json:"volume" db:"volume"`
	High      float64   `json:"high" db:"high"`
	Low       float64   `json:"low" db:"low"`
	Open      float64   `json:"open" db:"open"`
	Close     float64   `json:"close" db:"close"`
	Change    float64   `json:"change" db:"change"`
	ChangePercent float64 `json:"change_percent" db:"change_percent"`
	MarketCap int64     `json:"market_cap" db:"market_cap"`
	Timestamp time.Time `json:"timestamp" db:"timestamp"`
	Source    string    `json:"source" db:"source"`
}

type CryptoData struct {
	ID           int       `json:"id" db:"id"`
	Symbol       string    `json:"symbol" db:"symbol"`
	Name         string    `json:"name" db:"name"`
	Price        float64   `json:"price" db:"price"`
	Volume24h    float64   `json:"volume_24h" db:"volume_24h"`
	MarketCap    float64   `json:"market_cap" db:"market_cap"`
	Change24h    float64   `json:"change_24h" db:"change_24h"`
	ChangePercent24h float64 `json:"change_percent_24h" db:"change_percent_24h"`
	Timestamp    time.Time `json:"timestamp" db:"timestamp"`
	Source       string    `json:"source" db:"source"`
}

type NewsArticle struct {
	ID          int       `json:"id" db:"id"`
	Title       string    `json:"title" db:"title"`
	Description string    `json:"description" db:"description"`
	URL         string    `json:"url" db:"url"`
	Source      string    `json:"source" db:"source"`
	Author      string    `json:"author" db:"author"`
	PublishedAt time.Time `json:"published_at" db:"published_at"`
	Category    string    `json:"category" db:"category"`
	Sentiment   float64   `json:"sentiment" db:"sentiment"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type EconomicIndicator struct {
	ID          int       `json:"id" db:"id"`
	Series      string    `json:"series" db:"series"`
	Title       string    `json:"title" db:"title"`
	Value       float64   `json:"value" db:"value"`
	Date        time.Time `json:"date" db:"date"`
	Units       string    `json:"units" db:"units"`
	Frequency   string    `json:"frequency" db:"frequency"`
	Source      string    `json:"source" db:"source"`
	LastUpdated time.Time `json:"last_updated" db:"last_updated"`
}
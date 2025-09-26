-- QuestDB Initialization Script
-- Optimized for ultra-high frequency financial data ingestion

-- Market data table optimized for time-series ingestion
CREATE TABLE IF NOT EXISTS market_data_realtime (
    symbol SYMBOL CAPACITY 1000 CACHE,          -- Symbol dictionary for memory efficiency
    price DOUBLE,                                -- Current price
    volume LONG,                                 -- Trading volume
    bid DOUBLE,                                  -- Bid price
    ask DOUBLE,                                  -- Ask price
    high DOUBLE,                                 -- Session high
    low DOUBLE,                                  -- Session low
    open DOUBLE,                                 -- Session open
    close DOUBLE,                                -- Previous close
    volatility_pct DOUBLE,                       -- Calculated volatility %
    risk_level SYMBOL,                           -- Risk classification
    market_session SYMBOL,                       -- Market session type
    exchange SYMBOL CAPACITY 100 CACHE,         -- Exchange identifier
    timestamp TIMESTAMP                          -- Designated timestamp column
) TIMESTAMP(timestamp) PARTITION BY HOUR;        -- Hourly partitioning for optimal performance

-- Order book data (Level 2 market data)
CREATE TABLE IF NOT EXISTS order_book (
    symbol SYMBOL CAPACITY 1000 CACHE,
    side SYMBOL,                                 -- 'BUY' or 'SELL'
    price DOUBLE,
    size DOUBLE,
    order_count INT,
    exchange SYMBOL CAPACITY 100 CACHE,
    timestamp TIMESTAMP
) TIMESTAMP(timestamp) PARTITION BY HOUR;

-- Trade executions
CREATE TABLE IF NOT EXISTS trades (
    symbol SYMBOL CAPACITY 1000 CACHE,
    price DOUBLE,
    size DOUBLE,
    side SYMBOL,                                 -- 'BUY' or 'SELL'
    trade_id LONG,
    exchange SYMBOL CAPACITY 100 CACHE,
    timestamp TIMESTAMP
) TIMESTAMP(timestamp) PARTITION BY DAY;

-- Portfolio snapshots (less frequent updates)
CREATE TABLE IF NOT EXISTS portfolio_snapshots (
    portfolio_id SYMBOL CAPACITY 1000 CACHE,
    total_value DOUBLE,
    cash DOUBLE,
    unrealized_pnl DOUBLE,
    realized_pnl DOUBLE,
    positions STRING,                            -- JSON string of positions
    risk_metrics STRING,                         -- JSON string of risk metrics
    timestamp TIMESTAMP
) TIMESTAMP(timestamp) PARTITION BY DAY;

-- Technical indicators (computed values)
CREATE TABLE IF NOT EXISTS technical_indicators (
    symbol SYMBOL CAPACITY 1000 CACHE,
    indicator_type SYMBOL,                       -- 'SMA', 'EMA', 'RSI', etc.
    period INT,                                  -- Calculation period
    value DOUBLE,
    timestamp TIMESTAMP
) TIMESTAMP(timestamp) PARTITION BY DAY;

-- Economic data (fundamental analysis)
CREATE TABLE IF NOT EXISTS economic_data (
    indicator_name SYMBOL CAPACITY 500 CACHE,
    country SYMBOL CAPACITY 50 CACHE,
    frequency SYMBOL,                            -- 'DAILY', 'WEEKLY', 'MONTHLY'
    value DOUBLE,
    unit SYMBOL,
    release_date TIMESTAMP,
    timestamp TIMESTAMP
) TIMESTAMP(timestamp) PARTITION BY MONTH;

-- Performance optimization indexes
-- QuestDB automatically creates efficient indexes for SYMBOL columns

-- Sample data insertion using InfluxDB Line Protocol (ILP) for maximum speed
-- market_data_realtime,symbol=AAPL,exchange=NASDAQ price=150.25,volume=1000000i,volatility_pct=2.5 1640995200000000000

-- Views for common queries
-- CREATE VIEW latest_prices AS (
--     SELECT symbol, price, volume, timestamp
--     FROM market_data_realtime
--     LATEST ON timestamp PARTITION BY symbol
-- );

-- Performance monitoring
-- QuestDB provides built-in metrics via SQL:
-- SELECT * FROM pg_stat_activity;
-- SELECT * FROM information_schema.tables;
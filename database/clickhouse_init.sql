-- ClickHouse Initialization Script
-- Optimized for analytical queries on financial data

-- Market analytics table (aggregated from QuestDB every 5 minutes)
CREATE TABLE IF NOT EXISTS market_analytics (
    symbol LowCardinality(String),               -- Memory-efficient string storage
    date Date,                                   -- Date for partitioning
    timestamp DateTime64(3),                     -- Millisecond precision

    -- OHLCV data
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    volume UInt64,

    -- Derived metrics
    price_change Float64,
    price_change_pct Float64,
    volatility Float64,
    volatility_pct Float64,

    -- Technical indicators
    sma_20 Nullable(Float64),
    sma_50 Nullable(Float64),
    ema_12 Nullable(Float64),
    ema_26 Nullable(Float64),
    rsi_14 Nullable(Float64),
    macd Nullable(Float64),

    -- Volume indicators
    volume_sma_20 Nullable(UInt64),
    volume_ratio Float64,

    -- Market context
    market_session LowCardinality(String),
    exchange LowCardinality(String),
    sector LowCardinality(String),

    -- Risk metrics
    beta Nullable(Float64),
    correlation_spy Nullable(Float64),
    drawdown_pct Nullable(Float64)

) ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)                      -- Monthly partitioning
ORDER BY (symbol, timestamp)                     -- Primary key for fast lookups
SETTINGS index_granularity = 8192;

-- Portfolio performance analytics
CREATE TABLE IF NOT EXISTS portfolio_analytics (
    portfolio_id LowCardinality(String),
    date Date,
    timestamp DateTime64(3),

    -- Portfolio metrics
    total_value Float64,
    cash Float64,
    invested_value Float64,

    -- Performance metrics
    daily_return Float64,
    cumulative_return Float64,
    volatility Float64,
    sharpe_ratio Nullable(Float64),
    sortino_ratio Nullable(Float64),
    max_drawdown Float64,

    -- Risk metrics
    var_95 Float64,                              -- Value at Risk 95%
    cvar_95 Float64,                             -- Conditional VaR 95%
    beta Float64,

    -- Position metrics
    position_count UInt32,
    concentration_top_5 Float64,                 -- % in top 5 positions
    sector_diversification Float64,

    -- Attribution
    stock_selection_alpha Float64,
    market_timing_alpha Float64

) ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (portfolio_id, timestamp)
SETTINGS index_granularity = 4096;

-- Trade analytics (for execution analysis)
CREATE TABLE IF NOT EXISTS trade_analytics (
    trade_id UInt64,
    portfolio_id LowCardinality(String),
    symbol LowCardinality(String),
    date Date,
    timestamp DateTime64(3),

    -- Trade details
    side Enum8('BUY' = 1, 'SELL' = 2),
    quantity Float64,
    price Float64,
    commission Float64,

    -- Execution quality
    market_price Float64,                        -- Market price at execution
    slippage Float64,                           -- Execution slippage
    implementation_shortfall Float64,

    -- Strategy context
    strategy LowCardinality(String),
    signal_strength Float64,
    conviction_level Enum8('LOW' = 1, 'MEDIUM' = 2, 'HIGH' = 3),

    -- Market impact
    volume_participation Float64,               -- % of market volume
    price_impact Float64

) ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (symbol, timestamp)
SETTINGS index_granularity = 8192;

-- Economic indicators analytics
CREATE TABLE IF NOT EXISTS economic_analytics (
    indicator LowCardinality(String),
    country LowCardinality(String),
    date Date,
    timestamp DateTime64(3),

    value Float64,
    previous_value Nullable(Float64),
    forecast Nullable(Float64),

    -- Derived metrics
    change_abs Float64,
    change_pct Float64,
    surprise_factor Nullable(Float64),          -- (Actual - Forecast) / Forecast

    -- Impact analysis
    market_impact_score Float64,
    volatility_impact Float64,

    -- Frequency and timing
    frequency Enum8('DAILY' = 1, 'WEEKLY' = 2, 'MONTHLY' = 3, 'QUARTERLY' = 4),
    release_importance Enum8('LOW' = 1, 'MEDIUM' = 2, 'HIGH' = 3)

) ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (country, indicator, timestamp)
SETTINGS index_granularity = 4096;

-- Materialized views for real-time aggregations
CREATE MATERIALIZED VIEW market_daily_summary
ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (symbol, date)
AS SELECT
    symbol,
    toDate(timestamp) as date,
    argMin(open, timestamp) as open,
    max(high) as high,
    min(low) as low,
    argMax(close, timestamp) as close,
    sum(volume) as total_volume,
    count() as tick_count,
    avg(volatility_pct) as avg_volatility
FROM market_analytics
GROUP BY symbol, toDate(timestamp);

-- Performance optimization settings
SET max_memory_usage = '4GB';
SET max_threads = 8;
SET optimize_on_insert = 1;

-- Compression settings for storage efficiency
ALTER TABLE market_analytics MODIFY SETTING compress_block_size = 1048576;
ALTER TABLE portfolio_analytics MODIFY SETTING compress_block_size = 1048576;
ALTER TABLE trade_analytics MODIFY SETTING compress_block_size = 1048576;
use rocksdb::{DB, Options, WriteBatch, IteratorMode};
use serde::{Serialize, Deserialize};
use std::path::Path;
use std::sync::Arc;
use anyhow::Result;
use rmp_serde as msgpack;

/// Ultra-fast RocksDB-based persistence for calculation results
pub struct UltraFastDB {
    db: Arc<DB>,
}

impl UltraFastDB {
    /// Creates a new optimized RocksDB instance
    pub fn new<P: AsRef<Path>>(path: P) -> Result<Self> {
        let mut opts = Options::default();

        // Optimize for write-heavy workloads
        opts.create_if_missing(true);
        opts.set_max_background_jobs(8);
        opts.set_max_write_buffer_number(6);
        opts.set_write_buffer_size(128 * 1024 * 1024); // 128MB
        opts.set_target_file_size_base(256 * 1024 * 1024); // 256MB
        opts.set_compression_type(rocksdb::DBCompressionType::Lz4);

        // Enable WAL for durability with minimal performance impact
        opts.set_wal_ttl_seconds(300); // 5 minutes
        opts.set_wal_size_limit_mb(1024); // 1GB

        // Optimize for SSD
        opts.set_allow_mmap_reads(true);
        opts.set_allow_mmap_writes(false); // Safer for writes

        let db = DB::open(&opts, path)?;
        Ok(Self { db: Arc::new(db) })
    }

    /// Store data with MessagePack serialization (2x faster than JSON)
    pub fn put<K, V>(&self, key: K, value: &V) -> Result<()>
    where
        K: AsRef<[u8]>,
        V: Serialize,
    {
        let serialized = msgpack::to_vec(value)?;
        self.db.put(key, serialized)?;
        Ok(())
    }

    /// Retrieve and deserialize data
    pub fn get<K, V>(&self, key: K) -> Result<Option<V>>
    where
        K: AsRef<[u8]>,
        V: for<'de> Deserialize<'de>,
    {
        match self.db.get(key)? {
            Some(data) => {
                let value: V = msgpack::from_slice(&data)?;
                Ok(Some(value))
            }
            None => Ok(None),
        }
    }

    /// Batch write for maximum throughput
    pub fn batch_write<K, V>(&self, entries: Vec<(K, V)>) -> Result<()>
    where
        K: AsRef<[u8]>,
        V: Serialize,
    {
        let mut batch = WriteBatch::default();

        for (key, value) in entries {
            let serialized = msgpack::to_vec(&value)?;
            batch.put(key, serialized);
        }

        self.db.write(batch)?;
        Ok(())
    }

    /// Delete a key
    pub fn delete<K>(&self, key: K) -> Result<()>
    where
        K: AsRef<[u8]>,
    {
        self.db.delete(key)?;
        Ok(())
    }

    /// Scan with prefix for range queries
    pub fn scan_prefix<V>(&self, prefix: &[u8]) -> Result<Vec<(Vec<u8>, V)>>
    where
        V: for<'de> Deserialize<'de>,
    {
        let mut results = Vec::new();
        let iter = self.db.iterator(IteratorMode::From(prefix, rocksdb::Direction::Forward));

        for item in iter {
            let (key, value) = item?;

            // Stop if key doesn't start with prefix
            if !key.starts_with(prefix) {
                break;
            }

            let deserialized: V = msgpack::from_slice(&value)?;
            results.push((key.to_vec(), deserialized));
        }

        Ok(results)
    }

    /// Get database statistics
    pub fn stats(&self) -> Result<String> {
        Ok(self.db.property_value("rocksdb.stats")?.unwrap_or_default())
    }

    /// Force compaction for optimal read performance
    pub fn compact(&self) -> Result<()> {
        self.db.compact_range::<&[u8], &[u8]>(None, None);
        Ok(())
    }
}

/// Time-series data structure for financial metrics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TimeSeriesPoint {
    pub timestamp: u64,
    pub value: f64,
    pub metadata: Option<Vec<u8>>,
}

/// Portfolio state optimized for fast serialization
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PortfolioState {
    pub portfolio_id: String,
    pub timestamp: u64,
    pub total_value: f64,
    pub positions: Vec<Position>,
    pub cash: f64,
    pub unrealized_pnl: f64,
    pub realized_pnl: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Position {
    pub symbol: String,
    pub quantity: f64,
    pub avg_cost: f64,
    pub current_price: f64,
    pub unrealized_pnl: f64,
}

impl UltraFastDB {
    /// Store portfolio state with timestamp-based key
    pub fn store_portfolio_state(&self, state: &PortfolioState) -> Result<()> {
        let key = format!("portfolio:{}:{}", state.portfolio_id, state.timestamp);
        self.put(key.as_bytes(), state)
    }

    /// Retrieve latest portfolio state
    pub fn get_latest_portfolio_state(&self, portfolio_id: &str) -> Result<Option<PortfolioState>> {
        let prefix = format!("portfolio:{}:", portfolio_id);
        let states = self.scan_prefix::<PortfolioState>(prefix.as_bytes())?;

        // Get the most recent state (keys are timestamp-ordered)
        Ok(states.into_iter().last().map(|(_, state)| state))
    }

    /// Store time-series data point
    pub fn store_metric(&self, metric_name: &str, point: &TimeSeriesPoint) -> Result<()> {
        let key = format!("metric:{}:{}", metric_name, point.timestamp);
        self.put(key.as_bytes(), point)
    }

    /// Get time-series data for a metric within a time range
    pub fn get_metric_range(
        &self,
        metric_name: &str,
        start_time: u64,
        end_time: u64,
    ) -> Result<Vec<TimeSeriesPoint>> {
        let prefix = format!("metric:{}:", metric_name);
        let all_points = self.scan_prefix::<TimeSeriesPoint>(prefix.as_bytes())?;

        let filtered: Vec<TimeSeriesPoint> = all_points
            .into_iter()
            .map(|(_, point)| point)
            .filter(|point| point.timestamp >= start_time && point.timestamp <= end_time)
            .collect();

        Ok(filtered)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;
    use std::time::Instant;

    #[test]
    fn test_persistence_performance() -> Result<()> {
        let temp_dir = TempDir::new()?;
        let db = UltraFastDB::new(temp_dir.path())?;

        // Test batch write performance
        let start = Instant::now();
        let batch_data: Vec<(String, TimeSeriesPoint)> = (0..10000)
            .map(|i| {
                (
                    format!("key_{}", i),
                    TimeSeriesPoint {
                        timestamp: i as u64,
                        value: i as f64 * 1.5,
                        metadata: None,
                    },
                )
            })
            .collect();

        db.batch_write(batch_data)?;
        let write_duration = start.elapsed();

        // Test read performance
        let start = Instant::now();
        for i in 0..10000 {
            let key = format!("key_{}", i);
            let _: Option<TimeSeriesPoint> = db.get(key.as_bytes())?;
        }
        let read_duration = start.elapsed();

        println!("Batch write: {:?} for 10k items", write_duration);
        println!("Individual reads: {:?} for 10k items", read_duration);

        // Should be well under 100ms for both operations
        assert!(write_duration.as_millis() < 100);
        assert!(read_duration.as_millis() < 100);

        Ok(())
    }

    #[test]
    fn test_portfolio_operations() -> Result<()> {
        let temp_dir = TempDir::new()?;
        let db = UltraFastDB::new(temp_dir.path())?;

        let portfolio = PortfolioState {
            portfolio_id: "test_portfolio".to_string(),
            timestamp: 1640995200, // 2022-01-01
            total_value: 100000.0,
            positions: vec![
                Position {
                    symbol: "AAPL".to_string(),
                    quantity: 100.0,
                    avg_cost: 150.0,
                    current_price: 155.0,
                    unrealized_pnl: 500.0,
                },
            ],
            cash: 85000.0,
            unrealized_pnl: 500.0,
            realized_pnl: 0.0,
        };

        db.store_portfolio_state(&portfolio)?;

        let retrieved = db.get_latest_portfolio_state("test_portfolio")?;
        assert!(retrieved.is_some());
        assert_eq!(retrieved.unwrap().total_value, 100000.0);

        Ok(())
    }
}
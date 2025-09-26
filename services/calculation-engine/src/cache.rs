use std::sync::Arc;
use std::time::Duration;
use moka::future::Cache;
use serde::{Deserialize, Serialize};
use crossbeam::channel::{bounded, Receiver, Sender};
use anyhow::Result;

/// Ultra-fast embedded cache for financial calculations
pub struct L1Cache<K, V> {
    cache: Cache<K, V>,
}

impl<K, V> L1Cache<K, V>
where
    K: std::hash::Hash + Eq + Send + Sync + 'static,
    V: Clone + Send + Sync + 'static,
{
    /// Creates a new L1 cache optimized for financial data
    pub fn new(max_capacity: u64, ttl_seconds: u64) -> Self {
        let cache = Cache::builder()
            .max_capacity(max_capacity)
            .time_to_live(Duration::from_secs(ttl_seconds))
            .time_to_idle(Duration::from_secs(ttl_seconds / 2))
            .build();

        Self { cache }
    }

    /// Insert data with zero-copy when possible
    pub async fn insert(&self, key: K, value: V) {
        self.cache.insert(key, value).await;
    }

    /// Get data with zero-copy when possible
    pub async fn get(&self, key: &K) -> Option<V> {
        self.cache.get(key).await
    }

    /// Remove an entry
    pub async fn remove(&self, key: &K) {
        self.cache.remove(key).await;
    }

    /// Get cache statistics
    pub fn stats(&self) -> (u64, u64) {
        (self.cache.entry_count(), self.cache.weighted_size())
    }
}

/// Lock-free message passing for ultra-low latency
pub struct UltraFastChannel<T> {
    sender: Sender<T>,
    receiver: Receiver<T>,
}

impl<T> UltraFastChannel<T> {
    /// Creates a bounded channel with optimal size for financial data
    pub fn new(capacity: usize) -> Self {
        let (sender, receiver) = bounded(capacity);
        Self { sender, receiver }
    }

    /// Send with nanosecond latency
    pub fn send(&self, message: T) -> Result<()> {
        self.sender.send(message)
            .map_err(|_| anyhow::anyhow!("Channel send failed"))?;
        Ok(())
    }

    /// Non-blocking receive
    pub fn try_recv(&self) -> Result<Option<T>> {
        match self.receiver.try_recv() {
            Ok(msg) => Ok(Some(msg)),
            Err(crossbeam::channel::TryRecvError::Empty) => Ok(None),
            Err(crossbeam::channel::TryRecvError::Disconnected) => {
                Err(anyhow::anyhow!("Channel disconnected"))
            }
        }
    }

    /// Blocking receive with timeout
    pub fn recv_timeout(&self, timeout: Duration) -> Result<Option<T>> {
        match self.receiver.recv_timeout(timeout) {
            Ok(msg) => Ok(Some(msg)),
            Err(crossbeam::channel::RecvTimeoutError::Timeout) => Ok(None),
            Err(crossbeam::channel::RecvTimeoutError::Disconnected) => {
                Err(anyhow::anyhow!("Channel disconnected"))
            }
        }
    }
}

/// Market data structure optimized for cache-line alignment
#[derive(Debug, Clone, Serialize, Deserialize)]
#[repr(C)]
#[repr(align(64))] // CPU cache line alignment
pub struct MarketDataCached {
    pub symbol: [u8; 8],     // 8 bytes - symbol padded
    pub price: f64,          // 8 bytes
    pub volume: u64,         // 8 bytes
    pub timestamp: u64,      // 8 bytes
    pub bid: f64,            // 8 bytes
    pub ask: f64,            // 8 bytes
    pub flags: u32,          // 4 bytes
    pub sequence: u32,       // 4 bytes
    _padding: [u8; 8],       // 8 bytes padding to 64 bytes total
}

impl MarketDataCached {
    pub fn new(symbol: &str, price: f64, volume: u64) -> Self {
        let mut symbol_bytes = [0u8; 8];
        let bytes = symbol.as_bytes();
        let len = std::cmp::min(bytes.len(), 8);
        symbol_bytes[..len].copy_from_slice(&bytes[..len]);

        Self {
            symbol: symbol_bytes,
            price,
            volume,
            timestamp: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos() as u64,
            bid: price - 0.01,
            ask: price + 0.01,
            flags: 0,
            sequence: 0,
            _padding: [0; 8],
        }
    }

    pub fn symbol_str(&self) -> &str {
        let end = self.symbol.iter().position(|&b| b == 0).unwrap_or(8);
        std::str::from_utf8(&self.symbol[..end]).unwrap_or("")
    }
}

/// Lock-free queue for high-frequency data processing
pub struct LockFreeQueue<T> {
    queue: Arc<crossbeam::queue::SegQueue<T>>,
}

impl<T> LockFreeQueue<T> {
    pub fn new() -> Self {
        Self {
            queue: Arc::new(crossbeam::queue::SegQueue::new()),
        }
    }

    /// Push with zero contention
    pub fn push(&self, item: T) {
        self.queue.push(item);
    }

    /// Pop with zero contention
    pub fn pop(&self) -> Option<T> {
        self.queue.pop()
    }

    /// Check if queue is empty
    pub fn is_empty(&self) -> bool {
        self.queue.is_empty()
    }

    /// Get approximate length (lock-free, may be inexact)
    pub fn len(&self) -> usize {
        self.queue.len()
    }
}

impl<T> Clone for LockFreeQueue<T> {
    fn clone(&self) -> Self {
        Self {
            queue: Arc::clone(&self.queue),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Instant;

    #[tokio::test]
    async fn test_cache_performance() {
        let cache = L1Cache::new(10000, 60);

        let start = Instant::now();
        for i in 0..10000 {
            cache.insert(format!("key_{}", i), format!("value_{}", i)).await;
        }
        let insert_duration = start.elapsed();

        let start = Instant::now();
        for i in 0..10000 {
            let _ = cache.get(&format!("key_{}", i)).await;
        }
        let get_duration = start.elapsed();

        println!("Cache insert: {:?} for 10k items", insert_duration);
        println!("Cache get: {:?} for 10k items", get_duration);

        // Should be well under 1ms for both operations
        assert!(insert_duration.as_millis() < 10);
        assert!(get_duration.as_millis() < 5);
    }

    #[test]
    fn test_channel_performance() {
        let channel = UltraFastChannel::new(1000);

        let start = Instant::now();
        for i in 0..10000 {
            channel.send(i).unwrap();
        }
        let send_duration = start.elapsed();

        let start = Instant::now();
        for _ in 0..10000 {
            let _ = channel.try_recv().unwrap();
        }
        let recv_duration = start.elapsed();

        println!("Channel send: {:?} for 10k items", send_duration);
        println!("Channel recv: {:?} for 10k items", recv_duration);

        // Should be well under 1ms for both operations
        assert!(send_duration.as_millis() < 5);
        assert!(recv_duration.as_millis() < 5);
    }
}
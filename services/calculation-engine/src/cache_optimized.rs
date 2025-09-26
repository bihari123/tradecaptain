use std::sync::atomic::{AtomicU64, AtomicU32, Ordering};
use crossbeam::utils::CachePadded;
use std::alloc::{alloc, dealloc, Layout};
use std::ptr;

/// Cache-line aligned market data structure (64 bytes = 1 cache line)
/// Optimized to fit exactly in one CPU cache line for maximum performance
#[derive(Debug, Clone)]
#[repr(C)]
#[repr(align(64))]
pub struct CacheOptimizedMarketData {
    // Core price data (32 bytes)
    pub symbol: [u8; 8],     // 8 bytes - symbol padded with zeros
    pub price: f64,          // 8 bytes
    pub volume: u64,         // 8 bytes
    pub timestamp: u64,      // 8 bytes (nanoseconds since epoch)

    // Additional price levels (24 bytes)
    pub bid: f64,            // 8 bytes
    pub ask: f64,            // 8 bytes
    pub high: f64,           // 8 bytes

    // Metadata and flags (8 bytes)
    pub low: f32,            // 4 bytes
    pub sequence: u32,       // 4 bytes - for ordering
}

impl CacheOptimizedMarketData {
    /// Create new market data with symbol string
    pub fn new(symbol: &str, price: f64, volume: u64) -> Self {
        let mut symbol_bytes = [0u8; 8];
        let bytes = symbol.as_bytes();
        let len = std::cmp::min(bytes.len(), 8);
        symbol_bytes[..len].copy_from_slice(&bytes[..len]);

        Self {
            symbol: symbol_bytes,
            price,
            volume,
            timestamp: current_timestamp_nanos(),
            bid: price - 0.01,
            ask: price + 0.01,
            high: price,
            low: price as f32,
            sequence: 0,
        }
    }

    /// Get symbol as string
    pub fn symbol_str(&self) -> &str {
        let end = self.symbol.iter().position(|&b| b == 0).unwrap_or(8);
        std::str::from_utf8(&self.symbol[..end]).unwrap_or("")
    }

    /// Set sequence number for ordering
    pub fn with_sequence(mut self, seq: u32) -> Self {
        self.sequence = seq;
        self
    }
}

/// Cache-line aligned atomic counters to prevent false sharing
#[repr(C)]
#[repr(align(64))]
pub struct CacheAlignedCounters {
    pub trades_count: CachePadded<AtomicU64>,
    pub volume_total: CachePadded<AtomicU64>,
    pub price_updates: CachePadded<AtomicU64>,
    pub last_timestamp: CachePadded<AtomicU64>,
}

impl CacheAlignedCounters {
    pub fn new() -> Self {
        Self {
            trades_count: CachePadded::new(AtomicU64::new(0)),
            volume_total: CachePadded::new(AtomicU64::new(0)),
            price_updates: CachePadded::new(AtomicU64::new(0)),
            last_timestamp: CachePadded::new(AtomicU64::new(0)),
        }
    }

    pub fn update_trade(&self, volume: u64, timestamp: u64) {
        self.trades_count.fetch_add(1, Ordering::Relaxed);
        self.volume_total.fetch_add(volume, Ordering::Relaxed);
        self.last_timestamp.store(timestamp, Ordering::Relaxed);
    }

    pub fn update_price(&self, timestamp: u64) {
        self.price_updates.fetch_add(1, Ordering::Relaxed);
        self.last_timestamp.store(timestamp, Ordering::Relaxed);
    }

    pub fn get_stats(&self) -> CounterStats {
        CounterStats {
            trades_count: self.trades_count.load(Ordering::Relaxed),
            volume_total: self.volume_total.load(Ordering::Relaxed),
            price_updates: self.price_updates.load(Ordering::Relaxed),
            last_timestamp: self.last_timestamp.load(Ordering::Relaxed),
        }
    }
}

#[derive(Debug, Clone)]
pub struct CounterStats {
    pub trades_count: u64,
    pub volume_total: u64,
    pub price_updates: u64,
    pub last_timestamp: u64,
}

/// High-performance price array using cache-friendly layout
/// Data is stored in Structure of Arrays (SoA) format for vectorization
pub struct CacheOptimizedPriceArray {
    capacity: usize,
    length: usize,

    // Separate arrays for better cache utilization and SIMD
    prices: *mut f64,
    volumes: *mut u64,
    timestamps: *mut u64,
    symbols: *mut [u8; 8],

    layout_prices: Layout,
    layout_volumes: Layout,
    layout_timestamps: Layout,
    layout_symbols: Layout,
}

impl CacheOptimizedPriceArray {
    pub fn new(capacity: usize) -> Self {
        unsafe {
            // Allocate aligned memory for each array
            let layout_prices = Layout::from_size_align_unchecked(capacity * 8, 64);
            let layout_volumes = Layout::from_size_align_unchecked(capacity * 8, 64);
            let layout_timestamps = Layout::from_size_align_unchecked(capacity * 8, 64);
            let layout_symbols = Layout::from_size_align_unchecked(capacity * 8, 64);

            let prices = alloc(layout_prices) as *mut f64;
            let volumes = alloc(layout_volumes) as *mut u64;
            let timestamps = alloc(layout_timestamps) as *mut u64;
            let symbols = alloc(layout_symbols) as *mut [u8; 8];

            // Initialize memory
            ptr::write_bytes(prices, 0, capacity);
            ptr::write_bytes(volumes, 0, capacity);
            ptr::write_bytes(timestamps, 0, capacity);
            ptr::write_bytes(symbols, 0, capacity);

            Self {
                capacity,
                length: 0,
                prices,
                volumes,
                timestamps,
                symbols,
                layout_prices,
                layout_volumes,
                layout_timestamps,
                layout_symbols,
            }
        }
    }

    /// Add new price data (optimized for cache performance)
    pub fn push(&mut self, data: &CacheOptimizedMarketData) {
        if self.length >= self.capacity {
            panic!("Array capacity exceeded");
        }

        unsafe {
            *self.prices.add(self.length) = data.price;
            *self.volumes.add(self.length) = data.volume;
            *self.timestamps.add(self.length) = data.timestamp;
            *self.symbols.add(self.length) = data.symbol;
        }

        self.length += 1;
    }

    /// Batch update prices (vectorizable operation)
    pub fn update_prices_vectorized(&mut self, start_idx: usize, new_prices: &[f64]) {
        if start_idx + new_prices.len() > self.length {
            panic!("Index out of bounds");
        }

        unsafe {
            // This operation can be auto-vectorized by the compiler
            let dst = self.prices.add(start_idx);
            ptr::copy_nonoverlapping(new_prices.as_ptr(), dst, new_prices.len());
        }
    }

    /// Get price slice for vectorized operations
    pub fn get_prices_slice(&self, start: usize, len: usize) -> &[f64] {
        if start + len > self.length {
            panic!("Index out of bounds");
        }

        unsafe {
            std::slice::from_raw_parts(self.prices.add(start), len)
        }
    }

    /// Calculate average price using vectorized operations
    pub fn calculate_avg_price_vectorized(&self, start: usize, len: usize) -> f64 {
        let prices = self.get_prices_slice(start, len);

        // This can be auto-vectorized by LLVM
        let sum: f64 = prices.iter().sum();
        sum / len as f64
    }

    /// Find maximum price using vectorized operations
    pub fn find_max_price_vectorized(&self, start: usize, len: usize) -> f64 {
        let prices = self.get_prices_slice(start, len);

        // This can be auto-vectorized by LLVM
        prices.iter().fold(f64::NEG_INFINITY, |a, &b| a.max(b))
    }

    pub fn len(&self) -> usize {
        self.length
    }

    pub fn capacity(&self) -> usize {
        self.capacity
    }
}

impl Drop for CacheOptimizedPriceArray {
    fn drop(&mut self) {
        unsafe {
            dealloc(self.prices as *mut u8, self.layout_prices);
            dealloc(self.volumes as *mut u8, self.layout_volumes);
            dealloc(self.timestamps as *mut u8, self.layout_timestamps);
            dealloc(self.symbols as *mut u8, self.layout_symbols);
        }
    }
}

/// Cache-optimized moving average calculator
/// Uses circular buffer with cache-friendly memory layout
pub struct CacheOptimizedMovingAverage {
    buffer: Vec<f64>,
    index: usize,
    sum: f64,
    count: usize,
    period: usize,
    filled: bool,
}

impl CacheOptimizedMovingAverage {
    pub fn new(period: usize) -> Self {
        Self {
            buffer: vec![0.0; period],
            index: 0,
            sum: 0.0,
            count: 0,
            period,
            filled: false,
        }
    }

    /// Add new value and return updated moving average
    /// Optimized to minimize cache misses
    pub fn add(&mut self, value: f64) -> f64 {
        let old_value = self.buffer[self.index];
        self.buffer[self.index] = value;

        if self.filled {
            // Replace old value in sum
            self.sum = self.sum - old_value + value;
        } else {
            // Still filling the buffer
            self.sum += value;
            self.count += 1;
            if self.count == self.period {
                self.filled = true;
            }
        }

        // Advance index with efficient modulo using bitwise AND
        // (only works if period is power of 2, otherwise use % operator)
        self.index = (self.index + 1) % self.period;

        if self.filled {
            self.sum / self.period as f64
        } else {
            self.sum / self.count as f64
        }
    }

    pub fn current_average(&self) -> f64 {
        if self.filled {
            self.sum / self.period as f64
        } else if self.count > 0 {
            self.sum / self.count as f64
        } else {
            0.0
        }
    }
}

/// Prefetch hint for cache optimization
#[inline(always)]
pub fn prefetch_read<T>(ptr: *const T) {
    #[cfg(target_arch = "x86_64")]
    unsafe {
        std::arch::x86_64::_mm_prefetch(ptr as *const i8, std::arch::x86_64::_MM_HINT_T0);
    }
}

/// Get current timestamp in nanoseconds
fn current_timestamp_nanos() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_nanos() as u64
}

/// Memory allocation aligned to cache line boundaries
pub fn allocate_cache_aligned<T>(count: usize) -> *mut T {
    let size = count * std::mem::size_of::<T>();
    let layout = Layout::from_size_align(size, 64).unwrap();
    unsafe { alloc(layout) as *mut T }
}

/// Deallocate cache-aligned memory
pub fn deallocate_cache_aligned<T>(ptr: *mut T, count: usize) {
    let size = count * std::mem::size_of::<T>();
    let layout = Layout::from_size_align(size, 64).unwrap();
    unsafe { dealloc(ptr as *mut u8, layout) };
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Instant;

    #[test]
    fn test_cache_aligned_size() {
        // Verify our market data structure is exactly 64 bytes
        assert_eq!(std::mem::size_of::<CacheOptimizedMarketData>(), 64);
        assert_eq!(std::mem::align_of::<CacheOptimizedMarketData>(), 64);
    }

    #[test]
    fn test_price_array_vectorization() {
        let mut array = CacheOptimizedPriceArray::new(10000);

        // Fill with test data
        for i in 0..1000 {
            let data = CacheOptimizedMarketData::new("AAPL", 150.0 + i as f64, 1000);
            array.push(&data);
        }

        let start = Instant::now();
        let avg = array.calculate_avg_price_vectorized(0, 1000);
        let duration = start.elapsed();

        println!("Vectorized average calculation: {:?}", duration);
        assert!(avg > 150.0);

        // Should be very fast due to vectorization
        assert!(duration.as_nanos() < 10_000); // Less than 10 microseconds
    }

    #[test]
    fn test_moving_average_performance() {
        let mut ma = CacheOptimizedMovingAverage::new(100);
        let iterations = 100_000;

        let start = Instant::now();
        for i in 0..iterations {
            ma.add(100.0 + (i as f64) * 0.01);
        }
        let duration = start.elapsed();

        let avg_latency_ns = duration.as_nanos() / iterations as u128;
        println!("Moving average per operation: {} ns", avg_latency_ns);

        // Should be well under 100ns per operation
        assert!(avg_latency_ns < 100);
    }

    #[test]
    fn test_cache_aligned_counters() {
        let counters = CacheAlignedCounters::new();

        let num_threads = 8;
        let operations_per_thread = 100_000;

        let start = Instant::now();
        std::thread::scope(|s| {
            for _ in 0..num_threads {
                s.spawn(|| {
                    for i in 0..operations_per_thread {
                        counters.update_trade(100, i as u64);
                    }
                });
            }
        });
        let duration = start.elapsed();

        let stats = counters.get_stats();
        assert_eq!(stats.trades_count, num_threads * operations_per_thread);

        let throughput = (num_threads * operations_per_thread) as f64 / duration.as_secs_f64();
        println!("Cache-aligned counter throughput: {:.0} ops/sec", throughput);

        // Should achieve high throughput due to cache alignment
        assert!(throughput > 1_000_000.0); // > 1M ops/sec
    }
}
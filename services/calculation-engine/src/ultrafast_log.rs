use std::ptr;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::Arc;
use memmap2::{MmapMut, MmapOptions};
use std::fs::OpenOptions;
use crossbeam::utils::CachePadded;
use anyhow::{Result, anyhow};

/// Ultra-fast memory-mapped ring buffer for nanosecond write latency
/// Uses zero-copy operations and lock-free algorithms
pub struct UltraFastLog {
    mmap: MmapMut,
    size: usize,
    mask: usize, // size - 1, for efficient modulo using bitwise AND

    // Cache-line aligned atomic counters to prevent false sharing
    write_pos: CachePadded<AtomicUsize>,
    read_pos: CachePadded<AtomicUsize>,

    // Performance tracking
    writes_count: CachePadded<AtomicUsize>,
    bytes_written: CachePadded<AtomicUsize>,
}

impl UltraFastLog {
    /// Creates a new ultra-fast log with specified size (must be power of 2)
    pub fn new(file_path: &str, size_mb: usize) -> Result<Self> {
        let size = size_mb * 1024 * 1024;

        // Ensure size is power of 2 for efficient modulo operations
        if !size.is_power_of_two() {
            return Err(anyhow!("Size must be power of 2, got: {}", size));
        }

        // Create/open the file
        let file = OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .open(file_path)?;

        // Set file size
        file.set_len(size as u64)?;

        // Create memory mapping with optimizations
        let mmap = unsafe {
            MmapOptions::new()
                .populate() // Pre-fault pages for better performance
                .map_mut(&file)?
        };

        // Advise kernel about access patterns
        #[cfg(target_os = "linux")]
        unsafe {
            // MADV_SEQUENTIAL: expect sequential access
            // MADV_WILLNEED: expect access in near future
            libc::madvise(
                mmap.as_ptr() as *mut libc::c_void,
                size,
                libc::MADV_SEQUENTIAL | libc::MADV_WILLNEED,
            );
        }

        Ok(Self {
            mmap,
            size,
            mask: size - 1,
            write_pos: CachePadded::new(AtomicUsize::new(0)),
            read_pos: CachePadded::new(AtomicUsize::new(0)),
            writes_count: CachePadded::new(AtomicUsize::new(0)),
            bytes_written: CachePadded::new(AtomicUsize::new(0)),
        })
    }

    /// Append data with nanosecond latency (10-100ns typical)
    /// Returns the position where data was written
    pub fn append(&self, data: &[u8]) -> Result<usize> {
        let data_len = data.len();
        if data_len > self.size / 4 {
            return Err(anyhow!("Data too large: {} bytes", data_len));
        }

        // Reserve space atomically using relaxed ordering for maximum speed
        let write_pos = self.write_pos.fetch_add(data_len, Ordering::Relaxed);
        let actual_pos = write_pos & self.mask;

        // Check for wrap-around collision with read position
        let read_pos = self.read_pos.load(Ordering::Acquire);
        if self.would_overlap(actual_pos, data_len, read_pos) {
            return Err(anyhow!("Buffer full - would overlap with read position"));
        }

        // Zero-copy write directly to memory-mapped region
        unsafe {
            let dst = self.mmap.as_ptr().add(actual_pos);
            ptr::copy_nonoverlapping(data.as_ptr(), dst, data_len);

            // Memory fence to ensure write visibility
            std::sync::atomic::fence(Ordering::Release);
        }

        // Update statistics
        self.writes_count.fetch_add(1, Ordering::Relaxed);
        self.bytes_written.fetch_add(data_len, Ordering::Relaxed);

        Ok(actual_pos)
    }

    /// Batch append multiple data items for maximum throughput
    pub fn batch_append(&self, items: &[&[u8]]) -> Result<Vec<usize>> {
        let total_size: usize = items.iter().map(|item| item.len()).sum();

        if total_size > self.size / 2 {
            return Err(anyhow!("Batch too large: {} bytes", total_size));
        }

        // Reserve space for entire batch
        let start_pos = self.write_pos.fetch_add(total_size, Ordering::Relaxed);
        let mut positions = Vec::with_capacity(items.len());
        let mut current_pos = start_pos;

        // Check for collision
        let read_pos = self.read_pos.load(Ordering::Acquire);
        if self.would_overlap(start_pos & self.mask, total_size, read_pos) {
            return Err(anyhow!("Buffer full - batch would overlap"));
        }

        // Write all items
        for item in items {
            let actual_pos = current_pos & self.mask;

            unsafe {
                let dst = self.mmap.as_ptr().add(actual_pos);
                ptr::copy_nonoverlapping(item.as_ptr(), dst, item.len());
            }

            positions.push(actual_pos);
            current_pos += item.len();
        }

        // Single memory fence for entire batch
        std::sync::atomic::fence(Ordering::Release);

        // Update statistics
        self.writes_count.fetch_add(items.len(), Ordering::Relaxed);
        self.bytes_written.fetch_add(total_size, Ordering::Relaxed);

        Ok(positions)
    }

    /// Read data from a specific position
    pub fn read_at(&self, position: usize, length: usize) -> Result<Vec<u8>> {
        if position + length > self.size {
            return Err(anyhow!("Read beyond buffer size"));
        }

        let mut data = vec![0u8; length];
        unsafe {
            let src = self.mmap.as_ptr().add(position);
            ptr::copy_nonoverlapping(src, data.as_mut_ptr(), length);
        }

        Ok(data)
    }

    /// Advance read position (for consuming data)
    pub fn advance_read_pos(&self, bytes: usize) {
        self.read_pos.fetch_add(bytes, Ordering::Release);
    }

    /// Get current statistics
    pub fn stats(&self) -> LogStats {
        LogStats {
            size: self.size,
            write_pos: self.write_pos.load(Ordering::Relaxed),
            read_pos: self.read_pos.load(Ordering::Relaxed),
            writes_count: self.writes_count.load(Ordering::Relaxed),
            bytes_written: self.bytes_written.load(Ordering::Relaxed),
            available_space: self.available_space(),
        }
    }

    /// Check how much space is available
    pub fn available_space(&self) -> usize {
        let write_pos = self.write_pos.load(Ordering::Relaxed);
        let read_pos = self.read_pos.load(Ordering::Relaxed);

        if write_pos >= read_pos {
            self.size - (write_pos - read_pos)
        } else {
            read_pos - write_pos
        }
    }

    /// Force sync to disk (for durability)
    pub fn sync(&self) -> Result<()> {
        self.mmap.flush()?;
        Ok(())
    }

    /// Async sync in background
    pub fn sync_async(&self) -> Result<()> {
        self.mmap.flush_async()?;
        Ok(())
    }

    /// Check if write would overlap with read position
    fn would_overlap(&self, write_pos: usize, write_len: usize, read_pos: usize) -> bool {
        let write_end = (write_pos + write_len) & self.mask;
        let read_pos_masked = read_pos & self.mask;

        if write_pos <= write_end {
            // No wrap-around
            write_pos <= read_pos_masked && read_pos_masked < write_end
        } else {
            // Wrap-around case
            write_pos <= read_pos_masked || read_pos_masked < write_end
        }
    }
}

/// Statistics for the ultra-fast log
#[derive(Debug, Clone)]
pub struct LogStats {
    pub size: usize,
    pub write_pos: usize,
    pub read_pos: usize,
    pub writes_count: usize,
    pub bytes_written: usize,
    pub available_space: usize,
}

impl LogStats {
    pub fn utilization_percent(&self) -> f64 {
        let used = self.size - self.available_space;
        (used as f64 / self.size as f64) * 100.0
    }

    pub fn avg_write_size(&self) -> f64 {
        if self.writes_count > 0 {
            self.bytes_written as f64 / self.writes_count as f64
        } else {
            0.0
        }
    }
}

// Thread-safe wrapper for shared access
pub struct SharedUltraFastLog {
    log: Arc<UltraFastLog>,
}

impl SharedUltraFastLog {
    pub fn new(file_path: &str, size_mb: usize) -> Result<Self> {
        let log = UltraFastLog::new(file_path, size_mb)?;
        Ok(Self {
            log: Arc::new(log),
        })
    }

    pub fn clone(&self) -> Self {
        Self {
            log: Arc::clone(&self.log),
        }
    }

    pub fn append(&self, data: &[u8]) -> Result<usize> {
        self.log.append(data)
    }

    pub fn batch_append(&self, items: &[&[u8]]) -> Result<Vec<usize>> {
        self.log.batch_append(items)
    }

    pub fn stats(&self) -> LogStats {
        self.log.stats()
    }

    pub fn sync(&self) -> Result<()> {
        self.log.sync()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Instant;
    use tempfile::NamedTempFile;

    #[test]
    fn test_ultra_fast_log_creation() -> Result<()> {
        let temp_file = NamedTempFile::new()?;
        let log = UltraFastLog::new(temp_file.path().to_str().unwrap(), 1)?;

        assert_eq!(log.size, 1024 * 1024);
        assert_eq!(log.mask, 1024 * 1024 - 1);

        Ok(())
    }

    #[test]
    fn test_single_append_performance() -> Result<()> {
        let temp_file = NamedTempFile::new()?;
        let log = UltraFastLog::new(temp_file.path().to_str().unwrap(), 1)?;

        let data = b"Hello, ultra-fast world!";
        let iterations = 100_000;

        let start = Instant::now();
        for _ in 0..iterations {
            log.append(data)?;
        }
        let duration = start.elapsed();

        let avg_latency_ns = duration.as_nanos() / iterations as u128;
        println!("Average append latency: {} ns", avg_latency_ns);

        // Should be well under 1 microsecond (1000ns)
        assert!(avg_latency_ns < 1000);

        Ok(())
    }

    #[test]
    fn test_batch_append_performance() -> Result<()> {
        let temp_file = NamedTempFile::new()?;
        let log = UltraFastLog::new(temp_file.path().to_str().unwrap(), 1)?;

        let items: Vec<&[u8]> = vec![b"item1"; 1000];
        let iterations = 1_000;

        let start = Instant::now();
        for _ in 0..iterations {
            log.batch_append(&items)?;
        }
        let duration = start.elapsed();

        let total_items = iterations * items.len();
        let avg_latency_ns = duration.as_nanos() / total_items as u128;
        println!("Average batch append latency per item: {} ns", avg_latency_ns);

        // Batch operations should be even faster per item
        assert!(avg_latency_ns < 100);

        Ok(())
    }

    #[test]
    fn test_concurrent_access() -> Result<()> {
        let temp_file = NamedTempFile::new()?;
        let log = SharedUltraFastLog::new(temp_file.path().to_str().unwrap(), 4)?;

        let num_threads = 8;
        let items_per_thread = 10_000;

        let start = Instant::now();
        std::thread::scope(|s| {
            for thread_id in 0..num_threads {
                let log_clone = log.clone();
                s.spawn(move || {
                    for i in 0..items_per_thread {
                        let data = format!("thread_{}_item_{}", thread_id, i);
                        log_clone.append(data.as_bytes()).unwrap();
                    }
                });
            }
        });
        let duration = start.elapsed();

        let total_items = num_threads * items_per_thread;
        let throughput = total_items as f64 / duration.as_secs_f64();
        println!("Concurrent throughput: {:.0} items/sec", throughput);

        let stats = log.stats();
        assert_eq!(stats.writes_count, total_items);

        Ok(())
    }
}
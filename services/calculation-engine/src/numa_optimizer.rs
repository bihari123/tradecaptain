use std::collections::HashMap;
use std::fs;
use std::thread;
use std::sync::Arc;
use anyhow::{Result, anyhow};

/// NUMA topology information
#[derive(Debug, Clone)]
pub struct NumaTopology {
    pub nodes: Vec<NumaNode>,
    pub total_nodes: usize,
    pub current_node: usize,
}

/// NUMA node information
#[derive(Debug, Clone)]
pub struct NumaNode {
    pub id: usize,
    pub cpus: Vec<usize>,
    pub memory_gb: f64,
    pub local_latency_ns: u64,
    pub remote_latency_ns: HashMap<usize, u64>,
}

/// NUMA-aware thread scheduler for financial calculations
pub struct NumaScheduler {
    topology: NumaTopology,
    thread_assignments: HashMap<String, usize>, // Service -> NUMA node
}

impl NumaScheduler {
    /// Create a new NUMA scheduler
    pub fn new() -> Result<Self> {
        let topology = Self::detect_numa_topology()?;

        let mut scheduler = Self {
            topology,
            thread_assignments: HashMap::new(),
        };

        // Set up optimal thread assignments for trading workloads
        scheduler.setup_trading_assignments();

        Ok(scheduler)
    }

    /// Detect NUMA topology from /sys/devices/system/node/
    fn detect_numa_topology() -> Result<NumaTopology> {
        let node_paths = fs::read_dir("/sys/devices/system/node/")?
            .filter_map(|entry| entry.ok())
            .filter(|entry| {
                entry.file_name().to_string_lossy().starts_with("node")
            })
            .collect::<Vec<_>>();

        if node_paths.is_empty() {
            return Err(anyhow!("No NUMA nodes detected"));
        }

        let mut nodes = Vec::new();

        for node_path in node_paths {
            let node_name = node_path.file_name();
            let node_id_str = node_name.to_string_lossy()
                .strip_prefix("node")
                .ok_or_else(|| anyhow!("Invalid node name format"))?;

            let node_id: usize = node_id_str.parse()?;

            // Read CPU list for this node
            let cpulist_path = node_path.path().join("cpulist");
            let cpulist = fs::read_to_string(cpulist_path)?;
            let cpus = Self::parse_cpu_list(&cpulist.trim())?;

            // Read memory information
            let meminfo_path = node_path.path().join("meminfo");
            let memory_gb = if meminfo_path.exists() {
                Self::parse_memory_info(&meminfo_path)?
            } else {
                0.0
            };

            let node = NumaNode {
                id: node_id,
                cpus,
                memory_gb,
                local_latency_ns: 100,  // Typical local latency
                remote_latency_ns: HashMap::new(),
            };

            nodes.push(node);
        }

        // Sort nodes by ID
        nodes.sort_by_key(|n| n.id);

        let current_node = Self::get_current_numa_node()?;

        Ok(NumaTopology {
            total_nodes: nodes.len(),
            nodes,
            current_node,
        })
    }

    /// Parse CPU list string (e.g., "0-3,8-11")
    fn parse_cpu_list(cpulist: &str) -> Result<Vec<usize>> {
        let mut cpus = Vec::new();

        for range in cpulist.split(',') {
            if range.contains('-') {
                let parts: Vec<&str> = range.split('-').collect();
                if parts.len() != 2 {
                    return Err(anyhow!("Invalid CPU range: {}", range));
                }
                let start: usize = parts[0].parse()?;
                let end: usize = parts[1].parse()?;
                for cpu in start..=end {
                    cpus.push(cpu);
                }
            } else {
                cpus.push(range.parse()?);
            }
        }

        Ok(cpus)
    }

    /// Parse memory information from node meminfo
    fn parse_memory_info(meminfo_path: &std::path::Path) -> Result<f64> {
        let content = fs::read_to_string(meminfo_path)?;

        for line in content.lines() {
            if line.starts_with("Node") && line.contains("MemTotal:") {
                let parts: Vec<&str> = line.split_whitespace().collect();
                if parts.len() >= 4 {
                    let kb: u64 = parts[3].parse()?;
                    return Ok(kb as f64 / 1024.0 / 1024.0); // Convert KB to GB
                }
            }
        }

        Ok(0.0)
    }

    /// Get current NUMA node of the calling thread
    fn get_current_numa_node() -> Result<usize> {
        // Try to read from /proc/self/numa_maps or use getcpu syscall
        match Self::get_numa_node_via_getcpu() {
            Ok(node) => Ok(node),
            Err(_) => {
                // Fallback: assume node 0
                Ok(0)
            }
        }
    }

    /// Get NUMA node using getcpu syscall
    fn get_numa_node_via_getcpu() -> Result<usize> {
        #[cfg(target_os = "linux")]
        {
            let mut cpu: u32 = 0;
            let mut node: u32 = 0;

            unsafe {
                let ret = libc::syscall(
                    libc::SYS_getcpu,
                    &mut cpu as *mut u32,
                    &mut node as *mut u32,
                    std::ptr::null_mut::<libc::c_void>()
                );

                if ret == 0 {
                    Ok(node as usize)
                } else {
                    Err(anyhow!("getcpu syscall failed"))
                }
            }
        }
        #[cfg(not(target_os = "linux"))]
        {
            Err(anyhow!("NUMA detection not supported on this platform"))
        }
    }

    /// Set up optimal thread assignments for trading workloads
    fn setup_trading_assignments(&mut self) {
        if self.topology.total_nodes < 2 {
            // Single NUMA node - all services can run anywhere
            return;
        }

        // Assign latency-critical services to separate NUMA nodes
        match self.topology.total_nodes {
            2 => {
                // Dual-socket system
                self.thread_assignments.insert("market_data_ingestion".to_string(), 0);
                self.thread_assignments.insert("order_processing".to_string(), 0);
                self.thread_assignments.insert("risk_calculation".to_string(), 1);
                self.thread_assignments.insert("portfolio_calculation".to_string(), 1);
            }
            4 => {
                // Quad-socket system (rare but possible)
                self.thread_assignments.insert("market_data_ingestion".to_string(), 0);
                self.thread_assignments.insert("order_processing".to_string(), 1);
                self.thread_assignments.insert("risk_calculation".to_string(), 2);
                self.thread_assignments.insert("portfolio_calculation".to_string(), 3);
            }
            _ => {
                // More than 4 nodes - distribute evenly
                let services = vec![
                    "market_data_ingestion",
                    "order_processing",
                    "risk_calculation",
                    "portfolio_calculation",
                    "technical_analysis",
                    "news_processing"
                ];

                for (i, service) in services.iter().enumerate() {
                    let node = i % self.topology.total_nodes;
                    self.thread_assignments.insert(service.to_string(), node);
                }
            }
        }
    }

    /// Bind current thread to specific NUMA node
    pub fn bind_to_node(&self, node_id: usize) -> Result<()> {
        if node_id >= self.topology.total_nodes {
            return Err(anyhow!("Invalid NUMA node: {}", node_id));
        }

        #[cfg(target_os = "linux")]
        {
            self.set_numa_policy(node_id)?;
            self.set_cpu_affinity(node_id)?;
        }

        Ok(())
    }

    /// Set NUMA memory policy for current thread
    #[cfg(target_os = "linux")]
    fn set_numa_policy(&self, node_id: usize) -> Result<()> {
        use std::mem;

        let node_mask: u64 = 1 << node_id;
        let mask_ptr = &node_mask as *const u64 as *const libc::c_ulong;

        unsafe {
            let ret = libc::syscall(
                libc::SYS_set_mempolicy,
                libc::MPOL_BIND,           // Bind to specific nodes
                mask_ptr,                  // Node mask
                mem::size_of::<u64>() * 8  // Number of bits
            );

            if ret != 0 {
                return Err(anyhow!("Failed to set NUMA memory policy"));
            }
        }

        Ok(())
    }

    /// Set CPU affinity for current thread
    #[cfg(target_os = "linux")]
    fn set_cpu_affinity(&self, node_id: usize) -> Result<()> {
        let node = &self.topology.nodes[node_id];

        if node.cpus.is_empty() {
            return Err(anyhow!("No CPUs available on node {}", node_id));
        }

        // Create CPU set with all CPUs from the NUMA node
        let mut cpu_set: libc::cpu_set_t = unsafe { mem::zeroed() };

        for &cpu in &node.cpus {
            unsafe {
                libc::CPU_SET(cpu, &mut cpu_set);
            }
        }

        unsafe {
            let ret = libc::sched_setaffinity(
                0, // Current thread
                mem::size_of::<libc::cpu_set_t>(),
                &cpu_set
            );

            if ret != 0 {
                return Err(anyhow!("Failed to set CPU affinity"));
            }
        }

        Ok(())
    }

    /// Spawn thread on specific NUMA node
    pub fn spawn_on_node<F, T>(&self, node_id: usize, name: &str, f: F) -> Result<thread::JoinHandle<T>>
    where
        F: FnOnce() -> T + Send + 'static,
        T: Send + 'static,
    {
        let scheduler = self.clone();
        let thread_name = name.to_string();

        let handle = thread::Builder::new()
            .name(thread_name.clone())
            .spawn(move || {
                // Bind to NUMA node
                if let Err(e) = scheduler.bind_to_node(node_id) {
                    eprintln!("Failed to bind thread {} to NUMA node {}: {}", thread_name, node_id, e);
                }

                // Execute the function
                f()
            })?;

        Ok(handle)
    }

    /// Get optimal NUMA node for a service
    pub fn get_node_for_service(&self, service: &str) -> usize {
        self.thread_assignments.get(service).copied()
            .unwrap_or(self.topology.current_node)
    }

    /// Get topology information
    pub fn get_topology(&self) -> &NumaTopology {
        &self.topology
    }

    /// Print NUMA topology information
    pub fn print_topology(&self) {
        println!("NUMA Topology:");
        println!("  Total nodes: {}", self.topology.total_nodes);
        println!("  Current node: {}", self.topology.current_node);

        for node in &self.topology.nodes {
            println!("  Node {}:", node.id);
            println!("    CPUs: {:?}", node.cpus);
            println!("    Memory: {:.1} GB", node.memory_gb);
            println!("    Local latency: {} ns", node.local_latency_ns);
        }

        println!("\nService assignments:");
        for (service, node) in &self.thread_assignments {
            println!("  {}: Node {}", service, node);
        }
    }
}

impl Clone for NumaScheduler {
    fn clone(&self) -> Self {
        Self {
            topology: self.topology.clone(),
            thread_assignments: self.thread_assignments.clone(),
        }
    }
}

/// NUMA-aware memory allocator using huge pages
pub struct NumaAllocator {
    node_id: usize,
    hugepage_mount: String,
}

impl NumaAllocator {
    /// Create allocator for specific NUMA node
    pub fn new(node_id: usize) -> Self {
        Self {
            node_id,
            hugepage_mount: format!("/mnt/hugepages/node{}", node_id),
        }
    }

    /// Allocate memory on specific NUMA node with huge pages
    pub fn allocate_huge_pages(&self, size_mb: usize) -> Result<*mut u8> {
        #[cfg(target_os = "linux")]
        {
            use std::ffi::CString;
            use std::os::raw::c_int;

            let size = size_mb * 1024 * 1024;

            // Try to allocate using mmap with huge pages
            let addr = unsafe {
                libc::mmap(
                    std::ptr::null_mut(),
                    size,
                    libc::PROT_READ | libc::PROT_WRITE,
                    libc::MAP_PRIVATE | libc::MAP_ANONYMOUS | libc::MAP_HUGETLB,
                    -1,
                    0
                )
            };

            if addr == libc::MAP_FAILED {
                return Err(anyhow!("Failed to allocate huge pages"));
            }

            // Bind memory to NUMA node
            let node_mask: u64 = 1 << self.node_id;
            unsafe {
                libc::syscall(
                    libc::SYS_mbind,
                    addr,
                    size,
                    libc::MPOL_BIND,
                    &node_mask as *const u64,
                    64, // Number of bits
                    0   // Flags
                );
            }

            Ok(addr as *mut u8)
        }
        #[cfg(not(target_os = "linux"))]
        {
            Err(anyhow!("NUMA allocation not supported on this platform"))
        }
    }

    /// Deallocate huge page memory
    pub fn deallocate(&self, ptr: *mut u8, size_mb: usize) -> Result<()> {
        #[cfg(target_os = "linux")]
        {
            let size = size_mb * 1024 * 1024;
            unsafe {
                let ret = libc::munmap(ptr as *mut libc::c_void, size);
                if ret != 0 {
                    return Err(anyhow!("Failed to deallocate memory"));
                }
            }
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_numa_topology_detection() {
        match NumaScheduler::new() {
            Ok(scheduler) => {
                scheduler.print_topology();
                assert!(scheduler.get_topology().total_nodes > 0);
            }
            Err(e) => {
                println!("NUMA not available or detection failed: {}", e);
                // This is okay - not all systems have NUMA
            }
        }
    }

    #[test]
    fn test_cpu_list_parsing() {
        let cpus = NumaScheduler::parse_cpu_list("0-3,8-11").unwrap();
        assert_eq!(cpus, vec![0, 1, 2, 3, 8, 9, 10, 11]);

        let cpus = NumaScheduler::parse_cpu_list("0,2,4,6").unwrap();
        assert_eq!(cpus, vec![0, 2, 4, 6]);
    }
}
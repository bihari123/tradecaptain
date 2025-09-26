#!/bin/bash

# Setup Huge Pages for TradeCaptain Performance Optimization
# Author: Tarun Thakur (thakur[dot]cs[dot]tarun[at]gmail[dot]com)
# This script configures huge pages to reduce TLB misses and improve memory performance

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Get system memory information
get_memory_info() {
    log "Getting system memory information..."

    TOTAL_MEM_KB=$(grep MemTotal /proc/meminfo | awk '{print $2}')
    TOTAL_MEM_GB=$((TOTAL_MEM_KB / 1024 / 1024))

    log "Total system memory: ${TOTAL_MEM_GB} GB"

    # Check current huge page configuration
    HUGEPAGE_SIZE=$(grep Hugepagesize /proc/meminfo | awk '{print $2}')
    HUGEPAGES_TOTAL=$(grep HugePages_Total /proc/meminfo | awk '{print $2}')
    HUGEPAGES_FREE=$(grep HugePages_Free /proc/meminfo | awk '{print $2}')

    log "Current huge page size: ${HUGEPAGE_SIZE} KB"
    log "Current huge pages total: ${HUGEPAGES_TOTAL}"
    log "Current huge pages free: ${HUGEPAGES_FREE}"
}

# Calculate optimal huge page configuration
calculate_hugepage_config() {
    log "Calculating optimal huge page configuration..."

    # Reserve about 25% of memory for huge pages for financial applications
    # This leaves enough memory for the OS and other applications
    HUGEPAGE_MEMORY_GB=$((TOTAL_MEM_GB / 4))

    if [ $HUGEPAGE_MEMORY_GB -lt 1 ]; then
        HUGEPAGE_MEMORY_GB=1
    fi

    # Calculate number of 2MB huge pages needed
    HUGEPAGES_NEEDED=$((HUGEPAGE_MEMORY_GB * 1024 / 2))

    log "Recommended huge pages: ${HUGEPAGES_NEEDED} (${HUGEPAGE_MEMORY_GB} GB)"
}

# Configure huge pages
configure_hugepages() {
    log "Configuring huge pages..."

    # Set the number of huge pages
    echo $HUGEPAGES_NEEDED > /proc/sys/vm/nr_hugepages

    # Verify configuration
    sleep 1
    ACTUAL_HUGEPAGES=$(grep HugePages_Total /proc/meminfo | awk '{print $2}')

    if [ "$ACTUAL_HUGEPAGES" -eq "$HUGEPAGES_NEEDED" ]; then
        success "Successfully configured ${ACTUAL_HUGEPAGES} huge pages"
    else
        warning "Requested ${HUGEPAGES_NEEDED} huge pages, but only ${ACTUAL_HUGEPAGES} were allocated"
        warning "This may be due to memory fragmentation. Consider rebooting and running this script earlier."
    fi
}

# Make huge page configuration persistent
make_persistent() {
    log "Making huge page configuration persistent..."

    # Add to sysctl.conf if not already present
    SYSCTL_CONF="/etc/sysctl.conf"
    HUGEPAGE_LINE="vm.nr_hugepages = $HUGEPAGES_NEEDED"

    if ! grep -q "vm.nr_hugepages" $SYSCTL_CONF; then
        echo "# Huge pages for TradeCaptain performance" >> $SYSCTL_CONF
        echo "$HUGEPAGE_LINE" >> $SYSCTL_CONF
        success "Added huge page configuration to $SYSCTL_CONF"
    else
        # Update existing configuration
        sed -i "s/vm.nr_hugepages.*/vm.nr_hugepages = $HUGEPAGES_NEEDED/" $SYSCTL_CONF
        success "Updated huge page configuration in $SYSCTL_CONF"
    fi
}

# Configure huge page mount point
setup_hugepage_mount() {
    log "Setting up huge page mount point..."

    HUGEPAGE_MOUNT="/mnt/hugepages"

    # Create mount point if it doesn't exist
    if [ ! -d "$HUGEPAGE_MOUNT" ]; then
        mkdir -p $HUGEPAGE_MOUNT
        success "Created huge page mount point: $HUGEPAGE_MOUNT"
    fi

    # Check if already mounted
    if ! mount | grep -q "hugetlbfs"; then
        # Mount huge page filesystem
        mount -t hugetlbfs nodev $HUGEPAGE_MOUNT
        success "Mounted huge page filesystem at $HUGEPAGE_MOUNT"

        # Add to fstab for persistence
        FSTAB_ENTRY="nodev $HUGEPAGE_MOUNT hugetlbfs defaults 0 0"
        if ! grep -q "$HUGEPAGE_MOUNT" /etc/fstab; then
            echo "$FSTAB_ENTRY" >> /etc/fstab
            success "Added huge page mount to /etc/fstab"
        fi
    else
        log "Huge page filesystem already mounted"
    fi

    # Set appropriate permissions
    chown -R tradecaptain:tradecaptain $HUGEPAGE_MOUNT 2>/dev/null || true
    chmod 755 $HUGEPAGE_MOUNT
}

# Configure transparent huge pages (disable for consistent latency)
configure_thp() {
    log "Configuring Transparent Huge Pages (THP)..."

    # Disable THP for consistent latency in financial applications
    # THP can cause unpredictable memory allocation delays

    THP_ENABLED="/sys/kernel/mm/transparent_hugepage/enabled"
    THP_DEFRAG="/sys/kernel/mm/transparent_hugepage/defrag"

    if [ -f "$THP_ENABLED" ]; then
        echo never > $THP_ENABLED
        success "Disabled Transparent Huge Pages"
    fi

    if [ -f "$THP_DEFRAG" ]; then
        echo never > $THP_DEFRAG
        success "Disabled THP defragmentation"
    fi

    # Make THP settings persistent
    GRUB_CONFIG="/etc/default/grub"
    if [ -f "$GRUB_CONFIG" ]; then
        if ! grep -q "transparent_hugepage=never" $GRUB_CONFIG; then
            sed -i 's/GRUB_CMDLINE_LINUX="\(.*\)"/GRUB_CMDLINE_LINUX="\1 transparent_hugepage=never"/' $GRUB_CONFIG
            warning "Updated GRUB configuration. Run 'update-grub' and reboot to apply THP settings permanently."
        fi
    fi
}

# Optimize other memory settings for financial applications
optimize_memory_settings() {
    log "Optimizing memory settings for financial applications..."

    # Disable swap to prevent unpredictable latencies
    swapoff -a 2>/dev/null || true

    # Update sysctl settings for low-latency applications
    cat >> /etc/sysctl.conf << EOF

# TradeCaptain memory optimizations
vm.swappiness = 1
vm.dirty_ratio = 5
vm.dirty_background_ratio = 2
vm.dirty_expire_centisecs = 500
vm.dirty_writeback_centisecs = 100
vm.overcommit_memory = 1
vm.zone_reclaim_mode = 0

# Network optimizations
net.core.rmem_default = 262144
net.core.rmem_max = 16777216
net.core.wmem_default = 262144
net.core.wmem_max = 16777216
net.core.netdev_max_backlog = 5000
net.ipv4.tcp_rmem = 4096 65536 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216
net.ipv4.tcp_congestion_control = bbr

EOF

    success "Applied memory and network optimizations"
}

# Verify huge page configuration
verify_configuration() {
    log "Verifying huge page configuration..."

    echo "=== Memory Information ==="
    cat /proc/meminfo | grep -E "(MemTotal|Hugepagesize|HugePages)"

    echo -e "\n=== Huge Page Mount ==="
    mount | grep hugetlbfs || echo "No huge page mounts found"

    echo -e "\n=== THP Status ==="
    cat /sys/kernel/mm/transparent_hugepage/enabled 2>/dev/null || echo "THP not available"

    echo -e "\n=== Swap Status ==="
    swapon --show || echo "No swap enabled"

    success "Configuration verification completed"
}

# Generate application configuration
generate_app_config() {
    log "Generating application configuration..."

    APP_CONFIG="/home/tarun/Desktop/project/TradeCaptain/.env.hugepages"

    cat > $APP_CONFIG << EOF
# Huge Pages Configuration for TradeCaptain
# Generated on $(date)

# Huge page settings
HUGEPAGE_MOUNT=/mnt/hugepages
HUGEPAGE_SIZE_KB=$HUGEPAGE_SIZE
HUGEPAGES_AVAILABLE=$ACTUAL_HUGEPAGES

# Memory optimization flags
USE_HUGEPAGES=true
HUGEPAGE_POOL_SIZE_MB=$((HUGEPAGE_MEMORY_GB * 1024))

# Application hints
# Use mmap with MAP_HUGETLB flag for large allocations
# Use posix_madvise(MADV_HUGEPAGE) for large buffers
# Allocate order books and price arrays in huge page memory

EOF

    chown tradecaptain:tradecaptain $APP_CONFIG 2>/dev/null || true
    success "Generated application configuration: $APP_CONFIG"
}

# Usage information
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo "  -v, --verify   Only verify current configuration"
    echo "  -r, --reset    Reset huge page configuration"
    echo ""
    echo "This script configures huge pages for optimal TradeCaptain performance."
    echo "It should be run as root (with sudo)."
}

# Reset huge page configuration
reset_hugepages() {
    log "Resetting huge page configuration..."

    echo 0 > /proc/sys/vm/nr_hugepages

    # Remove from sysctl.conf
    sed -i '/vm.nr_hugepages/d' /etc/sysctl.conf 2>/dev/null || true
    sed -i '/TradeCaptain memory optimizations/,+15d' /etc/sysctl.conf 2>/dev/null || true

    # Unmount huge pages
    umount /mnt/hugepages 2>/dev/null || true

    success "Reset huge page configuration"
}

# Main execution
main() {
    case "${1:-setup}" in
        "verify"|"-v"|"--verify")
            get_memory_info
            verify_configuration
            ;;
        "reset"|"-r"|"--reset")
            check_root
            reset_hugepages
            ;;
        "help"|"-h"|"--help")
            usage
            ;;
        "setup"|"")
            log "Starting TradeCaptain huge page setup..."
            check_root
            get_memory_info
            calculate_hugepage_config
            configure_hugepages
            make_persistent
            setup_hugepage_mount
            configure_thp
            optimize_memory_settings
            verify_configuration
            generate_app_config

            success "Huge page setup completed successfully!"
            warning "For optimal performance, consider rebooting the system."
            log "Application configuration saved to .env.hugepages"
            ;;
        *)
            error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
}

# Execute main function
main "$@"
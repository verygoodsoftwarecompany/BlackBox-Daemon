# System Telemetry Component

## Overview

The System Telemetry component provides comprehensive monitoring of Linux system resources by reading directly from the `/proc` and `/sys` filesystems. It collects detailed metrics about CPU usage, memory consumption, network I/O, disk operations, and process statistics at configurable intervals.

## Architecture

### Data Collection Strategy
- **Direct Filesystem Access**: Reads from `/proc` and `/sys` for maximum efficiency
- **Periodic Collection**: Configurable interval (default: 1 second)  
- **Comprehensive Coverage**: All major system resource categories
- **Low Overhead**: Minimal CPU and memory impact
- **Thread-Safe**: Concurrent collection with proper synchronization

### Collection Categories
1. **CPU Metrics**: Per-core usage, load averages, context switches
2. **Memory Metrics**: Usage, allocation, swap, buffers, cache
3. **Network Metrics**: Interface I/O, packet counts, error rates
4. **Disk Metrics**: I/O operations, throughput, queue depth
5. **Process Metrics**: Count, file descriptors, thread statistics

## System Metrics Collected

### CPU Metrics
**Source**: `/proc/stat`

**Metrics Collected**:
```
cpu_usage_percent{core="cpu0"}     # Per-core CPU utilization (0-100%)
cpu_usage_percent{core="cpu1"}     # Individual core breakdown
cpu_usage_percent{core="cpu"}      # Overall CPU usage
```

**Calculation**:
```go
// Parse /proc/stat fields: user, nice, system, idle, iowait, irq, softirq
total := user + nice + system + idle + iowait + irq + softirq
usage := float64(total-idle) / float64(total) * 100
```

**Tags**: `core` (cpu0, cpu1, cpu, etc.)

### Memory Metrics  
**Source**: `/proc/meminfo`

**Metrics Collected**:
```
memory_total_bytes                 # Total system memory
memory_free_bytes                  # Free memory
memory_available_bytes             # Available memory (includes cache)
memory_buffers_bytes               # Buffer cache
memory_cached_bytes                # Page cache
memory_usage_percent               # Calculated usage percentage
swap_total_bytes                   # Total swap space
swap_free_bytes                    # Free swap space
```

**Calculation**:
```go
// Memory usage calculation
used := total - available
usagePercent := float64(used) / float64(total) * 100
```

### Network Metrics
**Source**: `/proc/net/dev`

**Metrics Collected**:
```
network_rx_bytes{interface="eth0"}      # Received bytes
network_rx_packets{interface="eth0"}    # Received packets  
network_rx_errors{interface="eth0"}     # Receive errors
network_tx_bytes{interface="eth0"}      # Transmitted bytes
network_tx_packets{interface="eth0"}    # Transmitted packets
network_tx_errors{interface="eth0"}     # Transmit errors
```

**Interface Filtering**: Excludes loopback (`lo`) interface  
**Tags**: `interface` (eth0, wlan0, etc.)

### Disk Metrics
**Source**: `/proc/diskstats`

**Metrics Collected**:
```
disk_read_ios{device="sda"}        # Read I/O operations
disk_read_bytes{device="sda"}      # Read bytes (sectors × 512)
disk_write_ios{device="sda"}       # Write I/O operations  
disk_write_bytes{device="sda"}     # Write bytes (sectors × 512)
```

**Device Filtering**: Only physical devices (`sd*`, `nvme*`)  
**Tags**: `device` (sda, nvme0n1, etc.)

### Process Metrics
**Sources**: `/proc/sys/fs/file-nr`, `/proc/*/`

**Metrics Collected**:
```
processes_total                    # Total process count
open_files_total                   # System-wide open file descriptors
```

**Process Counting**: Counts numeric directories in `/proc` (PIDs)

### Load Average Metrics
**Source**: `/proc/loadavg`

**Metrics Collected**:
```
load_1min                          # 1-minute load average
load_5min                          # 5-minute load average  
load_15min                         # 15-minute load average
```

## Implementation Details

### SystemCollector Structure
```go
type SystemCollector struct {
    mutex    sync.RWMutex           // Protects collector state
    interval time.Duration          // Collection frequency
    buffer   TelemetryBuffer        // Ring buffer for storage
}
```

### Collection Lifecycle
```go
func (sc *SystemCollector) Start(ctx context.Context) error {
    ticker := time.NewTicker(sc.interval)
    defer ticker.Stop()
    
    // Initial collection
    if err := sc.collectMetrics(); err != nil {
        return fmt.Errorf("failed initial collection: %w", err)
    }
    
    // Periodic collection loop
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := sc.collectMetrics(); err != nil {
                // Log error but continue collecting
                log.Printf("Collection error: %v", err)
            }
        }
    }
}
```

### Error Handling Strategy
- **Graceful Degradation**: Failed metrics don't stop other collection
- **Logging**: Errors logged but don't terminate collector
- **Retry Logic**: Transient filesystem errors are retried
- **Fallback Values**: Missing values default to zero or skip

### File System Access Patterns
```go
// CPU metrics from /proc/stat
func (sc *SystemCollector) collectCPUMetrics(timestamp time.Time) error {
    data, err := ioutil.ReadFile("/proc/stat")
    if err != nil {
        return err
    }
    
    // Parse lines starting with "cpu"
    lines := strings.Split(string(data), "\n")
    for _, line := range lines {
        if strings.HasPrefix(line, "cpu") {
            // Parse and calculate usage
        }
    }
}
```

## Configuration

### Environment Variables
```bash
BLACKBOX_COLLECTION_INTERVAL=1s        # Collection frequency
BLACKBOX_BUFFER_WINDOW_SIZE=60s        # How long to retain data
BLACKBOX_LOG_LEVEL=info                # Logging level for collection
```

### Container Requirements
```yaml
# Required volume mounts for system access
volumeMounts:
- name: proc
  mountPath: /proc
  readOnly: true
- name: sys  
  mountPath: /sys
  readOnly: true

volumes:
- name: proc
  hostPath:
    path: /proc
- name: sys
  hostPath:
    path: /sys
```

### Security Context
```yaml
securityContext:
  privileged: false              # Not required
  readOnlyRootFilesystem: true   # Recommended
  runAsNonRoot: true            # Recommended
  capabilities:
    drop:
    - ALL
```

## Performance Characteristics

### Resource Usage
- **CPU Impact**: <1% of single CPU core
- **Memory Usage**: ~10-20MB for data structures
- **Disk I/O**: ~1KB/second reading /proc files
- **Network**: Zero network overhead

### Collection Overhead
```go
// Typical collection times (on standard hardware)
CPU Collection:     ~2-5ms
Memory Collection:  ~1-2ms  
Network Collection: ~3-8ms
Disk Collection:    ~5-10ms
Process Collection: ~10-20ms
Total Per Cycle:    ~20-50ms
```

### Scalability Considerations
- **Collection Interval**: Balance between granularity and overhead
- **Buffer Size**: Larger windows require more memory
- **Metric Filtering**: Option to disable expensive collections
- **Batch Processing**: Group filesystem reads when possible

## Data Flow Integration

### Ring Buffer Integration
```go
// Each metric becomes a telemetry entry
sc.buffer.Add(types.TelemetryEntry{
    Timestamp: timestamp,
    Source:    types.SourceSystem,
    Type:      types.TypeCPU,
    Name:      "cpu0_usage_percent",
    Value:     cpuUsage,
    Tags: map[string]string{
        "core": "cpu0",
    },
})
```

### Prometheus Metrics Integration
```go
// Metrics are also exported to Prometheus
metricsCollector.RecordCPUUsage("cpu0", cpuUsage)
metricsCollector.RecordMemoryUsage("total", totalMemory)
metricsCollector.RecordNetworkBytes("eth0", "rx", rxBytes)
```

### Incident Context
When incidents occur, system telemetry provides:
- **Resource Pressure**: CPU/memory usage before crash
- **I/O Patterns**: Disk/network activity leading to incident  
- **System Load**: Overall system health indicators
- **Process Activity**: Process count and file descriptor usage

## Platform Compatibility

### Linux Distributions
- ✅ **Ubuntu/Debian**: Full support for all metrics
- ✅ **CentOS/RHEL**: Full support for all metrics  
- ✅ **Alpine Linux**: Full support (container optimized)
- ✅ **Amazon Linux**: Full support on EC2
- ⚠️ **Other Unix**: Limited support (may need adaptation)

### Container Runtimes
- ✅ **Docker**: Full support with proper volume mounts
- ✅ **containerd**: Full support with proper volume mounts
- ✅ **CRI-O**: Full support with proper volume mounts
- ✅ **Podman**: Full support with proper volume mounts

### Kubernetes Environments
- ✅ **Self-managed**: Full access to host filesystem
- ✅ **EKS**: Full support with proper node configuration
- ✅ **GKE**: Full support with proper node configuration  
- ✅ **AKS**: Full support with proper node configuration
- ⚠️ **Fargate**: Limited support (no host filesystem access)

## Monitoring and Observability

### Collection Health Metrics
```promql
# Collection success rate
rate(telemetry_collection_success_total[5m])
rate(telemetry_collection_errors_total[5m])

# Collection duration  
telemetry_collection_duration_seconds{type="cpu"}
telemetry_collection_duration_seconds{type="memory"}

# Data completeness
telemetry_entries_collected_total{source="system",type="cpu"}
telemetry_entries_collected_total{source="system",type="memory"}
```

### Error Monitoring
```go
// Collection error types
var (
    ErrProcRead    = "failed to read /proc file"
    ErrSysRead     = "failed to read /sys file"  
    ErrParsing     = "failed to parse system data"
    ErrPermission  = "permission denied accessing system files"
)
```

### Health Indicators
- **Collection Latency**: Time to complete full system scan
- **Success Rate**: Percentage of successful collections
- **Data Freshness**: Age of most recent telemetry
- **Coverage**: Number of metrics successfully collected

## Troubleshooting

### Common Issues

1. **Permission Denied Errors**
   ```bash
   # Verify volume mounts
   kubectl describe pod <blackbox-pod> | grep -A 10 "Mounts:"
   
   # Check host path accessibility
   kubectl exec <blackbox-pod> -- ls -la /proc/stat
   kubectl exec <blackbox-pod> -- ls -la /sys/class/net
   ```

2. **Missing Metrics**
   ```bash
   # Check for specific file access
   kubectl exec <blackbox-pod> -- cat /proc/meminfo | head -5
   kubectl exec <blackbox-pod> -- cat /proc/net/dev | head -5
   
   # Verify collection is running
   curl http://localhost:9090/metrics | grep blackbox_cpu_usage
   ```

3. **High Collection Latency**
   ```bash
   # Monitor collection timing
   curl http://localhost:9090/metrics | grep collection_duration
   
   # Check system load
   kubectl top nodes
   kubectl top pods -n kube-system
   ```

4. **Incomplete Data**
   ```bash
   # Check logs for parsing errors
   kubectl logs -n kube-system -l app=blackbox-daemon --tail=100 | grep -i error
   
   # Verify filesystem format compatibility  
   kubectl exec <blackbox-pod> -- cat /proc/version
   ```

### Debug Commands
```bash
# Manual metric collection test
kubectl exec <blackbox-pod> -- cat /proc/stat | head -1

# Network interface verification
kubectl exec <blackbox-pod> -- cat /proc/net/dev | grep -v "lo:"

# Disk device listing  
kubectl exec <blackbox-pod> -- cat /proc/diskstats | grep -E "(sd|nvme)"

# Memory info verification
kubectl exec <blackbox-pod> -- cat /proc/meminfo | grep -E "(MemTotal|MemFree|MemAvailable)"
```

## Best Practices

### Configuration
1. **Set appropriate collection intervals** (1-5 seconds typical)
2. **Configure buffer size** based on incident response time needs
3. **Monitor collection performance** and adjust intervals if needed
4. **Use read-only mounts** for security (/proc and /sys)

### Deployment  
1. **Ensure proper volume mounts** for /proc and /sys access
2. **Set resource limits** to prevent system impact
3. **Use DaemonSet deployment** for per-node collection
4. **Configure appropriate RBAC** permissions

### Monitoring
1. **Set up alerts** for collection failures or high latency
2. **Monitor data completeness** to ensure all metrics are collected
3. **Track collection overhead** and system impact
4. **Verify metric accuracy** against system monitoring tools

### Security
1. **Use minimal required permissions** (read-only access)
2. **Avoid privileged containers** when possible
3. **Secure volume mounts** to prevent unauthorized access  
4. **Monitor for unusual collection patterns** that might indicate compromise
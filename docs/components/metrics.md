# Metrics Component

## Overview

The Metrics component provides comprehensive Prometheus metrics collection and export for BlackBox-Daemon. It exposes both system telemetry metrics and operational metrics about the daemon itself, making the system observable and monitorable in production environments.

## Architecture

### Prometheus Integration
- **Registry**: Custom Prometheus registry for metric isolation
- **HTTP Server**: Dedicated metrics endpoint for scraping
- **Metric Types**: Counters, Gauges, Histograms, and custom metrics
- **Labels**: Rich labeling for dimensional metrics

### Metric Categories

#### 1. System Telemetry Metrics
```
blackbox_cpu_usage_percent{core="cpu0"}           # CPU usage per core
blackbox_memory_usage_bytes{type="total"}         # Memory metrics by type
blackbox_network_bytes{interface="eth0",dir="rx"} # Network I/O by interface
blackbox_disk_io_bytes{device="sda",dir="read"}   # Disk I/O by device
blackbox_processes_total                           # Total process count
blackbox_open_files_total                          # Open file descriptors
blackbox_load_average{period="1min"}               # System load averages
```

#### 2. BlackBox Operational Metrics
```
blackbox_sidecar_requests_total                    # Sidecar API requests
blackbox_incidents_total{type="crash",severity="high"} # Detected incidents
blackbox_buffer_size_bytes                         # Ring buffer size
blackbox_buffer_entries_total                      # Current buffer entries
```

#### 3. Custom Metrics
Support for application-specific metrics with configurable names and labels.

## Configuration

### Server Settings
```go
// Configuration options
port := 9090              // Metrics server port
metricsPath := "/metrics"  // Prometheus scrape path
```

### Environment Variables
```bash
BLACKBOX_METRICS_PORT=9090        # Metrics server port
BLACKBOX_METRICS_PATH=/metrics    # Metrics endpoint path
BLACKBOX_METRICS_ENABLED=true     # Enable metrics collection
```

## Implementation Details

### Collector Structure
```go
type Collector struct {
    registry     *prometheus.Registry  // Metric registry
    httpServer   *http.Server         // HTTP server
    
    // System metrics
    cpuUsageGauge     *prometheus.GaugeVec
    memoryUsageGauge  *prometheus.GaugeVec
    networkBytesGauge *prometheus.GaugeVec
    diskIOGauge       *prometheus.GaugeVec
    processCountGauge prometheus.Gauge
    openFilesGauge    prometheus.Gauge
    loadAvgGauge      *prometheus.GaugeVec
    
    // Operational metrics
    sidecarRequestsCounter prometheus.Counter
    incidentCounter        *prometheus.CounterVec
    bufferSizeGauge        prometheus.Gauge
    bufferEntriesGauge     prometheus.Gauge
    
    // Custom metrics
    customMetrics map[string]prometheus.Collector
}
```

### Metric Recording Flow
1. **System Collector** → **Metrics Recorder** → **Prometheus Registry**
2. **API Server** → **Metrics Recorder** → **Prometheus Registry**  
3. **Incident Handler** → **Metrics Recorder** → **Prometheus Registry**
4. **Prometheus** → **HTTP Scrape** → **Time Series Database**

## Key Features

### Thread Safety
- **Concurrent Access**: All metrics are thread-safe for concurrent updates
- **Atomic Operations**: Counters and gauges use atomic operations
- **Lock-Free Design**: Prometheus client handles synchronization

### Performance Optimizations
- **Pre-allocated Metrics**: All metrics created at startup
- **Label Caching**: Prometheus client caches label combinations
- **Efficient Updates**: Direct metric updates without overhead
- **Batched Collection**: System metrics collected in batches

### High Availability
- **Independent Server**: Metrics server runs independently of main application
- **Graceful Shutdown**: Clean shutdown with context cancellation
- **Error Isolation**: Metric collection errors don't affect main functionality
- **Circuit Breaker**: Automatic recovery from metric server failures

## Metric Types and Usage

### System Telemetry

#### CPU Metrics
```go
// Record CPU usage for specific core
collector.RecordCPUUsage("cpu0", 75.5)
```
- **Purpose**: Track CPU utilization per core
- **Labels**: `core` (cpu0, cpu1, etc.)
- **Range**: 0-100 percentage

#### Memory Metrics
```go
// Record different memory types
collector.RecordMemoryUsage("total", 8589934592)    // 8GB
collector.RecordMemoryUsage("free", 2147483648)     // 2GB
collector.RecordMemoryUsage("available", 4294967296) // 4GB
```
- **Purpose**: Track memory consumption
- **Labels**: `type` (total, free, available, cached, buffers)
- **Unit**: Bytes

#### Network Metrics
```go
// Record network traffic
collector.RecordNetworkBytes("eth0", "rx", 1048576) // 1MB received
collector.RecordNetworkBytes("eth0", "tx", 524288)  // 512KB transmitted
```
- **Purpose**: Track network I/O
- **Labels**: `interface`, `direction` (rx, tx)
- **Unit**: Bytes

#### Disk Metrics
```go
// Record disk I/O
collector.RecordDiskIO("sda", "read", 2097152)   // 2MB read
collector.RecordDiskIO("sda", "write", 1048576)  // 1MB written
```
- **Purpose**: Track disk I/O operations
- **Labels**: `device`, `direction` (read, write)
- **Unit**: Bytes

### Operational Metrics

#### Sidecar Requests
```go
// Increment request counter
collector.IncrementSidecarRequests()
```
- **Purpose**: Track API usage
- **Type**: Counter (monotonic increasing)

#### Incident Detection
```go
// Record incident detection
collector.IncrementIncidents("crash", "high")
collector.IncrementIncidents("oom", "medium")
```
- **Purpose**: Track system incidents
- **Labels**: `type` (crash, oom, timeout), `severity` (low, medium, high)
- **Type**: Counter

#### Buffer Monitoring
```go
// Record buffer statistics
collector.RecordBufferSize(12582912)    // 12MB buffer
collector.RecordBufferEntries(60000)    // 60k entries
```
- **Purpose**: Monitor ring buffer health
- **Type**: Gauge (current value)

### Custom Metrics

#### Creating Custom Metrics
```go
// Custom counter
counter, err := collector.NewCustomCounter(
    "api_requests", 
    "Custom API request counter",
    []string{"endpoint", "method"}
)

// Custom gauge
gauge, err := collector.NewCustomGauge(
    "queue_depth",
    "Processing queue depth", 
    []string{"queue_name"}
)

// Custom histogram
histogram, err := collector.NewCustomHistogram(
    "request_duration",
    "Request duration in seconds",
    []string{"handler"},
    prometheus.DefBuckets
)
```

#### Managing Custom Metrics
```go
// Register metric
err := collector.RegisterCustomMetric("my_metric", myMetric)

// Unregister metric  
err := collector.UnregisterCustomMetric("my_metric")

// List custom metrics
metrics := collector.ListCustomMetrics()
```

## Monitoring and Alerting

### Prometheus Configuration
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'blackbox-daemon'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 15s
    metrics_path: /metrics
```

### Key Metrics to Monitor

#### System Health
```promql
# High CPU usage alert
blackbox_cpu_usage_percent{core="cpu"} > 90

# Low memory availability
blackbox_memory_usage_bytes{type="available"} / blackbox_memory_usage_bytes{type="total"} < 0.1

# High disk I/O
rate(blackbox_disk_io_bytes[5m]) > 100MB
```

#### Operational Health
```promql
# No sidecar requests (application down)
rate(blackbox_sidecar_requests_total[5m]) == 0

# High incident rate
rate(blackbox_incidents_total[10m]) > 0.1

# Buffer utilization
blackbox_buffer_entries_total / on() group_left() blackbox_buffer_size_bytes * 200 > 0.8
```

### Grafana Dashboard

#### System Overview Panel
- CPU usage by core (time series)
- Memory usage breakdown (stacked area)  
- Network I/O rates (multi-series)
- Disk I/O patterns (heatmap)

#### BlackBox Operations Panel
- Sidecar request rate (single stat)
- Incident detection timeline (bar chart)
- Buffer utilization (gauge)
- Active incidents by type (pie chart)

#### Custom Metrics Panel
- Application-specific metrics
- Custom counters and gauges
- Business logic indicators

## Performance Characteristics

### Throughput
- **Metric Updates**: >10,000 updates/second per metric
- **HTTP Requests**: Handles concurrent scrape requests
- **Memory Usage**: ~1-5MB for metric storage
- **CPU Overhead**: <1% of system resources

### Scalability
- **Metric Count**: Supports 1000+ concurrent metrics
- **Label Cardinality**: High-cardinality labels supported
- **Time Series**: Efficient storage in Prometheus
- **Retention**: Configurable data retention policies

### Reliability
- **Error Handling**: Graceful degradation on failures
- **Recovery**: Automatic recovery from transient errors
- **Isolation**: Metrics failures don't affect main application
- **Validation**: Input validation for all metric operations

## Integration Points

### System Telemetry Collector
```go
// Called every collection interval (1 second)
collector.RecordCPUUsage(core, usage)
collector.RecordMemoryUsage(memType, bytes)
collector.RecordNetworkBytes(iface, direction, bytes)
// ... other metrics
```

### API Server
```go
// On each sidecar request
collector.IncrementSidecarRequests()

// On telemetry submission
collector.RecordBufferEntries(buffer.Count())
```

### Incident Handler
```go
// When incident detected
collector.IncrementIncidents(incidentType, severity)

// Periodic buffer health
collector.RecordBufferSize(buffer.SizeBytes())
```

### Kubernetes Integration
```go
// Pod-specific metrics (future enhancement)
collector.RecordPodCPUUsage(podName, namespace, usage)
collector.RecordPodMemoryUsage(podName, namespace, bytes)
```

## Best Practices

### Metric Design
1. **Use appropriate metric types** (Counter for rates, Gauge for current values)
2. **Keep label cardinality reasonable** (<1000 unique combinations)
3. **Use consistent naming conventions** (snake_case, descriptive names)
4. **Include units in metric names** (_bytes, _seconds, _total)

### Performance
1. **Pre-create metrics at startup** (avoid runtime creation)
2. **Cache label value combinations** 
3. **Batch metric updates when possible**
4. **Monitor scrape duration** (should be <1 second)

### Operations
1. **Set up alerting rules** for critical metrics
2. **Create meaningful dashboards** for different audiences  
3. **Document custom metrics** and their purpose
4. **Regular cleanup** of unused custom metrics

### Security
1. **Secure metrics endpoint** if exposing externally
2. **Rate limit scrape requests** to prevent abuse
3. **Sanitize custom metric names** to prevent injection
4. **Monitor for unusual metric patterns** (anomaly detection)
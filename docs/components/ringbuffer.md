# Ring Buffer Component

## Overview

The Ring Buffer is the heart of BlackBox-Daemon's telemetry storage system. It provides a thread-safe, high-performance circular buffer that maintains a sliding window of telemetry data while preventing unbounded memory growth.

## Design Goals

- **Bounded Memory**: Fixed-size buffer prevents memory exhaustion
- **High Performance**: In-memory storage for minimal latency
- **Thread Safety**: Concurrent reads/writes from multiple goroutines
- **Time-Based Queries**: Efficient filtering by time windows
- **Automatic Cleanup**: Expired entries are automatically removed

## Implementation Details

### Data Structure
```go
type RingBuffer struct {
    mutex      sync.RWMutex              // Reader-writer lock for thread safety
    entries    []types.TelemetryEntry    // Circular array of telemetry entries
    size       int                       // Maximum buffer capacity
    head       int                       // Current write position
    count      int                       // Number of entries stored
    windowSize time.Duration             // Time window for data retention
}
```

### Circular Buffer Algorithm
The ring buffer uses a circular array implementation:
- `head` points to the next write position
- When `head` reaches the end, it wraps to position 0
- Old entries are automatically overwritten
- `count` tracks actual entries (may be less than `size`)

### Memory Management
- Buffer size is calculated as `windowSize.Seconds() * 1000` entries
- Minimum size of 1000 entries ensures adequate capacity
- No dynamic allocation during operation (pre-allocated array)
- Periodic cleanup removes entries outside the time window

## Key Operations

### Adding Entries
```go
func (rb *RingBuffer) Add(entry types.TelemetryEntry)
```
- **Thread Safety**: Uses write lock for exclusive access
- **Overwrite Policy**: Oldest entries are overwritten when buffer is full
- **Performance**: O(1) constant time operation

### Querying by Time Window
```go
func (rb *RingBuffer) GetWindow(from time.Time) []types.TelemetryEntry
```
- **Time-Based Filtering**: Returns entries within the configured window
- **Efficient Traversal**: Single pass through buffer in chronological order
- **Thread Safety**: Uses read lock for concurrent access
- **Performance**: O(n) where n is number of entries in buffer

### Filtering Operations
```go
func (rb *RingBuffer) FilterBySource(source types.TelemetrySource, from time.Time) []types.TelemetryEntry
func (rb *RingBuffer) FilterByPod(podName string, from time.Time) []types.TelemetryEntry
```
- **Source Filtering**: Separate system vs. sidecar telemetry
- **Pod Filtering**: Telemetry for specific pods or system-wide
- **Combined Operations**: Time window + metadata filtering

## Performance Characteristics

### Throughput
- **Target Rate**: ~1000 entries per second
- **Add Operation**: O(1) constant time
- **Query Operation**: O(n) linear scan (optimized for time-locality)
- **Memory Usage**: Bounded by buffer size × entry size

### Concurrency
- **Multiple Readers**: Supported via RWMutex
- **Single Writer**: Write operations are serialized
- **Reader-Writer**: Readers don't block each other
- **Lock Granularity**: Buffer-level locking (not entry-level)

### Memory Usage
```
Buffer Memory = entries × sizeof(TelemetryEntry)
                ≈ 1000-60000 entries × ~200 bytes
                ≈ 200KB - 12MB per daemon instance
```

## Configuration

### Buffer Sizing
The buffer size is automatically calculated based on:
```go
estimatedSize := int(windowSize.Seconds() * 1000)
if estimatedSize < 1000 {
    estimatedSize = 1000 // Minimum buffer size
}
```

### Window Size Options
- **Default**: 60 seconds
- **Range**: 10 seconds to 600 seconds (10 minutes)
- **Consideration**: Larger windows require more memory
- **Environment**: `BLACKBOX_BUFFER_WINDOW_SIZE`

## Usage Patterns

### Typical Flow
1. **System Collector** adds entries every second
2. **Sidecar API** adds entries as received from applications
3. **Incident Handler** queries time window when crash occurs
4. **Cleanup Process** removes expired entries periodically

### Query Examples
```go
// Get all telemetry from last 60 seconds
entries := buffer.GetWindow(time.Now())

// Get only system telemetry
systemEntries := buffer.FilterBySource(types.SourceSystem, time.Now())

// Get telemetry for specific pod
podEntries := buffer.FilterByPod("my-app-pod", time.Now())
```

## Monitoring

### Buffer Statistics
```go
type BufferStats struct {
    TotalEntries  int           // Current entry count
    BufferSize    int           // Maximum capacity
    WindowSize    time.Duration // Configured window
    ActualWindow  time.Duration // Actual data span
    OldestEntry   time.Time     // Timestamp of oldest entry
    NewestEntry   time.Time     // Timestamp of newest entry
}
```

### Health Indicators
- **Utilization**: `TotalEntries / BufferSize`
- **Data Freshness**: `time.Now() - NewestEntry`
- **Window Coverage**: `ActualWindow / WindowSize`
- **Entry Rate**: Entries added per second

## Error Handling

### Graceful Degradation
- **Empty Buffer**: Returns empty slice, not error
- **Time Skew**: Handles entries with future timestamps
- **Capacity Issues**: Overwrites oldest data (configurable behavior)

### Recovery Scenarios
- **Memory Pressure**: Cleanup process frees expired entries
- **Clock Changes**: Time-based queries remain functional
- **Concurrent Access**: RWMutex prevents data races

## Integration Points

### System Telemetry Collector
- **Rate**: 1 entry per second per metric
- **Volume**: ~50-100 entries per collection cycle
- **Pattern**: Regular, predictable load

### Sidecar API
- **Rate**: Variable, application-dependent
- **Volume**: 1-1000 entries per request
- **Pattern**: Bursty, event-driven load

### Incident Handler
- **Access Pattern**: Read-heavy during incidents
- **Query Size**: Entire time window (potentially 60,000 entries)
- **Frequency**: Triggered by crashes (infrequent but critical)

## Tuning Guidelines

### High-Throughput Environments
- Increase buffer size for higher entry rates
- Consider shorter cleanup intervals
- Monitor memory usage and adjust window size

### Memory-Constrained Environments  
- Reduce window size to lower memory footprint
- Increase cleanup frequency
- Monitor buffer utilization

### Debug/Development
- Enable detailed logging for buffer operations
- Use smaller window sizes for faster testing
- Monitor buffer statistics via Prometheus metrics
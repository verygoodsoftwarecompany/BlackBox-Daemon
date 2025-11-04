# BlackBox-Daemon Monitoring & Metrics

This document describes the monitoring capabilities, Prometheus metrics, and observability features of BlackBox-Daemon.

## Overview

BlackBox-Daemon provides comprehensive metrics about its own operation and the system telemetry it collects. These metrics are exported in Prometheus format and can be scraped by monitoring systems for alerting and visualization.

## Prometheus Metrics

### System Metrics

#### CPU Metrics
```
# HELP blackbox_cpu_usage_percent Current CPU usage percentage per core
# TYPE blackbox_cpu_usage_percent gauge
blackbox_cpu_usage_percent{core="0"} 23.5
blackbox_cpu_usage_percent{core="1"} 45.2

# HELP blackbox_cpu_load_average System load averages
# TYPE blackbox_cpu_load_average gauge  
blackbox_cpu_load_average{period="1m"} 1.25
blackbox_cpu_load_average{period="5m"} 1.10
blackbox_cpu_load_average{period="15m"} 0.98
```

#### Memory Metrics
```
# HELP blackbox_memory_usage_bytes Memory usage in bytes by type
# TYPE blackbox_memory_usage_bytes gauge
blackbox_memory_usage_bytes{type="total"} 8589934592
blackbox_memory_usage_bytes{type="free"} 2147483648
blackbox_memory_usage_bytes{type="available"} 4294967296
blackbox_memory_usage_bytes{type="buffers"} 536870912
blackbox_memory_usage_bytes{type="cached"} 1073741824

# HELP blackbox_memory_utilization_percent Memory utilization percentage
# TYPE blackbox_memory_utilization_percent gauge
blackbox_memory_utilization_percent 75.5
```

#### Network Metrics
```
# HELP blackbox_network_bytes_total Network I/O bytes by interface and direction
# TYPE blackbox_network_bytes_total counter
blackbox_network_bytes_total{interface="eth0",direction="tx"} 1234567890
blackbox_network_bytes_total{interface="eth0",direction="rx"} 9876543210

# HELP blackbox_network_packets_total Network packets by interface and direction
# TYPE blackbox_network_packets_total counter
blackbox_network_packets_total{interface="eth0",direction="tx"} 1234567
blackbox_network_packets_total{interface="eth0",direction="rx"} 9876543

# HELP blackbox_network_errors_total Network errors by interface and direction
# TYPE blackbox_network_errors_total counter
blackbox_network_errors_total{interface="eth0",direction="tx"} 123
blackbox_network_errors_total{interface="eth0",direction="rx"} 456
```

#### Disk Metrics
```
# HELP blackbox_disk_operations_total Disk operations by device and type
# TYPE blackbox_disk_operations_total counter
blackbox_disk_operations_total{device="sda",type="read"} 1234567
blackbox_disk_operations_total{device="sda",type="write"} 7654321

# HELP blackbox_disk_bytes_total Disk I/O bytes by device and type
# TYPE blackbox_disk_bytes_total counter  
blackbox_disk_bytes_total{device="sda",type="read"} 1234567890123
blackbox_disk_bytes_total{device="sda",type="write"} 9876543210987

# HELP blackbox_disk_usage_bytes Disk space usage by mount point
# TYPE blackbox_disk_usage_bytes gauge
blackbox_disk_usage_bytes{mountpoint="/",type="total"} 107374182400
blackbox_disk_usage_bytes{mountpoint="/",type="used"} 53687091200
blackbox_disk_usage_bytes{mountpoint="/",type="free"} 53687091200
```

### Application Metrics

#### Ring Buffer Metrics
```
# HELP blackbox_buffer_entries_total Total telemetry entries in buffer
# TYPE blackbox_buffer_entries_total gauge
blackbox_buffer_entries_total 1250

# HELP blackbox_buffer_utilization_percent Buffer utilization percentage
# TYPE blackbox_buffer_utilization_percent gauge
blackbox_buffer_utilization_percent 34.7

# HELP blackbox_buffer_operations_total Buffer operations by type
# TYPE blackbox_buffer_operations_total counter
blackbox_buffer_operations_total{operation="add"} 125000
blackbox_buffer_operations_total{operation="cleanup"} 45
blackbox_buffer_operations_total{operation="query"} 1250

# HELP blackbox_buffer_memory_bytes Buffer memory usage in bytes
# TYPE blackbox_buffer_memory_bytes gauge
blackbox_buffer_memory_bytes 2567890
```

#### API Metrics
```
# HELP blackbox_api_requests_total HTTP API requests by endpoint and status
# TYPE blackbox_api_requests_total counter
blackbox_api_requests_total{endpoint="/api/v1/telemetry",method="POST",status="201"} 12500
blackbox_api_requests_total{endpoint="/api/v1/telemetry",method="POST",status="400"} 25
blackbox_api_requests_total{endpoint="/health",method="GET",status="200"} 1500

# HELP blackbox_api_request_duration_seconds HTTP request duration
# TYPE blackbox_api_request_duration_seconds histogram
blackbox_api_request_duration_seconds_bucket{endpoint="/api/v1/telemetry",le="0.1"} 12000
blackbox_api_request_duration_seconds_bucket{endpoint="/api/v1/telemetry",le="0.5"} 12450
blackbox_api_request_duration_seconds_bucket{endpoint="/api/v1/telemetry",le="1.0"} 12500
blackbox_api_request_duration_seconds_sum{endpoint="/api/v1/telemetry"} 1250.5
blackbox_api_request_duration_seconds_count{endpoint="/api/v1/telemetry"} 12500

# HELP blackbox_api_active_connections Current active API connections
# TYPE blackbox_api_active_connections gauge
blackbox_api_active_connections 15
```

#### Kubernetes Metrics
```
# HELP blackbox_k8s_pods_total Kubernetes pods by status
# TYPE blackbox_k8s_pods_total gauge
blackbox_k8s_pods_total{status="running"} 25
blackbox_k8s_pods_total{status="pending"} 2
blackbox_k8s_pods_total{status="failed"} 1

# HELP blackbox_k8s_incidents_total Kubernetes incidents by type and severity
# TYPE blackbox_k8s_incidents_total counter
blackbox_k8s_incidents_total{type="crash",severity="high"} 5
blackbox_k8s_incidents_total{type="oom",severity="critical"} 2
blackbox_k8s_incidents_total{type="restart",severity="medium"} 15

# HELP blackbox_k8s_pod_restarts_total Pod restart count by namespace and pod
# TYPE blackbox_k8s_pod_restarts_total counter
blackbox_k8s_pod_restarts_total{namespace="production",pod="user-service-abc123"} 3
```

#### Telemetry Collection Metrics
```
# HELP blackbox_telemetry_entries_total Telemetry entries collected by source
# TYPE blackbox_telemetry_entries_total counter
blackbox_telemetry_entries_total{source="system"} 125000
blackbox_telemetry_entries_total{source="sidecar"} 25000
blackbox_telemetry_entries_total{source="incident"} 150

# HELP blackbox_telemetry_collection_duration_seconds Time spent collecting telemetry
# TYPE blackbox_telemetry_collection_duration_seconds histogram
blackbox_telemetry_collection_duration_seconds_bucket{source="system",le="0.01"} 12000
blackbox_telemetry_collection_duration_seconds_bucket{source="system",le="0.1"} 12450
blackbox_telemetry_collection_duration_seconds_sum{source="system"} 125.5
blackbox_telemetry_collection_duration_seconds_count{source="system"} 12500

# HELP blackbox_telemetry_errors_total Telemetry collection errors by type
# TYPE blackbox_telemetry_errors_total counter
blackbox_telemetry_errors_total{type="file_not_found"} 45
blackbox_telemetry_errors_total{type="permission_denied"} 12
blackbox_telemetry_errors_total{type="parse_error"} 3
```

## Monitoring Setup

### Prometheus Configuration

```yaml
# prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
- job_name: 'blackbox-daemon'
  static_configs:
  - targets: ['localhost:9090']
  scrape_interval: 30s
  metrics_path: /metrics
  
# For Kubernetes deployment
- job_name: 'kubernetes-blackbox-daemon'
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:
  - source_labels: [__meta_kubernetes_pod_label_app]
    action: keep
    regex: blackbox-daemon
  - source_labels: [__meta_kubernetes_pod_container_port_name]
    action: keep
    regex: metrics
```

### ServiceMonitor (Prometheus Operator)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: blackbox-daemon
  labels:
    app: blackbox-daemon
spec:
  selector:
    matchLabels:
      app: blackbox-daemon
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
    scrapeTimeout: 10s
    honorLabels: true
```

## Alerting Rules

### Critical Alerts

```yaml
# alerts.yml
groups:
- name: blackbox-daemon-critical
  rules:
  
  # Buffer overflow alert
  - alert: BlackBoxBufferOverflow
    expr: blackbox_buffer_utilization_percent > 90
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "BlackBox buffer nearly full"
      description: "Buffer utilization is {{ $value }}% on node {{ $labels.node }}"
  
  # High error rate
  - alert: BlackBoxHighErrorRate
    expr: rate(blackbox_api_requests_total{status=~"4..|5.."}[5m]) / rate(blackbox_api_requests_total[5m]) > 0.1
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "High error rate in BlackBox API"
      description: "Error rate is {{ $value | humanizePercentage }} on node {{ $labels.node }}"
  
  # Service down
  - alert: BlackBoxServiceDown
    expr: up{job="blackbox-daemon"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "BlackBox daemon is down"
      description: "BlackBox daemon on node {{ $labels.node }} is not responding"
```

### Warning Alerts

```yaml
- name: blackbox-daemon-warning
  rules:
  
  # High memory usage
  - alert: BlackBoxHighMemoryUsage
    expr: blackbox_memory_utilization_percent > 80
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High memory usage detected"
      description: "Memory usage is {{ $value }}% on node {{ $labels.node }}"
  
  # Frequent pod restarts
  - alert: BlackBoxFrequentPodRestarts
    expr: rate(blackbox_k8s_incidents_total{type="restart"}[10m]) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Frequent pod restarts detected"
      description: "Pod restart rate is {{ $value }} per second on node {{ $labels.node }}"
  
  # Collection latency
  - alert: BlackBoxHighCollectionLatency
    expr: histogram_quantile(0.95, rate(blackbox_telemetry_collection_duration_seconds_bucket[5m])) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High telemetry collection latency"
      description: "95th percentile collection time is {{ $value }}s on node {{ $labels.node }}"
```

## Grafana Dashboards

### System Overview Dashboard

```json
{
  "dashboard": {
    "title": "BlackBox Daemon - System Overview",
    "panels": [
      {
        "title": "CPU Usage",
        "type": "stat",
        "targets": [
          {
            "expr": "avg(blackbox_cpu_usage_percent)",
            "legendFormat": "Average CPU %"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "unit": "percent",
            "min": 0,
            "max": 100
          }
        }
      },
      {
        "title": "Memory Usage", 
        "type": "stat",
        "targets": [
          {
            "expr": "blackbox_memory_utilization_percent",
            "legendFormat": "Memory %"
          }
        ]
      },
      {
        "title": "Buffer Utilization",
        "type": "gauge",
        "targets": [
          {
            "expr": "blackbox_buffer_utilization_percent",
            "legendFormat": "Buffer %"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "unit": "percent",
            "thresholds": {
              "steps": [
                { "color": "green", "value": 0 },
                { "color": "yellow", "value": 70 },
                { "color": "red", "value": 90 }
              ]
            }
          }
        }
      },
      {
        "title": "API Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(blackbox_api_requests_total[5m])",
            "legendFormat": "{{endpoint}} - {{status}}"
          }
        ]
      }
    ]
  }
}
```

### Application Performance Dashboard

```json
{
  "dashboard": {
    "title": "BlackBox Daemon - Application Performance",
    "panels": [
      {
        "title": "Telemetry Collection Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(blackbox_telemetry_entries_total[5m])",
            "legendFormat": "{{source}} entries/sec"
          }
        ]
      },
      {
        "title": "API Response Times",
        "type": "graph", 
        "targets": [
          {
            "expr": "histogram_quantile(0.50, rate(blackbox_api_request_duration_seconds_bucket[5m]))",
            "legendFormat": "50th percentile"
          },
          {
            "expr": "histogram_quantile(0.95, rate(blackbox_api_request_duration_seconds_bucket[5m]))", 
            "legendFormat": "95th percentile"
          },
          {
            "expr": "histogram_quantile(0.99, rate(blackbox_api_request_duration_seconds_bucket[5m]))",
            "legendFormat": "99th percentile"
          }
        ]
      },
      {
        "title": "Pod Incidents by Type",
        "type": "piechart",
        "targets": [
          {
            "expr": "increase(blackbox_k8s_incidents_total[1h])",
            "legendFormat": "{{type}}"
          }
        ]
      }
    ]
  }
}
```

## Custom Metrics

### Adding Application-Specific Metrics

```go
// internal/metrics/custom.go
import "github.com/prometheus/client_golang/prometheus"

type CustomMetrics struct {
    processingTime prometheus.Histogram
    errorCount     prometheus.Counter
}

func NewCustomMetrics(registry prometheus.Registerer) *CustomMetrics {
    metrics := &CustomMetrics{
        processingTime: prometheus.NewHistogram(prometheus.HistogramOpts{
            Name: "blackbox_custom_processing_duration_seconds",
            Help: "Time spent processing custom telemetry",
            Buckets: prometheus.DefBuckets,
        }),
        errorCount: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "blackbox_custom_errors_total", 
            Help: "Total custom processing errors",
        }),
    }
    
    registry.MustRegister(metrics.processingTime)
    registry.MustRegister(metrics.errorCount)
    
    return metrics
}

func (cm *CustomMetrics) RecordProcessingTime(duration time.Duration) {
    cm.processingTime.Observe(duration.Seconds())
}

func (cm *CustomMetrics) IncrementErrorCount() {
    cm.errorCount.Inc()
}
```

### Runtime Metrics Integration

```go
// Integrate with existing collector
func (c *Collector) RegisterCustomMetrics(name string, metric prometheus.Metric) error {
    return c.registry.Register(metric)
}

// Use in application code
func (sc *SystemCollector) collectWithMetrics() error {
    start := time.Now()
    defer func() {
        sc.metrics.RecordProcessingTime(time.Since(start))
    }()
    
    if err := sc.collect(); err != nil {
        sc.metrics.IncrementErrorCount()
        return err
    }
    
    return nil
}
```

## Health Monitoring

### Health Check Endpoint

```http
GET /health
```

Response format:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": "2h15m30s",
  "components": {
    "ring_buffer": "healthy",
    "system_collector": "healthy",
    "pod_watcher": "healthy", 
    "api_server": "healthy",
    "metrics_collector": "healthy"
  },
  "metrics": {
    "buffer_utilization": 34.7,
    "memory_usage_mb": 128,
    "goroutines": 45,
    "gc_pause_ms": 2.5
  }
}
```

### Kubernetes Probes

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
  timeoutSeconds: 3
  successThreshold: 1
  failureThreshold: 3
```

## Log Monitoring

### Structured Logging

BlackBox-Daemon uses structured JSON logging when `BLACKBOX_LOG_JSON=true`:

```json
{
  "timestamp": "2024-11-02T15:04:05Z",
  "level": "info",
  "component": "api",
  "message": "Telemetry received",
  "pod_name": "user-service-abc123",
  "namespace": "production",
  "runtime": "jvm",
  "duration_ms": 15.5
}
```

### Log Aggregation

#### Fluentd Configuration

```yaml
<source>
  @type tail
  path /var/log/blackbox/*.log
  pos_file /var/log/fluentd/blackbox.log.pos
  tag kubernetes.blackbox
  format json
  time_key timestamp
  time_format %Y-%m-%dT%H:%M:%S%z
</source>

<match kubernetes.blackbox>
  @type elasticsearch
  host elasticsearch.logging.svc.cluster.local
  port 9200
  index_name blackbox-logs
</match>
```

#### Promtail Configuration

```yaml
server:
  http_listen_port: 9080

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
- job_name: blackbox
  static_configs:
  - targets:
      - localhost
    labels:
      job: blackbox-daemon
      __path__: /var/log/blackbox/*.log
```

## Performance Monitoring

### Key Performance Indicators

1. **Throughput Metrics**:
   - Telemetry entries per second
   - API requests per second
   - Buffer operations per second

2. **Latency Metrics**:
   - System collection latency
   - API response times
   - Buffer operation times

3. **Resource Metrics**:
   - Memory utilization
   - CPU usage
   - Disk I/O rates

4. **Error Metrics**:
   - API error rates
   - Collection failures
   - System resource errors

### SLA Monitoring

```yaml
# SLO definitions for monitoring
slos:
  api_availability:
    target: 99.9%
    query: rate(blackbox_api_requests_total{status!~"5.."}[5m]) / rate(blackbox_api_requests_total[5m])
  
  collection_latency:
    target: 95% < 100ms
    query: histogram_quantile(0.95, rate(blackbox_telemetry_collection_duration_seconds_bucket[5m])) < 0.1
  
  buffer_utilization:
    target: < 80%
    query: blackbox_buffer_utilization_percent < 80
```

This monitoring setup provides comprehensive observability into BlackBox-Daemon's operation, enabling proactive issue detection and performance optimization.
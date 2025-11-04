# BlackBox-Daemon Configuration Guide

This document describes all configuration options for BlackBox-Daemon, including environment variables, configuration files, and runtime settings.

## Environment Variables

BlackBox-Daemon is configured entirely through environment variables for container-friendly deployment.

### Core Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `BLACKBOX_BUFFER_WINDOW_SIZE` | `"60s"` | Time window for telemetry retention in memory |
| `BLACKBOX_COLLECTION_INTERVAL` | `"1s"` | How often to collect system metrics |
| `BLACKBOX_API_KEY` | *required* | Authentication key for sidecar API access |

### API Server Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `BLACKBOX_API_PORT` | `8080` | Port for the REST API server |
| `BLACKBOX_SWAGGER_ENABLE` | `false` | Enable Swagger documentation endpoint |

### Metrics Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `BLACKBOX_METRICS_PORT` | `9090` | Port for Prometheus metrics export |
| `BLACKBOX_METRICS_PATH` | `"/metrics"` | Path for metrics endpoint |

### Output Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `BLACKBOX_OUTPUT_FORMATTERS` | `"default"` | Comma-separated list of output formatters |
| `BLACKBOX_OUTPUT_PATH` | `"/var/log/blackbox"` | Output directory for formatted data |

#### Available Formatters

- **default**: Human-readable format for debugging
- **json**: JSON format for structured logging
- **csv**: CSV format for data analysis

### Kubernetes Integration

| Variable | Default | Description |
|----------|---------|-------------|
| `NODE_NAME` | *auto-detected* | Name of the Kubernetes node |
| `POD_NAMESPACE` | *auto-detected* | Current pod's namespace |
| `KUBECONFIG` | *in-cluster* | Path to kubeconfig file (for development) |

### Logging Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `BLACKBOX_LOG_LEVEL` | `"info"` | Log verbosity (debug, info, warn, error) |
| `BLACKBOX_LOG_JSON` | `true` | Use JSON log format |

## Configuration Examples

### Production Kubernetes Deployment

```yaml
env:
- name: BLACKBOX_API_KEY
  valueFrom:
    secretKeyRef:
      name: blackbox-daemon-secret
      key: api-key
- name: BLACKBOX_BUFFER_WINDOW_SIZE
  value: "300s"  # 5 minutes for production
- name: BLACKBOX_COLLECTION_INTERVAL
  value: "5s"    # Less frequent collection
- name: BLACKBOX_OUTPUT_FORMATTERS
  value: "json,csv"
- name: BLACKBOX_LOG_LEVEL
  value: "warn"
```

### Development/Testing

```bash
export BLACKBOX_API_KEY="test-api-key-12345"
export BLACKBOX_BUFFER_WINDOW_SIZE="30s"
export BLACKBOX_COLLECTION_INTERVAL="1s"
export BLACKBOX_OUTPUT_FORMATTERS="default,json"
export BLACKBOX_LOG_LEVEL="debug"
export BLACKBOX_SWAGGER_ENABLE="true"
```

### High-Volume Environment

```yaml
env:
- name: BLACKBOX_BUFFER_WINDOW_SIZE
  value: "600s"  # 10 minutes
- name: BLACKBOX_COLLECTION_INTERVAL
  value: "10s"   # Reduce collection frequency
- name: BLACKBOX_OUTPUT_FORMATTERS
  value: "json"  # Single efficient formatter
- name: BLACKBOX_OUTPUT_PATH
  value: "/var/log/blackbox"
```

## Security Considerations

### API Key Management

- **Generate Strong Keys**: Use at least 32 characters with mixed case, numbers, and symbols
- **Rotate Keys**: Implement regular key rotation policies
- **Secret Storage**: Use Kubernetes secrets, never hardcode in configuration

```bash
# Generate secure API key
openssl rand -base64 32

# Create Kubernetes secret
kubectl create secret generic blackbox-daemon-secret \
  --from-literal=api-key="$(openssl rand -base64 32)"
```

### Network Security

- **Internal Only**: API should only be accessible within the cluster
- **TLS**: Consider TLS termination at ingress/service mesh level
- **RBAC**: Limit Kubernetes API access to minimum required permissions

## Performance Tuning

### Memory Usage

Buffer window size directly affects memory usage:
- 1 second interval × 60 second window = ~60 telemetry entries
- Each entry is approximately 1-2KB
- Total memory: ~120KB per component + overhead

### CPU Usage

Collection interval affects CPU load:
- More frequent collection = higher CPU usage
- Balance between granularity and performance
- Monitor using Prometheus metrics

### Disk I/O

Output configuration affects disk usage:
- Multiple formatters increase I/O
- Consider log rotation for file outputs
- Use appropriate volume types for output paths

## Validation

### Configuration Validation

The daemon validates configuration on startup:

```go
// Example validation errors
2024-11-02T15:04:05Z ERROR Invalid buffer window size: "invalid"
2024-11-02T15:04:05Z ERROR API key cannot be empty
2024-11-02T15:04:05Z ERROR Invalid collection interval: "0s"
```

### Runtime Health Checks

Monitor configuration health:

```bash
# Check health endpoint
curl http://localhost:8080/health

# Check metrics for config errors
curl http://localhost:9090/metrics | grep config_errors
```

## Advanced Configuration

### Custom Formatters

To add custom formatters, extend the formatter chain in code:

```go
// See internal/formatter/formatter.go
type CustomFormatter struct {
    // Implementation
}

func (cf *CustomFormatter) Format(entry *types.TelemetryEntry) ([]byte, error) {
    // Custom formatting logic
}
```

### Integration Patterns

#### Sidecar Configuration

```yaml
# Application pod with sidecar
spec:
  containers:
  - name: app
    # Main application
  - name: telemetry-sidecar
    image: my-app/telemetry-sidecar
    env:
    - name: BLACKBOX_API_ENDPOINT
      value: "http://localhost:8080/api/v1/telemetry"
    - name: BLACKBOX_API_KEY
      valueFrom:
        secretKeyRef:
          name: blackbox-daemon-secret
          key: api-key
```

#### Monitoring Integration

```yaml
# ServiceMonitor for Prometheus
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: blackbox-daemon
spec:
  selector:
    matchLabels:
      app: blackbox-daemon
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

## Troubleshooting Configuration

### Common Issues

1. **API Key Errors**: Verify secret exists and is properly mounted
2. **Port Conflicts**: Check if ports are already in use
3. **Permission Errors**: Ensure proper RBAC and volume permissions
4. **Memory Issues**: Reduce buffer window size or collection interval

### Debug Mode

Enable comprehensive logging:

```bash
export BLACKBOX_LOG_LEVEL="debug"
export BLACKBOX_LOG_JSON="false"  # Human-readable logs
```

### Configuration Dump

The daemon logs its configuration on startup:

```
2024-11-02T15:04:05Z INFO Configuration loaded successfully
2024-11-02T15:04:05Z INFO Buffer window: 60s, Collection interval: 1s
2024-11-02T15:04:05Z INFO API port: 8080, Metrics port: 9090
2024-11-02T15:04:05Z INFO Output formatters: [default json]
```
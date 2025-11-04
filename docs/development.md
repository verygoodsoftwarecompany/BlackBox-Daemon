# BlackBox-Daemon Development Guide

This guide covers building, testing, extending, and contributing to BlackBox-Daemon.

## Development Environment Setup

### Prerequisites

- **Go 1.25+** (required by Kubernetes client libraries)
- **Docker** 20.10+ (for containerization)
- **kubectl** (for Kubernetes testing)
- **Git** (for version control)
- **Make** (optional, for build automation)

### Getting Started

```bash
# Clone repository
git clone https://github.com/verygoodsoftwarecompany/blackbox-daemon.git
cd blackbox-daemon

# Install dependencies
go mod download

# Build project
go build -o blackbox-daemon ./cmd/blackbox-daemon

# Run locally (requires Linux)
export BLACKBOX_API_KEY="test-api-key-12345"
sudo ./blackbox-daemon
```

## Project Structure

```
blackbox-daemon/
├── cmd/
│   └── blackbox-daemon/        # Main application entry point
│       └── main.go
├── internal/                   # Private application code
│   ├── api/                   # REST API server
│   ├── config/                # Configuration management
│   ├── formatter/             # Output formatters
│   ├── k8s/                   # Kubernetes integration
│   ├── metrics/               # Prometheus metrics
│   ├── ringbuffer/            # Telemetry storage
│   └── telemetry/             # System metrics collection
├── pkg/                       # Public library code
│   └── types/                 # Shared data types
├── docs/                      # Documentation
├── deployments/               # Kubernetes manifests
├── images/                    # Project images/logos
├── docker-compose.yml         # Local development setup
├── Dockerfile                 # Container build
├── go.mod                     # Go module definition
└── README.md                  # Project overview
```

## Building from Source

### Local Build

```bash
# Clean build
go clean
go mod tidy
go build -v ./...

# Build specific target
go build -o blackbox-daemon ./cmd/blackbox-daemon

# Build with optimizations
CGO_ENABLED=0 go build -ldflags="-w -s" -o blackbox-daemon ./cmd/blackbox-daemon
```

### Cross-Platform Builds

```bash
# Linux (production target)
GOOS=linux GOARCH=amd64 go build -o blackbox-daemon-linux ./cmd/blackbox-daemon

# macOS (development)
GOOS=darwin GOARCH=amd64 go build -o blackbox-daemon-macos ./cmd/blackbox-daemon

# Windows
GOOS=windows GOARCH=amd64 go build -o blackbox-daemon.exe ./cmd/blackbox-daemon
```

### Docker Build

```bash
# Standard build
docker build -t blackbox-daemon:dev .

# Multi-stage build with debugging
docker build --target builder -t blackbox-daemon:builder .

# Build with specific Go version
docker build --build-arg GO_VERSION=1.25 -t blackbox-daemon:latest .
```

## Code Organization

### Package Guidelines

#### `internal/` - Private Code
- Code that should not be imported by external projects
- Business logic, implementations, and internal APIs
- Each package should have a single, well-defined responsibility

#### `pkg/` - Public Code  
- Types and interfaces that other projects might import
- Stable APIs with backward compatibility considerations
- Minimal dependencies on internal packages

#### `cmd/` - Applications
- Main applications and entry points
- Thin layer that composes internal packages
- Configuration and startup logic only

### Coding Standards

#### Go Style Guidelines

```go
// Package documentation
// Package api provides a REST API server for sidecar telemetry submission.
//
// The server accepts telemetry data from application sidecars and forwards
// it to the ring buffer for storage. It also provides endpoints for health
// checks and data export.
package api

// Interface documentation  
// TelemetryCollector defines the interface for collecting system telemetry.
type TelemetryCollector interface {
    // Start begins telemetry collection in the background.
    // It returns when the context is cancelled or an error occurs.
    Start(ctx context.Context) error
    
    // GetMetrics returns the current telemetry snapshot.
    GetMetrics() (*types.TelemetryEntry, error)
}

// Struct documentation
// SystemCollector collects Linux system metrics from /proc and /sys filesystems.
// It is safe for concurrent use and handles missing or inaccessible files gracefully.
type SystemCollector struct {
    // mutex protects concurrent access to collector state
    mutex    sync.RWMutex
    interval time.Duration
    buffer   *ringbuffer.RingBuffer
}

// Method documentation
// collectCPUStats reads CPU usage statistics from /proc/stat.
// Returns per-core CPU usage percentages and system load averages.
func (sc *SystemCollector) collectCPUStats() (*types.CPUStats, error) {
    // Implementation
}
```

#### Error Handling

```go
// Wrap errors with context
if err := sc.collectMetrics(); err != nil {
    return fmt.Errorf("failed to collect system metrics: %w", err)
}

// Define custom error types for API
var (
    ErrInvalidAPIKey = errors.New("invalid API key")
    ErrBufferFull    = errors.New("telemetry buffer is full")
)

// Handle expected errors gracefully
func (sc *SystemCollector) readProcFile(path string) ([]byte, error) {
    data, err := os.ReadFile(path)
    if os.IsNotExist(err) {
        // Expected on some systems
        return nil, nil
    }
    if err != nil {
        return nil, fmt.Errorf("reading %s: %w", path, err)
    }
    return data, nil
}
```

#### Logging Standards

```go
import "log"

// Use structured logging levels
log.Printf("Starting system collector with interval %v", interval)  // Info
log.Printf("Warning: failed to read optional file %s: %v", path, err)  // Warning  
log.Printf("Error: critical system failure: %v", err)  // Error

// Include context in log messages
log.Printf("Pod %s/%s crashed with exit code %d", namespace, name, exitCode)

// Use consistent prefixes for different components
log.Printf("[API] Client authentication failed: %v", err)
log.Printf("[K8S] Pod watcher started for node %s", nodeName)  
log.Printf("[BUFFER] Cleanup removed %d expired entries", count)
```

## Testing

### Unit Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package tests
go test ./internal/ringbuffer/

# Run with race detection
go test -race ./...
```

### Integration Tests

```bash
# Test with Docker Compose
docker-compose -f docker-compose.test.yml up --build --abort-on-container-exit

# Test Kubernetes deployment
kubectl apply -f deployments/test/
kubectl wait --for=condition=ready pod -l app=blackbox-daemon-test
kubectl logs -l app=blackbox-daemon-test
```

### Manual Testing

```bash
# Start daemon locally
export BLACKBOX_API_KEY="test-key"
export BLACKBOX_LOG_LEVEL="debug"
sudo ./blackbox-daemon &

# Test API endpoints
curl -H "Authorization: Bearer test-key" \
     -X POST http://localhost:8080/api/v1/telemetry \
     -d '{"pod_name":"test","namespace":"default","runtime":"test","data":{}}'

curl http://localhost:8080/health
curl http://localhost:9090/metrics
```

## Extending the System

### Adding New Formatters

1. **Create formatter struct**:

```go
// internal/formatter/custom.go
type CustomFormatter struct {
    config CustomConfig
}

func NewCustomFormatter(config CustomConfig) *CustomFormatter {
    return &CustomFormatter{config: config}
}

func (cf *CustomFormatter) Format(entry *types.TelemetryEntry) ([]byte, error) {
    // Custom formatting logic
    return customBytes, nil
}

func (cf *CustomFormatter) ContentType() string {
    return "application/x-custom"
}
```

2. **Register in formatter chain**:

```go
// internal/formatter/formatter.go
func createFormatter(name string, dest Destination) (Formatter, error) {
    switch name {
    case "custom":
        return NewCustomFormatter(CustomConfig{}), nil
    // ... existing formatters
    }
}
```

### Adding New Telemetry Sources

1. **Define data structures**:

```go
// pkg/types/custom.go
type CustomMetrics struct {
    Timestamp time.Time            `json:"timestamp"`
    Values    map[string]float64   `json:"values"`
    Tags      map[string]string    `json:"tags,omitempty"`
}
```

2. **Implement collector**:

```go
// internal/telemetry/custom.go
type CustomCollector struct {
    interval time.Duration
    buffer   *ringbuffer.RingBuffer
}

func (cc *CustomCollector) Start(ctx context.Context) error {
    ticker := time.NewTicker(cc.interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if metrics, err := cc.collect(); err == nil {
                cc.buffer.Add(metrics)
            }
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

func (cc *CustomCollector) collect() (*types.TelemetryEntry, error) {
    // Custom collection logic
}
```

3. **Integrate in main daemon**:

```go
// cmd/blackbox-daemon/main.go
func NewBlackBoxDaemon(cfg *config.Config) (*BlackBoxDaemon, error) {
    // ... existing collectors
    
    customCollector := telemetry.NewCustomCollector(cfg.CustomConfig, buffer)
    
    return &BlackBoxDaemon{
        // ... existing fields
        customCollector: customCollector,
    }, nil
}
```

### Adding New API Endpoints

1. **Define handlers**:

```go
// internal/api/custom_handlers.go
func (s *Server) handleCustomEndpoint(w http.ResponseWriter, r *http.Request) {
    // Authentication check
    if !s.authenticate(r) {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    
    // Implementation
    response := CustomResponse{/* ... */}
    s.writeJSON(w, response)
}
```

2. **Register routes**:

```go
// internal/api/server.go
func (s *Server) setupRoutes() {
    // ... existing routes
    s.mux.HandleFunc("/api/v1/custom", s.handleCustomEndpoint).Methods("GET", "POST")
}
```

## Performance Optimization

### Memory Management

```go
// Use object pools for frequent allocations
var telemetryPool = sync.Pool{
    New: func() interface{} {
        return &types.TelemetryEntry{}
    },
}

func (rb *RingBuffer) Add(entry *types.TelemetryEntry) {
    // Get from pool
    poolEntry := telemetryPool.Get().(*types.TelemetryEntry)
    *poolEntry = *entry
    
    // Use pooled object
    rb.addInternal(poolEntry)
}

func (rb *RingBuffer) cleanup() {
    for _, entry := range rb.expiredEntries {
        // Reset and return to pool
        *entry = types.TelemetryEntry{}
        telemetryPool.Put(entry)
    }
}
```

### CPU Optimization

```go
// Use buffered channels for high-throughput
const bufferSize = 1000
telemetryChan := make(chan *types.TelemetryEntry, bufferSize)

// Batch process metrics
func (sc *SystemCollector) processBatch(entries []*types.TelemetryEntry) {
    // Process multiple entries at once
    for _, entry := range entries {
        sc.buffer.Add(entry)
    }
}
```

### I/O Optimization  

```go
// Use bufio for efficient file operations
func (sc *SystemCollector) readProcStats() ([]byte, error) {
    file, err := os.Open("/proc/stat")
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    reader := bufio.NewReader(file)
    return reader.ReadAll()
}

// Batch filesystem operations
func (sc *SystemCollector) collectAll() error {
    var wg sync.WaitGroup
    results := make(chan result, 10)
    
    // Collect metrics in parallel
    for _, path := range paths {
        wg.Add(1)
        go func(p string) {
            defer wg.Done()
            if data, err := sc.readPath(p); err == nil {
                results <- result{path: p, data: data}
            }
        }(path)
    }
    
    go func() {
        wg.Wait()
        close(results)
    }()
    
    // Process results
    for result := range results {
        sc.processResult(result)
    }
    
    return nil
}
```

## Debugging

### Local Debugging

```bash
# Enable debug mode
export BLACKBOX_LOG_LEVEL="debug"
export BLACKBOX_LOG_JSON="false"

# Run with verbose output
go run ./cmd/blackbox-daemon -v

# Use delve debugger
dlv debug ./cmd/blackbox-daemon -- --config=debug.yaml
```

### Container Debugging

```bash
# Build debug image
docker build --target builder -t blackbox-daemon:debug .

# Run with debugging tools
docker run -it --rm \
  --privileged \
  --pid=host \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  blackbox-daemon:debug /bin/sh

# Inside container
/app/blackbox-daemon --help
cat /host/proc/meminfo
```

### Kubernetes Debugging

```bash
# Debug pod
kubectl debug blackbox-daemon-xyz -it --image=alpine -- sh

# Port forward for local access
kubectl port-forward svc/blackbox-daemon-api 8080:8080

# Check system calls
kubectl exec blackbox-daemon-xyz -- strace -p 1

# Monitor resource usage
kubectl top pods -l app=blackbox-daemon
```

## Contributing

### Development Workflow

1. **Fork and clone**:
```bash
git clone https://github.com/YOUR_USERNAME/blackbox-daemon.git
cd blackbox-daemon
git remote add upstream https://github.com/verygoodsoftwarecompany/blackbox-daemon.git
```

2. **Create feature branch**:
```bash
git checkout -b feature/new-collector
```

3. **Make changes and test**:
```bash
# Make changes
go test ./...
go build ./...
docker build -t blackbox-daemon:test .
```

4. **Commit and push**:
```bash
git add .
git commit -m "feat: add custom metrics collector

- Implements CustomCollector for application-specific metrics
- Adds configuration options for custom collection
- Updates documentation with usage examples"

git push origin feature/new-collector
```

5. **Create pull request**:
- Describe changes and motivation
- Include test results and documentation updates
- Link to any related issues

### Code Review Guidelines

#### Pull Request Requirements

- [ ] All tests pass (`go test ./...`)
- [ ] Code builds successfully (`go build ./...`)
- [ ] Docker image builds (`docker build .`)
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Commit messages follow conventional format

#### Review Checklist

- [ ] Code follows Go style guidelines
- [ ] Error handling is appropriate
- [ ] Thread safety considerations
- [ ] Performance impact assessed
- [ ] Security implications considered
- [ ] Backward compatibility maintained

### Release Process

1. **Version tagging**:
```bash
git tag v1.1.0
git push upstream v1.1.0
```

2. **Build release artifacts**:
```bash
# Cross-platform builds
make build-all

# Docker images
docker build -t blackbox-daemon:v1.1.0 .
docker tag blackbox-daemon:v1.1.0 blackbox-daemon:latest
```

3. **Update documentation**:
- Version compatibility matrix
- Migration guides for breaking changes
- Deployment manifest updates

## Troubleshooting Development Issues

### Common Build Problems

#### Go Version Mismatch
```bash
# Check Go version
go version

# Update to Go 1.25+
curl -fsSL https://golang.org/dl/go1.25.linux-amd64.tar.gz | sudo tar -xzC /usr/local
export PATH=/usr/local/go/bin:$PATH
```

#### Module Dependencies
```bash
# Clean module cache
go clean -modcache
go mod download

# Update dependencies
go mod tidy
go get -u ./...
```

#### CGO Issues
```bash
# Disable CGO for static builds
CGO_ENABLED=0 go build ./...

# Install build essentials if needed
sudo apt-get install build-essential
```

### Runtime Issues

#### Permission Errors
```bash
# Check capabilities
sudo setcap cap_sys_admin,cap_sys_ptrace+ep ./blackbox-daemon

# Run with sudo (development only)
sudo ./blackbox-daemon
```

#### File System Access
```bash
# Test /proc access
ls -la /proc/
cat /proc/meminfo

# Test /sys access  
ls -la /sys/class/net/
```

This development guide should help you get started with building and extending BlackBox-Daemon. For specific questions, please check the documentation or open an issue on GitHub.
# Build and Test Guide

This guide provides comprehensive instructions for building, testing, and developing the BlackBox Daemon project.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Build System](#build-system)
- [Testing](#testing)
- [Docker](#docker)
- [Development Workflow](#development-workflow)
- [Continuous Integration](#continuous-integration)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### Required

- **Go 1.21+**: The project requires Go 1.21 or later
- **Git**: For version information and dependency management
- **Make**: Build automation (standard on Linux/macOS, installable on Windows)

### Optional Development Tools

```bash
# Install recommended development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
go install github.com/cosmtrek/air@latest
```

### Kubernetes Development (Optional)

If you plan to test Kubernetes integration:

```bash
# For local Kubernetes testing
kubectl # Kubernetes command-line tool
kind # Kubernetes in Docker
minikube # Local Kubernetes cluster
```

## Quick Start

### 1. Clone and Build

```bash
# Clone the repository
git clone https://github.com/verygoodsoftwarecompany/blackbox-daemon.git
cd blackbox-daemon

# Quick build (clean, test, build)
make q
```

### 2. Run the Application

```bash
# Run locally
make run

# Or run the binary directly
./bin/blackbox-daemon
```

### 3. Docker Quick Start

```bash
# Build and test, then create Docker image
make qd

# Run in Docker
make docker-run
```

## Build System

The project uses a comprehensive Makefile with dependency management and proper build pipelines.

### Core Targets

| Target | Description | Dependencies |
|--------|-------------|--------------|
| `build` | Build the application binary | - |
| `test` | Run all tests with coverage | - |
| `clean` | Clean all build artifacts | - |
| `dockerize` | Build Docker image | `test` |

### Target Dependencies

The build system enforces quality gates:

```
dockerize → test → (individual tests)
    ↓
  Docker Image

build → (source files)
    ↓
  Binary Executable

test → (test files + source)
    ↓
  Coverage Report
```

### Advanced Build Targets

```bash
# Multi-platform builds
make build-all  # Linux, macOS, Windows (amd64, arm64)

# Development builds
make dev        # Hot reloading with 'air'
make profile    # Build with profiling support

# Release preparation
make release    # Full release package with all platforms
```

### Build Configuration

The build system supports customization through variables:

```bash
# Custom Docker tag
make dockerize tag=v1.2.3

# Cross-compilation
GOOS=linux GOARCH=arm64 make build

# Custom binary name
APP_NAME=my-daemon make build
```

## Testing

### Test Structure

The project includes comprehensive test coverage across all components:

```
pkg/
├── types/
│   └── telemetry_test.go      # Data structure tests
internal/
├── api/
│   └── server_test.go         # HTTP API tests
├── config/
│   └── config_test.go         # Configuration tests
├── formatter/
│   └── formatter_test.go      # Output formatting tests
├── k8s/
│   └── watcher_test.go        # Kubernetes integration tests
├── metrics/
│   └── collector_test.go      # Prometheus metrics tests
├── ringbuffer/
│   └── ringbuffer_test.go     # Circular buffer tests
└── telemetry/
    └── system_test.go         # System metrics tests
```

### Running Tests

```bash
# Basic test execution
make test                    # Run all tests with coverage
make race                   # Run tests with race detector
make benchmark              # Run performance benchmarks

# Coverage analysis
make coverage               # Generate and display coverage report
# Opens coverage/coverage.html in browser
```

### Test Categories

#### Unit Tests
- **Data Structures**: Type validation, serialization, field requirements
- **Business Logic**: Core algorithms, data processing, validation rules
- **Error Handling**: Edge cases, invalid input, resource constraints

#### Integration Tests
- **HTTP API**: Request/response handling, authentication, error responses
- **Kubernetes**: Event processing, resource watching, cluster integration
- **Metrics**: Prometheus integration, metric collection, HTTP endpoints

#### Concurrency Tests
- **Thread Safety**: Concurrent access to shared resources
- **Race Conditions**: Data races, synchronization issues
- **Performance**: Load testing, stress testing, resource limits

### Test Quality Standards

All tests follow these principles:

1. **Table-Driven Tests**: Multiple test cases in single functions
2. **Proper Isolation**: No shared state between tests
3. **Comprehensive Coverage**: Happy path, edge cases, and error conditions
4. **Mock Usage**: External dependencies are mocked appropriately
5. **Concurrent Testing**: Thread-safe components tested with goroutines

### Current Test Coverage

The project maintains high test coverage across all components:

- **Overall Coverage**: 72.2% (including main.go: 0.0%)
- **Package Coverage**: 91.1% (excluding main.go)
- **Ring Buffer**: 97.7% coverage (comprehensive circular buffer testing)
- **Telemetry System**: 87.8% coverage (Linux system metrics collection)
- **Metrics Collector**: 91.4% coverage (Prometheus integration)
- **Formatter Chain**: 90.2% coverage (multi-format output system)
- **API Server**: 91.5% coverage (REST endpoints and authentication)
- **Configuration**: 89.6% coverage (environment-based configuration)
- **K8s Integration**: 69.4% coverage (pod monitoring with comprehensive mocking)
- **Type Definitions**: 100% coverage (core data structures)

### Example Test Execution

```bash
# Run tests with verbose output
go test -v ./...

# Run specific package tests
go test -v ./internal/api/
go test -v ./internal/k8s/

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run with race detector
go test -race ./...

# Test individual packages with coverage
go test -cover ./internal/k8s      # Kubernetes integration tests
go test -cover ./internal/api      # REST API tests
go test -cover ./internal/metrics  # Prometheus metrics tests
```

**Note**: The K8s package tests use fake Kubernetes clientsets to simulate pod monitoring without requiring a real cluster. This ensures tests run quickly and reliably in CI/CD environments.

## Docker

### Docker Build Process

The Docker build follows these quality gates:

1. **Test Execution**: All tests must pass before building image
2. **Multi-Stage Build**: Optimized for production deployment
3. **Security**: Distroless base image, non-root user
4. **Metadata**: Version, commit, build time embedded

### Docker Commands

```bash
# Standard build (runs tests first)
make dockerize

# Custom tag
make dockerize tag=production-v1.2.3

# Run container locally
make docker-run

# Push to registry
make docker-push

# Clean Docker artifacts
make docker-clean
```

### Docker Development

```bash
# Build development image with debugging tools
docker build -f Dockerfile.dev -t blackbox-daemon:dev .

# Run with volume mounting for development
docker run -v $(pwd):/app blackbox-daemon:dev
```

### Production Deployment

The Docker image is optimized for production:

```yaml
# docker-compose.yml example
version: '3.8'
services:
  blackbox-daemon:
    image: blackbox-daemon:latest
    ports:
      - "8080:8080"
    environment:
      - BLACKBOX_LOG_LEVEL=info
      - BLACKBOX_METRICS_PORT=9090
    volumes:
      - ./config:/config:ro
    restart: unless-stopped
```

## Development Workflow

### Recommended Development Process

1. **Setup Development Environment**
   ```bash
   make deps        # Download dependencies
   make check       # Run code quality checks
   ```

2. **Make Changes**
   ```bash
   # Format code
   make fmt
   
   # Run linting
   make lint
   
   # Run static analysis
   make vet
   ```

3. **Test Changes**
   ```bash
   # Run tests
   make test
   
   # Check race conditions
   make race
   
   # Run security scan
   make sec-scan
   ```

4. **Build and Verify**
   ```bash
   # Quick build and test
   make q
   
   # Full CI pipeline
   make ci
   ```

### Code Quality Gates

The project enforces quality through automated checks:

- **Formatting**: `gofmt` ensures consistent code style
- **Linting**: `golangci-lint` catches common issues
- **Static Analysis**: `go vet` identifies potential bugs
- **Security**: `gosec` scans for security vulnerabilities
- **Testing**: Comprehensive test suite with coverage requirements

### Pre-commit Workflow

Recommended pre-commit checks:

```bash
#!/bin/bash
# .git/hooks/pre-commit

set -e

echo "Running pre-commit checks..."

# Format code
make fmt

# Run all quality checks
make check

# Run tests
make test

# Security scan
make sec-scan

echo "All pre-commit checks passed!"
```

### Hot Reloading Development

For rapid development iteration:

```bash
# Install air for hot reloading
go install github.com/cosmtrek/air@latest

# Start development with hot reloading
make dev
```

Create `.air.toml` for custom hot-reloading configuration:

```toml
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main ./cmd/blackbox-daemon"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "build", "bin"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html"]
  kill_delay = "0s"
  log = "build-errors.log"
  send_interrupt = false
  stop_on_root = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  time = false

[misc]
  clean_on_exit = false
```

## Continuous Integration

### CI Pipeline

The project supports a complete CI/CD pipeline:

```bash
# Full CI pipeline (recommended for CI systems)
make ci

# This runs:
# 1. make clean    - Clean artifacts
# 2. make deps     - Update dependencies
# 3. make check    - Code quality checks
# 4. make test     - Run all tests
# 5. make build    - Build binary
```

### GitHub Actions Example

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v3
      with:
        go-version: '1.21'
    
    - name: Run CI Pipeline
      run: make ci
    
    - name: Upload coverage reports
      uses: codecov/codecov-action@v3
      with:
        file: ./coverage/coverage.out

  docker:
    runs-on: ubuntu-latest
    needs: test
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Build Docker Image
      run: make dockerize
    
    - name: Push Docker Image
      if: github.ref == 'refs/heads/main'
      run: make docker-push
```

### Release Process

```bash
# Prepare release
make release

# This creates:
# - Multi-platform binaries
# - Release package
# - Documentation
# - Checksums
```

## Performance and Monitoring

### Performance Testing

```bash
# Run benchmarks
make benchmark

# Memory profiling
make profile
./bin/blackbox-daemon-profile

# CPU profiling (while running)
go tool pprof http://localhost:8080/debug/pprof/profile
```

### Production Monitoring

The application exposes metrics on `/metrics` endpoint:

```bash
# Check metrics
curl http://localhost:9090/metrics

# Prometheus configuration
# See prometheus.yml for scraping configuration
```

### Health Checks

```bash
# Health endpoint
curl http://localhost:8080/health

# Ready endpoint
curl http://localhost:8080/ready
```

## Troubleshooting

### Common Build Issues

#### Go Version Compatibility
```bash
# Check Go version
go version

# Should be 1.21 or later
# Update if necessary
```

#### Module Download Issues
```bash
# Clear module cache
go clean -modcache

# Re-download dependencies
make deps
```

#### Test Failures
```bash
# Run tests with verbose output
go test -v ./...

# Run specific failing test
go test -v -run TestSpecificFunction ./path/to/package

# Check for race conditions
go test -race ./...
```

### Docker Issues

#### Build Failures
```bash
# Check Docker daemon
docker info

# Clean Docker cache
docker system prune -f

# Rebuild without cache
docker build --no-cache -t blackbox-daemon .
```

#### Container Runtime Issues
```bash
# Check container logs
docker logs <container-id>

# Run with debug output
docker run -it blackbox-daemon /bin/sh
```

### Development Issues

#### Import Path Issues
```bash
# Ensure correct module name in go.mod
module github.com/verygoodsoftwarecompany/blackbox-daemon

# Tidy modules
go mod tidy
```

#### IDE Configuration
For VS Code, use these settings:

```json
{
    "go.useLanguageServer": true,
    "go.lintOnSave": "package",
    "go.vetOnSave": "package",
    "go.formatTool": "gofmt",
    "go.testFlags": ["-v"],
    "go.coverOnSave": true
}
```

### Performance Issues

#### High Memory Usage
```bash
# Check memory leaks
go test -memprofile=mem.prof ./...
go tool pprof mem.prof

# Monitor with tools
top -p $(pgrep blackbox-daemon)
```

#### High CPU Usage
```bash
# CPU profiling
go test -cpuprofile=cpu.prof ./...
go tool pprof cpu.prof

# System monitoring
htop
iostat -x 1
```

### Getting Help

1. **Check Documentation**: Review all files in `docs/` directory
2. **Run Diagnostics**: Use `make env` to check environment
3. **Enable Debug Logging**: Set `LOG_LEVEL=debug`
4. **Check Issues**: Review GitHub issues for similar problems
5. **Create Minimal Reproduction**: Isolate the specific issue

### Debug Configuration

```bash
# Enable debug logging
export BLACKBOX_LOG_LEVEL=debug

# Enable pprof endpoint
export BLACKBOX_PPROF_ENABLED=true

# Run with tracing
./bin/blackbox-daemon --trace
```

This comprehensive build and test guide ensures developers can effectively work with the BlackBox Daemon project, from initial setup through production deployment.
# Architecture Overview

## System Architecture

BlackBox-Daemon follows a modular, event-driven architecture designed for high-performance telemetry collection and incident analysis.

```
┌─────────────────────────────────────────────────────────────────────┐
│                         BlackBox Daemon                             │
├─────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐      │
│  │ System Telemetry│  │ Kubernetes      │  │ REST API        │      │
│  │ Collector       │  │ Pod Watcher     │  │ Server          │      │
│  │                 │  │                 │  │                 │      │
│  │ • CPU Usage     │  │ • Pod Events    │  │ • Sidecar       │      │
│  │ • Memory Stats  │  │ • Crash Detect  │  │   Telemetry     │      │
│  │ • Network I/O   │  │ • OOM Detection │  │ • Incident      │      │
│  │ • Disk I/O      │  │ • Restart Count │  │   Reports       │      │
│  │ • Process Info  │  │                 │  │ • Health Check  │      │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘      │
├─────────────────────────────────────────────────────────────────────┤
│                        Ring Buffer Storage                          │
│                    (Thread-Safe Circular Buffer)                    │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │             60-Second Sliding Window                        │    │
│  │  [Entry1] → [Entry2] → [Entry3] → ... → [EntryN]          │    │
│  │     ↑                                         ↑            │    │
│  │   Oldest                                   Newest          │    │
│  └─────────────────────────────────────────────────────────────┘    │
├─────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐      │
│  │ Incident        │  │ Formatter       │  │ Metrics         │      │
│  │ Handler         │  │ Chain           │  │ Exporter        │      │
│  │                 │  │                 │  │                 │      │
│  │ • Event Correl. │  │ • Default Format│  │ • Prometheus    │      │
│  │ • Context Build │  │ • JSON Format   │  │ • Custom        │      │
│  │ • Report Gen    │  │ • CSV Format    │  │   Metrics       │      │
│  │ • AI Prep       │  │ • Custom Format │  │ • Health Stats  │      │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘      │
└─────────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. System Telemetry Collector
- **Purpose**: Collects comprehensive Linux system metrics
- **Data Sources**: `/proc/*`, `/sys/*` filesystems
- **Collection Rate**: 1 second (configurable)
- **Metrics**: CPU, memory, network, disk, processes, load averages

### 2. Ring Buffer Storage
- **Purpose**: High-performance, bounded memory telemetry storage
- **Implementation**: Thread-safe circular buffer
- **Capacity**: Auto-sized based on window duration and throughput
- **Retention**: 60 seconds sliding window (configurable)

### 3. Kubernetes Pod Watcher  
- **Purpose**: Monitor pods and detect crashes/incidents
- **Integration**: Kubernetes API client with RBAC
- **Detection**: Container crashes, OOM kills, restarts
- **Scope**: Node-local pods only

### 4. REST API Server
- **Purpose**: Accept telemetry from application sidecars
- **Authentication**: Bearer token (API key)
- **Endpoints**: `/api/v1/telemetry`, `/api/v1/incident`, `/api/v1/health`
- **Documentation**: Optional Swagger UI

### 5. Incident Handler
- **Purpose**: Process crashes and generate flight recorder dumps
- **Triggers**: Pod crashes, manual reports, sidecar incidents
- **Output**: Correlated telemetry data via formatter chain

### 6. Formatter Chain
- **Purpose**: Configurable output formatting for incident reports  
- **Formats**: Default (human-readable), JSON, CSV
- **Destinations**: Files, stdout, HTTP endpoints
- **Extensibility**: Plugin architecture for custom formatters

### 7. Prometheus Metrics Exporter
- **Purpose**: Export operational and system metrics
- **Metrics**: System telemetry, buffer stats, incident counts
- **Endpoint**: `/metrics` (standard Prometheus format)
- **Monitoring**: Health, performance, and utilization metrics

## Data Flow

### 1. Telemetry Collection Flow
```
System Metrics → Telemetry Collector → Ring Buffer
Sidecar Data   → REST API           → Ring Buffer  
Pod Events     → Pod Watcher        → Incident Handler
```

### 2. Incident Processing Flow
```
Incident Detected → Get Telemetry Window → Format Data → Output Destinations
      ↑                       ↑                ↑              ↑
  Pod Crash            Ring Buffer      Format Chain    Files/HTTP/etc
  Manual Report        Filter & Query   (JSON/CSV/etc)
  Sidecar Alert
```

### 3. Monitoring Flow
```
System Metrics → Prometheus Exporter → /metrics Endpoint → Monitoring Systems
Buffer Stats   → 
Incident Stats →
```

## Design Principles

### Performance
- **In-Memory Storage**: Ring buffer avoids disk I/O for telemetry storage
- **Bounded Memory**: Fixed-size circular buffer prevents memory exhaustion
- **Efficient Queries**: Optimized time-based filtering and indexing
- **Minimal Overhead**: System collection designed for minimal CPU/memory impact

### Reliability  
- **Thread Safety**: All components use proper synchronization
- **Graceful Shutdown**: Context-based cancellation throughout
- **Error Handling**: Comprehensive error handling with recovery
- **Health Monitoring**: Built-in health checks and self-monitoring

### Scalability
- **Node-Local**: Each daemon instance handles only its node
- **Configurable**: Tunable collection intervals and buffer sizes  
- **Extensible**: Plugin architecture for custom metrics and formatters
- **Resource Limits**: Kubernetes resource constraints prevent runaway usage

### Security
- **RBAC**: Minimal Kubernetes permissions (read-only pods/events)
- **Authentication**: API key authentication for sidecar endpoints
- **Non-Root**: Runs as non-privileged user in containers
- **Input Validation**: Comprehensive validation of all inputs

## Integration Points

### Kubernetes Integration
- **ServiceAccount**: Uses RBAC for secure API access
- **DaemonSet**: Deployed on every node for comprehensive coverage
- **Node Affinity**: Ensures proper scheduling and locality
- **Health Probes**: Liveness and readiness checks

### Application Integration
- **Sidecar Pattern**: Language-agnostic telemetry submission
- **REST API**: Standard HTTP/JSON interface
- **Flexible Schema**: Free-form telemetry data with structured metadata
- **Runtime Specific**: Optimized for JVM, .NET, Go, Python runtimes

### Monitoring Integration
- **Prometheus**: Native metrics export
- **Alerting**: Integration with Prometheus Alertmanager
- **Grafana**: Dashboard templates available
- **Logging**: Structured JSON logging for log aggregation

## Deployment Architecture

### Single Node View
```
┌─────────────────────────────────────────────────────────┐
│                    Kubernetes Node                      │
│                                                         │
│  ┌─────────────────┐  ┌─────────────────┐               │
│  │ Application Pod │  │ Application Pod │               │
│  │ + Sidecar      │  │ + Sidecar      │  ... More Pods │
│  └─────────────────┘  └─────────────────┘               │
│           │                     │                       │
│           └─────────┬───────────┘                       │
│                     ▼                                   │
│  ┌─────────────────────────────────────────────────────┐ │
│  │            BlackBox Daemon                          │ │
│  │         (DaemonSet Instance)                        │ │
│  └─────────────────────────────────────────────────────┘ │
│                     │                                   │
│                     ▼                                   │
│  ┌─────────────────────────────────────────────────────┐ │
│  │               Host System                           │ │
│  │          (/proc, /sys, /dev)                       │ │
│  └─────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

### Cluster-Wide View
```
┌─────────────────────────────────────────────────────────┐
│                  Kubernetes Cluster                     │
│                                                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐      │
│  │   Node 1    │  │   Node 2    │  │   Node N    │      │  
│  │ BlackBox    │  │ BlackBox    │  │ BlackBox    │      │
│  │ Daemon      │  │ Daemon      │  │ Daemon      │      │
│  └─────────────┘  └─────────────┘  └─────────────┘      │
│         │               │               │               │
│         └───────────────┼───────────────┘               │
│                         ▼                               │
│  ┌─────────────────────────────────────────────────────┐ │
│  │          Prometheus / Monitoring Stack              │ │
│  │                                                     │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │ │
│  │  │ Prometheus  │  │  Grafana    │  │ AlertManager│  │ │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  │ │
│  └─────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

## Configuration Architecture

### Environment-Based Configuration
```
┌─────────────────────────────────────────────────────────┐
│               Configuration Sources                      │
│                                                         │
│  ┌─────────────────┐  ┌─────────────────┐               │
│  │  Environment    │  │    Defaults     │               │
│  │  Variables      │  │                 │               │
│  │                 │  │ • Buffer: 60s   │               │
│  │ • API_KEY       │  │ • Interval: 1s  │               │
│  │ • BUFFER_SIZE   │  │ • API Port:8080 │               │
│  │ • LOG_LEVEL     │  │ • Metrics:9090  │               │
│  │ • NODE_NAME     │  │                 │               │
│  └─────────────────┘  └─────────────────┘               │
│           │                     │                       │
│           └─────────┬───────────┘                       │
│                     ▼                                   │
│  ┌─────────────────────────────────────────────────────┐ │
│  │            Configuration Loader                     │ │
│  │                                                     │ │
│  │  • Parse & Validate                                 │ │
│  │  • Type Conversion                                  │ │
│  │  • Error Handling                                   │ │  
│  └─────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```
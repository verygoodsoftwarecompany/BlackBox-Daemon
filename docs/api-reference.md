# BlackBox-Daemon API Reference

This document provides comprehensive documentation for the BlackBox-Daemon REST API.

## Base Information

- **Base URL**: `http://localhost:8080/api/v1`
- **Authentication**: Bearer Token (API Key)
- **Content Type**: `application/json`
- **API Version**: v1

## Authentication

All API endpoints require authentication using a Bearer token in the Authorization header:

```http
Authorization: Bearer <your-api-key>
```

The API key is configured via the `BLACKBOX_API_KEY` environment variable.

## Endpoints

### 1. Health Check

Check daemon health and status.

```http
GET /health
```

#### Response

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": "2h15m30s",
  "components": {
    "ring_buffer": "healthy",
    "system_collector": "healthy", 
    "pod_watcher": "healthy",
    "metrics_collector": "healthy"
  }
}
```

#### Status Codes

- `200 OK`: Service is healthy
- `503 Service Unavailable`: Service is unhealthy

### 2. Submit Telemetry Data

Submit runtime telemetry from application sidecars.

```http
POST /api/v1/telemetry
```

#### Request Body

```json
{
  "pod_name": "my-app-pod-abc123",
  "namespace": "production",
  "container_id": "docker://1234567890abcdef...",
  "runtime": "jvm",
  "timestamp": "2024-11-02T15:04:05Z",
  "data": {
    "heap_used": 1073741824,
    "heap_max": 2147483648,
    "gc_collections": 15,
    "gc_time_ms": 250,
    "threads_active": 32,
    "threads_daemon": 8,
    "cpu_usage_percent": 45.7,
    "custom_metric": 42.5
  },
  "tags": {
    "environment": "production",
    "service": "user-service",
    "version": "2.1.0"
  }
}
```

#### Field Descriptions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `pod_name` | string | Yes | Kubernetes pod name |
| `namespace` | string | Yes | Kubernetes namespace |
| `container_id` | string | No | Docker/containerd container ID |
| `runtime` | string | Yes | Runtime type (jvm, nodejs, python, etc.) |
| `timestamp` | string | No | ISO8601 timestamp (defaults to current time) |
| `data` | object | Yes | Runtime-specific telemetry metrics |
| `tags` | object | No | Additional metadata tags |

#### Runtime Types

Supported runtime identifiers:

- `jvm`: Java Virtual Machine applications
- `nodejs`: Node.js applications  
- `python`: Python applications
- `dotnet`: .NET Core applications
- `go`: Go applications
- `ruby`: Ruby applications
- `custom`: Custom runtime implementations

#### Response

**Success (201 Created)**
```json
{
  "status": "accepted",
  "entry_id": "entry_1699012345_001",
  "timestamp": "2024-11-02T15:04:05Z"
}
```

**Error (400 Bad Request)**
```json
{
  "error": "validation_failed",
  "message": "Missing required field: pod_name",
  "details": {
    "field": "pod_name",
    "code": "required"
  }
}
```

#### Status Codes

- `201 Created`: Telemetry accepted and stored
- `400 Bad Request`: Invalid request body or missing fields
- `401 Unauthorized`: Missing or invalid API key
- `413 Payload Too Large`: Request body exceeds size limit
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Server error

### 3. Report Incident

Report application crashes, errors, or other incidents.

```http
POST /api/v1/incident
```

#### Request Body

```json
{
  "pod_name": "my-app-pod-abc123",
  "namespace": "production",
  "container_id": "docker://1234567890abcdef...",
  "severity": "critical",
  "type": "crash",
  "message": "Application crashed with OutOfMemoryError: Java heap space",
  "timestamp": "2024-11-02T15:04:05Z",
  "metadata": {
    "exception_class": "java.lang.OutOfMemoryError",
    "stack_trace": "java.lang.OutOfMemoryError: Java heap space\n\tat ...",
    "heap_usage": "98%",
    "gc_attempts": 15
  },
  "tags": {
    "component": "user-processor",
    "operation": "batch-import"
  }
}
```

#### Field Descriptions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `pod_name` | string | Yes | Kubernetes pod name |
| `namespace` | string | Yes | Kubernetes namespace |
| `container_id` | string | No | Container identifier |
| `severity` | string | Yes | Incident severity level |
| `type` | string | Yes | Type of incident |
| `message` | string | Yes | Human-readable incident description |
| `timestamp` | string | No | When the incident occurred |
| `metadata` | object | No | Additional incident context |
| `tags` | object | No | Classification tags |

#### Severity Levels

- `low`: Minor issues, warnings
- `medium`: Recoverable errors, degraded performance
- `high`: Significant errors, service interruptions
- `critical`: System failures, data loss, security breaches

#### Incident Types

- `crash`: Application or process crashes
- `oom`: Out of memory conditions
- `timeout`: Request or operation timeouts
- `error`: General application errors
- `degraded`: Performance degradation
- `security`: Security-related incidents

#### Response

**Success (201 Created)**
```json
{
  "status": "reported",
  "incident_id": "incident_1699012345_001",
  "timestamp": "2024-11-02T15:04:05Z"
}
```

#### Status Codes

- `201 Created`: Incident reported and recorded
- `400 Bad Request`: Invalid request format
- `401 Unauthorized`: Authentication required
- `500 Internal Server Error`: Server error

### 4. Get Buffer Status

Retrieve ring buffer status and statistics.

```http
GET /api/v1/buffer/status
```

#### Response

```json
{
  "window_size": "60s",
  "current_entries": 1250,
  "max_entries": 3600,
  "utilization_percent": 34.7,
  "oldest_entry": "2024-11-02T15:03:05Z",
  "newest_entry": "2024-11-02T15:04:05Z",
  "memory_usage_bytes": 2567890,
  "cleanup_runs": 45,
  "last_cleanup": "2024-11-02T15:03:30Z"
}
```

#### Status Codes

- `200 OK`: Status retrieved successfully
- `401 Unauthorized`: Authentication required
- `500 Internal Server Error`: Server error

### 5. Export Telemetry Data

Export telemetry data from the buffer for analysis.

```http
GET /api/v1/export?format={format}&since={timestamp}&until={timestamp}&pod={pod_name}
```

#### Query Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `format` | No | Export format (json, csv, default: json) |
| `since` | No | Start timestamp (ISO8601) |
| `until` | No | End timestamp (ISO8601) |
| `pod` | No | Filter by pod name |
| `namespace` | No | Filter by namespace |
| `severity` | No | Filter by incident severity |

#### Response Formats

**JSON Format**
```json
{
  "export_info": {
    "timestamp": "2024-11-02T15:04:05Z",
    "total_entries": 1250,
    "time_range": {
      "start": "2024-11-02T15:03:05Z", 
      "end": "2024-11-02T15:04:05Z"
    }
  },
  "entries": [
    {
      "id": "entry_1699012345_001",
      "timestamp": "2024-11-02T15:04:05Z",
      "type": "sidecar_telemetry",
      "pod_name": "my-app-pod",
      "namespace": "production",
      "data": { "..." }
    }
  ]
}
```

**CSV Format**
```csv
timestamp,type,pod_name,namespace,runtime,data_json
2024-11-02T15:04:05Z,sidecar_telemetry,my-app-pod,production,jvm,"{""heap_used"":1073741824}"
```

#### Status Codes

- `200 OK`: Export successful
- `400 Bad Request`: Invalid query parameters
- `401 Unauthorized`: Authentication required
- `404 Not Found`: No data matching criteria
- `500 Internal Server Error`: Server error

## Error Handling

### Error Response Format

All error responses follow a consistent format:

```json
{
  "error": "error_code",
  "message": "Human readable error description",
  "details": {
    "field": "field_name",
    "code": "validation_code"
  },
  "timestamp": "2024-11-02T15:04:05Z",
  "request_id": "req_1699012345_abc123"
}
```

### Common Error Codes

| Code | Description |
|------|-------------|
| `authentication_required` | Missing Authorization header |
| `invalid_api_key` | API key is invalid or expired |
| `validation_failed` | Request body validation failed |
| `rate_limit_exceeded` | Too many requests from client |
| `payload_too_large` | Request body exceeds size limits |
| `internal_error` | Unexpected server error |
| `service_unavailable` | Service is temporarily unavailable |

## Rate Limiting

The API implements rate limiting to prevent abuse:

- **Telemetry Endpoint**: 1000 requests per minute per API key
- **Incident Endpoint**: 100 requests per minute per API key  
- **Export Endpoint**: 10 requests per minute per API key
- **Other Endpoints**: 300 requests per minute per API key

Rate limit headers are included in responses:

```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 995
X-RateLimit-Reset: 1699012405
```

## Usage Examples

### Java Sidecar Integration

```java
// Java example using HTTP client
public class BlackBoxSidecar {
    private final String apiUrl = "http://localhost:8080/api/v1";
    private final String apiKey = System.getenv("BLACKBOX_API_KEY");
    
    public void submitTelemetry(JVMTelemetry telemetry) {
        var request = TelemetryRequest.builder()
            .podName(System.getenv("HOSTNAME"))
            .namespace(System.getenv("POD_NAMESPACE"))
            .runtime("jvm")
            .data(telemetry.toMap())
            .build();
            
        // Submit via HTTP POST
        httpClient.post(apiUrl + "/telemetry", request, apiKey);
    }
}
```

### Node.js Sidecar Integration

```javascript
// Node.js example
const axios = require('axios');

class BlackBoxSidecar {
    constructor() {
        this.apiUrl = 'http://localhost:8080/api/v1';
        this.apiKey = process.env.BLACKBOX_API_KEY;
    }
    
    async submitTelemetry(telemetryData) {
        try {
            const response = await axios.post(`${this.apiUrl}/telemetry`, {
                pod_name: process.env.HOSTNAME,
                namespace: process.env.POD_NAMESPACE,
                runtime: 'nodejs',
                data: telemetryData
            }, {
                headers: {
                    'Authorization': `Bearer ${this.apiKey}`,
                    'Content-Type': 'application/json'
                }
            });
            
            console.log('Telemetry submitted:', response.data);
        } catch (error) {
            console.error('Failed to submit telemetry:', error.response?.data);
        }
    }
}
```

### Python Sidecar Integration

```python
import requests
import os
import json
from datetime import datetime

class BlackBoxSidecar:
    def __init__(self):
        self.api_url = 'http://localhost:8080/api/v1'
        self.api_key = os.getenv('BLACKBOX_API_KEY')
        self.headers = {
            'Authorization': f'Bearer {self.api_key}',
            'Content-Type': 'application/json'
        }
    
    def submit_telemetry(self, telemetry_data):
        payload = {
            'pod_name': os.getenv('HOSTNAME'),
            'namespace': os.getenv('POD_NAMESPACE'),
            'runtime': 'python',
            'timestamp': datetime.utcnow().isoformat() + 'Z',
            'data': telemetry_data
        }
        
        try:
            response = requests.post(
                f'{self.api_url}/telemetry',
                json=payload,
                headers=self.headers
            )
            response.raise_for_status()
            print(f'Telemetry submitted: {response.json()}')
        except requests.exceptions.RequestException as e:
            print(f'Failed to submit telemetry: {e}')
```

## OpenAPI/Swagger

When `BLACKBOX_SWAGGER_ENABLE=true`, the full OpenAPI specification is available at:

```
http://localhost:8080/swagger/doc.json
http://localhost:8080/swagger/index.html
```

This provides an interactive API explorer and complete schema definitions.
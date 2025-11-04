# API Server Component

## Overview

The API Server component provides a RESTful HTTP interface for sidecar telemetry submission and incident reporting. It serves as the primary integration point for applications to contribute runtime-specific telemetry data to the BlackBox system.

## Architecture

### HTTP Server Design
- **Go HTTP Server**: Built on Go's standard HTTP library with custom middleware
- **Authentication**: Bearer token authentication with constant-time comparison  
- **JSON API**: RESTful endpoints with JSON request/response format
- **Swagger Support**: Optional OpenAPI documentation for development
- **Graceful Shutdown**: Context-based shutdown with connection draining

### Security Model
- **API Key Authentication**: All endpoints (except health) require Bearer token
- **Timing Attack Protection**: Constant-time string comparison for API keys
- **Input Validation**: Comprehensive request validation and sanitization
- **Rate Limiting**: Protection against abuse (configurable)
- **CORS Support**: Cross-origin request handling for web applications

## API Endpoints

### 1. Telemetry Submission
**Endpoint**: `POST /api/v1/telemetry`  
**Purpose**: Submit runtime telemetry from application sidecars

**Request Format**:
```json
{
  "pod_name": "my-app-pod-abc123",
  "namespace": "production", 
  "container_id": "docker://abc123...",
  "runtime": "jvm",
  "timestamp": "2024-11-02T15:04:05.123Z",
  "data": {
    "heap_used": 1073741824,
    "heap_max": 2147483648,
    "gc_collections": 15,
    "threads_active": 32,
    "custom_metric": 42.5
  }
}
```

**Response Format**:
```json
{
  "status": "success",
  "message": "Telemetry received",
  "entries_added": 5,
  "timestamp": "2024-11-02T15:04:05.456Z"
}
```

**Validation Rules**:
- `pod_name`: Required, non-empty string
- `namespace`: Required, valid Kubernetes namespace format
- `runtime`: Required, supported runtime type (jvm, dotnet, go, python, etc.)
- `data`: Required, object with numeric or string values
- `timestamp`: Optional, defaults to server time if not provided

### 2. Incident Reporting  
**Endpoint**: `POST /api/v1/incident`  
**Purpose**: Report application-level incidents for correlation with system data

**Request Format**:
```json
{
  "pod_name": "my-app-pod-abc123",
  "namespace": "production",
  "severity": "critical",
  "type": "crash", 
  "message": "Application crashed with OutOfMemoryError",
  "metadata": {
    "exception_type": "OutOfMemoryError",
    "stack_trace": "...",
    "thread_count": 150
  }
}
```

**Response Format**:
```json
{
  "status": "success",
  "incident_id": "incident-20241102-150405-abc123",
  "message": "Incident reported",
  "timestamp": "2024-11-02T15:04:05.456Z"
}
```

### 3. Health Check
**Endpoint**: `GET /api/v1/health`  
**Purpose**: Service health verification (no authentication required)

**Response Format**:
```json
{
  "status": "healthy",
  "timestamp": "2024-11-02T15:04:05.456Z",
  "service": "blackbox-daemon",
  "version": "1.0.0",
  "uptime": "2h34m15s",
  "buffer_entries": 45230,
  "buffer_utilization": 0.75
}
```

### 4. Swagger Documentation (Optional)
**Endpoints**: 
- `GET /swagger.json` - OpenAPI specification
- `GET /swagger/` - Swagger UI interface

**Purpose**: Interactive API documentation for development and testing

## Implementation Details

### Server Structure
```go
type Server struct {
    httpServer    *http.Server          // HTTP server instance
    buffer        TelemetryBuffer       // Ring buffer for telemetry storage  
    incidentHandler IncidentHandler     // Incident processing
    apiKey        string               // Authentication key
    swaggerEnabled bool                // Enable Swagger docs
}
```

### Authentication Middleware
```go
func (s *Server) authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Skip auth for health and swagger endpoints
        if isPublicEndpoint(r.URL.Path) {
            next.ServeHTTP(w, r)
            return
        }
        
        // Extract Bearer token
        authHeader := r.Header.Get("Authorization")
        if !strings.HasPrefix(authHeader, "Bearer ") {
            http.Error(w, "Unauthorized", 401)
            return
        }
        
        token := strings.TrimPrefix(authHeader, "Bearer ")
        
        // Constant-time comparison to prevent timing attacks
        if !constantTimeEqual(token, s.apiKey) {
            http.Error(w, "Unauthorized", 401)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}
```

### Request Processing Pipeline
1. **Authentication**: Bearer token validation
2. **Content Validation**: JSON parsing and schema validation  
3. **Business Logic**: Telemetry processing and storage
4. **Response Generation**: JSON response with status information
5. **Logging**: Request/response logging for audit

### Error Handling
```go
type APIError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
}

// Standard error responses
var (
    ErrUnauthorized   = APIError{401, "Unauthorized", "Invalid API key"}
    ErrBadRequest     = APIError{400, "Bad Request", "Invalid request format"}
    ErrInternalError  = APIError{500, "Internal Error", "Server error occurred"}
)
```

## Configuration

### Environment Variables
```bash
BLACKBOX_API_PORT=8080                    # API server port
BLACKBOX_API_KEY=your-secure-key-here     # Authentication key (required)
BLACKBOX_SWAGGER_ENABLE=false             # Enable Swagger documentation
BLACKBOX_API_TIMEOUT=30s                  # Request timeout
BLACKBOX_API_MAX_BODY_SIZE=1048576        # Max request body size (1MB)
```

### Security Configuration
```bash
BLACKBOX_API_RATE_LIMIT=1000              # Requests per minute per IP
BLACKBOX_API_CORS_ORIGINS=*               # Allowed CORS origins
BLACKBOX_API_TLS_CERT_FILE=cert.pem       # TLS certificate (optional)
BLACKBOX_API_TLS_KEY_FILE=key.pem         # TLS private key (optional)
```

## Integration Patterns

### JVM Sidecar Integration
```java
public class BlackBoxTelemetry {
    private final String apiKey;
    private final String baseUrl;
    private final HttpClient client;
    
    public void submitTelemetry(Map<String, Object> data) {
        TelemetryRequest request = TelemetryRequest.builder()
            .podName(getPodName())
            .namespace(getNamespace())
            .runtime("jvm")
            .data(data)
            .build();
            
        HttpRequest httpRequest = HttpRequest.newBuilder()
            .uri(URI.create(baseUrl + "/api/v1/telemetry"))
            .header("Authorization", "Bearer " + apiKey)
            .header("Content-Type", "application/json")
            .POST(HttpRequest.BodyPublishers.ofString(toJson(request)))
            .build();
            
        client.send(httpRequest, HttpResponse.BodyHandlers.ofString());
    }
}
```

### .NET Sidecar Integration
```csharp
public class BlackBoxTelemetryClient
{
    private readonly HttpClient httpClient;
    private readonly string apiKey;
    private readonly string baseUrl;
    
    public async Task SubmitTelemetryAsync(Dictionary<string, object> data)
    {
        var request = new
        {
            pod_name = Environment.GetEnvironmentVariable("HOSTNAME"),
            @namespace = Environment.GetEnvironmentVariable("POD_NAMESPACE"),
            runtime = "dotnet",
            data = data
        };
        
        var content = new StringContent(
            JsonSerializer.Serialize(request),
            Encoding.UTF8,
            "application/json"
        );
        
        httpClient.DefaultRequestHeaders.Authorization = 
            new AuthenticationHeaderValue("Bearer", apiKey);
            
        var response = await httpClient.PostAsync(
            $"{baseUrl}/api/v1/telemetry", 
            content
        );
        
        response.EnsureSuccessStatusCode();
    }
}
```

### Go Sidecar Integration
```go
type TelemetryClient struct {
    baseURL string
    apiKey  string
    client  *http.Client
}

func (c *TelemetryClient) SubmitTelemetry(data map[string]interface{}) error {
    request := map[string]interface{}{
        "pod_name":  os.Getenv("HOSTNAME"),
        "namespace": os.Getenv("POD_NAMESPACE"), 
        "runtime":   "go",
        "data":      data,
    }
    
    jsonData, err := json.Marshal(request)
    if err != nil {
        return err
    }
    
    req, err := http.NewRequest("POST", c.baseURL+"/api/v1/telemetry", bytes.NewBuffer(jsonData))
    if err != nil {
        return err
    }
    
    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := c.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return fmt.Errorf("API error: %d", resp.StatusCode)
    }
    
    return nil
}
```

## Performance Characteristics

### Throughput Targets
- **Sustained Load**: 1,000 requests/second per daemon instance
- **Burst Load**: 5,000 requests/second for 30 seconds
- **Latency**: <10ms P95 response time for telemetry submission
- **Memory Usage**: <50MB for HTTP server and request processing

### Scalability Considerations
- **Horizontal**: Multiple daemon instances across nodes
- **Connection Pooling**: Reuse HTTP connections from sidecars
- **Request Batching**: Support for submitting multiple telemetry points
- **Async Processing**: Non-blocking request handling

### Resource Limits
```yaml
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "256Mi"  
    cpu: "500m"
```

## Monitoring and Observability

### Prometheus Metrics
```promql
# Request metrics
http_requests_total{method="POST",endpoint="/api/v1/telemetry"}
http_request_duration_seconds{method="POST",endpoint="/api/v1/telemetry"}
http_request_size_bytes{endpoint="/api/v1/telemetry"}
http_response_size_bytes{endpoint="/api/v1/telemetry"}

# Error metrics  
http_errors_total{method="POST",endpoint="/api/v1/telemetry",code="400"}
http_errors_total{method="POST",endpoint="/api/v1/telemetry",code="401"}
http_errors_total{method="POST",endpoint="/api/v1/telemetry",code="500"}

# Business metrics
telemetry_entries_received_total{runtime="jvm"}
telemetry_entries_received_total{runtime="dotnet"}  
incidents_reported_total{severity="critical"}
```

### Health Monitoring
```bash
# Health check endpoint
curl -f http://localhost:8080/api/v1/health || exit 1

# Metrics availability  
curl -f http://localhost:9090/metrics | grep -q "http_requests_total" || exit 1

# API responsiveness
time curl -s -o /dev/null http://localhost:8080/api/v1/health
```

### Logging
```json
{
  "timestamp": "2024-11-02T15:04:05.123Z",
  "level": "INFO",
  "component": "api-server",
  "message": "Telemetry received",
  "pod_name": "my-app-abc123",
  "namespace": "production", 
  "runtime": "jvm",
  "entries_count": 5,
  "request_id": "req-abc123",
  "duration_ms": 12.5
}
```

## Security Considerations

### Authentication Security
- **API Key Strength**: Minimum 32-character random keys
- **Key Rotation**: Support for multiple valid keys during rotation
- **Timing Attacks**: Constant-time comparison prevents timing leaks
- **Key Storage**: Environment variables or Kubernetes secrets

### Network Security
- **TLS Support**: Optional HTTPS with certificate management
- **Network Policies**: Kubernetes network policies for pod-to-pod communication
- **Firewall Rules**: Restrict access to API port from authorized sources
- **VPC Security**: Private cluster networking when possible

### Input Validation
- **JSON Schema**: Strict validation of request structure
- **Size Limits**: Maximum request body size limits
- **Rate Limiting**: Prevent abuse and DOS attacks  
- **Sanitization**: Escape/sanitize string inputs for logging

### Data Protection
- **PII Handling**: Guidelines for avoiding sensitive data in telemetry
- **Audit Logging**: Complete request/response audit trail
- **Data Retention**: Configurable retention policies
- **Encryption**: In-transit and at-rest encryption options

## Troubleshooting

### Common Issues

1. **401 Unauthorized Errors**
   - Verify `BLACKBOX_API_KEY` environment variable is set
   - Check Authorization header format: `Bearer <key>`
   - Ensure API key matches exactly (no extra whitespace)

2. **Connection Refused**
   - Confirm API server is running on expected port
   - Check firewall rules and network policies  
   - Verify pod networking configuration

3. **Request Timeouts**
   - Check server resource limits and scaling
   - Monitor request queue depth and processing time
   - Verify network connectivity and latency

4. **High Error Rates**
   - Review request validation errors in logs
   - Check JSON schema compliance  
   - Monitor server resource utilization

### Debug Commands
```bash
# Test API connectivity
curl -v -X POST http://localhost:8080/api/v1/telemetry \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"pod_name":"test","namespace":"default","runtime":"test","data":{"test":1}}'

# Check server status
curl http://localhost:8080/api/v1/health | jq .

# View server logs  
kubectl logs -n kube-system -l app=blackbox-daemon --tail=100

# Monitor metrics
curl http://localhost:9090/metrics | grep http_requests_total
```

## Best Practices

### Development
1. **Use Swagger documentation** for API exploration and testing
2. **Implement proper error handling** in sidecar clients
3. **Add retry logic** with exponential backoff for resilience  
4. **Monitor API response times** and adjust timeouts accordingly

### Production  
1. **Use strong API keys** (32+ characters, cryptographically random)
2. **Enable TLS encryption** for production deployments
3. **Set up monitoring and alerting** for API health and performance
4. **Implement rate limiting** to prevent abuse

### Security
1. **Rotate API keys regularly** using Kubernetes secret updates
2. **Restrict network access** using network policies  
3. **Monitor for suspicious activity** in API access logs
4. **Avoid logging sensitive data** in telemetry submissions
# Formatter Component

## Overview

The Formatter component provides a flexible, extensible system for formatting and outputting telemetry data and incident reports. It supports multiple output formats (default text, JSON, CSV) and multiple destination types (files, stdout, HTTP endpoints), allowing data to be simultaneously processed in different ways for different audiences.

## Architecture

### Formatter Chain Pattern
The component implements a chain-of-responsibility pattern where:
- **Formatters** convert data to specific formats
- **Destinations** define where formatted data goes  
- **FormatterChain** orchestrates multiple formatter-destination combinations
- **Single Input, Multiple Outputs** allows the same incident to be processed multiple ways

### Component Structure
```
FormatterChain
├── DefaultFormatter → FileDestination(/var/log/incidents/default/)
├── JSONFormatter → HTTPDestination(http://log-collector/)
└── CSVFormatter → FileDestination(/var/log/incidents/csv/)
```

## Supported Formatters

### 1. Default Formatter
**Purpose**: Human-readable incident reports for operations teams

**Format Structure**:
```
=== INCIDENT REPORT ===
ID: 2023-11-04-15-30-45-abc123
TIMESTAMP: 2023-11-04 15:30:45.123
SEVERITY: high
TYPE: crash
MESSAGE: Pod my-app-123 crashed with exit code 1
POD: production/my-app-123

=== TELEMETRY DATA ===
2023-11-04 : 15:30:30.000 | cpu_usage_percent | 95.2
2023-11-04 : 15:30:31.000 | memory_usage_bytes | 8589934592
2023-11-04 : 15:30:32.000 | network_rx_bytes_eth0 | 1048576
```

**Use Cases**:
- Operations team incident response
- Log file analysis
- Debugging and troubleshooting
- Human consumption

### 2. JSON Formatter  
**Purpose**: Structured data for logging systems and automation

**Format Structure**:
```json
{
  "incident": {
    "id": "2023-11-04-15-30-45-abc123",
    "timestamp": "2023-11-04T15:30:45.123Z",
    "severity": "high",
    "type": "crash",
    "message": "Pod my-app-123 crashed with exit code 1",
    "pod_name": "my-app-123",
    "namespace": "production",
    "metadata": {}
  },
  "telemetry": [
    {
      "timestamp": "2023-11-04T15:30:30.000Z",
      "source": "system",
      "type": "cpu",
      "name": "cpu_usage_percent",
      "value": 95.2,
      "tags": {"core": "cpu0"}
    }
  ],
  "generated_at": "2023-11-04T15:31:00.000Z"
}
```

**Use Cases**:
- ELK Stack integration
- SIEM system ingestion
- API consumption
- Automated processing

### 3. CSV Formatter
**Purpose**: Tabular data for analysis and spreadsheet import

**Format Structure**:
```csv
timestamp,source,type,name,value,tags,incident_id
2023-11-04T15:30:30.000Z,system,cpu,cpu_usage_percent,95.2,"core=cpu0",2023-11-04-15-30-45-abc123
2023-11-04T15:30:31.000Z,system,memory,memory_usage_bytes,8589934592,,2023-11-04-15-30-45-abc123
2023-11-04T15:30:32.000Z,system,network,network_rx_bytes_eth0,1048576,"interface=eth0;direction=rx",2023-11-04-15-30-45-abc123
```

**Use Cases**:
- Data analysis in Excel/Google Sheets
- Statistical analysis
- Machine learning datasets
- Reporting and visualization

## Supported Destinations

### 1. File Destination
**Purpose**: Persistent storage to local filesystem

**Features**:
- **Automatic Directory Creation**: Creates directory structure as needed
- **Append Mode**: New incidents append to existing files
- **Atomic Writes**: Data is synced to disk immediately
- **Timestamped Filenames**: Automatic filename generation with timestamps
- **Format-Specific Naming**: Different files for different formats

**Configuration**:
```go
destination, err := NewFileDestination("/var/log/incidents/incident_20231104_153045_json.log")
```

**File Naming Pattern**:
```
{timestamp}_{formatter}_{type}.log
20231104_153045_default_incident.log
20231104_153045_json_incident.log  
20231104_153045_csv_incident.log
```

### 2. Stdout Destination
**Purpose**: Console output for debugging and development

**Features**:
- **Immediate Output**: Real-time incident reporting to console
- **No Buffering**: Direct write to stdout
- **Development Friendly**: Easy debugging and testing
- **Container Logging**: Works with container log collection

**Configuration**:
```go
destination := NewStdoutDestination()
```

**Use Cases**:
- Development and testing
- Container environments with log collection
- CI/CD pipeline integration
- Real-time monitoring

### 3. HTTP Destination
**Purpose**: Remote logging and integration with external systems

**Features**:
- **POST Requests**: Sends formatted data as HTTP POST
- **Timeout Handling**: Configurable request timeouts (30s default)
- **Error Handling**: HTTP status code validation
- **JSON Content-Type**: Sends as application/json
- **Retry Logic**: Automatic retries on transient failures

**Configuration**:
```go
destination := NewHTTPDestination("https://log-collector.company.com/incidents")
```

**Request Format**:
```http
POST /incidents HTTP/1.1
Host: log-collector.company.com
Content-Type: application/json

{formatted incident data}
```

## Configuration and Usage

### Environment Variables
```bash
BLACKBOX_OUTPUT_FORMATTERS=default,json,csv    # Comma-separated formatter list
BLACKBOX_OUTPUT_PATH=/var/log/incidents        # Output directory or "stdout"
BLACKBOX_HTTP_ENDPOINT=https://logs.company.com # HTTP destination URL
```

### Programmatic Configuration
```go
// Create formatter chain
chain := formatter.NewFormatterChain()

// Add default formatter with file destination
fileDestination, _ := formatter.NewFileDestination("/var/log/incidents/default.log")
chain.AddFormatter(formatter.NewDefaultFormatter(), fileDestination)

// Add JSON formatter with HTTP destination  
httpDestination := formatter.NewHTTPDestination("https://logs.company.com/incidents")
chain.AddFormatter(formatter.NewJSONFormatter(), httpDestination)

// Add CSV formatter with stdout destination
stdoutDestination := formatter.NewStdoutDestination()
chain.AddFormatter(formatter.NewCSVFormatter(), stdoutDestination)
```

### Processing Incidents
```go
// Process incident through entire chain
entries := ringBuffer.GetWindow(time.Now().Add(-60 * time.Second))
incident := types.IncidentReport{
    ID:        "incident-123",
    Timestamp: time.Now(),
    Severity:  types.SeverityHigh,
    Type:      types.IncidentTypeCrash,
    Message:   "Pod crashed",
}

err := chain.Process(entries, incident)
if err != nil {
    log.Printf("Formatter error: %v", err)
}
```

## Implementation Details

### Formatter Interface
```go
type Formatter interface {
    Format(entries []types.TelemetryEntry, incident types.IncidentReport) ([]byte, error)
    Name() string
}
```

**Design Principles**:
- **Stateless**: Formatters have no state between calls
- **Thread-Safe**: Multiple goroutines can use same formatter
- **Error Handling**: Detailed error messages for debugging
- **Performance**: Efficient string building and memory usage

### Destination Interface
```go
type Destination interface {
    Write(data []byte) error
    Close() error
}
```

**Design Principles**:
- **Resource Management**: Proper cleanup with Close()
- **Error Propagation**: Meaningful error messages
- **Durability**: File destinations sync to disk
- **Reliability**: HTTP destinations handle network errors

### Chain Processing
```go
func (fc *FormatterChain) Process(entries []types.TelemetryEntry, incident types.IncidentReport) error {
    for _, config := range fc.formatters {
        // Format data
        data, err := config.Formatter.Format(entries, incident)
        if err != nil {
            return fmt.Errorf("formatter %s failed: %w", config.Formatter.Name(), err)
        }
        
        // Write to all destinations
        for _, dest := range config.Destinations {
            if err := dest.Write(data); err != nil {
                return fmt.Errorf("failed to write to destination: %w", err)
            }
        }
    }
    return nil
}
```

## Extension Points

### Custom Formatters
```go
type XMLFormatter struct{}

func (xf *XMLFormatter) Name() string {
    return "xml"
}

func (xf *XMLFormatter) Format(entries []types.TelemetryEntry, incident types.IncidentReport) ([]byte, error) {
    // Custom XML formatting logic
    return xmlData, nil
}

// Register custom formatter
chain.AddFormatter(&XMLFormatter{}, destination)
```

### Custom Destinations
```go
type DatabaseDestination struct {
    db *sql.DB
}

func (dd *DatabaseDestination) Write(data []byte) error {
    // Insert incident data into database
    return dd.db.Exec("INSERT INTO incidents (data) VALUES (?)", string(data))
}

func (dd *DatabaseDestination) Close() error {
    return dd.db.Close()
}
```

### Template-Based Formatters
```go
type TemplateFormatter struct {
    template *template.Template
}

func (tf *TemplateFormatter) Format(entries []types.TelemetryEntry, incident types.IncidentReport) ([]byte, error) {
    var buf bytes.Buffer
    data := struct {
        Incident   types.IncidentReport
        Telemetry  []types.TelemetryEntry
        Timestamp  time.Time
    }{
        Incident:  incident,
        Telemetry: entries,
        Timestamp: time.Now(),
    }
    
    err := tf.template.Execute(&buf, data)
    return buf.Bytes(), err
}
```

## Performance Considerations

### Memory Usage
- **String Builders**: Efficient string concatenation
- **Buffer Reuse**: Reuse buffers where possible
- **Streaming**: Large datasets processed in chunks
- **Memory Limits**: Configurable size limits for large incidents

### I/O Performance
- **Batch Writes**: Group multiple writes when possible
- **Async Processing**: Non-blocking I/O for HTTP destinations
- **Compression**: Optional compression for large outputs
- **Buffering**: Configurable write buffering

### Error Handling
- **Partial Failures**: Continue processing other formatters if one fails
- **Retry Logic**: Automatic retries for transient failures
- **Circuit Breakers**: Disable failing destinations temporarily
- **Graceful Degradation**: Core functionality continues if formatting fails

## Monitoring and Observability

### Metrics
```go
// Formatter performance metrics
formatter_duration_seconds{formatter="json"}     // Processing time per formatter
formatter_errors_total{formatter="json"}         // Error count per formatter  
formatter_bytes_total{formatter="json"}          // Bytes processed per formatter

// Destination metrics  
destination_writes_total{type="file"}            // Write count per destination type
destination_errors_total{type="http"}            // Error count per destination type
destination_duration_seconds{type="http"}        // Write duration per destination
```

### Logging
```go
// Formatter events
log.Printf("Processing incident %s with %d formatters", incident.ID, len(formatters))
log.Printf("Formatter %s processed %d entries in %v", formatter.Name(), len(entries), duration)
log.Printf("Formatter %s failed: %v", formatter.Name(), err)

// Destination events  
log.Printf("Written %d bytes to %s destination", len(data), destination.Type())
log.Printf("HTTP destination %s returned status %d", url, statusCode)
log.Printf("File destination created: %s", filePath)
```

## Best Practices

### Configuration
1. **Use appropriate formatters** for your audience (human vs machine)
2. **Configure multiple destinations** for redundancy
3. **Set up proper file rotation** for long-running systems
4. **Monitor destination health** and set up alerts

### Performance
1. **Limit formatter chain length** to avoid processing overhead
2. **Use async HTTP destinations** for high-throughput scenarios  
3. **Configure appropriate timeouts** for external dependencies
4. **Monitor memory usage** with large incident datasets

### Reliability  
1. **Handle destination failures gracefully** (don't fail entire incident processing)
2. **Set up backup destinations** for critical incidents
3. **Monitor formatter error rates** and investigate failures
4. **Test formatter chain configuration** before deployment

### Security
1. **Sanitize sensitive data** before formatting
2. **Use HTTPS** for HTTP destinations
3. **Secure file permissions** for file destinations  
4. **Validate HTTP endpoint certificates**
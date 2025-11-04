// Package formatter provides a flexible system for formatting and outputting telemetry data
// and incident reports in multiple formats (default text, JSON, CSV) to various destinations.
package formatter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/verygoodsoftwarecompany/blackbox-daemon/pkg/types"
)

// Formatter defines the interface for output formatters that convert telemetry entries
// and incident reports into formatted byte output.
type Formatter interface {
	Format(entries []types.TelemetryEntry, incident types.IncidentReport) ([]byte, error)
	Name() string
}

// Destination defines where formatted output should be written (files, HTTP endpoints, etc.).
type Destination interface {
	Write(data []byte) error
	Close() error
}

// FormatterChain manages multiple formatters and their destinations, allowing
// telemetry data to be simultaneously output in different formats to different locations.
type FormatterChain struct {
	formatters []FormatterConfig
}

// FormatterConfig combines a formatter with its destinations, defining how
// data should be formatted and where it should be sent.
type FormatterConfig struct {
	Formatter    Formatter
	Destinations []Destination
}

// NewFormatterChain creates a new formatter chain with no configured formatters.
func NewFormatterChain() *FormatterChain {
	return &FormatterChain{
		formatters: make([]FormatterConfig, 0),
	}
}

// AddFormatter adds a formatter with its destinations to the chain, allowing
// the same data to be formatted and sent to multiple destinations.
func (fc *FormatterChain) AddFormatter(formatter Formatter, destinations ...Destination) {
	fc.formatters = append(fc.formatters, FormatterConfig{
		Formatter:    formatter,
		Destinations: destinations,
	})
}

// Process runs all formatters in the chain for the given incident, formatting the data
// with each formatter and writing to their respective destinations.
func (fc *FormatterChain) Process(entries []types.TelemetryEntry, incident types.IncidentReport) error {
	for _, config := range fc.formatters {
		data, err := config.Formatter.Format(entries, incident)
		if err != nil {
			return fmt.Errorf("formatter %s failed: %w", config.Formatter.Name(), err)
		}

		for _, dest := range config.Destinations {
			if err := dest.Write(data); err != nil {
				return fmt.Errorf("failed to write to destination: %w", err)
			}
		}
	}
	return nil
}

// Close closes all destinations in the chain, ensuring resources are properly cleaned up.
func (fc *FormatterChain) Close() error {
	var errors []string
	for _, config := range fc.formatters {
		for _, dest := range config.Destinations {
			if err := dest.Close(); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("errors closing destinations: %s", strings.Join(errors, "; "))
	}
	return nil
}

// DefaultFormatter implements the default "DATE : TIME | TELEMETRY ITEM NAME | VALUE" format
// with a human-readable incident report header.
type DefaultFormatter struct{}

// NewDefaultFormatter creates a new default formatter instance.
func NewDefaultFormatter() *DefaultFormatter {
	return &DefaultFormatter{}
}

// Name returns the formatter name for identification and logging.
func (df *DefaultFormatter) Name() string {
	return "default"
}

// Format formats telemetry entries using the default human-readable format with
// an incident report header followed by timestamped telemetry entries.
func (df *DefaultFormatter) Format(entries []types.TelemetryEntry, incident types.IncidentReport) ([]byte, error) {
	var output strings.Builder

	// Write incident header
	output.WriteString(fmt.Sprintf("=== INCIDENT REPORT ===\n"))
	output.WriteString(fmt.Sprintf("ID: %s\n", incident.ID))
	output.WriteString(fmt.Sprintf("TIMESTAMP: %s\n", incident.Timestamp.Format("2006-01-02 15:04:05.000")))
	output.WriteString(fmt.Sprintf("SEVERITY: %s\n", incident.Severity))
	output.WriteString(fmt.Sprintf("TYPE: %s\n", incident.Type))
	output.WriteString(fmt.Sprintf("MESSAGE: %s\n", incident.Message))
	if incident.PodName != "" {
		output.WriteString(fmt.Sprintf("POD: %s/%s\n", incident.Namespace, incident.PodName))
	}
	output.WriteString("\n")

	// Write telemetry data
	output.WriteString("=== TELEMETRY DATA ===\n")
	for _, entry := range entries {
		dateTime := entry.Timestamp.Format("2006-01-02 : 15:04:05.000")
		output.WriteString(fmt.Sprintf("%s | %s | %v\n", dateTime, entry.Name, entry.Value))
	}

	return []byte(output.String()), nil
}

// JSONFormatter formats output as structured JSON for machine consumption
// and integration with logging systems.
type JSONFormatter struct{}

// NewJSONFormatter creates a new JSON formatter instance.
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

// Name returns the formatter name for identification and logging.
func (jf *JSONFormatter) Name() string {
	return "json"
}

// Format formats the incident and telemetry data as structured JSON with
// a generation timestamp for audit purposes.
func (jf *JSONFormatter) Format(entries []types.TelemetryEntry, incident types.IncidentReport) ([]byte, error) {
	output := map[string]interface{}{
		"incident":     incident,
		"telemetry":    entries,
		"generated_at": time.Now(),
	}

	return json.MarshalIndent(output, "", "  ")
}

// CSVFormatter formats telemetry as CSV for data analysis and spreadsheet import.
type CSVFormatter struct{}

// NewCSVFormatter creates a new CSV formatter instance.
func NewCSVFormatter() *CSVFormatter {
	return &CSVFormatter{}
}

// Name returns the formatter name for identification and logging.
func (cf *CSVFormatter) Name() string {
	return "csv"
}

// Format formats telemetry entries as CSV with headers and properly escaped values,
// including tags as semicolon-separated key=value pairs.
func (cf *CSVFormatter) Format(entries []types.TelemetryEntry, incident types.IncidentReport) ([]byte, error) {
	var output strings.Builder

	// CSV header
	output.WriteString("timestamp,source,type,name,value,tags,incident_id\n")

	// CSV data
	for _, entry := range entries {
		tags := ""
		if entry.Tags != nil {
			tagPairs := make([]string, 0, len(entry.Tags))
			for k, v := range entry.Tags {
				tagPairs = append(tagPairs, fmt.Sprintf("%s=%s", k, v))
			}
			tags = strings.Join(tagPairs, ";")
		}

		output.WriteString(fmt.Sprintf("%s,%s,%s,%s,%v,\"%s\",%s\n",
			entry.Timestamp.Format("2006-01-02T15:04:05.000Z"),
			entry.Source,
			entry.Type,
			entry.Name,
			entry.Value,
			tags,
			incident.ID,
		))
	}

	return []byte(output.String()), nil
}

// File Destination

// FileDestination writes output to a file with automatic directory creation
// and proper file handle management.
type FileDestination struct {
	file     *os.File
	filePath string
}

// NewFileDestination creates a new file destination, creating directories as needed
// and opening the file in append mode.
func NewFileDestination(filePath string) (*FileDestination, error) {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	return &FileDestination{
		file:     file,
		filePath: filePath,
	}, nil
}

// Write writes data to the file and syncs to ensure data reaches disk.
func (fd *FileDestination) Write(data []byte) error {
	_, err := fd.file.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %w", fd.filePath, err)
	}
	return fd.file.Sync() // Ensure data is written to disk
}

// Close closes the file handle, flushing any buffered data.
func (fd *FileDestination) Close() error {
	if fd.file != nil {
		return fd.file.Close()
	}
	return nil
}

// Stdout Destination

// StdoutDestination writes output to standard output for debugging and development.
type StdoutDestination struct{}

// NewStdoutDestination creates a new stdout destination
func NewStdoutDestination() *StdoutDestination {
	return &StdoutDestination{}
}

// Write writes data to stdout
func (sd *StdoutDestination) Write(data []byte) error {
	_, err := os.Stdout.Write(data)
	return err
}

// Close is a no-op for stdout
func (sd *StdoutDestination) Close() error {
	return nil
}

// HTTP Destination

// HTTPDestination sends output to an HTTP endpoint
type HTTPDestination struct {
	url    string
	client *http.Client
}

// NewHTTPDestination creates a new HTTP destination
func NewHTTPDestination(url string) *HTTPDestination {
	return &HTTPDestination{
		url: url,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Write sends data to the HTTP endpoint
func (hd *HTTPDestination) Write(data []byte) error {
	resp, err := hd.client.Post(hd.url, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("failed to post to %s: %w", hd.url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Close is a no-op for HTTP destination
func (hd *HTTPDestination) Close() error {
	return nil
}

// Helper functions for creating formatter chains from configuration

// CreateFormatterChain creates a formatter chain from configuration strings
func CreateFormatterChain(formatters []string, outputPath string) (*FormatterChain, error) {
	chain := NewFormatterChain()

	for _, formatterName := range formatters {
		var formatter Formatter
		var destinations []Destination

		// Create formatter
		switch strings.ToLower(formatterName) {
		case "default":
			formatter = NewDefaultFormatter()
		case "json":
			formatter = NewJSONFormatter()
		case "csv":
			formatter = NewCSVFormatter()
		default:
			return nil, fmt.Errorf("unknown formatter: %s", formatterName)
		}

		// Create destinations
		if outputPath == "stdout" {
			destinations = append(destinations, NewStdoutDestination())
		} else if strings.HasPrefix(outputPath, "http://") || strings.HasPrefix(outputPath, "https://") {
			destinations = append(destinations, NewHTTPDestination(outputPath))
		} else {
			// File destination - include formatter name in filename
			timestamp := time.Now().Format("20060102_150405")
			filename := fmt.Sprintf("%s_%s_%s.log", timestamp, formatterName, "incident")
			filePath := filepath.Join(outputPath, filename)

			dest, err := NewFileDestination(filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to create file destination: %w", err)
			}
			destinations = append(destinations, dest)
		}

		chain.AddFormatter(formatter, destinations...)
	}

	return chain, nil
}

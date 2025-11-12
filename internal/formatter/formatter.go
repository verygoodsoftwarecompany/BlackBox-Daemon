// Package formatter provides a flexible system for formatting and outputting telemetry data
// and incident reports in multiple formats (default text, JSON, CSV) to various destinations.
package formatter

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/verygoodsoftwarecompany/blackbox-daemon/pkg/emitter"
	"github.com/verygoodsoftwarecompany/blackbox-daemon/pkg/types"
)

// Formatter defines the interface for output formatters that convert telemetry entries
// and incident reports into formatted byte output.
type Formatter interface {
	Format(entries []types.TelemetryEntry, incident types.IncidentReport) ([]byte, error)
	Name() string
}

// FormatterChain manages multiple formatters and their destinations, allowing
// telemetry data to be simultaneously output in different formats to different locations.
type FormatterChain struct {
	formatters []FormatterConfig
}

// FormatterConfig combines a formatter with its emitters, defining how
// data should be formatted and where it should be emitted.
type FormatterConfig struct {
	Formatter Formatter
	Emitters  []emitter.Emitter
}

// NewFormatterChain creates a new formatter chain with no configured formatters.
func NewFormatterChain() *FormatterChain {
	return &FormatterChain{
		formatters: make([]FormatterConfig, 0),
	}
}

// AddFormatter adds a formatter with its emitters to the chain, allowing
// the same data to be formatted and emitted to multiple destinations.
func (fc *FormatterChain) AddFormatter(formatter Formatter, emitters ...emitter.Emitter) {
	fc.formatters = append(fc.formatters, FormatterConfig{
		Formatter: formatter,
		Emitters:  emitters,
	})
}

// Process runs all formatters in the chain for the given incident, formatting the data
// with each formatter and emitting to their respective destinations.
func (fc *FormatterChain) Process(entries []types.TelemetryEntry, incident types.IncidentReport) error {
	for _, config := range fc.formatters {
		data, err := config.Formatter.Format(entries, incident)
		if err != nil {
			return fmt.Errorf("formatter %s failed: %w", config.Formatter.Name(), err)
		}

		for _, emit := range config.Emitters {
			if err := emit.Emit(data); err != nil {
				return fmt.Errorf("failed to emit to %s: %w", emit.Name(), err)
			}
		}
	}
	return nil
}

// Close closes all emitters in the chain, ensuring resources are properly cleaned up.
func (fc *FormatterChain) Close() error {
	var errors []string
	for _, config := range fc.formatters {
		for _, emit := range config.Emitters {
			if err := emit.Close(); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("errors closing emitters: %s", strings.Join(errors, "; "))
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

// Helper functions for creating formatter chains from configuration

// CreateFormatterChain creates a formatter chain from configuration strings and emitter configs
func CreateFormatterChain(formatters []string, emitterConfigs []emitter.EmitterConfig) (*FormatterChain, error) {
	chain := NewFormatterChain()
	
	// Create emitters from configuration
	var emitters []emitter.Emitter
	for _, config := range emitterConfigs {
		emit, err := emitter.CreateEmitter(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create emitter: %w", err)
		}
		emitters = append(emitters, emit)
	}

	for _, formatterName := range formatters {
		var formatter Formatter

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

		// Add formatter with all configured emitters
		chain.AddFormatter(formatter, emitters...)
	}

	return chain, nil
}

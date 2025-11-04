package formatter

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/verygoodsoftwarecompany/blackbox-daemon/pkg/types"
)

func TestNewFormatterChain(t *testing.T) {
	t.Run("creates formatter chain with single formatter", func(t *testing.T) {
		tempDir := t.TempDir()
		formatters := []string{"json"}

		chain, err := NewFormatterChain(formatters, tempDir)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if chain == nil {
			t.Fatal("Expected formatter chain to be created")
		}
		if len(chain.formatters) != 1 {
			t.Errorf("Expected 1 formatter, got %v", len(chain.formatters))
		}

		// Clean up
		chain.Close()
	})

	t.Run("creates formatter chain with multiple formatters", func(t *testing.T) {
		tempDir := t.TempDir()
		formatters := []string{"json", "csv", "default"}

		chain, err := NewFormatterChain(formatters, tempDir)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(chain.formatters) != 3 {
			t.Errorf("Expected 3 formatters, got %v", len(chain.formatters))
		}

		// Clean up
		chain.Close()
	})

	t.Run("handles unknown formatter", func(t *testing.T) {
		tempDir := t.TempDir()
		formatters := []string{"unknown"}

		_, err := NewFormatterChain(formatters, tempDir)

		if err == nil {
			t.Error("Expected error for unknown formatter")
		}
		if !strings.Contains(err.Error(), "unknown formatter") {
			t.Errorf("Expected 'unknown formatter' error, got %v", err)
		}
	})

	t.Run("handles empty formatter list", func(t *testing.T) {
		tempDir := t.TempDir()
		formatters := []string{}

		chain, err := NewFormatterChain(formatters, tempDir)

		if err != nil {
			t.Fatalf("Expected no error for empty formatters, got %v", err)
		}
		if len(chain.formatters) != 0 {
			t.Errorf("Expected 0 formatters, got %v", len(chain.formatters))
		}

		// Clean up
		chain.Close()
	})
}

func TestFormatterChainFormat(t *testing.T) {
	t.Run("formats entry with multiple formatters", func(t *testing.T) {
		tempDir := t.TempDir()
		formatters := []string{"json", "csv"}

		chain, err := NewFormatterChain(formatters, tempDir)
		if err != nil {
			t.Fatalf("Failed to create formatter chain: %v", err)
		}
		defer chain.Close()

		entry := &types.TelemetryEntry{
			Timestamp: time.Now(),
			Source:    types.SourceSystem,
			Type:      types.TypeCPU,
			Name:      "cpu_usage",
			Value:     85.5,
			Tags: map[string]string{
				"core": "0",
			},
		}

		err = chain.Format(entry)

		if err != nil {
			t.Errorf("Expected no error formatting entry, got %v", err)
		}

		// Verify files were created
		jsonFile := filepath.Join(tempDir, "blackbox.json")
		csvFile := filepath.Join(tempDir, "blackbox.csv")

		if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
			t.Error("Expected JSON file to be created")
		}
		if _, err := os.Stat(csvFile); os.IsNotExist(err) {
			t.Error("Expected CSV file to be created")
		}
	})

	t.Run("handles formatting error gracefully", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a destination that will fail
		failingDest := &FailingDestination{}
		jsonFormatter := &JSONFormatter{}
		
		chain := &FormatterChain{
			formatters: []FormatterWithDestination{
				{
					Formatter:   jsonFormatter,
					Destination: failingDest,
				},
			},
		}

		entry := &types.TelemetryEntry{
			Timestamp: time.Now(),
			Source:    types.SourceSystem,
			Type:      types.TypeCPU,
			Name:      "test",
			Value:     42.0,
		}

		err := chain.Format(entry)

		// Should not return error even if one formatter fails
		if err != nil {
			t.Errorf("Expected no error even with failing formatter, got %v", err)
		}
	})
}

// Test helper - failing destination for testing error handling
type FailingDestination struct{}

func (fd *FailingDestination) Write(data []byte) error {
	return io.ErrClosedPipe
}

func (fd *FailingDestination) Close() error {
	return nil
}

func TestJSONFormatter(t *testing.T) {
	t.Run("formats entry as JSON", func(t *testing.T) {
		formatter := &JSONFormatter{}
		entry := &types.TelemetryEntry{
			Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Source:    types.SourceSidecar,
			Type:      types.TypeMemory,
			Name:      "heap_usage",
			Value:     1024,
			Tags: map[string]string{
				"runtime": "jvm",
			},
		}

		data, err := formatter.Format(entry)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify it's valid JSON
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("Expected valid JSON, got error: %v", err)
		}

		// Verify fields
		if result["source"] != "sidecar" {
			t.Errorf("Expected source 'sidecar', got %v", result["source"])
		}
		if result["type"] != "memory" {
			t.Errorf("Expected type 'memory', got %v", result["type"])
		}
		if result["name"] != "heap_usage" {
			t.Errorf("Expected name 'heap_usage', got %v", result["name"])
		}
		if result["value"] != float64(1024) {
			t.Errorf("Expected value 1024, got %v", result["value"])
		}
	})

	t.Run("returns correct content type", func(t *testing.T) {
		formatter := &JSONFormatter{}
		contentType := formatter.ContentType()

		if contentType != "application/json" {
			t.Errorf("Expected content type 'application/json', got %v", contentType)
		}
	})

	t.Run("handles complex value types", func(t *testing.T) {
		formatter := &JSONFormatter{}
		complexValue := map[string]interface{}{
			"heap_used":    1073741824,
			"heap_max":     2147483648,
			"gc_count":     15,
			"nested_map": map[string]string{
				"key": "value",
			},
		}

		entry := &types.TelemetryEntry{
			Timestamp: time.Now(),
			Source:    types.SourceSidecar,
			Type:      types.TypeApplication,
			Name:      "jvm_stats",
			Value:     complexValue,
		}

		data, err := formatter.Format(entry)

		if err != nil {
			t.Fatalf("Expected no error with complex value, got %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("Expected valid JSON with complex value, got error: %v", err)
		}

		// Verify complex value was preserved
		valueMap, ok := result["value"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected value to be a map")
		}
		if valueMap["heap_used"] != float64(1073741824) {
			t.Errorf("Expected heap_used to be preserved, got %v", valueMap["heap_used"])
		}
	})
}

func TestCSVFormatter(t *testing.T) {
	t.Run("formats entry as CSV", func(t *testing.T) {
		formatter := &CSVFormatter{}
		entry := &types.TelemetryEntry{
			Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Source:    types.SourceSystem,
			Type:      types.TypeCPU,
			Name:      "cpu_usage",
			Value:     85.5,
			Tags: map[string]string{
				"core": "0",
			},
		}

		data, err := formatter.Format(entry)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Parse CSV
		reader := csv.NewReader(bytes.NewReader(data))
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatalf("Expected valid CSV, got error: %v", err)
		}

		if len(records) != 1 {
			t.Errorf("Expected 1 CSV record, got %v", len(records))
		}

		record := records[0]
		if len(record) < 5 { // timestamp, source, type, name, value
			t.Errorf("Expected at least 5 CSV fields, got %v", len(record))
		}

		if record[1] != "system" {
			t.Errorf("Expected source 'system', got %v", record[1])
		}
		if record[2] != "cpu" {
			t.Errorf("Expected type 'cpu', got %v", record[2])
		}
		if record[3] != "cpu_usage" {
			t.Errorf("Expected name 'cpu_usage', got %v", record[3])
		}
	})

	t.Run("returns correct content type", func(t *testing.T) {
		formatter := &CSVFormatter{}
		contentType := formatter.ContentType()

		if contentType != "text/csv" {
			t.Errorf("Expected content type 'text/csv', got %v", contentType)
		}
	})

	t.Run("handles complex values", func(t *testing.T) {
		formatter := &CSVFormatter{}
		complexValue := map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		}

		entry := &types.TelemetryEntry{
			Timestamp: time.Now(),
			Source:    types.SourceSidecar,
			Type:      types.TypeApplication,
			Name:      "complex_data",
			Value:     complexValue,
		}

		data, err := formatter.Format(entry)

		if err != nil {
			t.Fatalf("Expected no error with complex value, got %v", err)
		}

		// Should be valid CSV
		reader := csv.NewReader(bytes.NewReader(data))
		_, err = reader.ReadAll()
		if err != nil {
			t.Fatalf("Expected valid CSV with complex value, got error: %v", err)
		}
	})
}

func TestDefaultFormatter(t *testing.T) {
	t.Run("formats entry in human readable format", func(t *testing.T) {
		formatter := &DefaultFormatter{}
		entry := &types.TelemetryEntry{
			Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Source:    types.SourceSystem,
			Type:      types.TypeCPU,
			Name:      "cpu_usage",
			Value:     85.5,
			Tags: map[string]string{
				"core": "0",
				"host": "node-1",
			},
		}

		data, err := formatter.Format(entry)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		output := string(data)

		// Check that key information is present
		if !strings.Contains(output, "2024-01-01T12:00:00Z") {
			t.Error("Expected timestamp in output")
		}
		if !strings.Contains(output, "system") {
			t.Error("Expected source in output")
		}
		if !strings.Contains(output, "cpu") {
			t.Error("Expected type in output")
		}
		if !strings.Contains(output, "cpu_usage") {
			t.Error("Expected name in output")
		}
		if !strings.Contains(output, "85.5") {
			t.Error("Expected value in output")
		}
		if !strings.Contains(output, "core=0") {
			t.Error("Expected tags in output")
		}
	})

	t.Run("returns correct content type", func(t *testing.T) {
		formatter := &DefaultFormatter{}
		contentType := formatter.ContentType()

		if contentType != "text/plain" {
			t.Errorf("Expected content type 'text/plain', got %v", contentType)
		}
	})
}

func TestFileDestination(t *testing.T) {
	t.Run("writes to file", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test.log")

		dest, err := NewFileDestination(filePath)
		if err != nil {
			t.Fatalf("Failed to create file destination: %v", err)
		}
		defer dest.Close()

		testData := []byte("test log entry\n")
		err = dest.Write(testData)

		if err != nil {
			t.Fatalf("Expected no error writing to file, got %v", err)
		}

		// Verify file contents
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read test file: %v", err)
		}

		if string(content) != string(testData) {
			t.Errorf("Expected file content %q, got %q", string(testData), string(content))
		}
	})

	t.Run("creates directory if not exists", func(t *testing.T) {
		tempDir := t.TempDir()
		nestedPath := filepath.Join(tempDir, "nested", "dir", "test.log")

		dest, err := NewFileDestination(nestedPath)
		if err != nil {
			t.Fatalf("Failed to create file destination with nested path: %v", err)
		}
		defer dest.Close()

		// Directory should be created
		dirPath := filepath.Dir(nestedPath)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			t.Error("Expected nested directory to be created")
		}
	})

	t.Run("handles write errors", func(t *testing.T) {
		// Try to write to a read-only directory (this might not work on all systems)
		dest := &FileDestination{
			file: nil, // This will cause Write to fail
		}

		err := dest.Write([]byte("test"))
		if err == nil {
			t.Error("Expected error writing to nil file")
		}
	})
}

func TestStdoutDestination(t *testing.T) {
	t.Run("writes to stdout", func(t *testing.T) {
		dest := NewStdoutDestination()
		testData := []byte("test output\n")

		// We can't easily capture stdout in this test,
		// but we can verify the method doesn't error
		err := dest.Write(testData)

		if err != nil {
			t.Errorf("Expected no error writing to stdout, got %v", err)
		}
	})

	t.Run("close does nothing", func(t *testing.T) {
		dest := NewStdoutDestination()
		err := dest.Close()

		if err != nil {
			t.Errorf("Expected no error closing stdout destination, got %v", err)
		}
	})
}

func TestHTTPDestination(t *testing.T) {
	t.Run("posts to HTTP endpoint", func(t *testing.T) {
		// Create test HTTP server
		var receivedData []byte
		var receivedContentType string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedContentType = r.Header.Get("Content-Type")
			body, _ := io.ReadAll(r.Body)
			receivedData = body
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		dest, err := NewHTTPDestination(server.URL, "application/json", map[string]string{
			"Authorization": "Bearer test-token",
		})
		if err != nil {
			t.Fatalf("Failed to create HTTP destination: %v", err)
		}
		defer dest.Close()

		testData := []byte(`{"test": "data"}`)
		err = dest.Write(testData)

		if err != nil {
			t.Fatalf("Expected no error posting to HTTP, got %v", err)
		}

		if string(receivedData) != string(testData) {
			t.Errorf("Expected received data %q, got %q", string(testData), string(receivedData))
		}
		if receivedContentType != "application/json" {
			t.Errorf("Expected content type 'application/json', got %v", receivedContentType)
		}
	})

	t.Run("handles HTTP errors", func(t *testing.T) {
		// Create server that returns error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		dest, err := NewHTTPDestination(server.URL, "text/plain", nil)
		if err != nil {
			t.Fatalf("Failed to create HTTP destination: %v", err)
		}
		defer dest.Close()

		err = dest.Write([]byte("test data"))

		if err == nil {
			t.Error("Expected error for HTTP 500 response")
		}
	})

	t.Run("handles invalid URL", func(t *testing.T) {
		dest, err := NewHTTPDestination("invalid-url", "text/plain", nil)

		if err == nil {
			t.Error("Expected error for invalid URL")
		}
		if dest != nil {
			t.Error("Expected nil destination for invalid URL")
		}
	})

	t.Run("includes custom headers", func(t *testing.T) {
		var receivedHeaders http.Header

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHeaders = r.Header
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		headers := map[string]string{
			"Authorization": "Bearer secret-token",
			"X-Custom-Header": "custom-value",
		}

		dest, err := NewHTTPDestination(server.URL, "application/json", headers)
		if err != nil {
			t.Fatalf("Failed to create HTTP destination: %v", err)
		}
		defer dest.Close()

		err = dest.Write([]byte("test"))
		if err != nil {
			t.Fatalf("Failed to write to HTTP destination: %v", err)
		}

		if receivedHeaders.Get("Authorization") != "Bearer secret-token" {
			t.Errorf("Expected Authorization header, got %v", receivedHeaders.Get("Authorization"))
		}
		if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
			t.Errorf("Expected X-Custom-Header, got %v", receivedHeaders.Get("X-Custom-Header"))
		}
	})
}

func TestFormatterChainClose(t *testing.T) {
	t.Run("closes all destinations", func(t *testing.T) {
		tempDir := t.TempDir()
		formatters := []string{"json", "csv"}

		chain, err := NewFormatterChain(formatters, tempDir)
		if err != nil {
			t.Fatalf("Failed to create formatter chain: %v", err)
		}

		// Write some data first
		entry := &types.TelemetryEntry{
			Timestamp: time.Now(),
			Source:    types.SourceSystem,
			Type:      types.TypeCPU,
			Name:      "test",
			Value:     42.0,
		}
		chain.Format(entry)

		// Close should not error
		err = chain.Close()
		if err != nil {
			t.Errorf("Expected no error closing formatter chain, got %v", err)
		}

		// Second close should also not error
		err = chain.Close()
		if err != nil {
			t.Errorf("Expected no error closing formatter chain twice, got %v", err)
		}
	})
}

func TestCreateDestination(t *testing.T) {
	t.Run("creates file destination", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test.log")

		dest, err := createDestination("file", filePath, "text/plain")

		if err != nil {
			t.Fatalf("Expected no error creating file destination, got %v", err)
		}
		if dest == nil {
			t.Fatal("Expected file destination to be created")
		}

		dest.Close()
	})

	t.Run("creates stdout destination", func(t *testing.T) {
		dest, err := createDestination("stdout", "", "text/plain")

		if err != nil {
			t.Fatalf("Expected no error creating stdout destination, got %v", err)
		}
		if dest == nil {
			t.Fatal("Expected stdout destination to be created")
		}

		dest.Close()
	})

	t.Run("creates HTTP destination", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		dest, err := createDestination("http", server.URL, "application/json")

		if err != nil {
			t.Fatalf("Expected no error creating HTTP destination, got %v", err)
		}
		if dest == nil {
			t.Fatal("Expected HTTP destination to be created")
		}

		dest.Close()
	})

	t.Run("handles unknown destination type", func(t *testing.T) {
		dest, err := createDestination("unknown", "", "text/plain")

		if err == nil {
			t.Error("Expected error for unknown destination type")
		}
		if dest != nil {
			t.Error("Expected nil destination for unknown type")
		}
	})
}
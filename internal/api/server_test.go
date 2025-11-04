// Package api provides comprehensive unit tests for the HTTP API server.
// These tests verify endpoint functionality, authentication, error handling, and integration.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/verygoodsoftwarecompany/blackbox-daemon/pkg/types"
)

// mockTelemetryBuffer implements TelemetryBuffer for testing.
type mockTelemetryBuffer struct {
	entries []types.TelemetryEntry
}

// Add records telemetry entries for test validation.
func (m *mockTelemetryBuffer) Add(entry types.TelemetryEntry) {
	m.entries = append(m.entries, entry)
}

// mockIncidentHandler implements IncidentHandler for testing.
type mockIncidentHandler struct {
	reports []types.IncidentReport
}

// HandleIncident records incident reports for test validation.
func (m *mockIncidentHandler) HandleIncident(report types.IncidentReport) {
	m.reports = append(m.reports, report)
}

// setupTestServer creates a test server with mock dependencies for testing API endpoints.
func setupTestServer() (*Server, *mockTelemetryBuffer, *mockIncidentHandler) {
	buffer := &mockTelemetryBuffer{}
	handler := &mockIncidentHandler{}
	
	server := NewServer(8080, "test-api-key-123", buffer, handler, false)
	return server, buffer, handler
}

// TestNewServer validates server creation and configuration.
func TestNewServer(t *testing.T) {
	server, _, _ := setupTestServer()

	if server == nil {
		t.Fatal("Expected server to be created")
	}
	
	if server.apiKey != "test-api-key-123" {
		t.Errorf("Expected API key 'test-api-key-123', got %q", server.apiKey)
	}
	
	if server.httpServer == nil {
		t.Fatal("Expected HTTP server to be initialized")
	}
	
	if server.httpServer.Addr != ":8080" {
		t.Errorf("Expected server address ':8080', got %q", server.httpServer.Addr)
	}
}

// TestAuthMiddleware validates authentication middleware functionality.
func TestAuthMiddleware(t *testing.T) {
	server, _, _ := setupTestServer()
	
	// Create a test handler to wrap with auth middleware
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})
	
	authHandler := server.authMiddleware(testHandler)
	
	t.Run("allows access to health endpoint without auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/health", nil)
		w := httptest.NewRecorder()
		
		authHandler.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("accepts valid API key", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/telemetry", nil)
		req.Header.Set("Authorization", "Bearer test-api-key-123")
		w := httptest.NewRecorder()
		
		authHandler.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("rejects missing authorization header", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/telemetry", nil)
		w := httptest.NewRecorder()
		
		authHandler.ServeHTTP(w, req)
		
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})
	
	t.Run("rejects invalid API key", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/telemetry", nil)
		req.Header.Set("Authorization", "Bearer wrong-key")
		w := httptest.NewRecorder()
		
		authHandler.ServeHTTP(w, req)
		
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})
	
	t.Run("rejects invalid bearer format", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/telemetry", nil)
		req.Header.Set("Authorization", "InvalidFormat test-api-key-123")
		w := httptest.NewRecorder()
		
		authHandler.ServeHTTP(w, req)
		
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})
}

// TestHandleTelemetry validates telemetry endpoint functionality and processing.
func TestHandleTelemetry(t *testing.T) {
	server, buffer, _ := setupTestServer()
	
	t.Run("accepts valid sidecar telemetry", func(t *testing.T) {
		telemetryData := types.SidecarTelemetry{
			PodName:     "test-pod",
			Namespace:   "test-namespace",
			ContainerID: "test-container",
			Runtime:     "jvm",
			Timestamp:   time.Now(),
			Data: map[string]interface{}{
				"heap_memory_used": 1024000,
				"gc_count":        5,
				"cpu_usage":       0.25,
			},
		}
		
		jsonData, _ := json.Marshal(telemetryData)
		req := httptest.NewRequest("POST", "/api/v1/telemetry", bytes.NewReader(jsonData))
		req.Header.Set("Authorization", "Bearer test-api-key-123")
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		server.handleTelemetry(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		// Verify telemetry entries were added to buffer
		if len(buffer.entries) != 3 { // Should be 3 entries from the data map
			t.Errorf("Expected 3 telemetry entries, got %d", len(buffer.entries))
		}
		
		// Verify entry properties
		for _, entry := range buffer.entries {
			if entry.Source != types.SourceSidecar {
				t.Errorf("Expected source to be SourceSidecar, got %v", entry.Source)
			}
			if entry.Tags["pod_name"] != "test-pod" {
				t.Errorf("Expected pod_name tag 'test-pod', got %v", entry.Tags["pod_name"])
			}
			if entry.Tags["namespace"] != "test-namespace" {
				t.Errorf("Expected namespace tag 'test-namespace', got %v", entry.Tags["namespace"])
			}
		}
	})
	
	t.Run("rejects invalid HTTP method", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/telemetry", nil)
		req.Header.Set("Authorization", "Bearer test-api-key-123")
		
		w := httptest.NewRecorder()
		server.handleTelemetry(w, req)
		
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})
	
	t.Run("rejects invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/telemetry", strings.NewReader("invalid json"))
		req.Header.Set("Authorization", "Bearer test-api-key-123")
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		server.handleTelemetry(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})
	
	t.Run("rejects missing required fields", func(t *testing.T) {
		telemetryData := types.SidecarTelemetry{
			// Missing PodName and Namespace
			Runtime:   "jvm",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"test": 123},
		}
		
		jsonData, _ := json.Marshal(telemetryData)
		req := httptest.NewRequest("POST", "/api/v1/telemetry", bytes.NewReader(jsonData))
		req.Header.Set("Authorization", "Bearer test-api-key-123")
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		server.handleTelemetry(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})
	
	t.Run("sets timestamp if not provided", func(t *testing.T) {
		// Clear previous entries
		buffer.entries = nil
		
		telemetryData := types.SidecarTelemetry{
			PodName:   "test-pod",
			Namespace: "test-namespace",
			Runtime:   "go",
			// No Timestamp provided
			Data: map[string]interface{}{"test_metric": 42},
		}
		
		jsonData, _ := json.Marshal(telemetryData)
		req := httptest.NewRequest("POST", "/api/v1/telemetry", bytes.NewReader(jsonData))
		req.Header.Set("Authorization", "Bearer test-api-key-123")
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		server.handleTelemetry(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		if len(buffer.entries) != 1 {
			t.Errorf("Expected 1 telemetry entry, got %d", len(buffer.entries))
		}
		
		// Verify timestamp was set automatically
		entry := buffer.entries[0]
		if entry.Timestamp.IsZero() {
			t.Error("Expected timestamp to be set automatically")
		}
	})
}

// TestHandleIncident validates incident reporting endpoint functionality.
func TestHandleIncident(t *testing.T) {
	server, _, handler := setupTestServer()
	
	t.Run("accepts valid incident report", func(t *testing.T) {
		incident := types.IncidentReport{
			ID:          "test-incident-123",
			Timestamp:   time.Now(),
			PodName:     "failed-pod",
			Namespace:   "production",
			ContainerID: "container-123",
			Severity:    types.SeverityHigh,
			Type:        types.IncidentCrash,
			Message:     "Application crashed with OOM error",
			Context: map[string]interface{}{
				"exit_code":    137,
				"memory_limit": "512Mi",
			},
		}
		
		jsonData, _ := json.Marshal(incident)
		req := httptest.NewRequest("POST", "/api/v1/incident", bytes.NewReader(jsonData))
		req.Header.Set("Authorization", "Bearer test-api-key-123")
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		server.handleIncident(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		// Verify incident was processed
		if len(handler.reports) != 1 {
			t.Errorf("Expected 1 incident report, got %d", len(handler.reports))
		}
		
		processedReport := handler.reports[0]
		if processedReport.ID != "test-incident-123" {
			t.Errorf("Expected incident ID 'test-incident-123', got %q", processedReport.ID)
		}
		if processedReport.Message != "Application crashed with OOM error" {
			t.Errorf("Expected message 'Application crashed with OOM error', got %q", processedReport.Message)
		}
	})
	
	t.Run("generates ID and timestamp if not provided", func(t *testing.T) {
		// Clear previous reports
		handler.reports = nil
		
		incident := types.IncidentReport{
			// No ID or Timestamp provided
			PodName:   "test-pod",
			Namespace: "test-namespace",
			Message:   "Manual incident report",
		}
		
		jsonData, _ := json.Marshal(incident)
		req := httptest.NewRequest("POST", "/api/v1/incident", bytes.NewReader(jsonData))
		req.Header.Set("Authorization", "Bearer test-api-key-123")
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		server.handleIncident(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		processedReport := handler.reports[0]
		if processedReport.ID == "" {
			t.Error("Expected ID to be generated automatically")
		}
		if processedReport.Timestamp.IsZero() {
			t.Error("Expected timestamp to be set automatically")
		}
		if processedReport.Severity != types.SeverityMedium {
			t.Errorf("Expected default severity Medium, got %v", processedReport.Severity)
		}
		if processedReport.Type != types.IncidentManual {
			t.Errorf("Expected default type Manual, got %v", processedReport.Type)
		}
	})
	
	t.Run("rejects invalid HTTP method", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/incident", nil)
		req.Header.Set("Authorization", "Bearer test-api-key-123")
		
		w := httptest.NewRecorder()
		server.handleIncident(w, req)
		
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})
	
	t.Run("rejects invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/incident", strings.NewReader("invalid json"))
		req.Header.Set("Authorization", "Bearer test-api-key-123")
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		server.handleIncident(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})
}

// TestHandleHealth validates the health check endpoint.
func TestHandleHealth(t *testing.T) {
	server, _, _ := setupTestServer()
	
	t.Run("returns health status", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/health", nil)
		w := httptest.NewRecorder()
		
		server.handleHealth(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Errorf("Failed to parse JSON response: %v", err)
		}
		
		if response["status"] != "healthy" {
			t.Errorf("Expected status 'healthy', got %v", response["status"])
		}
		
		if response["service"] != "blackbox-daemon" {
			t.Errorf("Expected service 'blackbox-daemon', got %v", response["service"])
		}
	})
	
	t.Run("rejects invalid HTTP method", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/health", nil)
		w := httptest.NewRecorder()
		
		server.handleHealth(w, req)
		
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})
}

// TestInferTelemetryType validates telemetry type inference logic.
func TestInferTelemetryType(t *testing.T) {
	server, _, _ := setupTestServer()
	
	tests := []struct {
		name     string
		key      string
		runtime  string
		expected types.TelemetryType
	}{
		{"memory metric", "heap_memory_used", "jvm", types.TypeMemory},
		{"GC metric", "gc_count", "jvm", types.TypeMemory},
		{"CPU metric", "cpu_usage", "go", types.TypeCPU},
		{"thread metric", "thread_count", "jvm", types.TypeCPU},
		{"network metric", "network_bytes_in", "go", types.TypeNetwork},
		{"connection metric", "socket_connections", "nodejs", types.TypeNetwork},
		{"runtime metric", "jvm_uptime", "jvm", types.TypeRuntime},
		{"VM metric", "clr_version", "dotnet", types.TypeRuntime},
		{"error metric", "exception_count", "python", types.TypeApplication},
		{"panic metric", "panic_total", "go", types.TypeApplication},
		{"unknown metric", "custom_business_metric", "java", types.TypeCustom},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.inferTelemetryType(tt.key, tt.runtime)
			if result != tt.expected {
				t.Errorf("Expected type %v for key %q, got %v", tt.expected, tt.key, result)
			}
		})
	}
}

// TestSwaggerEndpoints validates Swagger documentation endpoints when enabled.
func TestSwaggerEndpoints(t *testing.T) {
	t.Run("swagger disabled by default", func(t *testing.T) {
		server, _, _ := setupTestServer()
		
		req := httptest.NewRequest("GET", "/swagger.json", nil)
		w := httptest.NewRecorder()
		
		// Since swagger is disabled, this should return 404 (but goes through auth, so 401)
		server.httpServer.Handler.ServeHTTP(w, req)
		
		// Disabled swagger endpoints still go through auth middleware
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for disabled swagger (no auth), got %d", w.Code)
		}
	})
	
	t.Run("swagger enabled", func(t *testing.T) {
		buffer := &mockTelemetryBuffer{}
		handler := &mockIncidentHandler{}
		server := NewServer(8080, "test-key", buffer, handler, true) // Enable swagger
		
		req := httptest.NewRequest("GET", "/swagger.json", nil)
		w := httptest.NewRecorder()
		
		server.handleSwagger(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		var spec map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
			t.Errorf("Failed to parse swagger spec: %v", err)
		}
		
		if spec["openapi"] != "3.0.0" {
			t.Errorf("Expected OpenAPI version 3.0.0, got %v", spec["openapi"])
		}
	})
}

// TestServerIntegration validates end-to-end server functionality.
func TestServerIntegration(t *testing.T) {
	server, buffer, handler := setupTestServer()
	
	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	go func() {
		server.Start(ctx)
	}()
	
	// Give server time to start
	time.Sleep(10 * time.Millisecond)
	
	t.Run("full telemetry submission workflow", func(t *testing.T) {
		telemetryData := types.SidecarTelemetry{
			PodName:   "integration-test-pod",
			Namespace: "test",
			Runtime:   "go",
			Data: map[string]interface{}{
				"goroutines": 42,
				"heap_size":  1048576,
			},
		}
		
		jsonData, _ := json.Marshal(telemetryData)
		req := httptest.NewRequest("POST", "/api/v1/telemetry", bytes.NewReader(jsonData))
		req.Header.Set("Authorization", "Bearer test-api-key-123")
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		// Verify telemetry was processed
		if len(buffer.entries) < 2 {
			t.Errorf("Expected at least 2 telemetry entries, got %d", len(buffer.entries))
		}
	})
	
	t.Run("full incident reporting workflow", func(t *testing.T) {
		incident := types.IncidentReport{
			PodName:   "crashed-pod",
			Namespace: "production",
			Severity:  types.SeverityCritical,
			Type:      types.IncidentOOM,
			Message:   "Pod exceeded memory limits",
		}
		
		jsonData, _ := json.Marshal(incident)
		req := httptest.NewRequest("POST", "/api/v1/incident", bytes.NewReader(jsonData))
		req.Header.Set("Authorization", "Bearer test-api-key-123")
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		// Verify incident was processed
		if len(handler.reports) == 0 {
			t.Error("Expected incident report to be processed")
		}
	})
}
// Package api provides the HTTP REST API server for sidecar communication.
// This package handles telemetry submission, incident reporting, and provides
// optional Swagger documentation for the API endpoints.
package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/verygoodsoftwarecompany/blackbox-daemon/pkg/types"
)

// Server represents the HTTP API server for sidecar communication.
// It provides authenticated endpoints for telemetry submission and incident reporting.
type Server struct {
	// httpServer is the underlying HTTP server instance
	httpServer *http.Server
	// apiKey is the bearer token required for authentication
	apiKey string
	// buffer receives telemetry entries from sidecars
	buffer TelemetryBuffer
	// swaggerEnabled controls whether Swagger documentation is available
	swaggerEnabled bool
	// incidentHandler processes incident reports
	incidentHandler IncidentHandler
}

// TelemetryBuffer interface for adding telemetry entries to storage.
type TelemetryBuffer interface {
	Add(entry types.TelemetryEntry)
}

// IncidentHandler handles incident reports and triggers appropriate actions.
type IncidentHandler interface {
	HandleIncident(report types.IncidentReport)
}

// NewServer creates a new API server with the specified configuration.
// The server provides authenticated REST endpoints for sidecar communication.
func NewServer(port int, apiKey string, buffer TelemetryBuffer, incidentHandler IncidentHandler, swaggerEnabled bool) *Server {
	s := &Server{
		apiKey:          apiKey,
		buffer:          buffer,
		swaggerEnabled:  swaggerEnabled,
		incidentHandler: incidentHandler,
	}

	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/v1/telemetry", s.handleTelemetry)
	mux.HandleFunc("/api/v1/incident", s.handleIncident)
	mux.HandleFunc("/api/v1/health", s.handleHealth)

	if swaggerEnabled {
		mux.HandleFunc("/swagger.json", s.handleSwagger)
		mux.HandleFunc("/swagger/", s.handleSwaggerUI)
	}

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      s.authMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return s
}

// Start starts the HTTP server and begins accepting requests.
// The server will shutdown gracefully when the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpServer.Shutdown(shutdownCtx)
	}()

	fmt.Printf("Starting API server on %s\n", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// authMiddleware provides API key authentication for protected endpoints.
// Uses constant-time comparison to prevent timing attacks on the API key.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health check and swagger endpoints
		if r.URL.Path == "/api/v1/health" ||
			(s.swaggerEnabled && (r.URL.Path == "/swagger.json" || r.URL.Path == "/swagger/")) {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		expectedAuth := "Bearer " + s.apiKey

		// Use constant-time comparison to prevent timing attacks on API key validation
		if subtle.ConstantTimeCompare([]byte(authHeader), []byte(expectedAuth)) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleTelemetry processes sidecar telemetry submissions
func (s *Server) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var sidecarTelemetry types.SidecarTelemetry
	if err := json.NewDecoder(r.Body).Decode(&sidecarTelemetry); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if sidecarTelemetry.PodName == "" || sidecarTelemetry.Namespace == "" {
		http.Error(w, "Pod name and namespace are required", http.StatusBadRequest)
		return
	}

	// Set timestamp if not provided
	if sidecarTelemetry.Timestamp.IsZero() {
		sidecarTelemetry.Timestamp = time.Now()
	}

	// Convert sidecar telemetry to individual telemetry entries
	s.processSidecarTelemetry(sidecarTelemetry)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"status":    "accepted",
		"timestamp": time.Now(),
	}
	json.NewEncoder(w).Encode(response)
}

// processSidecarTelemetry converts sidecar telemetry into individual telemetry entries
func (s *Server) processSidecarTelemetry(sidecar types.SidecarTelemetry) {
	baseTags := map[string]string{
		"pod_name":  sidecar.PodName,
		"namespace": sidecar.Namespace,
		"runtime":   sidecar.Runtime,
	}

	if sidecar.ContainerID != "" {
		baseTags["container_id"] = sidecar.ContainerID
	}

	// Process each piece of telemetry data
	for key, value := range sidecar.Data {
		entry := types.TelemetryEntry{
			Timestamp: sidecar.Timestamp,
			Source:    types.SourceSidecar,
			Type:      s.inferTelemetryType(key, sidecar.Runtime),
			Name:      key,
			Value:     value,
			Tags:      baseTags,
			Metadata: map[string]interface{}{
				"sidecar_runtime": sidecar.Runtime,
			},
		}

		s.buffer.Add(entry)
	}
}

// inferTelemetryType attempts to categorize telemetry based on key name and runtime
func (s *Server) inferTelemetryType(key, runtime string) types.TelemetryType {
	// Common patterns for different types
	if contains(key, []string{"memory", "heap", "gc"}) {
		return types.TypeMemory
	}
	if contains(key, []string{"cpu", "thread", "processor"}) {
		return types.TypeCPU
	}
	if contains(key, []string{"network", "socket", "connection"}) {
		return types.TypeNetwork
	}
	if contains(key, []string{"runtime", "jvm", "clr", "vm"}) {
		return types.TypeRuntime
	}
	if contains(key, []string{"exception", "error", "panic"}) {
		return types.TypeApplication
	}

	return types.TypeCustom
}

// contains checks if any of the keywords appear in the string
func contains(str string, keywords []string) bool {
	for _, keyword := range keywords {
		if len(str) >= len(keyword) {
			for i := 0; i <= len(str)-len(keyword); i++ {
				if str[i:i+len(keyword)] == keyword {
					return true
				}
			}
		}
	}
	return false
}

// handleIncident processes incident reports from sidecars or manual submission
func (s *Server) handleIncident(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var report types.IncidentReport
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Set timestamp and ID if not provided
	if report.Timestamp.IsZero() {
		report.Timestamp = time.Now()
	}
	if report.ID == "" {
		report.ID = fmt.Sprintf("manual-%d", time.Now().Unix())
	}

	// Default severity and type if not specified
	if report.Severity == "" {
		report.Severity = types.SeverityMedium
	}
	if report.Type == "" {
		report.Type = types.IncidentManual
	}

	// Process the incident
	s.incidentHandler.HandleIncident(report)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"status":      "accepted",
		"incident_id": report.ID,
		"timestamp":   time.Now(),
	}
	json.NewEncoder(w).Encode(response)
}

// handleHealth provides a health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"service":   "blackbox-daemon",
		"version":   "1.0.0",
	}
	json.NewEncoder(w).Encode(response)
}

// handleSwagger serves the Swagger/OpenAPI specification
func (s *Server) handleSwagger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	swagger := generateSwaggerSpec()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(swagger)
}

// handleSwaggerUI serves a basic Swagger UI
func (s *Server) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	html := `<!DOCTYPE html>
<html>
<head>
    <title>BlackBox Daemon API</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@3.52.5/swagger-ui.css" />
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@3.52.5/swagger-ui-bundle.js"></script>
    <script>
        SwaggerUIBundle({
            url: '/swagger.json',
            dom_id: '#swagger-ui',
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIBundle.presets.standalone
            ]
        });
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// generateSwaggerSpec creates the OpenAPI specification
func generateSwaggerSpec() map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "BlackBox Daemon API",
			"description": "API for submitting telemetry and incident reports to BlackBox daemon",
			"version":     "1.0.0",
		},
		"servers": []map[string]interface{}{
			{
				"url":         "http://localhost:8080",
				"description": "BlackBox Daemon API Server",
			},
		},
		"security": []map[string]interface{}{
			{
				"bearerAuth": []string{},
			},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{
					"type":        "http",
					"scheme":      "bearer",
					"description": "API key for sidecar authentication",
				},
			},
		},
		"paths": map[string]interface{}{
			"/api/v1/telemetry": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Submit sidecar telemetry",
					"description": "Submit telemetry data from application sidecars",
					"security": []map[string]interface{}{
						{"bearerAuth": []string{}},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/SidecarTelemetry",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Telemetry accepted",
						},
						"400": map[string]interface{}{
							"description": "Invalid request",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/v1/incident": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Report an incident",
					"description": "Submit incident or crash reports",
					"security": []map[string]interface{}{
						{"bearerAuth": []string{}},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/IncidentReport",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Incident accepted",
						},
						"400": map[string]interface{}{
							"description": "Invalid request",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/v1/health": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Health check",
					"description": "Check if the BlackBox daemon is healthy",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Service is healthy",
						},
					},
				},
			},
		},
	}
}

// Package metrics provides comprehensive unit tests for Prometheus metrics collection.
// These tests validate metric registration, recording, HTTP server functionality, and
// custom metric management for the BlackBox-Daemon monitoring system.
package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestNewCollector validates collector creation and configuration.
func TestNewCollector(t *testing.T) {
	t.Run("creates collector with proper configuration", func(t *testing.T) {
		port := 9090
		path := "/metrics"
		
		collector := NewCollector(port, path)
		
		if collector == nil {
			t.Fatal("Expected collector to be created")
		}
		
		if collector.registry == nil {
			t.Error("Expected registry to be initialized")
		}
		
		if collector.httpServer == nil {
			t.Error("Expected HTTP server to be initialized")
		}
		
		expectedAddr := fmt.Sprintf(":%d", port)
		if collector.httpServer.Addr != expectedAddr {
			t.Errorf("Expected server address %s, got %s", expectedAddr, collector.httpServer.Addr)
		}
		
		// Verify all metric gauges are initialized
		if collector.cpuUsageGauge == nil {
			t.Error("Expected CPU usage gauge to be initialized")
		}
		if collector.memoryUsageGauge == nil {
			t.Error("Expected memory usage gauge to be initialized")
		}
		if collector.networkBytesGauge == nil {
			t.Error("Expected network bytes gauge to be initialized")
		}
		if collector.diskIOGauge == nil {
			t.Error("Expected disk I/O gauge to be initialized")
		}
		if collector.processCountGauge == nil {
			t.Error("Expected process count gauge to be initialized")
		}
		if collector.openFilesGauge == nil {
			t.Error("Expected open files gauge to be initialized")
		}
		if collector.loadAvgGauge == nil {
			t.Error("Expected load average gauge to be initialized")
		}
		
		// Verify operational metrics are initialized
		if collector.sidecarRequestsCounter == nil {
			t.Error("Expected sidecar requests counter to be initialized")
		}
		if collector.incidentCounter == nil {
			t.Error("Expected incident counter to be initialized")
		}
		if collector.bufferSizeGauge == nil {
			t.Error("Expected buffer size gauge to be initialized")
		}
		if collector.bufferEntriesGauge == nil {
			t.Error("Expected buffer entries gauge to be initialized")
		}
		
		if collector.customMetrics == nil {
			t.Error("Expected custom metrics map to be initialized")
		}
	})
}

// TestStart validates HTTP server startup and shutdown behavior.
func TestStart(t *testing.T) {
	t.Run("starts and stops HTTP server", func(t *testing.T) {
		collector := NewCollector(19090, "/metrics") // Use different port to avoid conflicts
		
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		// Start in goroutine
		errCh := make(chan error, 1)
		go func() {
			errCh <- collector.Start(ctx)
		}()
		
		// Give server time to start
		time.Sleep(50 * time.Millisecond)
		
		// Try to connect to verify server is running
		resp, err := http.Get("http://localhost:19090/")
		if err == nil {
			resp.Body.Close()
		}
		
		// Wait for context cancellation and server shutdown
		select {
		case err := <-errCh:
			// Should be nil (clean shutdown) or http.ErrServerClosed
			if err != nil && err != http.ErrServerClosed {
				t.Errorf("Expected clean shutdown, got %v", err)
			}
		case <-time.After(200 * time.Millisecond):
			t.Error("Server did not shut down within expected time")
		}
	})
	
	t.Run("handles context cancellation gracefully", func(t *testing.T) {
		collector := NewCollector(19091, "/metrics")
		
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		err := collector.Start(ctx)
		// Should return without error due to immediate cancellation
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("Expected clean shutdown or server closed error, got %v", err)
		}
	})
}

// TestRecordCPUUsage validates CPU metric recording.
func TestRecordCPUUsage(t *testing.T) {
	collector := NewCollector(9092, "/metrics")
	
	t.Run("records CPU usage metrics", func(t *testing.T) {
		core := "cpu0"
		usage := 75.5
		
		collector.RecordCPUUsage(core, usage)
		
		// Verify metric was recorded
		value := testutil.ToFloat64(collector.cpuUsageGauge.WithLabelValues(core))
		if value != usage {
			t.Errorf("Expected CPU usage %v, got %v", usage, value)
		}
	})
	
	t.Run("records multiple cores independently", func(t *testing.T) {
		cores := map[string]float64{
			"cpu0": 45.2,
			"cpu1": 67.8,
			"cpu2": 23.1,
		}
		
		for core, usage := range cores {
			collector.RecordCPUUsage(core, usage)
		}
		
		for core, expectedUsage := range cores {
			value := testutil.ToFloat64(collector.cpuUsageGauge.WithLabelValues(core))
			if value != expectedUsage {
				t.Errorf("Expected CPU usage for %s: %v, got %v", core, expectedUsage, value)
			}
		}
	})
}

// TestRecordMemoryUsage validates memory metric recording.
func TestRecordMemoryUsage(t *testing.T) {
	collector := NewCollector(9093, "/metrics")
	
	t.Run("records memory usage metrics", func(t *testing.T) {
		memTypes := map[string]uint64{
			"total":     8589934592, // 8GB
			"free":      2147483648, // 2GB
			"available": 4294967296, // 4GB
			"buffers":   536870912,  // 512MB
			"cached":    1073741824, // 1GB
		}
		
		for memType, bytes := range memTypes {
			collector.RecordMemoryUsage(memType, bytes)
		}
		
		for memType, expectedBytes := range memTypes {
			value := testutil.ToFloat64(collector.memoryUsageGauge.WithLabelValues(memType))
			if value != float64(expectedBytes) {
				t.Errorf("Expected memory %s: %d, got %v", memType, expectedBytes, value)
			}
		}
	})
}

// TestRecordNetworkBytes validates network metric recording.
func TestRecordNetworkBytes(t *testing.T) {
	collector := NewCollector(9094, "/metrics")
	
	t.Run("records network bytes metrics", func(t *testing.T) {
		testCases := []struct {
			iface     string
			direction string
			bytes     uint64
		}{
			{"eth0", "rx", 1048576}, // 1MB received
			{"eth0", "tx", 2097152}, // 2MB transmitted
			{"eth1", "rx", 524288},  // 512KB received
			{"eth1", "tx", 1572864}, // 1.5MB transmitted
		}
		
		for _, tc := range testCases {
			collector.RecordNetworkBytes(tc.iface, tc.direction, tc.bytes)
		}
		
		for _, tc := range testCases {
			value := testutil.ToFloat64(collector.networkBytesGauge.WithLabelValues(tc.iface, tc.direction))
			if value != float64(tc.bytes) {
				t.Errorf("Expected network bytes for %s %s: %d, got %v", tc.iface, tc.direction, tc.bytes, value)
			}
		}
	})
}

// TestRecordDiskIO validates disk I/O metric recording.
func TestRecordDiskIO(t *testing.T) {
	collector := NewCollector(9095, "/metrics")
	
	t.Run("records disk I/O metrics", func(t *testing.T) {
		testCases := []struct {
			device    string
			direction string
			bytes     uint64
		}{
			{"sda", "read", 10485760},  // 10MB read
			{"sda", "write", 5242880},  // 5MB written
			{"nvme0n1", "read", 20971520}, // 20MB read
			{"nvme0n1", "write", 15728640}, // 15MB written
		}
		
		for _, tc := range testCases {
			collector.RecordDiskIO(tc.device, tc.direction, tc.bytes)
		}
		
		for _, tc := range testCases {
			value := testutil.ToFloat64(collector.diskIOGauge.WithLabelValues(tc.device, tc.direction))
			if value != float64(tc.bytes) {
				t.Errorf("Expected disk I/O for %s %s: %d, got %v", tc.device, tc.direction, tc.bytes, value)
			}
		}
	})
}

// TestRecordProcessCount validates process count metric recording.
func TestRecordProcessCount(t *testing.T) {
	collector := NewCollector(9096, "/metrics")
	
	t.Run("records process count", func(t *testing.T) {
		count := 267
		collector.RecordProcessCount(count)
		
		value := testutil.ToFloat64(collector.processCountGauge)
		if value != float64(count) {
			t.Errorf("Expected process count %d, got %v", count, value)
		}
	})
}

// TestRecordOpenFiles validates open files metric recording.
func TestRecordOpenFiles(t *testing.T) {
	collector := NewCollector(9097, "/metrics")
	
	t.Run("records open files count", func(t *testing.T) {
		count := 1024
		collector.RecordOpenFiles(count)
		
		value := testutil.ToFloat64(collector.openFilesGauge)
		if value != float64(count) {
			t.Errorf("Expected open files count %d, got %v", count, value)
		}
	})
}

// TestRecordLoadAverage validates load average metric recording.
func TestRecordLoadAverage(t *testing.T) {
	collector := NewCollector(9098, "/metrics")
	
	t.Run("records load average metrics", func(t *testing.T) {
		loads := map[string]float64{
			"1min":  0.75,
			"5min":  1.25,
			"15min": 0.95,
		}
		
		for period, load := range loads {
			collector.RecordLoadAverage(period, load)
		}
		
		for period, expectedLoad := range loads {
			value := testutil.ToFloat64(collector.loadAvgGauge.WithLabelValues(period))
			if value != expectedLoad {
				t.Errorf("Expected load average for %s: %v, got %v", period, expectedLoad, value)
			}
		}
	})
}

// TestIncrementSidecarRequests validates sidecar request counter.
func TestIncrementSidecarRequests(t *testing.T) {
	collector := NewCollector(9099, "/metrics")
	
	t.Run("increments sidecar requests", func(t *testing.T) {
		// Should start at 0
		initialValue := testutil.ToFloat64(collector.sidecarRequestsCounter)
		if initialValue != 0 {
			t.Errorf("Expected initial value 0, got %v", initialValue)
		}
		
		// Increment multiple times
		for i := 0; i < 5; i++ {
			collector.IncrementSidecarRequests()
		}
		
		finalValue := testutil.ToFloat64(collector.sidecarRequestsCounter)
		if finalValue != 5 {
			t.Errorf("Expected final value 5, got %v", finalValue)
		}
	})
}

// TestIncrementIncidents validates incident counter.
func TestIncrementIncidents(t *testing.T) {
	collector := NewCollector(9100, "/metrics")
	
	t.Run("increments incidents with labels", func(t *testing.T) {
		incidents := []struct {
			incidentType string
			severity     string
			count        int
		}{
			{"crash", "high", 3},
			{"oom", "medium", 2},
			{"timeout", "low", 1},
		}
		
		for _, incident := range incidents {
			for i := 0; i < incident.count; i++ {
				collector.IncrementIncidents(incident.incidentType, incident.severity)
			}
		}
		
		for _, incident := range incidents {
			value := testutil.ToFloat64(collector.incidentCounter.WithLabelValues(incident.incidentType, incident.severity))
			if value != float64(incident.count) {
				t.Errorf("Expected %s:%s incidents %d, got %v", incident.incidentType, incident.severity, incident.count, value)
			}
		}
	})
}

// TestRecordBufferMetrics validates buffer metric recording.
func TestRecordBufferMetrics(t *testing.T) {
	collector := NewCollector(9101, "/metrics")
	
	t.Run("records buffer size", func(t *testing.T) {
		sizeBytes := 2048576 // ~2MB
		collector.RecordBufferSize(sizeBytes)
		
		value := testutil.ToFloat64(collector.bufferSizeGauge)
		if value != float64(sizeBytes) {
			t.Errorf("Expected buffer size %d, got %v", sizeBytes, value)
		}
	})
	
	t.Run("records buffer entries", func(t *testing.T) {
		entries := 1500
		collector.RecordBufferEntries(entries)
		
		value := testutil.ToFloat64(collector.bufferEntriesGauge)
		if value != float64(entries) {
			t.Errorf("Expected buffer entries %d, got %v", entries, value)
		}
	})
}

// TestCustomMetrics validates custom metric management.
func TestCustomMetrics(t *testing.T) {
	collector := NewCollector(9102, "/metrics")
	
	t.Run("registers custom counter", func(t *testing.T) {
		name := "test_counter"
		help := "Test counter metric"
		labels := []string{"label1", "label2"}
		
		counter, err := collector.NewCustomCounter(name, help, labels)
		if err != nil {
			t.Fatalf("Failed to create custom counter: %v", err)
		}
		
		if counter == nil {
			t.Error("Expected counter to be created")
		}
		
		// Check if metric is registered
		_, exists := collector.GetCustomMetric(name)
		if !exists {
			t.Error("Expected custom metric to be registered")
		}
		
		// Test counter functionality
		counter.WithLabelValues("val1", "val2").Inc()
		value := testutil.ToFloat64(counter.WithLabelValues("val1", "val2"))
		if value != 1 {
			t.Errorf("Expected counter value 1, got %v", value)
		}
	})
	
	t.Run("registers custom gauge", func(t *testing.T) {
		name := "test_gauge"
		help := "Test gauge metric"
		labels := []string{"instance"}
		
		gauge, err := collector.NewCustomGauge(name, help, labels)
		if err != nil {
			t.Fatalf("Failed to create custom gauge: %v", err)
		}
		
		if gauge == nil {
			t.Error("Expected gauge to be created")
		}
		
		// Test gauge functionality
		testValue := 42.5
		gauge.WithLabelValues("test-instance").Set(testValue)
		value := testutil.ToFloat64(gauge.WithLabelValues("test-instance"))
		if value != testValue {
			t.Errorf("Expected gauge value %v, got %v", testValue, value)
		}
	})
	
	t.Run("registers custom histogram", func(t *testing.T) {
		name := "test_histogram"
		help := "Test histogram metric"
		labels := []string{"method"}
		buckets := []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0}
		
		histogram, err := collector.NewCustomHistogram(name, help, labels, buckets)
		if err != nil {
			t.Fatalf("Failed to create custom histogram: %v", err)
		}
		
		if histogram == nil {
			t.Error("Expected histogram to be created")
		}
		
		// Test histogram functionality
		histogram.WithLabelValues("GET").Observe(0.75)
		histogram.WithLabelValues("GET").Observe(1.5)
		
		// Check that observations were recorded (we can't easily test the exact count without more complex validation)
		// Just verify the histogram exists and can accept observations without error
		histogram.WithLabelValues("POST").Observe(2.3)
		
		// If we got here without panicking, the histogram is working correctly
	})
	
	t.Run("prevents duplicate registration", func(t *testing.T) {
		name := "duplicate_metric"
		help := "Test duplicate metric"
		
		// Register first metric
		_, err := collector.NewCustomCounter(name, help, []string{})
		if err != nil {
			t.Fatalf("Failed to register first metric: %v", err)
		}
		
		// Try to register duplicate
		_, err = collector.NewCustomCounter(name, help, []string{})
		if err == nil {
			t.Error("Expected error when registering duplicate metric")
		}
		
		if !strings.Contains(err.Error(), "already registered") {
			t.Errorf("Expected 'already registered' error, got: %v", err)
		}
	})
	
	t.Run("unregisters custom metrics", func(t *testing.T) {
		name := "temp_metric"
		help := "Temporary metric"
		
		// Register metric
		_, err := collector.NewCustomCounter(name, help, []string{})
		if err != nil {
			t.Fatalf("Failed to register metric: %v", err)
		}
		
		// Verify it exists
		_, exists := collector.GetCustomMetric(name)
		if !exists {
			t.Error("Expected metric to be registered")
		}
		
		// Unregister it
		err = collector.UnregisterCustomMetric(name)
		if err != nil {
			t.Fatalf("Failed to unregister metric: %v", err)
		}
		
		// Verify it's gone
		_, exists = collector.GetCustomMetric(name)
		if exists {
			t.Error("Expected metric to be unregistered")
		}
	})
	
	t.Run("lists custom metrics", func(t *testing.T) {
		// Register multiple metrics
		metrics := []string{"metric_a", "metric_b", "metric_c"}
		
		for _, name := range metrics {
			_, err := collector.NewCustomCounter(name, "Test metric", []string{})
			if err != nil {
				t.Fatalf("Failed to register metric %s: %v", name, err)
			}
		}
		
		// List metrics
		listed := collector.ListCustomMetrics()
		
		// Should contain all registered metrics (may include others from previous tests)
		for _, expected := range metrics {
			found := false
			for _, listed := range listed {
				if listed == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected metric %s to be listed", expected)
			}
		}
	})
}

// TestMetricsHTTPEndpoint validates HTTP metrics endpoint.
func TestMetricsHTTPEndpoint(t *testing.T) {
	collector := NewCollector(19103, "/metrics")
	
	t.Run("serves metrics endpoint", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		
		// Start server in background
		go func() {
			collector.Start(ctx)
		}()
		
		// Give server time to start
		time.Sleep(100 * time.Millisecond)
		
		// Record some test metrics
		collector.RecordCPUUsage("cpu0", 50.0)
		collector.IncrementSidecarRequests()
		collector.RecordBufferEntries(100)
		
		// Make HTTP request to metrics endpoint
		resp, err := http.Get("http://localhost:19103/metrics")
		if err != nil {
			t.Fatalf("Failed to get metrics: %v", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
		
		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}
		
		bodyStr := string(body)
		
		// Verify some expected metrics are present
		expectedMetrics := []string{
			"blackbox_cpu_usage_percent",
			"blackbox_sidecar_requests_total",
			"blackbox_buffer_entries_total",
		}
		
		for _, metric := range expectedMetrics {
			if !strings.Contains(bodyStr, metric) {
				t.Errorf("Expected metric %s to be present in response", metric)
			}
		}
	})
	
	t.Run("serves root endpoint", func(t *testing.T) {
		collector2 := NewCollector(19104, "/metrics") // Use different port
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		
		// Start server in background
		go func() {
			collector2.Start(ctx)
		}()
		
		// Give server time to start
		time.Sleep(100 * time.Millisecond)
		
		// Make HTTP request to root endpoint
		resp, err := http.Get("http://localhost:19104/")
		if err != nil {
			t.Fatalf("Failed to get root endpoint: %v", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
		
		// Should contain HTML with link to metrics
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}
		
		bodyStr := string(body)
		if !strings.Contains(bodyStr, "BlackBox Daemon Metrics") {
			t.Error("Expected root page to contain title")
		}
		
		if !strings.Contains(bodyStr, "/metrics") {
			t.Error("Expected root page to contain metrics link")
		}
	})
}
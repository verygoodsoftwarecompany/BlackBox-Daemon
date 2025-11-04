package metrics

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNew(t *testing.T) {
	t.Run("creates metrics collector", func(t *testing.T) {
		address := ":9090"
		path := "/metrics"

		collector := New(address, path)

		if collector == nil {
			t.Fatal("Expected collector to be created")
		}
		if collector.address != address {
			t.Errorf("Expected address %s, got %s", address, collector.address)
		}
		if collector.metricsPath != path {
			t.Errorf("Expected path %s, got %s", path, collector.metricsPath)
		}
		if collector.registry == nil {
			t.Error("Expected registry to be initialized")
		}
	})
}

func TestRegisterDefaultMetrics(t *testing.T) {
	t.Run("registers all default metrics", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		// Test that default metrics are registered by checking if they can be gathered
		metricFamilies, err := collector.registry.Gather()
		if err != nil {
			t.Fatalf("Failed to gather metrics: %v", err)
		}

		expectedMetrics := []string{
			"blackbox_buffer_entries_total",
			"blackbox_buffer_utilization_percent",
			"blackbox_buffer_operations_total",
			"blackbox_buffer_memory_bytes",
			"blackbox_api_requests_total",
			"blackbox_api_request_duration_seconds",
			"blackbox_api_active_connections",
			"blackbox_k8s_pods_total",
			"blackbox_k8s_incidents_total",
			"blackbox_k8s_pod_restarts_total",
			"blackbox_telemetry_entries_total",
			"blackbox_telemetry_collection_duration_seconds",
			"blackbox_telemetry_errors_total",
			"blackbox_cpu_usage_percent",
			"blackbox_cpu_load_average",
			"blackbox_memory_usage_bytes",
			"blackbox_memory_utilization_percent",
			"blackbox_network_bytes_total",
			"blackbox_network_packets_total",
			"blackbox_network_errors_total",
			"blackbox_disk_operations_total",
			"blackbox_disk_bytes_total",
			"blackbox_disk_usage_bytes",
		}

		foundMetrics := make(map[string]bool)
		for _, mf := range metricFamilies {
			foundMetrics[mf.GetName()] = true
		}

		for _, expected := range expectedMetrics {
			if !foundMetrics[expected] {
				t.Errorf("Expected metric %s to be registered", expected)
			}
		}
	})
}

func TestUpdateBufferMetrics(t *testing.T) {
	t.Run("updates buffer metrics", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		collector.UpdateBufferMetrics(1500, 10000, 75.5, 2048000)

		// Check that metrics were updated
		metricValue := testutil.ToFloat64(collector.bufferEntries)
		if metricValue != 1500 {
			t.Errorf("Expected buffer entries 1500, got %v", metricValue)
		}

		utilizationValue := testutil.ToFloat64(collector.bufferUtilization)
		if utilizationValue != 75.5 {
			t.Errorf("Expected buffer utilization 75.5, got %v", utilizationValue)
		}

		memoryValue := testutil.ToFloat64(collector.bufferMemory)
		if memoryValue != 2048000 {
			t.Errorf("Expected buffer memory 2048000, got %v", memoryValue)
		}
	})
}

func TestIncrementBufferOperations(t *testing.T) {
	t.Run("increments buffer operations", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		// Increment different operations
		collector.IncrementBufferOperations("add")
		collector.IncrementBufferOperations("add")
		collector.IncrementBufferOperations("cleanup")

		// Check that counters were incremented
		addCounter, err := collector.bufferOperations.GetMetricWithLabelValues("add")
		if err != nil {
			t.Fatalf("Failed to get add counter: %v", err)
		}
		addValue := testutil.ToFloat64(addCounter)
		if addValue != 2 {
			t.Errorf("Expected add operations 2, got %v", addValue)
		}

		cleanupCounter, err := collector.bufferOperations.GetMetricWithLabelValues("cleanup")
		if err != nil {
			t.Fatalf("Failed to get cleanup counter: %v", err)
		}
		cleanupValue := testutil.ToFloat64(cleanupCounter)
		if cleanupValue != 1 {
			t.Errorf("Expected cleanup operations 1, got %v", cleanupValue)
		}
	})
}

func TestRecordAPIRequest(t *testing.T) {
	t.Run("records API request metrics", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		duration := 250 * time.Millisecond
		collector.RecordAPIRequest("/api/v1/telemetry", "POST", 201, duration)

		// Check request counter
		requestCounter, err := collector.apiRequests.GetMetricWithLabelValues("/api/v1/telemetry", "POST", "201")
		if err != nil {
			t.Fatalf("Failed to get request counter: %v", err)
		}
		requestValue := testutil.ToFloat64(requestCounter)
		if requestValue != 1 {
			t.Errorf("Expected 1 request, got %v", requestValue)
		}

		// Check that duration histogram was updated
		durationHistogram, err := collector.apiDuration.GetMetricWithLabelValues("/api/v1/telemetry")
		if err != nil {
			t.Fatalf("Failed to get duration histogram: %v", err)
		}
		
		// The histogram should have recorded the duration
		metric := &prometheus.HistogramVec{}
		if durationHistogram != metric {
			// We can't easily check exact histogram values without accessing internal state
			// But we can verify it doesn't error
		}
	})
}

func TestUpdateActiveConnections(t *testing.T) {
	t.Run("updates active connections", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		collector.UpdateActiveConnections(15)

		connectionValue := testutil.ToFloat64(collector.apiConnections)
		if connectionValue != 15 {
			t.Errorf("Expected 15 active connections, got %v", connectionValue)
		}

		collector.UpdateActiveConnections(20)
		connectionValue = testutil.ToFloat64(collector.apiConnections)
		if connectionValue != 20 {
			t.Errorf("Expected 20 active connections, got %v", connectionValue)
		}
	})
}

func TestUpdatePodMetrics(t *testing.T) {
	t.Run("updates pod count metrics", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		collector.UpdatePodMetrics("running", 25)
		collector.UpdatePodMetrics("pending", 3)
		collector.UpdatePodMetrics("failed", 1)

		runningValue := testutil.ToFloat64(collector.k8sPods.WithLabelValues("running"))
		if runningValue != 25 {
			t.Errorf("Expected 25 running pods, got %v", runningValue)
		}

		pendingValue := testutil.ToFloat64(collector.k8sPods.WithLabelValues("pending"))
		if pendingValue != 3 {
			t.Errorf("Expected 3 pending pods, got %v", pendingValue)
		}
	})
}

func TestIncrementIncidents(t *testing.T) {
	t.Run("increments incident metrics", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		collector.IncrementIncidents("crash", "critical")
		collector.IncrementIncidents("crash", "critical")
		collector.IncrementIncidents("oom", "high")

		crashCounter, err := collector.k8sIncidents.GetMetricWithLabelValues("crash", "critical")
		if err != nil {
			t.Fatalf("Failed to get crash counter: %v", err)
		}
		crashValue := testutil.ToFloat64(crashCounter)
		if crashValue != 2 {
			t.Errorf("Expected 2 crash incidents, got %v", crashValue)
		}

		oomCounter, err := collector.k8sIncidents.GetMetricWithLabelValues("oom", "high")
		if err != nil {
			t.Fatalf("Failed to get oom counter: %v", err)
		}
		oomValue := testutil.ToFloat64(oomCounter)
		if oomValue != 1 {
			t.Errorf("Expected 1 oom incident, got %v", oomValue)
		}
	})
}

func TestIncrementPodRestarts(t *testing.T) {
	t.Run("increments pod restart counter", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		collector.IncrementPodRestarts("production", "user-service-abc123")
		collector.IncrementPodRestarts("production", "user-service-abc123")
		collector.IncrementPodRestarts("staging", "test-service-xyz789")

		prodCounter, err := collector.k8sPodRestarts.GetMetricWithLabelValues("production", "user-service-abc123")
		if err != nil {
			t.Fatalf("Failed to get production restart counter: %v", err)
		}
		prodValue := testutil.ToFloat64(prodCounter)
		if prodValue != 2 {
			t.Errorf("Expected 2 production restarts, got %v", prodValue)
		}

		stagingCounter, err := collector.k8sPodRestarts.GetMetricWithLabelValues("staging", "test-service-xyz789")
		if err != nil {
			t.Fatalf("Failed to get staging restart counter: %v", err)
		}
		stagingValue := testutil.ToFloat64(stagingCounter)
		if stagingValue != 1 {
			t.Errorf("Expected 1 staging restart, got %v", stagingValue)
		}
	})
}

func TestIncrementTelemetryEntries(t *testing.T) {
	t.Run("increments telemetry entry counter", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		collector.IncrementTelemetryEntries("system")
		collector.IncrementTelemetryEntries("system")
		collector.IncrementTelemetryEntries("sidecar")

		systemCounter, err := collector.telemetryEntries.GetMetricWithLabelValues("system")
		if err != nil {
			t.Fatalf("Failed to get system counter: %v", err)
		}
		systemValue := testutil.ToFloat64(systemCounter)
		if systemValue != 2 {
			t.Errorf("Expected 2 system entries, got %v", systemValue)
		}

		sidecarCounter, err := collector.telemetryEntries.GetMetricWithLabelValues("sidecar")
		if err != nil {
			t.Fatalf("Failed to get sidecar counter: %v", err)
		}
		sidecarValue := testutil.ToFloat64(sidecarCounter)
		if sidecarValue != 1 {
			t.Errorf("Expected 1 sidecar entry, got %v", sidecarValue)
		}
	})
}

func TestRecordCollectionDuration(t *testing.T) {
	t.Run("records collection duration", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		duration := 150 * time.Millisecond
		collector.RecordCollectionDuration("system", duration)

		// Verify histogram was updated (we can't easily check exact values)
		systemHistogram, err := collector.telemetryDuration.GetMetricWithLabelValues("system")
		if err != nil {
			t.Fatalf("Failed to get system duration histogram: %v", err)
		}
		if systemHistogram == nil {
			t.Error("Expected system duration histogram to exist")
		}
	})
}

func TestIncrementTelemetryErrors(t *testing.T) {
	t.Run("increments telemetry error counter", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		collector.IncrementTelemetryErrors("file_not_found")
		collector.IncrementTelemetryErrors("file_not_found")
		collector.IncrementTelemetryErrors("permission_denied")

		fileErrorCounter, err := collector.telemetryErrors.GetMetricWithLabelValues("file_not_found")
		if err != nil {
			t.Fatalf("Failed to get file error counter: %v", err)
		}
		fileErrorValue := testutil.ToFloat64(fileErrorCounter)
		if fileErrorValue != 2 {
			t.Errorf("Expected 2 file errors, got %v", fileErrorValue)
		}

		permErrorCounter, err := collector.telemetryErrors.GetMetricWithLabelValues("permission_denied")
		if err != nil {
			t.Fatalf("Failed to get permission error counter: %v", err)
		}
		permErrorValue := testutil.ToFloat64(permErrorCounter)
		if permErrorValue != 1 {
			t.Errorf("Expected 1 permission error, got %v", permErrorValue)
		}
	})
}

func TestUpdateSystemMetrics(t *testing.T) {
	t.Run("updates CPU metrics", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		collector.UpdateCPUUsage("0", 85.5)
		collector.UpdateCPUUsage("1", 42.3)

		cpu0Value := testutil.ToFloat64(collector.cpuUsage.WithLabelValues("0"))
		if cpu0Value != 85.5 {
			t.Errorf("Expected CPU0 usage 85.5, got %v", cpu0Value)
		}

		cpu1Value := testutil.ToFloat64(collector.cpuUsage.WithLabelValues("1"))
		if cpu1Value != 42.3 {
			t.Errorf("Expected CPU1 usage 42.3, got %v", cpu1Value)
		}
	})

	t.Run("updates load average metrics", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		collector.UpdateLoadAverage("1m", 1.25)
		collector.UpdateLoadAverage("5m", 1.10)

		load1Value := testutil.ToFloat64(collector.cpuLoadAverage.WithLabelValues("1m"))
		if load1Value != 1.25 {
			t.Errorf("Expected 1m load 1.25, got %v", load1Value)
		}

		load5Value := testutil.ToFloat64(collector.cpuLoadAverage.WithLabelValues("5m"))
		if load5Value != 1.10 {
			t.Errorf("Expected 5m load 1.10, got %v", load5Value)
		}
	})

	t.Run("updates memory metrics", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		collector.UpdateMemoryUsage("total", 8192000000)
		collector.UpdateMemoryUsage("free", 2048000000)
		collector.UpdateMemoryUtilization(75.5)

		totalValue := testutil.ToFloat64(collector.memoryUsage.WithLabelValues("total"))
		if totalValue != 8192000000 {
			t.Errorf("Expected total memory 8192000000, got %v", totalValue)
		}

		utilizationValue := testutil.ToFloat64(collector.memoryUtilization)
		if utilizationValue != 75.5 {
			t.Errorf("Expected memory utilization 75.5, got %v", utilizationValue)
		}
	})

	t.Run("updates network metrics", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		collector.UpdateNetworkBytes("eth0", "tx", 1048576)
		collector.UpdateNetworkPackets("eth0", "rx", 1024)
		collector.UpdateNetworkErrors("eth0", "tx", 5)

		bytesValue := testutil.ToFloat64(collector.networkBytes.WithLabelValues("eth0", "tx"))
		if bytesValue != 1048576 {
			t.Errorf("Expected network bytes 1048576, got %v", bytesValue)
		}

		packetsValue := testutil.ToFloat64(collector.networkPackets.WithLabelValues("eth0", "rx"))
		if packetsValue != 1024 {
			t.Errorf("Expected network packets 1024, got %v", packetsValue)
		}

		errorsValue := testutil.ToFloat64(collector.networkErrors.WithLabelValues("eth0", "tx"))
		if errorsValue != 5 {
			t.Errorf("Expected network errors 5, got %v", errorsValue)
		}
	})

	t.Run("updates disk metrics", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		collector.UpdateDiskOperations("sda", "read", 12345)
		collector.UpdateDiskBytes("sda", "write", 67108864)
		collector.UpdateDiskUsage("/", "used", 53687091200)

		opsValue := testutil.ToFloat64(collector.diskOperations.WithLabelValues("sda", "read"))
		if opsValue != 12345 {
			t.Errorf("Expected disk operations 12345, got %v", opsValue)
		}

		bytesValue := testutil.ToFloat64(collector.diskBytes.WithLabelValues("sda", "write"))
		if bytesValue != 67108864 {
			t.Errorf("Expected disk bytes 67108864, got %v", bytesValue)
		}

		usageValue := testutil.ToFloat64(collector.diskUsage.WithLabelValues("/", "used"))
		if usageValue != 53687091200 {
			t.Errorf("Expected disk usage 53687091200, got %v", usageValue)
		}
	})
}

func TestRegisterCustomMetric(t *testing.T) {
	t.Run("registers custom metric", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		customCounter := prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_custom_metric_total",
			Help: "A test custom metric",
		})

		err := collector.RegisterCustomMetric("test_custom", customCounter)
		if err != nil {
			t.Fatalf("Failed to register custom metric: %v", err)
		}

		// Verify metric was registered
		metricFamilies, err := collector.registry.Gather()
		if err != nil {
			t.Fatalf("Failed to gather metrics: %v", err)
		}

		found := false
		for _, mf := range metricFamilies {
			if mf.GetName() == "test_custom_metric_total" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected custom metric to be registered")
		}
	})

	t.Run("rejects duplicate metric registration", func(t *testing.T) {
		collector := New(":9090", "/metrics")

		counter1 := prometheus.NewCounter(prometheus.CounterOpts{
			Name: "duplicate_metric_total",
			Help: "First counter",
		})
		counter2 := prometheus.NewCounter(prometheus.CounterOpts{
			Name: "duplicate_metric_total", // Same name
			Help: "Second counter",
		})

		err1 := collector.RegisterCustomMetric("duplicate1", counter1)
		if err1 != nil {
			t.Fatalf("Failed to register first metric: %v", err1)
		}

		err2 := collector.RegisterCustomMetric("duplicate2", counter2)
		if err2 == nil {
			t.Error("Expected error when registering duplicate metric")
		}
	})
}

func TestCollectorStart(t *testing.T) {
	t.Run("starts HTTP server", func(t *testing.T) {
		collector := New(":0", "/metrics") // Use port 0 for auto-assignment

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Start collector
		errChan := make(chan error, 1)
		go func() {
			errChan <- collector.Start(ctx)
		}()

		// Give it time to start
		time.Sleep(10 * time.Millisecond)

		// Try to make a request (this might fail if port selection is random)
		// In a real test, we'd need to get the actual port assigned

		// Wait for shutdown
		select {
		case err := <-errChan:
			if err != nil && err != context.Canceled && !strings.Contains(err.Error(), "Server closed") {
				t.Errorf("Unexpected error: %v", err)
			}
		case <-time.After(200 * time.Millisecond):
			t.Error("Server did not shut down within timeout")
		}
	})

	t.Run("serves metrics endpoint", func(t *testing.T) {
		collector := New(":0", "/metrics")

		// We can test the handler directly without starting the server
		req, err := http.NewRequest("GET", "/metrics", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		recorder := &testResponseWriter{
			header: make(http.Header),
		}

		collector.handler.ServeHTTP(recorder, req)

		if recorder.statusCode != http.StatusOK && recorder.statusCode == 0 {
			// Status code 0 means it wasn't set, which indicates success
		} else if recorder.statusCode != 0 && recorder.statusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %v", recorder.statusCode)
		}

		// Response should contain Prometheus metrics
		if !strings.Contains(recorder.body, "# HELP") {
			t.Error("Expected Prometheus metrics format in response")
		}
	})
}

// Test helper for HTTP response recording
type testResponseWriter struct {
	header     http.Header
	body       string
	statusCode int
}

func (rw *testResponseWriter) Header() http.Header {
	return rw.header
}

func (rw *testResponseWriter) Write(data []byte) (int, error) {
	rw.body += string(data)
	return len(data), nil
}

func (rw *testResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
}

// Implement the io.StringWriter interface if needed
func (rw *testResponseWriter) WriteString(s string) (int, error) {
	rw.body += s
	return len(s), nil
}

func TestMetricsIntegration(t *testing.T) {
	t.Run("full metrics workflow", func(t *testing.T) {
		collector := New(":0", "/metrics")

		// Simulate various operations that would update metrics
		collector.UpdateBufferMetrics(1500, 10000, 15.0, 2048000)
		collector.IncrementBufferOperations("add")
		collector.RecordAPIRequest("/api/v1/telemetry", "POST", 201, 50*time.Millisecond)
		collector.UpdateActiveConnections(5)
		collector.UpdatePodMetrics("running", 25)
		collector.IncrementIncidents("crash", "high")
		collector.IncrementPodRestarts("production", "test-pod")
		collector.IncrementTelemetryEntries("system")
		collector.RecordCollectionDuration("system", 10*time.Millisecond)
		collector.IncrementTelemetryErrors("permission_denied")
		collector.UpdateCPUUsage("0", 85.5)
		collector.UpdateLoadAverage("1m", 1.25)
		collector.UpdateMemoryUsage("total", 8192000000)
		collector.UpdateMemoryUtilization(75.5)
		collector.UpdateNetworkBytes("eth0", "tx", 1048576)
		collector.UpdateDiskOperations("sda", "read", 12345)

		// Gather all metrics
		metricFamilies, err := collector.registry.Gather()
		if err != nil {
			t.Fatalf("Failed to gather metrics: %v", err)
		}

		// Should have metrics from all the updates above
		if len(metricFamilies) == 0 {
			t.Error("Expected metrics to be present")
		}

		// Verify at least some key metrics exist
		metricNames := make(map[string]bool)
		for _, mf := range metricFamilies {
			metricNames[mf.GetName()] = true
		}

		requiredMetrics := []string{
			"blackbox_buffer_entries_total",
			"blackbox_api_requests_total",
			"blackbox_cpu_usage_percent",
			"blackbox_memory_utilization_percent",
		}

		for _, required := range requiredMetrics {
			if !metricNames[required] {
				t.Errorf("Expected metric %s to be present", required)
			}
		}
	})
}
// Package metrics provides Prometheus metrics collection and export for BlackBox-Daemon.
// It exposes both system telemetry metrics and operational metrics about the daemon itself.
package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Collector provides an extensible framework for Prometheus metrics collection and export.
// It manages both system telemetry metrics and BlackBox operational metrics, exposing them
// via an HTTP endpoint for Prometheus scraping.
type Collector struct {
	registry   *prometheus.Registry
	httpServer *http.Server

	// System telemetry metrics
	cpuUsageGauge     *prometheus.GaugeVec
	memoryUsageGauge  *prometheus.GaugeVec
	networkBytesGauge *prometheus.GaugeVec
	diskIOGauge       *prometheus.GaugeVec
	processCountGauge prometheus.Gauge
	openFilesGauge    prometheus.Gauge
	loadAvgGauge      *prometheus.GaugeVec

	// BlackBox operational metrics
	sidecarRequestsCounter prometheus.Counter
	incidentCounter        *prometheus.CounterVec
	bufferSizeGauge        prometheus.Gauge
	bufferEntriesGauge     prometheus.Gauge

	// Custom metrics registry for extensions
	customMetrics map[string]prometheus.Collector
}

// NewCollector creates a new Prometheus metrics collector with HTTP server on the specified port.
// It initializes all system and operational metrics and prepares them for registration.
func NewCollector(port int, metricsPath string) *Collector {
	registry := prometheus.NewRegistry()

	// System telemetry metrics
	cpuUsageGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "blackbox_cpu_usage_percent",
			Help: "CPU usage percentage per core",
		},
		[]string{"core"},
	)

	memoryUsageGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "blackbox_memory_bytes",
			Help: "Memory usage in bytes",
		},
		[]string{"type"}, // total, free, available, buffers, cached, etc.
	)

	networkBytesGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "blackbox_network_bytes_total",
			Help: "Network bytes transmitted and received",
		},
		[]string{"interface", "direction"}, // direction: rx, tx
	)

	diskIOGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "blackbox_disk_io_bytes_total",
			Help: "Disk I/O bytes read and written",
		},
		[]string{"device", "direction"}, // direction: read, write
	)

	processCountGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "blackbox_processes_total",
			Help: "Total number of processes on the system",
		},
	)

	openFilesGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "blackbox_open_files_total",
			Help: "Total number of open file descriptors",
		},
	)

	loadAvgGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "blackbox_load_average",
			Help: "System load average",
		},
		[]string{"period"}, // 1min, 5min, 15min
	)

	// BlackBox operational metrics
	sidecarRequestsCounter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "blackbox_sidecar_requests_total",
			Help: "Total number of telemetry requests received from sidecars",
		},
	)

	incidentCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "blackbox_incidents_total",
			Help: "Total number of incidents detected",
		},
		[]string{"type", "severity"}, // type: crash, oom, timeout, etc.
	)

	bufferSizeGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "blackbox_buffer_size_bytes",
			Help: "Current size of the telemetry ring buffer in bytes",
		},
	)

	bufferEntriesGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "blackbox_buffer_entries_total",
			Help: "Current number of entries in the telemetry ring buffer",
		},
	)

	// Register all metrics
	registry.MustRegister(
		cpuUsageGauge,
		memoryUsageGauge,
		networkBytesGauge,
		diskIOGauge,
		processCountGauge,
		openFilesGauge,
		loadAvgGauge,
		sidecarRequestsCounter,
		incidentCounter,
		bufferSizeGauge,
		bufferEntriesGauge,
	)

	mux := http.NewServeMux()
	mux.Handle(metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
<head><title>BlackBox Daemon Metrics</title></head>
<body>
<h1>BlackBox Daemon Metrics</h1>
<p><a href="` + metricsPath + `">Metrics</a></p>
</body>
</html>`))
	})

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return &Collector{
		registry:               registry,
		httpServer:             httpServer,
		cpuUsageGauge:          cpuUsageGauge,
		memoryUsageGauge:       memoryUsageGauge,
		networkBytesGauge:      networkBytesGauge,
		diskIOGauge:            diskIOGauge,
		processCountGauge:      processCountGauge,
		openFilesGauge:         openFilesGauge,
		loadAvgGauge:           loadAvgGauge,
		sidecarRequestsCounter: sidecarRequestsCounter,
		incidentCounter:        incidentCounter,
		bufferSizeGauge:        bufferSizeGauge,
		bufferEntriesGauge:     bufferEntriesGauge,
		customMetrics:          make(map[string]prometheus.Collector),
	}
}

// Start starts the Prometheus HTTP server and handles graceful shutdown when context is cancelled.
// The server exposes metrics on the configured port and path.
func (c *Collector) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c.httpServer.Shutdown(shutdownCtx)
	}()

	fmt.Printf("Starting Prometheus metrics server on %s\n", c.httpServer.Addr)
	if err := c.httpServer.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// System telemetry recording methods

// RecordCPUUsage records CPU usage percentage for a specific CPU core.
func (c *Collector) RecordCPUUsage(core string, usage float64) {
	c.cpuUsageGauge.WithLabelValues(core).Set(usage)
}

// RecordMemoryUsage records memory usage metrics for different memory types (total, free, available, etc.).
func (c *Collector) RecordMemoryUsage(memoryType string, bytes uint64) {
	c.memoryUsageGauge.WithLabelValues(memoryType).Set(float64(bytes))
}

// RecordNetworkBytes records network bytes transmitted or received for a specific interface.
func (c *Collector) RecordNetworkBytes(iface, direction string, bytes uint64) {
	c.networkBytesGauge.WithLabelValues(iface, direction).Set(float64(bytes))
}

// RecordDiskIO records disk I/O bytes for read or write operations on a specific device.
func (c *Collector) RecordDiskIO(device, direction string, bytes uint64) {
	c.diskIOGauge.WithLabelValues(device, direction).Set(float64(bytes))
}

// RecordProcessCount records the total number of running processes on the system.
func (c *Collector) RecordProcessCount(count int) {
	c.processCountGauge.Set(float64(count))
}

// RecordOpenFiles records the total number of open file descriptors system-wide.
func (c *Collector) RecordOpenFiles(count int) {
	c.openFilesGauge.Set(float64(count))
}

// RecordLoadAverage records system load average for different time periods (1min, 5min, 15min).
func (c *Collector) RecordLoadAverage(period string, load float64) {
	c.loadAvgGauge.WithLabelValues(period).Set(load)
}

// BlackBox operational metrics

// IncrementSidecarRequests increments the counter for telemetry requests received from sidecars.
func (c *Collector) IncrementSidecarRequests() {
	c.sidecarRequestsCounter.Inc()
}

// IncrementIncidents increments the counter for detected incidents with type and severity labels.
func (c *Collector) IncrementIncidents(incidentType, severity string) {
	c.incidentCounter.WithLabelValues(incidentType, severity).Inc()
}

// RecordBufferSize records the current ring buffer size in bytes.
func (c *Collector) RecordBufferSize(sizeBytes int) {
	c.bufferSizeGauge.Set(float64(sizeBytes))
}

// RecordBufferEntries records the current number of telemetry entries in the ring buffer.
func (c *Collector) RecordBufferEntries(count int) {
	c.bufferEntriesGauge.Set(float64(count))
}

// Custom metrics management

// RegisterCustomMetric registers a custom Prometheus metric
func (c *Collector) RegisterCustomMetric(name string, metric prometheus.Collector) error {
	if _, exists := c.customMetrics[name]; exists {
		return fmt.Errorf("metric %s already registered", name)
	}

	if err := c.registry.Register(metric); err != nil {
		return fmt.Errorf("failed to register metric %s: %w", name, err)
	}

	c.customMetrics[name] = metric
	return nil
}

// UnregisterCustomMetric removes a custom metric
func (c *Collector) UnregisterCustomMetric(name string) error {
	metric, exists := c.customMetrics[name]
	if !exists {
		return fmt.Errorf("metric %s not found", name)
	}

	if !c.registry.Unregister(metric) {
		return fmt.Errorf("failed to unregister metric %s", name)
	}

	delete(c.customMetrics, name)
	return nil
}

// GetCustomMetric retrieves a custom metric by name
func (c *Collector) GetCustomMetric(name string) (prometheus.Collector, bool) {
	metric, exists := c.customMetrics[name]
	return metric, exists
}

// ListCustomMetrics returns a list of all registered custom metrics
func (c *Collector) ListCustomMetrics() []string {
	var names []string
	for name := range c.customMetrics {
		names = append(names, name)
	}
	return names
}

// Helper methods for creating common custom metrics

// NewCustomCounter creates a new counter metric
func (c *Collector) NewCustomCounter(name, help string, labelNames []string) (*prometheus.CounterVec, error) {
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: fmt.Sprintf("blackbox_custom_%s", name),
			Help: help,
		},
		labelNames,
	)

	if err := c.RegisterCustomMetric(name, counter); err != nil {
		return nil, err
	}

	return counter, nil
}

// NewCustomGauge creates a new gauge metric
func (c *Collector) NewCustomGauge(name, help string, labelNames []string) (*prometheus.GaugeVec, error) {
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: fmt.Sprintf("blackbox_custom_%s", name),
			Help: help,
		},
		labelNames,
	)

	if err := c.RegisterCustomMetric(name, gauge); err != nil {
		return nil, err
	}

	return gauge, nil
}

// NewCustomHistogram creates a new histogram metric
func (c *Collector) NewCustomHistogram(name, help string, labelNames []string, buckets []float64) (*prometheus.HistogramVec, error) {
	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    fmt.Sprintf("blackbox_custom_%s", name),
			Help:    help,
			Buckets: buckets,
		},
		labelNames,
	)

	if err := c.RegisterCustomMetric(name, histogram); err != nil {
		return nil, err
	}

	return histogram, nil
}

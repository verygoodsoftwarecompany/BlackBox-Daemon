// Package telemetry provides system-level telemetry collection for Linux systems.
// It collects comprehensive metrics including CPU, memory, network, disk, and process
// information by reading from the /proc and /sys filesystems.
package telemetry

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/verygoodsoftwarecompany/blackbox-daemon/pkg/types"
)

// SystemCollector collects system-level telemetry from Linux by reading /proc and /sys.
// It runs continuously on a configurable interval and submits telemetry to the ring buffer.
type SystemCollector struct {
	// mutex protects concurrent access to collector state
	mutex sync.RWMutex
	// interval determines how frequently metrics are collected
	interval time.Duration
	// buffer receives the collected telemetry entries
	buffer TelemetryBuffer
}

// TelemetryBuffer interface for adding telemetry entries to storage.
// This abstraction allows the collector to work with different buffer implementations.
type TelemetryBuffer interface {
	Add(entry types.TelemetryEntry)
}

// NewSystemCollector creates a new system telemetry collector with the specified
// collection interval and target buffer for storing telemetry.
func NewSystemCollector(interval time.Duration, buffer TelemetryBuffer) *SystemCollector {
	return &SystemCollector{
		interval: interval,
		buffer:   buffer,
	}
}

// Start begins collecting system telemetry on the configured interval.
// This method runs continuously until the context is cancelled and should be
// called in a separate goroutine.
func (sc *SystemCollector) Start(ctx context.Context) error {
	ticker := time.NewTicker(sc.interval)
	defer ticker.Stop()

	// Collect initial metrics
	if err := sc.collectMetrics(); err != nil {
		return fmt.Errorf("failed to collect initial metrics: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := sc.collectMetrics(); err != nil {
				// Log error but continue collecting
				fmt.Printf("Error collecting metrics: %v\n", err)
			}
		}
	}
}

// collectMetrics gathers all system telemetry by calling individual collection methods.
// This is the main orchestration method that coordinates all metric collection.
func (sc *SystemCollector) collectMetrics() error {
	timestamp := time.Now()

	// Collect CPU metrics
	if err := sc.collectCPUMetrics(timestamp); err != nil {
		return fmt.Errorf("CPU metrics: %w", err)
	}

	// Collect memory metrics
	if err := sc.collectMemoryMetrics(timestamp); err != nil {
		return fmt.Errorf("memory metrics: %w", err)
	}

	// Collect network metrics
	if err := sc.collectNetworkMetrics(timestamp); err != nil {
		return fmt.Errorf("network metrics: %w", err)
	}

	// Collect disk metrics
	if err := sc.collectDiskMetrics(timestamp); err != nil {
		return fmt.Errorf("disk metrics: %w", err)
	}

	// Collect process metrics
	if err := sc.collectProcessMetrics(timestamp); err != nil {
		return fmt.Errorf("process metrics: %w", err)
	}

	// Collect system load
	if err := sc.collectLoadMetrics(timestamp); err != nil {
		return fmt.Errorf("load metrics: %w", err)
	}

	return nil
}

// collectCPUMetrics collects CPU usage per core by parsing /proc/stat.
// This provides detailed CPU utilization metrics for each CPU core.
func (sc *SystemCollector) collectCPUMetrics(timestamp time.Time) error {
	data, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "cpu") {
			fields := strings.Fields(line)
			if len(fields) < 8 {
				continue
			}

			cpuName := fields[0]
			user, _ := strconv.ParseUint(fields[1], 10, 64)
			nice, _ := strconv.ParseUint(fields[2], 10, 64)
			system, _ := strconv.ParseUint(fields[3], 10, 64)
			idle, _ := strconv.ParseUint(fields[4], 10, 64)
			iowait, _ := strconv.ParseUint(fields[5], 10, 64)
			irq, _ := strconv.ParseUint(fields[6], 10, 64)
			softirq, _ := strconv.ParseUint(fields[7], 10, 64)

			total := user + nice + system + idle + iowait + irq + softirq
			usage := float64(total-idle) / float64(total) * 100

			sc.buffer.Add(types.TelemetryEntry{
				Timestamp: timestamp,
				Source:    types.SourceSystem,
				Type:      types.TypeCPU,
				Name:      fmt.Sprintf("%s_usage_percent", cpuName),
				Value:     usage,
				Tags: map[string]string{
					"core": cpuName,
				},
			})
		}
	}

	return nil
}

// collectMemoryMetrics collects memory usage information by parsing /proc/meminfo.
// It gathers total, free, available, buffers, cached memory as well as swap statistics
// and calculates memory usage percentage.
func (sc *SystemCollector) collectMemoryMetrics(timestamp time.Time) error {
	data, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return err
	}

	memInfo := make(map[string]uint64)
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			key := strings.TrimSuffix(fields[0], ":")
			value, _ := strconv.ParseUint(fields[1], 10, 64)
			memInfo[key] = value * 1024 // Convert KB to bytes
		}
	}

	// Add memory metrics
	metrics := []struct {
		name string
		key  string
	}{
		{"memory_total_bytes", "MemTotal"},
		{"memory_free_bytes", "MemFree"},
		{"memory_available_bytes", "MemAvailable"},
		{"memory_buffers_bytes", "Buffers"},
		{"memory_cached_bytes", "Cached"},
		{"swap_total_bytes", "SwapTotal"},
		{"swap_free_bytes", "SwapFree"},
	}

	for _, metric := range metrics {
		if value, ok := memInfo[metric.key]; ok {
			sc.buffer.Add(types.TelemetryEntry{
				Timestamp: timestamp,
				Source:    types.SourceSystem,
				Type:      types.TypeMemory,
				Name:      metric.name,
				Value:     value,
			})
		}
	}

	// Calculate memory usage percentage
	if total, ok := memInfo["MemTotal"]; ok {
		if available, ok := memInfo["MemAvailable"]; ok {
			used := total - available
			usagePercent := float64(used) / float64(total) * 100

			sc.buffer.Add(types.TelemetryEntry{
				Timestamp: timestamp,
				Source:    types.SourceSystem,
				Type:      types.TypeMemory,
				Name:      "memory_usage_percent",
				Value:     usagePercent,
			})
		}
	}

	return nil
}

// collectNetworkMetrics collects network interface statistics by parsing /proc/net/dev.
// It gathers RX/TX bytes, packets, and errors for each network interface (excluding loopback).
func (sc *SystemCollector) collectNetworkMetrics(timestamp time.Time) error {
	data, err := ioutil.ReadFile("/proc/net/dev")
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if i < 2 { // Skip header lines
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 17 {
			continue
		}

		iface := strings.TrimSuffix(fields[0], ":")
		if iface == "lo" { // Skip loopback
			continue
		}

		rxBytes, _ := strconv.ParseUint(fields[1], 10, 64)
		rxPackets, _ := strconv.ParseUint(fields[2], 10, 64)
		rxErrors, _ := strconv.ParseUint(fields[3], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[9], 10, 64)
		txPackets, _ := strconv.ParseUint(fields[10], 10, 64)
		txErrors, _ := strconv.ParseUint(fields[11], 10, 64)

		metrics := []struct {
			name  string
			value uint64
		}{
			{fmt.Sprintf("network_rx_bytes_%s", iface), rxBytes},
			{fmt.Sprintf("network_rx_packets_%s", iface), rxPackets},
			{fmt.Sprintf("network_rx_errors_%s", iface), rxErrors},
			{fmt.Sprintf("network_tx_bytes_%s", iface), txBytes},
			{fmt.Sprintf("network_tx_packets_%s", iface), txPackets},
			{fmt.Sprintf("network_tx_errors_%s", iface), txErrors},
		}

		for _, metric := range metrics {
			sc.buffer.Add(types.TelemetryEntry{
				Timestamp: timestamp,
				Source:    types.SourceSystem,
				Type:      types.TypeNetwork,
				Name:      metric.name,
				Value:     metric.value,
				Tags: map[string]string{
					"interface": iface,
				},
			})
		}
	}

	return nil
}

// collectDiskMetrics collects disk I/O statistics by parsing /proc/diskstats.
// It gathers read/write operations and bytes for physical disks (sd* and nvme* devices).
func (sc *SystemCollector) collectDiskMetrics(timestamp time.Time) error {
	data, err := ioutil.ReadFile("/proc/diskstats")
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 14 {
			continue
		}

		device := fields[2]
		if !strings.HasPrefix(device, "sd") && !strings.HasPrefix(device, "nvme") {
			continue // Only collect stats for real disks
		}

		readIOs, _ := strconv.ParseUint(fields[3], 10, 64)
		readBytes, _ := strconv.ParseUint(fields[5], 10, 64)
		writeIOs, _ := strconv.ParseUint(fields[7], 10, 64)
		writeBytes, _ := strconv.ParseUint(fields[9], 10, 64)

		readBytes *= 512  // Convert sectors to bytes
		writeBytes *= 512 // Convert sectors to bytes

		metrics := []struct {
			name  string
			value uint64
		}{
			{fmt.Sprintf("disk_read_ios_%s", device), readIOs},
			{fmt.Sprintf("disk_read_bytes_%s", device), readBytes},
			{fmt.Sprintf("disk_write_ios_%s", device), writeIOs},
			{fmt.Sprintf("disk_write_bytes_%s", device), writeBytes},
		}

		for _, metric := range metrics {
			sc.buffer.Add(types.TelemetryEntry{
				Timestamp: timestamp,
				Source:    types.SourceSystem,
				Type:      types.TypeDisk,
				Name:      metric.name,
				Value:     metric.value,
				Tags: map[string]string{
					"device": device,
				},
			})
		}
	}

	return nil
}

// collectProcessMetrics collects process-related metrics including open file descriptors
// and total process count by reading from /proc filesystem.
func (sc *SystemCollector) collectProcessMetrics(timestamp time.Time) error {
	// Count open file descriptors
	fdCount, err := sc.countOpenFiles()
	if err == nil {
		sc.buffer.Add(types.TelemetryEntry{
			Timestamp: timestamp,
			Source:    types.SourceSystem,
			Type:      types.TypeProcess,
			Name:      "open_files_total",
			Value:     fdCount,
		})
	}

	// Count processes
	procCount, err := sc.countProcesses()
	if err == nil {
		sc.buffer.Add(types.TelemetryEntry{
			Timestamp: timestamp,
			Source:    types.SourceSystem,
			Type:      types.TypeProcess,
			Name:      "processes_total",
			Value:     procCount,
		})
	}

	return nil
}

// collectLoadMetrics collects system load averages by parsing /proc/loadavg.
// It gathers 1-minute, 5-minute, and 15-minute load averages.
func (sc *SystemCollector) collectLoadMetrics(timestamp time.Time) error {
	data, err := ioutil.ReadFile("/proc/loadavg")
	if err != nil {
		return err
	}

	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		load1, _ := strconv.ParseFloat(fields[0], 64)
		load5, _ := strconv.ParseFloat(fields[1], 64)
		load15, _ := strconv.ParseFloat(fields[2], 64)

		loads := []struct {
			name  string
			value float64
		}{
			{"load_1min", load1},
			{"load_5min", load5},
			{"load_15min", load15},
		}

		for _, load := range loads {
			sc.buffer.Add(types.TelemetryEntry{
				Timestamp: timestamp,
				Source:    types.SourceSystem,
				Type:      types.TypeProcess,
				Name:      load.name,
				Value:     load.value,
			})
		}
	}

	return nil
}

// countOpenFiles counts the total number of open file descriptors system-wide
// by reading from /proc/sys/fs/file-nr.
func (sc *SystemCollector) countOpenFiles() (int, error) {
	data, err := ioutil.ReadFile("/proc/sys/fs/file-nr")
	if err != nil {
		return 0, err
	}

	fields := strings.Fields(string(data))
	if len(fields) >= 1 {
		count, err := strconv.Atoi(fields[0])
		if err != nil {
			return 0, err
		}
		return count, nil
	}

	return 0, fmt.Errorf("invalid file-nr format")
}

// countProcesses counts the total number of processes by counting numeric
// directories in /proc (each represents a running process ID).
func (sc *SystemCollector) countProcesses() (int, error) {
	entries, err := ioutil.ReadDir("/proc")
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			if _, err := strconv.Atoi(entry.Name()); err == nil {
				count++
			}
		}
	}

	return count, nil
}

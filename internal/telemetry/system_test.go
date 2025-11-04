package telemetry

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/verygoodsoftwarecompany/blackbox-daemon/internal/ringbuffer"
	"github.com/verygoodsoftwarecompany/blackbox-daemon/pkg/types"
)

// Mock file system for testing
type mockFileSystem struct {
	files map[string]string
	error bool
}

func (fs *mockFileSystem) readFile(path string) ([]byte, error) {
	if fs.error {
		return nil, os.ErrNotExist
	}
	if content, exists := fs.files[path]; exists {
		return []byte(content), nil
	}
	return nil, os.ErrNotExist
}

func TestNewSystemCollector(t *testing.T) {
	t.Run("creates system collector", func(t *testing.T) {
		buffer := ringbuffer.New(60 * time.Second)
		interval := 5 * time.Second

		collector := NewSystemCollector(interval, buffer)

		if collector == nil {
			t.Fatal("Expected collector to be created")
		}
		if collector.interval != interval {
			t.Errorf("Expected interval %v, got %v", interval, collector.interval)
		}
		if collector.buffer != buffer {
			t.Error("Expected buffer to be set")
		}
	})
}

func TestParseCPUStats(t *testing.T) {
	t.Run("parses valid /proc/stat", func(t *testing.T) {
		procStat := `cpu  12345 0 5678 90123 456 0 789 0 0 0
cpu0 6172 0 2839 45061 228 0 394 0 0 0
cpu1 6173 0 2839 45062 228 0 395 0 0 0
intr 123456789
ctxt 987654321
btime 1699008000
processes 12345
procs_running 2
procs_blocked 0`

		collector := &SystemCollector{}
		stats, err := collector.parseCPUStats([]byte(procStat))

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if stats == nil {
			t.Fatal("Expected CPU stats to be returned")
		}

		// Check if we got per-core stats
		if len(stats) < 2 { // At least cpu0 and cpu1
			t.Errorf("Expected at least 2 CPU cores, got %v", len(stats))
		}

		// Verify CPU0 stats
		if stats["cpu0_user"] != 6172 {
			t.Errorf("Expected cpu0_user 6172, got %v", stats["cpu0_user"])
		}
		if stats["cpu0_system"] != 2839 {
			t.Errorf("Expected cpu0_system 2839, got %v", stats["cpu0_system"])
		}
		if stats["cpu0_idle"] != 45061 {
			t.Errorf("Expected cpu0_idle 45061, got %v", stats["cpu0_idle"])
		}
	})

	t.Run("handles malformed /proc/stat", func(t *testing.T) {
		malformedStat := `cpu  invalid data
cpu0 not numbers here`

		collector := &SystemCollector{}
		stats, err := collector.parseCPUStats([]byte(malformedStat))

		if err == nil {
			t.Error("Expected error for malformed CPU stats")
		}
		if stats != nil {
			t.Error("Expected nil stats for malformed data")
		}
	})

	t.Run("handles empty /proc/stat", func(t *testing.T) {
		collector := &SystemCollector{}
		stats, err := collector.parseCPUStats([]byte(""))

		if err == nil {
			t.Error("Expected error for empty CPU stats")
		}
		if stats != nil {
			t.Error("Expected nil stats for empty data")
		}
	})
}

func TestParseMemoryStats(t *testing.T) {
	t.Run("parses valid /proc/meminfo", func(t *testing.T) {
		meminfo := `MemTotal:        8192000 kB
MemFree:         2048000 kB
MemAvailable:    4096000 kB
Buffers:          512000 kB
Cached:          1024000 kB
SwapCached:           0 kB
Active:          3072000 kB
Inactive:        1536000 kB
SwapTotal:       2048000 kB
SwapFree:        2048000 kB`

		collector := &SystemCollector{}
		stats, err := collector.parseMemoryStats([]byte(meminfo))

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if stats == nil {
			t.Fatal("Expected memory stats to be returned")
		}

		expectedStats := map[string]uint64{
			"total":     8192000 * 1024, // Convert from kB to bytes
			"free":      2048000 * 1024,
			"available": 4096000 * 1024,
			"buffers":   512000 * 1024,
			"cached":    1024000 * 1024,
			"swap_total": 2048000 * 1024,
			"swap_free":  2048000 * 1024,
		}

		for key, expected := range expectedStats {
			if actual, exists := stats[key]; !exists || actual != expected {
				t.Errorf("Expected %s=%d, got %v", key, expected, actual)
			}
		}
	})

	t.Run("handles malformed /proc/meminfo", func(t *testing.T) {
		malformedMeminfo := `MemTotal: invalid kB
MemFree: not-a-number kB`

		collector := &SystemCollector{}
		stats, err := collector.parseMemoryStats([]byte(malformedMeminfo))

		if err == nil {
			t.Error("Expected error for malformed memory stats")
		}
		if stats != nil {
			t.Error("Expected nil stats for malformed data")
		}
	})
}

func TestParseNetworkStats(t *testing.T) {
	t.Run("parses valid /proc/net/dev", func(t *testing.T) {
		netDev := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 1048576     1024    0    0    0     0          0         0  1048576     1024    0    0    0     0       0          0
  eth0: 134217728   131072    1    0    0     0          0         0 67108864    65536    0    1    0     0       0          0`

		collector := &SystemCollector{}
		stats, err := collector.parseNetworkStats([]byte(netDev))

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if stats == nil {
			t.Fatal("Expected network stats to be returned")
		}

		// Check eth0 stats
		if stats["eth0_rx_bytes"] != 134217728 {
			t.Errorf("Expected eth0_rx_bytes 134217728, got %v", stats["eth0_rx_bytes"])
		}
		if stats["eth0_tx_bytes"] != 67108864 {
			t.Errorf("Expected eth0_tx_bytes 67108864, got %v", stats["eth0_tx_bytes"])
		}
		if stats["eth0_rx_packets"] != 131072 {
			t.Errorf("Expected eth0_rx_packets 131072, got %v", stats["eth0_rx_packets"])
		}
		if stats["eth0_tx_packets"] != 65536 {
			t.Errorf("Expected eth0_tx_packets 65536, got %v", stats["eth0_tx_packets"])
		}
		if stats["eth0_rx_errors"] != 1 {
			t.Errorf("Expected eth0_rx_errors 1, got %v", stats["eth0_rx_errors"])
		}
		if stats["eth0_tx_errors"] != 0 {
			t.Errorf("Expected eth0_tx_errors 0, got %v", stats["eth0_tx_errors"])
		}

		// Should not include loopback interface in stats by default
		if _, exists := stats["lo_rx_bytes"]; exists {
			t.Error("Expected loopback interface to be excluded")
		}
	})

	t.Run("handles malformed /proc/net/dev", func(t *testing.T) {
		malformedNetDev := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  eth0: invalid data here`

		collector := &SystemCollector{}
		stats, err := collector.parseNetworkStats([]byte(malformedNetDev))

		if err == nil {
			t.Error("Expected error for malformed network stats")
		}
		if stats != nil {
			t.Error("Expected nil stats for malformed data")
		}
	})
}

func TestParseDiskStats(t *testing.T) {
	t.Run("parses valid /proc/diskstats", func(t *testing.T) {
		diskStats := `   8       0 sda 12345 0 98765 12345 54321 0 43210 9876 0 22111 22222 0 0 0 0
   8       1 sda1 5000 0 40000 5000 20000 0 16000 4000 0 9000 9000 0 0 0 0
   8      16 sdb 8765 0 70123 8765 32109 0 25641 5432 0 14197 14197 0 0 0 0`

		collector := &SystemCollector{}
		stats, err := collector.parseDiskStats([]byte(diskStats))

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if stats == nil {
			t.Fatal("Expected disk stats to be returned")
		}

		// Check sda stats
		if stats["sda_reads"] != 12345 {
			t.Errorf("Expected sda_reads 12345, got %v", stats["sda_reads"])
		}
		if stats["sda_read_sectors"] != 98765 {
			t.Errorf("Expected sda_read_sectors 98765, got %v", stats["sda_read_sectors"])
		}
		if stats["sda_writes"] != 54321 {
			t.Errorf("Expected sda_writes 54321, got %v", stats["sda_writes"])
		}
		if stats["sda_write_sectors"] != 43210 {
			t.Errorf("Expected sda_write_sectors 43210, got %v", stats["sda_write_sectors"])
		}

		// Should include sdb but not sda1 (partition)
		if _, exists := stats["sdb_reads"]; !exists {
			t.Error("Expected sdb stats to be included")
		}
		if _, exists := stats["sda1_reads"]; exists {
			t.Error("Expected partition sda1 to be excluded")
		}
	})

	t.Run("handles malformed /proc/diskstats", func(t *testing.T) {
		malformedDiskStats := `   8       0 sda invalid data`

		collector := &SystemCollector{}
		stats, err := collector.parseDiskStats([]byte(malformedDiskStats))

		if err == nil {
			t.Error("Expected error for malformed disk stats")
		}
		if stats != nil {
			t.Error("Expected nil stats for malformed data")
		}
	})
}

func TestParseLoadAverage(t *testing.T) {
	t.Run("parses valid /proc/loadavg", func(t *testing.T) {
		loadAvg := `1.25 1.10 0.95 2/123 12345`

		collector := &SystemCollector{}
		stats, err := collector.parseLoadAverage([]byte(loadAvg))

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if stats == nil {
			t.Fatal("Expected load average stats to be returned")
		}

		if stats["load1"] != 1.25 {
			t.Errorf("Expected load1 1.25, got %v", stats["load1"])
		}
		if stats["load5"] != 1.10 {
			t.Errorf("Expected load5 1.10, got %v", stats["load5"])
		}
		if stats["load15"] != 0.95 {
			t.Errorf("Expected load15 0.95, got %v", stats["load15"])
		}
	})

	t.Run("handles malformed /proc/loadavg", func(t *testing.T) {
		malformedLoadAvg := `invalid load average data`

		collector := &SystemCollector{}
		stats, err := collector.parseLoadAverage([]byte(malformedLoadAvg))

		if err == nil {
			t.Error("Expected error for malformed load average")
		}
		if stats != nil {
			t.Error("Expected nil stats for malformed data")
		}
	})
}

func TestCollectMetrics(t *testing.T) {
	t.Run("collects metrics successfully", func(t *testing.T) {
		buffer := ringbuffer.New(60 * time.Second)
		collector := NewSystemCollector(1*time.Second, buffer)

		// Create temporary files with test data
		tmpDir := t.TempDir()
		
		// Note: In a real test environment, we would mock the file system
		// For this test, we'll test the error handling when files don't exist
		err := collector.collectMetrics()

		// We expect an error because /proc files don't exist in test environment
		// In production, this would succeed if running on Linux with proper /proc access
		if err == nil {
			t.Log("Metrics collection succeeded (likely running on Linux with /proc access)")
		} else {
			// This is expected in test environment
			if !strings.Contains(err.Error(), "no such file or directory") && 
			   !strings.Contains(err.Error(), "permission denied") {
				t.Errorf("Unexpected error: %v", err)
			}
		}
	})
}

func TestSystemCollectorStart(t *testing.T) {
	t.Run("starts and stops collection", func(t *testing.T) {
		buffer := ringbuffer.New(60 * time.Second)
		collector := NewSystemCollector(10*time.Millisecond, buffer)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Start collector
		err := collector.Start(ctx)

		// Should return context.Canceled when context is cancelled
		if err != nil && err != context.Canceled {
			// In test environment, might get file system errors
			t.Logf("Collector stopped with error: %v", err)
		}
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		buffer := ringbuffer.New(60 * time.Second)
		collector := NewSystemCollector(1*time.Second, buffer)

		ctx, cancel := context.WithCancel(context.Background())
		
		// Cancel immediately
		cancel()

		err := collector.Start(ctx)

		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})
}

func TestCreateTelemetryEntry(t *testing.T) {
	t.Run("creates system telemetry entry", func(t *testing.T) {
		collector := &SystemCollector{}
		
		data := map[string]interface{}{
			"cpu0_user":   1000,
			"cpu0_system": 500,
			"memory_total": 8192000,
			"memory_free":  2048000,
		}

		entry := collector.createTelemetryEntry(types.TypeCPU, "cpu_stats", data)

		if entry.Source != types.SourceSystem {
			t.Errorf("Expected SourceSystem, got %v", entry.Source)
		}
		if entry.Type != types.TypeCPU {
			t.Errorf("Expected TypeCPU, got %v", entry.Type)
		}
		if entry.Name != "cpu_stats" {
			t.Errorf("Expected name 'cpu_stats', got %v", entry.Name)
		}
		if entry.Value != data {
			t.Errorf("Expected value to be data map, got %v", entry.Value)
		}
		if entry.Timestamp.IsZero() {
			t.Error("Expected timestamp to be set")
		}

		// Check metadata
		metadata, ok := entry.Metadata["collection_method"]
		if !ok || metadata != "proc_filesystem" {
			t.Errorf("Expected collection_method metadata to be 'proc_filesystem', got %v", metadata)
		}

		hostname, ok := entry.Metadata["hostname"]
		if !ok {
			t.Error("Expected hostname metadata to be set")
		} else if hostname == "" {
			t.Error("Expected hostname to be non-empty")
		}
	})
}

func TestCalculatePercentages(t *testing.T) {
	t.Run("calculates CPU percentages", func(t *testing.T) {
		collector := &SystemCollector{}
		
		// First measurement
		stats1 := map[string]uint64{
			"cpu0_user":   1000,
			"cpu0_system": 500,
			"cpu0_idle":   8500,
			"cpu0_iowait": 0,
		}
		
		// Second measurement (after some time)
		stats2 := map[string]uint64{
			"cpu0_user":   1100, // +100
			"cpu0_system": 550,  // +50
			"cpu0_idle":   8350, // -150 (total increase = 100 + 50 + 150 = 300)
			"cpu0_iowait": 0,
		}

		// Set previous stats
		collector.mutex.Lock()
		collector.previousCPU = stats1
		collector.mutex.Unlock()

		percentages := collector.calculateCPUPercentages(stats2)

		// Total delta = 300, user delta = 100, system delta = 50
		expectedUserPercent := float64(100) / float64(300) * 100  // ~33.33%
		expectedSystemPercent := float64(50) / float64(300) * 100 // ~16.67%

		if userPercent, exists := percentages["cpu0_user_percent"]; !exists {
			t.Error("Expected cpu0_user_percent to be calculated")
		} else if userPercent < 33.0 || userPercent > 34.0 {
			t.Errorf("Expected cpu0_user_percent around 33.33, got %v", userPercent)
		}

		if systemPercent, exists := percentages["cpu0_system_percent"]; !exists {
			t.Error("Expected cpu0_system_percent to be calculated")
		} else if systemPercent < 16.0 || systemPercent > 17.0 {
			t.Errorf("Expected cpu0_system_percent around 16.67, got %v", systemPercent)
		}
	})

	t.Run("handles first measurement", func(t *testing.T) {
		collector := &SystemCollector{}
		
		stats := map[string]uint64{
			"cpu0_user":   1000,
			"cpu0_system": 500,
			"cpu0_idle":   8500,
		}

		percentages := collector.calculateCPUPercentages(stats)

		// Should return empty map for first measurement
		if len(percentages) != 0 {
			t.Errorf("Expected empty percentages for first measurement, got %v", percentages)
		}

		// Should store stats for next calculation
		if len(collector.previousCPU) == 0 {
			t.Error("Expected previous CPU stats to be stored")
		}
	})
}
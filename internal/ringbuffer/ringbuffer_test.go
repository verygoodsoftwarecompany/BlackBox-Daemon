// Package ringbuffer provides comprehensive unit tests for the thread-safe circular buffer
// implementation used for telemetry data storage with automatic time-based expiration.
package ringbuffer

import (
	"sync"
	"testing"
	"time"

	"github.com/verygoodsoftwarecompany/blackbox-daemon/pkg/types"
)

// TestNew validates the ring buffer creation and initialization logic.
// This test ensures the buffer is properly sized based on window duration and
// that all internal structures are correctly initialized for thread-safe operations.
func TestNew(t *testing.T) {
	t.Run("creates ring buffer with correct window size", func(t *testing.T) {
		windowSize := 60 * time.Second
		rb := New(windowSize)

		if rb.windowSize != windowSize {
			t.Errorf("Expected window size %v, got %v", windowSize, rb.windowSize)
		}
		if rb.size <= 0 {
			t.Errorf("Expected positive size, got %v", rb.size)
		}
		if rb.count != 0 {
			t.Errorf("Expected count 0, got %v", rb.count)
		}
		if rb.head != 0 {
			t.Errorf("Expected head 0, got %v", rb.head)
		}
	})

	t.Run("minimum buffer size", func(t *testing.T) {
		windowSize := 100 * time.Millisecond
		rb := New(windowSize)

		if rb.size < 1000 {
			t.Errorf("Expected minimum size 1000, got %v", rb.size)
		}
	})

	t.Run("estimated size calculation", func(t *testing.T) {
		windowSize := 10 * time.Second
		rb := New(windowSize)

		expectedSize := 10000 // 10 seconds * 1000 entries/second
		if rb.size != expectedSize {
			t.Errorf("Expected size %v, got %v", expectedSize, rb.size)
		}
	})
}

func TestAdd(t *testing.T) {
	t.Run("adds entry to empty buffer", func(t *testing.T) {
		rb := New(60 * time.Second)
		entry := types.TelemetryEntry{
			Timestamp: time.Now(),
			Source:    types.SourceSystem,
			Type:      types.TypeCPU,
			Name:      "test_metric",
			Value:     42.5,
		}

		rb.Add(entry)

		if rb.count != 1 {
			t.Errorf("Expected count 1, got %v", rb.count)
		}
		if rb.head != 1 {
			t.Errorf("Expected head 1, got %v", rb.head)
		}

		// Check that the entry was stored correctly
		rb.mutex.RLock()
		storedEntry := rb.entries[0]
		rb.mutex.RUnlock()

		if storedEntry.Name != entry.Name {
			t.Errorf("Expected name %v, got %v", entry.Name, storedEntry.Name)
		}
		if storedEntry.Value != entry.Value {
			t.Errorf("Expected value %v, got %v", entry.Value, storedEntry.Value)
		}
	})

	t.Run("adds multiple entries", func(t *testing.T) {
		rb := New(60 * time.Second)

		for i := 0; i < 5; i++ {
			entry := types.TelemetryEntry{
				Timestamp: time.Now(),
				Source:    types.SourceSystem,
				Type:      types.TypeCPU,
				Name:      "test_metric",
				Value:     float64(i),
			}
			rb.Add(entry)
		}

		if rb.count != 5 {
			t.Errorf("Expected count 5, got %v", rb.count)
		}
		if rb.head != 5 {
			t.Errorf("Expected head 5, got %v", rb.head)
		}
	})

	t.Run("wraps around when buffer is full", func(t *testing.T) {
		rb := New(1 * time.Millisecond) // Small window to get small buffer
		originalSize := rb.size

		// Fill buffer beyond capacity
		for i := 0; i < originalSize+10; i++ {
			entry := types.TelemetryEntry{
				Timestamp: time.Now(),
				Source:    types.SourceSystem,
				Type:      types.TypeCPU,
				Name:      "test_metric",
				Value:     float64(i),
			}
			rb.Add(entry)
		}

		if rb.count != originalSize {
			t.Errorf("Expected count %v, got %v", originalSize, rb.count)
		}
		if rb.head != (originalSize+10)%originalSize {
			t.Errorf("Expected head %v, got %v", (originalSize+10)%originalSize, rb.head)
		}
	})
}

func TestGetWindow(t *testing.T) {
	t.Run("returns entries in time range", func(t *testing.T) {
		rb := New(60 * time.Second)
		baseTime := time.Now()

		// Add entries with different timestamps
		for i := 0; i < 5; i++ {
			entry := types.TelemetryEntry{
				Timestamp: baseTime.Add(time.Duration(i) * time.Second),
				Source:    types.SourceSystem,
				Type:      types.TypeCPU,
				Name:      "test_metric",
				Value:     float64(i),
			}
			rb.Add(entry)
		}

		// Query for entries between 1-3 seconds
		since := baseTime.Add(1 * time.Second)
		until := baseTime.Add(3 * time.Second)
		entries := rb.GetByTimeRange(since, until)

		expectedCount := 3 // entries at 1s, 2s, 3s
		if len(entries) != expectedCount {
			t.Errorf("Expected %v entries, got %v", expectedCount, len(entries))
		}

		// Verify entries are in correct order and range
		for i, entry := range entries {
			expectedValue := float64(i + 1) // Values 1, 2, 3
			if entry.Value != expectedValue {
				t.Errorf("Expected value %v, got %v", expectedValue, entry.Value)
			}
			if entry.Timestamp.Before(since) || entry.Timestamp.After(until) {
				t.Errorf("Entry timestamp %v not in range %v - %v", entry.Timestamp, since, until)
			}
		}
	})

	t.Run("returns empty slice for no matches", func(t *testing.T) {
		rb := New(60 * time.Second)
		baseTime := time.Now()

		entry := types.TelemetryEntry{
			Timestamp: baseTime,
			Source:    types.SourceSystem,
			Type:      types.TypeCPU,
			Name:      "test_metric",
			Value:     42.0,
		}
		rb.Add(entry)

		// Query for time range that doesn't include the entry
		since := baseTime.Add(10 * time.Second)
		until := baseTime.Add(20 * time.Second)
		entries := rb.GetByTimeRange(since, until)

		if len(entries) != 0 {
			t.Errorf("Expected 0 entries, got %v", len(entries))
		}
	})
}

func TestFilterBySource(t *testing.T) {
	t.Run("filters by source", func(t *testing.T) {
		rb := New(60 * time.Second)

		// Add entries with different sources
		systemEntry := types.TelemetryEntry{
			Timestamp: time.Now(),
			Source:    types.SourceSystem,
			Type:      types.TypeCPU,
			Name:      "cpu_usage",
			Value:     50.0,
		}
		sidecarEntry := types.TelemetryEntry{
			Timestamp: time.Now(),
			Source:    types.SourceSidecar,
			Type:      types.TypeMemory,
			Name:      "heap_usage",
			Value:     1024,
		}

		rb.Add(systemEntry)
		rb.Add(sidecarEntry)

		// Filter for system entries only
		filter := map[string]interface{}{
			"source": types.SourceSystem,
		}
		entries := rb.GetByFilter(filter)

		if len(entries) != 1 {
			t.Errorf("Expected 1 entry, got %v", len(entries))
		}
		if entries[0].Source != types.SourceSystem {
			t.Errorf("Expected SourceSystem, got %v", entries[0].Source)
		}
		if entries[0].Name != "cpu_usage" {
			t.Errorf("Expected cpu_usage, got %v", entries[0].Name)
		}
	})

	t.Run("filters by type", func(t *testing.T) {
		rb := New(60 * time.Second)

		entries := []types.TelemetryEntry{
			{
				Timestamp: time.Now(),
				Source:    types.SourceSystem,
				Type:      types.TypeCPU,
				Name:      "cpu_usage",
				Value:     50.0,
			},
			{
				Timestamp: time.Now(),
				Source:    types.SourceSystem,
				Type:      types.TypeMemory,
				Name:      "memory_usage",
				Value:     75.0,
			},
			{
				Timestamp: time.Now(),
				Source:    types.SourceSystem,
				Type:      types.TypeCPU,
				Name:      "cpu_load",
				Value:     1.5,
			},
		}

		for _, entry := range entries {
			rb.Add(entry)
		}

		// Filter for CPU entries only
		filter := map[string]interface{}{
			"type": types.TypeCPU,
		}
		cpuEntries := rb.GetByFilter(filter)

		if len(cpuEntries) != 2 {
			t.Errorf("Expected 2 CPU entries, got %v", len(cpuEntries))
		}
		for _, entry := range cpuEntries {
			if entry.Type != types.TypeCPU {
				t.Errorf("Expected TypeCPU, got %v", entry.Type)
			}
		}
	})

	t.Run("filters by multiple criteria", func(t *testing.T) {
		rb := New(60 * time.Second)

		entries := []types.TelemetryEntry{
			{
				Timestamp: time.Now(),
				Source:    types.SourceSystem,
				Type:      types.TypeCPU,
				Name:      "cpu_usage",
				Value:     50.0,
			},
			{
				Timestamp: time.Now(),
				Source:    types.SourceSidecar,
				Type:      types.TypeCPU,
				Name:      "jvm_cpu",
				Value:     30.0,
			},
		}

		for _, entry := range entries {
			rb.Add(entry)
		}

		// Filter for system CPU entries
		filter := map[string]interface{}{
			"source": types.SourceSystem,
			"type":   types.TypeCPU,
		}
		filtered := rb.GetByFilter(filter)

		if len(filtered) != 1 {
			t.Errorf("Expected 1 entry, got %v", len(filtered))
		}
		if filtered[0].Name != "cpu_usage" {
			t.Errorf("Expected cpu_usage, got %v", filtered[0].Name)
		}
	})
}

func TestGetStats(t *testing.T) {
	t.Run("returns correct stats", func(t *testing.T) {
		rb := New(60 * time.Second)

		// Add some entries
		for i := 0; i < 3; i++ {
			entry := types.TelemetryEntry{
				Timestamp: time.Now().Add(time.Duration(i) * time.Second),
				Source:    types.SourceSystem,
				Type:      types.TypeCPU,
				Name:      "test_metric",
				Value:     float64(i),
			}
			rb.Add(entry)
		}

		stats := rb.GetStats()

		if stats.TotalEntries != 3 {
			t.Errorf("Expected TotalEntries 3, got %v", stats.TotalEntries)
		}
		if stats.BufferSize != rb.size {
			t.Errorf("Expected BufferSize %v, got %v", rb.size, stats.BufferSize)
		}
		if stats.UtilizationPercent == 0 {
			t.Error("Expected non-zero utilization percent")
		}
		if stats.WindowSize != rb.windowSize {
			t.Errorf("Expected WindowSize %v, got %v", rb.windowSize, stats.WindowSize)
		}
	})

	t.Run("handles empty buffer", func(t *testing.T) {
		rb := New(60 * time.Second)
		stats := rb.GetStats()

		if stats.TotalEntries != 0 {
			t.Errorf("Expected TotalEntries 0, got %v", stats.TotalEntries)
		}
		if stats.UtilizationPercent != 0 {
			t.Errorf("Expected UtilizationPercent 0, got %v", stats.UtilizationPercent)
		}
		if !stats.OldestEntry.IsZero() {
			t.Error("Expected zero OldestEntry for empty buffer")
		}
		if !stats.NewestEntry.IsZero() {
			t.Error("Expected zero NewestEntry for empty buffer")
		}
	})
}

func TestCleanup(t *testing.T) {
	t.Run("removes expired entries", func(t *testing.T) {
		rb := New(1 * time.Second) // 1 second window

		// Add entries with timestamps outside the window
		oldTime := time.Now().Add(-2 * time.Second)
		recentTime := time.Now()

		oldEntry := types.TelemetryEntry{
			Timestamp: oldTime,
			Source:    types.SourceSystem,
			Type:      types.TypeCPU,
			Name:      "old_metric",
			Value:     10.0,
		}
		recentEntry := types.TelemetryEntry{
			Timestamp: recentTime,
			Source:    types.SourceSystem,
			Type:      types.TypeCPU,
			Name:      "recent_metric",
			Value:     20.0,
		}

		rb.Add(oldEntry)
		rb.Add(recentEntry)

		initialCount := rb.count
		removedCount := rb.Cleanup()

		// Should have removed the old entry
		if removedCount <= 0 {
			t.Errorf("Expected to remove at least 1 entry, removed %v", removedCount)
		}
		if rb.count >= initialCount {
			t.Errorf("Expected count to decrease from %v, got %v", initialCount, rb.count)
		}
	})

	t.Run("handles empty buffer", func(t *testing.T) {
		rb := New(60 * time.Second)
		removedCount := rb.Cleanup()

		if removedCount != 0 {
			t.Errorf("Expected to remove 0 entries from empty buffer, removed %v", removedCount)
		}
	})
}

func TestConcurrentAccess(t *testing.T) {
	t.Run("concurrent adds and reads", func(t *testing.T) {
		rb := New(60 * time.Second)
		var wg sync.WaitGroup
		numGoroutines := 10
		entriesPerGoroutine := 100

		// Concurrent adds
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < entriesPerGoroutine; j++ {
					entry := types.TelemetryEntry{
						Timestamp: time.Now(),
						Source:    types.SourceSystem,
						Type:      types.TypeCPU,
						Name:      "concurrent_test",
						Value:     float64(id*entriesPerGoroutine + j),
					}
					rb.Add(entry)
				}
			}(i)
		}

		// Concurrent reads
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < entriesPerGoroutine; j++ {
					_ = rb.GetStats()
					since := time.Now().Add(-10 * time.Second)
					until := time.Now()
					_ = rb.GetByTimeRange(since, until)
				}
			}()
		}

		wg.Wait()

		// Verify final state is consistent
		stats := rb.GetStats()
		if stats.TotalEntries > numGoroutines*entriesPerGoroutine {
			t.Errorf("Too many entries: expected <= %v, got %v", numGoroutines*entriesPerGoroutine, stats.TotalEntries)
		}
	})

	t.Run("concurrent cleanup", func(t *testing.T) {
		rb := New(100 * time.Millisecond) // Short window for cleanup
		var wg sync.WaitGroup

		// Add entries continuously
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				entry := types.TelemetryEntry{
					Timestamp: time.Now(),
					Source:    types.SourceSystem,
					Type:      types.TypeCPU,
					Name:      "cleanup_test",
					Value:     float64(i),
				}
				rb.Add(entry)
				time.Sleep(1 * time.Millisecond)
			}
		}()

		// Run cleanup concurrently
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				rb.Cleanup()
				time.Sleep(10 * time.Millisecond)
			}
		}()

		wg.Wait()

		// Should complete without race conditions or panics
	})
}
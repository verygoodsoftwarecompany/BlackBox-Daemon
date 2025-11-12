// Package ringbuffer provides comprehensive unit tests for the ring buffer implementation.
// These tests validate thread-safe operations, time-based filtering, memory management,
// and performance characteristics of the circular buffer used for telemetry storage.
package ringbuffer

import (
	"sync"
	"testing"
	"time"

	"github.com/verygoodsoftwarecompany/blackbox-daemon/pkg/types"
)

// TestNew validates RingBuffer creation and initialization with proper buffer sizing.
func TestNew(t *testing.T) {
	t.Run("creates buffer with correct window size", func(t *testing.T) {
		windowSize := 60 * time.Second
		rb := New(windowSize)
		
		if rb == nil {
			t.Fatal("Expected buffer to be created")
		}
		
		stats := rb.GetStats()
		if stats.WindowSize != windowSize {
			t.Errorf("Expected window size %v, got %v", windowSize, stats.WindowSize)
		}
		
		if stats.BufferSize < 1000 {
			t.Errorf("Expected buffer size >= 1000, got %d", stats.BufferSize)
		}
		
		if stats.TotalEntries != 0 {
			t.Errorf("Expected empty buffer, got %d entries", stats.TotalEntries)
		}
	})
	
	t.Run("calculates appropriate buffer size", func(t *testing.T) {
		tests := []struct {
			name       string
			windowSize time.Duration
			minSize    int
		}{
			{"small window", 1 * time.Second, 1000},
			{"medium window", 30 * time.Second, 30000},
			{"large window", 300 * time.Second, 300000},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				rb := New(tt.windowSize)
				stats := rb.GetStats()
				
				if stats.BufferSize < tt.minSize {
					t.Errorf("Expected buffer size >= %d, got %d", tt.minSize, stats.BufferSize)
				}
			})
		}
	})
}

// TestAdd validates entry insertion and circular buffer behavior.
func TestAdd(t *testing.T) {
	t.Run("adds single entry", func(t *testing.T) {
		rb := New(60 * time.Second)
		
		entry := types.TelemetryEntry{
			Timestamp: time.Now(),
			Source:    types.SourceSystem,
			Type:      types.TypeCPU,
			Name:      "cpu_usage",
			Value:     0.25,
			Tags:      map[string]string{"node": "test"},
		}
		
		rb.Add(entry)
		
		stats := rb.GetStats()
		if stats.TotalEntries != 1 {
			t.Errorf("Expected 1 entry, got %d", stats.TotalEntries)
		}
		
		entries := rb.GetAll()
		if len(entries) != 1 {
			t.Errorf("Expected 1 entry, got %d", len(entries))
		}
		
		if entries[0].Name != "cpu_usage" {
			t.Errorf("Expected name 'cpu_usage', got %q", entries[0].Name)
		}
	})
	
	t.Run("adds multiple entries", func(t *testing.T) {
		rb := New(60 * time.Second)
		
		baseTime := time.Now()
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
		
		stats := rb.GetStats()
		if stats.TotalEntries != 5 {
			t.Errorf("Expected 5 entries, got %d", stats.TotalEntries)
		}
		
		entries := rb.GetAll()
		if len(entries) != 5 {
			t.Errorf("Expected 5 entries, got %d", len(entries))
		}
		
		// Verify chronological order
		for i := 1; i < len(entries); i++ {
			if !entries[i].Timestamp.After(entries[i-1].Timestamp) {
				t.Error("Expected entries in chronological order")
			}
		}
	})
	
	t.Run("handles buffer overflow", func(t *testing.T) {
		// Create small buffer for testing overflow
		rb := New(1 * time.Millisecond) // Very small window to get small buffer
		stats := rb.GetStats()
		bufferSize := stats.BufferSize
		
		// Add more entries than buffer capacity
		baseTime := time.Now()
		for i := 0; i < bufferSize+10; i++ {
			entry := types.TelemetryEntry{
				Timestamp: baseTime.Add(time.Duration(i) * time.Microsecond),
				Source:    types.SourceSystem,
				Type:      types.TypeCPU,
				Name:      "overflow_test",
				Value:     float64(i),
			}
			rb.Add(entry)
		}
		
		// Buffer should not exceed capacity
		stats = rb.GetStats()
		if stats.TotalEntries > bufferSize {
			t.Errorf("Expected max %d entries, got %d", bufferSize, stats.TotalEntries)
		}
		
		entries := rb.GetAll()
		if len(entries) > bufferSize {
			t.Errorf("Expected max %d entries, got %d", bufferSize, len(entries))
		}
		
		// Should contain the most recent entries
		if len(entries) > 0 {
			lastEntry := entries[len(entries)-1]
			expectedValue := float64(bufferSize + 9) // Last added value
			if lastEntry.Value.(float64) != expectedValue {
				t.Errorf("Expected last value %f, got %f", expectedValue, lastEntry.Value.(float64))
			}
		}
	})
}

// TestGetWindow validates time-based window filtering.
func TestGetWindow(t *testing.T) {
	t.Run("returns entries within time window", func(t *testing.T) {
		rb := New(30 * time.Second)
		
		baseTime := time.Now()
		// Add entries spanning 60 seconds
		for i := 0; i < 60; i++ {
			entry := types.TelemetryEntry{
				Timestamp: baseTime.Add(time.Duration(i) * time.Second),
				Source:    types.SourceSystem,
				Type:      types.TypeCPU,
				Name:      "test_metric",
				Value:     float64(i),
			}
			rb.Add(entry)
		}
		
		// Get window from middle of timeline (should only get last 30 seconds)
		fromTime := baseTime.Add(45 * time.Second)
		entries := rb.GetWindow(fromTime)
		
		// Should only return entries from last 30 seconds (15-45 second range)
		if len(entries) == 0 {
			t.Error("Expected entries within window")
		}
		
		// All entries should be within the window (after cutoff)
		cutoff := fromTime.Add(-30 * time.Second)
		for _, entry := range entries {
			if entry.Timestamp.Before(cutoff) {
				t.Errorf("Entry %v before cutoff %v", entry.Timestamp, cutoff)
			}
			// Note: entries can be after fromTime since GetWindow looks backwards from fromTime
		}
	})
	
	t.Run("returns empty for empty buffer", func(t *testing.T) {
		rb := New(60 * time.Second)
		
		entries := rb.GetWindow(time.Now())
		if len(entries) != 0 {
			t.Errorf("Expected empty result, got %d entries", len(entries))
		}
	})
	
	t.Run("handles window before all entries", func(t *testing.T) {
		rb := New(60 * time.Second)
		
		// Add entries starting from now
		baseTime := time.Now()
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
		
		// Request window from before entries were added
		fromTime := baseTime.Add(-30 * time.Second)
		entries := rb.GetWindow(fromTime)
		
		// Should get some entries since GetWindow looks backwards from fromTime
		// and includes entries after the cutoff time
		// The window would be from fromTime-60s to fromTime, and our entries start at baseTime
		// Since baseTime is 30s after fromTime, all entries should be included
		if len(entries) != 5 {
			t.Errorf("Expected 5 entries, got %d", len(entries))
		}
	})
}

// TestFilterBySource validates source-based filtering.
func TestFilterBySource(t *testing.T) {
	rb := New(60 * time.Second)
	
	baseTime := time.Now()
	// Add mixed source entries
	for i := 0; i < 10; i++ {
		var source types.TelemetrySource
		if i%2 == 0 {
			source = types.SourceSystem
		} else {
			source = types.SourceSidecar
		}
		
		entry := types.TelemetryEntry{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Source:    source,
			Type:      types.TypeCPU,
			Name:      "test_metric",
			Value:     float64(i),
		}
		rb.Add(entry)
	}
	
	t.Run("filters system entries", func(t *testing.T) {
		entries := rb.FilterBySource(types.SourceSystem, baseTime.Add(30*time.Second))
		
		if len(entries) == 0 {
			t.Error("Expected system entries")
		}
		
		for _, entry := range entries {
			if entry.Source != types.SourceSystem {
				t.Errorf("Expected system source, got %v", entry.Source)
			}
		}
	})
	
	t.Run("filters sidecar entries", func(t *testing.T) {
		entries := rb.FilterBySource(types.SourceSidecar, baseTime.Add(30*time.Second))
		
		if len(entries) == 0 {
			t.Error("Expected sidecar entries")
		}
		
		for _, entry := range entries {
			if entry.Source != types.SourceSidecar {
				t.Errorf("Expected sidecar source, got %v", entry.Source)
			}
		}
	})
}

// TestFilterByPod validates pod-based filtering.
func TestFilterByPod(t *testing.T) {
	rb := New(60 * time.Second)
	
	baseTime := time.Now()
	// Add entries from different pods and system
	pods := []string{"", "pod-1", "pod-2", ""} // Empty string represents system entries
	
	for i := 0; i < 8; i++ {
		podName := pods[i%len(pods)]
		var source types.TelemetrySource
		var tags map[string]string
		
		if podName == "" {
			source = types.SourceSystem
		} else {
			source = types.SourceSidecar
			tags = map[string]string{"pod_name": podName}
		}
		
		entry := types.TelemetryEntry{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Source:    source,
			Type:      types.TypeCPU,
			Name:      "test_metric",
			Value:     float64(i),
			Tags:      tags,
		}
		rb.Add(entry)
	}
	
	t.Run("filters by specific pod", func(t *testing.T) {
		entries := rb.FilterByPod("pod-1", baseTime.Add(30*time.Second))
		
		if len(entries) == 0 {
			t.Error("Expected pod-1 entries")
		}
		
		for _, entry := range entries {
			if entry.Tags == nil || entry.Tags["pod_name"] != "pod-1" {
				t.Errorf("Expected pod-1 entries, got entry with tags %v", entry.Tags)
			}
		}
	})
	
	t.Run("returns system entries for empty pod name", func(t *testing.T) {
		entries := rb.FilterByPod("", baseTime.Add(30*time.Second))
		
		if len(entries) == 0 {
			t.Error("Expected system entries")
		}
		
		for _, entry := range entries {
			if entry.Source != types.SourceSystem {
				t.Errorf("Expected system entries, got source %v", entry.Source)
			}
		}
	})
}

// TestGetStats validates buffer statistics functionality.
func TestGetStats(t *testing.T) {
	t.Run("returns correct stats for populated buffer", func(t *testing.T) {
		rb := New(60 * time.Second)
		
		baseTime := time.Now()
		entryCount := 5
		
		for i := 0; i < entryCount; i++ {
			entry := types.TelemetryEntry{
				Timestamp: baseTime.Add(time.Duration(i) * time.Second),
				Source:    types.SourceSystem,
				Type:      types.TypeCPU,
				Name:      "test_metric",
				Value:     float64(i),
			}
			rb.Add(entry)
		}
		
		stats := rb.GetStats()
		
		if stats.TotalEntries != entryCount {
			t.Errorf("Expected %d entries, got %d", entryCount, stats.TotalEntries)
		}
		
		if stats.WindowSize != 60*time.Second {
			t.Errorf("Expected window size 60s, got %v", stats.WindowSize)
		}
		
		expectedOldest := baseTime
		expectedNewest := baseTime.Add(time.Duration(entryCount-1) * time.Second)
		
		if !stats.OldestEntry.Equal(expectedOldest) {
			t.Errorf("Expected oldest entry %v, got %v", expectedOldest, stats.OldestEntry)
		}
		
		if !stats.NewestEntry.Equal(expectedNewest) {
			t.Errorf("Expected newest entry %v, got %v", expectedNewest, stats.NewestEntry)
		}
		
		expectedWindow := expectedNewest.Sub(expectedOldest)
		if stats.ActualWindow != expectedWindow {
			t.Errorf("Expected actual window %v, got %v", expectedWindow, stats.ActualWindow)
		}
	})
	
	t.Run("returns zero stats for empty buffer", func(t *testing.T) {
		rb := New(60 * time.Second)
		
		stats := rb.GetStats()
		
		if stats.TotalEntries != 0 {
			t.Errorf("Expected 0 entries, got %d", stats.TotalEntries)
		}
		
		if !stats.OldestEntry.IsZero() {
			t.Error("Expected zero oldest entry time")
		}
		
		if !stats.NewestEntry.IsZero() {
			t.Error("Expected zero newest entry time")
		}
		
		if stats.ActualWindow != 0 {
			t.Errorf("Expected zero actual window, got %v", stats.ActualWindow)
		}
	})
}

// TestCleanup validates automatic cleanup of old entries.
func TestCleanup(t *testing.T) {
	t.Run("removes expired entries", func(t *testing.T) {
		rb := New(30 * time.Second) // 30 second window
		
		baseTime := time.Now().Add(-60 * time.Second) // Start 60 seconds ago
		
		// Add entries spanning 50 seconds (some should be expired)
		for i := 0; i < 50; i++ {
			entry := types.TelemetryEntry{
				Timestamp: baseTime.Add(time.Duration(i) * time.Second),
				Source:    types.SourceSystem,
				Type:      types.TypeCPU,
				Name:      "test_metric",
				Value:     float64(i),
			}
			rb.Add(entry)
		}
		
		initialStats := rb.GetStats()
		initialCount := initialStats.TotalEntries
		
		// Cleanup should remove entries older than 30 seconds from now
		rb.Cleanup()
		
		finalStats := rb.GetStats()
		
		// Should have fewer entries after cleanup
		if finalStats.TotalEntries >= initialCount {
			t.Errorf("Expected cleanup to reduce entries from %d, got %d", 
				initialCount, finalStats.TotalEntries)
		}
		
		// Remaining entries should all be within window
		entries := rb.GetAll()
		cutoff := time.Now().Add(-30 * time.Second)
		for _, entry := range entries {
			if entry.Timestamp.Before(cutoff) {
				t.Errorf("Found expired entry after cleanup: %v", entry.Timestamp)
			}
		}
	})
	
	t.Run("handles empty buffer cleanup", func(t *testing.T) {
		rb := New(60 * time.Second)
		
		// Should not panic on empty buffer
		rb.Cleanup()
		
		stats := rb.GetStats()
		if stats.TotalEntries != 0 {
			t.Errorf("Expected empty buffer after cleanup, got %d entries", stats.TotalEntries)
		}
	})
}

// TestThreadSafety validates concurrent access to the ring buffer.
func TestThreadSafety(t *testing.T) {
	t.Run("concurrent adds and reads", func(t *testing.T) {
		rb := New(60 * time.Second)
		
		var wg sync.WaitGroup
		numWorkers := 10
		entriesPerWorker := 100
		
		// Start concurrent writers
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < entriesPerWorker; j++ {
					entry := types.TelemetryEntry{
						Timestamp: time.Now(),
						Source:    types.SourceSystem,
						Type:      types.TypeCPU,
						Name:      "concurrent_test",
						Value:     float64(workerID*entriesPerWorker + j),
					}
					rb.Add(entry)
				}
			}(i)
		}
		
		// Start concurrent readers
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				for j := 0; j < entriesPerWorker/10; j++ {
					rb.GetAll()
					rb.GetWindow(time.Now())
					rb.GetStats()
					time.Sleep(time.Millisecond)
				}
			}()
		}
		
		wg.Wait()
		
		// Verify buffer is in a consistent state
		stats := rb.GetStats()
		if stats.TotalEntries < 0 {
			t.Error("Buffer in inconsistent state after concurrent access")
		}
		
		entries := rb.GetAll()
		if len(entries) != stats.TotalEntries {
			t.Errorf("Entry count mismatch: stats=%d, actual=%d", 
				stats.TotalEntries, len(entries))
		}
	})
}

// TestEdgeCases validates edge cases and error conditions.
func TestEdgeCases(t *testing.T) {
	t.Run("handles very small window", func(t *testing.T) {
		rb := New(1 * time.Nanosecond) // Extremely small window
		
		entry := types.TelemetryEntry{
			Timestamp: time.Now(),
			Source:    types.SourceSystem,
			Type:      types.TypeCPU,
			Name:      "test",
			Value:     1.0,
		}
		
		rb.Add(entry)
		
		// Should still function normally
		stats := rb.GetStats()
		if stats.TotalEntries != 1 {
			t.Errorf("Expected 1 entry, got %d", stats.TotalEntries)
		}
	})
	
	t.Run("handles entries with same timestamp", func(t *testing.T) {
		rb := New(60 * time.Second)
		
		timestamp := time.Now()
		for i := 0; i < 3; i++ {
			entry := types.TelemetryEntry{
				Timestamp: timestamp, // Same timestamp
				Source:    types.SourceSystem,
				Type:      types.TypeCPU,
				Name:      "same_time",
				Value:     float64(i),
			}
			rb.Add(entry)
		}
		
		entries := rb.GetAll()
		if len(entries) != 3 {
			t.Errorf("Expected 3 entries, got %d", len(entries))
		}
		
		// All should have the same timestamp
		for _, entry := range entries {
			if !entry.Timestamp.Equal(timestamp) {
				t.Error("Expected all entries to have same timestamp")
			}
		}
	})
}
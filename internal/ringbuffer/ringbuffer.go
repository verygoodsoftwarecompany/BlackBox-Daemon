// Package ringbuffer provides a thread-safe circular buffer implementation
// for storing telemetry data with automatic time-based expiration.
// This package is optimized for high-throughput telemetry collection while
// maintaining bounded memory usage.
package ringbuffer

import (
	"sync"
	"time"

	"github.com/verygoodsoftwarecompany/blackbox-daemon/pkg/types"
)

// RingBuffer is a thread-safe circular buffer that maintains telemetry data
// for a configurable time window. It automatically manages memory by overwriting
// old entries and provides efficient querying by time range and filters.
type RingBuffer struct {
	// mutex protects all buffer operations for thread safety
	mutex sync.RWMutex
	// entries is the circular array storing telemetry entries
	entries []types.TelemetryEntry
	// size is the maximum capacity of the buffer
	size int
	// head is the current write position in the circular buffer
	head int
	// count is the number of entries currently stored (may be less than size)
	count int
	// windowSize is the time duration for which entries should be retained
	windowSize time.Duration
}

// New creates a new ring buffer with the specified window size.
// The buffer size is automatically calculated based on the window size and
// expected telemetry throughput (~1000 entries per second).
func New(windowSize time.Duration) *RingBuffer {
	// Estimate buffer size based on window size and expected entry rate
	// Assuming ~1000 entries per second across all sources
	estimatedSize := int(windowSize.Seconds() * 1000)
	if estimatedSize < 1000 {
		estimatedSize = 1000 // Minimum buffer size
	}

	return &RingBuffer{
		entries:    make([]types.TelemetryEntry, estimatedSize),
		size:       estimatedSize,
		windowSize: windowSize,
	}
}

// Add inserts a new telemetry entry into the ring buffer.
// This operation is thread-safe and will overwrite the oldest entry if the buffer is full.
func (rb *RingBuffer) Add(entry types.TelemetryEntry) {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	// Store the entry at the current head position
	rb.entries[rb.head] = entry
	// Advance head position, wrapping around if necessary (circular buffer)
	rb.head = (rb.head + 1) % rb.size

	// Track the number of entries, capped at buffer size
	if rb.count < rb.size {
		rb.count++
	}
}

// GetWindow returns all entries within the specified time window from the given timestamp.
// The time window extends backwards from the 'from' timestamp by the buffer's window size.
// This is the primary method used during incident analysis to gather relevant telemetry.
func (rb *RingBuffer) GetWindow(from time.Time) []types.TelemetryEntry {
	rb.mutex.RLock()
	defer rb.mutex.RUnlock()

	if rb.count == 0 {
		return []types.TelemetryEntry{}
	}

	var result []types.TelemetryEntry
	// Calculate the cutoff time - only entries after this time are included
	cutoff := from.Add(-rb.windowSize)

	// Calculate the starting position of the oldest entry in the circular buffer
	// If head-count is negative, we need to wrap around to the end of the buffer
	start := rb.head - rb.count
	if start < 0 {
		start += rb.size
	}

	// Iterate through all entries in chronological order (oldest to newest)
	for i := 0; i < rb.count; i++ {
		// Calculate the actual array index, wrapping around for circular buffer
		idx := (start + i) % rb.size
		entry := rb.entries[idx]

		// Only include entries within the specified time window
		if entry.Timestamp.After(cutoff) {
			result = append(result, entry)
		}
	}

	return result
}

// GetAll returns all entries currently in the buffer in chronological order.
// This method is primarily used for debugging and administrative purposes.
func (rb *RingBuffer) GetAll() []types.TelemetryEntry {
	rb.mutex.RLock()
	defer rb.mutex.RUnlock()

	if rb.count == 0 {
		return []types.TelemetryEntry{}
	}

	result := make([]types.TelemetryEntry, rb.count)

	start := rb.head - rb.count
	if start < 0 {
		start += rb.size
	}

	for i := 0; i < rb.count; i++ {
		idx := (start + i) % rb.size
		result[i] = rb.entries[idx]
	}

	return result
}

// GetStats returns statistics about the ring buffer for monitoring and diagnostics.
// These statistics are useful for understanding buffer utilization and performance.
func (rb *RingBuffer) GetStats() BufferStats {
	rb.mutex.RLock()
	defer rb.mutex.RUnlock()

	stats := BufferStats{
		TotalEntries: rb.count,
		BufferSize:   rb.size,
		WindowSize:   rb.windowSize,
	}

	if rb.count > 0 {
		// Find oldest and newest entries
		start := rb.head - rb.count
		if start < 0 {
			start += rb.size
		}

		oldest := rb.entries[start]
		newest := rb.entries[(rb.head-1+rb.size)%rb.size]

		stats.OldestEntry = oldest.Timestamp
		stats.NewestEntry = newest.Timestamp
		stats.ActualWindow = newest.Timestamp.Sub(oldest.Timestamp)
	}

	return stats
}

// FilterBySource returns entries from the buffer filtered by source within the time window.
// This is useful for getting only system telemetry or only sidecar telemetry during analysis.
func (rb *RingBuffer) FilterBySource(source types.TelemetrySource, from time.Time) []types.TelemetryEntry {
	entries := rb.GetWindow(from)
	var filtered []types.TelemetryEntry

	for _, entry := range entries {
		if entry.Source == source {
			filtered = append(filtered, entry)
		}
	}

	return filtered
}

// FilterByPod returns entries from the buffer filtered by pod name within the time window.
// If podName is empty, returns all system telemetry. Otherwise, returns telemetry
// specifically associated with the named pod.
func (rb *RingBuffer) FilterByPod(podName string, from time.Time) []types.TelemetryEntry {
	entries := rb.GetWindow(from)
	var filtered []types.TelemetryEntry

	for _, entry := range entries {
		if podName == "" {
			// Include system telemetry when no specific pod is requested
			if entry.Source == types.SourceSystem {
				filtered = append(filtered, entry)
			}
		} else if entry.Tags != nil {
			if entryPod, ok := entry.Tags["pod_name"]; ok && entryPod == podName {
				filtered = append(filtered, entry)
			}
		}
	}

	return filtered
}

// BufferStats contains statistics about the ring buffer for monitoring and analysis.
type BufferStats struct {
	// TotalEntries is the number of entries currently stored in the buffer
	TotalEntries int `json:"total_entries"`
	// BufferSize is the maximum capacity of the buffer
	BufferSize int `json:"buffer_size"`
	// WindowSize is the configured time window for retention
	WindowSize time.Duration `json:"window_size"`
	// ActualWindow is the actual time span of data currently in the buffer
	ActualWindow time.Duration `json:"actual_window"`
	// OldestEntry is the timestamp of the oldest entry in the buffer
	OldestEntry time.Time `json:"oldest_entry"`
	// NewestEntry is the timestamp of the newest entry in the buffer
	NewestEntry time.Time `json:"newest_entry"`
}

// Cleanup removes entries older than the window size to free memory and prevent
// memory leaks. This should be called periodically by a background goroutine.
func (rb *RingBuffer) Cleanup() {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	if rb.count == 0 {
		return
	}

	now := time.Now()
	cutoff := now.Add(-rb.windowSize)

	// Count how many entries to remove
	removeCount := 0
	start := rb.head - rb.count
	if start < 0 {
		start += rb.size
	}

	for i := 0; i < rb.count; i++ {
		idx := (start + i) % rb.size
		if rb.entries[idx].Timestamp.Before(cutoff) {
			removeCount++
		} else {
			break
		}
	}

	// Update count and clear old entries
	if removeCount > 0 {
		rb.count -= removeCount
		// Clear the removed entries to help GC
		for i := 0; i < removeCount; i++ {
			idx := (start + i) % rb.size
			rb.entries[idx] = types.TelemetryEntry{}
		}
	}
}

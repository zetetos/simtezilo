// Package logstore provides an in-memory log storage with circular buffer functionality
package logstore

import (
	"encoding/json"
	"errors"
	"maps"
	"sync"
	"time"
)

// LogEntry represents a single structured log entry.
type LogEntry map[string]any

// Store manages in-memory log entries with a circular buffer.
type Store struct {
	entries    []LogEntry
	maxEntries int
	index      int
	full       bool
	mu         sync.RWMutex
}

// New creates a new log store with the specified maximum number of entries.
func New(maxEntries int) *Store {
	return &Store{
		entries:    make([]LogEntry, maxEntries),
		maxEntries: maxEntries,
		index:      0,
		full:       false,
	}
}

// Add adds a log entry to the store using circular buffer logic.
func (s *Store) Add(entry LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[s.index] = entry
	s.index++

	if s.index >= s.maxEntries {
		s.index = 0
		s.full = true
	}
}

// GetAll returns all log entries in chronological order.
func (s *Store) GetAll() []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.full {
		// Not full yet, return from start to current index
		result := make([]LogEntry, s.index)
		copy(result, s.entries[:s.index])

		return result
	}

	// Buffer is full, return in correct chronological order
	result := make([]LogEntry, s.maxEntries)

	// Copy from index to end (older entries)
	copy(result, s.entries[s.index:])

	// Copy from start to index (newer entries)
	copy(result[s.maxEntries-s.index:], s.entries[:s.index])

	return result
}

// GetRecent returns the most recent n log entries.
func (s *Store) GetRecent(wantCount int) []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := s.index
	if s.full {
		count = s.maxEntries
	}

	if wantCount > count {
		wantCount = count
	}

	result := make([]LogEntry, wantCount)

	if !s.full {
		// Not full yet, return the last n entries
		start := max(s.index-wantCount, 0)

		copy(result, s.entries[start:s.index])

		return result
	}

	// Buffer is full, get last n entries considering circular nature
	startIdx := (s.index - wantCount + s.maxEntries) % s.maxEntries

	if startIdx < s.index {
		// Entries are contiguous
		copy(result, s.entries[startIdx:s.index])
	} else {
		// Entries wrap around
		firstPart := s.maxEntries - startIdx
		copy(result, s.entries[startIdx:])
		copy(result[firstPart:], s.entries[:s.index])
	}

	return result
}

// GetPage returns a paginated slice of log entries (newest first)
// offset: number of entries to skip from the newest
// limit: maximum number of entries to return
func (s *Store) GetPage(offset, limit int) []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalCount := s.index
	if s.full {
		totalCount = s.maxEntries
	}

	// Validate and adjust parameters
	if offset < 0 {
		offset = 0
	}

	if offset >= totalCount {
		return []LogEntry{}
	}

	if limit < 1 {
		limit = 100
	}

	// Calculate how many entries we can actually return
	available := totalCount - offset
	if limit > available {
		limit = available
	}

	result := make([]LogEntry, limit)

	// We need to work backwards from the newest entry
	// The newest entry is at index-1 (or maxEntries-1 if we've wrapped)
	for idx := range limit {
		// Position from newest: offset + i
		posFromNewest := offset + idx

		var actualIdx int
		if !s.full {
			// Not wrapped yet, newest is at index-1
			actualIdx = s.index - 1 - posFromNewest
		} else {
			// Wrapped, newest is at index-1 (wrapping around)
			actualIdx = (s.index - 1 - posFromNewest + s.maxEntries) % s.maxEntries
		}

		result[idx] = s.entries[actualIdx]
	}

	return result
}

// Count returns the current number of log entries stored.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.full {
		return s.maxEntries
	}

	return s.index
}

// Clear removes all log entries from the store.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries = make([]LogEntry, s.maxEntries)
	s.index = 0
	s.full = false
}

// GetStats returns statistics about log levels.
func (s *Store) GetStats() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]int)
	entries := s.GetAll()

	for _, entry := range entries {
		if level, ok := entry["level"].(string); ok {
			stats[level]++
		}
	}

	return stats
}

// Write implements io.Writer interface for zerolog
// This allows the store to be used as a log output destination.
func (s *Store) Write(event []byte) (n int, err error) {
	// Parse the JSON log entry
	var rawEntry map[string]any

	err = json.Unmarshal(event, &rawEntry)
	if err != nil {
		return len(event), errors.New("failed to unmarshal log entry JSON")
	}

	// Store all fields from the raw entry
	entry := make(LogEntry)
	maps.Copy(entry, rawEntry)

	// Ensure timestamp field exists
	if _, ok := entry["timestamp"]; !ok {
		if ts, ok := entry["time"].(string); ok {
			entry["timestamp"] = ts
		} else {
			entry["timestamp"] = time.Now().Format(time.RFC3339)
		}
	}

	// Normalize time field to timestamp if time exists but timestamp doesn't
	if _, hasTimestamp := entry["timestamp"]; !hasTimestamp {
		if timeVal, hasTime := entry["time"]; hasTime {
			entry["timestamp"] = timeVal
		}
	}

	s.Add(entry)

	return len(event), nil
}

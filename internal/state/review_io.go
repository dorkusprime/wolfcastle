package state

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/dorkusprime/wolfcastle/internal/clock"
)

// LoadBatch reads the pending review batch from disk.
// Returns nil, nil if the file does not exist.
func LoadBatch(path string) (*Batch, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading review batch: %w", err)
	}
	var b Batch
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("parsing review batch: %w", err)
	}
	return &b, nil
}

// SaveBatch writes the review batch to disk atomically.
func SaveBatch(path string, b *Batch) error {
	return atomicWriteJSON(path, b)
}

// RemoveBatch deletes the review batch file.
func RemoveBatch(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// LoadHistory reads the review history from disk.
// Returns an empty History if the file does not exist.
func LoadHistory(path string) (*History, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &History{}, nil
		}
		return nil, fmt.Errorf("reading review history: %w", err)
	}
	var h History
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("parsing review history: %w", err)
	}
	return &h, nil
}

// SaveHistory writes the review history to disk atomically.
func SaveHistory(path string, h *History) error {
	return atomicWriteJSON(path, h)
}

// EnforceRetention trims history entries older than maxAgeDays and keeps
// at most maxEntries. Pass 0 for either to skip that limit.
// An optional clock may be provided for deterministic testing.
func EnforceRetention(h *History, maxEntries int, maxAgeDays int, clocks ...clock.Clock) {
	if maxAgeDays > 0 {
		clk := resolveOptionalClock(clocks)
		cutoff := clk.Now().AddDate(0, 0, -maxAgeDays)
		var kept []HistoryEntry
		for _, e := range h.Entries {
			if e.CompletedAt.After(cutoff) {
				kept = append(kept, e)
			}
		}
		h.Entries = kept
	}

	if maxEntries > 0 && len(h.Entries) > maxEntries {
		// Keep the most recent entries.
		sort.Slice(h.Entries, func(i, j int) bool {
			return h.Entries[i].CompletedAt.After(h.Entries[j].CompletedAt)
		})
		h.Entries = h.Entries[:maxEntries]
	}
}

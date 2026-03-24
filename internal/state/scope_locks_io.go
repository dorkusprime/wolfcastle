package state

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadScopeLocks reads and parses a scope-locks.json file. Returns an empty
// ScopeLockTable if the file does not exist.
func LoadScopeLocks(path string) (*ScopeLockTable, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewScopeLockTable(), nil
		}
		return nil, fmt.Errorf("reading scope locks: %w", err)
	}
	var t ScopeLockTable
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parsing scope locks: %w", err)
	}
	if t.Locks == nil {
		t.Locks = make(map[string]ScopeLock)
	}
	return &t, nil
}

// SaveScopeLocks writes the scope lock table to disk atomically.
func SaveScopeLocks(path string, t *ScopeLockTable) error {
	return atomicWriteJSON(path, t)
}

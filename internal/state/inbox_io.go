package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LoadInbox reads and parses an inbox.json file. Returns an empty InboxFile if
// the file does not exist.
func LoadInbox(path string) (*InboxFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &InboxFile{}, nil
		}
		return nil, fmt.Errorf("reading inbox: %w", err)
	}
	var f InboxFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing inbox: %w", err)
	}
	return &f, nil
}

// SaveInbox writes the inbox file to disk atomically.
func SaveInbox(path string, f *InboxFile) error {
	return atomicWriteJSON(path, f)
}

// InboxAppend atomically appends an item to the inbox under a file lock.
// This prevents races between concurrent inbox add commands and the
// daemon's intake goroutine marking items as filed.
func InboxAppend(inboxPath string, item InboxItem) error {
	lock := NewFileLock(filepath.Dir(inboxPath), 5*time.Second)
	return lock.WithLock(func() error {
		f, err := LoadInbox(inboxPath)
		if err != nil {
			return err
		}
		f.Items = append(f.Items, item)
		return SaveInbox(inboxPath, f)
	})
}

// InboxMutate atomically loads the inbox, calls fn to modify it, and
// saves it back under a file lock. Use this for any read-modify-write
// operation on the inbox (e.g., marking items as filed).
func InboxMutate(inboxPath string, fn func(*InboxFile) error) error {
	lock := NewFileLock(filepath.Dir(inboxPath), 5*time.Second)
	return lock.WithLock(func() error {
		f, err := LoadInbox(inboxPath)
		if err != nil {
			return err
		}
		if err := fn(f); err != nil {
			return err
		}
		return SaveInbox(inboxPath, f)
	})
}

package state

import (
	"encoding/json"
	"fmt"
	"os"
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

package inbox

import (
	"encoding/json"
	"fmt"
	"os"
)

// Load reads and parses an inbox.json file. Returns an empty File if the file
// does not exist.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{}, nil
		}
		return nil, fmt.Errorf("reading inbox: %w", err)
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing inbox: %w", err)
	}
	return &f, nil
}

// Save writes the inbox file to disk as indented JSON.
func Save(path string, f *File) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling inbox: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing inbox: %w", err)
	}
	return nil
}

package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LoadRootIndex reads the root state.json for an engineer namespace.
func LoadRootIndex(path string) (*RootIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var idx RootIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	if idx.Nodes == nil {
		idx.Nodes = make(map[string]IndexEntry)
	}
	return &idx, nil
}

// SaveRootIndex writes the root index atomically (write to temp, rename).
func SaveRootIndex(path string, idx *RootIndex) error {
	return atomicWriteJSON(path, idx)
}

// LoadNodeState reads a node's state.json.
func LoadNodeState(path string) (*NodeState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ns NodeState
	if err := json.Unmarshal(data, &ns); err != nil {
		return nil, err
	}
	return &ns, nil
}

// SaveNodeState writes a node's state.json atomically.
func SaveNodeState(path string, ns *NodeState) error {
	return atomicWriteJSON(path, ns)
}

// atomicWriteJSON writes JSON to a temp file then renames it into place.
func atomicWriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".wolfcastle-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

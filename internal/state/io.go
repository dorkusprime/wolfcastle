package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadRootIndex reads the root state.json for an engineer namespace.
func LoadRootIndex(path string) (*RootIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading root index: %w", err)
	}
	var idx RootIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parsing root index: %w", err)
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

// LoadNodeState reads a node's state.json and normalizes the audit state.
func LoadNodeState(path string) (*NodeState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading node state: %w", err)
	}
	var ns NodeState
	if err := json.Unmarshal(data, &ns); err != nil {
		return nil, fmt.Errorf("parsing node state: %w", err)
	}
	normalizeAuditState(&ns)
	return &ns, nil
}

// normalizeAuditState handles legacy or malformed audit state, ensuring
// all required fields have valid defaults.
func normalizeAuditState(ns *NodeState) {
	// Ensure slices are non-nil for consistent JSON output
	if ns.Audit.Breadcrumbs == nil {
		ns.Audit.Breadcrumbs = []Breadcrumb{}
	}
	if ns.Audit.Gaps == nil {
		ns.Audit.Gaps = []Gap{}
	}
	if ns.Audit.Escalations == nil {
		ns.Audit.Escalations = []Escalation{}
	}

	// Default empty audit status to pending
	if ns.Audit.Status == "" {
		ns.Audit.Status = AuditPending
	}
}

// SaveNodeState writes a node's state.json atomically.
func SaveNodeState(path string, ns *NodeState) error {
	return atomicWriteJSON(path, ns)
}

// AtomicWriteFile writes data to path atomically by writing to a temp
// file in the same directory and renaming. The caller sees either the
// old content or the new content, never a partial write.
func AtomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".wolfcastle-tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming temp file to %s: %w", filepath.Base(path), err)
	}
	return nil
}

// atomicWriteJSON marshals v as indented JSON and writes it atomically.
func atomicWriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	data = append(data, '\n')
	return AtomicWriteFile(path, data)
}

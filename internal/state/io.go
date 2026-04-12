package state

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dorkusprime/wolfcastle/internal/fsutil"
)

// StateFileName is the canonical filename for node and root-index state files.
const StateFileName = "state.json"

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
	return fsutil.AtomicWriteFile(path, data)
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

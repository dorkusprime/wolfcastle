package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// RecoveringNodeLoader: normal load, recovery, and error paths
// ═══════════════════════════════════════════════════════════════════════════

func TestRecoveringNodeLoader_NormalLoad(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	projDir := filepath.Join(dir, "system", "projects")
	nodeDir := filepath.Join(projDir, "my-node")
	_ = os.MkdirAll(nodeDir, 0755)

	ns := state.NewNodeState("my-node", "My Node", state.NodeLeaf)
	data, _ := json.MarshalIndent(ns, "", "  ")
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), data, 0644)

	loader := RecoveringNodeLoader(projDir, nil)
	loaded, err := loader("my-node")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.Name != "My Node" {
		t.Errorf("expected 'My Node', got %q", loaded.Name)
	}
}

func TestRecoveringNodeLoader_InvalidAddress(t *testing.T) {
	t.Parallel()
	loader := RecoveringNodeLoader("/tmp", nil)
	_, err := loader("INVALID_ADDRESS")
	if err == nil {
		t.Error("expected error for invalid address")
	}
}

func TestRecoveringNodeLoader_MissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	loader := RecoveringNodeLoader(dir, nil)
	_, err := loader("nonexistent-node")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestRecoveringNodeLoader_RecoveryPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	projDir := filepath.Join(dir, "system", "projects")
	nodeDir := filepath.Join(projDir, "my-node")
	_ = os.MkdirAll(nodeDir, 0755)

	// Write truncated JSON (missing closing brace)
	ns := state.NewNodeState("my-node", "My Node", state.NodeLeaf)
	data, _ := json.MarshalIndent(ns, "", "  ")
	truncated := data[:len(data)-2] // remove closing brace
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), truncated, 0644)

	var recovered bool
	loader := RecoveringNodeLoader(projDir, func(addr string, report *RecoveryReport) {
		recovered = true
		if addr != "my-node" {
			t.Errorf("expected addr 'my-node', got %q", addr)
		}
	})

	loaded, err := loader("my-node")
	if err != nil {
		t.Fatalf("recovery should succeed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil node state")
	}
	if !recovered {
		t.Error("expected onRecover callback to fire")
	}
}

func TestRecoveringNodeLoader_UnrecoverableFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	projDir := filepath.Join(dir, "system", "projects")
	nodeDir := filepath.Join(projDir, "my-node")
	_ = os.MkdirAll(nodeDir, 0755)

	// Write total garbage
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), []byte("not json at all"), 0644)

	loader := RecoveringNodeLoader(projDir, nil)
	_, err := loader("my-node")
	if err == nil {
		t.Error("expected error for unrecoverable file")
	}
}

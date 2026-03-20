package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestEnrichCmd_AddsEnrichment(t *testing.T) {
	env := newTestEnv(t)
	setupNode(t, env, "test-node", state.NodeLeaf)

	env.RootCmd.SetArgs([]string{"audit", "enrich", "--node", "test-node", "check error wrapping"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("enrich failed: %v", err)
	}

	ns := loadNode(t, env, "test-node")
	if len(ns.AuditEnrichment) != 1 || ns.AuditEnrichment[0] != "check error wrapping" {
		t.Errorf("expected enrichment, got %v", ns.AuditEnrichment)
	}
}

func TestEnrichCmd_DeduplicatesEnrichment(t *testing.T) {
	env := newTestEnv(t)
	setupNode(t, env, "test-node", state.NodeLeaf)

	env.RootCmd.SetArgs([]string{"audit", "enrich", "--node", "test-node", "check tests"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"audit", "enrich", "--node", "test-node", "check tests"})
	_ = env.RootCmd.Execute()

	ns := loadNode(t, env, "test-node")
	if len(ns.AuditEnrichment) != 1 {
		t.Errorf("expected 1 enrichment (deduped), got %d", len(ns.AuditEnrichment))
	}
}

func TestEnrichCmd_MultipleEnrichments(t *testing.T) {
	env := newTestEnv(t)
	setupNode(t, env, "test-node", state.NodeLeaf)

	env.RootCmd.SetArgs([]string{"audit", "enrich", "--node", "test-node", "first check"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"audit", "enrich", "--node", "test-node", "second check"})
	_ = env.RootCmd.Execute()

	ns := loadNode(t, env, "test-node")
	if len(ns.AuditEnrichment) != 2 {
		t.Errorf("expected 2 enrichments, got %d", len(ns.AuditEnrichment))
	}
}

func TestEnrichCmd_EmptyText_ReturnsError(t *testing.T) {
	env := newTestEnv(t)
	setupNode(t, env, "test-node", state.NodeLeaf)

	env.RootCmd.SetArgs([]string{"audit", "enrich", "--node", "test-node", "   "})
	if err := env.RootCmd.Execute(); err == nil {
		t.Error("expected error for whitespace-only enrichment text")
	}
}

func TestEnrichCmd_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true
	setupNode(t, env, "test-node", state.NodeLeaf)

	env.RootCmd.SetArgs([]string{"audit", "enrich", "--node", "test-node", "verify coverage"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("enrich with JSON failed: %v", err)
	}
}

func TestEnrichCmd_NoIdentity_ReturnsError(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"audit", "enrich", "--node", "test-node", "check tests"})
	if err := env.RootCmd.Execute(); err == nil {
		t.Error("expected error when identity is nil")
	}
}

// setupNode creates a leaf or orchestrator node with a root index entry.
func setupNode(t *testing.T, env *testEnv, addr string, nodeType state.NodeType) {
	t.Helper()
	idx := state.NewRootIndex()
	idx.Root = []string{addr}
	idx.Nodes[addr] = state.IndexEntry{
		Name: addr, Type: nodeType, State: state.StatusNotStarted, Address: addr,
	}
	saveJSON(t, filepath.Join(env.ProjectsDir, "state.json"), idx)

	ns := state.NewNodeState(addr, addr, nodeType)
	nodeDir := filepath.Join(env.ProjectsDir, addr)
	_ = os.MkdirAll(nodeDir, 0755)
	saveJSON(t, filepath.Join(nodeDir, "state.json"), ns)
}

// loadNode reads the node state for the given address.
func loadNode(t *testing.T, env *testEnv, addr string) *state.NodeState {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(env.ProjectsDir, addr, "state.json"))
	if err != nil {
		t.Fatalf("reading node state: %v", err)
	}
	var ns state.NodeState
	if err := json.Unmarshal(data, &ns); err != nil {
		t.Fatalf("parsing node state: %v", err)
	}
	return &ns
}

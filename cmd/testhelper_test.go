package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// testEnv sets up a temporary wolfcastle workspace with config, identity,
// resolver, and root index.  It returns a cleanup function that restores
// the original working directory, the root of the temp workspace, and the
// prepared App.
type testEnv struct {
	RootDir       string
	WolfcastleDir string
	ProjectsDir   string
	App           *cmdutil.App
	cleanup       func()
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	os.MkdirAll(wcDir, 0755)

	// Write minimal config.json
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "test", Machine: "dev"}
	cfg.Docs.Directory = "docs"
	cfgData, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(wcDir, "config.json"), cfgData, 0644)

	// Write config.local.json with identity
	localCfg := map[string]any{
		"identity": map[string]string{
			"user":    "test",
			"machine": "dev",
		},
	}
	localData, _ := json.MarshalIndent(localCfg, "", "  ")
	os.WriteFile(filepath.Join(wcDir, "config.local.json"), localData, 0644)

	// Create projects namespace directory
	ns := "test-dev"
	projDir := filepath.Join(wcDir, "projects", ns)
	os.MkdirAll(projDir, 0755)

	// Write empty root index
	idx := state.NewRootIndex()
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	os.WriteFile(filepath.Join(projDir, "state.json"), idxData, 0644)

	// Create docs dirs
	os.MkdirAll(filepath.Join(wcDir, "docs", "specs"), 0755)
	os.MkdirAll(filepath.Join(wcDir, "docs", "decisions"), 0755)

	resolver := &tree.Resolver{
		WolfcastleDir: wcDir,
		Namespace:     ns,
	}

	loadedCfg, err := config.Load(wcDir)
	if err != nil {
		t.Fatalf("loading test config: %v", err)
	}

	testApp := &cmdutil.App{
		WolfcastleDir: wcDir,
		Cfg:           loadedCfg,
		Resolver:      resolver,
	}

	// Save original cwd and switch to tmp (for init etc.)
	origDir, _ := os.Getwd()
	os.Chdir(tmp)

	env := &testEnv{
		RootDir:       tmp,
		WolfcastleDir: wcDir,
		ProjectsDir:   projDir,
		App:           testApp,
		cleanup:       func() { os.Chdir(origDir) },
	}

	t.Cleanup(func() { env.cleanup() })
	return env
}

// createLeafNode creates a leaf node with the given address in the test env.
func (e *testEnv) createLeafNode(t *testing.T, addr, name string) {
	t.Helper()
	parsed, _ := tree.ParseAddress(addr)
	nodeDir := filepath.Join(e.ProjectsDir, filepath.Join(parsed.Parts...))
	os.MkdirAll(nodeDir, 0755)

	ns := state.NewNodeState(parsed.Leaf(), name, state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "Audit task", State: state.StatusNotStarted, IsAudit: true},
	}
	nsData, _ := json.MarshalIndent(ns, "", "  ")
	os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)

	// Update root index
	idx, _ := state.LoadRootIndex(filepath.Join(e.ProjectsDir, "state.json"))
	idx.Nodes[addr] = state.IndexEntry{
		Name:     name,
		Type:     state.NodeLeaf,
		State:    state.StatusNotStarted,
		Address:  addr,
		Children: []string{},
	}
	state.SaveRootIndex(filepath.Join(e.ProjectsDir, "state.json"), idx)
}

// createOrchestratorNode creates an orchestrator node.
func (e *testEnv) createOrchestratorNode(t *testing.T, addr, name string, children []string) {
	t.Helper()
	parsed, _ := tree.ParseAddress(addr)
	nodeDir := filepath.Join(e.ProjectsDir, filepath.Join(parsed.Parts...))
	os.MkdirAll(nodeDir, 0755)

	ns := state.NewNodeState(parsed.Leaf(), name, state.NodeOrchestrator)
	nsData, _ := json.MarshalIndent(ns, "", "  ")
	os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)

	idx, _ := state.LoadRootIndex(filepath.Join(e.ProjectsDir, "state.json"))
	idx.Nodes[addr] = state.IndexEntry{
		Name:     name,
		Type:     state.NodeOrchestrator,
		State:    state.StatusNotStarted,
		Address:  addr,
		Children: children,
	}
	state.SaveRootIndex(filepath.Join(e.ProjectsDir, "state.json"), idx)
}

// loadNodeState is a convenience for loading a node's state.json.
func (e *testEnv) loadNodeState(t *testing.T, addr string) *state.NodeState {
	t.Helper()
	parsed, _ := tree.ParseAddress(addr)
	statePath := filepath.Join(e.ProjectsDir, filepath.Join(parsed.Parts...), "state.json")
	ns, err := state.LoadNodeState(statePath)
	if err != nil {
		t.Fatalf("loading node state for %s: %v", addr, err)
	}
	return ns
}

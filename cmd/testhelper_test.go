package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/clock"
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
	_ = os.MkdirAll(wcDir, 0755)

	// Write base/config.json with defaults
	_ = os.MkdirAll(filepath.Join(wcDir, "system", "base"), 0755)
	_ = os.MkdirAll(filepath.Join(wcDir, "system", "custom"), 0755)
	_ = os.MkdirAll(filepath.Join(wcDir, "system", "local"), 0755)

	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "test", Machine: "dev"}
	cfg.Docs.Directory = "docs"
	cfgData, _ := json.MarshalIndent(cfg, "", "  ")
	_ = os.WriteFile(filepath.Join(wcDir, "system", "base", "config.json"), cfgData, 0644)

	// Write custom/config.json (empty)
	_ = os.WriteFile(filepath.Join(wcDir, "system", "custom", "config.json"), []byte("{}"), 0644)

	// Write local/config.json with identity
	localCfg := map[string]any{
		"identity": map[string]string{
			"user":    "test",
			"machine": "dev",
		},
	}
	localData, _ := json.MarshalIndent(localCfg, "", "  ")
	_ = os.WriteFile(filepath.Join(wcDir, "system", "local", "config.json"), localData, 0644)

	// Create projects namespace directory
	ns := "test-dev"
	projDir := filepath.Join(wcDir, "system", "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	// Write empty root index
	idx := state.NewRootIndex()
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), idxData, 0644)

	// Create docs dirs
	_ = os.MkdirAll(filepath.Join(wcDir, "docs", "specs"), 0755)
	_ = os.MkdirAll(filepath.Join(wcDir, "docs", "decisions"), 0755)

	loadedCfg, err := config.Load(wcDir)
	if err != nil {
		t.Fatalf("loading test config: %v", err)
	}

	cfgRepo := config.NewConfigRepository(wcDir)
	identity, _ := config.IdentityFromConfig(loadedCfg)
	stateStore := state.NewStateStore(filepath.Join(wcDir, "system", "projects", ns), state.DefaultLockTimeout)

	testApp := &cmdutil.App{
		Config:        cfgRepo,
		Identity:      identity,
		State:         stateStore,
		WolfcastleDir: wcDir,
		Cfg:           loadedCfg,
		Store:         stateStore,
		Clock:         clock.New(),
	}

	// Save original cwd and switch to tmp (for init etc.)
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmp)

	env := &testEnv{
		RootDir:       tmp,
		WolfcastleDir: wcDir,
		ProjectsDir:   projDir,
		App:           testApp,
		cleanup:       func() { _ = os.Chdir(origDir) },
	}

	t.Cleanup(func() { env.cleanup() })
	return env
}

// createLeafNode creates a leaf node with the given address in the test env.
func (e *testEnv) createLeafNode(t *testing.T, addr, name string) {
	t.Helper()
	parsed, _ := tree.ParseAddress(addr)
	nodeDir := filepath.Join(e.ProjectsDir, filepath.Join(parsed.Parts...))
	_ = os.MkdirAll(nodeDir, 0755)

	ns := state.NewNodeState(parsed.Leaf(), name, state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "Audit task", State: state.StatusNotStarted, IsAudit: true},
	}
	nsData, _ := json.MarshalIndent(ns, "", "  ")
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)

	// Update root index
	idx, _ := state.LoadRootIndex(filepath.Join(e.ProjectsDir, "state.json"))
	idx.Nodes[addr] = state.IndexEntry{
		Name:     name,
		Type:     state.NodeLeaf,
		State:    state.StatusNotStarted,
		Address:  addr,
		Children: []string{},
	}
	_ = state.SaveRootIndex(filepath.Join(e.ProjectsDir, "state.json"), idx)
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

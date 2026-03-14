package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newStatusTestEnv(t *testing.T) *testEnv {
	t.Helper()
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	os.MkdirAll(wcDir, 0755)

	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "test", Machine: "dev"}

	ns := "test-dev"
	projDir := filepath.Join(wcDir, "projects", ns)
	os.MkdirAll(projDir, 0755)

	// Create root index with some nodes
	idx := state.NewRootIndex()
	idx.RootID = "my-project"
	idx.RootState = state.StatusInProgress

	// Add a leaf node
	idx.Nodes["my-project"] = state.IndexEntry{
		Name:     "My Project",
		Type:     state.NodeLeaf,
		State:    state.StatusInProgress,
		Address:  "my-project",
		Children: []string{},
	}

	// Create node dir and state
	nodeDir := filepath.Join(projDir, "my-project")
	os.MkdirAll(nodeDir, 0755)

	ns2 := state.NewNodeState("my-project", "My Project", state.NodeLeaf)
	ns2.State = state.StatusInProgress
	nsData, _ := json.MarshalIndent(ns2, "", "  ")
	os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)

	idxData, _ := json.MarshalIndent(idx, "", "  ")
	os.WriteFile(filepath.Join(projDir, "state.json"), idxData, 0644)

	resolver := &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns}
	testApp := &cmdutil.App{
		WolfcastleDir: wcDir,
		Cfg:           cfg,
		Resolver:      resolver,
	}

	rootCmd := &cobra.Command{Use: "wolfcastle"}
	Register(testApp, rootCmd)

	return &testEnv{
		WolfcastleDir: wcDir,
		ProjectsDir:   projDir,
		App:           testApp,
		RootCmd:       rootCmd,
	}
}

func TestStatusCmd_Success(t *testing.T) {
	env := newStatusTestEnv(t)
	env.RootCmd.SetArgs([]string{"status"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status failed: %v", err)
	}
}

func TestStatusCmd_WithScope(t *testing.T) {
	env := newStatusTestEnv(t)
	env.RootCmd.SetArgs([]string{"status", "--node", "my-project"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status --node failed: %v", err)
	}
}

func TestShowAllStatus_NoNamespaces(t *testing.T) {
	env := newTestEnv(t)
	// showAllStatus reads from projects/ dir
	err := showAllStatus(env.App)
	if err != nil {
		t.Fatalf("showAllStatus failed: %v", err)
	}
}

func TestShowTreeStatus_EmptyTree(t *testing.T) {
	env := newTestEnv(t)
	idx := state.NewRootIndex()
	err := showTreeStatus(env.App, idx, "")
	if err != nil {
		t.Fatalf("showTreeStatus failed: %v", err)
	}
}

func TestShowTreeStatus_WithNodes(t *testing.T) {
	env := newStatusTestEnv(t)
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	err := showTreeStatus(env.App, idx, "")
	if err != nil {
		t.Fatalf("showTreeStatus failed: %v", err)
	}
}

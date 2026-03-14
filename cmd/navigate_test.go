package cmd

import (
	"os"
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

func TestNavigate_NoTasks(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"navigate"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
}

func TestNavigate_FindsTask(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	env.createLeafNode(t, "my-project", "My Project")

	// Add a task
	parsed, _ := tree.ParseAddress("my-project")
	ns := env.loadNodeState(t, "my-project")
	task, _ := state.TaskAdd(ns, "implement API")
	state.SaveNodeState(app.Resolver.NodeStatePath(parsed), ns)

	rootCmd.SetArgs([]string{"navigate"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	_ = task
}

func TestNavigate_NoInit(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmp)

	app = &cmdutil.App{}

	rootCmd.SetArgs([]string{"navigate"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when .wolfcastle does not exist")
	}
}

func TestNavigate_WithScope(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	env.createLeafNode(t, "my-project", "My Project")

	rootCmd.SetArgs([]string{"navigate", "--node", "my-project"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("navigate --node failed: %v", err)
	}
}

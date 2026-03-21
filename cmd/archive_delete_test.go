package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestArchiveDelete_Success(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")
	app = env.App

	archiveNodeInEnv(t, env, "my-project")

	rootCmd.SetArgs([]string{"archive", "delete", "--node", "my-project", "--confirm"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("archive delete failed: %v", err)
	}

	// Verify the archived directory was removed.
	archiveDir := filepath.Join(env.ProjectsDir, ".archive", "my-project")
	if _, err := os.Stat(archiveDir); !os.IsNotExist(err) {
		t.Error("expected archived directory to be removed")
	}

	// Verify the node was purged from the index.
	idx, err := app.State.ReadIndex()
	if err != nil {
		t.Fatalf("reading index: %v", err)
	}

	if _, ok := idx.Nodes["my-project"]; ok {
		t.Error("node should have been purged from index")
	}

	for _, r := range idx.ArchivedRoot {
		if r == "my-project" {
			t.Error("node should not remain in ArchivedRoot")
		}
	}
}

func TestArchiveDelete_MissingConfirm(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")
	app = env.App

	archiveNodeInEnv(t, env, "my-project")

	rootCmd.SetArgs([]string{"archive", "delete", "--node", "my-project", "--confirm=false"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when --confirm is missing")
	}
	if !strings.Contains(err.Error(), "--confirm is required") {
		t.Errorf("expected '--confirm is required' error, got: %v", err)
	}
}

func TestArchiveDelete_NotFound(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"archive", "delete", "--node", "nonexistent", "--confirm"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestArchiveDelete_NotArchived(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")
	app = env.App

	rootCmd.SetArgs([]string{"archive", "delete", "--node", "my-project", "--confirm"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-archived node")
	}
	if !strings.Contains(err.Error(), "not archived") {
		t.Errorf("expected 'not archived' error, got: %v", err)
	}
}

func TestArchiveDelete_NotInArchivedRoot(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")
	app = env.App

	// Mark as archived in the index entry but don't add to ArchivedRoot.
	if err := app.State.MutateIndex(func(idx *state.RootIndex) error {
		if e, ok := idx.Nodes["my-project"]; ok {
			e.Archived = true
			idx.Nodes["my-project"] = e
		}
		return nil
	}); err != nil {
		t.Fatalf("mutating index: %v", err)
	}

	rootCmd.SetArgs([]string{"archive", "delete", "--node", "my-project", "--confirm"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for node not in archived_root")
	}
	if !strings.Contains(err.Error(), "not a root-level archived node") {
		t.Errorf("expected 'not a root-level archived node' error, got: %v", err)
	}
}

func TestArchiveDelete_JSONOutput(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")
	app = env.App
	app.JSON = true
	defer func() { app.JSON = false }()

	archiveNodeInEnv(t, env, "my-project")

	rootCmd.SetArgs([]string{"archive", "delete", "--node", "my-project", "--confirm"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("archive delete --json failed: %v", err)
	}
}

func TestArchiveDelete_MissingNodeFlag(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"archive", "delete", "--confirm"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when --node is missing")
	}
}

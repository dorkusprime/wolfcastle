package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// archiveNodeInEnv sets up an archived node: moves its directory into .archive/,
// moves it from Root to ArchivedRoot in the index, and sets the Archived flag.
func archiveNodeInEnv(t *testing.T, env *testEnv, addr string) {
	t.Helper()

	projectsDir := env.ProjectsDir
	parts := strings.Split(addr, "/")
	activeDir := filepath.Join(projectsDir, filepath.Join(parts...))
	archiveDir := filepath.Join(projectsDir, ".archive", filepath.Join(parts...))

	if err := os.MkdirAll(filepath.Dir(archiveDir), 0o755); err != nil {
		t.Fatalf("creating archive parent: %v", err)
	}
	if err := os.Rename(activeDir, archiveDir); err != nil {
		t.Fatalf("moving node to archive: %v", err)
	}

	if err := env.App.State.MutateIndex(func(idx *state.RootIndex) error {
		var newRoot []string
		for _, r := range idx.Root {
			if r != addr {
				newRoot = append(newRoot, r)
			}
		}
		idx.Root = newRoot
		idx.ArchivedRoot = append(idx.ArchivedRoot, addr)

		if e, ok := idx.Nodes[addr]; ok {
			e.Archived = true
			now := time.Now()
			e.ArchivedAt = &now
			idx.Nodes[addr] = e
		}
		return nil
	}); err != nil {
		t.Fatalf("mutating index for archive: %v", err)
	}
}

func TestArchiveRestore_Success(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")
	app = env.App

	archiveNodeInEnv(t, env, "my-project")

	rootCmd.SetArgs([]string{"archive", "restore", "--node", "my-project"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("archive restore failed: %v", err)
	}

	// Verify the node directory was moved back to active location.
	activeDir := filepath.Join(env.ProjectsDir, "my-project")
	if _, err := os.Stat(activeDir); os.IsNotExist(err) {
		t.Error("expected node directory to be restored to active location")
	}

	// Verify the index was updated.
	idx, err := app.State.ReadIndex()
	if err != nil {
		t.Fatalf("reading index: %v", err)
	}

	entry, ok := idx.Nodes["my-project"]
	if !ok {
		t.Fatal("node should still exist in index")
	}
	if entry.Archived {
		t.Error("node should no longer be marked as archived")
	}
	if entry.ArchivedAt != nil {
		t.Error("archived_at should be nil after restore")
	}

	foundInRoot := false
	for _, r := range idx.Root {
		if r == "my-project" {
			foundInRoot = true
			break
		}
	}
	if !foundInRoot {
		t.Error("node should be back in Root")
	}

	for _, r := range idx.ArchivedRoot {
		if r == "my-project" {
			t.Error("node should not remain in ArchivedRoot")
		}
	}
}

func TestArchiveRestore_NotFound(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"archive", "restore", "--node", "nonexistent"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestArchiveRestore_NotArchived(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")
	app = env.App

	rootCmd.SetArgs([]string{"archive", "restore", "--node", "my-project"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-archived node")
	}
	if !strings.Contains(err.Error(), "not archived") {
		t.Errorf("expected 'not archived' error, got: %v", err)
	}
}

func TestArchiveRestore_NotInArchivedRoot(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")
	app = env.App

	// Mark as archived in the index entry but don't move to ArchivedRoot.
	if err := app.State.MutateIndex(func(idx *state.RootIndex) error {
		if e, ok := idx.Nodes["my-project"]; ok {
			e.Archived = true
			now := time.Now()
			e.ArchivedAt = &now
			idx.Nodes["my-project"] = e
		}
		return nil
	}); err != nil {
		t.Fatalf("mutating index: %v", err)
	}

	rootCmd.SetArgs([]string{"archive", "restore", "--node", "my-project"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for node not in archived_root")
	}
	if !strings.Contains(err.Error(), "not a root-level archived node") {
		t.Errorf("expected 'not a root-level archived node' error, got: %v", err)
	}
}

func TestArchiveRestore_JSONOutput(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")
	app = env.App
	app.JSON = true
	defer func() { app.JSON = false }()

	archiveNodeInEnv(t, env, "my-project")

	rootCmd.SetArgs([]string{"archive", "restore", "--node", "my-project"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("archive restore --json failed: %v", err)
	}
}

func TestArchiveRestore_MissingNodeFlag(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"archive", "restore"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when --node is missing")
	}
}

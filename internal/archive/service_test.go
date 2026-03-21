package archive

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// writeJSON marshals v to pretty-printed JSON and writes it at path,
// creating parent directories as needed.
func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

// testService creates a Service backed by a temporary directory with an
// empty RootIndex already on disk. Returns the service and its backing dir.
func testService(t *testing.T, clk clock.Clock) (*Service, string) {
	t.Helper()
	tmp := t.TempDir()
	projDir := filepath.Join(tmp, "projects")
	wolfDir := filepath.Join(tmp, "wolfcastle")

	// Write an empty root index so the store can read it.
	idx := state.NewRootIndex()
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	store := state.NewStateStore(projDir, 5*time.Second)
	svc := &Service{
		Store:         store,
		WolfcastleDir: wolfDir,
		Clock:         clk,
	}
	return svc, projDir
}

// setupNode writes a node state file and adds it to the index.
func setupNode(t *testing.T, projDir string, addr string, ns *state.NodeState, ie state.IndexEntry) {
	t.Helper()
	parts := strings.Split(addr, "/")
	nodeDir := filepath.Join(projDir, filepath.Join(parts...))
	writeJSON(t, filepath.Join(nodeDir, "state.json"), ns)

	// Merge into the existing index.
	store := state.NewStateStore(projDir, 5*time.Second)
	if err := store.MutateIndex(func(idx *state.RootIndex) error {
		idx.Nodes[addr] = ie
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// addToRoot appends addr to the index's Root list.
func addToRoot(t *testing.T, projDir string, addr string) {
	t.Helper()
	store := state.NewStateStore(projDir, 5*time.Second)
	if err := store.MutateIndex(func(idx *state.RootIndex) error {
		idx.Root = append(idx.Root, addr)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// addToArchivedRoot appends addr to the index's ArchivedRoot list.
func addToArchivedRoot(t *testing.T, projDir string, addr string) {
	t.Helper()
	store := state.NewStateStore(projDir, 5*time.Second)
	if err := store.MutateIndex(func(idx *state.RootIndex) error {
		idx.ArchivedRoot = append(idx.ArchivedRoot, addr)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// ── Archive tests ───────────────────────────────────────────────────────

func TestArchive_MovesDirectoriesAndUpdatesIndex(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	svc, projDir := testService(t, clk)

	// Set up a completed root node with one child.
	ns := state.NewNodeState("my-project", "My Project", state.NodeOrchestrator)
	ns.State = state.StatusComplete
	ns.Audit.ResultSummary = "All tasks finished."
	setupNode(t, projDir, "my-project", ns, state.IndexEntry{
		Name:     "My Project",
		Type:     state.NodeOrchestrator,
		State:    state.StatusComplete,
		Address:  "my-project",
		Children: []string{"my-project/leaf"},
	})

	childNS := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	childNS.State = state.StatusComplete
	setupNode(t, projDir, "my-project/leaf", childNS, state.IndexEntry{
		Name:    "Leaf",
		Type:    state.NodeLeaf,
		State:   state.StatusComplete,
		Address: "my-project/leaf",
		Parent:  "my-project",
	})

	addToRoot(t, projDir, "my-project")

	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	if err := svc.Archive("my-project", cfg, "main"); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	// Active directories should be gone; archive directories should exist.
	if _, err := os.Stat(filepath.Join(projDir, "my-project")); !os.IsNotExist(err) {
		t.Error("expected active root dir to be removed")
	}
	if _, err := os.Stat(filepath.Join(projDir, "my-project", "leaf")); !os.IsNotExist(err) {
		t.Error("expected active child dir to be removed")
	}
	if _, err := os.Stat(filepath.Join(projDir, ".archive", "my-project", "state.json")); err != nil {
		t.Errorf("expected archived root dir to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projDir, ".archive", "my-project", "leaf", "state.json")); err != nil {
		t.Errorf("expected archived child dir to exist: %v", err)
	}

	// Index should reflect the archive.
	idx, err := svc.Store.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range idx.Root {
		if r == "my-project" {
			t.Error("my-project should no longer be in Root")
		}
	}
	found := false
	for _, r := range idx.ArchivedRoot {
		if r == "my-project" {
			found = true
		}
	}
	if !found {
		t.Error("my-project should appear in ArchivedRoot")
	}

	// Both nodes should be marked archived with timestamps.
	for _, addr := range []string{"my-project", "my-project/leaf"} {
		entry := idx.Nodes[addr]
		if !entry.Archived {
			t.Errorf("%s should be marked archived", addr)
		}
		if entry.ArchivedAt == nil {
			t.Errorf("%s should have ArchivedAt set", addr)
		} else if !entry.ArchivedAt.Equal(now) {
			t.Errorf("%s ArchivedAt = %v, want %v", addr, *entry.ArchivedAt, now)
		}
	}

	// Markdown rollup should be written.
	rollups, err := filepath.Glob(filepath.Join(svc.WolfcastleDir, "archive", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rollups) != 1 {
		t.Fatalf("expected 1 rollup file, got %d", len(rollups))
	}
	content, _ := os.ReadFile(rollups[0])
	if !strings.Contains(string(content), "# Archive: my-project") {
		t.Error("rollup should contain the archive header")
	}
}

func TestArchive_PreservesOtherRoots(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	svc, projDir := testService(t, clock.NewMock(now))

	// Two root nodes; archive only one.
	for _, addr := range []string{"project-a", "project-b"} {
		ns := state.NewNodeState(addr, addr, state.NodeLeaf)
		ns.State = state.StatusComplete
		setupNode(t, projDir, addr, ns, state.IndexEntry{
			Name:    addr,
			Type:    state.NodeLeaf,
			State:   state.StatusComplete,
			Address: addr,
		})
		addToRoot(t, projDir, addr)
	}

	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	if err := svc.Archive("project-a", cfg, "main"); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	idx, _ := svc.Store.ReadIndex()
	if len(idx.Root) != 1 || idx.Root[0] != "project-b" {
		t.Errorf("expected Root=[project-b], got %v", idx.Root)
	}
}

// ── Restore tests ───────────────────────────────────────────────────────

func TestRestore_MovesBackAndUpdatesIndex(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	svc, projDir := testService(t, clock.NewMock(now))

	// Simulate an archived node by placing state in .archive/.
	archivedAt := now.Add(-1 * time.Hour)
	archiveDir := filepath.Join(projDir, ".archive", "old-project")
	ns := state.NewNodeState("old-project", "Old", state.NodeLeaf)
	ns.State = state.StatusComplete
	writeJSON(t, filepath.Join(archiveDir, "state.json"), ns)

	store := state.NewStateStore(projDir, 5*time.Second)
	if err := store.MutateIndex(func(idx *state.RootIndex) error {
		idx.ArchivedRoot = append(idx.ArchivedRoot, "old-project")
		idx.Nodes["old-project"] = state.IndexEntry{
			Name:       "Old",
			Type:       state.NodeLeaf,
			State:      state.StatusComplete,
			Address:    "old-project",
			Archived:   true,
			ArchivedAt: &archivedAt,
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := svc.Restore("old-project"); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Archive dir should be gone; active dir should exist.
	if _, err := os.Stat(archiveDir); !os.IsNotExist(err) {
		t.Error("expected archive dir to be removed after restore")
	}
	if _, err := os.Stat(filepath.Join(projDir, "old-project", "state.json")); err != nil {
		t.Errorf("expected active dir to exist after restore: %v", err)
	}

	idx, _ := svc.Store.ReadIndex()
	for _, r := range idx.ArchivedRoot {
		if r == "old-project" {
			t.Error("old-project should no longer be in ArchivedRoot")
		}
	}
	foundRoot := false
	for _, r := range idx.Root {
		if r == "old-project" {
			foundRoot = true
		}
	}
	if !foundRoot {
		t.Error("old-project should be back in Root")
	}
	entry := idx.Nodes["old-project"]
	if entry.Archived {
		t.Error("entry should not be marked archived after restore")
	}
	if entry.ArchivedAt != nil {
		t.Error("ArchivedAt should be nil after restore")
	}
}

func TestRestore_RejectsNotArchived(t *testing.T) {
	t.Parallel()
	svc, projDir := testService(t, clock.NewMock(time.Now()))

	ns := state.NewNodeState("active", "Active", state.NodeLeaf)
	ns.State = state.StatusComplete
	setupNode(t, projDir, "active", ns, state.IndexEntry{
		Name:    "Active",
		Type:    state.NodeLeaf,
		State:   state.StatusComplete,
		Address: "active",
	})
	addToRoot(t, projDir, "active")

	err := svc.Restore("active")
	if err == nil {
		t.Fatal("expected error when restoring non-archived node")
	}
	if !strings.Contains(err.Error(), "not archived") {
		t.Errorf("error should mention 'not archived', got: %v", err)
	}
}

func TestRestore_RejectsNonRootArchived(t *testing.T) {
	t.Parallel()
	svc, projDir := testService(t, clock.NewMock(time.Now()))

	// A node that's archived but not in ArchivedRoot (a child).
	now := time.Now()
	store := state.NewStateStore(projDir, 5*time.Second)
	if err := store.MutateIndex(func(idx *state.RootIndex) error {
		idx.Nodes["parent/child"] = state.IndexEntry{
			Name:       "child",
			Type:       state.NodeLeaf,
			State:      state.StatusComplete,
			Address:    "parent/child",
			Parent:     "parent",
			Archived:   true,
			ArchivedAt: &now,
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	err := svc.Restore("parent/child")
	if err == nil {
		t.Fatal("expected error when restoring non-root archived node")
	}
	if !strings.Contains(err.Error(), "not a root-level") {
		t.Errorf("error should mention 'not a root-level', got: %v", err)
	}
}

// ── Delete tests ────────────────────────────────────────────────────────

func TestDelete_RemovesArchivedDirAndIndex(t *testing.T) {
	t.Parallel()
	svc, projDir := testService(t, clock.NewMock(time.Now()))

	// Set up an archived node with a child.
	now := time.Now()
	archiveRoot := filepath.Join(projDir, ".archive", "dead-project")
	writeJSON(t, filepath.Join(archiveRoot, "state.json"), map[string]string{"id": "dead-project"})
	writeJSON(t, filepath.Join(archiveRoot, "child", "state.json"), map[string]string{"id": "child"})

	store := state.NewStateStore(projDir, 5*time.Second)
	if err := store.MutateIndex(func(idx *state.RootIndex) error {
		idx.ArchivedRoot = append(idx.ArchivedRoot, "dead-project")
		idx.Nodes["dead-project"] = state.IndexEntry{
			Name:       "Dead",
			Type:       state.NodeOrchestrator,
			State:      state.StatusComplete,
			Address:    "dead-project",
			Children:   []string{"dead-project/child"},
			Archived:   true,
			ArchivedAt: &now,
		}
		idx.Nodes["dead-project/child"] = state.IndexEntry{
			Name:       "Child",
			Type:       state.NodeLeaf,
			State:      state.StatusComplete,
			Address:    "dead-project/child",
			Parent:     "dead-project",
			Archived:   true,
			ArchivedAt: &now,
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := svc.Delete("dead-project"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Archive directory should be gone.
	if _, err := os.Stat(archiveRoot); !os.IsNotExist(err) {
		t.Error("expected archived directory to be removed")
	}

	// Both entries should be gone from the index.
	idx, _ := svc.Store.ReadIndex()
	if _, ok := idx.Nodes["dead-project"]; ok {
		t.Error("dead-project should be removed from index")
	}
	if _, ok := idx.Nodes["dead-project/child"]; ok {
		t.Error("dead-project/child should be removed from index")
	}
	for _, r := range idx.ArchivedRoot {
		if r == "dead-project" {
			t.Error("dead-project should be removed from ArchivedRoot")
		}
	}
}

func TestDelete_RejectsNotArchived(t *testing.T) {
	t.Parallel()
	svc, projDir := testService(t, clock.NewMock(time.Now()))

	setupNode(t, projDir, "live", state.NewNodeState("live", "Live", state.NodeLeaf), state.IndexEntry{
		Name:    "Live",
		Type:    state.NodeLeaf,
		State:   state.StatusInProgress,
		Address: "live",
	})
	addToRoot(t, projDir, "live")

	err := svc.Delete("live")
	if err == nil {
		t.Fatal("expected error when deleting non-archived node")
	}
	if !strings.Contains(err.Error(), "not archived") {
		t.Errorf("error should mention 'not archived', got: %v", err)
	}
}

func TestDelete_RejectsNonRootArchived(t *testing.T) {
	t.Parallel()
	svc, projDir := testService(t, clock.NewMock(time.Now()))

	now := time.Now()
	store := state.NewStateStore(projDir, 5*time.Second)
	if err := store.MutateIndex(func(idx *state.RootIndex) error {
		idx.Nodes["parent/child"] = state.IndexEntry{
			Name:       "child",
			Type:       state.NodeLeaf,
			Address:    "parent/child",
			Parent:     "parent",
			Archived:   true,
			ArchivedAt: &now,
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	err := svc.Delete("parent/child")
	if err == nil {
		t.Fatal("expected error when deleting non-root archived node")
	}
	if !strings.Contains(err.Error(), "not a root-level") {
		t.Errorf("error should mention 'not a root-level', got: %v", err)
	}
}

// ── CollectSubtree tests ────────────────────────────────────────────────

func TestCollectSubtree_SingleNode(t *testing.T) {
	t.Parallel()
	idx := &state.RootIndex{
		Nodes: map[string]state.IndexEntry{
			"solo": {Name: "Solo", Address: "solo"},
		},
	}
	result := CollectSubtree(idx, "solo")
	if len(result) != 1 || result[0] != "solo" {
		t.Errorf("expected [solo], got %v", result)
	}
}

func TestCollectSubtree_DeepTree(t *testing.T) {
	t.Parallel()
	idx := &state.RootIndex{
		Nodes: map[string]state.IndexEntry{
			"root":             {Name: "Root", Address: "root", Children: []string{"root/a", "root/b"}},
			"root/a":          {Name: "A", Address: "root/a", Children: []string{"root/a/deep"}},
			"root/b":          {Name: "B", Address: "root/b"},
			"root/a/deep":     {Name: "Deep", Address: "root/a/deep"},
		},
	}
	result := CollectSubtree(idx, "root")
	if len(result) != 4 {
		t.Fatalf("expected 4 nodes, got %d: %v", len(result), result)
	}
	// Root should be first.
	if result[0] != "root" {
		t.Errorf("first element should be root, got %s", result[0])
	}
	// All nodes should be present.
	seen := map[string]bool{}
	for _, r := range result {
		seen[r] = true
	}
	for _, want := range []string{"root", "root/a", "root/b", "root/a/deep"} {
		if !seen[want] {
			t.Errorf("missing %s in subtree", want)
		}
	}
}

func TestCollectSubtree_UnknownRoot(t *testing.T) {
	t.Parallel()
	idx := &state.RootIndex{Nodes: map[string]state.IndexEntry{}}
	result := CollectSubtree(idx, "ghost")
	// Should still return the root address even if not in the index.
	if len(result) != 1 || result[0] != "ghost" {
		t.Errorf("expected [ghost], got %v", result)
	}
}

// ── Restore not found ───────────────────────────────────────────────────

func TestRestore_RejectsUnknownNode(t *testing.T) {
	t.Parallel()
	svc, _ := testService(t, clock.NewMock(time.Now()))

	err := svc.Restore("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown node")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestDelete_RejectsUnknownNode(t *testing.T) {
	t.Parallel()
	svc, _ := testService(t, clock.NewMock(time.Now()))

	err := svc.Delete("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown node")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

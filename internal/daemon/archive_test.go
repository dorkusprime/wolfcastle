package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

func archiveTestDaemon(t *testing.T, clk *clock.MockClock) *Daemon {
	t.Helper()
	d := testDaemon(t)
	d.Clock = clk
	d.Config.Archive = config.ArchiveConfig{
		AutoArchiveEnabled:    true,
		AutoArchiveDelayHours: 24,
		PollIntervalSeconds:   300,
	}
	return d
}

func setupCompletedOrchestrator(t *testing.T, d *Daemon, addr string, completedAt time.Time, children []string) {
	t.Helper()
	projDir := d.Store.Dir()
	idx, err := d.Store.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}

	// Add root entry
	idx.Root = append(idx.Root, addr)
	idx.Nodes[addr] = state.IndexEntry{
		Name:     addr,
		Type:     state.NodeOrchestrator,
		State:    state.StatusComplete,
		Address:  addr,
		Children: children,
	}
	for _, child := range children {
		idx.Nodes[child] = state.IndexEntry{
			Name:    child,
			Type:    state.NodeLeaf,
			State:   state.StatusComplete,
			Address: child,
			Parent:  addr,
		}
	}

	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	// Write root node state with CompletedAt
	ns := state.NewNodeState(addr, addr, state.NodeOrchestrator)
	ns.State = state.StatusComplete
	ns.Audit.CompletedAt = &completedAt
	ns.Audit.ResultSummary = "All done."
	writeJSON(t, filepath.Join(projDir, addr, "state.json"), ns)

	// Write child node states
	for _, child := range children {
		cns := state.NewNodeState(child, child, state.NodeLeaf)
		cns.State = state.StatusComplete
		cns.Audit.CompletedAt = &completedAt
		writeJSON(t, filepath.Join(projDir, child, "state.json"), cns)
	}
}

// --- findArchiveEligible ---

func TestFindArchiveEligible_CompletedAndOldEnough(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-25 * time.Hour) // 25h ago, past the 24h threshold
	setupCompletedOrchestrator(t, d, "old-project", completedAt, nil)

	idx, _ := d.Store.ReadIndex()
	eligible := d.findArchiveEligible(idx)
	if len(eligible) != 1 || eligible[0] != "old-project" {
		t.Errorf("expected [old-project], got %v", eligible)
	}
}

func TestFindArchiveEligible_TooRecent(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-12 * time.Hour) // 12h ago, under the 24h threshold
	setupCompletedOrchestrator(t, d, "recent-project", completedAt, nil)

	idx, _ := d.Store.ReadIndex()
	eligible := d.findArchiveEligible(idx)
	if len(eligible) != 0 {
		t.Errorf("expected no eligible nodes, got %v", eligible)
	}
}

func TestFindArchiveEligible_NotComplete(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	projDir := d.Store.Dir()
	idx := state.NewRootIndex()
	idx.Root = []string{"active-project"}
	idx.Nodes["active-project"] = state.IndexEntry{
		Name:    "active-project",
		Type:    state.NodeOrchestrator,
		State:   state.StatusInProgress,
		Address: "active-project",
	}
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	eligible := d.findArchiveEligible(idx)
	if len(eligible) != 0 {
		t.Errorf("expected no eligible nodes for in-progress, got %v", eligible)
	}
}

func TestFindArchiveEligible_AlreadyArchived(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-48 * time.Hour)
	setupCompletedOrchestrator(t, d, "archived-project", completedAt, nil)

	// Mark it archived in the index
	idx, _ := d.Store.ReadIndex()
	e := idx.Nodes["archived-project"]
	e.Archived = true
	idx.Nodes["archived-project"] = e
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	idx, _ = d.Store.ReadIndex()
	eligible := d.findArchiveEligible(idx)
	if len(eligible) != 0 {
		t.Errorf("expected no eligible nodes for already-archived, got %v", eligible)
	}
}

func TestFindArchiveEligible_NoCompletedAt(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	projDir := d.Store.Dir()
	idx := state.NewRootIndex()
	idx.Root = []string{"no-timestamp"}
	idx.Nodes["no-timestamp"] = state.IndexEntry{
		Name:    "no-timestamp",
		Type:    state.NodeOrchestrator,
		State:   state.StatusComplete,
		Address: "no-timestamp",
	}
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	// Write node state without CompletedAt
	ns := state.NewNodeState("no-timestamp", "no-timestamp", state.NodeOrchestrator)
	ns.State = state.StatusComplete
	writeJSON(t, filepath.Join(projDir, "no-timestamp", "state.json"), ns)

	eligible := d.findArchiveEligible(idx)
	if len(eligible) != 0 {
		t.Errorf("expected no eligible nodes without CompletedAt, got %v", eligible)
	}
}

// --- archiveNode ---

func TestArchiveNode_MovesDirectoriesAndUpdatesIndex(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-25 * time.Hour)
	setupCompletedOrchestrator(t, d, "my-project", completedAt, []string{"my-project/child-a"})

	err := d.archiveNode("my-project")
	if err != nil {
		t.Fatalf("archiveNode failed: %v", err)
	}

	projDir := d.Store.Dir()

	// Active directories should be gone
	if _, err := os.Stat(filepath.Join(projDir, "my-project")); !os.IsNotExist(err) {
		t.Error("expected active dir to be removed after archive")
	}

	// Archived directories should exist
	if _, err := os.Stat(filepath.Join(projDir, ".archive", "my-project", "state.json")); err != nil {
		t.Errorf("expected archived root state.json, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projDir, ".archive", "my-project", "child-a", "state.json")); err != nil {
		t.Errorf("expected archived child state.json, got: %v", err)
	}

	// Index should reflect the archive
	idx, err := d.Store.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}

	// Root array should not contain the address
	for _, r := range idx.Root {
		if r == "my-project" {
			t.Error("my-project should not be in Root after archive")
		}
	}

	// ArchivedRoot should contain it
	found := false
	for _, r := range idx.ArchivedRoot {
		if r == "my-project" {
			found = true
		}
	}
	if !found {
		t.Error("my-project should be in ArchivedRoot after archive")
	}

	// IndexEntry should be flagged
	entry := idx.Nodes["my-project"]
	if !entry.Archived {
		t.Error("expected Archived=true on my-project")
	}
	if entry.ArchivedAt == nil {
		t.Error("expected ArchivedAt to be set")
	}

	childEntry := idx.Nodes["my-project/child-a"]
	if !childEntry.Archived {
		t.Error("expected Archived=true on child-a")
	}

	// Markdown rollup should exist
	archiveMarkdownDir := filepath.Join(d.WolfcastleDir, "archive")
	entries, err := os.ReadDir(archiveMarkdownDir)
	if err != nil {
		t.Fatalf("reading archive markdown dir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 markdown rollup file, got %d", len(entries))
	}
}

// --- collectSubtree ---

func TestCollectSubtree_RootOnly(t *testing.T) {
	t.Parallel()
	idx := state.NewRootIndex()
	idx.Nodes["solo"] = state.IndexEntry{Address: "solo"}

	result := collectSubtree(idx, "solo")
	if len(result) != 1 || result[0] != "solo" {
		t.Errorf("expected [solo], got %v", result)
	}
}

func TestCollectSubtree_WithChildren(t *testing.T) {
	t.Parallel()
	idx := state.NewRootIndex()
	idx.Nodes["root"] = state.IndexEntry{
		Address:  "root",
		Children: []string{"root/a", "root/b"},
	}
	idx.Nodes["root/a"] = state.IndexEntry{
		Address:  "root/a",
		Children: []string{"root/a/x"},
	}
	idx.Nodes["root/a/x"] = state.IndexEntry{Address: "root/a/x"}
	idx.Nodes["root/b"] = state.IndexEntry{Address: "root/b"}

	result := collectSubtree(idx, "root")
	expected := map[string]bool{"root": true, "root/a": true, "root/a/x": true, "root/b": true}
	if len(result) != len(expected) {
		t.Fatalf("expected %d nodes, got %d: %v", len(expected), len(result), result)
	}
	for _, addr := range result {
		if !expected[addr] {
			t.Errorf("unexpected address in subtree: %s", addr)
		}
	}
}

// --- tryAutoArchive ---

func TestTryAutoArchive_Disabled(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)
	d.Config.Archive.AutoArchiveEnabled = false

	completedAt := now.Add(-25 * time.Hour)
	setupCompletedOrchestrator(t, d, "proj", completedAt, nil)

	idx, _ := d.Store.ReadIndex()
	if d.tryAutoArchive(idx) {
		t.Error("tryAutoArchive should return false when disabled")
	}
}

func TestTryAutoArchive_ThrottledByPollInterval(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-25 * time.Hour)
	setupCompletedOrchestrator(t, d, "proj", completedAt, nil)

	idx, _ := d.Store.ReadIndex()

	// First call should archive
	if !d.tryAutoArchive(idx) {
		t.Fatal("first tryAutoArchive should succeed")
	}

	// Set up a second eligible project
	clk.Advance(1 * time.Minute) // Only 1 minute later, poll interval is 300s
	setupCompletedOrchestrator(t, d, "proj2", completedAt, nil)
	idx, _ = d.Store.ReadIndex()

	if d.tryAutoArchive(idx) {
		t.Error("second tryAutoArchive should be throttled by poll interval")
	}

	// Advance past the poll interval
	clk.Advance(5 * time.Minute)
	if !d.tryAutoArchive(idx) {
		t.Error("tryAutoArchive should succeed after poll interval elapses")
	}
}

func TestTryAutoArchive_ArchivesOneAtATime(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-48 * time.Hour)
	setupCompletedOrchestrator(t, d, "proj-a", completedAt, nil)
	setupCompletedOrchestrator(t, d, "proj-b", completedAt, nil)

	idx, _ := d.Store.ReadIndex()

	// First call archives one
	if !d.tryAutoArchive(idx) {
		t.Fatal("first tryAutoArchive should succeed")
	}

	// Re-read index; one should be archived, one should remain
	idx, _ = d.Store.ReadIndex()
	archivedCount := 0
	for _, addr := range []string{"proj-a", "proj-b"} {
		if e, ok := idx.Nodes[addr]; ok && e.Archived {
			archivedCount++
		}
	}
	if archivedCount != 1 {
		t.Errorf("expected exactly 1 archived after first call, got %d", archivedCount)
	}
}

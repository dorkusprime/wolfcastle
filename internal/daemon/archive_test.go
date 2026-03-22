package daemon

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestFindArchiveEligible_NestedNodesExcluded(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-48 * time.Hour)
	setupCompletedOrchestrator(t, d, "parent-proj", completedAt, []string{"parent-proj/child"})

	// Verify: the child is in Nodes and is complete, but NOT in Root.
	// findArchiveEligible only iterates idx.Root, so child must be excluded.
	idx, _ := d.Store.ReadIndex()
	eligible := d.findArchiveEligible(idx)

	for _, addr := range eligible {
		if addr == "parent-proj/child" {
			t.Error("nested node parent-proj/child should not be eligible for archival")
		}
	}
	// Parent should be eligible.
	if len(eligible) != 1 || eligible[0] != "parent-proj" {
		t.Errorf("expected [parent-proj], got %v", eligible)
	}
}

func TestFindArchiveEligible_MissingNodeEntry(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	projDir := d.Store.Dir()
	idx := state.NewRootIndex()
	// Address in Root but no corresponding entry in Nodes.
	idx.Root = []string{"phantom"}
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	idx, _ = d.Store.ReadIndex()
	eligible := d.findArchiveEligible(idx)
	if len(eligible) != 0 {
		t.Errorf("expected no eligible nodes for missing Nodes entry, got %v", eligible)
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

func TestArchiveNode_CorruptNodeState(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	projDir := d.Store.Dir()
	idx := state.NewRootIndex()
	idx.Root = []string{"corrupt-proj"}
	idx.Nodes["corrupt-proj"] = state.IndexEntry{
		Name:    "corrupt-proj",
		Type:    state.NodeOrchestrator,
		State:   state.StatusComplete,
		Address: "corrupt-proj",
	}
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	// Write corrupt JSON so ReadNode returns an error.
	nodeDir := filepath.Join(projDir, "corrupt-proj")
	if err := os.MkdirAll(nodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeDir, "state.json"), []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := d.archiveNode("corrupt-proj")
	if err == nil {
		t.Fatal("expected error from archiveNode when node state is corrupt")
	}
}

func TestArchiveNode_MarkdownDirCreationError(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-25 * time.Hour)
	setupCompletedOrchestrator(t, d, "md-fail", completedAt, nil)

	// Place a regular file where the archive markdown directory would go,
	// so os.MkdirAll fails.
	blockingFile := filepath.Join(d.WolfcastleDir, "archive")
	if err := os.WriteFile(blockingFile, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := d.archiveNode("md-fail")
	if err == nil {
		t.Fatal("expected error when archive markdown dir creation is blocked by a file")
	}
}

func TestTryAutoArchive_NoEligibleAfterPoll(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	// No completed orchestrators at all, but auto-archive is enabled.
	idx, _ := d.Store.ReadIndex()
	result := d.tryAutoArchive(idx)
	if result {
		t.Error("tryAutoArchive should return false when no nodes are eligible")
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

func TestTryAutoArchive_ArchiveErrorDoesNotCrash(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-48 * time.Hour)

	// Set up an eligible node with a valid state file.
	setupCompletedOrchestrator(t, d, "broken-proj", completedAt, nil)

	idx, _ := d.Store.ReadIndex()

	// Make the wolfcastle archive directory unwritable so the markdown rollup
	// write fails inside archiveNode, after ReadNode succeeds.
	badDir := filepath.Join(d.WolfcastleDir, "archive")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(badDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(badDir, 0o755) })

	// tryAutoArchive should return false (error path), not panic.
	result := d.tryAutoArchive(idx)
	if result {
		t.Error("tryAutoArchive should return false when archiveNode errors")
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

// --- restoreNode ---

// archiveAndVerify is a test helper that archives a node and confirms it moved.
func archiveAndVerify(t *testing.T, d *Daemon, addr string) {
	t.Helper()
	if err := d.archiveNode(addr); err != nil {
		t.Fatalf("archiveNode(%s) failed: %v", addr, err)
	}
}

func TestRestoreNode_RestoresDirectoriesAndUpdatesIndex(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-25 * time.Hour)
	setupCompletedOrchestrator(t, d, "restore-proj", completedAt, []string{"restore-proj/child-a"})
	archiveAndVerify(t, d, "restore-proj")

	// Restore the archived node.
	err := d.restoreNode("restore-proj")
	if err != nil {
		t.Fatalf("restoreNode failed: %v", err)
	}

	projDir := d.Store.Dir()

	// Active directories should be back.
	if _, err := os.Stat(filepath.Join(projDir, "restore-proj", "state.json")); err != nil {
		t.Errorf("expected restored root state.json, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projDir, "restore-proj", "child-a", "state.json")); err != nil {
		t.Errorf("expected restored child state.json, got: %v", err)
	}

	// Archive directories should be gone.
	if _, err := os.Stat(filepath.Join(projDir, ".archive", "restore-proj")); !os.IsNotExist(err) {
		t.Error("expected .archive/restore-proj to be removed after restore")
	}

	// Index should reflect the restore.
	idx, err := d.Store.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}

	// Root should contain the address again.
	foundInRoot := false
	for _, r := range idx.Root {
		if r == "restore-proj" {
			foundInRoot = true
		}
	}
	if !foundInRoot {
		t.Error("restore-proj should be in Root after restore")
	}

	// ArchivedRoot should no longer contain it.
	for _, r := range idx.ArchivedRoot {
		if r == "restore-proj" {
			t.Error("restore-proj should not be in ArchivedRoot after restore")
		}
	}

	// IndexEntry flags should be cleared.
	entry := idx.Nodes["restore-proj"]
	if entry.Archived {
		t.Error("expected Archived=false on restore-proj after restore")
	}
	if entry.ArchivedAt != nil {
		t.Error("expected ArchivedAt=nil on restore-proj after restore")
	}

	childEntry := idx.Nodes["restore-proj/child-a"]
	if childEntry.Archived {
		t.Error("expected Archived=false on child-a after restore")
	}
	if childEntry.ArchivedAt != nil {
		t.Error("expected ArchivedAt=nil on child-a after restore")
	}
}

func TestRestoreNode_NotFoundInIndex(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	err := d.restoreNode("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestRestoreNode_NotArchived(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-25 * time.Hour)
	setupCompletedOrchestrator(t, d, "active-proj", completedAt, nil)

	err := d.restoreNode("active-proj")
	if err == nil {
		t.Fatal("expected error when restoring a non-archived node")
	}
}

func TestRestoreNode_NotInArchivedRoot(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	// Manually set up a node that has Archived=true but is not in ArchivedRoot
	// (e.g. a child node that was archived as part of a parent).
	completedAt := now.Add(-25 * time.Hour)
	setupCompletedOrchestrator(t, d, "parent", completedAt, []string{"parent/child"})
	archiveAndVerify(t, d, "parent")

	err := d.restoreNode("parent/child")
	if err == nil {
		t.Fatal("expected error when restoring a non-root archived node")
	}
}

func TestRestoreNode_RoundTripPreservesDirectoryStructure(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-25 * time.Hour)
	setupCompletedOrchestrator(t, d, "round-trip", completedAt, []string{"round-trip/leaf-a", "round-trip/leaf-b"})

	projDir := d.Store.Dir()

	// Snapshot the original directory entries before archiving.
	originalRoot, err := os.ReadDir(filepath.Join(projDir, "round-trip"))
	if err != nil {
		t.Fatal(err)
	}
	originalNames := make(map[string]bool)
	for _, e := range originalRoot {
		originalNames[e.Name()] = true
	}

	archiveAndVerify(t, d, "round-trip")

	if err := d.restoreNode("round-trip"); err != nil {
		t.Fatalf("restoreNode failed: %v", err)
	}

	// After round-trip, the directory listing should match the original.
	restoredRoot, err := os.ReadDir(filepath.Join(projDir, "round-trip"))
	if err != nil {
		t.Fatalf("reading restored dir: %v", err)
	}
	restoredNames := make(map[string]bool)
	for _, e := range restoredRoot {
		restoredNames[e.Name()] = true
	}

	for name := range originalNames {
		if !restoredNames[name] {
			t.Errorf("directory entry %q present before archive but missing after restore", name)
		}
	}
	for name := range restoredNames {
		if !originalNames[name] {
			t.Errorf("unexpected directory entry %q appeared after restore", name)
		}
	}

	// Verify index symmetry: flags should be fully cleared.
	idx, _ := d.Store.ReadIndex()
	for _, addr := range []string{"round-trip", "round-trip/leaf-a", "round-trip/leaf-b"} {
		e := idx.Nodes[addr]
		if e.Archived {
			t.Errorf("%s should not be archived after round-trip", addr)
		}
		if e.ArchivedAt != nil {
			t.Errorf("%s ArchivedAt should be nil after round-trip", addr)
		}
	}
}

func TestRestoreNode_RenameError(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-25 * time.Hour)
	setupCompletedOrchestrator(t, d, "rename-fail", completedAt, nil)
	archiveAndVerify(t, d, "rename-fail")

	projDir := d.Store.Dir()

	// Place a regular file where the active directory would go, blocking os.Rename.
	if err := os.WriteFile(filepath.Join(projDir, "rename-fail"), []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := d.restoreNode("rename-fail")
	if err == nil {
		t.Fatal("expected error when os.Rename cannot restore the directory")
	}
}

// --- deleteArchivedNode ---

func TestDeleteArchivedNode_RemovesDirectoriesAndPurgesIndex(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-25 * time.Hour)
	setupCompletedOrchestrator(t, d, "del-proj", completedAt, []string{"del-proj/child-a"})
	archiveAndVerify(t, d, "del-proj")

	projDir := d.Store.Dir()

	// Confirm archive directories exist before delete.
	if _, err := os.Stat(filepath.Join(projDir, ".archive", "del-proj")); err != nil {
		t.Fatalf("expected archive dir to exist before delete: %v", err)
	}

	err := d.deleteArchivedNode("del-proj")
	if err != nil {
		t.Fatalf("deleteArchivedNode failed: %v", err)
	}

	// Archive directories should be gone.
	if _, err := os.Stat(filepath.Join(projDir, ".archive", "del-proj")); !os.IsNotExist(err) {
		t.Error("expected .archive/del-proj to be removed after delete")
	}

	// Index should have no trace of the node or its children.
	idx, err := d.Store.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := idx.Nodes["del-proj"]; ok {
		t.Error("del-proj should be purged from Nodes after delete")
	}
	if _, ok := idx.Nodes["del-proj/child-a"]; ok {
		t.Error("del-proj/child-a should be purged from Nodes after delete")
	}

	for _, r := range idx.ArchivedRoot {
		if r == "del-proj" {
			t.Error("del-proj should not be in ArchivedRoot after delete")
		}
	}

	// Markdown rollup should still exist (permanent record).
	archiveMarkdownDir := filepath.Join(d.WolfcastleDir, "archive")
	entries, err := os.ReadDir(archiveMarkdownDir)
	if err != nil {
		t.Fatalf("reading archive markdown dir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 markdown rollup preserved, got %d", len(entries))
	}
}

func TestDeleteArchivedNode_NotFoundInIndex(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	err := d.deleteArchivedNode("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestDeleteArchivedNode_NotArchived(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-25 * time.Hour)
	setupCompletedOrchestrator(t, d, "active-proj", completedAt, nil)

	err := d.deleteArchivedNode("active-proj")
	if err == nil {
		t.Fatal("expected error when deleting a non-archived node")
	}
}

func TestDeleteArchivedNode_NotInArchivedRoot(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-25 * time.Hour)
	setupCompletedOrchestrator(t, d, "parent", completedAt, []string{"parent/child"})
	archiveAndVerify(t, d, "parent")

	// Try to delete a child node (archived but not in ArchivedRoot).
	err := d.deleteArchivedNode("parent/child")
	if err == nil {
		t.Fatal("expected error when deleting a non-root archived node")
	}
}

func TestDeleteArchivedNode_DeepSubtreePurgesAllDescendants(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-25 * time.Hour)

	// Build a three-level tree: root -> mid -> leaf
	projDir := d.Store.Dir()
	idx, _ := d.Store.ReadIndex()
	idx.Root = append(idx.Root, "deep-proj")
	idx.Nodes["deep-proj"] = state.IndexEntry{
		Name:     "deep-proj",
		Type:     state.NodeOrchestrator,
		State:    state.StatusComplete,
		Address:  "deep-proj",
		Children: []string{"deep-proj/mid"},
	}
	idx.Nodes["deep-proj/mid"] = state.IndexEntry{
		Name:     "mid",
		Type:     state.NodeOrchestrator,
		State:    state.StatusComplete,
		Address:  "deep-proj/mid",
		Parent:   "deep-proj",
		Children: []string{"deep-proj/mid/leaf"},
	}
	idx.Nodes["deep-proj/mid/leaf"] = state.IndexEntry{
		Name:    "leaf",
		Type:    state.NodeLeaf,
		State:   state.StatusComplete,
		Address: "deep-proj/mid/leaf",
		Parent:  "deep-proj/mid",
	}
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	ns := state.NewNodeState("deep-proj", "deep-proj", state.NodeOrchestrator)
	ns.State = state.StatusComplete
	ns.Audit.CompletedAt = &completedAt
	ns.Audit.ResultSummary = "Done."
	writeJSON(t, filepath.Join(projDir, "deep-proj", "state.json"), ns)

	mns := state.NewNodeState("mid", "deep-proj/mid", state.NodeOrchestrator)
	mns.State = state.StatusComplete
	mns.Audit.CompletedAt = &completedAt
	writeJSON(t, filepath.Join(projDir, "deep-proj", "mid", "state.json"), mns)

	lns := state.NewNodeState("leaf", "deep-proj/mid/leaf", state.NodeLeaf)
	lns.State = state.StatusComplete
	lns.Audit.CompletedAt = &completedAt
	writeJSON(t, filepath.Join(projDir, "deep-proj", "mid", "leaf", "state.json"), lns)

	archiveAndVerify(t, d, "deep-proj")

	if err := d.deleteArchivedNode("deep-proj"); err != nil {
		t.Fatalf("deleteArchivedNode failed: %v", err)
	}

	idx, _ = d.Store.ReadIndex()
	for _, addr := range []string{"deep-proj", "deep-proj/mid", "deep-proj/mid/leaf"} {
		if _, ok := idx.Nodes[addr]; ok {
			t.Errorf("%s should be purged from Nodes after delete", addr)
		}
	}
}

func TestDeleteArchivedNode_PreservesOtherArchivedNodes(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-25 * time.Hour)
	setupCompletedOrchestrator(t, d, "keep-proj", completedAt, nil)
	setupCompletedOrchestrator(t, d, "delete-proj", completedAt, nil)

	archiveAndVerify(t, d, "keep-proj")
	archiveAndVerify(t, d, "delete-proj")

	if err := d.deleteArchivedNode("delete-proj"); err != nil {
		t.Fatalf("deleteArchivedNode failed: %v", err)
	}

	idx, _ := d.Store.ReadIndex()

	// delete-proj should be gone entirely.
	if _, ok := idx.Nodes["delete-proj"]; ok {
		t.Error("delete-proj should be purged from Nodes")
	}
	for _, r := range idx.ArchivedRoot {
		if r == "delete-proj" {
			t.Error("delete-proj should not be in ArchivedRoot")
		}
	}

	// keep-proj should remain archived and untouched.
	keepEntry, ok := idx.Nodes["keep-proj"]
	if !ok {
		t.Fatal("keep-proj should still exist in Nodes")
	}
	if !keepEntry.Archived {
		t.Error("keep-proj should still be archived")
	}
	foundKeep := false
	for _, r := range idx.ArchivedRoot {
		if r == "keep-proj" {
			foundKeep = true
		}
	}
	if !foundKeep {
		t.Error("keep-proj should still be in ArchivedRoot")
	}
}

func TestDeleteArchivedNode_MissingArchiveDir(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-25 * time.Hour)
	setupCompletedOrchestrator(t, d, "no-dir", completedAt, nil)
	archiveAndVerify(t, d, "no-dir")

	// Manually remove the archive directory before calling delete.
	projDir := d.Store.Dir()
	_ = os.RemoveAll(filepath.Join(projDir, ".archive", "no-dir"))

	// Should still succeed (RemoveAll on nonexistent path returns nil).
	err := d.deleteArchivedNode("no-dir")
	if err != nil {
		t.Fatalf("deleteArchivedNode should tolerate missing archive dir: %v", err)
	}

	// Index should still be cleaned up.
	idx, _ := d.Store.ReadIndex()
	if _, ok := idx.Nodes["no-dir"]; ok {
		t.Error("no-dir should be purged from Nodes even without archive directory")
	}
}

func TestRestoreNode_MkdirAllError(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	clk := clock.NewMock(now)
	d := archiveTestDaemon(t, clk)

	completedAt := now.Add(-25 * time.Hour)
	children := []string{"rename-mkfail/nested/deep-child"}
	setupCompletedOrchestrator(t, d, "rename-mkfail", completedAt, children)
	archiveAndVerify(t, d, "rename-mkfail")

	projDir := d.Store.Dir()

	// The root node restores fine, but we block creation of the nested parent.
	// Place a read-only file where "rename-mkfail/nested" would need to be
	// created, so MkdirAll fails for the deep-child.
	nestParent := filepath.Join(projDir, "rename-mkfail", "nested")
	if err := os.MkdirAll(filepath.Dir(nestParent), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nestParent, []byte("blocker"), 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(nestParent, 0o755); _ = os.Remove(nestParent) })

	err := d.restoreNode("rename-mkfail")
	if err == nil {
		t.Fatal("expected error when MkdirAll cannot create parent directory for restore")
	}
}

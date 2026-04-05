package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════
// MutateNode: propagation path coverage (46.4% → target ~90%)
// ═══════════════════════════════════════════════════════════════════════════

func TestMutateNode_PropagatesStateToIndex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, 5*time.Second)

	// Seed a leaf node with a task.
	nodeDir := filepath.Join(dir, "my-leaf")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	ns := NewNodeState("my-leaf", "My Leaf", NodeLeaf)
	ns.Tasks = []Task{{ID: "task-0001", State: StatusNotStarted}}
	if err := SaveNodeState(filepath.Join(nodeDir, "state.json"), ns); err != nil {
		t.Fatal(err)
	}

	// Seed a root index referencing the leaf.
	idx := NewRootIndex()
	idx.Root = []string{"my-leaf"}
	idx.Nodes["my-leaf"] = IndexEntry{
		Name:    "My Leaf",
		Type:    NodeLeaf,
		State:   StatusNotStarted,
		Address: "my-leaf",
	}
	if err := SaveRootIndex(filepath.Join(dir, "state.json"), idx); err != nil {
		t.Fatal(err)
	}

	// Mutate the leaf to in_progress. MutateNode should propagate to index.
	err := s.MutateNode("my-leaf", func(ns *NodeState) error {
		ns.State = StatusInProgress
		ns.Tasks[0].State = StatusInProgress
		return nil
	})
	if err != nil {
		t.Fatalf("MutateNode error: %v", err)
	}

	got, err := s.ReadIndex()
	if err != nil {
		t.Fatalf("ReadIndex error: %v", err)
	}
	entry, ok := got.Nodes["my-leaf"]
	if !ok {
		t.Fatal("my-leaf not in index after propagation")
	}
	if entry.State != StatusInProgress {
		t.Errorf("expected index entry in_progress, got %s", entry.State)
	}
}

func TestMutateNode_PropagatesUpThroughParent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, 5*time.Second)

	// Seed an orchestrator parent.
	parentDir := filepath.Join(dir, "parent")
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		t.Fatal(err)
	}
	parentNS := NewNodeState("parent", "Parent", NodeOrchestrator)
	parentNS.Children = []ChildRef{
		{ID: "child", Address: "parent/child", State: StatusNotStarted},
	}
	if err := SaveNodeState(filepath.Join(parentDir, "state.json"), parentNS); err != nil {
		t.Fatal(err)
	}

	// Seed a leaf child.
	childDir := filepath.Join(dir, "parent", "child")
	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatal(err)
	}
	childNS := NewNodeState("child", "Child", NodeLeaf)
	childNS.Tasks = []Task{{ID: "task-0001", State: StatusNotStarted}}
	if err := SaveNodeState(filepath.Join(childDir, "state.json"), childNS); err != nil {
		t.Fatal(err)
	}

	// Seed root index with both nodes.
	idx := NewRootIndex()
	idx.Root = []string{"parent"}
	idx.Nodes["parent"] = IndexEntry{
		Name: "Parent", Type: NodeOrchestrator, State: StatusNotStarted, Address: "parent",
		Children: []string{"parent/child"},
	}
	idx.Nodes["parent/child"] = IndexEntry{
		Name: "Child", Type: NodeLeaf, State: StatusNotStarted, Address: "parent/child",
		Parent: "parent",
	}
	if err := SaveRootIndex(filepath.Join(dir, "state.json"), idx); err != nil {
		t.Fatal(err)
	}

	// Mutate child to in_progress; parent should propagate.
	err := s.MutateNode("parent/child", func(ns *NodeState) error {
		ns.State = StatusInProgress
		ns.Tasks[0].State = StatusInProgress
		return nil
	})
	if err != nil {
		t.Fatalf("MutateNode error: %v", err)
	}

	got, _ := s.ReadIndex()
	if got.Nodes["parent"].State != StatusInProgress {
		t.Errorf("expected parent in_progress in index, got %s", got.Nodes["parent"].State)
	}
	if got.RootState != StatusInProgress {
		t.Errorf("expected root state in_progress, got %s", got.RootState)
	}
}

func TestMutateNode_InvalidAddress_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, 5*time.Second)

	err := s.MutateNode("", func(*NodeState) error { return nil })
	if err == nil {
		t.Error("expected error for empty address")
	}
}

func TestMutateNode_SaveError_AfterMutation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, 5*time.Second)

	// Seed a node.
	nodeDir := filepath.Join(dir, "locked-node")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	ns := NewNodeState("locked-node", "Locked", NodeLeaf)
	if err := SaveNodeState(filepath.Join(nodeDir, "state.json"), ns); err != nil {
		t.Fatal(err)
	}

	// Make the directory read-only so SaveNodeState fails after LoadNodeState succeeds.
	if err := os.Chmod(nodeDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(nodeDir, 0755) })

	err := s.MutateNode("locked-node", func(ns *NodeState) error {
		ns.State = StatusInProgress
		return nil
	})
	if err == nil {
		t.Error("expected error when SaveNodeState fails")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// ReadNode / ReadIndex: non-ErrNotExist errors (corrupt JSON)
// ═══════════════════════════════════════════════════════════════════════════

func TestReadNode_CorruptJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, 5*time.Second)

	nodeDir := filepath.Join(dir, "bad-node")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeDir, "state.json"), []byte("{corrupt"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := s.ReadNode("bad-node")
	if err == nil {
		t.Error("expected error for corrupt node JSON")
	}
}

func TestReadIndex_CorruptJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, 5*time.Second)

	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("not-json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := s.ReadIndex()
	if err == nil {
		t.Error("expected error for corrupt index JSON")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// nodePath: edge cases
// ═══════════════════════════════════════════════════════════════════════════

func TestNodePath_DotSegment_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, 5*time.Second)

	_, err := s.ReadNode("parent/./child")
	if err == nil {
		t.Error("expected error for dot segment in address")
	}
}

func TestNodePath_DotDotSegment_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, 5*time.Second)

	_, err := s.ReadNode("parent/../escape")
	if err == nil {
		t.Error("expected error for .. segment in address")
	}
}

func TestNodePath_EmptySegment_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, 5*time.Second)

	_, err := s.ReadNode("parent//child")
	if err == nil {
		t.Error("expected error for empty segment in address")
	}
}

func TestNodePath_TabInSegment_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, 5*time.Second)

	_, err := s.ReadNode("bad\tnode")
	if err == nil {
		t.Error("expected error for tab in address segment")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// MutateIndex: corrupt JSON (non-ErrNotExist) error
// ═══════════════════════════════════════════════════════════════════════════

func TestMutateIndex_CorruptJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, 5*time.Second)

	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("bad{"), 0644); err != nil {
		t.Fatal(err)
	}

	err := s.MutateIndex(func(*RootIndex) error { return nil })
	if err == nil {
		t.Error("expected error for corrupt index JSON in MutateIndex")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// InboxMutate: callback error and save failure
// ═══════════════════════════════════════════════════════════════════════════

func TestInboxMutate_CallbackError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "inbox.json")

	sentinel := errors.New("mutate failed")
	err := InboxMutate(path, func(*InboxFile) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

func TestInboxMutate_SaveFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "inbox.json")

	// Seed an inbox file.
	if err := SaveInbox(path, &InboxFile{}); err != nil {
		t.Fatal(err)
	}

	// Make directory read-only so save fails.
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })

	err := InboxMutate(path, func(f *InboxFile) error {
		f.Items = append(f.Items, InboxItem{Text: "x"})
		return nil
	})
	if err == nil {
		t.Error("expected error when save fails in InboxMutate")
	}
}

func TestInboxAppend_SaveFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "inbox.json")

	if err := SaveInbox(path, &InboxFile{}); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })

	err := InboxAppend(path, InboxItem{Text: "test"})
	if err == nil {
		t.Error("expected error when save fails in InboxAppend")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// parentInList: no-dot edge case
// ═══════════════════════════════════════════════════════════════════════════

func TestParentInList_NoDot_ReturnsFalse(t *testing.T) {
	t.Parallel()
	tasks := []Task{{ID: "task-0001"}, {ID: "task-0002"}}
	if parentInList("task-0001", tasks) {
		t.Error("parentInList should return false for non-hierarchical ID")
	}
}

func TestParentInList_ParentNotFound(t *testing.T) {
	t.Parallel()
	tasks := []Task{{ID: "task-0002"}}
	if parentInList("task-0001.0001", tasks) {
		t.Error("parentInList should return false when parent task-0001 is not in list")
	}
}

func TestParentInList_ParentExists(t *testing.T) {
	t.Parallel()
	tasks := []Task{{ID: "task-0001"}, {ID: "task-0001.0001"}}
	if !parentInList("task-0001.0001", tasks) {
		t.Error("parentInList should return true when parent is present")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// TaskBlock: edge cases
// ═══════════════════════════════════════════════════════════════════════════

func TestTaskBlock_CompleteTask_ReturnsError(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("n", "N", NodeLeaf)
	ns.Tasks = []Task{{ID: "task-0001", State: StatusComplete}}

	err := TaskBlock(ns, "task-0001", "should fail")
	if err == nil {
		t.Error("expected error blocking a complete task")
	}
}

func TestTaskBlock_AlreadyBlocked_UpdatesReason(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("n", "N", NodeLeaf)
	ns.Tasks = []Task{{ID: "task-0001", State: StatusBlocked, BlockedReason: "old"}}

	err := TaskBlock(ns, "task-0001", "new reason")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ns.Tasks[0].BlockedReason != "new reason" {
		t.Errorf("expected updated reason, got %q", ns.Tasks[0].BlockedReason)
	}
}

func TestTaskBlock_AlreadyBlocked_EmptyReason_KeepsOld(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("n", "N", NodeLeaf)
	ns.Tasks = []Task{{ID: "task-0001", State: StatusBlocked, BlockedReason: "original"}}

	err := TaskBlock(ns, "task-0001", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ns.Tasks[0].BlockedReason != "original" {
		t.Errorf("expected reason unchanged, got %q", ns.Tasks[0].BlockedReason)
	}
}

func TestTaskBlock_AllNonAuditBlocked_AutoBlocksAudit(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("n", "N", NodeLeaf)
	ns.Tasks = []Task{
		{ID: "task-0001", State: StatusNotStarted},
		{ID: "task-0002", State: StatusBlocked, BlockedReason: "stuck"},
		{ID: "audit", State: StatusNotStarted, IsAudit: true},
	}

	// Block the last non-audit task. With all non-audit blocked and none complete,
	// the audit should auto-block.
	err := TaskBlock(ns, "task-0001", "also stuck")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ns.Tasks[2].State != StatusBlocked {
		t.Errorf("expected audit to be auto-blocked, got %s", ns.Tasks[2].State)
	}
	if ns.Tasks[2].BlockedReason != "all tasks blocked; nothing to audit" {
		t.Errorf("unexpected audit block reason: %q", ns.Tasks[2].BlockedReason)
	}
}

func TestTaskBlock_NotFoundTask_ReturnsError(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("n", "N", NodeLeaf)

	err := TaskBlock(ns, "nonexistent", "reason")
	if err == nil {
		t.Error("expected error for missing task")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RecomputeState: blocked derivation
// ═══════════════════════════════════════════════════════════════════════════

func TestRecomputeState_AllNonCompleteBlocked_CompleteAndBlocked(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusComplete},
		{ID: "b", State: StatusBlocked},
	}
	got := RecomputeState(children)
	if got != StatusBlocked {
		t.Errorf("expected blocked when all non-complete are blocked, got %s", got)
	}
}

func TestRecomputeState_MixedBlockedAndInProgress(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusBlocked},
		{ID: "b", State: StatusInProgress},
	}
	got := RecomputeState(children)
	if got != StatusInProgress {
		t.Errorf("expected in_progress with mixed blocked/in_progress, got %s", got)
	}
}

func TestRecomputeState_WithTasks_IncompleteAuditPreventsComplete(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusComplete},
	}
	tasks := []Task{
		{ID: "audit", State: StatusNotStarted, IsAudit: true},
	}
	got := RecomputeState(children, tasks)
	if got != StatusInProgress {
		t.Errorf("expected in_progress when audit incomplete, got %s", got)
	}
}

func TestRecomputeState_WithTasks_AllComplete(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusComplete},
	}
	tasks := []Task{
		{ID: "audit", State: StatusComplete, IsAudit: true},
	}
	got := RecomputeState(children, tasks)
	if got != StatusComplete {
		t.Errorf("expected complete when all children and tasks complete, got %s", got)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// LoadInbox: permission error (non-ErrNotExist)
// ═══════════════════════════════════════════════════════════════════════════

func TestLoadInbox_PermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "inbox.json")

	if err := os.WriteFile(path, []byte(`{"items":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0644) })

	_, err := LoadInbox(path)
	if err == nil {
		t.Error("expected error reading inbox with no permissions")
	}
}

func TestLoadInbox_CorruptJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "inbox.json")

	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadInbox(path)
	if err == nil {
		t.Error("expected error for corrupt inbox JSON")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// MutateInbox: corrupt inbox triggers load error
// ═══════════════════════════════════════════════════════════════════════════

func TestMutateInbox_CorruptInbox_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, 5*time.Second)

	if err := os.WriteFile(filepath.Join(dir, "inbox.json"), []byte("broken"), 0644); err != nil {
		t.Fatal(err)
	}

	err := s.MutateInbox(func(*InboxFile) error { return nil })
	if err == nil {
		t.Error("expected error for corrupt inbox in MutateInbox")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// PropagateUp: error paths
// ═══════════════════════════════════════════════════════════════════════════

func TestPropagateUp_LoadParentError_ReturnsWrapped(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("load failed")
	_, err := PropagateUp(
		"child",
		StatusInProgress,
		func(string) (*NodeState, error) { return nil, sentinel },
		func(string, *NodeState) error { return nil },
		func(string) string { return "parent" },
	)
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel, got %v", err)
	}
}

func TestPropagateUp_SaveParentError_Propagation(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("save failed")
	parent := NewNodeState("parent", "P", NodeOrchestrator)
	parent.Children = []ChildRef{{ID: "child", Address: "child", State: StatusNotStarted}}

	_, err := PropagateUp(
		"child",
		StatusInProgress,
		func(string) (*NodeState, error) { return parent, nil },
		func(string, *NodeState) error { return sentinel },
		func(addr string) string {
			if addr == "child" {
				return "parent"
			}
			return ""
		},
	)
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel, got %v", err)
	}
}

func TestPropagateUp_CycleDetection(t *testing.T) {
	t.Parallel()
	parent := NewNodeState("a", "A", NodeOrchestrator)
	parent.Children = []ChildRef{{ID: "b", Address: "b", State: StatusNotStarted}}

	_, err := PropagateUp(
		"b",
		StatusInProgress,
		func(string) (*NodeState, error) { return parent, nil },
		func(string, *NodeState) error { return nil },
		func(addr string) string {
			// Creates a cycle: b → a → b
			if addr == "b" {
				return "a"
			}
			return "b"
		},
	)
	if err == nil {
		t.Error("expected cycle detection error")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Propagate: error in re-walk ancestors
// ═══════════════════════════════════════════════════════════════════════════

func TestPropagate_LoadParentForIndex_Error(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Root = []string{"parent"}
	idx.Nodes["parent"] = IndexEntry{
		Name: "Parent", Type: NodeOrchestrator, State: StatusNotStarted, Address: "parent",
		Children: []string{"parent/child"},
	}
	idx.Nodes["parent/child"] = IndexEntry{
		Name: "Child", Type: NodeLeaf, State: StatusNotStarted, Address: "parent/child",
		Parent: "parent",
	}

	callCount := 0
	loadNode := func(addr string) (*NodeState, error) {
		callCount++
		if addr == "parent" {
			// First call from PropagateUp succeeds, second from re-walk fails.
			if callCount > 1 {
				return nil, fmt.Errorf("load failed on re-walk")
			}
			ns := NewNodeState("parent", "Parent", NodeOrchestrator)
			ns.Children = []ChildRef{{ID: "child", Address: "parent/child", State: StatusNotStarted}}
			return ns, nil
		}
		return NewNodeState("child", "Child", NodeLeaf), nil
	}

	err := Propagate("parent/child", StatusInProgress, idx, loadNode, func(string, *NodeState) error { return nil })
	if err == nil {
		t.Error("expected error when re-walk load fails")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// normalizeAuditState: coverage for default paths
// ═══════════════════════════════════════════════════════════════════════════

func TestNormalizeAuditState_DefaultsAllNilSlices(t *testing.T) {
	t.Parallel()
	ns := &NodeState{}
	normalizeAuditState(ns)

	if ns.Audit.Breadcrumbs == nil {
		t.Error("breadcrumbs should be non-nil")
	}
	if ns.Audit.Gaps == nil {
		t.Error("gaps should be non-nil")
	}
	if ns.Audit.Escalations == nil {
		t.Error("escalations should be non-nil")
	}
	if ns.Audit.Status != AuditPending {
		t.Errorf("expected pending audit status, got %s", ns.Audit.Status)
	}
}

func TestNormalizeAuditState_PreservesExistingInProgressStatus(t *testing.T) {
	t.Parallel()
	ns := &NodeState{Audit: AuditState{
		Status:      AuditInProgress,
		Breadcrumbs: []Breadcrumb{{Text: "existing"}},
		Gaps:        []Gap{},
		Escalations: []Escalation{},
	}}
	normalizeAuditState(ns)

	if ns.Audit.Status != AuditInProgress {
		t.Errorf("expected in_progress preserved, got %s", ns.Audit.Status)
	}
	if len(ns.Audit.Breadcrumbs) != 1 {
		t.Error("existing breadcrumbs should be preserved")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// LoadNodeState / LoadRootIndex: parse errors
// ═══════════════════════════════════════════════════════════════════════════

func TestLoadNodeState_CorruptJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("{bad json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadNodeState(path)
	if err == nil {
		t.Error("expected parse error for corrupt node state")
	}
}

func TestLoadRootIndex_CorruptJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRootIndex(path)
	if err == nil {
		t.Error("expected parse error for corrupt root index")
	}
}

func TestLoadRootIndex_NilNodesInitialized(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	// Valid JSON with no "nodes" field. Should initialize the map.
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0644); err != nil {
		t.Fatal(err)
	}

	idx, err := LoadRootIndex(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx.Nodes == nil {
		t.Error("Nodes map should be initialized even when absent from JSON")
	}
}

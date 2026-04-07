package state

import (
	"testing"
)

func TestTaskAddChild_CreatesHierarchicalID(t *testing.T) {
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "parent", State: StatusInProgress},
		{ID: "audit", Description: "audit", State: StatusNotStarted, IsAudit: true},
	}

	child, err := TaskAddChild(ns, "task-0001", "first child")
	if err != nil {
		t.Fatalf("TaskAddChild: %v", err)
	}
	if child.ID != "task-0001.0001" {
		t.Errorf("expected task-0001.0001, got %s", child.ID)
	}

	child2, err := TaskAddChild(ns, "task-0001", "second child")
	if err != nil {
		t.Fatalf("TaskAddChild: %v", err)
	}
	if child2.ID != "task-0001.0002" {
		t.Errorf("expected task-0001.0002, got %s", child2.ID)
	}
}

func TestTaskAddChild_NestedDecomposition(t *testing.T) {
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "root", State: StatusInProgress},
		{ID: "task-0001.0001", Description: "child", State: StatusInProgress},
	}

	grandchild, err := TaskAddChild(ns, "task-0001.0001", "grandchild")
	if err != nil {
		t.Fatalf("TaskAddChild: %v", err)
	}
	if grandchild.ID != "task-0001.0001.0001" {
		t.Errorf("expected task-0001.0001.0001, got %s", grandchild.ID)
	}
}

func TestTaskAddChild_ParentNotFound(t *testing.T) {
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "exists", State: StatusInProgress},
	}

	_, err := TaskAddChild(ns, "task-9999", "orphan")
	if err == nil {
		t.Fatal("expected error for missing parent")
	}
}

func TestTaskAddChild_InsertionOrder(t *testing.T) {
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "first", State: StatusInProgress},
		{ID: "task-0002", Description: "second", State: StatusNotStarted},
		{ID: "audit", Description: "audit", State: StatusNotStarted, IsAudit: true},
	}

	_, _ = TaskAddChild(ns, "task-0001", "child A")
	_, _ = TaskAddChild(ns, "task-0001", "child B")

	// Verify order: task-0001, task-0001.0001, task-0001.0002, task-0002, audit
	expected := []string{"task-0001", "task-0001.0001", "task-0001.0002", "task-0002", "audit"}
	if len(ns.Tasks) != len(expected) {
		t.Fatalf("expected %d tasks, got %d", len(expected), len(ns.Tasks))
	}
	for i, id := range expected {
		if ns.Tasks[i].ID != id {
			t.Errorf("position %d: expected %s, got %s", i, id, ns.Tasks[i].ID)
		}
	}
}

func TestTaskChildren_DetectsChildren(t *testing.T) {
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "parent", State: StatusInProgress},
		{ID: "task-0001.0001", Description: "child", State: StatusNotStarted},
		{ID: "task-0002", Description: "no children", State: StatusNotStarted},
	}

	if !TaskChildren(ns, "task-0001") {
		t.Error("task-0001 should have children")
	}
	if TaskChildren(ns, "task-0002") {
		t.Error("task-0002 should not have children")
	}
}

func TestDeriveParentStatus_AllComplete(t *testing.T) {
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "parent", State: StatusInProgress},
		{ID: "task-0001.0001", Description: "child a", State: StatusComplete},
		{ID: "task-0001.0002", Description: "child b", State: StatusComplete},
	}

	status, hasChildren := DeriveParentStatus(ns, "task-0001")
	if !hasChildren {
		t.Fatal("expected hasChildren=true")
	}
	if status != StatusComplete {
		t.Errorf("expected complete, got %s", status)
	}
}

func TestDeriveParentStatus_OneBlocked(t *testing.T) {
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "parent", State: StatusInProgress},
		{ID: "task-0001.0001", Description: "child a", State: StatusComplete},
		{ID: "task-0001.0002", Description: "child b", State: StatusBlocked},
	}

	status, _ := DeriveParentStatus(ns, "task-0001")
	if status != StatusBlocked {
		t.Errorf("expected blocked, got %s", status)
	}
}

func TestDeriveParentStatus_BlockedSupersededCountsAsComplete(t *testing.T) {
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "parent", State: StatusInProgress},
		{ID: "task-0001.0001", Description: "child a", State: StatusComplete},
		{ID: "task-0001.0002", Description: "child b", State: StatusBlocked, BlockedReason: "Superseded by node-level decomposition"},
		{ID: "task-0001.0003", Description: "child c", State: StatusBlocked, BlockedReason: "Decomposed into subtasks: task-0001.0001"},
	}

	status, hasChildren := DeriveParentStatus(ns, "task-0001")
	if !hasChildren {
		t.Fatal("expected hasChildren=true")
	}
	if status != StatusComplete {
		t.Errorf("expected complete (superseded/decomposed children should count as done), got %s", status)
	}
}

func TestDeriveParentStatus_NoChildren(t *testing.T) {
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "leaf task", State: StatusInProgress},
	}

	status, hasChildren := DeriveParentStatus(ns, "task-0001")
	if hasChildren {
		t.Error("expected hasChildren=false")
	}
	if status != StatusInProgress {
		t.Errorf("expected in_progress, got %s", status)
	}
}

func TestDeriveParentStatus_IgnoresGrandchildren(t *testing.T) {
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "root", State: StatusInProgress},
		{ID: "task-0001.0001", Description: "child", State: StatusComplete},
		{ID: "task-0001.0001.0001", Description: "grandchild", State: StatusNotStarted},
	}

	// Root's immediate children are all complete; grandchild doesn't count
	status, _ := DeriveParentStatus(ns, "task-0001")
	if status != StatusComplete {
		t.Errorf("expected complete (grandchild not counted), got %s", status)
	}
}

func TestDeriveParentStatus_AuditResetsWhenChildrenComplete(t *testing.T) {
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.Tasks = []Task{
		{ID: "audit", Description: "audit", State: StatusBlocked, IsAudit: true},
		{ID: "audit.0001", Description: "fix race condition", State: StatusComplete},
		{ID: "audit.0002", Description: "consolidate tests", State: StatusComplete},
	}

	status, hasChildren := DeriveParentStatus(ns, "audit")
	if !hasChildren {
		t.Fatal("expected hasChildren=true")
	}
	if status != StatusNotStarted {
		t.Errorf("audit should reset to not_started when children complete, got %s", status)
	}
}

func TestDeriveParentStatus_AuditCompletesAfterRerun(t *testing.T) {
	// Audit re-ran and passed (state=complete). Children are also complete.
	// DeriveParentStatus should return complete, not reset to not_started.
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.Tasks = []Task{
		{ID: "audit", Description: "audit", State: StatusComplete, IsAudit: true},
		{ID: "audit.0001", Description: "fix race condition", State: StatusComplete},
	}

	status, hasChildren := DeriveParentStatus(ns, "audit")
	if !hasChildren {
		t.Fatal("expected hasChildren=true")
	}
	if status != StatusComplete {
		t.Errorf("audit should be complete after re-verification, got %s", status)
	}
}

func TestDeriveParentStatus_AuditStaysInProgressWhenChildrenIncomplete(t *testing.T) {
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.Tasks = []Task{
		{ID: "audit", Description: "audit", State: StatusBlocked, IsAudit: true},
		{ID: "audit.0001", Description: "fix race condition", State: StatusComplete},
		{ID: "audit.0002", Description: "consolidate tests", State: StatusNotStarted},
	}

	status, hasChildren := DeriveParentStatus(ns, "audit")
	if !hasChildren {
		t.Fatal("expected hasChildren=true")
	}
	if status != StatusNotStarted {
		t.Errorf("audit with incomplete children should be not_started, got %s", status)
	}
}

func TestDeriveParentStatus_NonAuditStillCompletes(t *testing.T) {
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "parent", State: StatusInProgress},
		{ID: "task-0001.0001", Description: "child", State: StatusComplete},
	}

	status, _ := DeriveParentStatus(ns, "task-0001")
	if status != StatusComplete {
		t.Errorf("non-audit parent should complete, got %s", status)
	}
}

func TestNavigation_AuditRemediationChildrenRunnable(t *testing.T) {
	// audit is not_started (reset for re-verification) with remediation
	// children. Children must be runnable despite the parent being
	// not_started, because the audit's not_started state means
	// "waiting for children to finish."
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.State = StatusInProgress
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "work", State: StatusComplete},
		{ID: "audit", Description: "audit", State: StatusNotStarted, IsAudit: true},
		{ID: "audit.0001", Description: "fix race condition", State: StatusNotStarted},
	}

	idx := NewRootIndex()
	idx.Root = []string{"test"}
	idx.Nodes["test"] = IndexEntry{
		Name: "Test", Type: NodeLeaf, State: StatusInProgress, Address: "test",
	}

	loader := func(addr string) (*NodeState, error) { return ns, nil }
	nav, err := FindNextTask(idx, "", loader)
	if err != nil {
		t.Fatal(err)
	}
	if !nav.Found {
		t.Fatal("expected to find audit.0001 but navigation found nothing")
	}
	if nav.TaskID != "audit.0001" {
		t.Errorf("expected audit.0001, got %s", nav.TaskID)
	}
}

func TestNavigation_AuditRerunsAfterRemediationComplete(t *testing.T) {
	// All remediation children are complete. The audit should be
	// picked up for re-verification (DeriveParentStatus resets it
	// to not_started, and navigation should not skip it despite
	// having children).
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.State = StatusInProgress
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "work", State: StatusComplete},
		{ID: "audit", Description: "audit", State: StatusNotStarted, IsAudit: true},
		{ID: "audit.0001", Description: "fix race condition", State: StatusComplete},
	}

	idx := NewRootIndex()
	idx.Root = []string{"test"}
	idx.Nodes["test"] = IndexEntry{
		Name: "Test", Type: NodeLeaf, State: StatusInProgress, Address: "test",
	}

	loader := func(addr string) (*NodeState, error) { return ns, nil }
	nav, err := FindNextTask(idx, "", loader)
	if err != nil {
		t.Fatal(err)
	}
	if !nav.Found {
		t.Fatal("expected to find audit for re-verification but navigation found nothing")
	}
	if nav.TaskID != "audit" {
		t.Errorf("expected audit, got %s", nav.TaskID)
	}
}

func TestNavigation_HierarchicalDepthFirst(t *testing.T) {
	// task-0001 has children, task-0002 is a sibling.
	// Navigation should pick task-0001.0001 before task-0002.
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.State = StatusInProgress
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "parent", State: StatusInProgress},
		{ID: "task-0001.0001", Description: "child a", State: StatusNotStarted},
		{ID: "task-0001.0002", Description: "child b", State: StatusNotStarted},
		{ID: "task-0002", Description: "sibling", State: StatusNotStarted},
	}

	idx := NewRootIndex()
	idx.Root = []string{"test"}
	idx.Nodes["test"] = IndexEntry{
		Name: "Test", Type: NodeLeaf, State: StatusInProgress, Address: "test",
	}

	loader := func(addr string) (*NodeState, error) { return ns, nil }

	result, err := FindNextTask(idx, "", loader)
	if err != nil {
		t.Fatalf("FindNextTask: %v", err)
	}
	if !result.Found {
		t.Fatal("expected to find a task")
	}
	if result.TaskID != "task-0001.0001" {
		t.Errorf("expected task-0001.0001 (depth-first), got %s", result.TaskID)
	}
}

func TestNavigation_SkipsChildOfNotStartedParent(t *testing.T) {
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.State = StatusInProgress
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "not started parent", State: StatusNotStarted},
		{ID: "task-0001.0001", Description: "child", State: StatusNotStarted},
	}

	idx := NewRootIndex()
	idx.Root = []string{"test"}
	idx.Nodes["test"] = IndexEntry{
		Name: "Test", Type: NodeLeaf, State: StatusInProgress, Address: "test",
	}

	loader := func(addr string) (*NodeState, error) { return ns, nil }

	result, err := FindNextTask(idx, "", loader)
	if err != nil {
		t.Fatalf("FindNextTask: %v", err)
	}
	// task-0001 is a parent (has children), so it's skipped.
	// task-0001.0001 has a not_started ancestor, so it's skipped.
	// No actionable task.
	if result.Found {
		t.Errorf("expected no actionable task, got %s", result.TaskID)
	}
}

func TestNavigation_CrashRecoveryWithHierarchy(t *testing.T) {
	// task-0003.0002 is in_progress (crashed), task-0001 is not_started.
	// Crash recovery should pick task-0003.0002 despite ordering.
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.State = StatusInProgress
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "first", State: StatusNotStarted},
		{ID: "task-0002", Description: "second", State: StatusComplete},
		{ID: "task-0003", Description: "third parent", State: StatusInProgress},
		{ID: "task-0003.0001", Description: "done child", State: StatusComplete},
		{ID: "task-0003.0002", Description: "crashed child", State: StatusInProgress},
	}

	idx := NewRootIndex()
	idx.Root = []string{"test"}
	idx.Nodes["test"] = IndexEntry{
		Name: "Test", Type: NodeLeaf, State: StatusInProgress, Address: "test",
	}

	loader := func(addr string) (*NodeState, error) { return ns, nil }

	result, err := FindNextTask(idx, "", loader)
	if err != nil {
		t.Fatalf("FindNextTask: %v", err)
	}
	if !result.Found {
		t.Fatal("expected to find crashed task")
	}
	if result.TaskID != "task-0003.0002" {
		t.Errorf("crash recovery should pick task-0003.0002, got %s", result.TaskID)
	}
}

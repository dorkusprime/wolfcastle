package state

import "testing"

func TestSyncAuditLifecycle_NotStartedSetsPending(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.State = StatusNotStarted
	SyncAuditLifecycle(ns)
	if ns.Audit.Status != AuditPending {
		t.Errorf("expected pending, got %s", ns.Audit.Status)
	}
}

func TestSyncAuditLifecycle_InProgressSetsInProgress(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.State = StatusInProgress
	SyncAuditLifecycle(ns)
	if ns.Audit.Status != AuditInProgress {
		t.Errorf("expected in_progress, got %s", ns.Audit.Status)
	}
	if ns.Audit.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}
}

func TestSyncAuditLifecycle_CompleteWithNoGapsSetsPassed(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.State = StatusComplete
	SyncAuditLifecycle(ns)
	if ns.Audit.Status != AuditPassed {
		t.Errorf("expected passed, got %s", ns.Audit.Status)
	}
	if ns.Audit.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestSyncAuditLifecycle_CompleteWithOpenGapsBlocksTask(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.State = StatusComplete
	ns.Tasks = []Task{
		{ID: "task-0001", State: StatusComplete},
		{ID: "audit", State: StatusInProgress, IsAudit: true},
	}
	ns.Audit.Gaps = []Gap{
		{ID: "gap-1", Status: GapOpen, Description: "missing test"},
	}
	SyncAuditLifecycle(ns)
	if ns.Audit.Status != AuditFailed {
		t.Errorf("expected failed, got %s", ns.Audit.Status)
	}
	// Audit task should be blocked
	for _, task := range ns.Tasks {
		if task.IsAudit {
			if task.State != StatusBlocked {
				t.Errorf("expected audit task blocked, got %s", task.State)
			}
			if task.BlockedReason == "" {
				t.Error("expected blocked reason to be set")
			}
		}
	}
}

func TestSyncAuditLifecycle_BlockedSetsFailed(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.State = StatusBlocked
	SyncAuditLifecycle(ns)
	if ns.Audit.Status != AuditFailed {
		t.Errorf("expected failed, got %s", ns.Audit.Status)
	}
}

func TestSyncAuditLifecycle_FixingLastGapAllowsCompletion(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.State = StatusComplete
	ns.Audit.Gaps = []Gap{
		{ID: "gap-1", Status: GapFixed, Description: "was missing test"},
	}
	SyncAuditLifecycle(ns)
	if ns.Audit.Status != AuditPassed {
		t.Errorf("expected passed after all gaps fixed, got %s", ns.Audit.Status)
	}
}

func TestSyncAuditLifecycle_InProgressRecordsStartedAtOnce(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("test", "Test", NodeLeaf)
	ns.State = StatusInProgress
	SyncAuditLifecycle(ns)
	firstStartedAt := ns.Audit.StartedAt

	// Call again. Should not change StartedAt
	SyncAuditLifecycle(ns)
	if ns.Audit.StartedAt != firstStartedAt {
		t.Error("StartedAt should not change on subsequent calls")
	}
}

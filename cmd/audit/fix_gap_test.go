package audit

import (
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ---------------------------------------------------------------------------
// fix-gap with RemediationTaskID set
// ---------------------------------------------------------------------------

func TestFixGap_CompletesRemediationTask(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	// Set up a node with a gap, an audit task, and a remediation subtask.
	if err := env.App.State.MutateNode("my-project", func(ns *state.NodeState) error {
		ns.State = state.StatusBlocked
		ns.Tasks = []state.Task{
			{ID: "task-0001", Description: "do stuff", State: state.StatusComplete},
			{ID: "task-0002", Description: "audit", State: state.StatusBlocked, IsAudit: true, BlockedReason: "open gaps"},
			{ID: "task-0002.0001", Description: "Fix: missing tests\n\nAfter fixing, close the gap:\n  wolfcastle audit fix-gap --node my-project gap-my-project-1", State: state.StatusInProgress},
		}
		ns.Audit.Gaps = []state.Gap{
			{ID: "gap-my-project-1", Description: "missing tests", Status: state.GapOpen, RemediationTaskID: "task-0002.0001"},
		}
		ns.Audit.Status = state.AuditFailed
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "my-project", "gap-my-project-1"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("fix-gap failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")

	// Gap should be fixed.
	if ns.Audit.Gaps[0].Status != state.GapFixed {
		t.Errorf("expected gap fixed, got %s", ns.Audit.Gaps[0].Status)
	}

	// Remediation task should be complete.
	var remTask *state.Task
	for i := range ns.Tasks {
		if ns.Tasks[i].ID == "task-0002.0001" {
			remTask = &ns.Tasks[i]
			break
		}
	}
	if remTask == nil {
		t.Fatal("remediation task not found")
	}
	if remTask.State != state.StatusComplete {
		t.Errorf("expected remediation task complete, got %s", remTask.State)
	}

	// Audit task should be reset to not_started (all children complete).
	var auditTask *state.Task
	for i := range ns.Tasks {
		if ns.Tasks[i].ID == "task-0002" {
			auditTask = &ns.Tasks[i]
			break
		}
	}
	if auditTask == nil {
		t.Fatal("audit task not found")
	}
	if auditTask.State != state.StatusNotStarted {
		t.Errorf("expected audit task not_started, got %s", auditTask.State)
	}
}

// ---------------------------------------------------------------------------
// fix-gap with remediation task in various states
// ---------------------------------------------------------------------------

func TestFixGap_CompletesRemediationTask_NotStarted(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	if err := env.App.State.MutateNode("my-project", func(ns *state.NodeState) error {
		ns.State = state.StatusBlocked
		ns.Tasks = []state.Task{
			{ID: "task-0001", Description: "do stuff", State: state.StatusComplete},
			{ID: "task-0002", Description: "audit", State: state.StatusBlocked, IsAudit: true},
			{ID: "task-0002.0001", Description: "Fix: gap\n\nAfter fixing, close the gap:\n  wolfcastle audit fix-gap --node my-project gap-1", State: state.StatusNotStarted},
		}
		ns.Audit.Gaps = []state.Gap{
			{ID: "gap-1", Description: "gap", Status: state.GapOpen, RemediationTaskID: "task-0002.0001"},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "my-project", "gap-1"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("fix-gap failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	for _, task := range ns.Tasks {
		if task.ID == "task-0002.0001" {
			if task.State != state.StatusComplete {
				t.Errorf("expected complete, got %s", task.State)
			}
			return
		}
	}
	t.Fatal("remediation task not found")
}

func TestFixGap_CompletesRemediationTask_Blocked(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	if err := env.App.State.MutateNode("my-project", func(ns *state.NodeState) error {
		ns.State = state.StatusBlocked
		ns.Tasks = []state.Task{
			{ID: "task-0001", Description: "do stuff", State: state.StatusComplete},
			{ID: "task-0002", Description: "audit", State: state.StatusBlocked, IsAudit: true},
			{ID: "task-0002.0001", Description: "Fix: gap\n\nAfter fixing, close the gap:\n  wolfcastle audit fix-gap --node my-project gap-1", State: state.StatusBlocked, BlockedReason: "stuck"},
		}
		ns.Audit.Gaps = []state.Gap{
			{ID: "gap-1", Description: "gap", Status: state.GapOpen, RemediationTaskID: "task-0002.0001"},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "my-project", "gap-1"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("fix-gap failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	for _, task := range ns.Tasks {
		if task.ID == "task-0002.0001" {
			if task.State != state.StatusComplete {
				t.Errorf("expected complete, got %s", task.State)
			}
			if task.BlockedReason != "" {
				t.Errorf("expected empty blocked reason, got %q", task.BlockedReason)
			}
			return
		}
	}
	t.Fatal("remediation task not found")
}

// ---------------------------------------------------------------------------
// backward compat: gap without RemediationTaskID, fallback to description
// ---------------------------------------------------------------------------

func TestFixGap_FallbackDescriptionScan(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	if err := env.App.State.MutateNode("my-project", func(ns *state.NodeState) error {
		ns.State = state.StatusBlocked
		ns.Tasks = []state.Task{
			{ID: "task-0001", Description: "do stuff", State: state.StatusComplete},
			{ID: "task-0002", Description: "audit", State: state.StatusBlocked, IsAudit: true},
			{ID: "task-0002.0001", Description: "Fix: missing tests\n\nAfter fixing, close the gap:\n  wolfcastle audit fix-gap --node my-project gap-old-1", State: state.StatusInProgress},
		}
		// No RemediationTaskID set (legacy gap).
		ns.Audit.Gaps = []state.Gap{
			{ID: "gap-old-1", Description: "missing tests", Status: state.GapOpen},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "my-project", "gap-old-1"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("fix-gap failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	for _, task := range ns.Tasks {
		if task.ID == "task-0002.0001" {
			if task.State != state.StatusComplete {
				t.Errorf("expected complete via fallback, got %s", task.State)
			}
			return
		}
	}
	t.Fatal("remediation task not found")
}

// ---------------------------------------------------------------------------
// all gaps fixed: node transitions from blocked to in_progress
// ---------------------------------------------------------------------------

func TestFixGap_AllGapsFixed_NodeUnblocked(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	if err := env.App.State.MutateNode("my-project", func(ns *state.NodeState) error {
		ns.State = state.StatusBlocked
		ns.Tasks = []state.Task{
			{ID: "task-0001", Description: "do stuff", State: state.StatusComplete},
			{ID: "task-0002", Description: "audit", State: state.StatusBlocked, IsAudit: true},
			{ID: "task-0002.0001", Description: "Fix: gap one\n\nAfter fixing, close the gap:\n  wolfcastle audit fix-gap --node my-project gap-1", State: state.StatusComplete},
			{ID: "task-0002.0002", Description: "Fix: gap two\n\nAfter fixing, close the gap:\n  wolfcastle audit fix-gap --node my-project gap-2", State: state.StatusInProgress},
		}
		ns.Audit.Gaps = []state.Gap{
			{ID: "gap-1", Description: "gap one", Status: state.GapFixed, RemediationTaskID: "task-0002.0001"},
			{ID: "gap-2", Description: "gap two", Status: state.GapOpen, RemediationTaskID: "task-0002.0002"},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Fix the last remaining gap.
	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "my-project", "gap-2"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("fix-gap failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")

	// Node should no longer be blocked.
	if ns.State == state.StatusBlocked {
		t.Errorf("expected node to be unblocked, got %s", ns.State)
	}
	if ns.State != state.StatusInProgress {
		t.Errorf("expected node in_progress, got %s", ns.State)
	}

	// Audit task should be not_started (all children complete, ready for re-verification).
	for _, task := range ns.Tasks {
		if task.ID == "task-0002" {
			if task.State != state.StatusNotStarted {
				t.Errorf("expected audit task not_started, got %s", task.State)
			}
			break
		}
	}
}

// ---------------------------------------------------------------------------
// partial fix: one gap fixed, one still open; node stays blocked
// ---------------------------------------------------------------------------

func TestFixGap_PartialFix_NodeStaysBlocked(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	if err := env.App.State.MutateNode("my-project", func(ns *state.NodeState) error {
		ns.State = state.StatusBlocked
		ns.Tasks = []state.Task{
			{ID: "task-0001", Description: "do stuff", State: state.StatusComplete},
			{ID: "task-0002", Description: "audit", State: state.StatusBlocked, IsAudit: true},
			{ID: "task-0002.0001", Description: "Fix: gap one", State: state.StatusInProgress},
			{ID: "task-0002.0002", Description: "Fix: gap two", State: state.StatusNotStarted},
		}
		ns.Audit.Gaps = []state.Gap{
			{ID: "gap-1", Description: "gap one", Status: state.GapOpen, RemediationTaskID: "task-0002.0001"},
			{ID: "gap-2", Description: "gap two", Status: state.GapOpen, RemediationTaskID: "task-0002.0002"},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Fix only one gap.
	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "my-project", "gap-1"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("fix-gap failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")

	// Node should remain blocked because gap-2 is still open.
	if ns.State != state.StatusBlocked {
		t.Errorf("expected node to stay blocked, got %s", ns.State)
	}
}

// ---------------------------------------------------------------------------
// index entry is updated after node state change
// ---------------------------------------------------------------------------

func TestFixGap_UpdatesRootIndex(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	// Set node to blocked in both state and index.
	if err := env.App.State.MutateNode("my-project", func(ns *state.NodeState) error {
		ns.State = state.StatusBlocked
		ns.Tasks = []state.Task{
			{ID: "task-0001", Description: "do stuff", State: state.StatusComplete},
			{ID: "task-0002", Description: "audit", State: state.StatusBlocked, IsAudit: true},
			{ID: "task-0002.0001", Description: "Fix: gap\n\nAfter fixing, close the gap:\n  wolfcastle audit fix-gap --node my-project gap-1", State: state.StatusInProgress},
		}
		ns.Audit.Gaps = []state.Gap{
			{ID: "gap-1", Description: "gap", Status: state.GapOpen, RemediationTaskID: "task-0002.0001"},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "my-project", "gap-1"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("fix-gap failed: %v", err)
	}

	// Verify the root index reflects the state change.
	idx, err := env.App.State.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := idx.Nodes["my-project"]
	if !ok {
		t.Fatal("my-project not found in root index")
	}
	if entry.State == state.StatusBlocked {
		t.Errorf("expected index entry to no longer be blocked, got %s", entry.State)
	}
}

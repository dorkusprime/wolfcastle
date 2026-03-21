package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func setupSelfHealEnv(t *testing.T) (*state.StateStore, string) {
	t.Helper()
	tmp := t.TempDir()
	projDir := filepath.Join(tmp, "projects", "test-ns")
	_ = os.MkdirAll(projDir, 0755)

	// Create root index with one leaf node
	idx := state.NewRootIndex()
	idx.Nodes["proj"] = state.IndexEntry{
		Name:    "proj",
		Type:    state.NodeLeaf,
		State:   state.StatusInProgress,
		Address: "proj",
	}
	idx.Root = []string{"proj"}
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), idxData, 0644)

	// Create node state
	nodeDir := filepath.Join(projDir, "proj")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("proj", "proj", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{
		{ID: "task-0001", Title: "Do work", State: state.StatusNotStarted},
		{ID: "audit", Title: "Audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)

	store := state.NewStateStore(projDir, 5)
	return store, projDir
}

func TestSelfHeal_BlockedAuditWithOpenGaps_CreatesSubtasks(t *testing.T) {
	store, _ := setupSelfHealEnv(t)

	_ = store.MutateNode("proj", func(ns *state.NodeState) error {
		for i := range ns.Tasks {
			if ns.Tasks[i].ID == "task-0001" {
				ns.Tasks[i].State = state.StatusComplete
			}
			if ns.Tasks[i].IsAudit {
				ns.Tasks[i].State = state.StatusBlocked
				ns.Tasks[i].BlockedReason = "open gaps"
			}
		}
		ns.Audit.Gaps = []state.Gap{
			{ID: "gap-1", Description: "something wrong", Status: state.GapOpen},
		}
		ns.Audit.Status = state.AuditFailed
		return nil
	})

	d := &Daemon{Store: store}
	if err := d.selfHeal(); err != nil {
		t.Fatalf("selfHeal error: %v", err)
	}

	ns, err := store.ReadNode("proj")
	if err != nil {
		t.Fatalf("reading node: %v", err)
	}

	found := false
	for _, task := range ns.Tasks {
		if task.ID == "audit.0001" {
			found = true
			if task.State != state.StatusNotStarted {
				t.Errorf("subtask state = %s, want not_started", task.State)
			}
		}
	}
	if !found {
		t.Error("expected remediation subtask audit.0001 to be created")
	}

	for _, task := range ns.Tasks {
		if task.IsAudit && task.ID == "audit" {
			if task.State != state.StatusNotStarted {
				t.Errorf("audit state = %s, want not_started", task.State)
			}
			if task.BlockedReason != "" {
				t.Errorf("audit blocked reason = %q, want empty", task.BlockedReason)
			}
		}
	}
}

func TestSelfHeal_BlockedAuditWithExistingSubtasks_Skips(t *testing.T) {
	store, _ := setupSelfHealEnv(t)

	_ = store.MutateNode("proj", func(ns *state.NodeState) error {
		for i := range ns.Tasks {
			if ns.Tasks[i].IsAudit {
				ns.Tasks[i].State = state.StatusBlocked
			}
		}
		ns.Tasks = append(ns.Tasks, state.Task{
			ID:    "audit.0001",
			State: state.StatusNotStarted,
		})
		ns.Audit.Gaps = []state.Gap{
			{ID: "gap-1", Description: "something", Status: state.GapOpen},
		}
		return nil
	})

	d := &Daemon{Store: store}
	if err := d.selfHeal(); err != nil {
		t.Fatalf("selfHeal error: %v", err)
	}

	ns, _ := store.ReadNode("proj")
	subtaskCount := 0
	for _, task := range ns.Tasks {
		if len(task.ID) > 6 && task.ID[:6] == "audit." {
			subtaskCount++
		}
	}
	if subtaskCount != 1 {
		t.Errorf("expected 1 existing subtask, got %d", subtaskCount)
	}
}

func TestSelfHeal_BlockedAuditNoGaps_Skips(t *testing.T) {
	store, _ := setupSelfHealEnv(t)

	_ = store.MutateNode("proj", func(ns *state.NodeState) error {
		for i := range ns.Tasks {
			if ns.Tasks[i].IsAudit {
				ns.Tasks[i].State = state.StatusBlocked
				ns.Tasks[i].BlockedReason = "model said so"
			}
		}
		return nil
	})

	d := &Daemon{Store: store}
	if err := d.selfHeal(); err != nil {
		t.Fatalf("selfHeal error: %v", err)
	}

	ns, _ := store.ReadNode("proj")
	for _, task := range ns.Tasks {
		if task.IsAudit {
			if task.State != state.StatusBlocked {
				t.Errorf("audit should remain blocked without gaps, got %s", task.State)
			}
		}
	}
}

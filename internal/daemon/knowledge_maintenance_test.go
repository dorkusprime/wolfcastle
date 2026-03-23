package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// testDaemonWithKnowledge returns a test daemon with knowledge config and a
// populated knowledge file. Returns the daemon and a function to write
// knowledge content.
func testDaemonWithKnowledge(t *testing.T, maxTokens int) (*Daemon, func(content string)) {
	t.Helper()
	d := testDaemon(t)
	d.Config.Knowledge = config.KnowledgeConfig{MaxTokens: maxTokens}
	d.Config.Identity = &config.IdentityConfig{
		User:    "test",
		Machine: "host",
	}

	knowledgeDir := filepath.Join(d.WolfcastleDir, "docs", "knowledge")
	if err := os.MkdirAll(knowledgeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeKnowledge := func(content string) {
		t.Helper()
		p := filepath.Join(knowledgeDir, "test-host.md")
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return d, writeKnowledge
}

// ═══════════════════════════════════════════════════════════════════════════
// checkKnowledgeBudget
// ═══════════════════════════════════════════════════════════════════════════

func TestCheckKnowledgeBudget_CreatesTaskOnOverflow(t *testing.T) {
	t.Parallel()
	d, writeKnowledge := testDaemonWithKnowledge(t, 10) // very low budget

	// Write content that will exceed 10 tokens.
	writeKnowledge("- This is a knowledge entry with enough words to exceed the budget limit easily and comfortably")

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Some work", State: state.StatusInProgress},
	})

	created := d.checkKnowledgeBudget("my-node")
	if !created {
		t.Fatal("expected maintenance task to be created")
	}

	ns, err := d.Store.ReadNode("my-node")
	if err != nil {
		t.Fatal(err)
	}

	var maint *state.Task
	for i := range ns.Tasks {
		if ns.Tasks[i].TaskType == knowledgeMaintenanceTaskType {
			maint = &ns.Tasks[i]
			break
		}
	}
	if maint == nil {
		t.Fatal("maintenance task not found")
	}
	if maint.Title != knowledgeMaintenanceTitle {
		t.Errorf("expected title %q, got %q", knowledgeMaintenanceTitle, maint.Title)
	}
	if maint.State != state.StatusNotStarted {
		t.Errorf("expected state not_started, got %s", maint.State)
	}
}

func TestCheckKnowledgeBudget_NoDuplicateCreation(t *testing.T) {
	t.Parallel()
	d, writeKnowledge := testDaemonWithKnowledge(t, 10)

	writeKnowledge("- This is a knowledge entry with enough words to exceed the budget limit easily and comfortably")

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Some work", State: state.StatusInProgress},
	})

	// First call creates.
	d.checkKnowledgeBudget("my-node")

	// Second call should not create a duplicate.
	created := d.checkKnowledgeBudget("my-node")
	if created {
		t.Error("should not create duplicate maintenance task")
	}

	ns, err := d.Store.ReadNode("my-node")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, task := range ns.Tasks {
		if task.TaskType == knowledgeMaintenanceTaskType {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 maintenance task, found %d", count)
	}
}

func TestCheckKnowledgeBudget_NoTaskWhenWithinBudget(t *testing.T) {
	t.Parallel()
	d, writeKnowledge := testDaemonWithKnowledge(t, 5000) // generous budget

	writeKnowledge("- A short entry")

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Some work", State: state.StatusInProgress},
	})

	created := d.checkKnowledgeBudget("my-node")
	if created {
		t.Error("should not create maintenance task when within budget")
	}
}

func TestCheckKnowledgeBudget_NoTaskWhenNoKnowledgeFile(t *testing.T) {
	t.Parallel()
	d, _ := testDaemonWithKnowledge(t, 10)
	// Don't write any knowledge file.

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Some work", State: state.StatusInProgress},
	})

	created := d.checkKnowledgeBudget("my-node")
	if created {
		t.Error("should not create maintenance task when knowledge file doesn't exist")
	}
}

func TestCheckKnowledgeBudget_NoTaskWhenNamespaceEmpty(t *testing.T) {
	t.Parallel()
	d, writeKnowledge := testDaemonWithKnowledge(t, 10)
	d.Config.Identity = nil // clear identity

	writeKnowledge("- Many words here to exceed the low budget")

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Some work", State: state.StatusInProgress},
	})

	created := d.checkKnowledgeBudget("my-node")
	if created {
		t.Error("should not create maintenance task when namespace is empty")
	}
}

func TestCheckKnowledgeBudget_InsertsBeforeAudit(t *testing.T) {
	t.Parallel()
	d, writeKnowledge := testDaemonWithKnowledge(t, 10)

	writeKnowledge("- This is a knowledge entry with enough words to exceed the budget limit easily and comfortably")

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Some work", State: state.StatusInProgress},
		{ID: "audit", Description: "Audit", State: state.StatusNotStarted, IsAudit: true},
	})

	d.checkKnowledgeBudget("my-node")

	ns, err := d.Store.ReadNode("my-node")
	if err != nil {
		t.Fatal(err)
	}

	last := ns.Tasks[len(ns.Tasks)-1]
	if !last.IsAudit {
		t.Errorf("audit task should remain last, but got %s", last.ID)
	}

	// Maintenance should be before audit.
	if len(ns.Tasks) < 3 {
		t.Fatalf("expected at least 3 tasks, got %d", len(ns.Tasks))
	}
	secondToLast := ns.Tasks[len(ns.Tasks)-2]
	if secondToLast.TaskType != knowledgeMaintenanceTaskType {
		t.Errorf("maintenance task should be before audit, got %s (type: %s)", secondToLast.ID, secondToLast.TaskType)
	}
}

func TestCheckKnowledgeBudget_MissingNode(t *testing.T) {
	t.Parallel()
	d, writeKnowledge := testDaemonWithKnowledge(t, 10)

	writeKnowledge("- This is a knowledge entry with enough words to exceed the budget limit easily and comfortably")

	created := d.checkKnowledgeBudget("nonexistent")
	if created {
		t.Error("should return false for nonexistent node")
	}
}

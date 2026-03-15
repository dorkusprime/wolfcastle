package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestCheckDeliverables_NoDeliverables(t *testing.T) {
	t.Parallel()
	ns := &state.NodeState{
		Tasks: []state.Task{
			{ID: "task-0001", Description: "no deliverables"},
		},
	}
	missing := checkDeliverables(t.TempDir(), ns, "task-0001")
	if len(missing) != 0 {
		t.Errorf("expected no missing deliverables, got %v", missing)
	}
}

func TestCheckDeliverables_AllExist(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "docs"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "docs/report.md"), []byte("content"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "docs/summary.md"), []byte("more content"), 0644)

	ns := &state.NodeState{
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Description:  "with deliverables",
				Deliverables: []string{"docs/report.md", "docs/summary.md"},
			},
		},
	}
	missing := checkDeliverables(dir, ns, "task-0001")
	if len(missing) != 0 {
		t.Errorf("expected no missing deliverables, got %v", missing)
	}
}

func TestCheckDeliverables_MissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ns := &state.NodeState{
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Description:  "missing file",
				Deliverables: []string{"docs/nonexistent.md"},
			},
		},
	}
	missing := checkDeliverables(dir, ns, "task-0001")
	if len(missing) != 1 || missing[0] != "docs/nonexistent.md" {
		t.Errorf("expected [docs/nonexistent.md], got %v", missing)
	}
}

func TestCheckDeliverables_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "docs"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "docs/empty.md"), []byte(""), 0644)

	ns := &state.NodeState{
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Description:  "empty file",
				Deliverables: []string{"docs/empty.md"},
			},
		},
	}
	missing := checkDeliverables(dir, ns, "task-0001")
	if len(missing) != 1 || missing[0] != "docs/empty.md" {
		t.Errorf("expected [docs/empty.md], got %v", missing)
	}
}

func TestCheckDeliverables_MixedResults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "docs"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "docs/exists.md"), []byte("content"), 0644)

	ns := &state.NodeState{
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Description:  "mixed",
				Deliverables: []string{"docs/exists.md", "docs/missing.md"},
			},
		},
	}
	missing := checkDeliverables(dir, ns, "task-0001")
	if len(missing) != 1 || missing[0] != "docs/missing.md" {
		t.Errorf("expected [docs/missing.md], got %v", missing)
	}
}

func TestCheckDeliverables_TaskNotFound(t *testing.T) {
	t.Parallel()
	ns := &state.NodeState{
		Tasks: []state.Task{
			{ID: "task-0001", Description: "other task"},
		},
	}
	missing := checkDeliverables(t.TempDir(), ns, "task-9999")
	if len(missing) != 0 {
		t.Errorf("expected no missing for nonexistent task, got %v", missing)
	}
}

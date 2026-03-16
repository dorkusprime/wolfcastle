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

// ═══════════════════════════════════════════════════════════════════════════
// Glob pattern deliverables
// ═══════════════════════════════════════════════════════════════════════════

func TestCheckDeliverables_GlobMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "helloworld-2026-01-01.txt"), []byte("content"), 0644)

	ns := &state.NodeState{
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Deliverables: []string{"helloworld-*.txt"},
			},
		},
	}
	missing := checkDeliverables(dir, ns, "task-0001")
	if len(missing) != 0 {
		t.Errorf("glob should match existing file, got missing: %v", missing)
	}
}

func TestCheckDeliverables_GlobNoMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ns := &state.NodeState{
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Deliverables: []string{"helloworld-*.txt"},
			},
		},
	}
	missing := checkDeliverables(dir, ns, "task-0001")
	if len(missing) != 1 {
		t.Errorf("glob with no matches should be missing, got: %v", missing)
	}
}

func TestCheckDeliverables_GlobMatchesEmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "report-v1.md"), []byte(""), 0644)

	ns := &state.NodeState{
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Deliverables: []string{"report-*.md"},
			},
		},
	}
	missing := checkDeliverables(dir, ns, "task-0001")
	if len(missing) != 1 {
		t.Errorf("glob matching only empty files should be missing, got: %v", missing)
	}
}

func TestCheckDeliverables_GlobWithLiteralMix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "output-2026.csv"), []byte("data"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "summary.md"), []byte("summary"), 0644)

	ns := &state.NodeState{
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Deliverables: []string{"output-*.csv", "summary.md"},
			},
		},
	}
	missing := checkDeliverables(dir, ns, "task-0001")
	if len(missing) != 0 {
		t.Errorf("both glob and literal should pass, got missing: %v", missing)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// snapshotDeliverables + checkDeliverablesChanged
// ═══════════════════════════════════════════════════════════════════════════

func TestSnapshotDeliverables_MissingFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hashes := snapshotDeliverables(dir, []string{"nonexistent.txt"})
	if hashes["nonexistent.txt"] != "missing" {
		t.Errorf("expected 'missing', got %q", hashes["nonexistent.txt"])
	}
}

func TestSnapshotDeliverables_ExistingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "output.md"), []byte("hello"), 0644)
	hashes := snapshotDeliverables(dir, []string{"output.md"})
	if hashes["output.md"] == "missing" || hashes["output.md"] == "" {
		t.Error("expected a real hash for existing file")
	}
}

func TestSnapshotDeliverables_GlobPattern(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "report-2026.md"), []byte("data"), 0644)
	hashes := snapshotDeliverables(dir, []string{"report-*.md"})
	if _, ok := hashes["report-2026.md"]; !ok {
		t.Error("glob should expand to matched filename in hash map")
	}
}

func TestSnapshotDeliverables_Empty(t *testing.T) {
	t.Parallel()
	hashes := snapshotDeliverables("/tmp", nil)
	if hashes != nil {
		t.Error("nil deliverables should return nil hashes")
	}
}

func TestCheckDeliverablesChanged_NewFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ns := &state.NodeState{
		Tasks: []state.Task{{
			ID:             "task-0001",
			Deliverables:   []string{"output.txt"},
			BaselineHashes: map[string]string{"output.txt": "missing"},
		}},
	}
	// Create the file after baseline
	_ = os.WriteFile(filepath.Join(dir, "output.txt"), []byte("new content"), 0644)

	if !checkDeliverablesChanged(dir, ns, "task-0001") {
		t.Error("new file should count as changed")
	}
}

func TestCheckDeliverablesChanged_ModifiedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "output.txt"), []byte("original"), 0644)
	baseline := snapshotDeliverables(dir, []string{"output.txt"})

	// Modify the file
	_ = os.WriteFile(filepath.Join(dir, "output.txt"), []byte("modified"), 0644)

	ns := &state.NodeState{
		Tasks: []state.Task{{
			ID:             "task-0001",
			Deliverables:   []string{"output.txt"},
			BaselineHashes: baseline,
		}},
	}
	if !checkDeliverablesChanged(dir, ns, "task-0001") {
		t.Error("modified file should count as changed")
	}
}

func TestCheckDeliverablesChanged_Unchanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "output.txt"), []byte("same"), 0644)
	baseline := snapshotDeliverables(dir, []string{"output.txt"})

	ns := &state.NodeState{
		Tasks: []state.Task{{
			ID:             "task-0001",
			Deliverables:   []string{"output.txt"},
			BaselineHashes: baseline,
		}},
	}
	if checkDeliverablesChanged(dir, ns, "task-0001") {
		t.Error("unchanged file should not count as changed")
	}
}

func TestCheckDeliverablesChanged_NoBaseline(t *testing.T) {
	t.Parallel()
	ns := &state.NodeState{
		Tasks: []state.Task{{
			ID:           "task-0001",
			Deliverables: []string{"output.txt"},
		}},
	}
	// No baseline = always passes (backward compatible)
	if !checkDeliverablesChanged("/tmp", ns, "task-0001") {
		t.Error("no baseline should always pass")
	}
}

func TestCheckDeliverablesChanged_NoDeliverables(t *testing.T) {
	t.Parallel()
	ns := &state.NodeState{
		Tasks: []state.Task{{ID: "task-0001"}},
	}
	if !checkDeliverablesChanged("/tmp", ns, "task-0001") {
		t.Error("no deliverables should always pass")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// checkDeliverablesChanged — glob paths
// ═══════════════════════════════════════════════════════════════════════════

func TestCheckDeliverablesChanged_GlobUnchanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "report-v1.md"), []byte("original"), 0644)
	baseline := snapshotDeliverables(dir, []string{"report-*.md"})

	ns := &state.NodeState{
		Tasks: []state.Task{{
			ID:             "task-0001",
			Deliverables:   []string{"report-*.md"},
			BaselineHashes: baseline,
		}},
	}
	if checkDeliverablesChanged(dir, ns, "task-0001") {
		t.Error("glob files unchanged should return false")
	}
}

func TestCheckDeliverablesChanged_GlobNewFileAppears(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "report-v1.md"), []byte("original"), 0644)
	baseline := snapshotDeliverables(dir, []string{"report-*.md"})

	// A new file appears matching the glob after baseline.
	_ = os.WriteFile(filepath.Join(dir, "report-v2.md"), []byte("new"), 0644)

	ns := &state.NodeState{
		Tasks: []state.Task{{
			ID:             "task-0001",
			Deliverables:   []string{"report-*.md"},
			BaselineHashes: baseline,
		}},
	}
	if !checkDeliverablesChanged(dir, ns, "task-0001") {
		t.Error("new file matching glob should count as changed")
	}
}

func TestCheckDeliverablesChanged_GlobFileModified(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "report-v1.md"), []byte("original"), 0644)
	baseline := snapshotDeliverables(dir, []string{"report-*.md"})

	// Modify the existing file.
	_ = os.WriteFile(filepath.Join(dir, "report-v1.md"), []byte("changed content"), 0644)

	ns := &state.NodeState{
		Tasks: []state.Task{{
			ID:             "task-0001",
			Deliverables:   []string{"report-*.md"},
			BaselineHashes: baseline,
		}},
	}
	if !checkDeliverablesChanged(dir, ns, "task-0001") {
		t.Error("modified glob file should count as changed")
	}
}

func TestCheckDeliverablesChanged_GlobNoMatches(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Baseline captured when no files matched the glob.
	baseline := snapshotDeliverables(dir, []string{"output-*.csv"})

	ns := &state.NodeState{
		Tasks: []state.Task{{
			ID:             "task-0001",
			Deliverables:   []string{"output-*.csv"},
			BaselineHashes: baseline,
		}},
	}
	// Still no matches: no change detected, should return false.
	if checkDeliverablesChanged(dir, ns, "task-0001") {
		t.Error("glob with no matches at baseline and still no matches should return false")
	}
}

func TestCheckDeliverablesChanged_TaskNotFound(t *testing.T) {
	t.Parallel()
	ns := &state.NodeState{
		Tasks: []state.Task{{ID: "task-0001"}},
	}
	// Task not found in list: should return true (don't block).
	if !checkDeliverablesChanged("/tmp", ns, "task-9999") {
		t.Error("task not found should return true")
	}
}

func TestCheckDeliverablesChanged_MixedGlobAndLiteral(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "output-v1.csv"), []byte("data"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "summary.md"), []byte("summary"), 0644)
	baseline := snapshotDeliverables(dir, []string{"output-*.csv", "summary.md"})

	ns := &state.NodeState{
		Tasks: []state.Task{{
			ID:             "task-0001",
			Deliverables:   []string{"output-*.csv", "summary.md"},
			BaselineHashes: baseline,
		}},
	}
	// Nothing changed.
	if checkDeliverablesChanged(dir, ns, "task-0001") {
		t.Error("nothing changed in mixed glob+literal should return false")
	}
}

func TestIsGlob(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path string
		want bool
	}{
		{"hello.txt", false},
		{"docs/report.md", false},
		{"helloworld-*.txt", true},
		{"output-?.csv", true},
		{"data[0-9].json", true},
	}
	for _, tc := range cases {
		if got := isGlob(tc.path); got != tc.want {
			t.Errorf("isGlob(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

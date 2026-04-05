package validate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckOrphanedTempFiles_FindsTempFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a .wolfcastle-tmp- file that would be orphaned.
	orphan := filepath.Join(dir, ".wolfcastle-tmp-123456")
	if err := os.WriteFile(orphan, []byte("leftover"), 0644); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := &Report{}
	engine.checkOrphanedTempFiles(report)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatOrphanedTempFile {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected orphaned temp file issue, found none")
	}
}

func TestCheckOrphanedTempFiles_IgnoresNormalFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.WriteFile(filepath.Join(dir, "state.json"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "definition.md"), []byte("# test"), 0644)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := &Report{}
	engine.checkOrphanedTempFiles(report)

	for _, issue := range report.Issues {
		if issue.Category == CatOrphanedTempFile {
			t.Errorf("unexpected orphaned temp file issue: %s", issue.Description)
		}
	}
}

func TestCheckOrphanedTempFiles_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := &Report{}
	engine.checkOrphanedTempFiles(report)

	if len(report.Issues) != 0 {
		t.Errorf("expected no issues for empty dir, got %d", len(report.Issues))
	}
}

func TestCheckOrphanedTempFiles_NestedTempFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	sub := filepath.Join(dir, "node", "sub")
	_ = os.MkdirAll(sub, 0755)
	orphan := filepath.Join(sub, ".wolfcastle-tmp-abcdef")
	_ = os.WriteFile(orphan, []byte("stale"), 0644)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := &Report{}
	engine.checkOrphanedTempFiles(report)

	if len(report.Issues) != 1 {
		t.Errorf("expected 1 orphaned temp file issue, got %d", len(report.Issues))
	}
}

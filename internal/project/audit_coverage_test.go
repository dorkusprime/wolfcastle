package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAuditTaskMD_WritesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	WriteAuditTaskMD(dir)

	data, err := os.ReadFile(filepath.Join(dir, "audit.md"))
	if err != nil {
		t.Fatalf("audit.md not created: %v", err)
	}
	if len(data) == 0 {
		t.Error("audit.md should not be empty")
	}
}

func TestWriteAuditTaskMD_ReadOnlyDir(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skip in CI")
	}
	t.Parallel()
	dir := t.TempDir()
	_ = os.Chmod(dir, 0555)
	defer func() { _ = os.Chmod(dir, 0755) }()

	// Should not panic on write failure
	WriteAuditTaskMD(dir)
}

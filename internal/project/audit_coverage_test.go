package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

// newAuditPrompts creates a PromptRepository with the audit-task template
// seeded into the base tier.
func newAuditPrompts(t *testing.T) *testutil.Environment {
	t.Helper()
	env := testutil.NewEnvironment(t)
	sub, err := fs.Sub(Templates, "templates")
	if err != nil {
		t.Fatalf("extracting templates sub-FS: %v", err)
	}
	data, err := fs.ReadFile(sub, "artifacts/audit-task.md.tmpl")
	if err != nil {
		t.Fatalf("reading audit-task template: %v", err)
	}
	env.WithTemplate("artifacts/audit-task.md.tmpl", string(data))
	return env
}

func TestWriteAuditTaskMD_WritesFile(t *testing.T) {
	t.Parallel()
	env := newAuditPrompts(t)
	prompts := env.ToAppFields().Prompts
	dir := t.TempDir()

	WriteAuditTaskMD(prompts, dir)

	data, err := os.ReadFile(filepath.Join(dir, "audit.md"))
	if err != nil {
		t.Fatalf("audit.md not created: %v", err)
	}
	if len(data) == 0 {
		t.Error("audit.md should not be empty")
	}
}

func TestWriteAuditTaskMD_ReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	if os.Getenv("CI") != "" {
		t.Skip("skip in CI")
	}
	t.Parallel()
	env := newAuditPrompts(t)
	prompts := env.ToAppFields().Prompts
	dir := t.TempDir()
	_ = os.Chmod(dir, 0555)
	defer func() { _ = os.Chmod(dir, 0755) }()

	// Should not panic on write failure
	WriteAuditTaskMD(prompts, dir)
}

func TestWriteAuditTaskMD_NonexistentDir(t *testing.T) {
	t.Parallel()
	env := newAuditPrompts(t)
	prompts := env.ToAppFields().Prompts
	// Should not panic when directory does not exist
	WriteAuditTaskMD(prompts, "/nonexistent/path/that/does/not/exist")
}

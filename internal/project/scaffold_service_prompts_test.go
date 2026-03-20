package project

import (
	"errors"
	"io/fs"
	"testing"
)

// TestWriteBasePrompts_HappyPath verifies that writeBasePrompts passes
// the embedded templates sub-FS to the prompt writer.
func TestWriteBasePrompts_HappyPath(t *testing.T) {
	t.Parallel()
	svc, pw, _ := newScaffoldService(t)

	err := svc.writeBasePrompts()
	if err != nil {
		t.Fatalf("writeBasePrompts: %v", err)
	}
	if !pw.called {
		t.Error("expected WriteAllBase to be called")
	}
	if pw.templates == nil {
		t.Error("expected non-nil templates FS")
	}

	// Verify the sub-FS doesn't have the "templates/" prefix
	entries, err := fs.ReadDir(pw.templates, ".")
	if err != nil {
		t.Fatalf("reading root of sub-FS: %v", err)
	}
	if len(entries) == 0 {
		t.Error("sub-FS root is empty")
	}
}

// TestWriteBasePrompts_WriterError verifies that errors from the prompt
// writer propagate correctly.
func TestWriteBasePrompts_WriterError(t *testing.T) {
	t.Parallel()
	svc, pw, _ := newScaffoldService(t)
	sentinel := errors.New("writer broke")
	pw.err = sentinel

	err := svc.writeBasePrompts()
	if err == nil {
		t.Fatal("expected error from writeBasePrompts")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel in error chain, got: %v", err)
	}
}

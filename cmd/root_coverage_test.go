package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ---------------------------------------------------------------------------
// Execute coverage notes
// ---------------------------------------------------------------------------
//
// Execute (root.go:114) is a two-line wrapper: call executeRoot(), os.Exit(1)
// on error. Testing it directly would require forking a subprocess to capture
// the exit code, which adds complexity for negligible value. The underlying
// executeRoot() has full branch coverage (success, human-error, JSON-error)
// in root_test.go. Execute's only untested line is the os.Exit call itself.

// ---------------------------------------------------------------------------
// copyDir: MkdirAll failure path
// ---------------------------------------------------------------------------

func TestCopyDir_MkdirAllFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test relies on POSIX path semantics")
	}

	src := filepath.Join(t.TempDir(), "src")
	_ = os.MkdirAll(src, 0755)
	_ = os.WriteFile(filepath.Join(src, "file.txt"), []byte("data"), 0644)

	// Place a regular file where the destination directory needs to be created.
	// MkdirAll cannot create a directory when a path component is a file.
	blocker := filepath.Join(t.TempDir(), "blocker")
	_ = os.WriteFile(blocker, []byte("I am a file"), 0644)
	dst := filepath.Join(blocker, "nested")

	err := copyDir(src, dst)
	if err == nil {
		t.Error("expected error when MkdirAll cannot create destination")
	}
}

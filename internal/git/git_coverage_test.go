package git

import (
	"testing"
)

func TestService_HasProgress_NonRepo(t *testing.T) {
	t.Parallel()

	plainDir := t.TempDir()
	svc := NewService(plainDir)

	// Non-repo should conservatively report progress.
	if !svc.HasProgress("abc123") {
		t.Error("expected HasProgress=true for non-repo directory")
	}
}

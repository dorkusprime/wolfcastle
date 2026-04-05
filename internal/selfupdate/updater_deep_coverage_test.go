package selfupdate

import (
	"fmt"
	"strings"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════
// Apply: exercises the full code path including the unavailable branch
// ═══════════════════════════════════════════════════════════════════════════

func TestStubUpdater_Apply_FullPath(t *testing.T) {
	t.Parallel()
	u := NewUpdater("v1.2.3")

	// Apply calls Check, gets Unavailable=true, returns early
	result, err := u.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentVersion != "v1.2.3" {
		t.Errorf("expected v1.2.3, got %s", result.CurrentVersion)
	}
	if !result.Unavailable {
		t.Error("stub should report unavailable")
	}
	if result.AlreadyCurrent {
		t.Error("stub should not claim already current")
	}
	if result.Updated {
		t.Error("stub should never report updated")
	}
}

func TestStubUpdater_Check_EmptyVersion(t *testing.T) {
	t.Parallel()
	u := NewUpdater("")
	result, err := u.Check()
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentVersion != "" {
		t.Errorf("expected empty version, got %q", result.CurrentVersion)
	}
	if !result.Unavailable {
		t.Error("stub should report unavailable even with empty version")
	}
}

func TestStubUpdater_Apply_EmptyVersion(t *testing.T) {
	t.Parallel()
	u := NewUpdater("")
	result, err := u.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if !result.Unavailable {
		t.Error("empty version should still be unavailable")
	}
	if result.AlreadyCurrent {
		t.Error("stub should not claim already current")
	}
}

func TestStubUpdater_Apply_CheckError(t *testing.T) {
	t.Parallel()
	u := &stubUpdater{
		version: "v1.0.0",
		checkFn: func() (*Result, error) {
			return nil, fmt.Errorf("network timeout")
		},
	}
	_, err := u.Apply()
	if err == nil {
		t.Fatal("expected error from Apply when Check fails")
	}
	if !strings.Contains(err.Error(), "checking for updates") {
		t.Errorf("expected wrapped error, got: %s", err)
	}
	if !strings.Contains(err.Error(), "network timeout") {
		t.Errorf("expected original error in chain, got: %s", err)
	}
}

func TestStubUpdater_Apply_NotCurrentPath(t *testing.T) {
	t.Parallel()
	u := &stubUpdater{
		version: "v1.0.0",
		checkFn: func() (*Result, error) {
			return &Result{
				CurrentVersion: "v1.0.0",
				LatestVersion:  "v2.0.0",
				AlreadyCurrent: false,
			}, nil
		},
	}
	result, err := u.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if result.AlreadyCurrent {
		t.Error("should not be already current")
	}
	if result.Updated {
		t.Error("stub Apply does not perform actual updates yet")
	}
}

func TestStubUpdater_Check_WithOverride(t *testing.T) {
	t.Parallel()
	called := false
	u := &stubUpdater{
		version: "v1.0.0",
		checkFn: func() (*Result, error) {
			called = true
			return &Result{
				CurrentVersion: "v1.0.0",
				LatestVersion:  "v1.0.0",
				AlreadyCurrent: true,
			}, nil
		},
	}
	_, err := u.Check()
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("checkFn override was not called")
	}
}

package selfupdate

import "testing"

// ═══════════════════════════════════════════════════════════════════════════
// Apply — exercises the full code path including the non-update branch
// ═══════════════════════════════════════════════════════════════════════════

func TestStubUpdater_Apply_FullPath(t *testing.T) {
	t.Parallel()
	u := NewUpdater("v1.2.3")

	// Apply calls Check, gets AlreadyCurrent=true, returns early
	result, err := u.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentVersion != "v1.2.3" {
		t.Errorf("expected v1.2.3, got %s", result.CurrentVersion)
	}
	if result.LatestVersion != "v1.2.3" {
		t.Errorf("expected latest v1.2.3, got %s", result.LatestVersion)
	}
	if !result.AlreadyCurrent {
		t.Error("stub should always be already current")
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
}

func TestStubUpdater_Apply_EmptyVersion(t *testing.T) {
	t.Parallel()
	u := NewUpdater("")
	result, err := u.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if !result.AlreadyCurrent {
		t.Error("empty version should still be already current")
	}
}

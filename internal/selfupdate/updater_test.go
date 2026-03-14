package selfupdate

import "testing"

func TestStubUpdater_Check_ReportsAlreadyCurrent(t *testing.T) {
	t.Parallel()
	u := NewUpdater("v0.1.0")
	result, err := u.Check()
	if err != nil {
		t.Fatal(err)
	}
	if !result.AlreadyCurrent {
		t.Error("stub should report already current")
	}
	if result.CurrentVersion != "v0.1.0" {
		t.Errorf("expected current version v0.1.0, got %s", result.CurrentVersion)
	}
	if result.LatestVersion != "v0.1.0" {
		t.Errorf("expected latest version v0.1.0, got %s", result.LatestVersion)
	}
	if result.Updated {
		t.Error("stub should not report updated")
	}
}

func TestStubUpdater_Apply_ReportsAlreadyCurrent(t *testing.T) {
	t.Parallel()
	u := NewUpdater("v0.1.0")
	result, err := u.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if !result.AlreadyCurrent {
		t.Error("stub should report already current")
	}
	if result.Updated {
		t.Error("stub should not report updated")
	}
}

func TestNewUpdater_ReturnsUpdater(t *testing.T) {
	t.Parallel()
	u := NewUpdater("dev")
	if u == nil {
		t.Fatal("NewUpdater should not return nil")
	}
}

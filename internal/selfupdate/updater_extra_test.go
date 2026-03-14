package selfupdate

import (
	"fmt"
	"testing"
)

func TestResult_Fields(t *testing.T) {
	t.Parallel()
	r := &Result{
		CurrentVersion: "v1.0.0",
		LatestVersion:  "v2.0.0",
		Updated:        true,
		AlreadyCurrent: false,
	}
	if r.CurrentVersion != "v1.0.0" {
		t.Errorf("expected CurrentVersion v1.0.0, got %s", r.CurrentVersion)
	}
	if r.LatestVersion != "v2.0.0" {
		t.Errorf("expected LatestVersion v2.0.0, got %s", r.LatestVersion)
	}
	if !r.Updated {
		t.Error("expected Updated to be true")
	}
	if r.AlreadyCurrent {
		t.Error("expected AlreadyCurrent to be false")
	}
}

func TestStubUpdater_Check_VersionsMatch(t *testing.T) {
	t.Parallel()
	u := NewUpdater("dev-build")
	result, err := u.Check()
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentVersion != result.LatestVersion {
		t.Errorf("stub Check should report same current and latest version")
	}
}

func TestStubUpdater_Apply_DelegatesToCheck(t *testing.T) {
	t.Parallel()
	u := NewUpdater("v0.2.0")
	result, err := u.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentVersion != "v0.2.0" {
		t.Errorf("expected version v0.2.0, got %s", result.CurrentVersion)
	}
	if result.LatestVersion != "v0.2.0" {
		t.Errorf("expected latest version v0.2.0, got %s", result.LatestVersion)
	}
}

func TestStubUpdater_EmptyVersion(t *testing.T) {
	t.Parallel()
	u := NewUpdater("")
	result, err := u.Check()
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentVersion != "" {
		t.Errorf("expected empty version, got %q", result.CurrentVersion)
	}
	if !result.AlreadyCurrent {
		t.Error("should be already current even with empty version")
	}
}

// mockUpdater lets us test Apply's non-current codepath.
type mockUpdater struct {
	checkResult *Result
	checkErr    error
}

func (m *mockUpdater) Check() (*Result, error) {
	return m.checkResult, m.checkErr
}

func (m *mockUpdater) Apply() (*Result, error) {
	result, err := m.Check()
	if err != nil {
		return nil, err
	}
	if result.AlreadyCurrent {
		return result, nil
	}
	// Simulate download and replace
	result.Updated = true
	return result, nil
}

func TestUpdaterInterface_Compliance(t *testing.T) {
	t.Parallel()
	// Verify the interface is satisfied by both stub and mock
	var _ Updater = NewUpdater("v1.0.0")
	var _ Updater = &mockUpdater{}
}

func TestMockUpdater_ApplyWhenNotCurrent(t *testing.T) {
	t.Parallel()
	m := &mockUpdater{
		checkResult: &Result{
			CurrentVersion: "v1.0.0",
			LatestVersion:  "v2.0.0",
			AlreadyCurrent: false,
		},
	}
	result, err := m.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if !result.Updated {
		t.Error("expected Updated=true when not current")
	}
}

func TestMockUpdater_ApplyWithCheckError(t *testing.T) {
	t.Parallel()
	m := &mockUpdater{
		checkErr: fmt.Errorf("network failure"),
	}
	_, err := m.Apply()
	if err == nil {
		t.Error("expected error when Check fails")
	}
}

package cmd

import (
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/validate"
)

func TestReportValidationIssues_NoIssues(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	err := reportValidationIssues(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportValidationIssues_WithIssues(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	issues := []validate.Issue{
		{Severity: validate.SeverityError, Category: "test", Description: "error one"},
		{Severity: validate.SeverityWarning, Category: "test", Description: "warning one", Node: "my-node"},
		{Severity: validate.SeverityInfo, Category: "test", Description: "info one", FixType: "deterministic"},
	}

	err := reportValidationIssues(issues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportValidationIssues_JSONOutput(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.JSON = true
	defer func() { app.JSON = false }()

	issues := []validate.Issue{
		{Severity: validate.SeverityError, Category: "test", Description: "err"},
	}
	err := reportValidationIssues(issues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDoctorCmd_NoInit(t *testing.T) {
	// Doctor should fail when no .wolfcastle exists
	// (PersistentPreRunE fails, returning error before doctor runs)
	oldApp := app
	defer func() { app = oldApp }()

	tmp := t.TempDir() // no .wolfcastle in this dir
	t.Chdir(tmp)

	app = &cmdutil.App{}

	rootCmd.SetArgs([]string{"doctor"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when .wolfcastle does not exist")
	}
}

func TestDoctorCmd_WithIdentity(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"doctor"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
}

func TestDoctorCmd_JSONOutput(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.JSON = true
	defer func() { app.JSON = false }()

	rootCmd.SetArgs([]string{"doctor", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doctor --json failed: %v", err)
	}
}

func TestReportValidationIssues_AllSeverities(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	issues := []validate.Issue{
		{Severity: validate.SeverityError, Category: "a", Description: "err", Node: "n1"},
		{Severity: validate.SeverityWarning, Category: "b", Description: "warn"},
		{Severity: validate.SeverityInfo, Category: "c", Description: "info", FixType: "deterministic"},
	}

	err := reportValidationIssues(issues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

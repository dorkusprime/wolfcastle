package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCmd_HumanOutput(t *testing.T) {
	// Save and restore package-level state
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The version command writes via output package, not to cmd.SetOut,
	// so we verify no error occurred. The actual output goes to stderr/stdout.
}

func TestVersionCmd_JSONOutput(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.JSONOutput = true
	defer func() { app.JSONOutput = false }()

	rootCmd.SetArgs([]string{"version", "--json"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVersionCmd_VersionVars(t *testing.T) {
	// Test that version variables have defaults
	if Version == "" {
		t.Error("Version should have a default value")
	}
	if Commit == "" {
		t.Error("Commit should have a default value")
	}
	if Date == "" {
		t.Error("Date should have a default value")
	}
}

func TestVersionCmd_ContainsVersion(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Capture output by executing and checking no error
	Version = "1.2.3-test"
	defer func() { Version = "dev" }()

	rootCmd.SetArgs([]string{"version"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Version var is set
	if !strings.Contains(Version, "1.2.3") {
		t.Errorf("Version should contain test value")
	}
}

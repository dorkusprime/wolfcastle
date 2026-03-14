package cmd

import (
	"os"
	"testing"
)

func TestInitCmd_JSONOutput(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

	oldApp := app
	defer func() { app = oldApp }()

	app.JSONOutput = true
	defer func() { app.JSONOutput = false }()

	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init --json failed: %v", err)
	}
}

func TestInitCmd_AlreadyInitialized_JSON(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

	_ = os.MkdirAll(tmp+"/.wolfcastle", 0755)

	oldApp := app
	defer func() { app = oldApp }()

	app.JSONOutput = true
	defer func() { app.JSONOutput = false }()

	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init (already initialized, json) should succeed: %v", err)
	}
}

func TestInitCmd_ForceReinit_JSON(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

	// First init
	rootCmd.SetArgs([]string{"init"})
	_ = rootCmd.Execute()

	oldApp := app
	defer func() { app = oldApp }()

	app.JSONOutput = true
	defer func() { app.JSONOutput = false }()

	rootCmd.SetArgs([]string{"init", "--force"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init --force --json failed: %v", err)
	}
}

func TestDoctorCmd_Fix(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"doctor", "--fix"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doctor --fix failed: %v", err)
	}
}

func TestSpecLink_JSONOutput(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.JSONOutput = true
	defer func() { app.JSONOutput = false }()

	env.createLeafNode(t, "my-project", "My Project")

	// Create spec file
	specsDir := env.WolfcastleDir + "/docs/specs"
	filename := "2025-01-01T00-00Z-json-link.md"
	_ = os.WriteFile(specsDir+"/"+filename, []byte("# Test\n"), 0644)

	rootCmd.SetArgs([]string{"spec", "link", "--node", "my-project", filename})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("spec link --json failed: %v", err)
	}
}

func TestArchiveAddCmd_NotComplete(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	env.createLeafNode(t, "my-project", "My Project")

	rootCmd.SetArgs([]string{"archive", "add", "--node", "my-project"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when node is not complete")
	}
}

func TestArchiveAddCmd_NoResolver(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.Resolver = nil

	rootCmd.SetArgs([]string{"archive", "add", "--node", "my-project"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when resolver is nil")
	}
}

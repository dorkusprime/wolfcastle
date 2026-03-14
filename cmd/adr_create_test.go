package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestADRCreate_Basic(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"adr", "create", "Use JWT for authentication"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("adr create failed: %v", err)
	}

	decisionsDir := filepath.Join(env.WolfcastleDir, "docs", "decisions")
	entries, err := os.ReadDir(decisionsDir)
	if err != nil {
		t.Fatalf("reading decisions dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no ADR file created")
	}

	name := entries[0].Name()
	if !strings.HasSuffix(name, "-use-jwt-for-authentication.md") {
		t.Errorf("unexpected filename: %s", name)
	}

	// Verify content has template sections
	data, _ := os.ReadFile(filepath.Join(decisionsDir, name))
	content := string(data)
	if !strings.Contains(content, "## Status") {
		t.Error("ADR should contain Status section")
	}
	if !strings.Contains(content, "## Context") {
		t.Error("ADR should contain Context section")
	}
	if !strings.Contains(content, "## Decision") {
		t.Error("ADR should contain Decision section")
	}
	if !strings.Contains(content, "## Consequences") {
		t.Error("ADR should contain Consequences section")
	}
}

func TestADRCreate_EmptyTitle(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"adr", "create", "   "})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for empty title")
	}
}

func TestADRCreate_WithFile(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Create body file
	bodyFile := filepath.Join(env.RootDir, "body.md")
	_ = os.WriteFile(bodyFile, []byte("## Custom Context\nThis is the context.\n"), 0644)

	rootCmd.SetArgs([]string{"adr", "create", "--file", bodyFile, "Custom ADR"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("adr create with --file failed: %v", err)
	}

	decisionsDir := filepath.Join(env.WolfcastleDir, "docs", "decisions")
	entries, _ := os.ReadDir(decisionsDir)
	data, _ := os.ReadFile(filepath.Join(decisionsDir, entries[0].Name()))
	if !strings.Contains(string(data), "Custom Context") {
		t.Error("ADR should contain body from file")
	}
}

func TestADRCreate_JSONOutput(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.JSONOutput = true
	defer func() { app.JSONOutput = false }()

	// Explicitly clear --file to avoid flag pollution from other tests
	rootCmd.SetArgs([]string{"adr", "create", "--file", "", "JSON ADR"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("adr create --json failed: %v", err)
	}
}

func TestADRCreate_StdinAndFileMutuallyExclusive(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"adr", "create", "--stdin", "--file", "foo.md", "Title"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when both --stdin and --file provided")
	}
}

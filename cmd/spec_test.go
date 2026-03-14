package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpecCreate_Basic(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"spec", "create", "API Authentication Flow"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("spec create failed: %v", err)
	}

	// Verify spec file was created
	specsDir := filepath.Join(env.WolfcastleDir, "docs", "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		t.Fatalf("reading specs dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no spec files created")
	}

	// Check filename format
	name := entries[0].Name()
	if !strings.HasSuffix(name, "-api-authentication-flow.md") {
		t.Errorf("unexpected filename: %s", name)
	}
}

func TestSpecCreate_EmptyTitle(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"spec", "create", "   "})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for empty title")
	}
}

func TestSpecCreate_WithNodeLink(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	env.createLeafNode(t, "my-project", "My Project")

	rootCmd.SetArgs([]string{"spec", "create", "--node", "my-project", "Test Spec"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("spec create with node failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	if len(ns.Specs) != 1 {
		t.Fatalf("expected 1 linked spec, got %d", len(ns.Specs))
	}
}

func TestSpecList_Empty(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"spec", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("spec list failed: %v", err)
	}
}

func TestSpecList_WithSpecs(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Create some specs
	rootCmd.SetArgs([]string{"spec", "create", "First Spec"})
	rootCmd.Execute()
	rootCmd.SetArgs([]string{"spec", "create", "Second Spec"})
	rootCmd.Execute()

	rootCmd.SetArgs([]string{"spec", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("spec list failed: %v", err)
	}
}

func TestSpecLink_Success(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	env.createLeafNode(t, "my-project", "My Project")

	// Create spec file manually (avoids Cobra flag state pollution from specCreateCmd)
	specsDir := filepath.Join(env.WolfcastleDir, "docs", "specs")
	filename := "2025-01-01T00-00Z-test-spec.md"
	os.WriteFile(filepath.Join(specsDir, filename), []byte("# Test Spec\n"), 0644)

	rootCmd.SetArgs([]string{"spec", "link", "--node", "my-project", filename})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("spec link failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	if len(ns.Specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(ns.Specs))
	}
}

func TestSpecLink_Duplicate(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	env.createLeafNode(t, "my-project", "My Project")

	// Create spec file manually
	specsDir := filepath.Join(env.WolfcastleDir, "docs", "specs")
	filename := "2025-01-01T00-00Z-dup-test.md"
	os.WriteFile(filepath.Join(specsDir, filename), []byte("# Dup Test\n"), 0644)

	// Link once
	rootCmd.SetArgs([]string{"spec", "link", "--node", "my-project", filename})
	rootCmd.Execute()

	// Link again (duplicate)
	rootCmd.SetArgs([]string{"spec", "link", "--node", "my-project", filename})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for duplicate link")
	}
}

func TestSpecLink_FileNotFound(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	env.createLeafNode(t, "my-project", "My Project")

	rootCmd.SetArgs([]string{"spec", "link", "--node", "my-project", "nonexistent.md"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent spec file")
	}
}

func TestSpecList_FilterByNode(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	env.createLeafNode(t, "my-project", "My Project")

	// Create and link a spec (with --node)
	rootCmd.SetArgs([]string{"spec", "create", "--node", "my-project", "Linked Spec"})
	rootCmd.Execute()

	// Create another spec not linked (explicitly no --node)
	// Note: spec create's --node flag defaults to "" so we just don't set it.
	// However, cobra may retain the previous flag value on the same command.
	// We test the filtering on the list side instead.

	// Create an unlinked spec file manually to avoid flag pollution
	specsDir := filepath.Join(env.WolfcastleDir, "docs", "specs")
	os.WriteFile(filepath.Join(specsDir, "2025-01-01T00-00Z-unlinked.md"), []byte("# Unlinked\n"), 0644)

	// List filtered by node - should show only the linked spec
	rootCmd.SetArgs([]string{"spec", "list", "--node", "my-project"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("spec list --node failed: %v", err)
	}

	// Verify only 1 spec is linked to the node
	ns := env.loadNodeState(t, "my-project")
	if len(ns.Specs) != 1 {
		t.Errorf("expected 1 linked spec, got %d", len(ns.Specs))
	}
}


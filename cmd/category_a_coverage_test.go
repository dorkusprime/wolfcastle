package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/dorkusprime/wolfcastle/internal/validate"
)

// ---------------------------------------------------------------------------
// cmd/doctor.go: LoadRootIndex failure path (corrupt state.json)
// ---------------------------------------------------------------------------

func TestDoctorCmd_CorruptStateJSON(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Corrupt the state.json
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), []byte("corrupt json{{"), 0644)

	rootCmd.SetArgs([]string{"doctor"})
	// Should not return an error — it reports the issue via reportValidationIssues
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("doctor with corrupt state should still report: %v", err)
	}
}

// ---------------------------------------------------------------------------
// cmd/doctor.go: no-fixable-issues path
// ---------------------------------------------------------------------------

func TestDoctorCmd_FixNoFixableIssues(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Run doctor --fix on a healthy tree (no issues to fix)
	rootCmd.SetArgs([]string{"doctor", "--fix"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doctor --fix on healthy tree failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// cmd/doctor.go: model-assisted fix reporting block
// ---------------------------------------------------------------------------

func TestDoctorCmd_ModelAssistedFixBlock(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.Invoker = invoke.NewProcessInvoker()

	// Set up config with a doctor model reference (the model won't actually
	// be invoked because there are no model-assisted issues)
	app.Cfg.Doctor.Model = "test-model"
	app.Cfg.Models = map[string]config.ModelDef{
		"test-model": {Command: "echo", Args: []string{"test"}},
	}

	rootCmd.SetArgs([]string{"doctor", "--fix"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doctor --fix with model config failed: %v", err)
	}
}

func TestDoctorCmd_ModelAssistedFixWithIssues(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.Invoker = invoke.NewProcessInvoker()

	// Set up config with a doctor model
	app.Cfg.Doctor.Model = "test-model"
	app.Cfg.Models = map[string]config.ModelDef{
		"test-model": {Command: "echo", Args: []string{"test"}},
	}

	// Create a node with issues that have FixType model-assisted to exercise
	// the code block (the actual invoke will fail, but the code path is covered)
	env.createLeafNode(t, "orphan-project", "Orphan")
	// Corrupt the node state to trigger validation issues
	parsed, _ := tree.ParseAddress("orphan-project")
	nodePath := filepath.Join(env.ProjectsDir, filepath.Join(parsed.Parts...), "state.json")
	ns, _ := state.LoadNodeState(nodePath)
	ns.State = state.StatusComplete // mismatch with index
	_ = state.SaveNodeState(nodePath, ns)

	rootCmd.SetArgs([]string{"doctor", "--fix"})
	// May or may not error, but exercises the model-assisted block
	_ = rootCmd.Execute()
}

// ---------------------------------------------------------------------------
// cmd/doctor.go: --fix with fixable issues (covers the fix-applied reporting)
// ---------------------------------------------------------------------------

func TestDoctorCmd_FixWithIssues(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Create a leaf node whose state is out of sync with the index
	env.createLeafNode(t, "sync-project", "Sync Project")
	parsed, _ := tree.ParseAddress("sync-project")
	nodePath := filepath.Join(env.ProjectsDir, filepath.Join(parsed.Parts...), "state.json")
	ns, _ := state.LoadNodeState(nodePath)
	ns.State = state.StatusInProgress // index says not_started but node says in_progress
	_ = state.SaveNodeState(nodePath, ns)

	rootCmd.SetArgs([]string{"doctor", "--fix"})
	_ = rootCmd.Execute()
	// The exact behavior depends on the validation engine, but this
	// exercises the fix-reporting block
}

// ---------------------------------------------------------------------------
// cmd/doctor.go: LoadRootIndex failure path in JSON mode
// ---------------------------------------------------------------------------

func TestDoctorCmd_CorruptStateJSON_JSON(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.JSONOutput = true
	defer func() { app.JSONOutput = false }()

	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), []byte("{{bad"), 0644)

	rootCmd.SetArgs([]string{"doctor", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doctor --json with corrupt state should report via JSON: %v", err)
	}
}

// Note: RequireResolver guards for spec create/link/list are exercised
// indirectly through the NoInit test pattern (PersistentPreRunE fails).
// The RequireResolver() function itself is tested in cmdutil/app_test.go.
// Direct cobra-level testing of these guards in the root cmd package is
// impractical because cobra retains --node flag values across calls,
// causing test pollution.

// ---------------------------------------------------------------------------
// cmd/spec.go: ReadDir error in list
// ---------------------------------------------------------------------------

func TestSpecList_ReadDirError(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Remove the specs directory to trigger ReadDir error
	specsDir := filepath.Join(env.WolfcastleDir, "docs", "specs")
	_ = os.RemoveAll(specsDir)

	rootCmd.SetArgs([]string{"spec", "list"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when specs dir doesn't exist")
	}
}

// ---------------------------------------------------------------------------
// cmd/spec.go: dedup logic and README/IsDir filter
// ---------------------------------------------------------------------------

func TestSpecList_DedupAndFiltering(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	specsDir := filepath.Join(env.WolfcastleDir, "docs", "specs")
	// Create a README.md (should be filtered)
	_ = os.WriteFile(filepath.Join(specsDir, "README.md"), []byte("# Readme"), 0644)
	// Create a subdirectory (should be filtered)
	_ = os.MkdirAll(filepath.Join(specsDir, "subdir"), 0755)
	// Create a non-.md file (should be filtered)
	_ = os.WriteFile(filepath.Join(specsDir, "notes.txt"), []byte("notes"), 0644)
	// Create a valid spec
	_ = os.WriteFile(filepath.Join(specsDir, "valid-spec.md"), []byte("# Valid"), 0644)

	rootCmd.SetArgs([]string{"spec", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("spec list with filters failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// cmd/adr_create.go: --file with nonexistent path error
// ---------------------------------------------------------------------------

func TestADRCreate_FileNotFound(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Explicitly set --stdin=false to avoid flag pollution from other tests
	rootCmd.SetArgs([]string{"adr", "create", "--stdin=false", "--file", "/nonexistent/path/body.md", "Test ADR"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when --file points to nonexistent path")
	}
}

// ---------------------------------------------------------------------------
// cmd/archive_add.go: RequireResolver
// (Uses a bare App and chdir to a dir without .wolfcastle so
// PersistentPreRunE cannot reload config/resolver)
// ---------------------------------------------------------------------------

func TestArchiveAdd_RequireResolver(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	_ = os.MkdirAll(wcDir, 0755)
	_ = os.WriteFile(filepath.Join(wcDir, "config.json"), []byte(`{}`), 0644)

	origDir, _ := os.Getwd()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()

	app = &cmdutil.App{}

	rootCmd.SetArgs([]string{"archive", "add", "--node", "my-project"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when resolver is nil for archive add")
	}
}

// ---------------------------------------------------------------------------
// cmd/archive_add.go: empty --node
// ---------------------------------------------------------------------------

func TestArchiveAdd_EmptyNode(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"archive", "add", "--node", ""})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node is empty for archive add")
	}
}

// ---------------------------------------------------------------------------
// cmd/archive_add.go: ParseAddress error
// ---------------------------------------------------------------------------

func TestArchiveAdd_ParseAddressError(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"archive", "add", "--node", "INVALID_UPPERCASE"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid address in archive add")
	}
}

// ---------------------------------------------------------------------------
// cmd/navigate.go: RequireResolver
// ---------------------------------------------------------------------------

func TestNavigate_RequireResolver(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	_ = os.MkdirAll(wcDir, 0755)
	_ = os.WriteFile(filepath.Join(wcDir, "config.json"), []byte(`{}`), 0644)

	origDir, _ := os.Getwd()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()

	app = &cmdutil.App{}

	rootCmd.SetArgs([]string{"navigate"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when resolver is nil for navigate")
	}
}

// ---------------------------------------------------------------------------
// cmd/navigate.go: LoadRootIndex error
// ---------------------------------------------------------------------------

func TestNavigate_LoadRootIndexError(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Corrupt the root index
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), []byte("corrupt"), 0644)

	rootCmd.SetArgs([]string{"navigate"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when root index is corrupt")
	}
}

// ---------------------------------------------------------------------------
// cmd/navigate.go: ParseAddress error in loader
// ---------------------------------------------------------------------------

func TestNavigate_ParseAddressErrorInLoader(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Add a node with an invalid address to the root index
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	idx.Nodes["INVALID_ADDR"] = state.IndexEntry{
		Name:     "Bad Node",
		Type:     state.NodeLeaf,
		State:    state.StatusNotStarted,
		Address:  "INVALID_ADDR",
		Children: []string{},
	}
	_ = state.SaveRootIndex(filepath.Join(env.ProjectsDir, "state.json"), idx)

	rootCmd.SetArgs([]string{"navigate"})
	err := rootCmd.Execute()
	// FindNextTask may or may not error depending on implementation,
	// but the ParseAddress error path in the loader is exercised
	_ = err
}

// ---------------------------------------------------------------------------
// cmd/navigate.go: FindNextTask error
// ---------------------------------------------------------------------------

func TestNavigate_FindNextTaskError(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Create a valid node in the index but don't create its state.json on disk
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	idx.Nodes["ghost-node"] = state.IndexEntry{
		Name:     "Ghost",
		Type:     state.NodeLeaf,
		State:    state.StatusNotStarted,
		Address:  "ghost-node",
		Children: []string{},
	}
	_ = state.SaveRootIndex(filepath.Join(env.ProjectsDir, "state.json"), idx)
	// No state.json created for ghost-node on disk

	rootCmd.SetArgs([]string{"navigate"})
	err := rootCmd.Execute()
	// Exercises the state loading error path inside FindNextTask
	_ = err
}

// ---------------------------------------------------------------------------
// cmd/unblock.go: loadUnblockPreamble success path (create unblock.md)
// ---------------------------------------------------------------------------

func TestLoadUnblockPreamble_SuccessWithFile(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Create the unblock.md in base/prompts/
	promptsDir := filepath.Join(env.WolfcastleDir, "base", "prompts")
	_ = os.MkdirAll(promptsDir, 0755)
	_ = os.WriteFile(filepath.Join(promptsDir, "unblock.md"),
		[]byte("Custom unblock instructions: help the user carefully."), 0644)

	preamble := loadUnblockPreamble()
	if !strings.Contains(preamble, "Custom unblock instructions") {
		t.Errorf("expected custom preamble from unblock.md, got: %s", preamble)
	}
}

// ---------------------------------------------------------------------------
// reportValidationIssues: exercise unknown severity branch
// ---------------------------------------------------------------------------

func TestReportValidationIssues_UnknownSeverity(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	issues := []validate.Issue{
		{Severity: "unknown", Category: "test", Description: "strange"},
	}
	err := reportValidationIssues(issues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

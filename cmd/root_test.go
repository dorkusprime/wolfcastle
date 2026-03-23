package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// nodeStatePath constructs the path to a node's state.json for test setup.
func nodeStatePath(projectsDir string, parsed tree.Address) string {
	return filepath.Join(projectsDir, filepath.Join(parsed.Parts...), "state.json")
}

func TestInitCmd_JSONOutput(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	oldApp := app
	defer func() { app = oldApp }()

	app.JSON = true
	defer func() { app.JSON = false }()

	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init --json failed: %v", err)
	}
}

func TestInitCmd_AlreadyInitialized_JSON(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	_ = os.MkdirAll(tmp+"/.wolfcastle", 0755)

	oldApp := app
	defer func() { app = oldApp }()

	app.JSON = true
	defer func() { app.JSON = false }()

	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init (already initialized, json) should succeed: %v", err)
	}
}

func TestInitCmd_ForceReinit_JSON(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	// First init
	rootCmd.SetArgs([]string{"init"})
	_ = rootCmd.Execute()

	oldApp := app
	defer func() { app = oldApp }()

	app.JSON = true
	defer func() { app.JSON = false }()

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
	app.JSON = true
	defer func() { app.JSON = false }()

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

func TestArchiveAddCmd_NoIdentity(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.Identity = nil

	rootCmd.SetArgs([]string{"archive", "add", "--node", "my-project"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when identity is nil")
	}
}

// ---------------------------------------------------------------------------
// Execute() error formatting paths
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// setupCommands and executeRoot
// ---------------------------------------------------------------------------

func TestSetupCommands(t *testing.T) {
	// setupCommands is idempotent — calling it again should not panic
	setupCommands()

	// Verify groups are assigned
	if initCmd.GroupID != "lifecycle" {
		t.Error("initCmd should be in lifecycle group")
	}
	if navigateCmd.GroupID != "work" {
		t.Error("navigateCmd should be in work group")
	}
	if doctorCmd.GroupID != "diagnostics" {
		t.Error("doctorCmd should be in diagnostics group")
	}
	if specCmd.GroupID != "docs" {
		t.Error("specCmd should be in docs group")
	}
	if installCmd.GroupID != "integration" {
		t.Error("installCmd should be in integration group")
	}
}

func TestExecuteRoot_Success(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"version"})
	err := executeRoot()
	if err != nil {
		t.Fatalf("executeRoot with version should succeed: %v", err)
	}
}

func TestExecuteRoot_Error_HumanOutput(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.JSON = false

	rootCmd.SetArgs([]string{"nonexistent-cmd-xyz"})
	err := executeRoot()
	if err == nil {
		t.Error("executeRoot with bad command should fail")
	}
}

func TestExecuteRoot_Error_JSONOutput(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.JSON = true
	defer func() { app.JSON = false }()

	rootCmd.SetArgs([]string{"nonexistent-cmd-xyz"})
	err := executeRoot()
	if err == nil {
		t.Error("executeRoot with bad command should fail in JSON mode")
	}
}

func TestRootCmd_UnknownCommand(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"nonexistent-command-xyz"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestRootCmd_UnknownCommand_JSON(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.JSON = true
	defer func() { app.JSON = false }()

	rootCmd.SetArgs([]string{"nonexistent-command-xyz"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for unknown command in JSON mode")
	}
}

// ---------------------------------------------------------------------------
// runInteractiveUnblock error paths
// ---------------------------------------------------------------------------

func TestRunInteractiveUnblock_ModelNotFound(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	_ = app.Config.WriteCustom(map[string]any{
		"unblock": map[string]any{"model": "nonexistent-model"},
	})

	err := runInteractiveUnblock(t.Context(), "my-project/task-0001", "diagnostic text")
	if err == nil {
		t.Error("expected error when model not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention model not found, got: %v", err)
	}
}

// TestRunInteractiveUnblock_InvocationFails was removed: readline requires
// a terminal and the shared app state triggers race detector warnings under
// `go test -race`. The unblock diagnostic builder is tested separately.

// ---------------------------------------------------------------------------
// loadUnblockPreamble with template resolution failure
// ---------------------------------------------------------------------------

func TestLoadUnblockPreamble_TemplateNotFound(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	// Point to a wolfcastle dir that exists but has no unblock.md template
	env := newTestEnv(t)
	app = env.App

	preamble := loadUnblockPreamble()
	if preamble == "" {
		t.Error("expected non-empty preamble (either template or fallback)")
	}
}

// ---------------------------------------------------------------------------
// unblock command - invalid node address format
// ---------------------------------------------------------------------------

func TestUnblockCmd_InvalidNodeAddress(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Node flag with no slash (invalid task address)
	rootCmd.SetArgs([]string{"unblock", "--node", "just-a-project"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for node without task component")
	}
}

func TestUnblockCmd_NonexistentNode(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"unblock", "--node", "nonexistent-project/task-0001"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

// ---------------------------------------------------------------------------
// install skill command - JSON output
// ---------------------------------------------------------------------------

func TestInstallSkillCmd_JSONOutput(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.JSON = true
	defer func() { app.JSON = false }()

	sourceDir := filepath.Join(env.WolfcastleDir, "system", "base", "skills")
	_ = os.MkdirAll(sourceDir, 0755)
	_ = os.WriteFile(filepath.Join(sourceDir, "wolfcastle.md"), []byte("# Skill\n"), 0644)

	rootCmd.SetArgs([]string{"install", "skill"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("install skill --json failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// copyDir error paths
// ---------------------------------------------------------------------------

func TestCopyDir_UnreadableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-based test not supported on Windows")
	}
	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	_ = os.MkdirAll(src, 0755)

	// Create a file then remove read permission
	filePath := filepath.Join(src, "secret.txt")
	_ = os.WriteFile(filePath, []byte("data"), 0000)

	err := copyDir(src, dst)
	if err == nil {
		t.Error("expected error when file is unreadable")
	}

	// Restore permissions for cleanup
	_ = os.Chmod(filePath, 0644)
}

// ---------------------------------------------------------------------------
// spec list - JSON output with node filter
// ---------------------------------------------------------------------------

func TestSpecList_JSONWithNodeFilter(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.JSON = true
	defer func() { app.JSON = false }()
	env.createLeafNode(t, "my-project", "My Project")

	rootCmd.SetArgs([]string{"spec", "list", "--node", "my-project"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("spec list --json --node failed: %v", err)
	}

	// Reset the --node flag to avoid polluting subsequent spec list tests
	rootCmd.SetArgs([]string{"spec", "list", "--node", ""})
	_ = rootCmd.Execute()
}

// ---------------------------------------------------------------------------
// spec create - JSON output with node link
// ---------------------------------------------------------------------------

func TestSpecLink_JSONWithNode(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.JSON = true
	defer func() { app.JSON = false }()
	env.createLeafNode(t, "my-project", "My Project")

	// Create a spec file manually
	specsDir := filepath.Join(env.WolfcastleDir, "docs", "specs")
	filename := "2025-01-01T00-00Z-json-link-test.md"
	_ = os.WriteFile(filepath.Join(specsDir, filename), []byte("# Test\n"), 0644)

	rootCmd.SetArgs([]string{"spec", "link", "--node", "my-project", filename})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("spec link --json with node failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// navigate - not-found reason path
// ---------------------------------------------------------------------------

func TestNavigate_NotFoundReason(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Empty tree => "no actionable tasks" path
	rootCmd.SetArgs([]string{"navigate"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// doctor - with validation issues and fix mode
// ---------------------------------------------------------------------------

func TestDoctorCmd_FixMode_NoIssues(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"doctor", "--fix"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doctor --fix (no issues) failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// archive add - JSON output path
// ---------------------------------------------------------------------------

func TestArchiveAddCmd_JSONNotComplete(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.JSON = true
	defer func() { app.JSON = false }()
	env.createLeafNode(t, "my-project", "My Project")

	rootCmd.SetArgs([]string{"archive", "add", "--node", "my-project"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when node is not complete (JSON mode)")
	}
}

// ---------------------------------------------------------------------------
// init command - force reinit JSON output
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// archive add - success path (complete node)
// ---------------------------------------------------------------------------

func TestArchiveAddCmd_SuccessComplete(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	env.createLeafNode(t, "done-project", "Done Project")

	// Mark node as complete
	parsed, _ := tree.ParseAddress("done-project")
	ns := env.loadNodeState(t, "done-project")
	ns.State = state.StatusComplete
	_ = state.SaveNodeState(nodeStatePath(env.ProjectsDir, parsed), ns)

	rootCmd.SetArgs([]string{"archive", "add", "--node", "done-project"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("archive add for complete node failed: %v", err)
	}

	// Verify archive file was created
	archiveDir := filepath.Join(env.WolfcastleDir, "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatalf("reading archive dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("no archive file created")
	}
}

func TestArchiveAddCmd_SuccessComplete_JSON(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.JSON = true
	defer func() { app.JSON = false }()
	env.createLeafNode(t, "done-json", "Done JSON")

	parsed, _ := tree.ParseAddress("done-json")
	ns := env.loadNodeState(t, "done-json")
	ns.State = state.StatusComplete
	_ = state.SaveNodeState(nodeStatePath(env.ProjectsDir, parsed), ns)

	rootCmd.SetArgs([]string{"archive", "add", "--node", "done-json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("archive add --json for complete node failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// doctor - fix mode with various paths
// ---------------------------------------------------------------------------

func TestDoctorCmd_Fix_JSONOutput(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.JSON = true
	defer func() { app.JSON = false }()

	rootCmd.SetArgs([]string{"doctor", "--fix"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doctor --fix --json failed: %v", err)
	}
}

func TestDoctorCmd_Fix_WithNodes(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	env.createLeafNode(t, "fix-project", "Fix Project")

	rootCmd.SetArgs([]string{"doctor", "--fix"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doctor --fix with nodes failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// update command - covers the error/already-current paths
// ---------------------------------------------------------------------------

func TestUpdateCmd_Execution(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	rootCmd.SetArgs([]string{"update"})
	// Update will likely fail (no internet or wrong version), but exercises paths
	_ = rootCmd.Execute()
}

func TestUpdateCmd_JSONOutput(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.JSON = true
	defer func() { app.JSON = false }()

	rootCmd.SetArgs([]string{"update"})
	_ = rootCmd.Execute()
}

// ---------------------------------------------------------------------------
// archive add - success with git available
// ---------------------------------------------------------------------------

func TestArchiveAddCmd_SuccessWithGitBranch(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Init git repo so branch detection works
	repoDir := filepath.Dir(env.WolfcastleDir)
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	_ = cmd.Run()
	cmd = exec.Command("git", "checkout", "-b", "test-branch")
	cmd.Dir = repoDir
	_ = cmd.Run()

	env.createLeafNode(t, "git-project", "Git Project")
	parsed, _ := tree.ParseAddress("git-project")
	ns := env.loadNodeState(t, "git-project")
	ns.State = state.StatusComplete
	_ = state.SaveNodeState(nodeStatePath(env.ProjectsDir, parsed), ns)

	rootCmd.SetArgs([]string{"archive", "add", "--node", "git-project"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("archive add with git failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// doctor - fix mode that actually finds and fixes issues
// ---------------------------------------------------------------------------

func TestDoctorCmd_Fix_WithFixableIssues(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	env.createLeafNode(t, "broken-node", "Broken Node")

	// Remove the audit task to create a fixable validation issue
	parsed, _ := tree.ParseAddress("broken-node")
	ns := env.loadNodeState(t, "broken-node")
	ns.Tasks = nil // Remove all tasks including audit
	_ = state.SaveNodeState(nodeStatePath(env.ProjectsDir, parsed), ns)

	rootCmd.SetArgs([]string{"doctor", "--fix"})
	// Should not error — it reports and fixes
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doctor --fix with fixable issues failed: %v", err)
	}
}

func TestInitCmd_ForceReinit_HumanOutput(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	oldApp := app
	defer func() { app = oldApp }()

	app.JSON = false

	rootCmd.SetArgs([]string{"init"})
	_ = rootCmd.Execute()

	rootCmd.SetArgs([]string{"init", "--force"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init --force (human) failed: %v", err)
	}
}

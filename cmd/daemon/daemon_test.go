package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

type testEnv struct {
	WolfcastleDir string
	ProjectsDir   string
	App           *cmdutil.App
	RootCmd       *cobra.Command
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	os.MkdirAll(wcDir, 0755)

	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "test", Machine: "dev"}

	ns := "test-dev"
	projDir := filepath.Join(wcDir, "projects", ns)
	os.MkdirAll(projDir, 0755)

	idx := state.NewRootIndex()
	data, _ := json.MarshalIndent(idx, "", "  ")
	os.WriteFile(filepath.Join(projDir, "state.json"), data, 0644)

	resolver := &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns}
	testApp := &cmdutil.App{
		WolfcastleDir: wcDir,
		Cfg:           cfg,
		Resolver:      resolver,
	}

	rootCmd := &cobra.Command{Use: "wolfcastle"}
	rootCmd.AddGroup(
		&cobra.Group{ID: "lifecycle", Title: "Lifecycle:"},
		&cobra.Group{ID: "work", Title: "Work Management:"},
		&cobra.Group{ID: "audit", Title: "Auditing:"},
		&cobra.Group{ID: "docs", Title: "Documentation:"},
		&cobra.Group{ID: "diagnostics", Title: "Diagnostics:"},
		&cobra.Group{ID: "integration", Title: "Integration:"},
	)
	Register(testApp, rootCmd)

	return &testEnv{
		WolfcastleDir: wcDir,
		ProjectsDir:   projDir,
		App:           testApp,
		RootCmd:       rootCmd,
	}
}

// ---------------------------------------------------------------------------
// getDaemonStatus
// ---------------------------------------------------------------------------

func TestGetDaemonStatus_NoPidFile(t *testing.T) {
	tmp := t.TempDir()
	status := getDaemonStatus(tmp)
	if status != "stopped" {
		t.Errorf("expected 'stopped', got %q", status)
	}
}

func TestGetDaemonStatus_MalformedPid(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "wolfcastle.pid"), []byte("not-a-number"), 0644)
	status := getDaemonStatus(tmp)
	if status != "unknown (malformed PID file)" {
		t.Errorf("expected malformed PID message, got %q", status)
	}
}

func TestGetDaemonStatus_StalePid(t *testing.T) {
	tmp := t.TempDir()
	// Use PID 1 (which exists but won't be our daemon), or a very large PID
	os.WriteFile(filepath.Join(tmp, "wolfcastle.pid"), []byte("99999999"), 0644)
	status := getDaemonStatus(tmp)
	if status == "" {
		t.Error("status should not be empty")
	}
	// Should report stopped with stale PID
}

// ---------------------------------------------------------------------------
// isInSubtree
// ---------------------------------------------------------------------------

func TestIsInSubtree_DirectMatch(t *testing.T) {
	idx := &state.RootIndex{
		Nodes: map[string]state.IndexEntry{
			"auth": {Address: "auth"},
		},
	}
	if !isInSubtree(idx, "auth", "auth") {
		t.Error("direct match should return true")
	}
}

func TestIsInSubtree_ChildMatch(t *testing.T) {
	idx := &state.RootIndex{
		Nodes: map[string]state.IndexEntry{
			"auth":       {Address: "auth"},
			"auth/login": {Address: "auth/login", Parent: "auth"},
		},
	}
	if !isInSubtree(idx, "auth/login", "auth") {
		t.Error("child of scope should return true")
	}
}

func TestIsInSubtree_NoMatch(t *testing.T) {
	idx := &state.RootIndex{
		Nodes: map[string]state.IndexEntry{
			"auth": {Address: "auth"},
			"api":  {Address: "api"},
		},
	}
	if isInSubtree(idx, "api", "auth") {
		t.Error("unrelated node should return false")
	}
}

// ---------------------------------------------------------------------------
// recoverStaleDaemonState
// ---------------------------------------------------------------------------

func TestRecoverStaleDaemonState_NoPidFile(t *testing.T) {
	tmp := t.TempDir()
	// Should not panic
	recoverStaleDaemonState(tmp)
}

func TestRecoverStaleDaemonState_MalformedPid(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "wolfcastle.pid"), []byte("garbage"), 0644)
	recoverStaleDaemonState(tmp)

	// PID file should be removed
	if _, err := os.Stat(filepath.Join(tmp, "wolfcastle.pid")); !os.IsNotExist(err) {
		t.Error("malformed PID file should be cleaned up")
	}
}

func TestRecoverStaleDaemonState_DeadProcess(t *testing.T) {
	tmp := t.TempDir()
	// Use a very large PID that almost certainly doesn't exist
	os.WriteFile(filepath.Join(tmp, "wolfcastle.pid"), []byte("99999999"), 0644)
	os.WriteFile(filepath.Join(tmp, "daemon.meta.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(tmp, "stop"), []byte(""), 0644)

	recoverStaleDaemonState(tmp)

	// All stale files should be cleaned up
	if _, err := os.Stat(filepath.Join(tmp, "wolfcastle.pid")); !os.IsNotExist(err) {
		t.Error("stale PID file should be removed")
	}
	if _, err := os.Stat(filepath.Join(tmp, "daemon.meta.json")); !os.IsNotExist(err) {
		t.Error("stale daemon meta file should be removed")
	}
	if _, err := os.Stat(filepath.Join(tmp, "stop")); !os.IsNotExist(err) {
		t.Error("stale stop file should be removed")
	}
}

// ---------------------------------------------------------------------------
// status command (no resolver)
// ---------------------------------------------------------------------------

func TestStatusCmd_NoResolver(t *testing.T) {
	env := newTestEnv(t)
	env.App.Resolver = nil

	env.RootCmd.SetArgs([]string{"status"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when resolver is nil")
	}
}

// ---------------------------------------------------------------------------
// follow command - just unit test the log line formatting
// ---------------------------------------------------------------------------

func TestFormatAndPrintLogLine_StageStart(t *testing.T) {
	line := `{"type":"stage_start","stage":"execute","node":"my-project","task":"task-1"}`
	// Should not panic
	formatAndPrintLogLine(line)
}

func TestFormatAndPrintLogLine_InvalidJSON(t *testing.T) {
	// Should not panic on invalid JSON
	formatAndPrintLogLine("not json")
}

func TestFormatAndPrintLogLine_AllTypes(t *testing.T) {
	lines := []string{
		`{"type":"stage_start","stage":"expand","node":"n","task":"t"}`,
		`{"type":"stage_complete","stage":"expand","exit_code":0}`,
		`{"type":"stage_error","stage":"expand","error":"something failed"}`,
		`{"type":"assistant","text":"Hello world"}`,
		`{"type":"failure_increment","task":"task-1","count":3}`,
		`{"type":"auto_block","task":"task-1","reason":"too many failures"}`,
	}
	for _, line := range lines {
		formatAndPrintLogLine(line)
	}
}

// ---------------------------------------------------------------------------
// offset tracking
// ---------------------------------------------------------------------------

func TestOffsetTracking(t *testing.T) {
	path := "/tmp/test-log.ndjson"
	if getOffset(path) != 0 {
		t.Error("initial offset should be 0")
	}

	setOffset(path, 1024)
	if getOffset(path) != 1024 {
		t.Errorf("expected 1024, got %d", getOffset(path))
	}

	setOffset(path, 2048)
	if getOffset(path) != 2048 {
		t.Errorf("expected 2048, got %d", getOffset(path))
	}
}

// ---------------------------------------------------------------------------
// stop command - no running daemon
// ---------------------------------------------------------------------------

func TestStopCmd_NoPidFile(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"stop"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when no PID file exists")
	}
}

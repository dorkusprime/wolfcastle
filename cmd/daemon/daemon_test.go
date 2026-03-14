package daemon

import (
	"encoding/json"
	"fmt"
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

func TestStopCmd_StalePid(t *testing.T) {
	env := newTestEnv(t)
	os.WriteFile(filepath.Join(env.WolfcastleDir, "wolfcastle.pid"), []byte("99999999"), 0644)
	env.RootCmd.SetArgs([]string{"stop"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for stale PID")
	}
}

// ---------------------------------------------------------------------------
// showHistoricalLines
// ---------------------------------------------------------------------------

func TestShowHistoricalLines_ValidLog(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "test.ndjson")
	lines := `{"type":"stage_start","stage":"expand","node":"n","task":"t"}
{"type":"assistant","text":"Hello"}
{"type":"stage_complete","stage":"expand","exit_code":0}
`
	os.WriteFile(logFile, []byte(lines), 0644)

	// Reset offsets
	delete(fileOffsets, logFile)

	// Should not panic, should set offset
	showHistoricalLines(logFile, 2)

	offset := getOffset(logFile)
	if offset == 0 {
		t.Error("expected offset to be set after showing historical lines")
	}
}

func TestShowHistoricalLines_NonexistentFile(t *testing.T) {
	// Should not panic on missing file
	showHistoricalLines("/tmp/nonexistent-log-file-xyz.ndjson", 10)
}

func TestShowHistoricalLines_MoreLinesThanAvailable(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "short.ndjson")
	os.WriteFile(logFile, []byte(`{"type":"assistant","text":"only one"}`+"\n"), 0644)
	delete(fileOffsets, logFile)

	showHistoricalLines(logFile, 100) // Asking for 100 lines when only 1 exists
}

// ---------------------------------------------------------------------------
// tailFileStreaming
// ---------------------------------------------------------------------------

func TestTailFileStreaming_NoNewData(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "tail.ndjson")
	content := `{"type":"assistant","text":"Hello"}` + "\n"
	os.WriteFile(logFile, []byte(content), 0644)

	info, _ := os.Stat(logFile)
	setOffset(logFile, info.Size())

	err := tailFileStreaming(logFile)
	if err != nil {
		t.Fatalf("tailFileStreaming failed: %v", err)
	}
}

func TestTailFileStreaming_WithNewData(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "tail2.ndjson")
	content := `{"type":"assistant","text":"first"}` + "\n"
	os.WriteFile(logFile, []byte(content), 0644)

	setOffset(logFile, 0) // Start from beginning

	err := tailFileStreaming(logFile)
	if err != nil {
		t.Fatalf("tailFileStreaming failed: %v", err)
	}

	offset := getOffset(logFile)
	if offset == 0 {
		t.Error("offset should have advanced after reading new data")
	}
}

func TestTailFileStreaming_FileNotFound(t *testing.T) {
	err := tailFileStreaming("/tmp/nonexistent-tail-xyz.ndjson")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// ---------------------------------------------------------------------------
// formatAndPrintLogLine edge cases
// ---------------------------------------------------------------------------

func TestFormatAndPrintLogLine_StageStartNoNode(t *testing.T) {
	// Stage start without node/task
	formatAndPrintLogLine(`{"type":"stage_start","stage":"expand"}`)
}

func TestFormatAndPrintLogLine_AssistantWithNewline(t *testing.T) {
	formatAndPrintLogLine(`{"type":"assistant","text":"line\n"}`)
}

func TestFormatAndPrintLogLine_UnknownType(t *testing.T) {
	// Unknown type should be silently ignored
	formatAndPrintLogLine(`{"type":"unknown_event","data":"stuff"}`)
}

// ---------------------------------------------------------------------------
// status command JSON output
// ---------------------------------------------------------------------------

func TestStatusCmd_JSONOutput(t *testing.T) {
	env := newStatusTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"status"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status --json failed: %v", err)
	}
}

func TestShowAllStatus_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	err := showAllStatus(env.App)
	if err != nil {
		t.Fatalf("showAllStatus JSON failed: %v", err)
	}
}

func TestShowAllStatus_WithNamespace(t *testing.T) {
	env := newStatusTestEnv(t)
	// The projects dir already has a namespace with a state.json
	err := showAllStatus(env.App)
	if err != nil {
		t.Fatalf("showAllStatus with namespace failed: %v", err)
	}
}

func TestShowTreeStatus_JSONOutput(t *testing.T) {
	env := newStatusTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	err := showTreeStatus(env.App, idx, "")
	if err != nil {
		t.Fatalf("showTreeStatus JSON failed: %v", err)
	}
}

func TestShowTreeStatus_WithScope(t *testing.T) {
	env := newStatusTestEnv(t)
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	err := showTreeStatus(env.App, idx, "my-project")
	if err != nil {
		t.Fatalf("showTreeStatus with scope failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// isInSubtree edge cases
// ---------------------------------------------------------------------------

func TestIsInSubtree_MissingNode(t *testing.T) {
	idx := &state.RootIndex{
		Nodes: map[string]state.IndexEntry{},
	}
	if isInSubtree(idx, "missing", "auth") {
		t.Error("missing node should return false")
	}
}

func TestIsInSubtree_DeepChild(t *testing.T) {
	idx := &state.RootIndex{
		Nodes: map[string]state.IndexEntry{
			"root":             {Address: "root"},
			"root/mid":         {Address: "root/mid", Parent: "root"},
			"root/mid/deep":    {Address: "root/mid/deep", Parent: "root/mid"},
		},
	}
	if !isInSubtree(idx, "root/mid/deep", "root") {
		t.Error("deep child should be in subtree of root")
	}
}

// ---------------------------------------------------------------------------
// Register command
// ---------------------------------------------------------------------------

func TestRegister_AllCommandsPresent(t *testing.T) {
	env := newTestEnv(t)
	cmds := env.RootCmd.Commands()
	names := make(map[string]bool)
	for _, c := range cmds {
		names[c.Name()] = true
	}
	for _, expected := range []string{"start", "stop", "follow", "status"} {
		if !names[expected] {
			t.Errorf("expected command %q to be registered", expected)
		}
	}
}

// ---------------------------------------------------------------------------
// getDaemonStatus with running process (self PID)
// ---------------------------------------------------------------------------

func TestGetDaemonStatus_RunningProcess(t *testing.T) {
	tmp := t.TempDir()
	pid := os.Getpid()
	os.WriteFile(filepath.Join(tmp, "wolfcastle.pid"), []byte(fmt.Sprintf("%d", pid)), 0644)
	status := getDaemonStatus(tmp)
	if status == "stopped" {
		t.Error("expected running status for own PID")
	}
}

// ---------------------------------------------------------------------------
// recoverStaleDaemonState edge cases
// ---------------------------------------------------------------------------

func TestRecoverStaleDaemonState_RunningProcess(t *testing.T) {
	tmp := t.TempDir()
	// Use our own PID (which is running)
	pid := os.Getpid()
	os.WriteFile(filepath.Join(tmp, "wolfcastle.pid"), []byte(fmt.Sprintf("%d", pid)), 0644)

	recoverStaleDaemonState(tmp)

	// PID file should still exist since process is running
	if _, err := os.Stat(filepath.Join(tmp, "wolfcastle.pid")); os.IsNotExist(err) {
		t.Error("PID file should not be removed for a running process")
	}
}

// ---------------------------------------------------------------------------
// start command edge cases
// ---------------------------------------------------------------------------

func TestStartCmd_NoResolver(t *testing.T) {
	env := newTestEnv(t)
	env.App.Resolver = nil
	env.RootCmd.SetArgs([]string{"start"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when resolver is nil")
	}
}

func TestStartCmd_AlreadyRunning(t *testing.T) {
	env := newTestEnv(t)
	// Write our own PID as the running daemon
	pid := os.Getpid()
	os.WriteFile(filepath.Join(env.WolfcastleDir, "wolfcastle.pid"), []byte(fmt.Sprintf("%d", pid)), 0644)

	env.RootCmd.SetArgs([]string{"start"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when daemon is already running")
	}
}

// ---------------------------------------------------------------------------
// showAllStatus with data
// ---------------------------------------------------------------------------

func TestShowAllStatus_JSONWithNamespace(t *testing.T) {
	env := newStatusTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	err := showAllStatus(env.App)
	if err != nil {
		t.Fatalf("showAllStatus JSON with namespace failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// stop command JSON output
// ---------------------------------------------------------------------------

func TestStopCmd_StalePidJSON(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	os.WriteFile(filepath.Join(env.WolfcastleDir, "wolfcastle.pid"), []byte("99999999"), 0644)
	env.RootCmd.SetArgs([]string{"stop"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for stale PID")
	}
}

// ---------------------------------------------------------------------------
// showHistoricalLines edge cases
// ---------------------------------------------------------------------------

func TestShowHistoricalLines_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "empty.ndjson")
	os.WriteFile(logFile, []byte(""), 0644)
	delete(fileOffsets, logFile)
	showHistoricalLines(logFile, 10)
}

// ---------------------------------------------------------------------------
// status command with --all flag
// ---------------------------------------------------------------------------

func TestStatusCmd_AllFlag(t *testing.T) {
	env := newStatusTestEnv(t)
	env.RootCmd.SetArgs([]string{"status", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status --all failed: %v", err)
	}
}

func TestStatusCmd_AllFlagJSON(t *testing.T) {
	env := newStatusTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"status", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status --all --json failed: %v", err)
	}
}

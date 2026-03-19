package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/config"
	dmn "github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/logging"
	"github.com/dorkusprime/wolfcastle/internal/state"
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
	_ = os.MkdirAll(wcDir, 0755)

	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "test", Machine: "dev"}

	ns := "test-dev"
	projDir := filepath.Join(wcDir, "system", "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	idx := state.NewRootIndex()
	data, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), data, 0644)

	id, _ := config.IdentityFromConfig(cfg)
	testApp := &cmdutil.App{
		WolfcastleDir: wcDir,
		Cfg:           cfg,
		Identity:      id,
		Config:        config.NewConfigRepository(wcDir),
		Daemon:        dmn.NewDaemonRepository(wcDir),
		State:         state.NewStateStore(projDir, state.DefaultLockTimeout),
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
	_ = os.MkdirAll(filepath.Join(tmp, "system"), 0755)
	_ = os.WriteFile(filepath.Join(tmp, "system", "wolfcastle.pid"), []byte("not-a-number"), 0644)
	status := getDaemonStatus(tmp)
	if status != "unknown (malformed PID file)" {
		t.Errorf("expected malformed PID message, got %q", status)
	}
}

func TestGetDaemonStatus_StalePid(t *testing.T) {
	tmp := t.TempDir()
	// Use PID 1 (which exists but won't be our daemon), or a very large PID
	_ = os.MkdirAll(filepath.Join(tmp, "system"), 0755)
	_ = os.WriteFile(filepath.Join(tmp, "system", "wolfcastle.pid"), []byte("99999999"), 0644)
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
	_ = os.MkdirAll(filepath.Join(tmp, "system"), 0755)
	_ = os.WriteFile(filepath.Join(tmp, "system", "wolfcastle.pid"), []byte("garbage"), 0644)
	recoverStaleDaemonState(tmp)

	// PID file should be removed
	if _, err := os.Stat(filepath.Join(tmp, "system", "wolfcastle.pid")); !os.IsNotExist(err) {
		t.Error("malformed PID file should be cleaned up")
	}
}

func TestRecoverStaleDaemonState_DeadProcess(t *testing.T) {
	tmp := t.TempDir()
	// Use a very large PID that almost certainly doesn't exist
	_ = os.MkdirAll(filepath.Join(tmp, "system"), 0755)
	_ = os.WriteFile(filepath.Join(tmp, "system", "wolfcastle.pid"), []byte("99999999"), 0644)
	_ = os.WriteFile(filepath.Join(tmp, "system", "daemon.meta.json"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(tmp, "system", "stop"), []byte(""), 0644)

	recoverStaleDaemonState(tmp)

	// All stale files should be cleaned up
	if _, err := os.Stat(filepath.Join(tmp, "system", "wolfcastle.pid")); !os.IsNotExist(err) {
		t.Error("stale PID file should be removed")
	}
	if _, err := os.Stat(filepath.Join(tmp, "system", "daemon.meta.json")); !os.IsNotExist(err) {
		t.Error("stale daemon meta file should be removed")
	}
	if _, err := os.Stat(filepath.Join(tmp, "system", "stop")); !os.IsNotExist(err) {
		t.Error("stale stop file should be removed")
	}
}

// ---------------------------------------------------------------------------
// status command (no resolver)
// ---------------------------------------------------------------------------

func TestStatusCmd_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"status"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when identity is nil")
	}
}

// ---------------------------------------------------------------------------
// follow command - just unit test the log line formatting
// ---------------------------------------------------------------------------

func TestFormatAndPrintLogLine_StageStart(t *testing.T) {
	line := `{"type":"stage_start","stage":"execute","node":"my-project","task":"task-0001"}`
	// Should not panic
	formatAndPrintLogLine(line, logging.LevelDebug)
}

func TestFormatAndPrintLogLine_InvalidJSON(t *testing.T) {
	// Should not panic on invalid JSON
	formatAndPrintLogLine("not json", logging.LevelDebug)
}

func TestFormatAndPrintLogLine_AllTypes(t *testing.T) {
	lines := []string{
		`{"type":"stage_start","stage":"expand","node":"n","task":"t"}`,
		`{"type":"stage_complete","stage":"expand","exit_code":0}`,
		`{"type":"stage_error","stage":"expand","error":"something failed"}`,
		`{"type":"assistant","text":"Hello world"}`,
		`{"type":"failure_increment","task":"task-0001","count":3}`,
		`{"type":"auto_block","task":"task-0001","reason":"too many failures"}`,
	}
	for _, line := range lines {
		formatAndPrintLogLine(line, logging.LevelDebug)
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
	_ = os.MkdirAll(filepath.Join(env.WolfcastleDir, "system"), 0755)
	_ = os.WriteFile(filepath.Join(env.WolfcastleDir, "system", "wolfcastle.pid"), []byte("99999999"), 0644)
	env.RootCmd.SetArgs([]string{"stop"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for stale PID")
	}
}

func TestStopCmd_StalePid_Force(t *testing.T) {
	env := newTestEnv(t)
	_ = os.MkdirAll(filepath.Join(env.WolfcastleDir, "system"), 0755)
	_ = os.WriteFile(filepath.Join(env.WolfcastleDir, "system", "wolfcastle.pid"), []byte("99999999"), 0644)
	env.RootCmd.SetArgs([]string{"stop", "--force"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for stale PID even with force")
	}
}

func TestStopCmd_RunningProcess_SIGTERM(t *testing.T) {
	env := newTestEnv(t)

	// Start a long-lived subprocess that we can safely send SIGTERM to
	sleepCmd := exec.Command("sleep", "60")
	if err := sleepCmd.Start(); err != nil {
		t.Fatalf("starting sleep process: %v", err)
	}
	defer func() { _ = sleepCmd.Process.Kill(); _ = sleepCmd.Wait() }()

	pid := sleepCmd.Process.Pid
	_ = os.MkdirAll(filepath.Join(env.WolfcastleDir, "system"), 0755)
	_ = os.WriteFile(filepath.Join(env.WolfcastleDir, "system", "wolfcastle.pid"), []byte(fmt.Sprintf("%d", pid)), 0644)

	env.RootCmd.SetArgs([]string{"stop"})
	err := env.RootCmd.Execute()
	if err != nil {
		t.Fatalf("stop should succeed for running process: %v", err)
	}
}

func TestStopCmd_RunningProcess_SIGTERM_JSON(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	sleepCmd := exec.Command("sleep", "60")
	if err := sleepCmd.Start(); err != nil {
		t.Fatalf("starting sleep process: %v", err)
	}
	defer func() { _ = sleepCmd.Process.Kill(); _ = sleepCmd.Wait() }()

	pid := sleepCmd.Process.Pid
	_ = os.MkdirAll(filepath.Join(env.WolfcastleDir, "system"), 0755)
	_ = os.WriteFile(filepath.Join(env.WolfcastleDir, "system", "wolfcastle.pid"), []byte(fmt.Sprintf("%d", pid)), 0644)

	env.RootCmd.SetArgs([]string{"stop"})
	err := env.RootCmd.Execute()
	if err != nil {
		t.Fatalf("stop --json should succeed for running process: %v", err)
	}
}

func TestStopCmd_RunningProcess_Force(t *testing.T) {
	env := newTestEnv(t)

	sleepCmd := exec.Command("sleep", "60")
	if err := sleepCmd.Start(); err != nil {
		t.Fatalf("starting sleep process: %v", err)
	}
	defer func() { _ = sleepCmd.Process.Kill(); _ = sleepCmd.Wait() }()

	pid := sleepCmd.Process.Pid
	_ = os.MkdirAll(filepath.Join(env.WolfcastleDir, "system"), 0755)
	_ = os.WriteFile(filepath.Join(env.WolfcastleDir, "system", "wolfcastle.pid"), []byte(fmt.Sprintf("%d", pid)), 0644)

	env.RootCmd.SetArgs([]string{"stop", "--force"})
	err := env.RootCmd.Execute()
	if err != nil {
		t.Fatalf("stop --force should succeed for running process: %v", err)
	}
}

// ---------------------------------------------------------------------------
// follow command - RunE path testing
// ---------------------------------------------------------------------------

func TestFollowCmd_WithLogFile(t *testing.T) {
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, "001-test.jsonl")
	_ = os.WriteFile(logFile, []byte(`{"type":"assistant","text":"hello"}`+"\n"), 0644)

	done := make(chan error, 1)
	go func() {
		env.RootCmd.SetArgs([]string{"follow", "--lines", "5"})
		done <- env.RootCmd.Execute()
	}()

	// Follow loops forever; let it run long enough to exercise the code paths
	select {
	case err := <-done:
		_ = err
	case <-time.After(1500 * time.Millisecond):
		// Expected: still running in the tail loop
	}
}

func TestFollowCmd_NoLogs(t *testing.T) {
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)

	done := make(chan error, 1)
	go func() {
		env.RootCmd.SetArgs([]string{"follow"})
		done <- env.RootCmd.Execute()
	}()

	// Follow waits for logs (2s sleep + retry), let it run briefly
	select {
	case err := <-done:
		_ = err
	case <-time.After(2500 * time.Millisecond):
		// Expected: stuck waiting for log files
	}
}

func TestStopCmd_RunningProcess_Force_JSON(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	sleepCmd := exec.Command("sleep", "60")
	if err := sleepCmd.Start(); err != nil {
		t.Fatalf("starting sleep process: %v", err)
	}
	defer func() { _ = sleepCmd.Process.Kill(); _ = sleepCmd.Wait() }()

	pid := sleepCmd.Process.Pid
	_ = os.MkdirAll(filepath.Join(env.WolfcastleDir, "system"), 0755)
	_ = os.WriteFile(filepath.Join(env.WolfcastleDir, "system", "wolfcastle.pid"), []byte(fmt.Sprintf("%d", pid)), 0644)

	env.RootCmd.SetArgs([]string{"stop", "--force"})
	err := env.RootCmd.Execute()
	if err != nil {
		t.Fatalf("stop --force --json should succeed: %v", err)
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
	_ = os.WriteFile(logFile, []byte(lines), 0644)

	// Reset offsets
	delete(fileOffsets, logFile)

	// Should not panic, should set offset
	showHistoricalLines(logFile, 2, logging.LevelDebug)

	offset := getOffset(logFile)
	if offset == 0 {
		t.Error("expected offset to be set after showing historical lines")
	}
}

func TestShowHistoricalLines_NonexistentFile(t *testing.T) {
	// Should not panic on missing file
	showHistoricalLines("/tmp/nonexistent-log-file-xyz.ndjson", 10, logging.LevelDebug)
}

func TestShowHistoricalLines_MoreLinesThanAvailable(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "short.ndjson")
	_ = os.WriteFile(logFile, []byte(`{"type":"assistant","text":"only one"}`+"\n"), 0644)
	delete(fileOffsets, logFile)

	showHistoricalLines(logFile, 100, logging.LevelDebug) // Asking for 100 lines when only 1 exists
}

// ---------------------------------------------------------------------------
// tailFileStreaming
// ---------------------------------------------------------------------------

func TestTailFileStreaming_NoNewData(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "tail.ndjson")
	content := `{"type":"assistant","text":"Hello"}` + "\n"
	_ = os.WriteFile(logFile, []byte(content), 0644)

	info, _ := os.Stat(logFile)
	setOffset(logFile, info.Size())

	err := tailFileStreaming(logFile, logging.LevelDebug)
	if err != nil {
		t.Fatalf("tailFileStreaming failed: %v", err)
	}
}

func TestTailFileStreaming_WithNewData(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "tail2.ndjson")
	content := `{"type":"assistant","text":"first"}` + "\n"
	_ = os.WriteFile(logFile, []byte(content), 0644)

	setOffset(logFile, 0) // Start from beginning

	err := tailFileStreaming(logFile, logging.LevelDebug)
	if err != nil {
		t.Fatalf("tailFileStreaming failed: %v", err)
	}

	offset := getOffset(logFile)
	if offset == 0 {
		t.Error("offset should have advanced after reading new data")
	}
}

func TestTailFileStreaming_FileNotFound(t *testing.T) {
	err := tailFileStreaming("/tmp/nonexistent-tail-xyz.ndjson", logging.LevelDebug)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// ---------------------------------------------------------------------------
// formatAndPrintLogLine edge cases
// ---------------------------------------------------------------------------

func TestFormatAndPrintLogLine_StageStartNoNode(t *testing.T) {
	// Stage start without node/task
	formatAndPrintLogLine(`{"type":"stage_start","stage":"expand"}`, logging.LevelDebug)
}

func TestFormatAndPrintLogLine_AssistantWithNewline(t *testing.T) {
	formatAndPrintLogLine(`{"type":"assistant","text":"line\n"}`, logging.LevelDebug)
}

func TestFormatAndPrintLogLine_UnknownType(t *testing.T) {
	// Unknown type should be silently ignored
	formatAndPrintLogLine(`{"type":"unknown_event","data":"stuff"}`, logging.LevelDebug)
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
			"root":          {Address: "root"},
			"root/mid":      {Address: "root/mid", Parent: "root"},
			"root/mid/deep": {Address: "root/mid/deep", Parent: "root/mid"},
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
	for _, expected := range []string{"start", "stop", "log", "status"} {
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
	_ = os.MkdirAll(filepath.Join(tmp, "system"), 0755)
	_ = os.WriteFile(filepath.Join(tmp, "system", "wolfcastle.pid"), []byte(fmt.Sprintf("%d", pid)), 0644)
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
	_ = os.MkdirAll(filepath.Join(tmp, "system"), 0755)
	_ = os.WriteFile(filepath.Join(tmp, "system", "wolfcastle.pid"), []byte(fmt.Sprintf("%d", pid)), 0644)

	recoverStaleDaemonState(tmp)

	// PID file should still exist since process is running
	if _, err := os.Stat(filepath.Join(tmp, "system", "wolfcastle.pid")); os.IsNotExist(err) {
		t.Error("PID file should not be removed for a running process")
	}
}

// ---------------------------------------------------------------------------
// start command edge cases
// ---------------------------------------------------------------------------

func TestStartCmd_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil
	env.RootCmd.SetArgs([]string{"start"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when identity is nil")
	}
}

func TestStartCmd_AlreadyRunning(t *testing.T) {
	env := newTestEnv(t)
	// Write our own PID as the running daemon
	pid := os.Getpid()
	_ = os.MkdirAll(filepath.Join(env.WolfcastleDir, "system"), 0755)
	_ = os.WriteFile(filepath.Join(env.WolfcastleDir, "system", "wolfcastle.pid"), []byte(fmt.Sprintf("%d", pid)), 0644)

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

	_ = os.MkdirAll(filepath.Join(env.WolfcastleDir, "system"), 0755)
	_ = os.WriteFile(filepath.Join(env.WolfcastleDir, "system", "wolfcastle.pid"), []byte("99999999"), 0644)
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
	_ = os.WriteFile(logFile, []byte(""), 0644)
	delete(fileOffsets, logFile)
	showHistoricalLines(logFile, 10, logging.LevelDebug)
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

// ---------------------------------------------------------------------------
// status --interval flag accepts floats
// ---------------------------------------------------------------------------

func TestStatusCmd_IntervalAcceptsFloat(t *testing.T) {
	env := newTestEnv(t)
	// Verify --interval parses a float without error
	env.RootCmd.SetArgs([]string{"status", "--interval", "0.5"})
	// This will fail because no resolver, but the flag parsing succeeds
	err := env.RootCmd.Execute()
	// We only care that it didn't fail with "invalid argument" for --interval
	if err != nil && strings.Contains(err.Error(), "ParseInt") {
		t.Errorf("--interval should accept floats: %v", err)
	}
}

func TestStatusCmd_IntervalAcceptsSubSecond(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"status", "--interval", "0.05"})
	err := env.RootCmd.Execute()
	if err != nil && strings.Contains(err.Error(), "ParseInt") {
		t.Errorf("--interval should accept sub-second floats: %v", err)
	}
}

// ---------------------------------------------------------------------------
// log command has --follow flag
// ---------------------------------------------------------------------------

func TestLogCmd_FollowFlagExists(t *testing.T) {
	env := newTestEnv(t)
	// Verify the log command accepts --follow without parse error
	env.RootCmd.SetArgs([]string{"log", "--follow=false", "--lines", "5"})
	// Will fail (no logs dir) but flag parsing should succeed
	_ = env.RootCmd.Execute()
}

func TestLogCmd_AliasFollow(t *testing.T) {
	env := newTestEnv(t)
	// "follow" should work as an alias for "log"
	env.RootCmd.SetArgs([]string{"follow", "--lines", "5"})
	_ = env.RootCmd.Execute()
}

// ---------------------------------------------------------------------------
// nodeGlyph
// ---------------------------------------------------------------------------

func TestNodeGlyph(t *testing.T) {
	// In tests, output.IsTerminal() is false (piped), so we get plain glyphs.
	tests := []struct {
		status state.NodeStatus
		want   string
	}{
		{state.StatusComplete, "●"},
		{state.StatusInProgress, "◐"},
		{state.StatusBlocked, "☢"},
		{state.StatusNotStarted, "◯"},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := nodeGlyph(tt.status)
			if got != tt.want {
				t.Errorf("nodeGlyph(%s) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// taskGlyph
// ---------------------------------------------------------------------------

func TestTaskGlyph(t *testing.T) {
	tests := []struct {
		status state.NodeStatus
		want   string
	}{
		{state.StatusComplete, "✓"},
		{state.StatusInProgress, "→"},
		{state.StatusBlocked, "✖"},
		{state.StatusNotStarted, "○"},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := taskGlyph(tt.status)
			if got != tt.want {
				t.Errorf("taskGlyph(%s) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// printNodeTree
// ---------------------------------------------------------------------------

func TestPrintNodeTree(t *testing.T) {
	env := newStatusTestEnv(t)

	// Build a tree: one orchestrator ("orch") with two leaf children ("leaf-a", "leaf-b").
	// Each leaf has tasks in various states.
	idx := state.NewRootIndex()
	idx.Root = []string{"orch"}
	idx.Nodes["orch"] = state.IndexEntry{
		Name:     "Orchestrator",
		Type:     state.NodeOrchestrator,
		State:    state.StatusInProgress,
		Address:  "orch",
		Children: []string{"leaf-a", "leaf-b"},
	}
	idx.Nodes["leaf-a"] = state.IndexEntry{
		Name:    "Leaf A",
		Type:    state.NodeLeaf,
		State:   state.StatusComplete,
		Address: "leaf-a",
		Parent:  "orch",
	}
	idx.Nodes["leaf-b"] = state.IndexEntry{
		Name:    "Leaf B",
		Type:    state.NodeLeaf,
		State:   state.StatusBlocked,
		Address: "leaf-b",
		Parent:  "orch",
	}

	// Create node state files for the leaves
	leafADir := filepath.Join(env.ProjectsDir, "leaf-a")
	_ = os.MkdirAll(leafADir, 0755)
	nsA := state.NewNodeState("leaf-a", "Leaf A", state.NodeLeaf)
	nsA.Tasks = []state.Task{
		{ID: "task-0001", Title: "First task", State: state.StatusComplete},
		{ID: "task-0002", Title: "Second task", State: state.StatusInProgress},
	}
	nsAData, _ := json.MarshalIndent(nsA, "", "  ")
	_ = os.WriteFile(filepath.Join(leafADir, "state.json"), nsAData, 0644)

	leafBDir := filepath.Join(env.ProjectsDir, "leaf-b")
	_ = os.MkdirAll(leafBDir, 0755)
	nsB := state.NewNodeState("leaf-b", "Leaf B", state.NodeLeaf)
	nsB.Tasks = []state.Task{
		{ID: "task-0003", Title: "Blocked task", State: state.StatusBlocked, BlockedReason: "waiting on API"},
		{ID: "task-0004", Description: "Not started yet", State: state.StatusNotStarted},
		{ID: "task-0005", Title: "Failing task", State: state.StatusInProgress, FailureCount: 3},
	}
	nsBData, _ := json.MarshalIndent(nsB, "", "  ")
	_ = os.WriteFile(filepath.Join(leafBDir, "state.json"), nsBData, 0644)

	// Build details map the same way showTreeStatus does
	details := map[string]*nodeDetail{
		"orch":   {entry: idx.Nodes["orch"]},
		"leaf-a": {entry: idx.Nodes["leaf-a"], ns: nsA},
		"leaf-b": {entry: idx.Nodes["leaf-b"], ns: nsB},
	}

	// Should not panic; exercises orchestrator recursion, leaf task rendering,
	// blocked reason display, failure count display, and title/description fallback.
	printNodeTree(env.App, idx, details, "orch", "  ")
}

func TestPrintNodeTree_MissingAddr(t *testing.T) {
	env := newStatusTestEnv(t)
	idx := state.NewRootIndex()
	details := map[string]*nodeDetail{}

	// Calling with an address not in details should return silently.
	printNodeTree(env.App, idx, details, "nonexistent", "  ")
}

func TestPrintNodeTree_LeafWithNilNodeState(t *testing.T) {
	env := newStatusTestEnv(t)
	idx := state.NewRootIndex()
	idx.Nodes["leaf"] = state.IndexEntry{
		Name:    "Leaf",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: "leaf",
	}
	details := map[string]*nodeDetail{
		"leaf": {entry: idx.Nodes["leaf"], ns: nil},
	}

	// Should not panic when ns is nil (no tasks to print).
	printNodeTree(env.App, idx, details, "leaf", "  ")
}

// ---------------------------------------------------------------------------
// startBackground
// ---------------------------------------------------------------------------

func TestStartBackground_HappyPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfDir := filepath.Join(dir, ".wolfcastle")
	_ = os.MkdirAll(wolfDir, 0755)
	_ = os.MkdirAll(filepath.Join(wolfDir, "system"), 0755)
	_ = os.MkdirAll(filepath.Join(wolfDir, "system"), 0755)

	// Use "sleep" as the child process; it starts and we release it.
	err := startBackground(wolfDir, "", "", "sleep")
	if err != nil {
		t.Fatalf("startBackground failed: %v", err)
	}

	// PID file should exist
	pidData, err := os.ReadFile(filepath.Join(wolfDir, "system", "wolfcastle.pid"))
	if err != nil {
		t.Fatal("PID file should exist after startBackground")
	}
	if len(pidData) == 0 {
		t.Error("PID file should not be empty")
	}

	// daemon.log should exist
	if _, err := os.Stat(filepath.Join(wolfDir, "system", "daemon.log")); err != nil {
		t.Error("daemon.log should exist")
	}
}

func TestStartBackground_WithNodeScope(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfDir := filepath.Join(dir, ".wolfcastle")
	_ = os.MkdirAll(wolfDir, 0755)
	_ = os.MkdirAll(filepath.Join(wolfDir, "system"), 0755)

	err := startBackground(wolfDir, "my-project", "", "sleep")
	if err != nil {
		t.Fatalf("startBackground with scope failed: %v", err)
	}
}

func TestStartBackground_WithWorktree(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfDir := filepath.Join(dir, ".wolfcastle")
	_ = os.MkdirAll(wolfDir, 0755)
	_ = os.MkdirAll(filepath.Join(wolfDir, "system"), 0755)

	err := startBackground(wolfDir, "", "feature-branch", "sleep")
	if err != nil {
		t.Fatalf("startBackground with worktree failed: %v", err)
	}
}

func TestStartBackground_BadExecutable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfDir := filepath.Join(dir, ".wolfcastle")
	_ = os.MkdirAll(wolfDir, 0755)
	_ = os.MkdirAll(filepath.Join(wolfDir, "system"), 0755)

	err := startBackground(wolfDir, "", "", "/nonexistent/binary")
	if err == nil {
		t.Error("expected error for nonexistent executable")
	}
}

func TestStartBackground_LogDirNotWritable(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skip in CI")
	}
	t.Parallel()
	dir := t.TempDir()
	wolfDir := filepath.Join(dir, ".wolfcastle")
	_ = os.MkdirAll(wolfDir, 0755)
	_ = os.MkdirAll(filepath.Join(wolfDir, "system"), 0755)

	// Make wolfDir read-only so daemon.log creation fails
	_ = os.Chmod(filepath.Join(wolfDir, "system"), 0555)
	defer func() { _ = os.Chmod(filepath.Join(wolfDir, "system"), 0755) }()

	err := startBackground(wolfDir, "", "", "sleep")
	if err == nil {
		t.Error("expected error when log dir is not writable")
	}
}

// ---------------------------------------------------------------------------
// nodeGlyph and taskGlyph — all states
// ---------------------------------------------------------------------------

func TestNodeGlyph_AllStates(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status state.NodeStatus
		glyph  string
	}{
		{state.StatusComplete, "●"},
		{state.StatusInProgress, "◐"},
		{state.StatusBlocked, "☢"},
		{state.StatusNotStarted, "◯"},
		{"unknown_status", "◯"},
	}
	for _, tc := range cases {
		result := nodeGlyph(tc.status)
		if !strings.Contains(result, tc.glyph) {
			t.Errorf("nodeGlyph(%s) = %q, expected to contain %q", tc.status, result, tc.glyph)
		}
	}
}

func TestTaskGlyph_AllStates(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status state.NodeStatus
		glyph  string
	}{
		{state.StatusComplete, "✓"},
		{state.StatusInProgress, "→"},
		{state.StatusBlocked, "✖"},
		{state.StatusNotStarted, "○"},
		{"unknown_status", "○"},
	}
	for _, tc := range cases {
		result := taskGlyph(tc.status)
		if !strings.Contains(result, tc.glyph) {
			t.Errorf("taskGlyph(%s) = %q, expected to contain %q", tc.status, result, tc.glyph)
		}
	}
}

// ---------------------------------------------------------------------------
// watchStatus — runs one cycle then context cancels
// ---------------------------------------------------------------------------

func TestWatchStatus_SingleCycle(t *testing.T) {
	t.Parallel()
	env := newStatusTestEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- watchStatus(ctx, env.App, "", false, 0.1)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("watchStatus error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watchStatus did not exit after context cancellation")
	}
}

func TestWatchStatus_WithScope(t *testing.T) {
	t.Parallel()
	env := newStatusTestEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- watchStatus(ctx, env.App, "my-project", false, 0.1)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("watchStatus with scope error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watchStatus did not exit")
	}
}

// ---------------------------------------------------------------------------
// followLogs — context cancellation
// ---------------------------------------------------------------------------

func TestFollowLogs_NoLogs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := followLogs(ctx, dir, 20, logging.LevelInfo)
	if err != nil {
		t.Fatalf("followLogs error: %v", err)
	}
}

func TestFollowLogs_WithLogFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a log file
	logLine := `{"level":"info","type":"stage_start","stage":"execute","timestamp":"2026-03-16T00:00:00Z"}` + "\n"
	_ = os.WriteFile(filepath.Join(dir, "0001-20260316T00-00Z.jsonl"), []byte(logLine), 0644)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := followLogs(ctx, dir, 5, logging.LevelInfo)
	if err != nil {
		t.Fatalf("followLogs error: %v", err)
	}
}

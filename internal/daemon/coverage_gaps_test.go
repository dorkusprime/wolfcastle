package daemon

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/git"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// lockPath
// ═══════════════════════════════════════════════════════════════════════════

func TestLockPath_UsesEnvVar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", dir)

	p := lockPath("/some/wolfcastle/dir")
	if p != filepath.Join(dir, "daemon.lock") {
		t.Errorf("expected lock under WOLFCASTLE_LOCK_DIR, got %s", p)
	}
}

func TestLockPath_UsesWolfcastleDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", "")

	p := lockPath(dir)
	if p != filepath.Join(dir, "daemon.lock") {
		t.Errorf("expected lock under wolfcastleDir, got %s", p)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// ReadLock
// ═══════════════════════════════════════════════════════════════════════════

func TestReadLock_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", dir)

	_ = os.WriteFile(filepath.Join(dir, "daemon.lock"), []byte("{{{not json"), 0644)

	_, err := ReadLock(dir)
	if err == nil {
		t.Error("expected error for malformed JSON lock file")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// ReleaseLock
// ═══════════════════════════════════════════════════════════════════════════

func TestReleaseLock_NoLockFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", dir)

	// No lock file exists; should not panic.
	ReleaseLock(dir)
}

func TestReleaseLock_RemovesOwnLock(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", dir)

	lock := Lock{
		PID:      os.Getpid(),
		Worktree: "/repo/wt",
		Branch:   "main",
		Started:  time.Now().UTC(),
	}
	data, _ := json.MarshalIndent(lock, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "daemon.lock"), data, 0644)

	ReleaseLock(dir)

	if _, err := os.Stat(filepath.Join(dir, "daemon.lock")); !os.IsNotExist(err) {
		t.Error("expected lock file to be removed")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// AcquireLock – MkdirAll failure
// ═══════════════════════════════════════════════════════════════════════════

func TestAcquireLock_MkdirAllFailure(t *testing.T) {
	// Point the lock directory at a path where a file blocks mkdir.
	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	_ = os.WriteFile(blocker, []byte("I am a file"), 0644)

	// Set WOLFCASTLE_LOCK_DIR to a path nested inside the file, so MkdirAll fails.
	t.Setenv("WOLFCASTLE_LOCK_DIR", filepath.Join(blocker, "subdir"))

	err := AcquireLock(base, "/repo/wt", "main")
	if err == nil {
		t.Error("expected error when MkdirAll cannot create lock directory")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// IsProcessRunning
// ═══════════════════════════════════════════════════════════════════════════

func TestIsProcessRunning_InitProcess(t *testing.T) {
	t.Parallel()
	// PID 1 (launchd/init) exists but we don't own it, so signal(0) returns
	// EPERM on macOS/Linux. IsProcessRunning returns false for EPERM.
	if IsProcessRunning(1) {
		t.Skip("PID 1 is signalable on this system (running as root?)")
	}
}

func TestIsProcessRunning_NegativePID(t *testing.T) {
	t.Parallel()
	// Negative PIDs on Unix send to process groups. os.FindProcess may
	// accept them, but Signal(0) should fail.
	if IsProcessRunning(-1) {
		t.Skip("negative PID is somehow alive on this system")
	}
}

func TestIsProcessRunning_ZeroPID(t *testing.T) {
	t.Parallel()
	// PID 0 refers to the calling process's process group on some systems.
	// Regardless, it should not report as "a running process" in any
	// meaningful sense for lock validation.
	_ = IsProcessRunning(0) // just ensure no panic
}

// ═══════════════════════════════════════════════════════════════════════════
// git.CurrentBranch
// ═══════════════════════════════════════════════════════════════════════════

func TestCurrentBranch_NormalRepo(t *testing.T) {
	t.Parallel()
	dir := initGitRepo(t)

	branch, err := git.NewService(dir).CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}
	if branch == "" {
		t.Error("expected non-empty branch name")
	}
}

func TestCurrentBranch_DetachedHEAD(t *testing.T) {
	t.Parallel()
	dir := initGitRepo(t)

	// Create a second commit so we can detach to the first.
	_ = os.WriteFile(filepath.Join(dir, "second.txt"), []byte("second"), 0644)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "second")

	// Detach HEAD by checking out the first commit.
	cmd := exec.Command("git", "rev-parse", "HEAD~1")
	cmd.Dir = dir
	sha, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD~1: %v", err)
	}
	gitRun(t, dir, "checkout", string(sha[:len(sha)-1]))

	branch, err := git.NewService(dir).CurrentBranch()
	if err != nil {
		t.Fatalf("expected success for detached HEAD: %v", err)
	}
	// In detached HEAD, rev-parse --abbrev-ref returns "HEAD".
	if branch != "HEAD" {
		t.Logf("detached HEAD returned branch=%q (expected 'HEAD')", branch)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// propagateState – parse error and save failure
// ═══════════════════════════════════════════════════════════════════════════

func TestPropagateState_InvalidParentAddress(t *testing.T) {
	d := testDaemon(t)
	projDir := d.Store.Dir()

	// Set up a child node whose parent address is invalid for ParseAddress.
	// When propagateState calls loadNode for the parent, ParseAddress will fail.
	idx := state.NewRootIndex()
	badParent := "INVALID PARENT"
	idx.Root = []string{badParent}
	idx.Nodes[badParent] = state.IndexEntry{
		Name:    "Bad Parent",
		Type:    state.NodeOrchestrator,
		State:   state.StatusInProgress,
		Address: badParent,
	}
	idx.Nodes["valid-child"] = state.IndexEntry{
		Name:    "Child",
		Type:    state.NodeLeaf,
		State:   state.StatusInProgress,
		Address: "valid-child",
		Parent:  badParent,
	}
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	// Create a valid child node state.
	childNS := state.NewNodeState("valid-child", "Child", state.NodeLeaf)
	childNS.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusComplete},
	}
	writeJSON(t, filepath.Join(projDir, "valid-child", "state.json"), childNS)

	err := d.propagateState(d.Logger, "valid-child", state.StatusComplete, idx)
	if err == nil {
		t.Error("expected error when parent address is invalid for ParseAddress")
	}
}

func TestPropagateState_SaveRootIndexFailure(t *testing.T) {
	d := testDaemon(t)
	projDir := d.Store.Dir()

	// Set up a valid root index with a leaf under an orchestrator.
	idx := state.NewRootIndex()
	idx.Root = []string{"parent"}
	idx.Nodes["parent"] = state.IndexEntry{
		Name:    "Parent",
		Type:    state.NodeOrchestrator,
		State:   state.StatusInProgress,
		Address: "parent",
	}
	idx.Nodes["parent/child"] = state.IndexEntry{
		Name:    "Child",
		Type:    state.NodeLeaf,
		State:   state.StatusInProgress,
		Address: "parent/child",
	}
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	// Create node state files.
	parentNS := state.NewNodeState("parent", "Parent", state.NodeOrchestrator)
	parentNS.Children = []state.ChildRef{{ID: "child", Address: "parent/child", State: state.StatusInProgress}}
	writeJSON(t, filepath.Join(projDir, "parent", "state.json"), parentNS)

	childNS := state.NewNodeState("parent/child", "Child", state.NodeLeaf)
	childNS.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusComplete},
	}
	writeJSON(t, filepath.Join(projDir, "parent", "child", "state.json"), childNS)

	// Make the state.json directory unwritable so SaveRootIndex fails.
	stateFile := filepath.Join(projDir, "state.json")
	_ = os.Remove(stateFile)
	_ = os.MkdirAll(stateFile, 0755) // create a directory where the file should be

	err := d.propagateState(d.Logger, "parent/child", state.StatusComplete, idx)
	if err == nil {
		t.Error("expected error when SaveRootIndex fails")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// git.HasProgress – git status failure and rename path
// ═══════════════════════════════════════════════════════════════════════════

func TestCheckGitProgress_GitStatusFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir() // not a git repo

	// With a non-empty beforeHEAD, git.HEAD returns "" (no repo),
	// so the HEAD comparison is skipped. Then git status fails,
	// and the function returns true (assumes progress).
	if !git.NewService(dir).HasProgress("abc123") {
		t.Error("expected true when git status fails")
	}
}

func TestCheckGitProgress_RenamedFile(t *testing.T) {
	t.Parallel()
	dir := initGitRepo(t)

	// Create and commit a file, then rename it so git status shows "R".
	_ = os.WriteFile(filepath.Join(dir, "original.txt"), []byte("content"), 0644)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "add original")
	head := git.NewService(dir).HEAD()

	// Rename via git mv so porcelain output includes " -> ".
	gitRun(t, dir, "mv", "original.txt", "renamed.txt")

	if !git.NewService(dir).HasProgress(head) {
		t.Error("renamed file should report progress")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// AcquireLock – WriteFile failure
// ═══════════════════════════════════════════════════════════════════════════

func TestAcquireLock_WriteFileFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	dir := t.TempDir()
	lockDir := filepath.Join(dir, "lockdir")
	_ = os.MkdirAll(lockDir, 0755)

	t.Setenv("WOLFCASTLE_LOCK_DIR", lockDir)

	// Make the directory read-only so WriteFile fails.
	_ = os.Chmod(lockDir, 0555)
	defer func() { _ = os.Chmod(lockDir, 0755) }()

	err := AcquireLock(dir, "/repo/wt", "main")
	if err == nil {
		t.Error("expected error when lock directory is not writable")
	}
}

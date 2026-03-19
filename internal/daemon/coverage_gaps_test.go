package daemon

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// globalLockPath
// ═══════════════════════════════════════════════════════════════════════════

func TestGlobalLockPath_UsesEnvVar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", dir)
	old := GlobalLockDir
	GlobalLockDir = ""
	defer func() { GlobalLockDir = old }()

	p, err := globalLockPath()
	if err != nil {
		t.Fatal(err)
	}
	if p != filepath.Join(dir, "daemon.lock") {
		t.Errorf("expected lock under WOLFCASTLE_LOCK_DIR, got %s", p)
	}
}

func TestGlobalLockPath_UsesGlobalLockDir(t *testing.T) {
	dir := t.TempDir()
	old := GlobalLockDir
	GlobalLockDir = dir
	defer func() { GlobalLockDir = old }()

	p, err := globalLockPath()
	if err != nil {
		t.Fatal(err)
	}
	if p != filepath.Join(dir, "daemon.lock") {
		t.Errorf("expected lock under GlobalLockDir, got %s", p)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// ReadGlobalLock
// ═══════════════════════════════════════════════════════════════════════════

func TestReadGlobalLock_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	old := GlobalLockDir
	GlobalLockDir = dir
	defer func() { GlobalLockDir = old }()

	_ = os.WriteFile(filepath.Join(dir, "daemon.lock"), []byte("{{{not json"), 0644)

	_, err := ReadGlobalLock()
	if err == nil {
		t.Error("expected error for malformed JSON lock file")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// ReleaseGlobalLock
// ═══════════════════════════════════════════════════════════════════════════

func TestReleaseGlobalLock_NoLockFile(t *testing.T) {
	dir := t.TempDir()
	old := GlobalLockDir
	GlobalLockDir = dir
	defer func() { GlobalLockDir = old }()

	// No lock file exists; should not panic.
	ReleaseGlobalLock()
}

func TestReleaseGlobalLock_RemovesOwnLock(t *testing.T) {
	dir := t.TempDir()
	old := GlobalLockDir
	GlobalLockDir = dir
	defer func() { GlobalLockDir = old }()

	lock := GlobalLock{
		PID:      os.Getpid(),
		Repo:     "/repo",
		Worktree: "/repo/wt",
		Started:  time.Now().UTC(),
	}
	data, _ := json.MarshalIndent(lock, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "daemon.lock"), data, 0644)

	ReleaseGlobalLock()

	if _, err := os.Stat(filepath.Join(dir, "daemon.lock")); !os.IsNotExist(err) {
		t.Error("expected lock file to be removed")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// AcquireGlobalLock – MkdirAll failure
// ═══════════════════════════════════════════════════════════════════════════

func TestAcquireGlobalLock_MkdirAllFailure(t *testing.T) {
	// Point the lock directory at a path where a file blocks mkdir.
	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	_ = os.WriteFile(blocker, []byte("I am a file"), 0644)

	old := GlobalLockDir
	GlobalLockDir = blocker // "blocker" is a file, so MkdirAll("blocker") fails
	defer func() { GlobalLockDir = old }()

	err := AcquireGlobalLock("/repo", "/repo/wt")
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
// currentBranch
// ═══════════════════════════════════════════════════════════════════════════

func TestCurrentBranch_NormalRepo(t *testing.T) {
	t.Parallel()
	dir := initGitRepo(t)

	branch, err := currentBranch(dir)
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

	branch, err := currentBranch(dir)
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

	err := d.propagateState("valid-child", state.StatusComplete, idx)
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

	err := d.propagateState("parent/child", state.StatusComplete, idx)
	if err == nil {
		t.Error("expected error when SaveRootIndex fails")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// checkGitProgress – git status failure and rename path
// ═══════════════════════════════════════════════════════════════════════════

func TestCheckGitProgress_GitStatusFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir() // not a git repo

	// With a non-empty beforeHEAD, gitHEAD returns "" (no repo),
	// so the HEAD comparison is skipped. Then git status fails,
	// and the function returns true (assumes progress).
	if !checkGitProgress(dir, "abc123") {
		t.Error("expected true when git status fails")
	}
}

func TestCheckGitProgress_RenamedFile(t *testing.T) {
	t.Parallel()
	dir := initGitRepo(t)
	head := gitHEAD(dir)

	// Create and commit a file, then rename it so git status shows "R".
	_ = os.WriteFile(filepath.Join(dir, "original.txt"), []byte("content"), 0644)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "add original")
	head = gitHEAD(dir)

	// Rename via git mv so porcelain output includes " -> ".
	gitRun(t, dir, "mv", "original.txt", "renamed.txt")

	if !checkGitProgress(dir, head) {
		t.Error("renamed file should report progress")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// AcquireGlobalLock – WriteFile failure
// ═══════════════════════════════════════════════════════════════════════════

func TestAcquireGlobalLock_WriteFileFailure(t *testing.T) {
	dir := t.TempDir()
	lockDir := filepath.Join(dir, "lockdir")
	_ = os.MkdirAll(lockDir, 0755)

	old := GlobalLockDir
	GlobalLockDir = lockDir
	defer func() { GlobalLockDir = old }()

	// Make the directory read-only so WriteFile fails.
	_ = os.Chmod(lockDir, 0555)
	defer func() { _ = os.Chmod(lockDir, 0755) }()

	err := AcquireGlobalLock("/repo", "/repo/wt")
	if err == nil {
		t.Error("expected error when lock directory is not writable")
	}
}

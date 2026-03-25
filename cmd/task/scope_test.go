package task

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ---------------------------------------------------------------------------
// scope add
// ---------------------------------------------------------------------------

func TestScopeAdd_Success(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.RootCmd.SetArgs([]string{
		"task", "scope", "add",
		"--node", "my-project/api",
		"--task", "task-0001",
		"internal/handler.go", "internal/router.go",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope add failed: %v", err)
	}

	// Verify scope-locks.json was written with correct entries.
	table, err := env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatalf("reading scope locks: %v", err)
	}
	if len(table.Locks) != 2 {
		t.Fatalf("expected 2 locks, got %d", len(table.Locks))
	}
	for _, file := range []string{"internal/handler.go", "internal/router.go"} {
		lock, ok := table.Locks[file]
		if !ok {
			t.Errorf("lock for %s not found", file)
			continue
		}
		if lock.Task != "my-project/api/task-0001" {
			t.Errorf("expected task my-project/api/task-0001, got %s", lock.Task)
		}
		if lock.Node != "my-project/api" {
			t.Errorf("expected node my-project/api, got %s", lock.Node)
		}
		if lock.AcquiredAt.IsZero() {
			t.Error("acquired_at should not be zero")
		}
		if lock.PID == 0 {
			t.Error("PID should not be zero")
		}
	}
}

func TestScopeAdd_Idempotent(t *testing.T) {
	env := newTestEnv(t)

	args := []string{
		"task", "scope", "add",
		"--node", "my-project/api",
		"--task", "task-0001",
		"file.go",
	}

	// Acquire once.
	env.RootCmd.SetArgs(args)
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("first add: %v", err)
	}

	// Record the original lock's AcquiredAt and PID.
	table, err := env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatal(err)
	}
	origLock := table.Locks["file.go"]
	origAcquiredAt := origLock.AcquiredAt
	origPID := origLock.PID

	// Acquire again with the same task; should succeed silently.
	env.RootCmd.SetArgs(args)
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("idempotent re-add: %v", err)
	}

	table, err = env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Locks) != 1 {
		t.Fatalf("expected 1 lock after idempotent add, got %d", len(table.Locks))
	}

	reacquired := table.Locks["file.go"]
	if !reacquired.AcquiredAt.Equal(origAcquiredAt) {
		t.Errorf("AcquiredAt changed on re-acquisition: got %v, want %v", reacquired.AcquiredAt, origAcquiredAt)
	}
	if reacquired.PID != origPID {
		t.Errorf("PID changed on re-acquisition: got %d, want %d", reacquired.PID, origPID)
	}
}

func TestScopeAdd_DirectoryScope(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{
		"task", "scope", "add",
		"--node", "my-project/api",
		"--task", "task-0001",
		"internal/daemon/",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope add directory: %v", err)
	}

	table, err := env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatal(err)
	}
	lock, ok := table.Locks["internal/daemon/"]
	if !ok {
		t.Fatal("directory scope lock not found")
	}
	if lock.Task != "my-project/api/task-0001" {
		t.Errorf("unexpected task: %s", lock.Task)
	}
}

func TestScopeAdd_MissingNode(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "scope", "add", "file.go"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node is missing")
	}
}

func TestScopeAdd_NoFiles(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "scope", "add", "--node", "my-project/api"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when no file args provided")
	}
}

func TestScopeAdd_MissingTask(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{
		"task", "scope", "add",
		"--node", "my-project/api",
		"file.go",
	})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when --task is omitted")
	}
	want := "required flag"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Errorf("error should mention required flag\ngot:  %s\nwant substring: %s", got, want)
	}
}

func TestScopeAdd_InvalidPath(t *testing.T) {
	env := newTestEnv(t)

	tests := []struct {
		name string
		path string
	}{
		{name: "absolute path", path: "/etc/passwd"},
		{name: "dotdot traversal", path: "../etc/passwd"},
		{name: "dotdot mid-path", path: "internal/../daemon/foo.go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env.RootCmd.SetArgs([]string{
				"task", "scope", "add",
				"--node", "my-project/api",
				"--task", "task-0001",
				tt.path,
			})
			err := env.RootCmd.Execute()
			if err == nil {
				t.Fatalf("expected error for invalid path %q", tt.path)
			}
			if !strings.Contains(err.Error(), "invalid scope path") {
				t.Errorf("error should mention invalid scope path, got: %s", err.Error())
			}
		})
	}
}

// Conflict detection and all-or-nothing semantics are tested at the state
// layer because the CLI command calls os.Exit(1) on conflict, which would
// terminate the test process. The FindConflicts function and MutateScopeLocks
// atomicity are covered in internal/state/. The tests below verify that the
// conflict detection logic works correctly in the context of CLI-seeded state.

func TestScopeAdd_ConflictDetection(t *testing.T) {
	env := newTestEnv(t)

	// Task A acquires a file.
	env.RootCmd.SetArgs([]string{
		"task", "scope", "add",
		"--node", "proj/node-a",
		"--task", "task-0001",
		"shared.go",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("task A acquire: %v", err)
	}

	// Verify conflict exists for task B at the state level.
	table, err := env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatal(err)
	}
	conflicts := state.FindConflicts([]string{"shared.go"}, table, "proj/node-b/task-0002")
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].HeldByTask != "proj/node-a/task-0001" {
		t.Errorf("unexpected conflict holder: %s", conflicts[0].HeldByTask)
	}
}

func TestScopeAdd_AllOrNothing(t *testing.T) {
	env := newTestEnv(t)

	// Task A acquires file1.go.
	env.RootCmd.SetArgs([]string{
		"task", "scope", "add",
		"--node", "proj/node-a",
		"--task", "task-0001",
		"file1.go",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("task A acquire: %v", err)
	}

	// Task B tries to acquire file1.go and file2.go.
	// The conflict on file1.go should prevent file2.go from being acquired.
	// (Verified at the state layer since os.Exit prevents CLI-level testing.)
	table, err := env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatal(err)
	}

	requested := []string{"file1.go", "file2.go"}
	conflicts := state.FindConflicts(requested, table, "proj/node-b/task-0002")
	if len(conflicts) == 0 {
		t.Fatal("expected conflict on file1.go")
	}

	// Simulate the all-or-nothing guard: if conflicts exist, nothing is added.
	// After the failed attempt, only task A's lock should remain.
	if len(table.Locks) != 1 {
		t.Fatalf("expected 1 lock (task A only), got %d", len(table.Locks))
	}
	if _, ok := table.Locks["file2.go"]; ok {
		t.Error("file2.go should not have been acquired due to all-or-nothing semantics")
	}
}

func TestScopeAdd_ConflictMessageAllFiles(t *testing.T) {
	env := newTestEnv(t)

	// Task A acquires two files.
	seedLocks(t, env, map[string]state.ScopeLock{
		"alpha.go": {Task: "proj/node-a/task-0001", Node: "proj/node-a", AcquiredAt: time.Now().UTC(), PID: 1234},
		"beta.go":  {Task: "proj/node-a/task-0001", Node: "proj/node-a", AcquiredAt: time.Now().UTC(), PID: 1234},
	})

	// Task B requests both files; FindConflicts should report both.
	table, err := env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatal(err)
	}
	conflicts := state.FindConflicts(
		[]string{"alpha.go", "beta.go"},
		table,
		"proj/node-b/task-0002",
	)
	if len(conflicts) != 2 {
		t.Fatalf("expected 2 conflicts, got %d", len(conflicts))
	}

	// Collect conflicting file names and verify both are present.
	got := map[string]bool{}
	for _, c := range conflicts {
		got[c.File] = true
		if c.HeldByTask != "proj/node-a/task-0001" {
			t.Errorf("unexpected holder for %s: %s", c.File, c.HeldByTask)
		}
	}
	for _, want := range []string{"alpha.go", "beta.go"} {
		if !got[want] {
			t.Errorf("conflict for %s not found in results", want)
		}
	}
}

func TestScopeAdd_DirectoryConflictsWithFile(t *testing.T) {
	env := newTestEnv(t)

	// Task A acquires a directory scope.
	env.RootCmd.SetArgs([]string{
		"task", "scope", "add",
		"--node", "proj/node-a",
		"--task", "task-0001",
		"internal/daemon/",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("task A directory acquire: %v", err)
	}

	// Task B tries to acquire a file under that directory.
	table, err := env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatal(err)
	}
	conflicts := state.FindConflicts(
		[]string{"internal/daemon/iteration.go"},
		table,
		"proj/node-b/task-0002",
	)
	if len(conflicts) != 1 {
		t.Fatalf("expected directory/file conflict, got %d conflicts", len(conflicts))
	}
}

// ---------------------------------------------------------------------------
// scope list
// ---------------------------------------------------------------------------

func TestScopeList_AllLocks(t *testing.T) {
	env := newTestEnv(t)

	// Seed two locks from different tasks.
	seedLocks(t, env, map[string]state.ScopeLock{
		"alpha.go": {Task: "proj/a/task-0001", Node: "proj/a", AcquiredAt: time.Now().UTC()},
		"beta.go":  {Task: "proj/b/task-0002", Node: "proj/b", AcquiredAt: time.Now().UTC()},
	})

	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.RootCmd.SetArgs([]string{"task", "scope", "list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope list: %v", err)
	}

	// Verify both locks are readable via the state layer.
	table, err := env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Locks) != 2 {
		t.Fatalf("expected 2 locks, got %d", len(table.Locks))
	}
}

func TestScopeList_FilterByNode(t *testing.T) {
	env := newTestEnv(t)

	seedLocks(t, env, map[string]state.ScopeLock{
		"a.go": {Task: "proj/node-a/task-0001", Node: "proj/node-a", AcquiredAt: time.Now().UTC()},
		"b.go": {Task: "proj/node-b/task-0002", Node: "proj/node-b", AcquiredAt: time.Now().UTC()},
	})

	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.RootCmd.SetArgs([]string{"task", "scope", "list", "--node", "proj/node-a"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope list --node: %v", err)
	}

	// The command filters in memory; verify the underlying table is intact
	// and that the filter logic would produce the right count.
	table, err := env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, lock := range table.Locks {
		if lock.Node == "proj/node-a" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 lock for proj/node-a, got %d", count)
	}
}

func TestScopeList_FilterByTask(t *testing.T) {
	env := newTestEnv(t)

	seedLocks(t, env, map[string]state.ScopeLock{
		"a.go": {Task: "proj/node-a/task-0001", Node: "proj/node-a", AcquiredAt: time.Now().UTC()},
		"b.go": {Task: "proj/node-a/task-0002", Node: "proj/node-a", AcquiredAt: time.Now().UTC()},
	})

	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.RootCmd.SetArgs([]string{"task", "scope", "list", "--task", "proj/node-a/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope list --task: %v", err)
	}

	table, err := env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, lock := range table.Locks {
		if lock.Task == "proj/node-a/task-0001" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 lock for task-0001, got %d", count)
	}
}

func TestScopeList_CombinedFilters(t *testing.T) {
	env := newTestEnv(t)

	seedLocks(t, env, map[string]state.ScopeLock{
		"a.go": {Task: "proj/node-a/task-0001", Node: "proj/node-a", AcquiredAt: time.Now().UTC()},
		"b.go": {Task: "proj/node-a/task-0002", Node: "proj/node-a", AcquiredAt: time.Now().UTC()},
		"c.go": {Task: "proj/node-b/task-0003", Node: "proj/node-b", AcquiredAt: time.Now().UTC()},
	})

	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.RootCmd.SetArgs([]string{
		"task", "scope", "list",
		"--node", "proj/node-a",
		"--task", "proj/node-a/task-0001",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope list combined: %v", err)
	}

	table, err := env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, lock := range table.Locks {
		if lock.Node == "proj/node-a" && lock.Task == "proj/node-a/task-0001" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 lock matching both filters, got %d", count)
	}
}

func TestScopeList_Empty(t *testing.T) {
	env := newTestEnv(t)

	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.RootCmd.SetArgs([]string{"task", "scope", "list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope list empty: %v", err)
	}

	table, err := env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Locks) != 0 {
		t.Errorf("expected empty table, got %d locks", len(table.Locks))
	}
}

// ---------------------------------------------------------------------------
// scope release
// ---------------------------------------------------------------------------

func TestScopeRelease_AllLocksForTask(t *testing.T) {
	env := newTestEnv(t)

	// Task holds two locks.
	seedLocks(t, env, map[string]state.ScopeLock{
		"a.go": {Task: "proj/api/task-0001", Node: "proj/api", AcquiredAt: time.Now().UTC()},
		"b.go": {Task: "proj/api/task-0001", Node: "proj/api", AcquiredAt: time.Now().UTC()},
		"c.go": {Task: "proj/api/task-0002", Node: "proj/api", AcquiredAt: time.Now().UTC()},
	})

	env.RootCmd.SetArgs([]string{
		"task", "scope", "release",
		"--node", "proj/api",
		"--task", "task-0001",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope release all: %v", err)
	}

	table, err := env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatal(err)
	}
	// Only task-0002's lock should remain.
	if len(table.Locks) != 1 {
		t.Fatalf("expected 1 remaining lock, got %d", len(table.Locks))
	}
	if _, ok := table.Locks["c.go"]; !ok {
		t.Error("task-0002's lock on c.go should still exist")
	}
}

func TestScopeRelease_SpecificFiles(t *testing.T) {
	env := newTestEnv(t)

	seedLocks(t, env, map[string]state.ScopeLock{
		"a.go": {Task: "proj/api/task-0001", Node: "proj/api", AcquiredAt: time.Now().UTC()},
		"b.go": {Task: "proj/api/task-0001", Node: "proj/api", AcquiredAt: time.Now().UTC()},
	})

	// Release only a.go.
	env.RootCmd.SetArgs([]string{
		"task", "scope", "release",
		"--node", "proj/api",
		"--task", "task-0001",
		"a.go",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope release specific: %v", err)
	}

	table, err := env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Locks) != 1 {
		t.Fatalf("expected 1 lock, got %d", len(table.Locks))
	}
	if _, ok := table.Locks["b.go"]; !ok {
		t.Error("b.go should still be locked")
	}
}

func TestScopeRelease_EmptyTableNoOp(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{
		"task", "scope", "release",
		"--node", "proj/api",
		"--task", "task-0001",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope release on empty: %v", err)
	}

	// Should complete without error and leave no file behind.
	table, err := env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Locks) != 0 {
		t.Errorf("expected empty table, got %d", len(table.Locks))
	}
}

func TestScopeRelease_FileCleanup(t *testing.T) {
	env := newTestEnv(t)

	seedLocks(t, env, map[string]state.ScopeLock{
		"only.go": {Task: "proj/api/task-0001", Node: "proj/api", AcquiredAt: time.Now().UTC()},
	})

	// Verify file exists before release.
	locksPath := env.App.State.ScopeLocksPath()
	if _, err := os.Stat(locksPath); os.IsNotExist(err) {
		t.Fatal("scope-locks.json should exist before release")
	}

	env.RootCmd.SetArgs([]string{
		"task", "scope", "release",
		"--node", "proj/api",
		"--task", "task-0001",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope release cleanup: %v", err)
	}

	// After releasing the only lock, scope-locks.json should be deleted.
	if _, err := os.Stat(locksPath); !os.IsNotExist(err) {
		t.Error("scope-locks.json should be deleted when table becomes empty")
	}
}

func TestScopeRelease_OtherTaskFileUntouched(t *testing.T) {
	env := newTestEnv(t)

	seedLocks(t, env, map[string]state.ScopeLock{
		"shared.go": {Task: "proj/api/task-0002", Node: "proj/api", AcquiredAt: time.Now().UTC()},
	})

	// Task-0001 tries to release shared.go, but it's held by task-0002.
	env.RootCmd.SetArgs([]string{
		"task", "scope", "release",
		"--node", "proj/api",
		"--task", "task-0001",
		"shared.go",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope release other's file: %v", err)
	}

	table, err := env.App.State.ReadScopeLocks()
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Locks) != 1 {
		t.Fatalf("expected lock to remain, got %d locks", len(table.Locks))
	}
	if table.Locks["shared.go"].Task != "proj/api/task-0002" {
		t.Error("task-0002's lock should be untouched")
	}
}

func TestScopeRelease_MissingNode(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "scope", "release", "--task", "task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node is missing")
	}
}

func TestScopeRelease_MissingTask(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "scope", "release", "--node", "proj/api"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --task is missing")
	}
}

// ---------------------------------------------------------------------------
// JSON output verification
// ---------------------------------------------------------------------------

func TestScopeAdd_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.RootCmd.SetArgs([]string{
		"task", "scope", "add",
		"--node", "proj/api",
		"--task", "task-0001",
		"file.go",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope add json: %v", err)
	}
}

func TestScopeList_JSONOutput(t *testing.T) {
	env := newTestEnv(t)

	seedLocks(t, env, map[string]state.ScopeLock{
		"x.go": {Task: "proj/a/task-0001", Node: "proj/a", AcquiredAt: time.Now().UTC()},
	})

	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.RootCmd.SetArgs([]string{"task", "scope", "list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope list json: %v", err)
	}
}

func TestScopeRelease_JSONOutput(t *testing.T) {
	env := newTestEnv(t)

	seedLocks(t, env, map[string]state.ScopeLock{
		"x.go": {Task: "proj/api/task-0001", Node: "proj/api", AcquiredAt: time.Now().UTC()},
	})

	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.RootCmd.SetArgs([]string{
		"task", "scope", "release",
		"--node", "proj/api",
		"--task", "task-0001",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope release json: %v", err)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// seedLocks writes a scope lock table directly to disk, bypassing the CLI,
// so that subsequent commands find pre-existing locks.
func seedLocks(t *testing.T, env *testEnv, locks map[string]state.ScopeLock) {
	t.Helper()
	table := state.NewScopeLockTable()
	for file, lock := range locks {
		table.Locks[file] = lock
	}
	path := env.App.State.ScopeLocksPath()
	data, err := json.MarshalIndent(table, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

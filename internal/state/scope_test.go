package state

import (
	"errors"
	"testing"
	"time"
)

func TestScopeConflicts(t *testing.T) {
	tests := []struct {
		name      string
		requested string
		existing  string
		want      bool
	}{
		{name: "identical files", requested: "internal/daemon/iteration.go", existing: "internal/daemon/iteration.go", want: true},
		{name: "identical dirs", requested: "internal/daemon/", existing: "internal/daemon/", want: true},
		{name: "dir contains file", requested: "internal/daemon/", existing: "internal/daemon/iteration.go", want: true},
		{name: "file inside dir", requested: "internal/daemon/iteration.go", existing: "internal/daemon/", want: true},
		{name: "parent contains child dir", requested: "internal/", existing: "internal/daemon/", want: true},
		{name: "child inside parent", requested: "internal/daemon/", existing: "internal/", want: true},
		{name: "parent dir contains nested file", requested: "internal/", existing: "internal/daemon/iteration.go", want: true},
		{name: "nested file inside parent dir", requested: "internal/daemon/iteration.go", existing: "internal/", want: true},
		{name: "non-overlapping files", requested: "internal/daemon/iteration.go", existing: "internal/state/types.go", want: false},
		{name: "non-overlapping dirs", requested: "internal/daemon/", existing: "internal/state/", want: false},
		{name: "file vs unrelated dir", requested: "cmd/main.go", existing: "internal/", want: false},
		{name: "partial name overlap not conflict", requested: "internal/daemontools/", existing: "internal/daemon/", want: false},
		{name: "file without slash not dir", requested: "internal/daemon", existing: "internal/daemon/iteration.go", want: false},
		{name: "file prefix not conflict", requested: "foo.go", existing: "foo.go.bak", want: false},

		// Validation: invalid paths never conflict.
		{name: "empty requested", requested: "", existing: "internal/daemon/", want: false},
		{name: "empty existing", requested: "internal/daemon/", existing: "", want: false},
		{name: "both empty", requested: "", existing: "", want: false},
		{name: "dotdot in requested", requested: "../etc/passwd", existing: "internal/", want: false},
		{name: "dotdot in existing", requested: "internal/", existing: "foo/../bar", want: false},
		{name: "dotdot mid-path requested", requested: "internal/../daemon/", existing: "internal/daemon/", want: false},
		{name: "absolute requested", requested: "/etc/passwd", existing: "internal/", want: false},
		{name: "absolute existing", requested: "internal/", existing: "/etc/passwd", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScopeConflicts(tt.requested, tt.existing)
			if got != tt.want {
				t.Errorf("ScopeConflicts(%q, %q) = %v, want %v", tt.requested, tt.existing, got, tt.want)
			}
		})
	}
}

func TestValidateScopePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "simple file", path: "foo.go", want: true},
		{name: "nested file", path: "internal/daemon/iteration.go", want: true},
		{name: "directory", path: "internal/daemon/", want: true},
		{name: "empty", path: "", want: false},
		{name: "absolute path", path: "/etc/passwd", want: false},
		{name: "dotdot at start", path: "../foo.go", want: false},
		{name: "dotdot mid-path", path: "internal/../daemon/foo.go", want: false},
		{name: "dotdot at end", path: "internal/..", want: false},
		{name: "dotdot only", path: "..", want: false},
		{name: "single dot is fine", path: "internal/./foo.go", want: true},
		{name: "dotdot-like name is fine", path: "internal/...foo/bar.go", want: true},
		{name: "double slash", path: "internal//daemon/foo.go", want: false},
		{name: "trailing double slash", path: "internal/daemon//", want: false},
		{name: "leading double slash relative", path: "//foo.go", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateScopePath(tt.path)
			if got != tt.want {
				t.Errorf("ValidateScopePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestFindConflicts(t *testing.T) {
	now := time.Now()
	table := &ScopeLockTable{
		Version: 1,
		Locks: map[string]ScopeLock{
			"internal/daemon/iteration.go": {Task: "proj/node-a/task-0001", Node: "proj/node-a", AcquiredAt: now, PID: 1234},
			"internal/state/":              {Task: "proj/node-b/task-0002", Node: "proj/node-b", AcquiredAt: now, PID: 1234},
			"cmd/main.go":                  {Task: "proj/node-a/task-0001", Node: "proj/node-a", AcquiredAt: now, PID: 1234},
		},
	}

	t.Run("no conflicts", func(t *testing.T) {
		conflicts := FindConflicts([]string{"README.md"}, table, "proj/node-c/task-0003")
		if len(conflicts) != 0 {
			t.Errorf("expected no conflicts, got %d", len(conflicts))
		}
	})

	t.Run("conflict with file lock", func(t *testing.T) {
		conflicts := FindConflicts([]string{"internal/daemon/iteration.go"}, table, "proj/node-c/task-0003")
		if len(conflicts) != 1 {
			t.Fatalf("expected 1 conflict, got %d", len(conflicts))
		}
		c := conflicts[0]
		if c.File != "internal/daemon/iteration.go" {
			t.Errorf("conflict file = %q, want %q", c.File, "internal/daemon/iteration.go")
		}
		if c.HeldByTask != "proj/node-a/task-0001" {
			t.Errorf("held_by_task = %q, want %q", c.HeldByTask, "proj/node-a/task-0001")
		}
		if c.HeldByNode != "proj/node-a" {
			t.Errorf("held_by_node = %q, want %q", c.HeldByNode, "proj/node-a")
		}
	})

	t.Run("conflict with dir lock", func(t *testing.T) {
		conflicts := FindConflicts([]string{"internal/state/types.go"}, table, "proj/node-c/task-0003")
		if len(conflicts) != 1 {
			t.Fatalf("expected 1 conflict, got %d", len(conflicts))
		}
		if conflicts[0].HeldByTask != "proj/node-b/task-0002" {
			t.Errorf("held_by_task = %q, want %q", conflicts[0].HeldByTask, "proj/node-b/task-0002")
		}
	})

	t.Run("skip own locks", func(t *testing.T) {
		conflicts := FindConflicts([]string{"internal/daemon/iteration.go", "cmd/main.go"}, table, "proj/node-a/task-0001")
		if len(conflicts) != 0 {
			t.Errorf("expected no conflicts for own locks, got %d", len(conflicts))
		}
	})

	t.Run("multiple conflicts", func(t *testing.T) {
		conflicts := FindConflicts([]string{"internal/daemon/iteration.go", "internal/state/scope.go"}, table, "proj/node-c/task-0003")
		if len(conflicts) != 2 {
			t.Fatalf("expected 2 conflicts, got %d", len(conflicts))
		}
	})

	t.Run("empty table", func(t *testing.T) {
		empty := NewScopeLockTable()
		conflicts := FindConflicts([]string{"anything.go"}, empty, "proj/node-a/task-0001")
		if len(conflicts) != 0 {
			t.Errorf("expected no conflicts on empty table, got %d", len(conflicts))
		}
	})

	t.Run("empty requested", func(t *testing.T) {
		conflicts := FindConflicts([]string{}, table, "proj/node-c/task-0003")
		if len(conflicts) != 0 {
			t.Errorf("expected no conflicts for empty request, got %d", len(conflicts))
		}
	})

	t.Run("invalid paths skipped", func(t *testing.T) {
		conflicts := FindConflicts(
			[]string{"", "../etc/passwd", "/absolute/path", "internal/daemon/iteration.go"},
			table, "proj/node-c/task-0003",
		)
		if len(conflicts) != 1 {
			t.Fatalf("expected 1 conflict (only valid path), got %d", len(conflicts))
		}
		if conflicts[0].File != "internal/daemon/iteration.go" {
			t.Errorf("conflict file = %q, want %q", conflicts[0].File, "internal/daemon/iteration.go")
		}
	})
}

// TestMutateScopeLocks_AllOrNothing exercises the all-or-nothing guarantee at
// the persistence layer. When a batch of files is requested and one conflicts
// with an existing lock, the mutation callback returns an error, aborting the
// write. Re-reading the table afterward confirms that none of the non-
// conflicting files leaked through.
func TestMutateScopeLocks_AllOrNothing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, 5*time.Second)

	now := time.Now().UTC()
	taskA := "proj/node-a/task-0001"
	taskB := "proj/node-b/task-0002"

	// Seed: task-A holds file1.go.
	err := s.MutateScopeLocks(func(tbl *ScopeLockTable) error {
		tbl.Locks["file1.go"] = ScopeLock{
			Task:       taskA,
			Node:       "proj/node-a",
			AcquiredAt: now,
			PID:        1000,
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seeding task-A lock: %v", err)
	}

	// Task-B requests [file1.go, file2.go, file3.go]. file1.go conflicts.
	// The callback detects the conflict and returns an error, aborting the
	// entire mutation so that file2.go and file3.go are never persisted.
	errConflict := errors.New("scope conflict: all-or-nothing abort")
	requested := []string{"file1.go", "file2.go", "file3.go"}

	err = s.MutateScopeLocks(func(tbl *ScopeLockTable) error {
		conflicts := FindConflicts(requested, tbl, taskB)
		if len(conflicts) > 0 {
			return errConflict
		}
		for _, f := range requested {
			tbl.Locks[f] = ScopeLock{
				Task:       taskB,
				Node:       "proj/node-b",
				AcquiredAt: now,
				PID:        2000,
			}
		}
		return nil
	})
	if !errors.Is(err, errConflict) {
		t.Fatalf("expected conflict error, got %v", err)
	}

	// Re-read and verify: only task-A's file1.go lock should exist.
	tbl, err := s.ReadScopeLocks()
	if err != nil {
		t.Fatalf("reading scope locks after abort: %v", err)
	}

	if len(tbl.Locks) != 1 {
		t.Errorf("expected exactly 1 lock after aborted batch, got %d", len(tbl.Locks))
	}
	for _, leaked := range []string{"file2.go", "file3.go"} {
		if _, ok := tbl.Locks[leaked]; ok {
			t.Errorf("%s was acquired despite conflict on file1.go; all-or-nothing violated", leaked)
		}
	}
	if lock, ok := tbl.Locks["file1.go"]; !ok {
		t.Error("task-A's file1.go lock should still be present")
	} else if lock.Task != taskA {
		t.Errorf("file1.go holder = %q, want %q", lock.Task, taskA)
	}
}

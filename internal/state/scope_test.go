package state

import (
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
}

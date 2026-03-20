package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestIndexPath_ReturnsExpectedPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	got := s.IndexPath()
	want := filepath.Join(dir, "state.json")
	if got != want {
		t.Errorf("IndexPath() = %q, want %q", got, want)
	}
}

func TestNodePath_ValidAddress(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	got, err := s.NodePath("root/child")
	if err != nil {
		t.Fatalf("NodePath: unexpected error: %v", err)
	}
	want := filepath.Join(dir, "root", "child", "state.json")
	if got != want {
		t.Errorf("NodePath() = %q, want %q", got, want)
	}
}

func TestNodePath_EmptyAddress_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	_, err := s.NodePath("")
	if err == nil {
		t.Error("NodePath with empty address: expected error, got nil")
	}
}

func TestInboxPath_ReturnsExpectedPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	got := s.InboxPath()
	want := filepath.Join(dir, "inbox.json")
	if got != want {
		t.Errorf("InboxPath() = %q, want %q", got, want)
	}
}

func TestParentTaskID_WithDot(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"task-0001.0002", "task-0001"},
		{"a.b.c", "a.b"},
		{"no-dot", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parentTaskID(tt.input)
			if got != tt.want {
				t.Errorf("parentTaskID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

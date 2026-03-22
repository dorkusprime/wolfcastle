package logrender

import (
	"os"
	"path/filepath"
	"testing"
)

// touch creates an empty file at the given path.
func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("creating test file %s: %v", path, err)
	}
}

func TestListSessions_SingleSession(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"))
	touch(t, filepath.Join(dir, "0002-20260321T18-05Z.jsonl"))
	touch(t, filepath.Join(dir, "0003-20260321T18-06Z.jsonl"))

	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if len(sessions[0].Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(sessions[0].Files))
	}
}

func TestListSessions_MultipleSessions(t *testing.T) {
	dir := t.TempDir()
	// Session A (older)
	touch(t, filepath.Join(dir, "0001-20260320T10-00Z.jsonl"))
	touch(t, filepath.Join(dir, "0002-20260320T10-01Z.jsonl"))
	// Session B (newer)
	touch(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"))
	touch(t, filepath.Join(dir, "0002-20260321T18-05Z.jsonl"))
	touch(t, filepath.Join(dir, "0003-20260321T18-06Z.jsonl"))

	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	// Most recent first.
	if len(sessions[0].Files) != 3 {
		t.Errorf("latest session: expected 3 files, got %d", len(sessions[0].Files))
	}
	if len(sessions[1].Files) != 2 {
		t.Errorf("older session: expected 2 files, got %d", len(sessions[1].Files))
	}
	// Verify ordering: latest session files should contain the 20260321 date.
	for _, f := range sessions[0].Files {
		base := filepath.Base(f)
		if base[:4] == "0001" && !contains(base, "20260321") {
			t.Errorf("latest session file %s doesn't look like the newer session", base)
		}
	}
}

func TestListSessions_CompressedFiles(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "0001-20260320T10-00Z.jsonl.gz"))
	touch(t, filepath.Join(dir, "0002-20260320T10-01Z.jsonl"))
	touch(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"))

	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if len(sessions[1].Files) != 2 {
		t.Errorf("older session: expected 2 files (one .gz, one .jsonl), got %d", len(sessions[1].Files))
	}
}

func TestListSessions_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSessions_IgnoresNonLogFiles(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"))
	touch(t, filepath.Join(dir, "notes.txt"))
	touch(t, filepath.Join(dir, "config.json"))

	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if len(sessions[0].Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(sessions[0].Files))
	}
}

func TestListSessions_IgnoresSubdirectories(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"))
	if err := os.Mkdir(filepath.Join(dir, "0002-20260321T18-05Z.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}

	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if len(sessions[0].Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(sessions[0].Files))
	}
}

func TestListSessions_NonexistentDir(t *testing.T) {
	_, err := ListSessions("/no/such/directory")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestListSessions_FileOrder(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"))
	touch(t, filepath.Join(dir, "0003-20260321T18-06Z.jsonl"))
	touch(t, filepath.Join(dir, "0002-20260321T18-05Z.jsonl"))

	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	// Files should be in iteration order regardless of filesystem order.
	expected := []string{
		filepath.Join(dir, "0001-20260321T18-04Z.jsonl"),
		filepath.Join(dir, "0002-20260321T18-05Z.jsonl"),
		filepath.Join(dir, "0003-20260321T18-06Z.jsonl"),
	}
	for i, f := range sessions[0].Files {
		if f != expected[i] {
			t.Errorf("file %d: got %s, want %s", i, f, expected[i])
		}
	}
}

func TestListSessions_ThreeSessions(t *testing.T) {
	dir := t.TempDir()
	// Session 1 (oldest)
	touch(t, filepath.Join(dir, "0001-20260319T08-00Z.jsonl.gz"))
	// Session 2
	touch(t, filepath.Join(dir, "0001-20260320T10-00Z.jsonl.gz"))
	touch(t, filepath.Join(dir, "0002-20260320T10-01Z.jsonl.gz"))
	touch(t, filepath.Join(dir, "0003-20260320T10-02Z.jsonl"))
	// Session 3 (newest)
	touch(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"))
	touch(t, filepath.Join(dir, "0002-20260321T18-05Z.jsonl"))

	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}
	if len(sessions[0].Files) != 2 {
		t.Errorf("newest session: expected 2 files, got %d", len(sessions[0].Files))
	}
	if len(sessions[1].Files) != 3 {
		t.Errorf("middle session: expected 3 files, got %d", len(sessions[1].Files))
	}
	if len(sessions[2].Files) != 1 {
		t.Errorf("oldest session: expected 1 file, got %d", len(sessions[2].Files))
	}
}

func TestResolveSession_Latest(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "0001-20260320T10-00Z.jsonl"))
	touch(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"))
	touch(t, filepath.Join(dir, "0002-20260321T18-05Z.jsonl"))

	s, err := ResolveSession(dir, 0)
	if err != nil {
		t.Fatalf("ResolveSession: %v", err)
	}
	if len(s.Files) != 2 {
		t.Errorf("expected 2 files in latest session, got %d", len(s.Files))
	}
}

func TestResolveSession_Previous(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "0001-20260320T10-00Z.jsonl"))
	touch(t, filepath.Join(dir, "0002-20260320T10-01Z.jsonl"))
	touch(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"))

	s, err := ResolveSession(dir, 1)
	if err != nil {
		t.Fatalf("ResolveSession: %v", err)
	}
	if len(s.Files) != 2 {
		t.Errorf("expected 2 files in previous session, got %d", len(s.Files))
	}
}

func TestResolveSession_OutOfRange(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"))

	_, err := ResolveSession(dir, 1)
	if err == nil {
		t.Fatal("expected error for out-of-range index")
	}
}

func TestResolveSession_NegativeIndex(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"))

	_, err := ResolveSession(dir, -1)
	if err == nil {
		t.Fatal("expected error for negative index")
	}
}

func TestResolveSession_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	_, err := ResolveSession(dir, 0)
	if err == nil {
		t.Fatal("expected error for empty directory")
	}
}

func TestParseIteration(t *testing.T) {
	tests := []struct {
		name string
		want int
	}{
		{"0001-20260321T18-04Z.jsonl", 1},
		{"0010-20260321T18-04Z.jsonl", 10},
		{"0123-20260321T18-04Z.jsonl", 123},
		{"9999-20260321T18-04Z.jsonl", 9999},
		{"0001-20260321T18-04Z.jsonl.gz", 1},
		{"abc", 0},
		{"", 0},
		{"00x1-20260321T18-04Z.jsonl", 0},
	}
	for _, tt := range tests {
		got := parseIteration(tt.name)
		if got != tt.want {
			t.Errorf("parseIteration(%q) = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestTimestampKey_ExtractsAfterHyphen(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"0001-20260321T18-04Z.jsonl", "20260321T18-04Z.jsonl"},
		{"0010-20260320T10-00Z.jsonl.gz", "20260320T10-00Z.jsonl.gz"},
		{"nohyphen", "nohyphen"},
		{"a-b", "b"},
	}
	for _, tt := range tests {
		got := timestampKey(tt.input)
		if got != tt.want {
			t.Errorf("timestampKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// contains checks whether s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

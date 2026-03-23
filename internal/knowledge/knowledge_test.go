package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilePath(t *testing.T) {
	t.Parallel()
	got := FilePath("/project/.wolfcastle", "wild-macbook-pro")
	want := filepath.Join("/project/.wolfcastle", "docs", "knowledge", "wild-macbook-pro.md")
	if got != want {
		t.Errorf("FilePath = %q, want %q", got, want)
	}
}

func TestRead_MissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content, err := Read(dir, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty string for missing file, got %q", content)
	}
}

func TestRead_ExistingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := FilePath(dir, "test")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	want := "- some knowledge entry\n"
	if err := os.WriteFile(p, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Read(dir, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("Read = %q, want %q", got, want)
	}
}

func TestRead_PermissionError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := FilePath(dir, "test")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })

	_, err := Read(dir, "test")
	if err == nil {
		t.Fatal("expected permission error, got nil")
	}
}

func TestAppend_CreatesFileAndDirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if err := Append(dir, "new-ns", "first entry"); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	got, err := os.ReadFile(FilePath(dir, "new-ns"))
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(got) != "- first entry\n" {
		t.Errorf("file content = %q, want %q", string(got), "- first entry\n")
	}
}

func TestAppend_PreservesPrefix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if err := Append(dir, "ns", "- already prefixed"); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	got, err := os.ReadFile(FilePath(dir, "ns"))
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(got) != "- already prefixed\n" {
		t.Errorf("file content = %q, want %q", string(got), "- already prefixed\n")
	}
}

func TestAppend_PreservesTrailingNewline(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if err := Append(dir, "ns", "- has newline\n"); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	got, err := os.ReadFile(FilePath(dir, "ns"))
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(got) != "- has newline\n" {
		t.Errorf("file content = %q, want %q", string(got), "- has newline\n")
	}
}

func TestAppend_MultipleEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	entries := []string{"first entry", "second entry", "- third entry"}
	for _, e := range entries {
		if err := Append(dir, "multi", e); err != nil {
			t.Fatalf("Append(%q) failed: %v", e, err)
		}
	}

	got, err := os.ReadFile(FilePath(dir, "multi"))
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	want := "- first entry\n- second entry\n- third entry\n"
	if string(got) != want {
		t.Errorf("file content = %q, want %q", string(got), want)
	}
}

func TestTokenCount_Empty(t *testing.T) {
	t.Parallel()
	if got := TokenCount(""); got != 0 {
		t.Errorf("TokenCount(\"\") = %d, want 0", got)
	}
}

func TestTokenCount_Whitespace(t *testing.T) {
	t.Parallel()
	if got := TokenCount("   \n\t  "); got != 0 {
		t.Errorf("TokenCount(whitespace) = %d, want 0", got)
	}
}

func TestTokenCount_Words(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "one word", input: "hello", want: 2},            // ceil(1/0.75) = 2
		{name: "three words", input: "one two three", want: 4}, // ceil(3/0.75) = 4
		{name: "four words", input: "a b c d", want: 6},        // ceil(4/0.75) = 6
		{name: "six words", input: "a b c d e f", want: 8},     // ceil(6/0.75) = 8
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := TokenCount(tt.input); got != tt.want {
				t.Errorf("TokenCount(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestTokenCount_MarkdownContent(t *testing.T) {
	t.Parallel()
	content := "- The integration tests require docker compose up before running\n"
	got := TokenCount(content)
	words := len(strings.Fields(content))
	if got == 0 {
		t.Error("expected non-zero token count for markdown content")
	}
	if got < words {
		t.Errorf("token count (%d) should be >= word count (%d)", got, words)
	}
}

func TestCheckBudget_WithinBudget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if err := CheckBudget(dir, "ns", 100, "small entry"); err != nil {
		t.Errorf("expected nil for within-budget, got: %v", err)
	}
}

func TestCheckBudget_WithinBudget_ExistingContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := Append(dir, "ns", "existing entry"); err != nil {
		t.Fatal(err)
	}

	if err := CheckBudget(dir, "ns", 1000, "new entry"); err != nil {
		t.Errorf("expected nil for within-budget, got: %v", err)
	}
}

func TestCheckBudget_ExceedsBudget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Write enough content to fill the budget.
	big := strings.Repeat("word ", 100)
	if err := Append(dir, "ns", big); err != nil {
		t.Fatal(err)
	}

	err := CheckBudget(dir, "ns", 10, "one more entry")
	if err == nil {
		t.Fatal("expected budget error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds budget") {
		t.Errorf("error should mention exceeds budget, got: %v", err)
	}
}

func TestCheckBudget_BoundaryExact(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Three words = ceil(3/0.75) = 4 tokens.
	content := "one two three"
	err := CheckBudget(dir, "ns", 4, content)
	if err != nil {
		t.Errorf("expected nil at exact boundary, got: %v", err)
	}
}

func TestCheckBudget_BoundaryOneOver(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Three words = ceil(3/0.75) = 4 tokens, budget is 3.
	content := "one two three"
	err := CheckBudget(dir, "ns", 3, content)
	if err == nil {
		t.Fatal("expected budget error at boundary+1, got nil")
	}
}

func TestCheckBudget_MissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// No existing file, so only the new entry counts.
	err := CheckBudget(dir, "nonexistent", 100, "small entry")
	if err != nil {
		t.Errorf("expected nil for missing file, got: %v", err)
	}
}

func TestAppend_InvalidDirectory(t *testing.T) {
	t.Parallel()
	// Use a path under a file (not a directory) to trigger MkdirAll failure.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Append(blocker, "ns", "entry")
	if err == nil {
		t.Fatal("expected error for invalid directory, got nil")
	}
}

func TestAppend_ReadOnlyDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	knowledgeDir := filepath.Join(dir, "docs", "knowledge")
	if err := os.MkdirAll(knowledgeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Make the directory read-only so OpenFile fails.
	if err := os.Chmod(knowledgeDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(knowledgeDir, 0o755) })

	err := Append(dir, "ns", "entry")
	if err == nil {
		t.Fatal("expected error for read-only directory, got nil")
	}
}

func TestCheckBudget_ReadError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := FilePath(dir, "ns")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })

	err := CheckBudget(dir, "ns", 100, "entry")
	if err == nil {
		t.Fatal("expected error from unreadable file, got nil")
	}
}

func TestRead_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := FilePath(dir, "empty")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	content, err := Read(dir, "empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty string for empty file, got %q", content)
	}
}

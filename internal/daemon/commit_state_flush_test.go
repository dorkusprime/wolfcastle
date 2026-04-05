package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/logging"
)

// initFlushRepo initializes a git repo with one commit so git status works.
func initFlushRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.name", "test"},
		{"config", "user.email", "test@test.com"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	_ = os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init"), 0644)
	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func flushLogger(t *testing.T) *logging.Logger {
	t.Helper()
	logDir := filepath.Join(t.TempDir(), "logs")
	_ = os.MkdirAll(logDir, 0755)
	l, err := logging.NewLogger(logDir)
	if err != nil {
		t.Fatal(err)
	}
	_ = l.StartIteration()
	return l
}

func TestCommitStateFlush_AutoCommitDisabled(t *testing.T) {
	t.Parallel()
	dir := initFlushRepo(t)
	logger := flushLogger(t)
	defer logger.Close()

	cfg := config.GitConfig{AutoCommit: false, CommitState: true}
	// Should return immediately without error or commit.
	commitStateFlush(dir, logger, cfg)
}

func TestCommitStateFlush_CommitStateDisabled(t *testing.T) {
	t.Parallel()
	dir := initFlushRepo(t)
	logger := flushLogger(t)
	defer logger.Close()

	cfg := config.GitConfig{AutoCommit: true, CommitState: false}
	commitStateFlush(dir, logger, cfg)
}

func TestCommitStateFlush_NoChanges(t *testing.T) {
	t.Parallel()
	dir := initFlushRepo(t)
	logger := flushLogger(t)
	defer logger.Close()

	cfg := config.GitConfig{AutoCommit: true, CommitState: true}
	// No .wolfcastle changes: should return without committing.
	commitStateFlush(dir, logger, cfg)
}

func TestCommitStateFlush_WithChanges(t *testing.T) {
	t.Parallel()
	dir := initFlushRepo(t)
	logger := flushLogger(t)
	defer logger.Close()

	// Create .wolfcastle/ state changes.
	wcDir := filepath.Join(dir, ".wolfcastle")
	_ = os.MkdirAll(wcDir, 0755)
	_ = os.WriteFile(filepath.Join(wcDir, "state.json"), []byte(`{"status":"complete"}`), 0644)

	cfg := config.GitConfig{
		AutoCommit:            true,
		CommitState:           true,
		SkipHooksOnAutoCommit: true,
	}
	commitStateFlush(dir, logger, cfg)

	// Verify a commit was made.
	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(string(out), "update project state") {
		t.Errorf("expected commit with 'update project state', got:\n%s", out)
	}
}

func TestCommitStateFlush_WithPrefix(t *testing.T) {
	t.Parallel()
	dir := initFlushRepo(t)
	logger := flushLogger(t)
	defer logger.Close()

	wcDir := filepath.Join(dir, ".wolfcastle")
	_ = os.MkdirAll(wcDir, 0755)
	_ = os.WriteFile(filepath.Join(wcDir, "data.json"), []byte(`{}`), 0644)

	cfg := config.GitConfig{
		AutoCommit:            true,
		CommitState:           true,
		CommitPrefix:          "wolfcastle",
		SkipHooksOnAutoCommit: true,
	}
	commitStateFlush(dir, logger, cfg)

	cmd := exec.Command("git", "log", "-1", "--format=%s")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	subject := strings.TrimSpace(string(out))
	if subject != "wolfcastle: update project state" {
		t.Errorf("expected prefixed commit message, got %q", subject)
	}
}

func TestCommitStateFlush_NonRepoDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir() // not a git repo
	logger := flushLogger(t)
	defer logger.Close()

	cfg := config.GitConfig{AutoCommit: true, CommitState: true}
	// Should not panic; git status will fail and function returns.
	commitStateFlush(dir, logger, cfg)
}

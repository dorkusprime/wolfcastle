package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/knowledge"
	"github.com/dorkusprime/wolfcastle/internal/logging"
)

// persistFixture wires just enough Daemon state for persistKnowledgeEntries
// to run: a WolfcastleDir, a Logger, and a Config with a resolvable identity
// plus a knowledge token budget.
type persistFixture struct {
	daemon        *Daemon
	wolfcastleDir string
	namespace     string
}

func newPersistFixture(t *testing.T, maxTokens int) persistFixture {
	t.Helper()
	dir := t.TempDir()
	wolfcastleDir := filepath.Join(dir, ".wolfcastle")
	logDir := filepath.Join(wolfcastleDir, "system", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}

	logger, err := logging.NewLogger(logDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { logger.Close() })

	cfg := &config.Config{
		Identity: &config.IdentityConfig{
			User:    "tester",
			Machine: "laptop",
		},
		Knowledge: config.KnowledgeConfig{
			MaxTokens: maxTokens,
		},
	}

	return persistFixture{
		daemon: &Daemon{
			Config:        cfg,
			WolfcastleDir: wolfcastleDir,
			Logger:        logger,
		},
		wolfcastleDir: wolfcastleDir,
		namespace:     "tester-laptop",
	}
}

func TestPersistKnowledgeEntries_NoMarkersIsNoop(t *testing.T) {
	t.Parallel()
	f := newPersistFixture(t, 10000)

	f.daemon.persistKnowledgeEntries("some-orch", "WOLFCASTLE_CONTINUE\n")

	// No knowledge file should have been created.
	if _, err := os.Stat(knowledge.FilePath(f.wolfcastleDir, f.namespace)); !os.IsNotExist(err) {
		t.Errorf("expected no knowledge file, stat err = %v", err)
	}
}

func TestPersistKnowledgeEntries_AppendsToKnowledgeFile(t *testing.T) {
	t.Parallel()
	f := newPersistFixture(t, 10000)

	output := `Found issues.
WOLFCASTLE_KNOWLEDGE: Seed UUIDs must be unique across the entire file.
WOLFCASTLE_KNOWLEDGE: Integration tests must import testify/require.
WOLFCASTLE_CONTINUE`
	f.daemon.persistKnowledgeEntries("warzone/backend", output)

	got, err := knowledge.Read(f.wolfcastleDir, f.namespace)
	if err != nil {
		t.Fatalf("reading knowledge file: %v", err)
	}
	if !strings.Contains(got, "Seed UUIDs must be unique") {
		t.Errorf("first entry missing: %q", got)
	}
	if !strings.Contains(got, "Integration tests must import testify/require") {
		t.Errorf("second entry missing: %q", got)
	}
	// Each entry should be on its own bullet line.
	if strings.Count(got, "- ") != 2 {
		t.Errorf("expected 2 bullets, got %q", got)
	}
}

func TestPersistKnowledgeEntries_BudgetExceededSkipsEntry(t *testing.T) {
	t.Parallel()
	// MaxTokens=5 is tiny; a multi-word entry blows the budget immediately.
	f := newPersistFixture(t, 5)

	output := `WOLFCASTLE_KNOWLEDGE: This entry has many words and will certainly exceed a five-token budget.`
	f.daemon.persistKnowledgeEntries("n", output)

	// File should not exist because the single entry was rejected.
	if _, err := os.Stat(knowledge.FilePath(f.wolfcastleDir, f.namespace)); !os.IsNotExist(err) {
		t.Errorf("expected no file after budget rejection, stat err = %v", err)
	}
}

func TestPersistKnowledgeEntries_ZeroBudgetSkipsCheck(t *testing.T) {
	t.Parallel()
	// MaxTokens=0 means the config didn't set a budget. We should still
	// append rather than reject.
	f := newPersistFixture(t, 0)

	f.daemon.persistKnowledgeEntries("n", "WOLFCASTLE_KNOWLEDGE: No budget applied.\n")

	got, err := knowledge.Read(f.wolfcastleDir, f.namespace)
	if err != nil {
		t.Fatalf("reading knowledge file: %v", err)
	}
	if !strings.Contains(got, "No budget applied.") {
		t.Errorf("expected entry persisted, got %q", got)
	}
}

func TestPersistKnowledgeEntries_MissingIdentityIsNoop(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfcastleDir := filepath.Join(dir, ".wolfcastle")
	logDir := filepath.Join(wolfcastleDir, "system", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logger, err := logging.NewLogger(logDir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	// No Identity on the config → namespace() returns "".
	d := &Daemon{
		Config:        &config.Config{Knowledge: config.KnowledgeConfig{MaxTokens: 1000}},
		WolfcastleDir: wolfcastleDir,
		Logger:        logger,
	}

	d.persistKnowledgeEntries("n", "WOLFCASTLE_KNOWLEDGE: would be lost without identity\n")

	// No file should have been created.
	entries, _ := os.ReadDir(filepath.Join(wolfcastleDir, "docs", "knowledge"))
	if len(entries) != 0 {
		t.Errorf("expected no knowledge files without identity, got %+v", entries)
	}
}

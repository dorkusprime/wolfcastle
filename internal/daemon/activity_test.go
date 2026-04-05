package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/logging"
)

func TestWriteActivity_WritesAndLoads(t *testing.T) {
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

	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	d := &Daemon{
		WolfcastleDir: wolfcastleDir,
		Clock:         clock.NewFixed(now),
		Logger:        logger,
	}
	d.iteration.Store(42)

	d.writeActivity("proj/auth", "task-0001")

	got := LoadDaemonActivity(wolfcastleDir)
	if got == nil {
		t.Fatal("LoadDaemonActivity returned nil")
	}
	if !got.LastActivityAt.Equal(now) {
		t.Errorf("LastActivityAt = %v, want %v", got.LastActivityAt, now)
	}
	if got.Iteration != 42 {
		t.Errorf("Iteration = %d, want 42", got.Iteration)
	}
	if got.CurrentNode != "proj/auth" {
		t.Errorf("CurrentNode = %q, want %q", got.CurrentNode, "proj/auth")
	}
	if got.CurrentTask != "task-0001" {
		t.Errorf("CurrentTask = %q, want %q", got.CurrentTask, "task-0001")
	}
}

func TestRemoveActivityFile_Cleans(t *testing.T) {
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

	d := &Daemon{
		WolfcastleDir: wolfcastleDir,
		Clock:         clock.NewFixed(time.Now()),
		Logger:        logger,
	}

	d.writeActivity("node", "task")
	if LoadDaemonActivity(wolfcastleDir) == nil {
		t.Fatal("activity file should exist after write")
	}

	d.removeActivityFile()
	if LoadDaemonActivity(wolfcastleDir) != nil {
		t.Error("activity file should not exist after remove")
	}
}

func TestLoadDaemonActivity_ReturnsNilForMissingFile(t *testing.T) {
	t.Parallel()
	if got := LoadDaemonActivity(t.TempDir()); got != nil {
		t.Errorf("expected nil for missing file, got %+v", got)
	}
}

func TestLoadDaemonActivity_ReturnsNilForBadJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfcastleDir := filepath.Join(dir, ".wolfcastle")
	if err := os.MkdirAll(filepath.Join(wolfcastleDir, "system"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(activityPath(wolfcastleDir), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LoadDaemonActivity(wolfcastleDir); got != nil {
		t.Errorf("expected nil for bad JSON, got %+v", got)
	}
}

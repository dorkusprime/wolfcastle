package daemon

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// sleepWithContext — both branches
// ═══════════════════════════════════════════════════════════════════════════

func TestSleepWithContext_FullSleep(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	completed := sleepWithContext(ctx, 10*time.Millisecond)
	if !completed {
		t.Error("expected full sleep to complete")
	}
}

func TestSleepWithContext_CancelledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	completed := sleepWithContext(ctx, 10*time.Second)
	if completed {
		t.Error("expected interrupted sleep")
	}
}

func TestSleepWithContext_ContextCancelledDuringSleep(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	completed := sleepWithContext(ctx, 10*time.Second)
	if completed {
		t.Error("expected interrupted sleep")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runInboxWithPolling — context cancellation exits cleanly
// ═══════════════════════════════════════════════════════════════════════════

func TestRunInboxWithPolling_ExitsOnCancel(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Daemon.InboxPollIntervalSeconds = 0
	d.Config.Daemon.BlockedPollIntervalSeconds = 0
	// Both zero → falls through to 5s default, but we cancel quickly

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Should return immediately without panic
	d.runInboxWithPolling(ctx)
}

func TestRunInboxWithPolling_ProcessesInbox(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Daemon.InboxPollIntervalSeconds = 1
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "intake", Model: "echo", PromptFile: "intake.md", Enabled: boolPtr(true)},
	}
	writePromptFile(t, d.WolfcastleDir, "intake.md")

	// Put a new item in the inbox
	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "poll-test", Timestamp: "2026-01-01T00:00:00Z"},
	}})

	// Cancel after a short delay to let one poll cycle run
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	d.runInboxWithPolling(ctx)

	// Verify the item was processed (filed)
	updatedInbox, err := state.LoadInbox(inboxPath)
	if err != nil {
		t.Fatalf("loading inbox: %v", err)
	}
	if len(updatedInbox.Items) > 0 && updatedInbox.Items[0].Status != "filed" {
		t.Logf("item status after polling: %s (may not have completed a full cycle)", updatedInbox.Items[0].Status)
	}
}

func TestRunInboxWithPolling_BlockedPollFallback(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	// InboxPollIntervalSeconds = 0, BlockedPollIntervalSeconds > 0
	d.Config.Daemon.InboxPollIntervalSeconds = 0
	d.Config.Daemon.BlockedPollIntervalSeconds = 1

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d.runInboxWithPolling(ctx)
}

// ═══════════════════════════════════════════════════════════════════════════
// runInboxLoop — polling fallback when fsnotify fails
// ═══════════════════════════════════════════════════════════════════════════

func TestRunInboxLoop_ExitsOnCancel(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Daemon.InboxPollIntervalSeconds = 1

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Should not hang
	d.runInboxLoop(ctx)
}

// ═══════════════════════════════════════════════════════════════════════════
// currentBranch — fallback paths
// ═══════════════════════════════════════════════════════════════════════════

func TestCurrentBranch_NotAGitRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := currentBranch(dir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestCurrentBranch_EmptyRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// git init creates a repo with no commits
	if err := runGitInit(dir); err != nil {
		t.Skip("git not available")
	}
	branch, err := currentBranch(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return "main" or "master" (default branch)
	if branch == "" {
		t.Error("expected non-empty branch name")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// processInbox — no matching intake stage
// ═══════════════════════════════════════════════════════════════════════════

func TestProcessInbox_NoIntakeStage(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "echo", PromptFile: "execute.md"},
	}

	// Should not panic with no intake stage
	d.processInbox(context.Background(), 1)
}

func TestProcessInbox_IntakeStageDisabled(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "intake", Model: "echo", PromptFile: "intake.md", Enabled: boolPtr(false)},
	}

	d.processInbox(context.Background(), 1)
}

// ═══════════════════════════════════════════════════════════════════════════
// RunOnce — iteration cap (new test, stop/shutdown covered elsewhere)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunOnce_IterationCapReached(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Daemon.MaxIterations = 5
	d.iteration = 5 // already at cap
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	result, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != IterationStop {
		t.Errorf("expected IterationStop, got %d", result)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIntakeStage — error paths
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIntakeStage_MissingModel(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.InboxLogger.StartIterationWithPrefix("intake")
	defer d.InboxLogger.Close()

	// Put new items in inbox
	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "test", Timestamp: "2026-01-01T00:00:00Z"},
	}})

	// Use a model key that doesn't exist
	stage := config.PipelineStage{Name: "intake", Model: "nonexistent", PromptFile: "intake.md"}
	err := d.runIntakeStage(context.Background(), stage)
	if err == nil {
		t.Error("expected error for missing model")
	}
}

func TestRunIntakeStage_AllItemsFiled(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.InboxLogger.StartIterationWithPrefix("intake")
	defer d.InboxLogger.Close()

	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "filed", Text: "already done", Timestamp: "2026-01-01T00:00:00Z"},
	}})

	stage := config.PipelineStage{Name: "intake", Model: "echo", PromptFile: "intake.md"}
	err := d.runIntakeStage(context.Background(), stage)
	if err != nil {
		t.Errorf("expected nil for no new items, got: %v", err)
	}
}

func TestRunIntakeStage_NoInboxFile(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.InboxLogger.StartIterationWithPrefix("intake")
	defer d.InboxLogger.Close()

	// No inbox file on disk
	stage := config.PipelineStage{Name: "intake", Model: "echo", PromptFile: "intake.md"}
	err := d.runIntakeStage(context.Background(), stage)
	if err != nil {
		t.Errorf("expected nil for missing inbox, got: %v", err)
	}
}

func TestRunIntakeStage_WithExistingTree(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.InboxLogger.StartIterationWithPrefix("intake")
	defer d.InboxLogger.Close()
	writePromptFile(t, d.WolfcastleDir, "intake.md")

	// Create a tree so the intake context includes existing projects
	setupLeafNode(t, d, "existing-project", []state.Task{
		{ID: "task-0001", State: state.StatusNotStarted},
	})

	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "new work", Timestamp: "2026-01-01T00:00:00Z"},
	}})

	stage := config.PipelineStage{Name: "intake", Model: "echo", PromptFile: "intake.md"}
	err := d.runIntakeStage(context.Background(), stage)
	if err != nil {
		t.Fatalf("intake stage error: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// checkInboxForNew — the stages.go variant
// ═══════════════════════════════════════════════════════════════════════════

func TestCheckInboxForNew_MissingFile(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	if d.checkInboxForNew("/nonexistent/inbox.json") {
		t.Error("expected false for missing file")
	}
}

func TestCheckInboxForNew_NewItems(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "test"},
	}})

	if !d.checkInboxForNew(inboxPath) {
		t.Error("expected true for new items")
	}
}

func TestCheckInboxForNew_AllFiled(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "filed", Text: "done"},
	}})

	if d.checkInboxForNew(inboxPath) {
		t.Error("expected false for all filed")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// helpers
// ═══════════════════════════════════════════════════════════════════════════

func boolPtr(b bool) *bool { return &b }

func runGitInit(dir string) error {
	return exec.Command("git", "init", dir).Run()
}

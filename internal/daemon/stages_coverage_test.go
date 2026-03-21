package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// runInboxLoop — fallback to polling when watcher.Add fails
// ═══════════════════════════════════════════════════════════════════════════

func TestRunInboxLoop_FallbackToPolling_WatcherAddFails(t *testing.T) {
	d := testDaemon(t)
	d.Config.Daemon.InboxPollIntervalSeconds = 1

	// Point the store at a nonexistent directory so watcher.Add fails
	// while watcher creation itself succeeds.
	origDir := d.Store.Dir()
	badDir := filepath.Join(origDir, "nonexistent-subdir-for-watcher")
	d.Store = state.NewStateStore(badDir, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the polling loop exits after one pass
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// This should fall through to runInboxWithPolling because watcher.Add
	// fails on the nonexistent directory.
	d.runInboxLoop(ctx)
}

// ═══════════════════════════════════════════════════════════════════════════
// runInboxWithFsnotify — watcher.Events channel closed
// ═══════════════════════════════════════════════════════════════════════════

func TestRunInboxWithFsnotify_EventsChannelClosed(t *testing.T) {
	d := testDaemon(t)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Skipf("cannot create watcher: %v", err)
	}

	// Close the watcher immediately so its Events channel closes,
	// causing the !ok branch to fire.
	_ = watcher.Close()

	ctx := context.Background()
	// runInboxWithFsnotify should return promptly once Events is closed.
	done := make(chan struct{})
	go func() {
		d.runInboxWithFsnotify(ctx, watcher)
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("runInboxWithFsnotify did not return after Events channel closed")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runInboxWithFsnotify — watcher error event
// ═══════════════════════════════════════════════════════════════════════════

func TestRunInboxWithFsnotify_WatcherError(t *testing.T) {
	d := testDaemon(t)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Skipf("cannot create watcher: %v", err)
	}
	defer func() { _ = watcher.Close() }()

	projDir := d.Store.Dir()
	if addErr := watcher.Add(projDir); addErr != nil {
		t.Skipf("cannot watch dir: %v", addErr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		d.runInboxWithFsnotify(ctx, watcher)
		close(done)
	}()

	// Give the goroutine a moment to enter the select loop, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runInboxWithFsnotify did not exit on ctx cancel")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runInboxWithFsnotify — context done during select
// ═══════════════════════════════════════════════════════════════════════════

func TestRunInboxWithFsnotify_ContextCancelledDuringLoop(t *testing.T) {
	d := testDaemon(t)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Skipf("cannot create watcher: %v", err)
	}
	defer func() { _ = watcher.Close() }()

	projDir := d.Store.Dir()
	if err := watcher.Add(projDir); err != nil {
		t.Skipf("cannot watch dir: %v", err)
	}

	// Pre-cancel context so the ctx.Done case fires on first iteration
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should return immediately
	d.runInboxWithFsnotify(ctx, watcher)
}

// ═══════════════════════════════════════════════════════════════════════════
// processInbox — intake stage error propagation
// ═══════════════════════════════════════════════════════════════════════════

func TestProcessInbox_IntakeStageError(t *testing.T) {
	d := testDaemon(t)
	_ = d.InboxLogger.StartIterationWithPrefix("intake")
	defer d.InboxLogger.Close()

	// Configure with a model that doesn't exist, so runIntakeStage
	// returns an error from the model-not-found path.
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"intake": {Model: "nonexistent-model", PromptFile: "stages/intake.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"intake"}

	// Write an inbox with "new" items so runIntakeStage gets past the
	// empty-inbox early return.
	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "trigger error", Timestamp: "2026-01-01T00:00:00Z"},
	}})

	// processInbox catches the error and prints it; should not panic.
	d.processInbox(context.Background(), 1)
}

// ═══════════════════════════════════════════════════════════════════════════
// processInbox — no intake stage configured (empty stages)
// ═══════════════════════════════════════════════════════════════════════════

func TestProcessInbox_DisabledIntakeStage(t *testing.T) {
	d := testDaemon(t)

	f := false
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"intake": {Model: "echo", PromptFile: "stages/intake.md", Enabled: &f},
	}
	d.Config.Pipeline.StageOrder = []string{"intake"}

	// Disabled stage should be skipped
	d.processInbox(context.Background(), 1)
}

// ═══════════════════════════════════════════════════════════════════════════
// runIntakeStage — LoadInbox error (corrupt JSON)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIntakeStage_LoadInboxError(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	// Write corrupt JSON to inbox.json
	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	_ = os.WriteFile(inboxPath, []byte("{corrupt"), 0644)

	stage := config.PipelineStage{Model: "echo", PromptFile: "stages/intake.md"}
	err := d.runIntakeStage(context.Background(), stage)
	// LoadInbox error returns nil (treated as no inbox)
	if err != nil {
		t.Errorf("expected nil for corrupt inbox, got: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIntakeStage — planning enabled switches prompt file
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIntakeStage_PlanningEnabled(t *testing.T) {
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = true
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "stages/intake-planning.md")

	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "planning item", Timestamp: "2026-01-01T00:00:00Z"},
	}})

	stage := config.PipelineStage{Model: "echo", PromptFile: "stages/intake.md"}
	if err := d.runIntakeStage(context.Background(), stage); err != nil {
		t.Fatalf("intake stage error: %v", err)
	}

	// Verify item was filed
	updated, err := state.LoadInbox(inboxPath)
	if err != nil {
		t.Fatalf("loading inbox: %v", err)
	}
	if updated.Items[0].Status != "filed" {
		t.Errorf("expected filed, got %q", updated.Items[0].Status)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIntakeStage — prompt assembly error
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIntakeStage_PromptAssemblyError(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	// Don't write the prompt file so AssemblePrompt fails

	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "item", Timestamp: "2026-01-01T00:00:00Z"},
	}})

	stage := config.PipelineStage{Model: "echo", PromptFile: "nonexistent-prompt.md"}
	err := d.runIntakeStage(context.Background(), stage)
	if err == nil {
		t.Error("expected error for missing prompt file")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIntakeStage — invocation error (bad model command)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIntakeStage_InvocationError(t *testing.T) {
	d := testDaemon(t)
	d.Config.Models["bad-cmd"] = config.ModelDef{
		Command: "nonexistent-command-xyz",
		Args:    []string{},
	}
	d.Config.Retries = config.RetriesConfig{MaxRetries: 0}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "stages/intake.md")

	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "item", Timestamp: "2026-01-01T00:00:00Z"},
	}})

	stage := config.PipelineStage{Model: "bad-cmd", PromptFile: "stages/intake.md"}
	err := d.runIntakeStage(context.Background(), stage)
	if err == nil {
		t.Error("expected error for bad model command")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIntakeStage — non-zero exit code (items not filed)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIntakeStage_NonZeroExitCode(t *testing.T) {
	d := testDaemon(t)
	d.Config.Models["fail"] = config.ModelDef{
		Command: "sh",
		Args:    []string{"-c", "exit 1"},
	}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "stages/intake.md")

	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "should stay new", Timestamp: "2026-01-01T00:00:00Z"},
	}})

	stage := config.PipelineStage{Model: "fail", PromptFile: "stages/intake.md"}
	err := d.runIntakeStage(context.Background(), stage)
	if err != nil {
		t.Fatalf("non-zero exit should not be a fatal error: %v", err)
	}

	// Items should remain "new"
	updated, _ := state.LoadInbox(inboxPath)
	if updated.Items[0].Status != "new" {
		t.Errorf("expected item to remain 'new', got %q", updated.Items[0].Status)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIntakeStage — existing tree context included in prompt
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIntakeStage_ExistingTreeContext(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "stages/intake.md")

	// Set up an existing node so the tree context branch is hit
	setupLeafNode(t, d, "existing-project", []state.Task{
		{ID: "task-0001", State: state.StatusNotStarted},
	})

	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "add to existing", Timestamp: "2026-01-01T00:00:00Z"},
	}})

	stage := config.PipelineStage{Model: "echo", PromptFile: "stages/intake.md"}
	if err := d.runIntakeStage(context.Background(), stage); err != nil {
		t.Fatalf("intake stage error: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIntakeStage — existing tree with parent node context
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIntakeStage_ExistingTreeWithParent(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "stages/intake.md")

	// Set up a parent-child relationship so both code paths
	// (entry.Parent != "" and entry.Parent == "") are covered
	projDir := d.Store.Dir()
	idx := state.NewRootIndex()
	idx.Root = []string{"parent"}
	idx.Nodes["parent"] = state.IndexEntry{
		Name: "Parent", Type: state.NodeOrchestrator, State: state.StatusNotStarted,
		Address: "parent", Children: []string{"parent/child"},
	}
	idx.Nodes["parent/child"] = state.IndexEntry{
		Name: "Child", Type: state.NodeLeaf, State: state.StatusNotStarted,
		Address: "parent/child", Parent: "parent",
	}
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	inboxPath := filepath.Join(projDir, "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "new work", Timestamp: "2026-01-01T00:00:00Z"},
	}})

	stage := config.PipelineStage{Model: "echo", PromptFile: "stages/intake.md"}
	if err := d.runIntakeStage(context.Background(), stage); err != nil {
		t.Fatalf("intake stage error: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIntakeStage — planning enabled with overlap markers in output
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIntakeStage_PlanningWithOverlapMarkers(t *testing.T) {
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = true
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "stages/intake-planning.md")

	// Set up a target node for overlap delivery
	projDir := d.Store.Dir()
	ns := state.NewNodeState("target-proj", "Target Project", state.NodeOrchestrator)
	writeJSON(t, filepath.Join(projDir, "target-proj", "state.json"), ns)

	// Model outputs an OVERLAP marker
	d.Config.Models["overlap-echo"] = config.ModelDef{
		Command: "echo",
		Args:    []string{`OVERLAP: "test overlap" overlaps with Target Project (target-proj)`},
	}

	inboxPath := filepath.Join(projDir, "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "overlapping work", Timestamp: "2026-01-01T00:00:00Z"},
	}})

	stage := config.PipelineStage{Model: "overlap-echo", PromptFile: "stages/intake.md"}
	if err := d.runIntakeStage(context.Background(), stage); err != nil {
		t.Fatalf("intake stage error: %v", err)
	}

	// Verify overlap was delivered as pending scope
	updated, err := d.Store.ReadNode("target-proj")
	if err != nil {
		t.Fatalf("reading target node: %v", err)
	}
	if len(updated.PendingScope) != 1 {
		t.Errorf("expected 1 pending scope, got %d", len(updated.PendingScope))
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIntakeStage — workAvailable channel already full (default branch)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIntakeStage_WorkAvailableAlreadyFull(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "stages/intake.md")

	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "item", Timestamp: "2026-01-01T00:00:00Z"},
	}})

	// Pre-fill the workAvailable channel so the non-blocking send
	// hits the default branch
	d.workAvailable <- struct{}{}

	stage := config.PipelineStage{Model: "echo", PromptFile: "stages/intake.md"}
	if err := d.runIntakeStage(context.Background(), stage); err != nil {
		t.Fatalf("intake stage error: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// parseOverlapMarkers — malformed markers
// ═══════════════════════════════════════════════════════════════════════════

func TestParseOverlapMarkers_MissingOpenQuote(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	// OVERLAP line with no quotes at all
	d.parseOverlapMarkers(`OVERLAP: no quotes here (my-addr)`)
}

func TestParseOverlapMarkers_MissingCloseQuote(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	// OVERLAP line with only one quote
	d.parseOverlapMarkers(`OVERLAP: "only one quote (my-addr)`)
}

func TestParseOverlapMarkers_MissingParentheses(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	// OVERLAP line with quotes but no parentheses
	d.parseOverlapMarkers(`OVERLAP: "valid quotes" overlaps with something`)
}

func TestParseOverlapMarkers_EmptyParentheses(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	// Parentheses present but parenEnd <= parenStart (empty parens)
	d.parseOverlapMarkers(`OVERLAP: "scope" overlaps with ()`)
}

func TestParseOverlapMarkers_EmptyLinesBetweenMarkers(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	projDir := d.Store.Dir()

	ns := state.NewNodeState("proj-a", "Project A", state.NodeOrchestrator)
	writeJSON(t, filepath.Join(projDir, "proj-a", "state.json"), ns)

	// Multiple markers with empty lines between them
	input := "OVERLAP: \"scope one\" overlaps with Project A (proj-a)\n" +
		"\n" +
		"some other output\n" +
		"\n" +
		"OVERLAP: \"scope two\" overlaps with Project A (proj-a)\n"

	d.parseOverlapMarkers(input)

	updated, err := d.Store.ReadNode("proj-a")
	if err != nil {
		t.Fatalf("reading node: %v", err)
	}
	if len(updated.PendingScope) != 2 {
		t.Errorf("expected 2 pending scope items, got %d", len(updated.PendingScope))
	}
}

func TestParseOverlapMarkers_MixedValidAndInvalid(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	projDir := d.Store.Dir()

	ns := state.NewNodeState("good-proj", "Good Project", state.NodeOrchestrator)
	writeJSON(t, filepath.Join(projDir, "good-proj", "state.json"), ns)

	// Mix of valid, malformed, and non-existent target markers
	input := "OVERLAP: no quotes (bad)\n" +
		"OVERLAP: \"one quote only\n" +
		"OVERLAP: \"valid\" overlaps with Good Project (good-proj)\n" +
		"OVERLAP: \"scope\" but no parens\n"

	d.parseOverlapMarkers(input)

	updated, err := d.Store.ReadNode("good-proj")
	if err != nil {
		t.Fatalf("reading node: %v", err)
	}
	if len(updated.PendingScope) != 1 {
		t.Errorf("expected 1 pending scope (only the valid one), got %d", len(updated.PendingScope))
	}
}

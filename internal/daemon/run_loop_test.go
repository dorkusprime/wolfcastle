package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// Run — stop-file detection mid-loop
// ═══════════════════════════════════════════════════════════════════════════

func TestRun_StopFileMidLoop(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false

	// Set up an all-complete tree so the first iteration returns NoWork,
	// then place a stop file so the second iteration sees it and exits.
	setupLeafNode(t, d, "done-node", []state.Task{
		{ID: "task-0001", State: state.StatusComplete},
	})
	idx, _ := d.Store.ReadIndex()
	entry := idx.Nodes["done-node"]
	entry.State = state.StatusComplete
	idx.Nodes["done-node"] = entry
	_ = state.SaveRootIndex(filepath.Join(d.Store.Dir(), "state.json"), idx)

	// After the first NoWork idle wait, place the stop file so the next
	// RunOnce detects it. Use BlockedPollIntervalSeconds=0 so the idle
	// select falls through on the poll timeout immediately.
	d.Config.Daemon.BlockedPollIntervalSeconds = 0

	// Place the stop file after a brief delay so Run enters the loop first.
	go func() {
		time.Sleep(50 * time.Millisecond)
		stopPath := filepath.Join(d.WolfcastleDir, "system", "stop")
		_ = os.WriteFile(stopPath, []byte("stop"), 0644)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := d.Run(ctx)
	if err != nil {
		t.Fatalf("Run should exit cleanly on stop file: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Run — shutdown channel closes during idle select
// ═══════════════════════════════════════════════════════════════════════════

func TestRun_ShutdownDuringIdle(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false

	// Empty tree: RunOnce returns NoWork, Run enters idle select.
	idx := state.NewRootIndex()
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	// Close shutdown channel after a short delay to hit the d.shutdown
	// case inside the idle select (lines 341-343).
	go func() {
		time.Sleep(50 * time.Millisecond)
		d.shutdownOnce.Do(func() { close(d.shutdown) })
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := d.Run(ctx)
	if err != nil {
		t.Fatalf("Run should exit cleanly on shutdown: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Run — workAvailable channel wakes idle loop
// ═══════════════════════════════════════════════════════════════════════════

func TestRun_WorkAvailableWakesIdleLoop(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	d.Config.Daemon.MaxIterations = 1
	// Long poll so the only way to wake is workAvailable or shutdown.
	d.Config.Daemon.BlockedPollIntervalSeconds = 300

	// Start with all-complete tree so Run goes idle (NoWork).
	setupLeafNode(t, d, "wakeup-node", []state.Task{
		{ID: "task-0001", State: state.StatusComplete},
	})
	idx, _ := d.Store.ReadIndex()
	entry := idx.Nodes["wakeup-node"]
	entry.State = state.StatusComplete
	idx.Nodes["wakeup-node"] = entry
	_ = state.SaveRootIndex(filepath.Join(d.Store.Dir(), "state.json"), idx)

	// After a brief delay, add a not_started task and signal workAvailable.
	// The daemon wakes from idle and discovers work on the next RunOnce.
	go func() {
		time.Sleep(50 * time.Millisecond)
		// Add a new actionable task via MutateNode
		_ = d.Store.MutateNode("wakeup-node", func(ns *state.NodeState) error {
			ns.Tasks = append(ns.Tasks, state.Task{
				ID:          "task-0002",
				Description: "do work",
				State:       state.StatusNotStarted,
			})
			return nil
		})
		writePromptFile(t, d.WolfcastleDir, "execute.md")
		// Signal work available (non-blocking send)
		select {
		case d.workAvailable <- struct{}{}:
		default:
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := d.Run(ctx)
	if err != nil {
		t.Fatalf("Run should exit cleanly after workAvailable signal: %v", err)
	}
	if d.iteration != 1 {
		t.Errorf("expected 1 iteration after wake, got %d", d.iteration)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Run — context cancellation at top of loop (with spinner active)
// ═══════════════════════════════════════════════════════════════════════════

func TestRun_ContextCancelAtLoopTop(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false

	// BlockedPollIntervalSeconds=0 means the idle select's poll timeout
	// fires immediately, returning to the top of the loop via continue.
	// With a short context timeout, the top-of-loop select catches
	// ctx.Done() while the spinner is still alive (lines 306-308).
	d.Config.Daemon.BlockedPollIntervalSeconds = 0

	// Empty tree → NoWork → idle → poll timeout → top-of-loop → ctx.Done
	idx := state.NewRootIndex()
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	err := d.Run(ctx)
	if err != nil {
		t.Fatalf("Run should return nil on context cancel: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Run — IterationError path sleeps then retries
// ═══════════════════════════════════════════════════════════════════════════

func TestRun_IterationErrorSleepsAndRetries(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	d.Config.Daemon.PollIntervalSeconds = 0

	// Point at a model that doesn't exist, causing runIteration to fail.
	// RunOnce returns IterationError, Run sleeps (0s) and retries.
	// Use MaxIterations=2 to limit retries then stop via stop file.
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "nonexistent", PromptFile: "execute.md"},
	}

	setupLeafNode(t, d, "err-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	// Place stop file after a short delay so Run exits after a couple of
	// error iterations.
	go func() {
		time.Sleep(100 * time.Millisecond)
		stopPath := filepath.Join(d.WolfcastleDir, "system", "stop")
		_ = os.WriteFile(stopPath, []byte("stop"), 0644)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := d.Run(ctx)
	if err != nil {
		t.Fatalf("Run should exit cleanly via stop file: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Run — IterationError with context cancel during sleep
// ═══════════════════════════════════════════════════════════════════════════

func TestRun_IterationErrorContextCancelDuringSleep(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	d.Config.Daemon.PollIntervalSeconds = 60 // long sleep so ctx cancel fires first

	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "nonexistent", PromptFile: "execute.md"},
	}

	setupLeafNode(t, d, "err-cancel-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	// Cancel quickly so sleepWithContext returns false inside the
	// IterationError branch, causing Run to return nil.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := d.Run(ctx)
	if err != nil {
		t.Fatalf("Run should return nil when ctx cancelled during error sleep: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Run — IterationDidWork exercises log retention
// ═══════════════════════════════════════════════════════════════════════════

func TestRun_DidWorkEnforcesLogRetention(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	d.Config.Daemon.MaxIterations = 1
	d.Config.Logs.Compress = true // exercise the compression option path

	setupLeafNode(t, d, "work-node", []state.Task{
		{ID: "task-0001", Description: "do work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := d.Run(ctx)
	if err != nil {
		t.Fatalf("Run should exit cleanly after DidWork: %v", err)
	}
	if d.iteration != 1 {
		t.Errorf("expected iteration=1, got %d", d.iteration)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Run — context cancelled during idle select (ctx.Done case, lines 338-340)
// ═══════════════════════════════════════════════════════════════════════════

func TestRun_ContextCancelDuringIdleSelect(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	// Use a long poll interval so the idle select blocks on all channels
	// except ctx.Done.
	d.Config.Daemon.BlockedPollIntervalSeconds = 300

	// Empty tree → NoWork → idle select
	idx := state.NewRootIndex()
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := d.Run(ctx)
	if err != nil {
		t.Fatalf("Run should return nil on context cancel during idle: %v", err)
	}
}

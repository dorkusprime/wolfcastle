// Package daemon implements the Wolfcastle daemon loop: finding actionable
// tasks via tree navigation, running pipeline stages (intake, execute),
// parsing model output markers, and propagating state changes to ancestor
// nodes and the root index. The daemon supports crash recovery via a
// supervisor wrapper, signal-driven graceful shutdown, stop-file detection,
// and configurable iteration caps.
//
// Inbox processing runs in a parallel goroutine that polls for new items
// and runs the intake stage independently of the main execution loop
// (ADR-064).
//
// File layout follows ADR-045:
//
//   - daemon.go    — Daemon struct, New, Run, RunWithSupervisor, RunOnce
//   - iteration.go   — per-iteration pipeline dispatch, terminal marker scanning
//   - stages.go      — intake stage handler, parallel inbox goroutine
//   - deliverables.go — deliverable file verification
//   - retry.go       — invocation retry with exponential backoff
//   - propagate.go   — state propagation via StateStore
//   - branch.go          — git branch detection
//   - pid.go             — PID file operations
//   - signals_unix.go    — shutdown signals (Unix)
//   - signals_windows.go — shutdown signals (Windows)
package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/config"
	werrors "github.com/dorkusprime/wolfcastle/internal/errors"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/logging"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// inboxIterationOffset separates inbox log file numbering from execute
// loop numbering so both can write to the same directory without
// filename collisions.
const inboxIterationOffset = 10000

// Daemon is the main Wolfcastle daemon loop.
type Daemon struct {
	Config         *config.Config
	WolfcastleDir  string
	Store          *state.StateStore
	ScopeNode      string
	Logger         *logging.Logger
	InboxLogger    *logging.Logger // separate logger for the inbox goroutine
	RepoDir        string
	Clock          clock.Clock
	ContextBuilder *pipeline.ContextBuilder
	SleepFunc      func(time.Duration) // override for testing; nil defaults to time.Sleep

	shutdown      chan struct{}
	shutdownOnce  sync.Once
	workAvailable chan struct{}
	sigChan       chan os.Signal
	runWg         sync.WaitGroup // tracks goroutines started by Run
	branch        string
	iteration     int
	lastNoWorkMsg string // dedup "no targets" / "WOLFCASTLE_COMPLETE" messages
}

// repo returns a DaemonRepository for the daemon's wolfcastle directory.
func (d *Daemon) repo() *DaemonRepository {
	return NewDaemonRepository(d.WolfcastleDir)
}

// New creates a new daemon.
func New(cfg *config.Config, wolfcastleDir string, store *state.StateStore, scopeNode string, repoDir string) (*Daemon, error) {
	logDir := filepath.Join(wolfcastleDir, "system", "logs")
	logger, err := logging.NewLogger(logDir)
	if err != nil {
		return nil, err
	}

	// Apply the configured console log level (ADR-046).
	if lvl, ok := logging.ParseLevel(cfg.Daemon.LogLevel); ok {
		logger.ConsoleLevel = lvl
	}
	// Resume iteration numbering from existing log files.
	logger.Iteration = logging.IterationFromDir(logDir)

	// Create a separate logger for the inbox goroutine so it doesn't
	// race with the execute loop's logger on file handles and counters.
	inboxLogger, err := logging.NewLogger(logDir)
	if err != nil {
		return nil, err
	}
	if lvl, ok := logging.ParseLevel(cfg.Daemon.LogLevel); ok {
		inboxLogger.ConsoleLevel = lvl
	}
	// Offset inbox iterations by 10000 to avoid filename collisions
	// with the execute loop. Both write to the same directory but
	// their iteration numbers never overlap.
	inboxLogger.Iteration = inboxIterationOffset + logging.IterationFromDir(logDir)

	// Build domain repositories for prompt and class resolution, then
	// assemble a ContextBuilder that replaces the legacy standalone
	// buildIterationContext functions.
	prompts := pipeline.NewPromptRepository(wolfcastleDir)
	classes := pipeline.NewClassRepository(prompts)
	classes.Reload(cfg.TaskClasses)
	ctxBuilder := pipeline.NewContextBuilder(prompts, classes)

	return &Daemon{
		Config:         cfg,
		WolfcastleDir:  wolfcastleDir,
		Store:          store,
		ScopeNode:      scopeNode,
		Logger:         logger,
		InboxLogger:    inboxLogger,
		RepoDir:        repoDir,
		Clock:          clock.New(),
		ContextBuilder: ctxBuilder,
		shutdown:       make(chan struct{}),
		workAvailable:  make(chan struct{}, 1),
	}, nil
}

// errNoChange is a sentinel used by selfHeal to signal that MutateNode
// should skip writing when no tasks were actually modified.
var errNoChange = fmt.Errorf("no change")

// selfHeal scans the tree for stale in_progress tasks on startup (ADR-020).
func (d *Daemon) selfHeal() error {
	output.PrintHuman("Scanning for casualties...")
	idx, err := d.Store.ReadIndex()
	if err != nil {
		output.PrintHuman("No root index. Nothing to recover.")
		return nil
	}

	healed := 0
	for addr, entry := range idx.Nodes {
		if entry.Type != state.NodeLeaf && entry.Type != state.NodeOrchestrator {
			continue
		}
		if _, parseErr := tree.ParseAddress(addr); parseErr != nil {
			continue
		}

		// Peek at the node to see if healing is needed before acquiring
		// the write lock. Most nodes won't need changes.
		ns, readErr := d.Store.ReadNode(addr)
		if readErr != nil {
			continue
		}
		needsHeal := false
		for i := range ns.Tasks {
			t := &ns.Tasks[i]
			if t.State == state.StatusInProgress {
				needsHeal = true
				break
			}
			if derived, hasChildren := state.DeriveParentStatus(ns, t.ID); hasChildren && derived != t.State {
				needsHeal = true
				break
			}
			// Blocked audit with open gaps but no remediation subtasks:
			// the daemon crashed or exited before creating them.
			if t.IsAudit && t.State == state.StatusBlocked {
				hasOpenGaps := false
				for _, g := range ns.Audit.Gaps {
					if g.Status == state.GapOpen {
						hasOpenGaps = true
						break
					}
				}
				if hasOpenGaps {
					hasSubtasks := false
					prefix := t.ID + "."
					for _, other := range ns.Tasks {
						if len(other.ID) > len(prefix) && other.ID[:len(prefix)] == prefix {
							hasSubtasks = true
							break
						}
					}
					if !hasSubtasks {
						needsHeal = true
						break
					}
				}
			}
		}
		if !needsHeal {
			continue
		}

		mutErr := d.Store.MutateNode(addr, func(ns *state.NodeState) error {
			changed := false
			for i := range ns.Tasks {
				t := &ns.Tasks[i]
				if t.State == state.StatusInProgress {
					if derived, hasChildren := state.DeriveParentStatus(ns, t.ID); hasChildren {
						t.State = derived
						output.PrintHuman("  Derived %s/%s → %s", addr, t.ID, derived)
					} else {
						t.State = state.StatusNotStarted
						output.PrintHuman("  Reset %s/%s → not_started", addr, t.ID)
					}
					changed = true
					healed++
				} else if derived, hasChildren := state.DeriveParentStatus(ns, t.ID); hasChildren && derived != t.State {
					output.PrintHuman("  Derived %s/%s → %s (was %s)", addr, t.ID, derived, t.State)
					t.State = derived
					changed = true
					healed++
				}
			}
			// Blocked audit with open gaps but no remediation subtasks:
			// create the subtasks that should have been created when
			// the audit first blocked.
			for i := range ns.Tasks {
				t := &ns.Tasks[i]
				if !t.IsAudit || t.State != state.StatusBlocked {
					continue
				}
				prefix := t.ID + "."
				hasSubtasks := false
				for _, other := range ns.Tasks {
					if len(other.ID) > len(prefix) && other.ID[:len(prefix)] == prefix {
						hasSubtasks = true
						break
					}
				}
				if hasSubtasks {
					continue
				}
				subCount := 0
				for _, g := range ns.Audit.Gaps {
					if g.Status != state.GapOpen {
						continue
					}
					childID := fmt.Sprintf("%s.%04d", t.ID, subCount+1)
					ns.Tasks = append(ns.Tasks, state.Task{
						ID:          childID,
						Description: fmt.Sprintf("Fix: %s", g.Description),
						State:       state.StatusNotStarted,
					})
					subCount++
				}
				if subCount > 0 {
					// Re-index after append may have reallocated the slice.
					ns.Tasks[i].State = state.StatusNotStarted
					ns.Tasks[i].BlockedReason = ""
					output.PrintHuman("  Created %d remediation subtask(s) for %s/%s", subCount, addr, ns.Tasks[i].ID)
					changed = true
					healed += subCount
				}
			}

			if !changed {
				return errNoChange
			}
			return nil
		})
		if mutErr != nil && mutErr != errNoChange {
			output.PrintHuman("  Warning: could not save %s: %v", addr, mutErr)
		}
	}

	if healed > 0 {
		output.PrintHuman("Healed %d interrupted task(s).", healed)
	} else {
		output.PrintHuman("All clear. No interrupted tasks.")
	}
	return nil
}

// RunWithSupervisor wraps Run with crash recovery and configurable restarts.
func (d *Daemon) RunWithSupervisor(ctx context.Context) error {
	maxRestarts := d.Config.Daemon.MaxRestarts
	delay := time.Duration(d.Config.Daemon.RestartDelaySeconds) * time.Second

	sleepFn := d.SleepFunc
	if sleepFn == nil {
		sleepFn = time.Sleep
	}

	for restart := 0; ; restart++ {
		err := d.Run(ctx)
		if err == nil || ctx.Err() != nil {
			return err
		}
		if restart >= maxRestarts {
			return fmt.Errorf("daemon exceeded max restarts (%d): %w", maxRestarts, err)
		}
		output.PrintHuman("Crash (attempt %d/%d): %v. Restarting in %v.", restart+1, maxRestarts, err, delay)
		sleepFn(delay)

		// Close shutdown so goroutines from the previous Run() exit,
		// then wait for them to finish before resetting state.
		d.shutdownOnce.Do(func() { close(d.shutdown) })
		d.runWg.Wait()

		d.shutdown = make(chan struct{})
		d.shutdownOnce = sync.Once{}
		d.workAvailable = make(chan struct{}, 1)
		d.iteration = 0
	}
}

// IterationResult describes the outcome of a single daemon iteration.
type IterationResult int

const (
	// IterationDidWork means work was found and the pipeline ran.
	IterationDidWork IterationResult = iota
	// IterationNoWork means no actionable tasks were found.
	IterationNoWork
	// IterationStop means the daemon should shut down (signal, stop file, cap).
	IterationStop
	// IterationError means the iteration encountered a recoverable error.
	IterationError
)

// Run executes the daemon loop.
func (d *Daemon) Run(ctx context.Context) error {
	// Root the daemon in a cancelable signal context so shutdown signals
	// cancel in-flight model invocations (ADR-024 shutdown compliance).
	ctx, cancel := signal.NotifyContext(ctx, shutdownSignals...)
	defer cancel()

	// Dedicated signal channel as a backup. Child processes may corrupt
	// Go's signal infrastructure by leaving the terminal in raw mode
	// (ISIG off). RestoreTerminal re-enables ISIG after each invocation,
	// and this channel provides a second delivery path for signals.
	d.sigChan = make(chan os.Signal, 2)
	signal.Notify(d.sigChan, shutdownSignals...)
	defer signal.Stop(d.sigChan)
	d.runWg.Add(1)
	go func() {
		defer d.runWg.Done()
		for {
			select {
			case _, ok := <-d.sigChan:
				if !ok {
					return
				}
				output.PrintHuman("\n=== Wolfcastle standing down (signal) ===")
				cancel()
				d.shutdownOnce.Do(func() { close(d.shutdown) })
				go func() {
					time.Sleep(2 * time.Second)
					_ = d.repo().RemovePID()
					os.Exit(0)
				}()
				return
			case <-d.shutdown:
				return
			}
		}
	}()

	// Also close the shutdown channel when context cancels (covers
	// programmatic cancellation, not just signals).
	d.runWg.Add(1)
	go func() {
		defer d.runWg.Done()
		<-ctx.Done()
		d.shutdownOnce.Do(func() { close(d.shutdown) })
	}()

	d.iteration = 0
	_ = d.Logger.Log(map[string]any{"type": "daemon_start", "scope": d.scopeLabel()})
	output.PrintHuman("=== Wolfcastle engaged (scope=%s) ===", d.scopeLabel())

	// Self-healing phase (ADR-020)
	if err := d.selfHeal(); err != nil {
		return fmt.Errorf("self-healing failed: %w", err)
	}

	// Record starting branch (skip if not in a git repo)
	if d.Config.Git.VerifyBranch {
		var err error
		d.branch, err = currentBranch(d.RepoDir)
		if err != nil {
			output.PrintHuman("Not a git repository. Branch verification off.")
			d.Config.Git.VerifyBranch = false
		}
	}

	// Start the parallel inbox processing goroutine (ADR-064).
	// It watches for new inbox items and runs the intake stage
	// independently of the main execution loop.
	d.runWg.Add(1)
	go func() {
		defer d.runWg.Done()
		d.runInboxLoop(ctx)
	}()

	var idleSpinner *output.Spinner
	for {
		// Check for context cancellation before each iteration.
		// This catches signals that arrived during the previous
		// RunOnce call or while the spinner was animating.
		select {
		case <-ctx.Done():
			if idleSpinner != nil {
				idleSpinner.Stop()
			}
			return nil
		default:
		}

		// Stop the spinner before RunOnce so it doesn't animate
		// over stage log messages if the poll timeout raced with
		// workAvailable.
		if idleSpinner != nil {
			idleSpinner.Stop()
			idleSpinner = nil
		}

		result, err := d.RunOnce(ctx)
		if err != nil {
			return err
		}

		switch result {
		case IterationStop:
			return nil
		case IterationNoWork:
			// Start spinner on first idle cycle; keep it running
			// across poll timeouts to avoid a visible jitter every
			// BlockedPollIntervalSeconds.
			if idleSpinner == nil {
				idleSpinner = output.NewSpinner()
				idleSpinner.Start()
			}
			select {
			case <-ctx.Done():
				idleSpinner.Stop()
				return nil
			case <-d.shutdown:
				idleSpinner.Stop()
				return nil
			case <-d.workAvailable:
				// New work arrived from inbox goroutine
			case <-time.After(time.Duration(d.Config.Daemon.BlockedPollIntervalSeconds) * time.Second):
				// Poll timeout — loop back to RunOnce, spinner stays alive
				continue
			}
			// Leaving idle state: stop and discard spinner
			idleSpinner.Stop()
			idleSpinner = nil
		case IterationError:
			if !sleepWithContext(ctx, time.Duration(d.Config.Daemon.PollIntervalSeconds)*time.Second) {
				return nil
			}
		case IterationDidWork:
			retOpts := []logging.RetentionOption{}
			if d.Config.Logs.Compress {
				retOpts = append(retOpts, logging.WithCompression())
			}
			_ = logging.EnforceRetention(
				filepath.Join(d.WolfcastleDir, "system", "logs"),
				d.Config.Logs.MaxFiles,
				d.Config.Logs.MaxAgeDays,
				retOpts...,
			)
			// No sleep after successful work. If there's more to do,
			// the next iteration will find it immediately. The daemon
			// only sleeps when idle (NoWork) or recovering (Error).
		}
	}
}

// RunOnce executes a single daemon iteration: check preconditions, find work,
// and run the pipeline. Returns a result indicating what happened and a
// non-nil error only for fatal conditions that should halt the daemon.
func (d *Daemon) RunOnce(ctx context.Context) (IterationResult, error) {
	// Check shutdown signal
	select {
	case <-d.shutdown:
		_ = d.Logger.Log(map[string]any{"type": "daemon_stop", "reason": "signal"})
		output.PrintHuman("=== Wolfcastle standing down (signal) ===")
		return IterationStop, nil
	default:
	}

	// Check stop file
	if d.repo().HasStopFile() {
		_ = d.repo().RemoveStopFile()
		_ = d.Logger.Log(map[string]any{"type": "daemon_stop", "reason": "stop_file"})
		output.PrintHuman("=== Wolfcastle standing down (stop file) ===")
		return IterationStop, nil
	}

	// Max iterations check
	maxIter := d.Config.Daemon.MaxIterations
	if maxIter > 0 && d.iteration >= maxIter {
		_ = d.Logger.Log(map[string]any{"type": "daemon_stop", "reason": "iteration_cap", "iterations": d.iteration})
		output.PrintHuman("=== Iteration cap reached (%d) ===", maxIter)
		return IterationStop, nil
	}

	// Verify branch hasn't changed
	if d.Config.Git.VerifyBranch {
		current, err := currentBranch(d.RepoDir)
		if err == nil && current != d.branch {
			return IterationStop, fmt.Errorf("%s: branch changed from %s to %s", invoke.MarkerStringBlocked, d.branch, current)
		}
	}

	// Load the tree. Inbox processing runs in a parallel goroutine
	// (ADR-064), so the main loop handles execution and planning.
	idx, err := d.Store.ReadIndex()
	if err != nil {
		return IterationStop, werrors.Navigation(fmt.Errorf("loading root index: %w", err))
	}

	// Deliver any buffered pending scope from intake.
	d.deliverPendingScope(idx)

	// Reconcile orchestrator states before planning or navigation
	// decisions. Catches stale parent states (e.g., not_started while
	// all children are complete) that would otherwise persist until
	// the next daemon restart and selfHeal.
	d.reconcileOrchestratorStates(idx)

	nodeLoader := func(addr string) (*state.NodeState, error) {
		a, parseErr := tree.ParseAddress(addr)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing address %q: %w", addr, parseErr)
		}
		return state.LoadNodeState(filepath.Join(d.Store.Dir(), filepath.Join(a.Parts...), "state.json"))
	}

	// Step 1: Try to find an actionable task. Execute if found.
	navResult, err := state.FindNextTask(idx, d.ScopeNode, nodeLoader)
	if err != nil {
		return IterationStop, werrors.Navigation(fmt.Errorf("navigation failed: %w", err))
	}
	if ctx.Err() != nil {
		return IterationStop, nil
	}

	if navResult.Found {
		// Work available. Skip to execution below.
		goto execute
	}

	// Step 2: No actionable task. Plan the next childless orchestrator.
	// Planning is lazy: it only fires when navigation finds nothing to
	// execute. This means each orchestrator gets planned right before
	// its subtree needs work, not before.
	if planAddr, planNS := d.findPlanningTarget(idx); planAddr != "" {
		if err := d.runPlanningPass(ctx, planAddr, planNS, idx); err != nil {
			output.PrintHuman("Planning error: %v", err)
			return IterationError, nil
		}
		return IterationDidWork, nil
	}

	// Step 3: Nothing to execute, nothing to plan. Report why.
	{
		var msg string
		switch navResult.Reason {
		case "all_complete":
			msg = invoke.MarkerStringComplete
		case "empty_tree":
			msg = "Nothing to destroy. Feed the inbox."
		case "all_blocked":
			msg = "Blocked on all fronts. Human intervention required."
		default:
			msg = "Standing by. (" + navResult.Reason + ")"
		}
		if msg != d.lastNoWorkMsg {
			output.PrintHuman(msg)
			d.lastNoWorkMsg = msg
		}
		return IterationNoWork, nil
	}

execute:

	// Tree has work again; reset the dedup so next idle prints fresh.
	d.lastNoWorkMsg = ""

	d.iteration++
	output.PrintHuman("--- Iteration %d: %s/%s ---", d.iteration, navResult.NodeAddress, navResult.TaskID)

	// Start iteration log with "exec" trace prefix
	_ = d.Logger.StartIterationWithPrefix("exec")

	// Run pipeline stages
	err = d.runIteration(ctx, navResult, idx)
	d.Logger.Close()

	if err != nil {
		output.PrintHuman("Iteration error: %v", err)

		// State corruption is fatal: continuing risks further damage.
		var stateErr *werrors.StateError
		if errors.As(err, &stateErr) {
			return IterationStop, fmt.Errorf("fatal state error: %w", err)
		}

		return IterationError, nil
	}

	// After task completion, check if any orchestrator needs re-planning.
	// Re-read the index since the iteration may have changed node states.
	if d.Config.Pipeline.Planning.Enabled {
		freshIdx, readErr := d.Store.ReadIndex()
		if readErr == nil {
			d.checkReplanningTriggers(navResult.NodeAddress, navResult.TaskID, freshIdx)
		}
	}

	// If a spec task just completed, queue a review task so the spec
	// gets audited before it drives implementation.
	d.checkSpecReviewNeeded(navResult.NodeAddress, navResult.TaskID)

	return IterationDidWork, nil
}

// sleepWithContext sleeps for the given duration but returns immediately
// if the context is cancelled. Returns true if the full sleep completed,
// false if interrupted by context cancellation.
func sleepWithContext(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

func (d *Daemon) scopeLabel() string {
	if d.ScopeNode != "" {
		return d.ScopeNode
	}
	return "full tree"
}

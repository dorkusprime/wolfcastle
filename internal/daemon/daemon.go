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
//   - daemon.go   : Daemon struct, New, Run, RunWithSupervisor, RunOnce
//   - iteration.go  : per-iteration pipeline dispatch, terminal marker scanning
//   - stages.go     : intake stage handler, parallel inbox goroutine
//   - deliverables.go: deliverable file verification
//   - retry.go      : invocation retry with exponential backoff
//   - propagate.go  : state propagation via Store
//   - pid.go            : PID file operations
//   - signals_unix.go   : shutdown signals (Unix)
//   - signals_windows.go: shutdown signals (Windows)
package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/config"
	werrors "github.com/dorkusprime/wolfcastle/internal/errors"
	"github.com/dorkusprime/wolfcastle/internal/git"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/logging"
	"github.com/dorkusprime/wolfcastle/internal/output" // retained for idle spinner + New() warning
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
	Store          *state.Store
	ScopeNode      string
	Logger         *logging.Logger
	InboxLogger    *logging.Logger // separate logger for the inbox goroutine
	RepoDir        string
	Clock          clock.Clock
	Git            git.Provider
	ContextBuilder *pipeline.ContextBuilder
	ExitWhenDone   bool                // stop after all work is complete (--exit-when-done)
	SleepFunc      func(time.Duration) // override for testing; nil defaults to time.Sleep

	dispatcher       *ParallelDispatcher // nil when parallel mode is disabled
	mu               sync.Mutex          // protects lastNoWorkMsg and lastArchiveCheck
	gitMu            sync.Mutex          // serializes git commit operations across parallel workers
	hasWorked        bool                // tracks whether the daemon has done work this run
	draining         bool                // finish current work then exit
	shutdown         chan struct{}
	shutdownOnce     sync.Once
	workAvailable    chan struct{}
	sigChan          chan os.Signal
	runWg            sync.WaitGroup // tracks goroutines started by Run
	branch           string
	iteration        atomic.Int64
	lastNoWorkMsg    string    // dedup "no targets" / "WOLFCASTLE_COMPLETE" messages
	lastArchiveCheck time.Time // throttle auto-archive polling
}

// log is a nil-safe wrapper around d.Logger.Log. Tests that construct a
// Daemon without a Logger will silently drop records instead of panicking.
func (d *Daemon) log(record map[string]any) {
	if d.Logger != nil {
		_ = d.Logger.Log(record)
	}
}

// logInbox is a nil-safe wrapper around d.InboxLogger.Log.
func (d *Daemon) logInbox(record map[string]any) {
	if d.InboxLogger != nil {
		_ = d.InboxLogger.Log(record)
	}
}

// repo returns a DaemonRepository for the daemon's wolfcastle directory.
func (d *Daemon) repo() *DaemonRepository {
	return NewDaemonRepository(d.WolfcastleDir)
}

// namespace derives the engineer namespace from the daemon's config identity.
// Returns "" when identity is not configured.
func (d *Daemon) namespace() string {
	id, err := config.IdentityFromConfig(d.Config)
	if err != nil {
		return ""
	}
	return id.Namespace
}

// New creates a new daemon.
func New(cfg *config.Config, wolfcastleDir string, store *state.Store, scopeNode string, repoDir string) (*Daemon, error) {
	logDir := NewDaemonRepository(wolfcastleDir).LogDir()
	logger, err := logging.NewLogger(logDir)
	if err != nil {
		return nil, err
	}

	// Resume iteration numbering from existing log files.
	logger.Iteration = logging.IterationFromDir(logDir)

	// Create a separate logger for the inbox goroutine so it doesn't
	// race with the execute loop's logger on file handles and counters.
	inboxLogger, err := logging.NewLogger(logDir)
	if err != nil {
		return nil, err
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
	// output.PrintHuman retained here: the logger has no active iteration
	// inside New(), so structured logging is not yet available.
	if missing := classes.Validate(); len(missing) > 0 {
		output.PrintHuman("Warning: task classes with missing prompt files: %v", missing)
	}
	ctxBuilder := pipeline.NewContextBuilder(prompts, classes, wolfcastleDir)

	return &Daemon{
		Config:         cfg,
		WolfcastleDir:  wolfcastleDir,
		Store:          store,
		ScopeNode:      scopeNode,
		Logger:         logger,
		InboxLogger:    inboxLogger,
		RepoDir:        repoDir,
		Clock:          clock.New(),
		Git:            git.NewService(repoDir),
		ContextBuilder: ctxBuilder,
		shutdown:       make(chan struct{}),
		workAvailable:  make(chan struct{}, 1),
	}, nil
}

// errNoChange is a sentinel used by selfHeal to signal that MutateNode
// should skip writing when no tasks were actually modified.
var errNoChange = errors.New("no change")

// selfHeal scans the tree for stale in_progress tasks on startup (ADR-020).
func (d *Daemon) selfHeal() error {
	d.log(map[string]any{"type": "self_heal", "action": "scan_start", "text": "Scanning for casualties..."})
	idx, err := d.Store.ReadIndex()
	if err != nil {
		d.log(map[string]any{"type": "self_heal", "action": "no_index", "text": "No root index. Nothing to recover."})
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
						d.log(map[string]any{"type": "self_heal", "action": "derive", "text": fmt.Sprintf("Derived %s/%s → %s", addr, t.ID, derived)})
					} else {
						t.State = state.StatusNotStarted
						d.log(map[string]any{"type": "self_heal", "action": "reset", "text": fmt.Sprintf("Reset %s/%s → not_started", addr, t.ID)})
					}
					changed = true
					healed++
				} else if derived, hasChildren := state.DeriveParentStatus(ns, t.ID); hasChildren && derived != t.State {
					d.log(map[string]any{"type": "self_heal", "action": "derive", "text": fmt.Sprintf("Derived %s/%s → %s (was %s)", addr, t.ID, derived, t.State)})
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
				// No existing subtasks (confirmed by hasSubtasks check above),
				// so start numbering at 1.
				nextNum := 1
				subCount := 0
				for gi := range ns.Audit.Gaps {
					if ns.Audit.Gaps[gi].Status != state.GapOpen {
						continue
					}
					childID := fmt.Sprintf("%s.%04d", t.ID, nextNum)
					ns.Tasks = append(ns.Tasks, state.Task{
						ID:          childID,
						Description: fmt.Sprintf("Fix: %s\n\nAfter fixing, close the gap:\n  wolfcastle audit fix-gap --node %s %s", ns.Audit.Gaps[gi].Description, addr, ns.Audit.Gaps[gi].ID),
						State:       state.StatusNotStarted,
					})
					ns.Audit.Gaps[gi].RemediationTaskID = childID
					nextNum++
					subCount++
				}
				if subCount > 0 {
					// Re-index after append may have reallocated the slice.
					ns.Tasks[i].State = state.StatusNotStarted
					ns.Tasks[i].BlockedReason = ""
					// The node itself must transition to in_progress so navigation
					// can enter it and reach the new remediation subtasks.
					ns.State = state.StatusInProgress
					d.log(map[string]any{"type": "self_heal", "action": "remediation", "text": fmt.Sprintf("Created %d remediation subtask(s) for %s/%s", subCount, addr, ns.Tasks[i].ID)})
					changed = true
					healed += subCount
				}
			}

			if !changed {
				return errNoChange
			}
			return nil
		})
		if mutErr != nil && !errors.Is(mutErr, errNoChange) {
			d.log(map[string]any{"type": "self_heal", "action": "save_error", "text": fmt.Sprintf("Warning: could not save %s: %v", addr, mutErr)})
		}

		// If the node was healed and its state changed, update the root
		// index so navigation sees the new state immediately. Without
		// this, a node that was blocked (with remediation subtasks now
		// created) stays blocked in the index and dfs() skips it.
		if mutErr == nil {
			if updatedNS, readErr := d.Store.ReadNode(addr); readErr == nil {
				if e, ok := idx.Nodes[addr]; ok && e.State != updatedNS.State {
					e.State = updatedNS.State
					idx.Nodes[addr] = e
					_ = d.propagateState(addr, updatedNS.State, idx)
				}
			}
		}
	}

	if healed > 0 {
		d.log(map[string]any{"type": "self_heal", "action": "complete", "text": fmt.Sprintf("Healed %d interrupted task(s).", healed)})
	} else {
		d.log(map[string]any{"type": "self_heal", "action": "complete", "text": "All clear. No interrupted tasks."})
	}

	// Clean up stale scope locks left by dead processes or previous daemon runs.
	if err := d.cleanStaleScopeLocks(); err != nil {
		d.log(map[string]any{"type": "self_heal", "action": "scope_cleanup_error", "text": fmt.Sprintf("Warning: scope lock cleanup failed: %v", err)})
	}

	return nil
}

// cleanStaleScopeLocks removes scope locks held by processes that are no
// longer running. If every lock in the table belongs to a dead process,
// the entire file is removed as a leftover from a crashed run.
func (d *Daemon) cleanStaleScopeLocks() error {
	// Use WithLock so the staleness check, deletion, and optional file
	// removal all happen atomically under the advisory file lock.
	return d.Store.WithLock(func() error {
		table, err := d.Store.ReadScopeLocks()
		if err != nil {
			return err
		}
		if len(table.Locks) == 0 {
			return nil
		}

		myPID := os.Getpid()
		var staleScopes []string

		for scope, lock := range table.Locks {
			if lock.PID == myPID {
				continue
			}
			if !IsProcessRunning(lock.PID) {
				staleScopes = append(staleScopes, scope)
			}
		}

		if len(staleScopes) == 0 {
			return nil
		}

		for _, scope := range staleScopes {
			d.log(map[string]any{"type": "self_heal", "action": "scope_removed", "text": fmt.Sprintf("Removed stale scope lock: %s (PID %d dead)", scope, table.Locks[scope].PID)})
			delete(table.Locks, scope)
		}

		if len(table.Locks) == 0 {
			// All locks were stale; remove the file entirely while
			// still holding the lock to avoid a TOCTOU race.
			if rmErr := os.Remove(d.Store.ScopeLocksPath()); rmErr != nil && !os.IsNotExist(rmErr) {
				return rmErr
			}
			return nil
		}

		return state.SaveScopeLocks(d.Store.ScopeLocksPath(), table)
	})
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
		_ = d.Logger.StartIterationWithPrefix("crash")
		d.log(map[string]any{"type": "daemon_lifecycle", "event": "crash_restart", "attempt": restart + 1, "text": fmt.Sprintf("Crash (attempt %d/%d): %v. Restarting in %v.", restart+1, maxRestarts, err, delay)})
		d.Logger.Close()
		sleepFn(delay)

		// Close shutdown so goroutines from the previous Run() exit,
		// then wait for them to finish before resetting state.
		d.shutdownOnce.Do(func() { close(d.shutdown) })
		d.runWg.Wait()

		d.shutdown = make(chan struct{})
		d.shutdownOnce = sync.Once{}
		d.workAvailable = make(chan struct{}, 1)
		d.iteration.Store(0)
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

// RunInbox runs only the inbox processing loop, blocking until the context
// is cancelled. This is the non-daemon counterpart to the inbox goroutine
// that Run starts in the background: same intake pipeline, same fsnotify
// watcher with polling fallback, but without the main execution loop.
func (d *Daemon) RunInbox(ctx context.Context) {
	d.shutdown = make(chan struct{})
	d.workAvailable = make(chan struct{}, 1)
	go func() {
		<-ctx.Done()
		d.shutdownOnce.Do(func() { close(d.shutdown) })
	}()
	d.runInboxLoop(ctx)
}

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
				d.log(map[string]any{"type": "daemon_lifecycle", "event": "standing_down", "reason": "signal", "text": "Wolfcastle standing down (signal)"})
				cancel()
				d.shutdownOnce.Do(func() { close(d.shutdown) })
				go func() {
					time.Sleep(2 * time.Second)
					_ = d.Logger.Log(map[string]any{"type": "force_exit", "message": "signal handler force exit after 2s grace period"})
					_ = d.repo().RemovePID()
					d.removeActivityFile()
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

	defer d.removeActivityFile()

	d.iteration.Store(0)
	d.hasWorked = false

	// Open a "heal" iteration to capture the startup banner, self-heal
	// output, and git-check warning in a single structured log file.
	_ = d.Logger.StartIterationWithPrefix("heal")
	d.log(map[string]any{"type": "daemon_lifecycle", "event": "engaged", "scope": d.scopeLabel(), "text": fmt.Sprintf("Wolfcastle engaged (scope=%s)", d.scopeLabel())})

	// Self-healing phase (ADR-020)
	if err := d.selfHeal(); err != nil {
		d.Logger.Close()
		return fmt.Errorf("self-healing failed: %w", err)
	}

	// Record starting branch (skip if not in a git repo)
	if d.Config.Git.VerifyBranch {
		var err error
		d.branch, err = d.Git.CurrentBranch()
		if err != nil {
			d.log(map[string]any{"type": "config_warning", "text": "Not a git repository. Branch verification off."})
			d.Config.Git.VerifyBranch = false
		}
	}
	d.Logger.Close()

	// Initialize the parallel dispatcher when parallel mode is enabled.
	// This must happen after selfHeal (which cleans stale scope locks)
	// and before the main loop begins dispatching work.
	if d.Config.Daemon.Parallel.Enabled {
		d.dispatcher = NewParallelDispatcher(d, d.Config.Daemon.Parallel.MaxWorkers)
	} else {
		// Remove stale parallel status from a previous session so
		// `wolfcastle status` doesn't show misleading worker counts.
		_ = os.Remove(parallelStatusPath(d.WolfcastleDir))
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
			if d.draining {
				_ = d.Logger.StartIterationWithPrefix("shutdown")
				_ = d.Logger.Log(map[string]any{"type": "daemon_stop", "reason": "drain"})
				d.log(map[string]any{"type": "daemon_lifecycle", "event": "standing_down", "reason": "drain", "text": "Drain complete. Wolfcastle standing down."})
				return nil
			}
			if d.ExitWhenDone && d.hasWorked {
				_ = d.Logger.StartIterationWithPrefix("shutdown")
				_ = d.Logger.Log(map[string]any{"type": "daemon_stop", "reason": "exit_when_done"})
				d.log(map[string]any{"type": "daemon_lifecycle", "event": "standing_down", "reason": "exit_when_done", "text": "Work complete. Wolfcastle standing down."})
				return nil
			}
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
				// Poll timeout. Loop back to RunOnce, spinner stays alive
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
			d.hasWorked = true
			retOpts := []logging.RetentionOption{}
			if d.Config.Logs.Compress {
				retOpts = append(retOpts, logging.WithCompression())
			}
			_ = logging.EnforceRetention(
				d.repo().LogDir(),
				d.Config.Logs.MaxFiles,
				d.Config.Logs.MaxAgeDays,
				retOpts...,
			)
			if d.draining {
				_ = d.Logger.StartIterationWithPrefix("shutdown")
				_ = d.Logger.Log(map[string]any{"type": "daemon_stop", "reason": "drain"})
				d.log(map[string]any{"type": "daemon_lifecycle", "event": "standing_down", "reason": "drain", "text": "Drain complete. Wolfcastle standing down."})
				return nil
			}
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
		_ = d.Logger.StartIterationWithPrefix("shutdown")
		_ = d.Logger.Log(map[string]any{"type": "daemon_stop", "reason": "signal"})
		d.log(map[string]any{"type": "daemon_lifecycle", "event": "standing_down", "reason": "signal", "text": "Wolfcastle standing down (signal)"})
		return IterationStop, nil
	default:
	}

	// Check stop file
	if d.repo().HasStopFile() {
		_ = d.repo().RemoveStopFile()
		_ = d.Logger.StartIterationWithPrefix("shutdown")
		_ = d.Logger.Log(map[string]any{"type": "daemon_stop", "reason": "stop_file"})
		d.log(map[string]any{"type": "daemon_lifecycle", "event": "standing_down", "reason": "stop_file", "text": "Wolfcastle standing down (stop file)"})
		return IterationStop, nil
	}

	// Check drain file: finish current work then exit.
	if !d.draining && d.repo().HasDrainFile() {
		_ = d.repo().RemoveDrainFile()
		d.draining = true
		_ = d.Logger.StartIterationWithPrefix("lifecycle")
		_ = d.Logger.Log(map[string]any{"type": "daemon_drain"})
		d.log(map[string]any{"type": "daemon_lifecycle", "event": "drain", "text": "Drain mode: will exit after current work completes."})
	}

	// Max iterations check
	maxIter := d.Config.Daemon.MaxIterations
	if maxIter > 0 && d.iteration.Load() >= int64(maxIter) {
		_ = d.Logger.StartIterationWithPrefix("shutdown")
		_ = d.Logger.Log(map[string]any{"type": "daemon_stop", "reason": "iteration_cap", "iterations": d.iteration.Load()})
		d.log(map[string]any{"type": "daemon_lifecycle", "event": "standing_down", "reason": "iteration_cap", "text": fmt.Sprintf("Iteration cap reached (%d)", maxIter)})
		return IterationStop, nil
	}

	// Verify branch hasn't changed
	if d.Config.Git.VerifyBranch {
		current, err := d.Git.CurrentBranch()
		if err == nil && current != d.branch {
			// In parallel mode, cancel all active workers and wait for
			// them to drain before returning. This prevents orphaned
			// goroutines from writing to a stale branch.
			if d.dispatcher != nil {
				d.dispatcher.cancelAll()
				d.dispatcher.waitAndDrain()
			}
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

	// Parallel dispatch path: drain completed workers, fill open slots,
	// and fall through to planning only when the worker pool is empty.
	if d.dispatcher != nil {
		return d.runOnceParallel(ctx, idx)
	}

	// Serial dispatch path: find one task and execute it.
	return d.runOnceSerial(ctx, idx)
}

// runOnceSerial is the original single-task dispatch path. It finds the next
// actionable task, runs the pipeline, and returns.
func (d *Daemon) runOnceSerial(ctx context.Context, idx *state.RootIndex) (IterationResult, error) {
	nodeLoader := func(addr string) (*state.NodeState, error) {
		p, pathErr := d.Store.NodePath(addr)
		if pathErr != nil {
			return nil, fmt.Errorf("resolving address %q: %w", addr, pathErr)
		}
		return state.LoadNodeState(p)
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
			d.log(map[string]any{"type": "task_event", "action": "planning_error", "text": fmt.Sprintf("Planning error: %v", err), "error": err.Error()})
			return IterationError, nil
		}
		return IterationDidWork, nil
	}

	// Step 3: No tasks, no planning. Check for archive-eligible nodes.
	if d.tryAutoArchive(idx) {
		return IterationDidWork, nil
	}

	// Step 4: Nothing to execute, nothing to plan, nothing to archive.
	// Commit any lingering state changes (from reconciliation, archiving
	// on a prior tick, or propagation) so the tree is clean at idle.
	commitStateFlush(d.RepoDir, d.Logger, d.Config.Git)

	// Report why we're idle.
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
		d.mu.Lock()
		changed := msg != d.lastNoWorkMsg
		if changed {
			d.lastNoWorkMsg = msg
		}
		d.mu.Unlock()
		if changed {
			d.log(map[string]any{"type": "idle_reason", "reason": navResult.Reason, "text": msg})
		}
		return IterationNoWork, nil
	}

execute:

	// Tree has work again; reset the dedup so next idle prints fresh.
	d.mu.Lock()
	d.lastNoWorkMsg = ""
	d.mu.Unlock()

	d.iteration.Add(1)
	d.writeActivity(navResult.NodeAddress, navResult.TaskID)

	// Start iteration log with "exec" trace prefix
	_ = d.Logger.StartIterationWithPrefix("exec")
	d.log(map[string]any{"type": "iteration_header", "iteration": int(d.iteration.Load()), "kind": "execute", "text": fmt.Sprintf("%s/%s", navResult.NodeAddress, navResult.TaskID)})
	_ = d.Logger.LogIterationStart("execute", navResult.NodeAddress)

	// Run pipeline stages
	err = d.runIteration(ctx, navResult, idx)
	d.Logger.Close()

	if err != nil {
		d.log(map[string]any{"type": "task_event", "action": "iteration_error", "text": fmt.Sprintf("Iteration error: %v", err), "error": err.Error()})

		// State corruption is fatal: continuing risks further damage.
		var stateErr *werrors.StateError
		if errors.As(err, &stateErr) {
			return IterationStop, fmt.Errorf("fatal state error: %w", err)
		}

		return IterationError, nil
	}

	// Replanning triggers are now checked inside runIteration, before
	// propagation marks parent orchestrators complete. This ensures
	// findPlanningTarget can still find the orchestrator on the next pass.

	// If a spec task just completed, queue a review task so the spec
	// gets audited before it drives implementation.
	d.checkSpecReviewNeeded(navResult.NodeAddress, navResult.TaskID)

	// If the knowledge file exceeds its token budget, queue a
	// maintenance task to prune it.
	d.checkKnowledgeBudget(navResult.NodeAddress)

	return IterationDidWork, nil
}

// runOnceParallel is the multi-worker dispatch path. It drains completed
// workers, fills open slots, and falls through to planning only when the
// entire worker pool is empty (preventing plan-while-executing races).
func (d *Daemon) runOnceParallel(ctx context.Context, idx *state.RootIndex) (IterationResult, error) {
	pd := d.dispatcher

	// Step 1: Drain completed workers. Each result triggers a scoped
	// commit, scope release, and state propagation inside drainCompleted.
	completed := pd.drainCompleted()

	// Post-iteration hooks: check whether any successfully completed task
	// produced a spec that needs review or pushed a knowledge file over
	// its token budget. These mirror the serial path (lines 771-775).
	for _, wr := range completed {
		if wr.Error == nil && !wr.ScopeConflict {
			d.checkSpecReviewNeeded(wr.Node, wr.Task)
			d.checkKnowledgeBudget(wr.Node)
		}
	}

	// Step 2: Reclaim orphaned in_progress tasks whose workers are gone.
	pd.reclaimOrphans(idx)

	// Step 3: Fill open worker slots with eligible tasks.
	launched := pd.fillSlots(ctx, idx)

	// Step 4: Determine the iteration outcome.
	pd.mu.Lock()
	activeCount := len(pd.active)
	pd.mu.Unlock()

	if activeCount > 0 || launched > 0 {
		// Workers are running (or were just launched). Report progress
		// even if no new slots were filled this tick; the active workers
		// represent in-flight work.
		return IterationDidWork, nil
	}

	// Pool is empty and nothing was dispatched. Safe to run planning,
	// archiving, and state flush since no workers are modifying the tree.

	if planAddr, planNS := d.findPlanningTarget(idx); planAddr != "" {
		if err := d.runPlanningPass(ctx, planAddr, planNS, idx); err != nil {
			d.log(map[string]any{"type": "task_event", "action": "planning_error", "text": fmt.Sprintf("Planning error: %v", err), "error": err.Error()})
			return IterationError, nil
		}
		return IterationDidWork, nil
	}

	if d.tryAutoArchive(idx) {
		return IterationDidWork, nil
	}

	commitStateFlush(d.RepoDir, d.Logger, d.Config.Git)

	return IterationNoWork, nil
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

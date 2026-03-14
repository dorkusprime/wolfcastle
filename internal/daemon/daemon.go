// Package daemon implements the Wolfcastle daemon loop: finding actionable
// tasks via tree navigation, running pipeline stages (expand, file, execute),
// parsing model output markers, and propagating state changes to ancestor
// nodes and the root index. The daemon supports crash recovery via a
// supervisor wrapper, signal-driven graceful shutdown, stop-file detection,
// and configurable iteration caps.
//
// File layout follows ADR-045:
//
//   - daemon.go    — Daemon struct, New, Run, RunWithSupervisor, RunOnce
//   - iteration.go — per-iteration pipeline dispatch
//   - stages.go    — inbox-specific stage handlers (expand, file)
//   - markers.go   — WOLFCASTLE_* marker parsing and state mutation
//   - retry.go     — invocation retry with exponential backoff
//   - propagate.go — state propagation and inbox helpers
//   - branch.go    — git branch detection
//   - pid.go       — PID file operations
package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/logging"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// Daemon is the main Wolfcastle daemon loop.
type Daemon struct {
	Config        *config.Config
	WolfcastleDir string
	Resolver      *tree.Resolver
	ScopeNode     string
	Logger        *logging.Logger
	RepoDir       string
	Clock         clock.Clock

	shutdown     chan struct{}
	shutdownOnce sync.Once
	branch       string
	iteration    int
}

// New creates a new daemon.
func New(cfg *config.Config, wolfcastleDir string, resolver *tree.Resolver, scopeNode string, repoDir string) (*Daemon, error) {
	logDir := filepath.Join(wolfcastleDir, "logs")
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

	return &Daemon{
		Config:        cfg,
		WolfcastleDir: wolfcastleDir,
		Resolver:      resolver,
		ScopeNode:     scopeNode,
		Logger:        logger,
		RepoDir:       repoDir,
		Clock:         clock.New(),
		shutdown:      make(chan struct{}),
	}, nil
}

// selfHeal scans the tree for stale in_progress tasks on startup (ADR-020).
func (d *Daemon) selfHeal() error {
	output.PrintHuman("Running self-healing check...")
	idx, err := d.Resolver.LoadRootIndex()
	if err != nil {
		output.PrintHuman("No root index found — nothing to heal.")
		return nil
	}

	var inProgress []struct{ addr, taskID string }
	for addr, entry := range idx.Nodes {
		if entry.Type != state.NodeLeaf {
			continue
		}
		a, err := tree.ParseAddress(addr)
		if err != nil {
			continue
		}
		ns, err := state.LoadNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json"))
		if err != nil {
			continue
		}
		for _, t := range ns.Tasks {
			if t.State == state.StatusInProgress {
				inProgress = append(inProgress, struct{ addr, taskID string }{addr, t.ID})
			}
		}
	}

	if len(inProgress) > 1 {
		return fmt.Errorf("state corruption: %d tasks in progress (serial execution requires at most 1)", len(inProgress))
	}
	if len(inProgress) == 1 {
		output.PrintHuman("Found interrupted task: %s/%s — will resume on next iteration",
			inProgress[0].addr, inProgress[0].taskID)
	} else {
		output.PrintHuman("No interrupted tasks found.")
	}
	return nil
}

// RunWithSupervisor wraps Run with crash recovery and configurable restarts.
func (d *Daemon) RunWithSupervisor(ctx context.Context) error {
	maxRestarts := d.Config.Daemon.MaxRestarts
	delay := time.Duration(d.Config.Daemon.RestartDelaySeconds) * time.Second

	for restart := 0; ; restart++ {
		err := d.Run(ctx)
		if err == nil || ctx.Err() != nil {
			return err
		}
		if restart >= maxRestarts {
			return fmt.Errorf("daemon exceeded max restarts (%d): %w", maxRestarts, err)
		}
		output.PrintHuman("Daemon crashed (attempt %d/%d): %v — restarting in %v", restart+1, maxRestarts, err, delay)
		time.Sleep(delay)

		// Reset daemon state for next Run() invocation.
		// Resetting sync.Once is safe here because all goroutines from
		// the previous Run() have exited — the signal-forwarding goroutine
		// terminates when ctx.Done() closes, and Run() has returned.
		d.shutdown = make(chan struct{})
		d.shutdownOnce = sync.Once{}
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
	// Root the daemon in a cancelable signal context so SIGINT/SIGTERM
	// cancels in-flight model invocations (ADR-024 shutdown compliance).
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Also close the shutdown channel for backward compatibility with
	// stop-file and supervisor checks.
	go func() {
		<-ctx.Done()
		d.shutdownOnce.Do(func() { close(d.shutdown) })
	}()

	// Self-healing phase (ADR-020)
	if err := d.selfHeal(); err != nil {
		return fmt.Errorf("self-healing failed: %w", err)
	}

	// Record starting branch
	if d.Config.Git.VerifyBranch {
		var err error
		d.branch, err = currentBranch(d.RepoDir)
		if err != nil {
			return fmt.Errorf("getting current branch: %w", err)
		}
	}

	d.iteration = 0
	_ = d.Logger.Log(map[string]any{"type": "daemon_start", "scope": d.scopeLabel()})
	output.PrintHuman("=== Wolfcastle starting (scope=%s) ===", d.scopeLabel())

	for {
		result, err := d.RunOnce(ctx)
		if err != nil {
			return err
		}

		switch result {
		case IterationStop:
			return nil
		case IterationNoWork:
			time.Sleep(time.Duration(d.Config.Daemon.BlockedPollIntervalSeconds) * time.Second)
		case IterationError:
			time.Sleep(time.Duration(d.Config.Daemon.PollIntervalSeconds) * time.Second)
		case IterationDidWork:
			retOpts := []logging.RetentionOption{}
			if d.Config.Logs.Compress {
				retOpts = append(retOpts, logging.WithCompression())
			}
			_ = logging.EnforceRetention(
				filepath.Join(d.WolfcastleDir, "logs"),
				d.Config.Logs.MaxFiles,
				d.Config.Logs.MaxAgeDays,
				retOpts...,
			)
			time.Sleep(time.Duration(d.Config.Daemon.PollIntervalSeconds) * time.Second)
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
		output.PrintHuman("=== Wolfcastle stopped by signal ===")
		return IterationStop, nil
	default:
	}

	// Check stop file
	stopFilePath := filepath.Join(d.WolfcastleDir, "stop")
	if _, err := os.Stat(stopFilePath); err == nil {
		_ = os.Remove(stopFilePath)
		_ = d.Logger.Log(map[string]any{"type": "daemon_stop", "reason": "stop_file"})
		output.PrintHuman("=== Wolfcastle stopped by stop file ===")
		return IterationStop, nil
	}

	// Max iterations check
	maxIter := d.Config.Daemon.MaxIterations
	if maxIter > 0 && d.iteration >= maxIter {
		_ = d.Logger.Log(map[string]any{"type": "daemon_stop", "reason": "iteration_cap", "iterations": d.iteration})
		output.PrintHuman("=== Wolfcastle hit iteration cap (%d) ===", maxIter)
		return IterationStop, nil
	}

	// Verify branch hasn't changed
	if d.Config.Git.VerifyBranch {
		current, err := currentBranch(d.RepoDir)
		if err == nil && current != d.branch {
			return IterationStop, fmt.Errorf("WOLFCASTLE_BLOCKED: branch changed from %s to %s", d.branch, current)
		}
	}

	// Navigate to find work
	idx, err := d.Resolver.LoadRootIndex()
	if err != nil {
		return IterationStop, fmt.Errorf("loading root index: %w", err)
	}

	navResult, err := state.FindNextTask(idx, d.ScopeNode, func(addr string) (*state.NodeState, error) {
		a, err := tree.ParseAddress(addr)
		if err != nil {
			return nil, fmt.Errorf("parsing address %q: %w", addr, err)
		}
		return state.LoadNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json"))
	})
	if err != nil {
		return IterationStop, fmt.Errorf("navigation failed: %w", err)
	}

	if !navResult.Found {
		if navResult.Reason == "all_complete" {
			output.PrintHuman("WOLFCASTLE_COMPLETE")
		} else {
			output.PrintHuman("No work: %s — sleeping %ds", navResult.Reason, d.Config.Daemon.BlockedPollIntervalSeconds)
		}
		return IterationNoWork, nil
	}

	d.iteration++
	output.PrintHuman("--- Iteration %d: %s/%s ---", d.iteration, navResult.NodeAddress, navResult.TaskID)

	// Start iteration log
	_ = d.Logger.StartIteration()

	// Run pipeline stages
	err = d.runIteration(ctx, navResult, idx)
	d.Logger.Close()

	if err != nil {
		output.PrintHuman("Iteration error: %v", err)
		return IterationError, nil
	}

	return IterationDidWork, nil
}

func (d *Daemon) scopeLabel() string {
	if d.ScopeNode != "" {
		return d.ScopeNode
	}
	return "full tree"
}

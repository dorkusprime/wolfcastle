package daemon

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ErrYieldScopeConflict is returned by runIteration when a worker yields
// with a scope_conflict suffix. It carries the addresses of the yielding
// task and the task that holds the conflicting scope locks.
type ErrYieldScopeConflict struct {
	Task    string // address of the task that yielded
	Blocker string // address of the task holding the conflicting locks
}

func (e *ErrYieldScopeConflict) Error() string {
	return fmt.Sprintf("scope conflict: %s blocked by %s", e.Task, e.Blocker)
}

// ParallelDispatcher coordinates concurrent task execution across multiple
// worker slots. It serializes git operations through the parent Daemon's gitMu
// and tracks yield-backoff conflicts in its blocked map.
type ParallelDispatcher struct {
	daemon     *Daemon
	maxWorkers int
	active     map[string]*WorkerSlot // task address -> slot
	mu         sync.Mutex
	results    chan WorkerResult
	blocked    map[string]string // task address -> conflicting task address (yield backoff)
}

// WorkerSlot represents a single active worker executing a task on a node.
type WorkerSlot struct {
	Node   string
	Task   string
	Cancel context.CancelFunc
}

// WorkerResult captures the outcome of a single worker iteration.
type WorkerResult struct {
	Node          string
	Task          string
	Result        IterationResult
	Error         error
	ScopeConflict bool   // true when the worker yielded due to a scope conflict
	Blocker       string // address of the task holding the conflicting scope locks
}

// NewParallelDispatcher creates a ParallelDispatcher bound to the given Daemon.
// The results channel is buffered to maxWorkers so that completing workers
// never block on sends.
func NewParallelDispatcher(d *Daemon, maxWorkers int) *ParallelDispatcher {
	return &ParallelDispatcher{
		daemon:     d,
		maxWorkers: maxWorkers,
		active:     make(map[string]*WorkerSlot),
		results:    make(chan WorkerResult, maxWorkers),
		blocked:    make(map[string]string),
	}
}

// runWorker launches a single task in a goroutine. The caller must ensure
// that pd.daemon.runWg.Add(1) is called before this method returns, which
// it handles internally before spawning the goroutine. This prevents a
// race where runWg.Wait() completes before the goroutine calls Add().
func (pd *ParallelDispatcher) runWorker(ctx context.Context, nav *state.NavigationResult, idx *state.RootIndex) {
	taskAddr := nav.NodeAddress + "/" + nav.TaskID
	workerCtx, cancel := context.WithCancel(ctx)

	logger := pd.daemon.Logger.Child(fmt.Sprintf("worker-%s", nav.TaskID))

	pd.mu.Lock()
	pd.active[taskAddr] = &WorkerSlot{
		Node:   nav.NodeAddress,
		Task:   nav.TaskID,
		Cancel: cancel,
	}
	pd.mu.Unlock()

	pd.daemon.runWg.Add(1)
	go func() {
		defer pd.daemon.runWg.Done()
		defer cancel()

		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				_ = logger.Log(map[string]any{
					"type":  "worker_panic",
					"task":  taskAddr,
					"panic": fmt.Sprintf("%v", r),
					"stack": string(stack),
				})
				pd.results <- WorkerResult{
					Node:   nav.NodeAddress,
					Task:   nav.TaskID,
					Result: IterationError,
					Error:  fmt.Errorf("worker panic: %v", r),
				}
			}
			// Scope release and active-map cleanup are handled by
			// drainCompleted, which needs the scope intact for
			// scoped commits under gitMu.
		}()

		_ = logger.StartIteration()
		_ = logger.LogIterationStart("execute", nav.NodeAddress)

		err := pd.daemon.runIteration(workerCtx, nav, idx)
		logger.Close()

		wr := WorkerResult{
			Node: nav.NodeAddress,
			Task: nav.TaskID,
		}

		var scopeErr *ErrYieldScopeConflict
		switch {
		case errors.As(err, &scopeErr):
			// Scope-conflict yield: not a failure, just a scheduling conflict.
			wr.Result = IterationDidWork
			wr.ScopeConflict = true
			wr.Blocker = scopeErr.Blocker
		case err != nil:
			wr.Result = IterationError
			wr.Error = err
		default:
			wr.Result = IterationDidWork
		}

		pd.results <- wr
	}()
}

// drainCompleted performs a non-blocking read of all available results from the
// results channel. For each completed worker it reads the task's scope from the
// lock table, commits scoped changes under gitMu, releases scope locks, removes
// the worker from the active map, and clears any blocked entries that were
// waiting on the completed task.
func (pd *ParallelDispatcher) drainCompleted() []WorkerResult {
	var collected []WorkerResult
	for {
		select {
		case wr := <-pd.results:
			collected = append(collected, wr)
		default:
			goto done
		}
	}
done:

	d := pd.daemon
	for _, wr := range collected {
		taskAddr := wr.Node + "/" + wr.Task

		// Read the task's scope before releasing locks. The scope file
		// list feeds commitAfterIteration so only this worker's files
		// are staged.
		scope := pd.scopeFiles(taskAddr)

		// Read node state for commit metadata and state propagation.
		ns, _ := d.Store.ReadNode(wr.Node)

		switch {
		case wr.ScopeConflict:
			// Scope-conflict yield: record the conflict so fillSlots
			// skips this task while the blocker is still active. Do not
			// increment the failure count (this is a scheduling conflict,
			// not an execution failure).
			pd.mu.Lock()
			pd.blocked[taskAddr] = wr.Blocker
			pd.mu.Unlock()

			// Commit any partial work the agent produced before yielding.
			if d.Config.Git.CommitOnFailure {
				d.gitMu.Lock()
				meta := extractTaskCommitMeta(ns, wr.Task)
				commitAfterIteration(d.RepoDir, d.Logger, wr.Task, "failure", 0, d.Config.Git, meta, scope)
				d.gitMu.Unlock()
			}

			// Release scope locks so the blocker (or other tasks) can
			// acquire files this task was holding.
			pd.releaseScope(taskAddr)

			// Reset the task to not_started so it is eligible for
			// re-dispatch once the blocker completes. Also recalculate
			// the node-level state so the index entry reflects the
			// task reset (without this, the index stays in_progress
			// and FindParallelTasks skips the node).
			if err := d.Store.MutateNode(wr.Node, func(mns *state.NodeState) error {
				for i, t := range mns.Tasks {
					if t.ID == wr.Task {
						mns.Tasks[i].State = state.StatusNotStarted
						break
					}
				}
				// Derive node state from tasks.
				hasInProgress := false
				for _, t := range mns.Tasks {
					if t.State == state.StatusInProgress {
						hasInProgress = true
						break
					}
				}
				if !hasInProgress {
					mns.State = state.StatusNotStarted
				}
				return nil
			}); err != nil {
				// State write failed: the task remains in_progress permanently
				// because fillSlots will never rediscover it. Log so operators
				// can intervene.
				_ = d.Logger.Log(map[string]any{
					"type":  "scope_conflict_reset_error",
					"task":  taskAddr,
					"node":  wr.Node,
					"error": err.Error(),
				})
			}

			// Remove from the active map but do NOT clear blocked entries
			// (this task yielded; it did not complete).
			pd.mu.Lock()
			delete(pd.active, taskAddr)
			pd.mu.Unlock()

		case wr.Error == nil:
			// Success path: commit scoped changes under gitMu, then
			// propagate the node's state up through parent orchestrators.
			d.gitMu.Lock()
			meta := extractTaskCommitMeta(ns, wr.Task)
			commitAfterIteration(d.RepoDir, d.Logger, wr.Task, "success", 0, d.Config.Git, meta, scope)
			d.gitMu.Unlock()

			if ns != nil {
				// Re-read to pick up any state mutations runIteration applied.
				if updated, err := d.Store.ReadNode(wr.Node); err == nil {
					idx, idxErr := d.Store.ReadIndex()
					if idxErr == nil {
						_ = d.propagateState(wr.Node, updated.State, idx)
					}
				}
			}

			// Release scope locks and clean up active/blocked state.
			pd.releaseScope(taskAddr)
			pd.mu.Lock()
			delete(pd.active, taskAddr)
			for blocked, blocker := range pd.blocked {
				if blocker == taskAddr {
					delete(pd.blocked, blocked)
				}
			}
			pd.mu.Unlock()

		default:
			// Failure path: increment the failure count and commit
			// partial work so retries don't redo completed portions.
			var failCount int
			if err := d.Store.MutateNode(wr.Node, func(mns *state.NodeState) error {
				var err error
				failCount, err = state.IncrementFailure(mns, wr.Task)
				return err
			}); err != nil {
				// State write failed: failure count was not incremented, so
				// the task may retry indefinitely or remain stuck. Log so
				// operators can intervene.
				_ = d.Logger.Log(map[string]any{
					"type":  "failure_increment_error",
					"task":  taskAddr,
					"node":  wr.Node,
					"error": err.Error(),
				})
			}

			d.gitMu.Lock()
			failMeta := extractTaskCommitMeta(ns, wr.Task)
			commitAfterIteration(d.RepoDir, d.Logger, wr.Task, "failure", failCount, d.Config.Git, failMeta, scope)
			d.gitMu.Unlock()

			// Release scope locks and clean up active/blocked state.
			pd.releaseScope(taskAddr)
			pd.mu.Lock()
			delete(pd.active, taskAddr)
			for blocked, blocker := range pd.blocked {
				if blocker == taskAddr {
					delete(pd.blocked, blocked)
				}
			}
			pd.mu.Unlock()
		}
	}

	return collected
}

// fillSlots finds eligible parallel tasks and launches workers for them,
// up to the number of available slots. Returns the count of workers launched.
func (pd *ParallelDispatcher) fillSlots(ctx context.Context, idx *state.RootIndex) int {
	pd.mu.Lock()
	available := pd.maxWorkers - len(pd.active)
	pd.mu.Unlock()

	if available <= 0 {
		return 0
	}

	d := pd.daemon
	nodeLoader := func(addr string) (*state.NodeState, error) {
		p, err := d.Store.NodePath(addr)
		if err != nil {
			return nil, fmt.Errorf("resolving address %q: %w", addr, err)
		}
		return state.LoadNodeState(p)
	}

	tasks, err := state.FindParallelTasks(idx, d.ScopeNode, nodeLoader, available)
	if err != nil {
		_ = d.Logger.Log(map[string]any{
			"type":  "fill_slots_error",
			"error": err.Error(),
		})
		return 0
	}

	launched := 0
	for _, nav := range tasks {
		taskAddr := nav.NodeAddress + "/" + nav.TaskID

		// Skip tasks whose blocker is still running.
		if pd.isBlocked(taskAddr) {
			continue
		}

		// Claim the task (not_started -> in_progress).
		claimErr := d.Store.MutateNode(nav.NodeAddress, func(ns *state.NodeState) error {
			return state.TaskClaim(ns, nav.TaskID)
		})
		if claimErr != nil {
			_ = d.Logger.Log(map[string]any{
				"type":  "claim_error",
				"task":  taskAddr,
				"error": claimErr.Error(),
			})
			continue
		}

		pd.runWorker(ctx, nav, idx)
		launched++
	}

	return launched
}

// isBlocked checks whether taskAddr is in the blocked map with an active
// blocker. If the blocker has already been drained (no longer in pd.active),
// the stale blocked entry is removed and the task is eligible for dispatch.
func (pd *ParallelDispatcher) isBlocked(taskAddr string) bool {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	blocker, ok := pd.blocked[taskAddr]
	if !ok {
		return false
	}
	if _, active := pd.active[blocker]; active {
		return true
	}
	// Blocker finished; clear the stale entry.
	delete(pd.blocked, taskAddr)
	return false
}

// cancelAll cancels every active worker's context. Used during branch
// verification failure to stop in-flight work before the daemon exits.
func (pd *ParallelDispatcher) cancelAll() {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	for _, slot := range pd.active {
		slot.Cancel()
	}
}

// waitAndDrain waits for all running worker goroutines to finish, then
// drains their results. This ensures no goroutines are orphaned when the
// daemon shuts down due to a branch change.
func (pd *ParallelDispatcher) waitAndDrain() {
	pd.daemon.runWg.Wait()
	pd.drainCompleted()
}

// releaseScope deletes all scope locks held by the given task address.
func (pd *ParallelDispatcher) releaseScope(taskAddr string) {
	_ = pd.daemon.Store.MutateScopeLocks(func(table *state.ScopeLockTable) error {
		for file, lock := range table.Locks {
			if lock.Task == taskAddr {
				delete(table.Locks, file)
			}
		}
		return nil
	})
}

// scopeFiles reads the scope lock table and returns the list of files
// locked by the given task. Returns nil if no locks are held or the
// table cannot be read.
func (pd *ParallelDispatcher) scopeFiles(taskAddr string) []string {
	table, err := pd.daemon.Store.ReadScopeLocks()
	if err != nil {
		return nil
	}
	var files []string
	for file, lock := range table.Locks {
		if lock.Task == taskAddr {
			files = append(files, file)
		}
	}
	return files
}

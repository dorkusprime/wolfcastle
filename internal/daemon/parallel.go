package daemon

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

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
	Node   string
	Task   string
	Result IterationResult
	Error  error
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

		result := IterationDidWork
		if err != nil {
			result = IterationError
		}

		pd.results <- WorkerResult{
			Node:   nav.NodeAddress,
			Task:   nav.TaskID,
			Result: result,
			Error:  err,
		}
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

		default:
			// Failure path: increment the failure count and commit
			// partial work so retries don't redo completed portions.
			var failCount int
			_ = d.Store.MutateNode(wr.Node, func(mns *state.NodeState) error {
				var err error
				failCount, err = state.IncrementFailure(mns, wr.Task)
				return err
			})

			d.gitMu.Lock()
			failMeta := extractTaskCommitMeta(ns, wr.Task)
			commitAfterIteration(d.RepoDir, d.Logger, wr.Task, "failure", failCount, d.Config.Git, failMeta, scope)
			d.gitMu.Unlock()
		}

		// Release scope locks now that the commit is done.
		pd.releaseScope(taskAddr)

		// Remove from the active map.
		pd.mu.Lock()
		delete(pd.active, taskAddr)

		// Unblock any yielded siblings that were waiting on this task.
		for blocked, blocker := range pd.blocked {
			if blocker == taskAddr {
				delete(pd.blocked, blocked)
			}
		}
		pd.mu.Unlock()
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
		pd.mu.Lock()
		if blocker, blocked := pd.blocked[taskAddr]; blocked {
			if _, active := pd.active[blocker]; active {
				pd.mu.Unlock()
				continue
			}
			// Blocker finished; clear the stale entry.
			delete(pd.blocked, taskAddr)
		}
		pd.mu.Unlock()

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

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

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

// BlockedEntry records why a task is blocked and how many times it has yielded.
type BlockedEntry struct {
	Blocker        string    // address of the task holding the conflicting locks
	YieldCount     int       // number of times this task has yielded due to scope conflict
	FirstBlockedAt time.Time // when the task first entered the blocked state
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
	blocked    map[string]*BlockedEntry // task address -> block details (yield backoff)
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
		blocked:    make(map[string]*BlockedEntry),
	}
}

// runWorker launches a single task in a goroutine. The caller must ensure
// that pd.daemon.runWg.Add(1) is called before this method returns, which
// it handles internally before spawning the goroutine. This prevents a
// race where runWg.Wait() completes before the goroutine calls Add().
func (pd *ParallelDispatcher) runWorker(ctx context.Context, nav *state.NavigationResult, idx *state.RootIndex) {
	taskAddr := nav.NodeAddress + "/" + nav.TaskID
	workerCtx, cancel := context.WithCancel(ctx)

	// Include the slugified node address in the prefix so workers on
	// different nodes with the same task ID (e.g., "task-0001") don't
	// collide on the same filename. The Child logger's iteration
	// counter starts at zero per instance, so without the node slug
	// every worker with the same TaskID would produce the identical
	// "0001-worker-{TaskID}-{ts}.jsonl" path, stomping each other's
	// writes.
	nodeSlug := strings.ReplaceAll(nav.NodeAddress, "/", "-")
	logger := pd.daemon.Logger.Child(fmt.Sprintf("worker-%s-%s", nodeSlug, nav.TaskID))

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
		// StartIteration stamps the full slugified prefix onto TraceID,
		// which makes each record carry a 100+ character trace field
		// (the same string we use for the filename to guarantee unique
		// paths across workers). Override to a compact form — the last
		// segment of the node address plus the task ID — so the log
		// view's [trace] column stays readable. Collisions in the
		// compact form are fine; records already carry the full node
		// address for disambiguation.
		shortNode := nav.NodeAddress
		if idx := strings.LastIndex(shortNode, "/"); idx >= 0 {
			shortNode = shortNode[idx+1:]
		}
		logger.TraceID = fmt.Sprintf("worker-%s-%s-%04d", shortNode, nav.TaskID, logger.Iteration)
		_ = logger.LogIterationStart("execute", nav.NodeAddress)

		err := pd.daemon.runIteration(workerCtx, logger, nav, idx)
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
		// A ReadNode failure (corrupted state, disk I/O) yields a nil ns;
		// callers below must guard against that rather than crashing.
		ns, nsErr := d.Store.ReadNode(wr.Node)
		if nsErr != nil {
			_ = d.Logger.Log(map[string]any{
				"type":  "read_node_error",
				"task":  taskAddr,
				"node":  wr.Node,
				"error": nsErr.Error(),
			})
		}

		switch {
		case wr.ScopeConflict:
			// Scope-conflict yield: record the conflict so fillSlots
			// skips this task while the blocker is still active. Do not
			// increment the failure count (this is a scheduling conflict,
			// not an execution failure).
			pd.mu.Lock()
			if existing, ok := pd.blocked[taskAddr]; ok {
				existing.Blocker = wr.Blocker
				existing.YieldCount++
			} else {
				pd.blocked[taskAddr] = &BlockedEntry{
					Blocker:        wr.Blocker,
					YieldCount:     1,
					FirstBlockedAt: time.Now(),
				}
			}
			entry := pd.blocked[taskAddr]
			pd.mu.Unlock()

			_ = d.Logger.Log(map[string]any{
				"type":        "scope_yield_tracking",
				"task":        taskAddr,
				"blocker":     wr.Blocker,
				"yield_count": entry.YieldCount,
				"blocked_for": time.Since(entry.FirstBlockedAt).String(),
			})

			// Commit any partial work the agent produced before yielding.
			if d.Config.Git.CommitOnFailure {
				d.gitMu.Lock()
				var meta taskCommitMeta
				if ns != nil {
					meta = extractTaskCommitMeta(ns, wr.Task)
				}
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
			var meta taskCommitMeta
			if ns != nil {
				meta = extractTaskCommitMeta(ns, wr.Task)
			}
			commitAfterIteration(d.RepoDir, d.Logger, wr.Task, "success", 0, d.Config.Git, meta, scope)
			d.gitMu.Unlock()

			if ns != nil {
				// Re-read to pick up any state mutations runIteration applied.
				if updated, err := d.Store.ReadNode(wr.Node); err == nil {
					idx, idxErr := d.Store.ReadIndex()
					if idxErr == nil {
						// The worker's logger is already closed at this
						// point; drainCompleted runs back in the main
						// loop. The fallback-diagnostic record in
						// propagateState only fires if the index can't
						// be re-read, so dropping it to d.Logger is a
						// small regression if no parent file is open.
						_ = d.propagateState(d.Logger, wr.Node, updated.State, idx)
					}
				}
			}

			// Release scope locks and clean up active/blocked state.
			pd.releaseScope(taskAddr)
			pd.mu.Lock()
			delete(pd.active, taskAddr)
			for blocked, entry := range pd.blocked {
				if entry.Blocker == taskAddr {
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
			var failMeta taskCommitMeta
			if ns != nil {
				failMeta = extractTaskCommitMeta(ns, wr.Task)
			}
			commitAfterIteration(d.RepoDir, d.Logger, wr.Task, "failure", failCount, d.Config.Git, failMeta, scope)
			d.gitMu.Unlock()

			// Release scope locks and clean up active/blocked state.
			pd.releaseScope(taskAddr)
			pd.mu.Lock()
			delete(pd.active, taskAddr)
			for blocked, entry := range pd.blocked {
				if entry.Blocker == taskAddr {
					delete(pd.blocked, blocked)
				}
			}
			pd.mu.Unlock()
		}
	}

	pd.writeStatusSnapshot()
	return collected
}

// reclaimOrphans finds in_progress tasks with no active worker and resets
// them to not_started. This handles the case where a worker was lost (stall
// kill, daemon restart) and its task was never cleaned up. Without this,
// orphaned tasks stay in_progress forever because fillSlots can't claim
// them and the navigator returns them ahead of not_started tasks.
func (pd *ParallelDispatcher) reclaimOrphans(idx *state.RootIndex) int {
	d := pd.daemon
	reclaimed := 0

	for addr, entry := range idx.Nodes {
		if entry.Type != state.NodeLeaf {
			continue
		}
		if entry.State != state.StatusInProgress {
			continue
		}

		ns, err := d.Store.ReadNode(addr)
		if err != nil {
			continue
		}

		for _, task := range ns.Tasks {
			if task.State != state.StatusInProgress {
				continue
			}
			// Parent tasks (with children) derive their status; skip them.
			if state.TaskChildren(ns, task.ID) {
				continue
			}

			taskAddr := addr + "/" + task.ID
			pd.mu.Lock()
			_, active := pd.active[taskAddr]
			pd.mu.Unlock()
			if active {
				continue
			}

			// This task is in_progress but no worker owns it. Reset it.
			if err := d.Store.MutateNode(addr, func(mns *state.NodeState) error {
				for i := range mns.Tasks {
					if mns.Tasks[i].ID == task.ID && mns.Tasks[i].State == state.StatusInProgress {
						mns.Tasks[i].State = state.StatusNotStarted
						break
					}
				}
				mns.State = state.RecomputeState(mns.Children, mns.Tasks)
				return nil
			}); err != nil {
				continue
			}

			_ = d.Logger.Log(map[string]any{
				"type": "reclaim_orphan",
				"task": taskAddr,
				"node": addr,
			})
			reclaimed++
		}
	}

	return reclaimed
}

// fillSlots finds eligible parallel tasks and launches workers for them,
// up to the number of available slots. Returns the count of workers launched.
func (pd *ParallelDispatcher) fillSlots(ctx context.Context, idx *state.RootIndex) int {
	// In drain mode, don't launch new workers. Let active ones finish.
	if pd.daemon.draining {
		return 0
	}

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

		// Skip tasks already running in the pool.
		pd.mu.Lock()
		_, alreadyActive := pd.active[taskAddr]
		pd.mu.Unlock()
		if alreadyActive {
			continue
		}

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

	if launched > 0 {
		pd.writeStatusSnapshot()
	}
	return launched
}

// isBlocked checks whether taskAddr is in the blocked map with an active
// blocker. If the blocker has already been drained (no longer in pd.active),
// the stale blocked entry is removed and the task is eligible for dispatch.
func (pd *ParallelDispatcher) isBlocked(taskAddr string) bool {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	entry, ok := pd.blocked[taskAddr]
	if !ok {
		return false
	}
	if _, active := pd.active[entry.Blocker]; active {
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
		if slot.Cancel != nil {
			slot.Cancel()
		}
	}
}

// waitAndDrain waits for all running worker goroutines to finish, then
// drains their results. This ensures no goroutines are orphaned when the
// daemon shuts down due to a branch change.
func (pd *ParallelDispatcher) waitAndDrain() {
	pd.daemon.runWg.Wait()
	pd.drainCompleted()
	pd.removeStatusFile()
}

// releaseScope deletes all scope locks held by the given task address.
func (pd *ParallelDispatcher) releaseScope(taskAddr string) {
	if err := pd.daemon.Store.MutateScopeLocks(func(table *state.ScopeLockTable) error {
		for file, lock := range table.Locks {
			if lock.Task == taskAddr {
				delete(table.Locks, file)
			}
		}
		return nil
	}); err != nil {
		_ = pd.daemon.Logger.Log(map[string]any{
			"type":  "scope_release_error",
			"task":  taskAddr,
			"error": err.Error(),
		})
	}
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

// ParallelStatus is a snapshot of the dispatcher's state, written to disk
// so that `wolfcastle status` can display worker pool information without
// needing to communicate with the running daemon process.
type ParallelStatus struct {
	MaxWorkers int                    `json:"max_workers"`
	Active     []ParallelWorkerEntry  `json:"active"`
	Yielded    []ParallelYieldedEntry `json:"yielded"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// ParallelWorkerEntry describes a single active worker in the status snapshot.
type ParallelWorkerEntry struct {
	Task  string   `json:"task"`
	Node  string   `json:"node"`
	Scope []string `json:"scope,omitempty"`
}

// ParallelYieldedEntry describes a task that yielded due to a scope conflict.
type ParallelYieldedEntry struct {
	Task           string `json:"task"`
	Blocker        string `json:"blocker"`
	YieldCount     int    `json:"yield_count"`
	BlockedForSecs int    `json:"blocked_for_secs"`
}

// snapshot captures the current dispatcher state as a ParallelStatus.
func (pd *ParallelDispatcher) snapshot() *ParallelStatus {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	now := time.Now()
	status := &ParallelStatus{
		MaxWorkers: pd.maxWorkers,
		Active:     make([]ParallelWorkerEntry, 0, len(pd.active)),
		Yielded:    make([]ParallelYieldedEntry, 0, len(pd.blocked)),
		UpdatedAt:  now,
	}

	for taskAddr, slot := range pd.active {
		entry := ParallelWorkerEntry{
			Task: taskAddr,
			Node: slot.Node,
		}
		// Scope is filled by writeStatusSnapshot after this method returns
		// and pd.mu is released, since scopeFiles acquires its own lock.
		status.Active = append(status.Active, entry)
	}
	sort.Slice(status.Active, func(i, j int) bool { return status.Active[i].Task < status.Active[j].Task })

	for taskAddr, blocked := range pd.blocked {
		status.Yielded = append(status.Yielded, ParallelYieldedEntry{
			Task:           taskAddr,
			Blocker:        blocked.Blocker,
			YieldCount:     blocked.YieldCount,
			BlockedForSecs: int(now.Sub(blocked.FirstBlockedAt).Seconds()),
		})
	}
	sort.Slice(status.Yielded, func(i, j int) bool { return status.Yielded[i].Task < status.Yielded[j].Task })

	return status
}

// writeStatusSnapshot writes the current dispatcher state to
// parallel-status.json in the .wolfcastle/system/ directory. The status
// command reads this file to display worker pool information. Errors are
// logged but do not interrupt the dispatch cycle.
func (pd *ParallelDispatcher) writeStatusSnapshot() {
	status := pd.snapshot()

	// Fill scope information outside the pd.mu lock.
	for i, entry := range status.Active {
		status.Active[i].Scope = pd.scopeFiles(entry.Task)
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		_ = pd.daemon.Logger.Log(map[string]any{
			"type":  "parallel_status_error",
			"error": err.Error(),
		})
		return
	}

	statusPath := parallelStatusPath(pd.daemon.WolfcastleDir)
	if err := state.AtomicWriteFile(statusPath, data); err != nil {
		_ = pd.daemon.Logger.Log(map[string]any{
			"type":  "parallel_status_write_error",
			"error": err.Error(),
		})
	}
}

// removeStatusFile removes the parallel-status.json file. Called when the
// dispatcher shuts down so stale status doesn't persist.
func (pd *ParallelDispatcher) removeStatusFile() {
	if err := os.Remove(parallelStatusPath(pd.daemon.WolfcastleDir)); err != nil && !os.IsNotExist(err) {
		_ = pd.daemon.Logger.Log(map[string]any{
			"type":  "parallel_status_remove_error",
			"error": err.Error(),
		})
	}
}

// parallelStatusPath returns the path to the parallel status snapshot file.
func parallelStatusPath(wolfcastleDir string) string {
	return filepath.Join(NewRepository(wolfcastleDir).systemDir, "parallel-status.json")
}

// LoadParallelStatus reads the parallel status snapshot from disk.
// Returns nil if the file doesn't exist or can't be parsed.
func LoadParallelStatus(wolfcastleDir string) *ParallelStatus {
	data, err := os.ReadFile(parallelStatusPath(wolfcastleDir))
	if err != nil {
		return nil
	}
	var status ParallelStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil
	}
	return &status
}

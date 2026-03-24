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

			pd.releaseScope(taskAddr)

			pd.mu.Lock()
			delete(pd.active, taskAddr)
			pd.mu.Unlock()
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

// releaseScope releases scope locks held by the given task address.
// This is a placeholder that will be wired to the Store's scope lock
// table once the full dispatch loop is implemented.
func (pd *ParallelDispatcher) releaseScope(taskAddr string) {
	_ = taskAddr // TODO: release scope locks via pd.daemon.Store
}

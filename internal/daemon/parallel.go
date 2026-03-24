package daemon

import (
	"context"
	"sync"
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

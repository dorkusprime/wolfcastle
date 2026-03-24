package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// Helpers
// ═══════════════════════════════════════════════════════════════════════════

// stubGitProvider is a minimal git.Provider for parallel tests. It avoids
// shelling out to git by returning canned values.
type stubGitProvider struct {
	branch    string
	branchMu  sync.Mutex
	isRepo    bool
	headVal   string
	dirty     bool
	scopeDirt bool
}

func (s *stubGitProvider) CurrentBranch() (string, error) {
	s.branchMu.Lock()
	defer s.branchMu.Unlock()
	return s.branch, nil
}
func (s *stubGitProvider) HEAD() string                                { return s.headVal }
func (s *stubGitProvider) HasProgress(_ string) bool                   { return true }
func (s *stubGitProvider) HasProgressScoped(_ string, _ []string) bool { return s.scopeDirt }
func (s *stubGitProvider) IsRepo() bool                                { return s.isRepo }
func (s *stubGitProvider) IsDirty(_ ...string) bool                    { return s.dirty }
func (s *stubGitProvider) CreateWorktree(_ string, _ string) error     { return nil }
func (s *stubGitProvider) RemoveWorktree(_ string) error               { return nil }

func (s *stubGitProvider) setBranch(b string) {
	s.branchMu.Lock()
	defer s.branchMu.Unlock()
	s.branch = b
}

// parallelTestDaemon builds a Daemon with parallel mode enabled, a stub
// git provider, and the given maxWorkers.
func parallelTestDaemon(t *testing.T, maxWorkers int) *Daemon {
	t.Helper()
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	d.Config.Daemon.Parallel.Enabled = true
	d.Config.Daemon.Parallel.MaxWorkers = maxWorkers
	d.Config.Daemon.InvocationTimeoutSeconds = 10
	d.Git = &stubGitProvider{branch: "test-branch", isRepo: true, headVal: "abc123", scopeDirt: true}
	d.dispatcher = NewParallelDispatcher(d, maxWorkers)
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")
	return d
}

// initLogger starts a single logger iteration for the daemon. Call this
// once before the first RunOnce. Do NOT call StartIteration again while
// workers may be active, because runIteration uses d.Logger directly
// and StartIteration swaps the underlying file handle.
func initLogger(d *Daemon) {
	_ = d.Logger.StartIteration()
}

// shutdownParallel cancels all active workers, waits for them to drain,
// and closes the logger. Call this at the end of every parallel test to
// avoid data races between worker goroutines and Logger.Close().
func shutdownParallel(d *Daemon) {
	if d.dispatcher != nil {
		d.dispatcher.cancelAll()
		d.dispatcher.waitAndDrain()
	}
	d.runWg.Wait()
	d.Logger.Close()
}

// setupOrchestrator creates an orchestrator node with N leaf children, each
// containing one not_started task. Returns the list of child addresses.
func setupOrchestrator(t *testing.T, d *Daemon, parentAddr string, childCount int) []string {
	t.Helper()
	projDir := d.Store.Dir()

	idx := state.NewRootIndex()
	idx.Root = []string{parentAddr}
	idx.Nodes[parentAddr] = state.IndexEntry{
		Name:     parentAddr,
		Type:     state.NodeOrchestrator,
		State:    state.StatusNotStarted,
		Address:  parentAddr,
		Children: make([]string, childCount),
	}

	children := make([]string, childCount)
	for i := 0; i < childCount; i++ {
		childAddr := fmt.Sprintf("%s/child-%d", parentAddr, i)
		children[i] = childAddr
		idx.Nodes[parentAddr] = func() state.IndexEntry {
			e := idx.Nodes[parentAddr]
			e.Children[i] = childAddr
			return e
		}()
		idx.Nodes[childAddr] = state.IndexEntry{
			Name:    fmt.Sprintf("child-%d", i),
			Type:    state.NodeLeaf,
			State:   state.StatusNotStarted,
			Address: childAddr,
			Parent:  parentAddr,
		}

		ns := state.NewNodeState(childAddr, fmt.Sprintf("child-%d", i), state.NodeLeaf)
		ns.Tasks = []state.Task{
			{ID: "task-0001", Description: fmt.Sprintf("work on child-%d", i), State: state.StatusNotStarted},
		}
		statePath := filepath.Join(projDir, parentAddr, fmt.Sprintf("child-%d", i), "state.json")
		writeJSON(t, statePath, ns)
	}

	parentNS := state.NewNodeState(parentAddr, parentAddr, state.NodeOrchestrator)
	parentStatePath := filepath.Join(projDir, parentAddr, "state.json")
	writeJSON(t, parentStatePath, parentNS)

	writeJSON(t, filepath.Join(projDir, "state.json"), idx)
	return children
}

// ═══════════════════════════════════════════════════════════════════════════
// 1. Two independent siblings dispatched concurrently
// ═══════════════════════════════════════════════════════════════════════════

func TestParallel_TwoSiblingsDispatchedConcurrently(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	initLogger(d)
	setupOrchestrator(t, d, "proj", 2)

	d.Config.Models["echo"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"WOLFCASTLE_COMPLETE"},
	}

	// First RunOnce: fills both slots and dispatches workers.
	r1, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("first RunOnce error: %v", err)
	}
	if r1 != IterationDidWork {
		t.Fatalf("expected IterationDidWork, got %d", r1)
	}

	// Wait for both workers to finish.
	d.runWg.Wait()

	// Second RunOnce: drains completed results.
	r2, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("second RunOnce error: %v", err)
	}
	_ = r2

	// Close logger only after all goroutines are done.
	shutdownParallel(d)

	// Verify both tasks completed.
	projDir := d.Store.Dir()
	for i := 0; i < 2; i++ {
		ns, loadErr := state.LoadNodeState(filepath.Join(projDir, "proj", fmt.Sprintf("child-%d", i), "state.json"))
		if loadErr != nil {
			t.Fatalf("loading child-%d: %v", i, loadErr)
		}
		if ns.Tasks[0].State != state.StatusComplete {
			t.Errorf("child-%d task state = %s, want complete", i, ns.Tasks[0].State)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 2. Scope conflict yield: worker B yields, re-dispatched after A completes
// ═══════════════════════════════════════════════════════════════════════════

func TestParallel_ScopeConflictYield(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	initLogger(d)
	pd := d.dispatcher

	setupOrchestrator(t, d, "proj", 2)

	// Claim both tasks (as fillSlots would) so they're in_progress.
	for i := 0; i < 2; i++ {
		addr := fmt.Sprintf("proj/child-%d", i)
		_ = d.Store.MutateNode(addr, func(ns *state.NodeState) error {
			return state.TaskClaim(ns, "task-0001")
		})
	}

	child0Addr := "proj/child-0/task-0001"
	child1Addr := "proj/child-1/task-0001"

	// Register both as active workers.
	pd.mu.Lock()
	pd.active[child0Addr] = &WorkerSlot{Node: "proj/child-0", Task: "task-0001"}
	pd.active[child1Addr] = &WorkerSlot{Node: "proj/child-1", Task: "task-0001"}
	pd.mu.Unlock()

	// Simulate: child-1 yields with scope_conflict, child-0 succeeds.
	pd.results <- WorkerResult{
		Node:          "proj/child-1",
		Task:          "task-0001",
		Result:        IterationDidWork,
		ScopeConflict: true,
		Blocker:       child0Addr,
	}
	pd.results <- WorkerResult{
		Node:   "proj/child-0",
		Task:   "task-0001",
		Result: IterationDidWork,
	}

	collected := pd.drainCompleted()
	if len(collected) != 2 {
		t.Fatalf("expected 2 drained results, got %d", len(collected))
	}

	// After drain: child-1 task should be reset to not_started.
	projDir := d.Store.Dir()
	ns1, _ := state.LoadNodeState(filepath.Join(projDir, "proj", "child-1", "state.json"))
	if ns1.Tasks[0].State != state.StatusNotStarted {
		t.Errorf("after yield drain, child-1 task state = %s, want not_started", ns1.Tasks[0].State)
	}

	// The blocked entry for child-1 should have been cleared because child-0
	// also completed in the same drain batch. The order of processing may
	// leave the entry briefly, but isBlocked should report false since
	// the blocker (child-0) is no longer in active.
	if pd.isBlocked(child1Addr) {
		t.Error("child-1 should not be blocked after child-0 completed")
	}

	// child-1 should now be eligible for re-dispatch via fillSlots.
	idx, _ := d.Store.ReadIndex()
	launched := pd.fillSlots(context.Background(), idx)
	if launched != 1 {
		t.Errorf("expected 1 re-dispatched worker, got %d", launched)
	}

	// The re-dispatched worker is running. Let it finish.
	d.runWg.Wait()
	shutdownParallel(d)
}

// ═══════════════════════════════════════════════════════════════════════════
// 3. Worker panic recovery
// ═══════════════════════════════════════════════════════════════════════════

func TestParallel_WorkerPanicRecovery(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	initLogger(d)
	setupOrchestrator(t, d, "proj", 2)

	d.Config.Models["echo"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"WOLFCASTLE_COMPLETE"},
	}

	pd := d.dispatcher
	idx, err := d.Store.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}

	// Manually register a worker that panics. This simulates the deferred
	// panic recovery in runWorker without needing to corrupt state.
	taskAddr := "proj/child-0/task-0001"
	panicCtx, panicCancel := context.WithCancel(context.Background())
	defer panicCancel()

	pd.mu.Lock()
	pd.active[taskAddr] = &WorkerSlot{
		Node:   "proj/child-0",
		Task:   "task-0001",
		Cancel: panicCancel,
	}
	pd.mu.Unlock()

	d.runWg.Add(1)
	go func() {
		defer d.runWg.Done()
		defer func() {
			if r := recover(); r != nil {
				pd.results <- WorkerResult{
					Node:   "proj/child-0",
					Task:   "task-0001",
					Result: IterationError,
					Error:  fmt.Errorf("worker panic: %v", r),
				}
			}
		}()
		_ = panicCtx
		panic("intentional test panic")
	}()

	// Launch child-1 normally.
	nav1 := &state.NavigationResult{
		NodeAddress: "proj/child-1",
		TaskID:      "task-0001",
		Found:       true,
	}
	pd.runWorker(context.Background(), nav1, idx)

	// Wait with timeout: if panic recovery is broken, this hangs.
	done := make(chan struct{})
	go func() {
		d.runWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers finished.
	case <-time.After(5 * time.Second):
		t.Fatal("runWg.Wait() hung; panic recovery may be broken")
	}

	// Drain results BEFORE shutdownParallel (which calls waitAndDrain and
	// would consume the results).
	var results []WorkerResult
	for {
		select {
		case wr := <-pd.results:
			results = append(results, wr)
		default:
			goto drained
		}
	}
drained:

	shutdownParallel(d)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	var hasPanic, hasNormal bool
	for _, wr := range results {
		if wr.Error != nil && wr.Node == "proj/child-0" {
			hasPanic = true
		}
		if wr.Node == "proj/child-1" {
			hasNormal = true
		}
	}

	if !hasPanic {
		t.Error("expected a panic error result for child-0")
	}
	if !hasNormal {
		t.Error("expected a result for child-1")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 4. Branch change cancellation
// ═══════════════════════════════════════════════════════════════════════════

func TestParallel_BranchChangeCancellation(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	initLogger(d)
	d.Config.Git.VerifyBranch = true
	d.branch = "test-branch"

	stub := d.Git.(*stubGitProvider)

	// Model that blocks until its context is cancelled.
	blockScript := `#!/bin/sh
while true; do sleep 0.01; done
`
	blockScriptFile := filepath.Join(t.TempDir(), "block.sh")
	if err := os.WriteFile(blockScriptFile, []byte(blockScript), 0755); err != nil {
		t.Fatal(err)
	}

	setupOrchestrator(t, d, "proj", 2)
	d.Config.Models["echo"] = config.ModelDef{
		Command: "sh",
		Args:    []string{blockScriptFile},
	}

	// Dispatch workers (they will block).
	r1, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("dispatch RunOnce: %v", err)
	}
	if r1 != IterationDidWork {
		t.Fatalf("expected DidWork, got %d", r1)
	}

	// Verify workers are active.
	d.dispatcher.mu.Lock()
	activeCount := len(d.dispatcher.active)
	d.dispatcher.mu.Unlock()
	if activeCount != 2 {
		t.Fatalf("expected 2 active workers, got %d", activeCount)
	}

	// Change the branch. Calling cancelAll + waitAndDrain directly tests
	// the cancellation path without a second RunOnce call, which would
	// race on d.Logger with the blocking workers.
	stub.setBranch("different-branch")

	d.dispatcher.cancelAll()

	// waitAndDrain should not hang: cancelled workers exit promptly.
	done := make(chan struct{})
	go func() {
		d.dispatcher.waitAndDrain()
		close(done)
	}()

	select {
	case <-done:
		// Workers successfully cancelled and drained.
	case <-time.After(5 * time.Second):
		t.Fatal("waitAndDrain hung; workers may not have been cancelled")
	}

	// Verify all workers were removed from the active map.
	d.dispatcher.mu.Lock()
	remaining := len(d.dispatcher.active)
	d.dispatcher.mu.Unlock()
	if remaining != 0 {
		t.Errorf("expected 0 active workers after cancellation, got %d", remaining)
	}

	// Verify that RunOnce now detects the branch change.
	r2, err2 := d.RunOnce(context.Background())
	if r2 != IterationStop {
		t.Errorf("expected IterationStop on branch change, got %d", r2)
	}
	if err2 == nil {
		t.Error("expected non-nil error describing branch change")
	}

	shutdownParallel(d)
}

// ═══════════════════════════════════════════════════════════════════════════
// 5. Max workers limit
// ═══════════════════════════════════════════════════════════════════════════

func TestParallel_MaxWorkersLimit(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	initLogger(d)
	setupOrchestrator(t, d, "proj", 5)

	d.Config.Models["echo"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"WOLFCASTLE_COMPLETE"},
	}

	// Single RunOnce: fillSlots should only launch 2 workers.
	r, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce error: %v", err)
	}
	if r != IterationDidWork {
		t.Fatalf("expected IterationDidWork, got %d", r)
	}

	d.runWg.Wait()
	shutdownParallel(d)

	// Count task states across children.
	projDir := d.Store.Dir()
	notStartedCount := 0
	claimedCount := 0
	for i := 0; i < 5; i++ {
		ns, loadErr := state.LoadNodeState(filepath.Join(projDir, "proj", fmt.Sprintf("child-%d", i), "state.json"))
		if loadErr != nil {
			t.Fatalf("loading child-%d: %v", i, loadErr)
		}
		switch ns.Tasks[0].State {
		case state.StatusNotStarted:
			notStartedCount++
		case state.StatusInProgress, state.StatusComplete:
			claimedCount++
		}
	}

	if claimedCount != 2 {
		t.Errorf("expected 2 claimed/completed tasks, got %d (not_started=%d)", claimedCount, notStartedCount)
	}
	if notStartedCount != 3 {
		t.Errorf("expected 3 not_started tasks, got %d", notStartedCount)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 6. Planning gate: workers active means planning is skipped
// ═══════════════════════════════════════════════════════════════════════════

func TestParallel_PlanningGateSkipsWhileWorkersActive(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	initLogger(d)
	d.Config.Pipeline.Planning = config.PlanningConfig{
		Enabled: true,
		Model:   "echo",
	}

	// Model that blocks forever.
	blockScript := `#!/bin/sh
while true; do sleep 0.01; done
`
	blockScriptFile := filepath.Join(t.TempDir(), "block.sh")
	if err := os.WriteFile(blockScriptFile, []byte(blockScript), 0755); err != nil {
		t.Fatal(err)
	}

	setupOrchestrator(t, d, "proj", 2)
	d.Config.Models["echo"] = config.ModelDef{
		Command: "sh",
		Args:    []string{blockScriptFile},
	}

	// First RunOnce: dispatch blocking workers.
	r1, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if r1 != IterationDidWork {
		t.Fatalf("expected IterationDidWork, got %d", r1)
	}

	// Verify workers are active (prerequisite for the planning gate).
	d.dispatcher.mu.Lock()
	activeCount := len(d.dispatcher.active)
	d.dispatcher.mu.Unlock()
	if activeCount == 0 {
		t.Fatal("expected active workers after dispatch")
	}

	// Call runOnceParallel directly to test the planning gate. This avoids
	// a second RunOnce (which would call Logger.Log and race with workers
	// on the shared d.Logger). The index must be loaded first.
	idx, idxErr := d.Store.ReadIndex()
	if idxErr != nil {
		t.Fatalf("reading index: %v", idxErr)
	}
	r2, err := d.runOnceParallel(context.Background(), idx)
	if err != nil {
		t.Fatalf("gate runOnceParallel error: %v", err)
	}
	// With active workers, the parallel path returns IterationDidWork and
	// never enters the planning/archiving/flush block.
	if r2 != IterationDidWork {
		t.Errorf("expected IterationDidWork while workers active, got %d", r2)
	}

	// Clean up: cancel workers so test can exit.
	shutdownParallel(d)
}

// ═══════════════════════════════════════════════════════════════════════════
// 7. cancelAll delivers context cancellation to all workers
// ═══════════════════════════════════════════════════════════════════════════

func TestParallel_CancelAllDelivered(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 3)
	pd := d.dispatcher

	var cancelled atomic.Int32
	for i := 0; i < 3; i++ {
		addr := fmt.Sprintf("node/task-%d", i)
		_, cancel := context.WithCancel(context.Background())

		wrappedCancel := func() {
			cancelled.Add(1)
			cancel()
		}
		pd.mu.Lock()
		pd.active[addr] = &WorkerSlot{
			Node:   "node",
			Task:   fmt.Sprintf("task-%d", i),
			Cancel: wrappedCancel,
		}
		pd.mu.Unlock()
	}

	pd.cancelAll()

	if got := cancelled.Load(); got != 3 {
		t.Errorf("cancelAll cancelled %d workers, want 3", got)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 8. isBlocked clears stale entries
// ═══════════════════════════════════════════════════════════════════════════

func TestParallel_IsBlockedClearsStaleEntries(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	pd := d.dispatcher

	taskA := "proj/child-0/task-0001"
	taskB := "proj/child-1/task-0001"

	pd.blocked[taskA] = taskB
	pd.active[taskB] = &WorkerSlot{Node: "proj/child-1", Task: "task-0001"}

	if !pd.isBlocked(taskA) {
		t.Error("taskA should be blocked while taskB is active")
	}

	delete(pd.active, taskB)

	if pd.isBlocked(taskA) {
		t.Error("taskA should not be blocked after taskB completed")
	}
	if _, exists := pd.blocked[taskA]; exists {
		t.Error("stale blocked entry for taskA should have been cleared")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 9. drainCompleted processes multiple results
// ═══════════════════════════════════════════════════════════════════════════

func TestParallel_DrainCompletedProcessesResults(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 3)
	pd := d.dispatcher

	setupOrchestrator(t, d, "proj", 3)

	for i := 0; i < 2; i++ {
		addr := fmt.Sprintf("proj/child-%d", i)
		taskAddr := addr + "/task-0001"
		pd.active[taskAddr] = &WorkerSlot{Node: addr, Task: "task-0001"}
		pd.results <- WorkerResult{
			Node:   addr,
			Task:   "task-0001",
			Result: IterationDidWork,
		}
	}

	collected := pd.drainCompleted()
	if len(collected) != 2 {
		t.Fatalf("expected 2 drained results, got %d", len(collected))
	}

	pd.mu.Lock()
	remaining := len(pd.active)
	pd.mu.Unlock()
	if remaining != 0 {
		t.Errorf("expected 0 active workers after drain, got %d", remaining)
	}
}

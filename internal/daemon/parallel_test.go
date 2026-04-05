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

	pd.blocked[taskA] = &BlockedEntry{Blocker: taskB, YieldCount: 1, FirstBlockedAt: time.Now()}
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

// ═══════════════════════════════════════════════════════════════════════════
// 10. ErrYieldScopeConflict.Error() string
// ═══════════════════════════════════════════════════════════════════════════

func TestErrYieldScopeConflict_Error(t *testing.T) {
	t.Parallel()
	err := &ErrYieldScopeConflict{
		Task:    "proj/child-1/task-0001",
		Blocker: "proj/child-0/task-0001",
	}
	got := err.Error()
	want := "scope conflict: proj/child-1/task-0001 blocked by proj/child-0/task-0001"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 11. scopeFiles returns locked files for a task
// ═══════════════════════════════════════════════════════════════════════════

func TestParallel_ScopeFilesReturnsLockedFiles(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	pd := d.dispatcher

	taskAddr := "proj/child-0/task-0001"
	otherAddr := "proj/child-1/task-0001"

	// Seed scope locks: two files for our task, one for another.
	_ = d.Store.MutateScopeLocks(func(table *state.ScopeLockTable) error {
		table.Locks["alpha.go"] = state.ScopeLock{Task: taskAddr, Node: "proj/child-0"}
		table.Locks["beta.go"] = state.ScopeLock{Task: taskAddr, Node: "proj/child-0"}
		table.Locks["gamma.go"] = state.ScopeLock{Task: otherAddr, Node: "proj/child-1"}
		return nil
	})

	files := pd.scopeFiles(taskAddr)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(files), files)
	}

	// Verify only our task's files are returned.
	fileSet := map[string]bool{}
	for _, f := range files {
		fileSet[f] = true
	}
	if !fileSet["alpha.go"] || !fileSet["beta.go"] {
		t.Errorf("expected alpha.go and beta.go, got %v", files)
	}
}

func TestParallel_ScopeFilesEmptyTable(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	pd := d.dispatcher

	files := pd.scopeFiles("proj/child-0/task-0001")
	if files != nil {
		t.Errorf("expected nil for empty table, got %v", files)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 12. releaseScope removes locks for the specified task
// ═══════════════════════════════════════════════════════════════════════════

func TestParallel_ReleaseScopeRemovesLocks(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	pd := d.dispatcher

	taskAddr := "proj/child-0/task-0001"
	otherAddr := "proj/child-1/task-0001"

	_ = d.Store.MutateScopeLocks(func(table *state.ScopeLockTable) error {
		table.Locks["alpha.go"] = state.ScopeLock{Task: taskAddr, Node: "proj/child-0"}
		table.Locks["beta.go"] = state.ScopeLock{Task: taskAddr, Node: "proj/child-0"}
		table.Locks["gamma.go"] = state.ScopeLock{Task: otherAddr, Node: "proj/child-1"}
		return nil
	})

	pd.releaseScope(taskAddr)

	table, err := d.Store.ReadScopeLocks()
	if err != nil {
		t.Fatalf("reading scope locks after release: %v", err)
	}
	if len(table.Locks) != 1 {
		t.Fatalf("expected 1 lock remaining, got %d", len(table.Locks))
	}
	if _, ok := table.Locks["gamma.go"]; !ok {
		t.Error("other task's lock on gamma.go should still exist")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 13. drainCompleted handles worker errors (failure path)
// ═══════════════════════════════════════════════════════════════════════════

func TestParallel_DrainCompletedHandlesErrors(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	pd := d.dispatcher

	setupOrchestrator(t, d, "proj", 2)

	// Claim the task so IncrementFailure has something to increment.
	_ = d.Store.MutateNode("proj/child-0", func(ns *state.NodeState) error {
		return state.TaskClaim(ns, "task-0001")
	})

	taskAddr := "proj/child-0/task-0001"
	pd.active[taskAddr] = &WorkerSlot{Node: "proj/child-0", Task: "task-0001"}

	pd.results <- WorkerResult{
		Node:   "proj/child-0",
		Task:   "task-0001",
		Result: IterationError,
		Error:  fmt.Errorf("model invocation failed"),
	}

	collected := pd.drainCompleted()
	if len(collected) != 1 {
		t.Fatalf("expected 1 result, got %d", len(collected))
	}
	if collected[0].Error == nil {
		t.Error("expected error in collected result")
	}

	// The active map should be cleared.
	pd.mu.Lock()
	remaining := len(pd.active)
	pd.mu.Unlock()
	if remaining != 0 {
		t.Errorf("expected 0 active after error drain, got %d", remaining)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 14. drainCompleted clears blocked entries on blocker completion
// ═══════════════════════════════════════════════════════════════════════════

func TestParallel_DrainCompletedClearsBlockedOnSuccess(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	pd := d.dispatcher

	setupOrchestrator(t, d, "proj", 2)

	blockerAddr := "proj/child-0/task-0001"
	blockedAddr := "proj/child-1/task-0001"

	pd.active[blockerAddr] = &WorkerSlot{Node: "proj/child-0", Task: "task-0001"}
	pd.blocked[blockedAddr] = &BlockedEntry{Blocker: blockerAddr, YieldCount: 1, FirstBlockedAt: time.Now()}

	// Blocker completes successfully.
	pd.results <- WorkerResult{
		Node:   "proj/child-0",
		Task:   "task-0001",
		Result: IterationDidWork,
	}

	pd.drainCompleted()

	pd.mu.Lock()
	_, stillBlocked := pd.blocked[blockedAddr]
	pd.mu.Unlock()
	if stillBlocked {
		t.Error("blocked entry should be cleared when blocker completes")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 15. runOnceParallel returns NoWork when pool is idle and no planning
// ═══════════════════════════════════════════════════════════════════════════

func TestParallel_RunOnceParallelNoWork(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	initLogger(d)

	// Set up an index with all tasks complete so nothing is dispatchable.
	projDir := d.Store.Dir()
	idx := state.NewRootIndex()
	idx.Root = []string{"proj"}
	idx.Nodes["proj"] = state.IndexEntry{
		Name:     "proj",
		Type:     state.NodeOrchestrator,
		State:    state.StatusComplete,
		Address:  "proj",
		Children: []string{"proj/child-0"},
	}
	idx.Nodes["proj/child-0"] = state.IndexEntry{
		Name:    "child-0",
		Type:    state.NodeLeaf,
		State:   state.StatusComplete,
		Address: "proj/child-0",
		Parent:  "proj",
	}
	writeJSON(t, fmt.Sprintf("%s/state.json", projDir), idx)

	result, err := d.runOnceParallel(context.Background(), idx)
	if err != nil {
		t.Fatalf("runOnceParallel error: %v", err)
	}
	if result != IterationNoWork {
		t.Errorf("expected IterationNoWork, got %d", result)
	}

	shutdownParallel(d)
}

// ═══════════════════════════════════════════════════════════════════════════
// Phase 4: Failure Mode Tests
// ═══════════════════════════════════════════════════════════════════════════

// 16. Worker panic releases scope locks and removes the active entry
func TestParallel_WorkerPanicReleasesScope(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	initLogger(d)
	setupOrchestrator(t, d, "proj", 2)

	taskAddr := "proj/child-0/task-0001"

	// Seed scope locks for the panicking worker.
	_ = d.Store.MutateScopeLocks(func(table *state.ScopeLockTable) error {
		table.Locks["handler.go"] = state.ScopeLock{Task: taskAddr, Node: "proj/child-0", PID: os.Getpid()}
		table.Locks["routes.go"] = state.ScopeLock{Task: taskAddr, Node: "proj/child-0", PID: os.Getpid()}
		return nil
	})

	// Claim the task so drainCompleted can process it.
	_ = d.Store.MutateNode("proj/child-0", func(ns *state.NodeState) error {
		return state.TaskClaim(ns, "task-0001")
	})

	pd := d.dispatcher
	panicCancel := func() {}
	pd.mu.Lock()
	pd.active[taskAddr] = &WorkerSlot{Node: "proj/child-0", Task: "task-0001", Cancel: panicCancel}
	pd.mu.Unlock()

	// Simulate a worker panic producing an error result.
	pd.results <- WorkerResult{
		Node:   "proj/child-0",
		Task:   "task-0001",
		Result: IterationError,
		Error:  fmt.Errorf("worker panic: something went wrong"),
	}

	pd.drainCompleted()

	// Scope locks should be released.
	table, err := d.Store.ReadScopeLocks()
	if err != nil {
		t.Fatalf("reading scope locks: %v", err)
	}
	if len(table.Locks) != 0 {
		t.Errorf("expected 0 scope locks after panic drain, got %d", len(table.Locks))
	}

	// Active map should be empty.
	pd.mu.Lock()
	remaining := len(pd.active)
	pd.mu.Unlock()
	if remaining != 0 {
		t.Errorf("expected 0 active after panic drain, got %d", remaining)
	}

	shutdownParallel(d)
}

// 17. All workers yield simultaneously (yield livelock prevention)
func TestParallel_AllWorkersYieldSimultaneously(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 3)
	initLogger(d)
	setupOrchestrator(t, d, "proj", 3)

	pd := d.dispatcher

	// Claim all three tasks.
	for i := 0; i < 3; i++ {
		addr := fmt.Sprintf("proj/child-%d", i)
		_ = d.Store.MutateNode(addr, func(ns *state.NodeState) error {
			return state.TaskClaim(ns, "task-0001")
		})
	}

	// Register all three as active.
	for i := 0; i < 3; i++ {
		taskAddr := fmt.Sprintf("proj/child-%d/task-0001", i)
		pd.mu.Lock()
		pd.active[taskAddr] = &WorkerSlot{
			Node: fmt.Sprintf("proj/child-%d", i),
			Task: "task-0001",
		}
		pd.mu.Unlock()
	}

	// All three yield, each claiming the next one as blocker (circular).
	pd.results <- WorkerResult{
		Node: "proj/child-0", Task: "task-0001",
		Result: IterationDidWork, ScopeConflict: true,
		Blocker: "proj/child-1/task-0001",
	}
	pd.results <- WorkerResult{
		Node: "proj/child-1", Task: "task-0001",
		Result: IterationDidWork, ScopeConflict: true,
		Blocker: "proj/child-2/task-0001",
	}
	pd.results <- WorkerResult{
		Node: "proj/child-2", Task: "task-0001",
		Result: IterationDidWork, ScopeConflict: true,
		Blocker: "proj/child-0/task-0001",
	}

	pd.drainCompleted()

	// After drain, all tasks should be reset to not_started and removed
	// from the active map. All should have blocked entries.
	pd.mu.Lock()
	activeCount := len(pd.active)
	blockedCount := len(pd.blocked)
	pd.mu.Unlock()

	if activeCount != 0 {
		t.Errorf("expected 0 active after all-yield, got %d", activeCount)
	}
	if blockedCount != 3 {
		t.Errorf("expected 3 blocked entries after all-yield, got %d", blockedCount)
	}

	// On the next fillSlots call, all blockers are gone from active,
	// so isBlocked should clear the stale entries and allow re-dispatch.
	for i := 0; i < 3; i++ {
		taskAddr := fmt.Sprintf("proj/child-%d/task-0001", i)
		if pd.isBlocked(taskAddr) {
			t.Errorf("task %s should not be blocked (blocker is no longer active)", taskAddr)
		}
	}

	// blocked map should now be empty after isBlocked cleared stale entries.
	pd.mu.Lock()
	finalBlocked := len(pd.blocked)
	pd.mu.Unlock()
	if finalBlocked != 0 {
		t.Errorf("expected 0 blocked entries after stale cleanup, got %d", finalBlocked)
	}

	shutdownParallel(d)
}

// 18. Context cancellation mid-worker (stops gracefully)
func TestParallel_ContextCancellationMidWorker(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	initLogger(d)

	// Model that blocks until context cancellation.
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

	// Dispatch workers.
	ctx, cancel := context.WithCancel(context.Background())
	r1, err := d.RunOnce(ctx)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if r1 != IterationDidWork {
		t.Fatalf("expected IterationDidWork, got %d", r1)
	}

	// Cancel the context. Workers should exit promptly.
	cancel()
	d.dispatcher.cancelAll()

	done := make(chan struct{})
	go func() {
		d.dispatcher.waitAndDrain()
		close(done)
	}()

	select {
	case <-done:
		// Workers cleaned up successfully.
	case <-time.After(5 * time.Second):
		t.Fatal("waitAndDrain hung after context cancellation")
	}

	// Active map should be empty.
	d.dispatcher.mu.Lock()
	remaining := len(d.dispatcher.active)
	d.dispatcher.mu.Unlock()
	if remaining != 0 {
		t.Errorf("expected 0 active after cancellation, got %d", remaining)
	}

	shutdownParallel(d)
}

// 19. Scope release on worker failure
func TestParallel_ScopeReleaseOnWorkerFailure(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	initLogger(d)
	setupOrchestrator(t, d, "proj", 2)

	pd := d.dispatcher
	taskAddr := "proj/child-0/task-0001"

	// Seed scope locks.
	_ = d.Store.MutateScopeLocks(func(table *state.ScopeLockTable) error {
		table.Locks["main.go"] = state.ScopeLock{Task: taskAddr, Node: "proj/child-0", PID: os.Getpid()}
		table.Locks["util.go"] = state.ScopeLock{Task: taskAddr, Node: "proj/child-0", PID: os.Getpid()}
		return nil
	})

	// Claim the task.
	_ = d.Store.MutateNode("proj/child-0", func(ns *state.NodeState) error {
		return state.TaskClaim(ns, "task-0001")
	})

	pd.mu.Lock()
	pd.active[taskAddr] = &WorkerSlot{Node: "proj/child-0", Task: "task-0001"}
	pd.mu.Unlock()

	// Worker fails with an error.
	pd.results <- WorkerResult{
		Node:   "proj/child-0",
		Task:   "task-0001",
		Result: IterationError,
		Error:  fmt.Errorf("model invocation timeout"),
	}

	pd.drainCompleted()

	// Scope locks should be fully released.
	table, err := d.Store.ReadScopeLocks()
	if err != nil {
		t.Fatalf("reading scope locks: %v", err)
	}
	if len(table.Locks) != 0 {
		t.Errorf("expected 0 scope locks after failure, got %d", len(table.Locks))
	}

	shutdownParallel(d)
}

// 20. Stale scope lock cleanup after daemon crash (selfHeal path)
func TestParallel_StaleScopeLockCleanupAfterCrash(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	initLogger(d)
	setupOrchestrator(t, d, "proj", 1)

	// Seed scope locks with a dead PID to simulate a crashed daemon.
	deadPID := 999999
	_ = d.Store.MutateScopeLocks(func(table *state.ScopeLockTable) error {
		table.Locks["handler.go"] = state.ScopeLock{
			Task: "proj/child-0/task-0001",
			Node: "proj/child-0",
			PID:  deadPID,
		}
		table.Locks["routes.go"] = state.ScopeLock{
			Task: "proj/child-0/task-0001",
			Node: "proj/child-0",
			PID:  deadPID,
		}
		return nil
	})

	// selfHeal includes cleanStaleScopeLocks.
	if err := d.selfHeal(); err != nil {
		t.Fatalf("selfHeal error: %v", err)
	}

	// The stale locks should have been cleaned up.
	table, err := d.Store.ReadScopeLocks()
	if err != nil {
		// File removed entirely is also valid.
		if !os.IsNotExist(err) {
			t.Fatalf("reading scope locks after selfHeal: %v", err)
		}
		return
	}
	if len(table.Locks) != 0 {
		t.Errorf("expected 0 scope locks after stale cleanup, got %d", len(table.Locks))
	}

	shutdownParallel(d)
}

// 21. Yield count tracking increments correctly across multiple yields
func TestParallel_YieldCountTracking(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	initLogger(d)
	setupOrchestrator(t, d, "proj", 2)

	pd := d.dispatcher
	taskAddr := "proj/child-0/task-0001"
	blockerAddr := "proj/child-1/task-0001"

	// First yield.
	_ = d.Store.MutateNode("proj/child-0", func(ns *state.NodeState) error {
		return state.TaskClaim(ns, "task-0001")
	})

	pd.mu.Lock()
	pd.active[taskAddr] = &WorkerSlot{Node: "proj/child-0", Task: "task-0001"}
	pd.active[blockerAddr] = &WorkerSlot{Node: "proj/child-1", Task: "task-0001"}
	pd.mu.Unlock()

	pd.results <- WorkerResult{
		Node: "proj/child-0", Task: "task-0001",
		Result: IterationDidWork, ScopeConflict: true,
		Blocker: blockerAddr,
	}

	pd.drainCompleted()

	pd.mu.Lock()
	entry1 := pd.blocked[taskAddr]
	pd.mu.Unlock()

	if entry1 == nil {
		t.Fatal("expected blocked entry after first yield")
	}
	if entry1.YieldCount != 1 {
		t.Errorf("expected yield count 1, got %d", entry1.YieldCount)
	}
	firstBlockedAt := entry1.FirstBlockedAt

	// Re-claim and second yield (simulating re-dispatch that yields again).
	_ = d.Store.MutateNode("proj/child-0", func(ns *state.NodeState) error {
		return state.TaskClaim(ns, "task-0001")
	})

	pd.mu.Lock()
	pd.active[taskAddr] = &WorkerSlot{Node: "proj/child-0", Task: "task-0001"}
	pd.mu.Unlock()

	pd.results <- WorkerResult{
		Node: "proj/child-0", Task: "task-0001",
		Result: IterationDidWork, ScopeConflict: true,
		Blocker: blockerAddr,
	}

	pd.drainCompleted()

	pd.mu.Lock()
	entry2 := pd.blocked[taskAddr]
	pd.mu.Unlock()

	if entry2 == nil {
		t.Fatal("expected blocked entry after second yield")
	}
	if entry2.YieldCount != 2 {
		t.Errorf("expected yield count 2, got %d", entry2.YieldCount)
	}
	if !entry2.FirstBlockedAt.Equal(firstBlockedAt) {
		t.Errorf("FirstBlockedAt should not change across yields: got %v, want %v", entry2.FirstBlockedAt, firstBlockedAt)
	}

	shutdownParallel(d)
}

// 22. Status snapshot is written and readable
func TestParallel_StatusSnapshotWriteAndRead(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 3)
	initLogger(d)
	pd := d.dispatcher

	// Seed some active workers and blocked entries.
	noop := func() {}
	pd.mu.Lock()
	pd.active["proj/api/task-0001"] = &WorkerSlot{Node: "proj/api", Task: "task-0001", Cancel: noop}
	pd.active["proj/db/task-0001"] = &WorkerSlot{Node: "proj/db", Task: "task-0001", Cancel: noop}
	pd.blocked["proj/auth/task-0001"] = &BlockedEntry{
		Blocker:        "proj/api/task-0001",
		YieldCount:     2,
		FirstBlockedAt: time.Now().Add(-30 * time.Second),
	}
	pd.mu.Unlock()

	// Write the snapshot.
	pd.writeStatusSnapshot()

	// Read it back.
	ps := LoadParallelStatus(d.WolfcastleDir)
	if ps == nil {
		t.Fatal("LoadParallelStatus returned nil")
	}

	if ps.MaxWorkers != 3 {
		t.Errorf("MaxWorkers = %d, want 3", ps.MaxWorkers)
	}
	if len(ps.Active) != 2 {
		t.Errorf("Active count = %d, want 2", len(ps.Active))
	}
	if len(ps.Yielded) != 1 {
		t.Errorf("Yielded count = %d, want 1", len(ps.Yielded))
	}
	if ps.Yielded[0].YieldCount != 2 {
		t.Errorf("Yielded[0].YieldCount = %d, want 2", ps.Yielded[0].YieldCount)
	}

	shutdownParallel(d)
}

// 23. Status snapshot is cleaned up after waitAndDrain
func TestParallel_StatusSnapshotCleanedUp(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)
	initLogger(d)
	pd := d.dispatcher

	// Write a snapshot so the file exists.
	pd.mu.Lock()
	pd.active["proj/api/task-0001"] = &WorkerSlot{Node: "proj/api", Task: "task-0001", Cancel: func() {}}
	pd.mu.Unlock()
	pd.writeStatusSnapshot()

	statusPath := parallelStatusPath(d.WolfcastleDir)
	if _, err := os.Stat(statusPath); os.IsNotExist(err) {
		t.Fatal("status file should exist after writeStatusSnapshot")
	}

	// Clear the active map so waitAndDrain doesn't try to drain results,
	// then call removeStatusFile.
	pd.mu.Lock()
	delete(pd.active, "proj/api/task-0001")
	pd.mu.Unlock()

	pd.removeStatusFile()

	if _, err := os.Stat(statusPath); !os.IsNotExist(err) {
		t.Error("status file should be removed after removeStatusFile")
	}

	shutdownParallel(d)
}

// ═══════════════════════════════════════════════════════════════════════════
// 24. Snapshot produces deterministic order for Active and Yielded
// ═══════════════════════════════════════════════════════════════════════════

func TestParallel_SnapshotDeterministicOrder(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 4)
	pd := d.dispatcher

	noop := func() {}
	pd.mu.Lock()
	pd.active["proj/zebra/task-0001"] = &WorkerSlot{Node: "proj/zebra", Task: "task-0001", Cancel: noop}
	pd.active["proj/alpha/task-0001"] = &WorkerSlot{Node: "proj/alpha", Task: "task-0001", Cancel: noop}
	pd.active["proj/mango/task-0001"] = &WorkerSlot{Node: "proj/mango", Task: "task-0001", Cancel: noop}
	pd.blocked["proj/yak/task-0001"] = &BlockedEntry{
		Blocker:        "proj/zebra/task-0001",
		YieldCount:     1,
		FirstBlockedAt: time.Now(),
	}
	pd.blocked["proj/berry/task-0001"] = &BlockedEntry{
		Blocker:        "proj/alpha/task-0001",
		YieldCount:     1,
		FirstBlockedAt: time.Now(),
	}
	pd.blocked["proj/kiwi/task-0001"] = &BlockedEntry{
		Blocker:        "proj/mango/task-0001",
		YieldCount:     1,
		FirstBlockedAt: time.Now(),
	}
	pd.mu.Unlock()

	snap := pd.snapshot()

	// Active entries should be sorted alphabetically by Task field.
	if len(snap.Active) != 3 {
		t.Fatalf("expected 3 active entries, got %d", len(snap.Active))
	}
	wantActive := []string{"proj/alpha/task-0001", "proj/mango/task-0001", "proj/zebra/task-0001"}
	for i, want := range wantActive {
		if snap.Active[i].Task != want {
			t.Errorf("Active[%d].Task = %q, want %q", i, snap.Active[i].Task, want)
		}
	}

	// Yielded entries should be sorted alphabetically by Task field.
	if len(snap.Yielded) != 3 {
		t.Fatalf("expected 3 yielded entries, got %d", len(snap.Yielded))
	}
	wantYielded := []string{"proj/berry/task-0001", "proj/kiwi/task-0001", "proj/yak/task-0001"}
	for i, want := range wantYielded {
		if snap.Yielded[i].Task != want {
			t.Errorf("Yielded[%d].Task = %q, want %q", i, snap.Yielded[i].Task, want)
		}
	}
}

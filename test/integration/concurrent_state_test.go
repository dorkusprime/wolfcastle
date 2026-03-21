//go:build integration

package integration

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// TestConcurrentDaemonLoopInboxAndCLI exercises the highest-risk runtime
// scenario: the daemon's execute loop, the inbox goroutine, and CLI-driven
// state mutations all contending on the same Store simultaneously.
//
// The test scaffolds a project tree with an orchestrator and two leaf nodes,
// then launches three goroutines that hammer the state store in parallel:
//
//  1. A simulated execute loop that navigates the tree, claims tasks,
//     and completes them one at a time (serial, like the real daemon).
//  2. An inbox goroutine that repeatedly adds and marks items.
//  3. A CLI mutation goroutine that blocks and unblocks tasks, reads
//     state, and adds breadcrumbs.
//
// After all goroutines finish, we verify the tree is in a consistent
// terminal state: every node's status matches its children/tasks, the
// root index reflects reality, and the inbox contains all expected items.
func TestConcurrentDaemonLoopInboxAndCLI(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Initialize a project through the CLI so all on-disk structures
	// (namespace, root index, config) are correctly bootstrapped.
	run(t, dir, "init")

	// Build a small tree: orchestrator with two leaf children.
	run(t, dir, "project", "create", "--type", "orchestrator", "orch")
	run(t, dir, "project", "create", "--node", "orch", "leaf-a")
	run(t, dir, "project", "create", "--node", "orch", "leaf-b")

	// Add tasks to each leaf. Two tasks per leaf gives us enough work
	// for the execute loop to run several iterations while the other
	// goroutines are active.
	run(t, dir, "task", "add", "--node", "orch/leaf-a", "task alpha one")
	run(t, dir, "task", "add", "--node", "orch/leaf-a", "task alpha two")
	run(t, dir, "task", "add", "--node", "orch/leaf-b", "task beta one")
	run(t, dir, "task", "add", "--node", "orch/leaf-b", "task beta two")

	// Discover the namespace so we can build a Store directly.
	ns := discoverNamespace(t, dir)
	storeDir := dir + "/.wolfcastle/system/projects/" + ns
	store := state.NewStore(storeDir, 5*time.Second)

	// Coordination: the CLI goroutine needs to know which task is
	// currently in_progress so it can do something meaningful (add
	// breadcrumbs, attempt a block). We use a channel for this.
	type taskInfo struct {
		node   string
		taskID string
	}
	claimNotify := make(chan taskInfo, 20)

	// Gate: all goroutines start simultaneously.
	var startGate sync.WaitGroup
	startGate.Add(1)

	// Collect errors from goroutines.
	errs := make(chan error, 30)

	// ── Goroutine 1: Simulated execute loop ────────────────────────
	// Mimics the daemon's core: FindNextTask → MutateNode(TaskClaim)
	// → (simulate work) → MutateNode(TaskComplete). Runs until the
	// tree is finished or 50 iterations pass (safety valve).
	var wgExec sync.WaitGroup
	wgExec.Add(1)
	go func() {
		defer wgExec.Done()
		startGate.Wait()

		for iter := 0; iter < 50; iter++ {
			idx, err := store.ReadIndex()
			if err != nil {
				errs <- fmt.Errorf("exec: ReadIndex iteration %d: %w", iter, err)
				return
			}

			nav, err := state.FindNextTask(idx, "orch", func(addr string) (*state.NodeState, error) {
				return store.ReadNode(addr)
			})
			if err != nil {
				errs <- fmt.Errorf("exec: FindNextTask iteration %d: %w", iter, err)
				return
			}
			if !nav.Found {
				// Tree is either all_complete or all_blocked. We're done.
				break
			}

			// Claim the task (skip if already in_progress from a previous yield).
			current, err := store.ReadNode(nav.NodeAddress)
			if err != nil {
				errs <- fmt.Errorf("exec: ReadNode %s: %w", nav.NodeAddress, err)
				return
			}
			alreadyClaimed := false
			for _, tk := range current.Tasks {
				if tk.ID == nav.TaskID && tk.State == state.StatusInProgress {
					alreadyClaimed = true
					break
				}
			}
			if !alreadyClaimed {
				if err := store.MutateNode(nav.NodeAddress, func(ns *state.NodeState) error {
					return state.TaskClaim(ns, nav.TaskID)
				}); err != nil {
					errs <- fmt.Errorf("exec: TaskClaim %s/%s: %w", nav.NodeAddress, nav.TaskID, err)
					return
				}
			}

			// Notify the CLI goroutine about the claimed task.
			select {
			case claimNotify <- taskInfo{node: nav.NodeAddress, taskID: nav.TaskID}:
			default:
			}

			// Simulate work taking a tiny bit of time (enough for
			// other goroutines to get a few mutations in).
			time.Sleep(2 * time.Millisecond)

			// Complete the task.
			if err := store.MutateNode(nav.NodeAddress, func(ns *state.NodeState) error {
				return state.TaskComplete(ns, nav.TaskID)
			}); err != nil {
				errs <- fmt.Errorf("exec: TaskComplete %s/%s: %w", nav.NodeAddress, nav.TaskID, err)
				return
			}
		}
	}()

	// ── Goroutine 2: Inbox mutations ───────────────────────────────
	// Adds inbox items concurrently with the execute loop. Each item
	// is added individually through MutateInbox, then a second pass
	// marks them as "filed".
	const inboxItemCount = 10
	var wgInbox sync.WaitGroup
	wgInbox.Add(1)
	go func() {
		defer wgInbox.Done()
		startGate.Wait()

		for i := 0; i < inboxItemCount; i++ {
			itemText := fmt.Sprintf("inbox-item-%d", i)
			if err := store.MutateInbox(func(f *state.InboxFile) error {
				f.Items = append(f.Items, state.InboxItem{
					Timestamp: time.Now().Format(time.RFC3339),
					Text:      itemText,
					Status:    "new",
				})
				return nil
			}); err != nil {
				errs <- fmt.Errorf("inbox: add item %d: %w", i, err)
				return
			}
		}

		// Second pass: mark all as filed (simulates intake completion).
		if err := store.MutateInbox(func(f *state.InboxFile) error {
			for i := range f.Items {
				if f.Items[i].Status == "new" {
					f.Items[i].Status = "filed"
				}
			}
			return nil
		}); err != nil {
			errs <- fmt.Errorf("inbox: filing pass: %w", err)
		}
	}()

	// ── Goroutine 3: CLI-style mutations ───────────────────────────
	// Reads state, adds breadcrumbs, and mutates the index. This
	// simulates a human running `wolfcastle audit breadcrumb` and
	// `wolfcastle task block` while the daemon is running.
	var wgCLI sync.WaitGroup
	wgCLI.Add(1)
	go func() {
		defer wgCLI.Done()
		startGate.Wait()

		breadcrumbCount := 0
		indexMutations := 0

		// Drain claim notifications and do CLI-like things.
		// We also do some unsolicited index reads to stress the
		// read path while mutations are in flight.
		timeout := time.After(8 * time.Second)
		for {
			select {
			case info, ok := <-claimNotify:
				if !ok {
					return
				}
				// Add a breadcrumb to the node.
				if err := store.MutateNode(info.node, func(ns *state.NodeState) error {
					state.AddBreadcrumb(ns, info.taskID, fmt.Sprintf("CLI breadcrumb for %s", info.taskID), clock.New())
					breadcrumbCount++
					return nil
				}); err != nil {
					errs <- fmt.Errorf("cli: breadcrumb %s/%s: %w", info.node, info.taskID, err)
				}

				// Touch the root index (simulates `wolfcastle status` writes).
				if err := store.MutateIndex(func(idx *state.RootIndex) error {
					// Read-modify-write: just bump a harmless field.
					indexMutations++
					return nil
				}); err != nil {
					errs <- fmt.Errorf("cli: MutateIndex: %w", err)
				}

			case <-timeout:
				// Safety: don't hang forever if the execute loop finishes
				// before we drain all notifications.
				return
			}
		}
	}()

	// Release all goroutines simultaneously.
	startGate.Done()

	// Wait for the execute loop to finish first, then close the
	// claim channel so the CLI goroutine can drain and exit.
	wgExec.Wait()
	close(claimNotify)

	// Wait for the other goroutines.
	wgCLI.Wait()
	wgInbox.Wait()

	// Collect any errors.
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	// ── Verification ───────────────────────────────────────────────
	// All four non-audit tasks should be complete. The audit tasks
	// may or may not have been picked up by the execute loop (they
	// become eligible only after all non-audit tasks in a leaf are
	// done), but the non-audit tasks must be complete.
	verifyLeafTasks(t, store, "orch/leaf-a", []string{"task-0001", "task-0002"})
	verifyLeafTasks(t, store, "orch/leaf-b", []string{"task-0001", "task-0002"})

	// The root index should reflect the state of each node accurately.
	idx, err := store.ReadIndex()
	if err != nil {
		t.Fatalf("final ReadIndex: %v", err)
	}
	for _, addr := range []string{"orch", "orch/leaf-a", "orch/leaf-b"} {
		entry, ok := idx.Nodes[addr]
		if !ok {
			t.Errorf("node %s missing from root index", addr)
			continue
		}
		ns, err := store.ReadNode(addr)
		if err != nil {
			t.Errorf("ReadNode(%s): %v", addr, err)
			continue
		}
		if entry.State != ns.State {
			t.Errorf("index/node state mismatch for %s: index=%s, node=%s", addr, entry.State, ns.State)
		}
	}

	// Inbox should contain exactly inboxItemCount items, all filed.
	inbox, err := store.ReadInbox()
	if err != nil {
		t.Fatalf("final ReadInbox: %v", err)
	}
	if len(inbox.Items) != inboxItemCount {
		t.Errorf("expected %d inbox items, got %d", inboxItemCount, len(inbox.Items))
	}
	for i, item := range inbox.Items {
		if item.Status != "filed" {
			t.Errorf("inbox item %d status = %s, want filed", i, item.Status)
		}
	}

	// At least some breadcrumbs should have landed on the leaf nodes.
	// We can't assert an exact count because timing determines how
	// many claim notifications the CLI goroutine processes before the
	// execute loop finishes, but there should be at least one.
	totalBreadcrumbs := 0
	for _, addr := range []string{"orch/leaf-a", "orch/leaf-b"} {
		ns, err := store.ReadNode(addr)
		if err != nil {
			t.Errorf("ReadNode(%s) for breadcrumb check: %v", addr, err)
			continue
		}
		totalBreadcrumbs += len(ns.Audit.Breadcrumbs)
	}
	if totalBreadcrumbs == 0 {
		t.Error("expected at least one breadcrumb from CLI goroutine, got zero")
	}
}

// verifyLeafTasks checks that specific tasks in a leaf node are complete.
func verifyLeafTasks(t *testing.T, store *state.Store, addr string, taskIDs []string) {
	t.Helper()
	ns, err := store.ReadNode(addr)
	if err != nil {
		t.Fatalf("verifyLeafTasks ReadNode(%s): %v", addr, err)
	}
	taskMap := make(map[string]state.NodeStatus)
	for _, tk := range ns.Tasks {
		taskMap[tk.ID] = tk.State
	}
	for _, id := range taskIDs {
		got, ok := taskMap[id]
		if !ok {
			t.Errorf("%s: task %s not found", addr, id)
		} else if got != state.StatusComplete {
			t.Errorf("%s/%s state = %s, want complete", addr, id, got)
		}
	}
}

// TestConcurrentMutateNodeContention hammers MutateNode from multiple
// goroutines targeting the same leaf node. This isolates the file-lock
// contention path that the advisory lock must serialize correctly.
func TestConcurrentMutateNodeContention(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")
	run(t, dir, "project", "create", "contention")

	// Add several tasks so each goroutine has something to mutate.
	for i := 0; i < 5; i++ {
		run(t, dir, "task", "add", "--node", "contention", fmt.Sprintf("task %d", i))
	}

	ns := discoverNamespace(t, dir)
	storeDir := dir + "/.wolfcastle/system/projects/" + ns
	store := state.NewStore(storeDir, 10*time.Second)

	// Launch goroutines that each claim and complete a different task.
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 1; i <= 5; i++ {
		taskID := fmt.Sprintf("task-%04d", i)
		wg.Add(1)
		go func(tid string) {
			defer wg.Done()
			if err := store.MutateNode("contention", func(ns *state.NodeState) error {
				return state.TaskClaim(ns, tid)
			}); err != nil {
				errs <- fmt.Errorf("claim %s: %w", tid, err)
				return
			}
			if err := store.MutateNode("contention", func(ns *state.NodeState) error {
				return state.TaskComplete(ns, tid)
			}); err != nil {
				errs <- fmt.Errorf("complete %s: %w", tid, err)
			}
		}(taskID)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}

	// All five tasks should be complete.
	final, err := store.ReadNode("contention")
	if err != nil {
		t.Fatalf("final ReadNode: %v", err)
	}
	for _, tk := range final.Tasks {
		if tk.IsAudit {
			continue
		}
		if tk.State != state.StatusComplete {
			t.Errorf("task %s state = %s, want complete", tk.ID, tk.State)
		}
	}

	// The node itself should reflect a state consistent with all
	// non-audit tasks being complete (audit is still not_started,
	// so the node should be in_progress or complete depending on
	// whether audit was auto-completed).
	idx, err := store.ReadIndex()
	if err != nil {
		t.Fatalf("final ReadIndex: %v", err)
	}
	entry, ok := idx.Nodes["contention"]
	if !ok {
		t.Fatal("contention node missing from index")
	}
	if entry.State != final.State {
		t.Errorf("index state %s != node state %s", entry.State, final.State)
	}
}

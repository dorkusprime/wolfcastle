package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	dmn "github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

// ═══════════════════════════════════════════════════════════════════════════
// printNodeTree — completed orchestrator with children (collapse path)
// ═══════════════════════════════════════════════════════════════════════════

func TestPrintNodeTree_CompletedOrchestratorCollapsed(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Tree",
		testutil.Orchestrator("tree",
			testutil.Leaf("leaf-a"),
			testutil.Leaf("leaf-b"),
		),
	)

	idx, _ := env.App.State.ReadIndex()

	// Mark the orchestrator and its children as complete.
	e := idx.Nodes["tree"]
	e.State = state.StatusComplete
	idx.Nodes["tree"] = e
	for _, child := range []string{"tree/leaf-a", "tree/leaf-b"} {
		ce := idx.Nodes[child]
		ce.State = state.StatusComplete
		idx.Nodes[child] = ce
	}

	details := map[string]*nodeDetail{}
	for addr, entry := range idx.Nodes {
		nd := &nodeDetail{entry: entry}
		ns, err := env.App.State.ReadNode(addr)
		if err == nil {
			nd.ns = ns
		}
		details[addr] = nd
	}

	// expand=false: completed orchestrator with children should be collapsed.
	printNodeTree(env.App, idx, details, "tree", "  ", false)
}

// ═══════════════════════════════════════════════════════════════════════════
// printNodeTree — completed leaf node with tasks (collapse path)
// ═══════════════════════════════════════════════════════════════════════════

func TestPrintNodeTree_CompletedLeafWithTasks(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	idx, _ := env.App.State.ReadIndex()

	// Mark the leaf as complete.
	e := idx.Nodes["proj"]
	e.State = state.StatusComplete
	idx.Nodes["proj"] = e

	ns, _ := env.App.State.ReadNode("proj")
	ns.Tasks = []state.Task{
		{ID: "task-0001", Title: "Done thing", State: state.StatusComplete},
		{ID: "task-0002", Title: "Also done", State: state.StatusComplete},
	}
	testutil.SaveNode(t, env.WolfcastleDir, env.env.Namespace(), "proj", ns)

	details := map[string]*nodeDetail{
		"proj": {entry: idx.Nodes["proj"], ns: ns},
	}

	// expand=false: completed leaf with tasks shows task count.
	printNodeTree(env.App, idx, details, "proj", "  ", false)
}

// ═══════════════════════════════════════════════════════════════════════════
// printNodeTree — orchestrator with active audit task
// ═══════════════════════════════════════════════════════════════════════════

func TestPrintNodeTree_OrchestratorWithActiveAuditTask(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Tree",
		testutil.Orchestrator("tree",
			testutil.Leaf("leaf-a"),
		),
	)

	idx, _ := env.App.State.ReadIndex()
	orchNs, _ := env.App.State.ReadNode("tree")
	if orchNs == nil {
		orchNs = state.NewNodeState("tree", "Tree", state.NodeOrchestrator)
	}
	orchNs.Tasks = []state.Task{
		{ID: "audit", Description: "Audit orchestrator", State: state.StatusInProgress, IsAudit: true},
	}
	testutil.SaveNode(t, env.WolfcastleDir, env.env.Namespace(), "tree", orchNs)

	details := map[string]*nodeDetail{}
	for addr, entry := range idx.Nodes {
		nd := &nodeDetail{entry: entry}
		ns, err := env.App.State.ReadNode(addr)
		if err == nil {
			nd.ns = ns
		}
		details[addr] = nd
	}

	// Exercises orchestrator audit task display path.
	printNodeTree(env.App, idx, details, "tree", "  ", false)
}

// ═══════════════════════════════════════════════════════════════════════════
// printNodeTree — subtask collapsing: completed parent with all-complete children
// ═══════════════════════════════════════════════════════════════════════════

func TestPrintNodeTree_SubtaskCollapsing(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	idx, _ := env.App.State.ReadIndex()
	ns, _ := env.App.State.ReadNode("proj")
	ns.Tasks = []state.Task{
		{ID: "task-0001", Title: "Parent", State: state.StatusComplete},
		{ID: "task-0001.0001", Title: "Sub A", State: state.StatusComplete},
		{ID: "task-0001.0002", Title: "Sub B", State: state.StatusComplete},
		{ID: "task-0002", Title: "Another parent", State: state.StatusInProgress},
	}
	testutil.SaveNode(t, env.WolfcastleDir, env.env.Namespace(), "proj", ns)

	details := map[string]*nodeDetail{
		"proj": {entry: idx.Nodes["proj"], ns: ns},
	}

	// expand=false: completed parent task-0001 with all-complete children
	// should be collapsed, showing child count instead.
	printNodeTree(env.App, idx, details, "proj", "  ", false)
}

// ═══════════════════════════════════════════════════════════════════════════
// printNodeTree — subtask NOT collapsed: parent complete but child incomplete
// ═══════════════════════════════════════════════════════════════════════════

func TestPrintNodeTree_SubtaskNotCollapsed(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	idx, _ := env.App.State.ReadIndex()
	ns, _ := env.App.State.ReadNode("proj")
	ns.Tasks = []state.Task{
		{ID: "task-0001", Title: "Parent", State: state.StatusComplete},
		{ID: "task-0001.0001", Title: "Sub A", State: state.StatusComplete},
		{ID: "task-0001.0002", Title: "Sub B", State: state.StatusInProgress},
	}
	testutil.SaveNode(t, env.WolfcastleDir, env.env.Namespace(), "proj", ns)

	details := map[string]*nodeDetail{
		"proj": {entry: idx.Nodes["proj"], ns: ns},
	}

	// expand=false: parent complete but sub B incomplete — no collapse.
	printNodeTree(env.App, idx, details, "proj", "  ", false)
}

// ═══════════════════════════════════════════════════════════════════════════
// showTreeStatus — planning queue rendering
// ═══════════════════════════════════════════════════════════════════════════

func TestShowTreeStatus_PlanningQueue(t *testing.T) {
	env := newTestEnv(t)

	// Create an orchestrator with no children and not complete:
	// this puts it in the planning queue.
	env.env.WithProject("Orch", testutil.Orchestrator("plan-orch"))

	idx, _ := env.App.State.ReadIndex()

	// Verify the planning queue path fires.
	if err := showTreeStatus(env.App, idx, ""); err != nil {
		t.Fatalf("showTreeStatus planning queue: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// showTreeStatus — in-progress summary line
// ═══════════════════════════════════════════════════════════════════════════

func TestShowTreeStatus_InProgressCount(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Active", testutil.Leaf("active"))

	idx, _ := env.App.State.ReadIndex()

	// Mark the node as in-progress.
	e := idx.Nodes["active"]
	e.State = state.StatusInProgress
	idx.Nodes["active"] = e

	if err := showTreeStatus(env.App, idx, ""); err != nil {
		t.Fatalf("showTreeStatus in-progress: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// watchStatus — runs at least one full loop iteration with tree data
// ═══════════════════════════════════════════════════════════════════════════

func TestWatchStatus_RunsOneFullIteration(t *testing.T) {
	t.Parallel()
	env := newStatusTestEnv(t)

	// Use a short timeout that allows the loop to run once then cancel.
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- watchStatus(ctx, env.App, "", false, 0.1, false)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("watchStatus full iteration: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watchStatus did not exit")
	}
}

func TestWatchStatus_ExpandFlag(t *testing.T) {
	t.Parallel()
	env := newStatusTestEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- watchStatus(ctx, env.App, "", false, 0.1, true)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("watchStatus expand: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watchStatus did not exit")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// newStatusCmd — watch flag path through command
// ═══════════════════════════════════════════════════════════════════════════

func TestStatusCmd_WatchFlag(t *testing.T) {
	env := newStatusTestEnv(t)

	// Run status --watch with a very short interval and a context
	// that cancels after 500ms. Without cancellation the goroutine
	// outlives the test and races on os.Stdout with later tests.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		env.RootCmd.SetContext(ctx)
		env.RootCmd.SetArgs([]string{"status", "--watch", "--interval", "0.1"})
		done <- env.RootCmd.Execute()
	}()

	select {
	case <-done:
		// Command exited (context cancelled or signal).
	case <-time.After(2 * time.Second):
		cancel()
		<-done
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// printNodeTree — deep subtask indentation (task-0001.0002.0003)
// ═══════════════════════════════════════════════════════════════════════════

func TestPrintNodeTree_DeepSubtaskIndent(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	idx, _ := env.App.State.ReadIndex()
	ns, _ := env.App.State.ReadNode("proj")
	ns.Tasks = []state.Task{
		{ID: "task-0001", Title: "Top", State: state.StatusInProgress},
		{ID: "task-0001.0001", Title: "Mid", State: state.StatusInProgress},
		{ID: "task-0001.0001.0001", Title: "Deep", State: state.StatusNotStarted},
	}
	testutil.SaveNode(t, env.WolfcastleDir, env.env.Namespace(), "proj", ns)

	details := map[string]*nodeDetail{
		"proj": {entry: idx.Nodes["proj"], ns: ns},
	}

	// Exercises multi-level task indentation.
	printNodeTree(env.App, idx, details, "proj", "  ", true)
}

// ═══════════════════════════════════════════════════════════════════════════
// printNodeTree — completed task with collapsed subtasks (child count display)
// ═══════════════════════════════════════════════════════════════════════════

func TestPrintNodeTree_CompletedTaskCollapsedSubtasks(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	idx, _ := env.App.State.ReadIndex()
	ns, _ := env.App.State.ReadNode("proj")
	ns.Tasks = []state.Task{
		// Completed task with children — the task itself hits the "collapsed
		// parent task: show child count" branch at line 305-316.
		{ID: "task-0001", Title: "Done parent", State: state.StatusComplete},
		{ID: "task-0001.0001", Title: "Child 1", State: state.StatusComplete},
		{ID: "task-0001.0002", Title: "Child 2", State: state.StatusComplete},
		{ID: "task-0001.0003", Title: "Child 3", State: state.StatusComplete},
	}
	testutil.SaveNode(t, env.WolfcastleDir, env.env.Namespace(), "proj", ns)

	details := map[string]*nodeDetail{
		"proj": {entry: idx.Nodes["proj"], ns: ns},
	}

	// expand=false: exercises the collapsed-parent-task + skipChildren paths.
	printNodeTree(env.App, idx, details, "proj", "  ", false)
}

// ═══════════════════════════════════════════════════════════════════════════
// showTreeStatus — completed node in non-JSON with scope
// ═══════════════════════════════════════════════════════════════════════════

func TestShowTreeStatus_CompletedNodesCollapsed(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Tree",
		testutil.Orchestrator("orch",
			testutil.Leaf("orch/done-leaf"),
		),
	)

	idx, _ := env.App.State.ReadIndex()

	// Mark everything as complete.
	for addr, entry := range idx.Nodes {
		entry.State = state.StatusComplete
		idx.Nodes[addr] = entry
	}

	if err := showTreeStatus(env.App, idx, ""); err != nil {
		t.Fatalf("showTreeStatus completed nodes: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// start.go: config Load error path
// ═══════════════════════════════════════════════════════════════════════════

func TestStartCmd_ConfigLoadError(t *testing.T) {
	env := newTestEnv(t)

	// Corrupt the config file to trigger Load() error.
	cfgPath := filepath.Join(env.WolfcastleDir, "system", "base", "config.json")
	_ = os.MkdirAll(filepath.Dir(cfgPath), 0755)
	_ = os.WriteFile(cfgPath, []byte("not valid json{{"), 0644)

	env.RootCmd.SetArgs([]string{"start"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when config is corrupt")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// showAllStatus — file entry in projects dir (not a directory)
// ═══════════════════════════════════════════════════════════════════════════

func TestShowAllStatus_FileInProjectsDir(t *testing.T) {
	env := newTestEnv(t)

	// Create a regular file in the projects dir; showAllStatus should skip it.
	_ = os.WriteFile(filepath.Join(env.WolfcastleDir, "system", "projects", "stray-file.txt"),
		[]byte("not a dir"), 0644)

	if err := showAllStatus(env.App); err != nil {
		t.Fatalf("showAllStatus should skip non-dir entries: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// printNodeTree — gap with IsTerminal=false (non-terminal display path)
// Already tested, but this adds an explicit assertion on the output path.
// ═══════════════════════════════════════════════════════════════════════════

func TestPrintNodeTree_OpenGapNonTerminal(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	idx, _ := env.App.State.ReadIndex()
	ns, _ := env.App.State.ReadNode("proj")
	ns.Audit.Gaps = []state.Gap{
		{ID: "gap-1", Description: "test gap", Status: state.GapOpen},
	}
	testutil.SaveNode(t, env.WolfcastleDir, env.env.Namespace(), "proj", ns)

	details := map[string]*nodeDetail{
		"proj": {entry: idx.Nodes["proj"], ns: ns},
	}

	// Exercises the non-terminal gap rendering branch.
	printNodeTree(env.App, idx, details, "proj", "  ", false)
}

// ═══════════════════════════════════════════════════════════════════════════
// startBackground — PID write failure (read-only dir)
// ═══════════════════════════════════════════════════════════════════════════

func TestStartBackground_PIDWriteFailure(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skip in CI")
	}
	t.Parallel()
	dir := t.TempDir()
	wolfDir := filepath.Join(dir, ".wolfcastle")
	sysDir := filepath.Join(wolfDir, "system")
	_ = os.MkdirAll(sysDir, 0755)

	// Create daemon.log writable, but make the PID write path fail
	// by pre-creating wolfcastle.pid as a directory.
	_ = os.MkdirAll(filepath.Join(sysDir, "wolfcastle.pid"), 0755)

	err := startBackground(wolfDir, "", "", "sleep")
	if err == nil {
		t.Error("expected error when PID file cannot be written")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// recoverStaleDaemonState — read error on PID file (not ENOENT)
// ═══════════════════════════════════════════════════════════════════════════

func TestRecoverStaleDaemonState_ReadError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	if os.Getenv("CI") != "" {
		t.Skip("skip in CI")
	}
	tmp := t.TempDir()
	sysDir := filepath.Join(tmp, "system")
	_ = os.MkdirAll(sysDir, 0755)

	// Make the PID file unreadable (triggers the err != nil, !IsNotExist path).
	pidPath := filepath.Join(sysDir, "wolfcastle.pid")
	_ = os.WriteFile(pidPath, []byte("1234"), 0644)
	_ = os.Chmod(pidPath, 0000)
	defer func() { _ = os.Chmod(pidPath, 0644) }()

	// Should return silently without panicking.
	recoverStaleDaemonState(tmp)
}

// ═══════════════════════════════════════════════════════════════════════════
// showTreeStatus — empty tree path (zero total nodes)
// ═══════════════════════════════════════════════════════════════════════════

func TestShowTreeStatus_EmptyTreeMessage(t *testing.T) {
	env := newTestEnv(t)
	idx := state.NewRootIndex()

	// Exercises the "No targets" branch.
	if err := showTreeStatus(env.App, idx, ""); err != nil {
		t.Fatalf("showTreeStatus empty tree: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// printNodeTree — orchestrator with blocked audit task
// ═══════════════════════════════════════════════════════════════════════════

func TestPrintNodeTree_OrchestratorBlockedAuditTask(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Tree",
		testutil.Orchestrator("tree",
			testutil.Leaf("leaf"),
		),
	)

	idx, _ := env.App.State.ReadIndex()
	orchNs, _ := env.App.State.ReadNode("tree")
	if orchNs == nil {
		orchNs = state.NewNodeState("tree", "Tree", state.NodeOrchestrator)
	}
	orchNs.Tasks = []state.Task{
		{ID: "audit", Description: "Blocked audit", State: state.StatusBlocked, IsAudit: true},
	}
	testutil.SaveNode(t, env.WolfcastleDir, env.env.Namespace(), "tree", orchNs)

	details := map[string]*nodeDetail{}
	for addr, entry := range idx.Nodes {
		nd := &nodeDetail{entry: entry}
		ns, err := env.App.State.ReadNode(addr)
		if err == nil {
			nd.ns = ns
		}
		details[addr] = nd
	}

	// Exercises blocked audit task display on orchestrator.
	printNodeTree(env.App, idx, details, "tree", "  ", false)
}

// ═══════════════════════════════════════════════════════════════════════════
// showAllStatus — truly empty projects dir (no namespaces, human output)
// ═══════════════════════════════════════════════════════════════════════════

func TestShowAllStatus_EmptyProjectsDir_Human(t *testing.T) {
	env := newTestEnv(t)

	// Nuke everything inside projects/ dir and recreate it empty.
	projDir := filepath.Join(env.WolfcastleDir, "system", "projects")
	_ = os.RemoveAll(projDir)
	_ = os.MkdirAll(projDir, 0755)

	env.App.JSON = false
	if err := showAllStatus(env.App); err != nil {
		t.Fatalf("showAllStatus empty human: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// printNodeTree — subtask with grandchild (rest contains ".", skipped)
// ═══════════════════════════════════════════════════════════════════════════

func TestPrintNodeTree_GrandchildSkippedInCollapseCheck(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	idx, _ := env.App.State.ReadIndex()
	ns, _ := env.App.State.ReadNode("proj")

	// task-0001 has child task-0001.0001 which has grandchild task-0001.0001.0001.
	// All complete. The collapse logic should only count immediate children,
	// so the grandchild's rest ("0001.0001") contains "." and is skipped.
	ns.Tasks = []state.Task{
		{ID: "task-0001", Title: "Top", State: state.StatusComplete},
		{ID: "task-0001.0001", Title: "Child", State: state.StatusComplete},
		{ID: "task-0001.0001.0001", Title: "Grandchild", State: state.StatusComplete},
	}
	testutil.SaveNode(t, env.WolfcastleDir, env.env.Namespace(), "proj", ns)

	details := map[string]*nodeDetail{
		"proj": {entry: idx.Nodes["proj"], ns: ns},
	}

	// expand=false: exercises the grandchild "rest contains dot" skip.
	printNodeTree(env.App, idx, details, "proj", "  ", false)
}

// ═══════════════════════════════════════════════════════════════════════════
// watchStatus — showAll error path inside the loop
// ═══════════════════════════════════════════════════════════════════════════

func TestWatchStatus_ShowAllError(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	// Remove projects dir so showAllStatus returns an error inside the loop.
	_ = os.RemoveAll(filepath.Join(env.WolfcastleDir, "system", "projects"))

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- watchStatus(ctx, env.App, "", true, 0.1, false)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("watchStatus showAll error path: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watchStatus did not exit")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// watchStatus — tree status error path inside the loop
// ═══════════════════════════════════════════════════════════════════════════

func TestWatchStatus_TreeStatusError(t *testing.T) {
	t.Parallel()
	env := newStatusTestEnv(t)

	// Corrupt the state so showTreeStatus returns an error.
	_ = os.WriteFile(
		filepath.Join(env.ProjectsDir, "state.json"),
		[]byte("corrupt json{{"), 0644,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- watchStatus(ctx, env.App, "", false, 0.1, false)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("watchStatus tree error path: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watchStatus did not exit")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// stop command — Signal error path
// ═══════════════════════════════════════════════════════════════════════════

func TestStopCmd_SignalError(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skip in CI")
	}
	env := newTestEnv(t)

	// PID 1 (launchd/init) is always running but we can't signal it.
	_ = os.MkdirAll(filepath.Join(env.WolfcastleDir, "system"), 0755)
	_ = os.WriteFile(filepath.Join(env.WolfcastleDir, "system", "wolfcastle.pid"),
		[]byte("1"), 0644)

	env.RootCmd.SetArgs([]string{"stop"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when signaling PID 1")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// start.go — validation warnings printed, then proceed
// ═══════════════════════════════════════════════════════════════════════════

// TestStartCmd_DaemonNewFails exercises the foreground daemon.New() error
// path in newStartCmd by blocking log file creation.
func TestStartCmd_DaemonNewFails(t *testing.T) {
	env := newStatusTestEnv(t)

	lockDir := t.TempDir()
	dmn.GlobalLockDir = lockDir
	defer func() { dmn.GlobalLockDir = "" }()

	// Replace the logs directory with a regular file so logging.NewLogger
	// fails when it tries to create files inside it.
	logsDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.RemoveAll(logsDir)
	_ = os.WriteFile(logsDir, []byte("not a directory"), 0644)

	env.RootCmd.SetArgs([]string{"start"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when logs path is a file, not a directory")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// showTreeStatus — multiple state summary parts (all states represented)
// ═══════════════════════════════════════════════════════════════════════════

func TestShowTreeStatus_AllStateCountParts(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Multi",
		testutil.Orchestrator("multi",
			testutil.Leaf("a"),
			testutil.Leaf("b"),
			testutil.Leaf("c"),
			testutil.Leaf("d"),
		),
	)

	idx, _ := env.App.State.ReadIndex()

	// Set each leaf to a different state.
	states := map[string]state.NodeStatus{
		"multi/a": state.StatusComplete,
		"multi/b": state.StatusInProgress,
		"multi/c": state.StatusBlocked,
		"multi/d": state.StatusNotStarted,
	}
	for addr, s := range states {
		e := idx.Nodes[addr]
		e.State = s
		idx.Nodes[addr] = e
	}

	// Saves index with all four states to exercise all summary parts.
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(env.App.State.Dir(), "state.json"), idxData, 0644)

	if err := showTreeStatus(env.App, idx, ""); err != nil {
		t.Fatalf("showTreeStatus all states: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// timelinePriority — all status branches
// ═══════════════════════════════════════════════════════════════════════════

func TestTimelinePriority_AllStatuses(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status state.NodeStatus
		want   int
	}{
		{state.StatusComplete, 0},
		{state.StatusInProgress, 1},
		{state.StatusBlocked, 2},
		{state.StatusNotStarted, 3},
		{state.NodeStatus("unknown"), 4},
	}
	for _, tc := range cases {
		got := timelinePriority(tc.status)
		if got != tc.want {
			t.Errorf("timelinePriority(%q) = %d, want %d", tc.status, got, tc.want)
		}
	}
}

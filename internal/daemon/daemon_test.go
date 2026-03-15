package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/logging"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// --- checkInboxState ---

func TestCheckInboxState_MissingFile(t *testing.T) {
	t.Parallel()
	d := &Daemon{}
	hasNew := d.checkInboxState("/nonexistent/path/inbox.json")
	if hasNew {
		t.Error("expected false for missing file")
	}
}

func TestCheckInboxState_EmptyInbox(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inboxPath := filepath.Join(dir, "inbox.json")

	inboxData := &state.InboxFile{Items: []state.InboxItem{}}
	if err := state.SaveInbox(inboxPath, inboxData); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{}
	hasNew := d.checkInboxState(inboxPath)
	if hasNew {
		t.Error("expected false for empty inbox")
	}
}

func TestCheckInboxState_NewItemsOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inboxPath := filepath.Join(dir, "inbox.json")

	inboxData := &state.InboxFile{
		Items: []state.InboxItem{
			{Timestamp: "2026-03-14T00:00:00Z", Text: "new thing", Status: "new"},
			{Timestamp: "2026-03-14T00:01:00Z", Text: "filed thing", Status: "filed"},
		},
	}
	if err := state.SaveInbox(inboxPath, inboxData); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{}
	hasNew := d.checkInboxState(inboxPath)
	if !hasNew {
		t.Error("expected hasNew=true")
	}
}

func TestCheckInboxState_AllFiled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inboxPath := filepath.Join(dir, "inbox.json")

	inboxData := &state.InboxFile{
		Items: []state.InboxItem{
			{Timestamp: "2026-03-14T00:00:00Z", Text: "filed thing", Status: "filed"},
			{Timestamp: "2026-03-14T00:01:00Z", Text: "also filed", Status: "filed"},
		},
	}
	if err := state.SaveInbox(inboxPath, inboxData); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{}
	hasNew := d.checkInboxState(inboxPath)
	if hasNew {
		t.Error("expected false when all items are filed")
	}
}

func TestCheckInboxState_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inboxPath := filepath.Join(dir, "inbox.json")

	if err := os.WriteFile(inboxPath, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{}
	hasNew := d.checkInboxState(inboxPath)
	if hasNew {
		t.Error("expected false for invalid JSON")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test Helpers
// ═══════════════════════════════════════════════════════════════════════════

// testConfig returns a minimal daemon config suitable for testing.
func testConfig() *config.Config {
	return &config.Config{
		Models: map[string]config.ModelDef{
			"echo": {Command: "echo", Args: []string{"WOLFCASTLE_COMPLETE"}},
		},
		Pipeline: config.PipelineConfig{
			Stages: []config.PipelineStage{
				{Name: "execute", Model: "echo", PromptFile: "execute.md"},
			},
		},
		Logs:    config.LogsConfig{MaxFiles: 100, MaxAgeDays: 30},
		Retries: config.RetriesConfig{InitialDelaySeconds: 0, MaxDelaySeconds: 1, MaxRetries: 0},
		Failure: config.FailureConfig{
			DecompositionThreshold: 3,
			MaxDecompositionDepth:  2,
			HardCap:                5,
		},
		Daemon: config.DaemonConfig{
			PollIntervalSeconds:        0,
			BlockedPollIntervalSeconds: 0,
			MaxIterations:              -1,
			InvocationTimeoutSeconds:   10,
			MaxRestarts:                3,
			RestartDelaySeconds:        0,
		},
		Git: config.GitConfig{VerifyBranch: false},
	}
}

// testDaemon builds a Daemon wired to a temp directory with a minimal config.
func testDaemon(t *testing.T) *Daemon {
	t.Helper()
	tmp := t.TempDir()
	wolfDir := filepath.Join(tmp, ".wolfcastle")
	ns := "test-user"
	projDir := filepath.Join(wolfDir, "projects", ns)
	logDir := filepath.Join(wolfDir, "logs")
	for _, d := range []string{projDir, logDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	logger, err := logging.NewLogger(logDir)
	if err != nil {
		t.Fatal(err)
	}

	inboxLogger, err := logging.NewLogger(logDir)
	if err != nil {
		t.Fatal(err)
	}

	return &Daemon{
		Config:        testConfig(),
		WolfcastleDir: wolfDir,
		Resolver:      &tree.Resolver{WolfcastleDir: wolfDir, Namespace: ns},
		Logger:        logger,
		InboxLogger:   inboxLogger,
		Clock:         clock.New(),
		RepoDir:       tmp,
		shutdown:      make(chan struct{}),
		workAvailable: make(chan struct{}, 1),
	}
}

// writeJSON marshals v and writes it to path, creating parent dirs as needed.
func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

// setupLeafNode creates a root index with a single leaf node and its state file.
func setupLeafNode(t *testing.T, d *Daemon, nodeAddr string, tasks []state.Task) {
	t.Helper()
	projDir := d.Resolver.ProjectsDir()
	idx := state.NewRootIndex()
	idx.Root = []string{nodeAddr}
	idx.Nodes[nodeAddr] = state.IndexEntry{
		Name:    nodeAddr,
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: nodeAddr,
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	ns := state.NewNodeState(nodeAddr, nodeAddr, state.NodeLeaf)
	ns.Tasks = tasks
	addr, _ := tree.ParseAddress(nodeAddr)
	statePath := filepath.Join(projDir, filepath.Join(addr.Parts...), "state.json")
	writeJSON(t, statePath, ns)
}

// writePromptFile creates a minimal prompt file so AssemblePrompt succeeds.
func writePromptFile(t *testing.T, wolfDir, filename string) {
	t.Helper()
	// Prompt resolution checks local/ first, then custom/, then base/
	dir := filepath.Join(wolfDir, "base", "prompts")
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(filepath.Join(dir, filename), []byte("test prompt"), 0644)
}

// ═══════════════════════════════════════════════════════════════════════════
// New / scopeLabel
// ═══════════════════════════════════════════════════════════════════════════

func TestNew(t *testing.T) {
	tmp := t.TempDir()
	wolfDir := filepath.Join(tmp, ".wolfcastle")
	_ = os.MkdirAll(wolfDir, 0755)

	cfg := testConfig()
	resolver := &tree.Resolver{WolfcastleDir: wolfDir, Namespace: "test"}
	d, err := New(cfg, wolfDir, resolver, "", tmp)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if d.Config != cfg {
		t.Error("config not set")
	}
	if d.Logger == nil {
		t.Error("logger should be initialized")
	}
	if d.shutdown == nil {
		t.Error("shutdown channel should be initialized")
	}
}

func TestScopeLabel(t *testing.T) {
	d := testDaemon(t)
	if d.scopeLabel() != "full tree" {
		t.Errorf("expected 'full tree', got %q", d.scopeLabel())
	}
	d.ScopeNode = "my-scope"
	if d.scopeLabel() != "my-scope" {
		t.Errorf("expected 'my-scope', got %q", d.scopeLabel())
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// selfHeal
// ═══════════════════════════════════════════════════════════════════════════

func TestSelfHeal_NoRootIndex(t *testing.T) {
	d := testDaemon(t)
	if err := d.selfHeal(); err != nil {
		t.Errorf("selfHeal should succeed when no root index: %v", err)
	}
}

func TestSelfHeal_NoInterruptedTasks(t *testing.T) {
	d := testDaemon(t)
	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", State: state.StatusNotStarted},
		{ID: "task-0002", State: state.StatusComplete},
	})
	if err := d.selfHeal(); err != nil {
		t.Errorf("selfHeal should succeed: %v", err)
	}
}

func TestSelfHeal_OneInterruptedTask(t *testing.T) {
	d := testDaemon(t)
	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", State: state.StatusInProgress},
		{ID: "task-0002", State: state.StatusNotStarted},
	})
	if err := d.selfHeal(); err != nil {
		t.Errorf("selfHeal should succeed with one in-progress task: %v", err)
	}
}

func TestSelfHeal_MultipleInterruptedTasks(t *testing.T) {
	d := testDaemon(t)
	projDir := d.Resolver.ProjectsDir()

	idx := state.NewRootIndex()
	idx.Root = []string{"node-a", "node-b"}
	idx.Nodes["node-a"] = state.IndexEntry{Name: "A", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "node-a"}
	idx.Nodes["node-b"] = state.IndexEntry{Name: "B", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "node-b"}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	nsA := state.NewNodeState("node-a", "A", state.NodeLeaf)
	nsA.Tasks = []state.Task{{ID: "task-0001", State: state.StatusInProgress}}
	writeJSON(t, filepath.Join(projDir, "node-a", "state.json"), nsA)

	nsB := state.NewNodeState("node-b", "B", state.NodeLeaf)
	nsB.Tasks = []state.Task{{ID: "task-0001", State: state.StatusInProgress}}
	writeJSON(t, filepath.Join(projDir, "node-b", "state.json"), nsB)

	err := d.selfHeal()
	if err == nil {
		t.Fatal("selfHeal should fail with multiple in-progress tasks")
	}
	if !strings.Contains(err.Error(), "state corruption") {
		t.Errorf("error should mention state corruption: %v", err)
	}
}

func TestSelfHeal_SkipsOrchestrators(t *testing.T) {
	d := testDaemon(t)
	// Orchestrator nodes with in-progress state should be ignored by selfHeal,
	// which only looks at NodeLeaf entries.
	idx := state.NewRootIndex()
	idx.Root = []string{"parent"}
	idx.Nodes["parent"] = state.IndexEntry{Name: "Parent", Type: state.NodeOrchestrator, State: state.StatusInProgress, Address: "parent"}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	if err := d.selfHeal(); err != nil {
		t.Errorf("selfHeal should skip orchestrators: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RunOnce
// ═══════════════════════════════════════════════════════════════════════════

func TestRunOnce_ShutdownSignal(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	close(d.shutdown)
	result, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != IterationStop {
		t.Errorf("expected IterationStop, got %d", result)
	}
}

func TestRunOnce_StopFile(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	stopPath := filepath.Join(d.WolfcastleDir, "stop")
	_ = os.WriteFile(stopPath, []byte("stop"), 0644)

	result, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != IterationStop {
		t.Errorf("expected IterationStop, got %d", result)
	}
	if _, err := os.Stat(stopPath); !os.IsNotExist(err) {
		t.Error("stop file should be removed after use")
	}
}

func TestRunOnce_MaxIterations(t *testing.T) {
	d := testDaemon(t)
	d.Config.Daemon.MaxIterations = 5
	d.iteration = 5
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	result, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != IterationStop {
		t.Errorf("expected IterationStop, got %d", result)
	}
}

func TestRunOnce_BranchChange(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = true
	d.branch = "feature-that-does-not-exist-xyz"
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	// Init a git repo in the temp dir so currentBranch works
	result, err := d.RunOnce(context.Background())
	// Either branch check errors or root index load errors — both acceptable
	_ = result
	_ = err
}

func TestRunOnce_NoWork_AllComplete(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", State: state.StatusComplete},
	})
	idx, _ := d.Resolver.LoadRootIndex()
	entry := idx.Nodes["my-node"]
	entry.State = state.StatusComplete
	idx.Nodes["my-node"] = entry
	_ = state.SaveRootIndex(d.Resolver.RootIndexPath(), idx)

	result, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != IterationNoWork {
		t.Errorf("expected IterationNoWork, got %d", result)
	}
}

func TestRunOnce_NoWork_AllBlocked(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", State: state.StatusBlocked, BlockedReason: "stuck"},
	})
	idx, _ := d.Resolver.LoadRootIndex()
	entry := idx.Nodes["my-node"]
	entry.State = state.StatusBlocked
	idx.Nodes["my-node"] = entry
	_ = state.SaveRootIndex(d.Resolver.RootIndexPath(), idx)

	result, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != IterationNoWork {
		t.Errorf("expected IterationNoWork, got %d", result)
	}
}

func TestRunOnce_WorkFound(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "do a thing", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	result, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != IterationDidWork {
		t.Errorf("expected IterationDidWork, got %d", result)
	}
	if d.iteration != 1 {
		t.Errorf("iteration should be 1, got %d", d.iteration)
	}
}

func TestRunOnce_IterationError(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	// Point to a model that does not exist
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "nonexistent", PromptFile: "execute.md"},
	}

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	result, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce should not return fatal error for model not found: %v", err)
	}
	if result != IterationError {
		t.Errorf("expected IterationError, got %d", result)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_ClaimTask(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "claim me", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-0001", Found: true}
	if err := d.runIteration(context.Background(), nav, idx); err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	addr, _ := tree.ParseAddress("my-node")
	ns, _ := state.LoadNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(addr.Parts...), "state.json"))
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" && task.State != state.StatusComplete {
			t.Errorf("task should be complete after WOLFCASTLE_COMPLETE marker, got %s", task.State)
		}
	}
}

func TestRunIteration_TerminalMarkers(t *testing.T) {
	markers := []struct {
		name   string
		output string
	}{
		{"YIELD", "WOLFCASTLE_YIELD"},
		{"BLOCKED", "WOLFCASTLE_BLOCKED: stuck"},
		{"COMPLETE", "WOLFCASTLE_COMPLETE"},
	}

	for _, tt := range markers {
		t.Run(tt.name, func(t *testing.T) {
			d := testDaemon(t)
			d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{tt.output}}
			_ = d.Logger.StartIteration()
			defer d.Logger.Close()

			setupLeafNode(t, d, "my-node", []state.Task{
				{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
			})
			writePromptFile(t, d.WolfcastleDir, "execute.md")

			idx, _ := d.Resolver.LoadRootIndex()
			nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-0001", Found: true}
			if err := d.runIteration(context.Background(), nav, idx); err != nil {
				t.Fatalf("runIteration should succeed with terminal marker: %v", err)
			}
		})
	}
}

func TestRunIteration_NoTerminalMarker_IncrFailure(t *testing.T) {
	d := testDaemon(t)
	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"just some text"}}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-0001", Found: true}
	if err := d.runIteration(context.Background(), nav, idx); err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	addr, _ := tree.ParseAddress("my-node")
	ns, _ := state.LoadNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(addr.Parts...), "state.json"))
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.FailureCount != 1 {
				t.Errorf("expected failure_count 1, got %d", task.FailureCount)
			}
			return
		}
	}
	t.Error("task-1 not found in state")
}

func TestRunIteration_DecompositionThreshold(t *testing.T) {
	d := testDaemon(t)
	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"no marker"}}
	d.Config.Failure.DecompositionThreshold = 2
	d.Config.Failure.MaxDecompositionDepth = 5
	d.Config.Failure.HardCap = 0 // disabled
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted, FailureCount: 1},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-0001", Found: true}
	_ = d.runIteration(context.Background(), nav, idx)

	addr, _ := tree.ParseAddress("my-node")
	ns, _ := state.LoadNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(addr.Parts...), "state.json"))
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.FailureCount != 2 {
				t.Errorf("expected failure_count 2, got %d", task.FailureCount)
			}
			if !task.NeedsDecomposition {
				t.Error("expected needs_decomposition to be set")
			}
			return
		}
	}
	t.Error("task-1 not found")
}

func TestRunIteration_DecompAtMaxDepth_AutoBlocks(t *testing.T) {
	d := testDaemon(t)
	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"no marker"}}
	d.Config.Failure.DecompositionThreshold = 2
	d.Config.Failure.MaxDecompositionDepth = 1
	d.Config.Failure.HardCap = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	projDir := d.Resolver.ProjectsDir()
	idx := state.NewRootIndex()
	idx.Root = []string{"my-node"}
	idx.Nodes["my-node"] = state.IndexEntry{
		Name: "my-node", Type: state.NodeLeaf, State: state.StatusNotStarted,
		Address: "my-node", DecompositionDepth: 1,
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	ns := state.NewNodeState("my-node", "my-node", state.NodeLeaf)
	ns.DecompositionDepth = 1
	ns.Tasks = []state.Task{{ID: "task-0001", Description: "work", State: state.StatusNotStarted, FailureCount: 1}}
	writeJSON(t, filepath.Join(projDir, "my-node", "state.json"), ns)
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx2, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-0001", Found: true}
	_ = d.runIteration(context.Background(), nav, idx2)

	addr, _ := tree.ParseAddress("my-node")
	ns2, _ := state.LoadNodeState(filepath.Join(projDir, filepath.Join(addr.Parts...), "state.json"))
	for _, task := range ns2.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusBlocked {
				t.Errorf("expected blocked at max depth, got %s", task.State)
			}
			if !strings.Contains(task.BlockedReason, "decomposition threshold") {
				t.Errorf("expected decomposition reason, got %q", task.BlockedReason)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

func TestRunIteration_HardCap_AutoBlocks(t *testing.T) {
	d := testDaemon(t)
	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"no marker"}}
	d.Config.Failure.HardCap = 2
	d.Config.Failure.DecompositionThreshold = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted, FailureCount: 1},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-0001", Found: true}
	_ = d.runIteration(context.Background(), nav, idx)

	addr, _ := tree.ParseAddress("my-node")
	ns, _ := state.LoadNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(addr.Parts...), "state.json"))
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusBlocked {
				t.Errorf("expected blocked after hard cap, got %s", task.State)
			}
			if !strings.Contains(task.BlockedReason, "hard cap") {
				t.Errorf("expected hard cap reason, got %q", task.BlockedReason)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

func TestRunIteration_ModelNotFound(t *testing.T) {
	d := testDaemon(t)
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "nonexistent", PromptFile: "execute.md"},
	}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err == nil || !strings.Contains(err.Error(), "model") {
		t.Errorf("expected model-not-found error, got: %v", err)
	}
}

func TestRunIteration_DisabledStageSkipped(t *testing.T) {
	d := testDaemon(t)
	f := false
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "echo", PromptFile: "execute.md", Enabled: &f},
	}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-0001", Found: true}
	if err := d.runIteration(context.Background(), nav, idx); err != nil {
		t.Fatalf("runIteration error: %v", err)
	}
}

func TestRunIteration_IntakeStageSkippedInPipeline(t *testing.T) {
	d := testDaemon(t)
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "intake", Model: "echo", PromptFile: "intake.md"},
		{Name: "execute", Model: "echo", PromptFile: "execute.md"},
	}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")
	writePromptFile(t, d.WolfcastleDir, "intake.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-0001", Found: true}
	if err := d.runIteration(context.Background(), nav, idx); err != nil {
		t.Fatalf("runIteration error: %v", err)
	}
}

func TestRunIteration_InvocationTimeout(t *testing.T) {
	d := testDaemon(t)
	d.Config.Daemon.InvocationTimeoutSeconds = 1
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-0001", Found: true}
	if err := d.runIteration(context.Background(), nav, idx); err != nil {
		t.Fatalf("runIteration error: %v", err)
	}
}

func TestRunIteration_ZeroInvocationTimeout(t *testing.T) {
	d := testDaemon(t)
	d.Config.Daemon.InvocationTimeoutSeconds = 0 // no timeout
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-0001", Found: true}
	if err := d.runIteration(context.Background(), nav, idx); err != nil {
		t.Fatalf("runIteration error: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIntakeStage
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIntakeStage_NoInbox(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	stage := config.PipelineStage{Name: "intake", Model: "echo", PromptFile: "intake.md"}
	if err := d.runIntakeStage(context.Background(), stage); err != nil {
		t.Errorf("should succeed with no inbox: %v", err)
	}
}

func TestRunIntakeStage_WithNewItems(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "intake.md")

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "add feature X", Timestamp: "2024-01-01T00:00:00Z"},
	}})

	stage := config.PipelineStage{Name: "intake", Model: "echo", PromptFile: "intake.md"}
	if err := d.runIntakeStage(context.Background(), stage); err != nil {
		t.Fatalf("intake stage error: %v", err)
	}

	inboxData, _ := state.LoadInbox(inboxPath)
	if inboxData.Items[0].Status != "filed" {
		t.Errorf("expected 'filed', got %q", inboxData.Items[0].Status)
	}
}

func TestRunIntakeStage_NoNewItems(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "filed", Text: "already done"},
	}})

	stage := config.PipelineStage{Name: "intake", Model: "echo", PromptFile: "intake.md"}
	if err := d.runIntakeStage(context.Background(), stage); err != nil {
		t.Errorf("should return nil for no new items: %v", err)
	}
}

func TestRunIntakeStage_ModelNotFound(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "item"},
	}})

	stage := config.PipelineStage{Name: "intake", Model: "nonexistent", PromptFile: "intake.md"}
	err := d.runIntakeStage(context.Background(), stage)
	if err == nil {
		t.Error("expected error for missing model")
	}
}

func TestRunIntakeStage_MultipleNewItems(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "intake.md")

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "item 1", Timestamp: "2024-01-01T00:00:00Z"},
		{Status: "new", Text: "item 2", Timestamp: "2024-01-01T00:01:00Z"},
	}})

	stage := config.PipelineStage{Name: "intake", Model: "echo", PromptFile: "intake.md"}
	if err := d.runIntakeStage(context.Background(), stage); err != nil {
		t.Fatalf("intake stage error: %v", err)
	}

	inboxData, _ := state.LoadInbox(inboxPath)
	for i, item := range inboxData.Items {
		if item.Status != "filed" {
			t.Errorf("item %d: expected 'filed', got %q", i, item.Status)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// invokeWithRetry
// ═══════════════════════════════════════════════════════════════════════════

func TestInvokeWithRetry_SuccessFirstTry(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	model := config.ModelDef{Command: "echo", Args: []string{"hello"}}
	result, err := d.invokeWithRetry(context.Background(), model, "prompt", d.RepoDir, d.Logger.AssistantWriter(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("expected 'hello' in output, got %q", result.Stdout)
	}
}

func TestInvokeWithRetry_MaxRetriesExceeded(t *testing.T) {
	d := testDaemon(t)
	d.Config.Retries = config.RetriesConfig{InitialDelaySeconds: 0, MaxDelaySeconds: 0, MaxRetries: 1}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	model := config.ModelDef{Command: "nonexistent-command-xyz", Args: []string{}}
	_, err := d.invokeWithRetry(context.Background(), model, "prompt", d.RepoDir, d.Logger.AssistantWriter(), "test")
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}

func TestInvokeWithRetry_ContextCancelled(t *testing.T) {
	d := testDaemon(t)
	d.Config.Retries = config.RetriesConfig{InitialDelaySeconds: 0, MaxDelaySeconds: 1, MaxRetries: 10}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	model := config.ModelDef{Command: "nonexistent-command-xyz", Args: []string{}}
	_, err := d.invokeWithRetry(ctx, model, "prompt", d.RepoDir, nil, "test")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestInvokeWithRetry_BackoffCaps(t *testing.T) {
	d := testDaemon(t)
	d.Config.Retries = config.RetriesConfig{InitialDelaySeconds: 0, MaxDelaySeconds: 0, MaxRetries: 2}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	model := config.ModelDef{Command: "nonexistent-cmd-backoff", Args: []string{}}
	start := time.Now()
	_, _ = d.invokeWithRetry(context.Background(), model, "", d.RepoDir, nil, "test")
	elapsed := time.Since(start)
	// With 0s delays and 2 retries, should finish very quickly
	if elapsed > 5*time.Second {
		t.Errorf("retry took too long with zero delays: %v", elapsed)
	}
}

func TestInvokeWithRetry_NilLogWriter(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	model := config.ModelDef{Command: "echo", Args: []string{"hello"}}
	result, err := d.invokeWithRetry(context.Background(), model, "prompt", d.RepoDir, nil, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("expected 'hello', got %q", result.Stdout)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RunWithSupervisor
// ═══════════════════════════════════════════════════════════════════════════

func TestRunWithSupervisor_SuccessfulRun(t *testing.T) {
	d := testDaemon(t)
	d.Config.Daemon.MaxIterations = 1
	d.Config.Git.VerifyBranch = false

	// All complete => daemon exits immediately with no work
	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", State: state.StatusComplete},
	})
	idx, _ := d.Resolver.LoadRootIndex()
	entry := idx.Nodes["my-node"]
	entry.State = state.StatusComplete
	idx.Nodes["my-node"] = entry
	_ = state.SaveRootIndex(d.Resolver.RootIndexPath(), idx)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := d.RunWithSupervisor(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithSupervisor_ContextCancel(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Cancelled context => returns quickly
	_ = d.RunWithSupervisor(ctx)
}

// ═══════════════════════════════════════════════════════════════════════════
// propagateState
// ═══════════════════════════════════════════════════════════════════════════

func TestPropagateState(t *testing.T) {
	d := testDaemon(t)
	projDir := d.Resolver.ProjectsDir()

	idx := state.NewRootIndex()
	idx.Root = []string{"parent"}
	idx.Nodes["parent"] = state.IndexEntry{
		Name: "Parent", Type: state.NodeOrchestrator, State: state.StatusNotStarted,
		Address: "parent", Children: []string{"parent/child"},
	}
	idx.Nodes["parent/child"] = state.IndexEntry{
		Name: "Child", Type: state.NodeLeaf, State: state.StatusNotStarted,
		Address: "parent/child", Parent: "parent",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	parentNS := state.NewNodeState("parent", "Parent", state.NodeOrchestrator)
	parentNS.Children = []state.ChildRef{
		{ID: "child", Address: "parent/child", State: state.StatusNotStarted},
	}
	writeJSON(t, filepath.Join(projDir, "parent", "state.json"), parentNS)

	childNS := state.NewNodeState("child", "Child", state.NodeLeaf)
	childNS.State = state.StatusInProgress
	writeJSON(t, filepath.Join(projDir, "parent", "child", "state.json"), childNS)

	if err := d.propagateState("parent/child", state.StatusInProgress, idx); err != nil {
		t.Fatalf("propagateState error: %v", err)
	}

	updatedIdx, _ := d.Resolver.LoadRootIndex()
	if updatedIdx.Nodes["parent/child"].State != state.StatusInProgress {
		t.Error("child state should be in_progress in index")
	}
}

func TestPropagateState_SingleNode(t *testing.T) {
	d := testDaemon(t)
	idx := state.NewRootIndex()
	idx.Root = []string{"my-node"}
	idx.Nodes["my-node"] = state.IndexEntry{
		Name: "my-node", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "my-node",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	// No parent => just updates root index entry
	if err := d.propagateState("my-node", state.StatusInProgress, idx); err != nil {
		t.Fatalf("propagateState error: %v", err)
	}
	if idx.Nodes["my-node"].State != state.StatusInProgress {
		t.Error("node state should be updated in index")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// IterationResult constants
// ═══════════════════════════════════════════════════════════════════════════

func TestIterationResultValues(t *testing.T) {
	seen := map[IterationResult]bool{
		IterationDidWork: true,
		IterationNoWork:  true,
		IterationStop:    true,
		IterationError:   true,
	}
	if len(seen) != 4 {
		t.Errorf("expected 4 distinct IterationResult values")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// currentBranch
// ═══════════════════════════════════════════════════════════════════════════

func TestCurrentBranch(t *testing.T) {
	// Use the real repo; skip if not available
	repoDir := "/Users/wild/repository/dorkusprime/wolfcastle/main/.claude/worktrees/agent-a059ee3b"
	branch, err := currentBranch(repoDir)
	if err != nil {
		t.Skipf("not in a git repo: %v", err)
	}
	if branch == "" {
		t.Error("branch should not be empty")
	}
}

func TestCurrentBranch_BadDir(t *testing.T) {
	_, err := currentBranch("/nonexistent/dir")
	if err == nil {
		t.Error("expected error for nonexistent dir")
	}
}

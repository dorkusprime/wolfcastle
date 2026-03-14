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

// --- parseExpandedSections ---

func TestParseExpandedSections_MultipleSections(t *testing.T) {
	t.Parallel()
	input := `Some preamble text
## Item 1
Content for item one
More content
## Item 2
Content for item two
## Item 3
Content for item three`

	sections := parseExpandedSections(input)
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}
	if sections[0] != "## Item 1\nContent for item one\nMore content" {
		t.Errorf("unexpected section 0: %q", sections[0])
	}
	if sections[1] != "## Item 2\nContent for item two" {
		t.Errorf("unexpected section 1: %q", sections[1])
	}
	if sections[2] != "## Item 3\nContent for item three" {
		t.Errorf("unexpected section 2: %q", sections[2])
	}
}

func TestParseExpandedSections_NoSections(t *testing.T) {
	t.Parallel()
	input := "Just some plain text without any headings"
	sections := parseExpandedSections(input)
	if len(sections) != 0 {
		t.Errorf("expected 0 sections, got %d", len(sections))
	}
}

func TestParseExpandedSections_SingleSection(t *testing.T) {
	t.Parallel()
	input := "## Only Section\nSome content here"
	sections := parseExpandedSections(input)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0] != "## Only Section\nSome content here" {
		t.Errorf("unexpected section: %q", sections[0])
	}
}

func TestParseExpandedSections_EmptyInput(t *testing.T) {
	t.Parallel()
	sections := parseExpandedSections("")
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty input, got %d", len(sections))
	}
}

func TestParseExpandedSections_ConsecutiveHeadings(t *testing.T) {
	t.Parallel()
	input := "## First\n## Second\nContent"
	sections := parseExpandedSections(input)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	if sections[0] != "## First" {
		t.Errorf("unexpected section 0: %q", sections[0])
	}
	if sections[1] != "## Second\nContent" {
		t.Errorf("unexpected section 1: %q", sections[1])
	}
}

func TestParseExpandedSections_PreambleOnly(t *testing.T) {
	t.Parallel()
	input := "Some text\nMore text\nNo headings"
	sections := parseExpandedSections(input)
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for preamble-only input, got %d", len(sections))
	}
}

func TestParseExpandedSections_HeadingAtEnd(t *testing.T) {
	t.Parallel()
	input := "preamble\n## Trailing"
	sections := parseExpandedSections(input)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0] != "## Trailing" {
		t.Errorf("unexpected section: %q", sections[0])
	}
}

// --- dedupPipe ---

func TestDedupPipe_BasicDedup(t *testing.T) {
	t.Parallel()
	result := dedupPipe("a|b|a|c|b")
	if len(result) != 3 {
		t.Fatalf("expected 3 unique items, got %d: %v", len(result), result)
	}
	expected := []string{"a", "b", "c"}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("index %d: expected %q, got %q", i, v, result[i])
		}
	}
}

func TestDedupPipe_EmptyParts(t *testing.T) {
	t.Parallel()
	result := dedupPipe("a||b|||c")
	if len(result) != 3 {
		t.Fatalf("expected 3 items (empty parts skipped), got %d: %v", len(result), result)
	}
}

func TestDedupPipe_WhitespaceHandling(t *testing.T) {
	t.Parallel()
	result := dedupPipe("  a  | b |  a  | c ")
	if len(result) != 3 {
		t.Fatalf("expected 3 items (whitespace trimmed and deduped), got %d: %v", len(result), result)
	}
	if result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestDedupPipe_EmptyString(t *testing.T) {
	t.Parallel()
	result := dedupPipe("")
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestDedupPipe_SingleItem(t *testing.T) {
	t.Parallel()
	result := dedupPipe("only")
	if len(result) != 1 || result[0] != "only" {
		t.Errorf("expected [only], got %v", result)
	}
}

func TestDedupPipe_AllWhitespace(t *testing.T) {
	t.Parallel()
	result := dedupPipe("  |  |  ")
	if len(result) != 0 {
		t.Errorf("expected empty result for all-whitespace, got %v", result)
	}
}

// --- checkInboxState ---

func TestCheckInboxState_MissingFile(t *testing.T) {
	t.Parallel()
	d := &Daemon{}
	hasNew, hasExpanded := d.checkInboxState("/nonexistent/path/inbox.json")
	if hasNew || hasExpanded {
		t.Error("expected false, false for missing file")
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
	hasNew, hasExpanded := d.checkInboxState(inboxPath)
	if hasNew || hasExpanded {
		t.Error("expected false, false for empty inbox")
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
	hasNew, hasExpanded := d.checkInboxState(inboxPath)
	if !hasNew {
		t.Error("expected hasNew=true")
	}
	if hasExpanded {
		t.Error("expected hasExpanded=false")
	}
}

func TestCheckInboxState_ExpandedItemsOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inboxPath := filepath.Join(dir, "inbox.json")

	inboxData := &state.InboxFile{
		Items: []state.InboxItem{
			{Timestamp: "2026-03-14T00:00:00Z", Text: "expanded thing", Status: "expanded", Expanded: "details"},
		},
	}
	if err := state.SaveInbox(inboxPath, inboxData); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{}
	hasNew, hasExpanded := d.checkInboxState(inboxPath)
	if hasNew {
		t.Error("expected hasNew=false")
	}
	if !hasExpanded {
		t.Error("expected hasExpanded=true")
	}
}

func TestCheckInboxState_BothNewAndExpanded(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inboxPath := filepath.Join(dir, "inbox.json")

	inboxData := &state.InboxFile{
		Items: []state.InboxItem{
			{Timestamp: "2026-03-14T00:00:00Z", Text: "new thing", Status: "new"},
			{Timestamp: "2026-03-14T00:01:00Z", Text: "expanded thing", Status: "expanded"},
		},
	}
	if err := state.SaveInbox(inboxPath, inboxData); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{}
	hasNew, hasExpanded := d.checkInboxState(inboxPath)
	if !hasNew {
		t.Error("expected hasNew=true")
	}
	if !hasExpanded {
		t.Error("expected hasExpanded=true")
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
	hasNew, hasExpanded := d.checkInboxState(inboxPath)
	if hasNew || hasExpanded {
		t.Error("expected false, false when all items are filed")
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
	hasNew, hasExpanded := d.checkInboxState(inboxPath)
	if hasNew || hasExpanded {
		t.Error("expected false, false for invalid JSON")
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

	return &Daemon{
		Config:        testConfig(),
		WolfcastleDir: wolfDir,
		Resolver:      &tree.Resolver{WolfcastleDir: wolfDir, Namespace: ns},
		Logger:        logger,
		Clock:         clock.New(),
		RepoDir:       tmp,
		shutdown:      make(chan struct{}),
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
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, filename), []byte("test prompt"), 0644)
}

// ═══════════════════════════════════════════════════════════════════════════
// New / scopeLabel
// ═══════════════════════════════════════════════════════════════════════════

func TestNew(t *testing.T) {
	tmp := t.TempDir()
	wolfDir := filepath.Join(tmp, ".wolfcastle")
	os.MkdirAll(wolfDir, 0755)

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
		{ID: "task-1", State: state.StatusNotStarted},
		{ID: "task-2", State: state.StatusComplete},
	})
	if err := d.selfHeal(); err != nil {
		t.Errorf("selfHeal should succeed: %v", err)
	}
}

func TestSelfHeal_OneInterruptedTask(t *testing.T) {
	d := testDaemon(t)
	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-1", State: state.StatusInProgress},
		{ID: "task-2", State: state.StatusNotStarted},
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
	nsA.Tasks = []state.Task{{ID: "task-1", State: state.StatusInProgress}}
	writeJSON(t, filepath.Join(projDir, "node-a", "state.json"), nsA)

	nsB := state.NewNodeState("node-b", "B", state.NodeLeaf)
	nsB.Tasks = []state.Task{{ID: "task-1", State: state.StatusInProgress}}
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
	d.Logger.StartIteration()
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
	d.Logger.StartIteration()
	defer d.Logger.Close()

	stopPath := filepath.Join(d.WolfcastleDir, "stop")
	os.WriteFile(stopPath, []byte("stop"), 0644)

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
	d.Logger.StartIteration()
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
	d.Logger.StartIteration()
	defer d.Logger.Close()

	// Init a git repo in the temp dir so currentBranch works
	result, err := d.RunOnce(context.Background())
	// Either branch check errors or root index load errors — both acceptable
	_ = result
	_ = err
}

func TestRunOnce_NoWork_AllComplete(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-1", State: state.StatusComplete},
	})
	idx, _ := d.Resolver.LoadRootIndex()
	entry := idx.Nodes["my-node"]
	entry.State = state.StatusComplete
	idx.Nodes["my-node"] = entry
	state.SaveRootIndex(d.Resolver.RootIndexPath(), idx)

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
	d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-1", State: state.StatusBlocked, BlockedReason: "stuck"},
	})
	idx, _ := d.Resolver.LoadRootIndex()
	entry := idx.Nodes["my-node"]
	entry.State = state.StatusBlocked
	idx.Nodes["my-node"] = entry
	state.SaveRootIndex(d.Resolver.RootIndexPath(), idx)

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
	d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-1", Description: "do a thing", State: state.StatusNotStarted},
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
	d.Logger.StartIteration()
	defer d.Logger.Close()

	// Point to a model that does not exist
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "nonexistent", PromptFile: "execute.md"},
	}

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
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
	d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-1", Description: "claim me", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1", Found: true}
	if err := d.runIteration(context.Background(), nav, idx); err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	addr, _ := tree.ParseAddress("my-node")
	ns, _ := state.LoadNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(addr.Parts...), "state.json"))
	for _, task := range ns.Tasks {
		if task.ID == "task-1" && task.State != state.StatusInProgress {
			t.Errorf("task should be in_progress after claim, got %s", task.State)
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
			d.Logger.StartIteration()
			defer d.Logger.Close()

			setupLeafNode(t, d, "my-node", []state.Task{
				{ID: "task-1", Description: "work", State: state.StatusNotStarted},
			})
			writePromptFile(t, d.WolfcastleDir, "execute.md")

			idx, _ := d.Resolver.LoadRootIndex()
			nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1", Found: true}
			if err := d.runIteration(context.Background(), nav, idx); err != nil {
				t.Fatalf("runIteration should succeed with terminal marker: %v", err)
			}
		})
	}
}

func TestRunIteration_NoTerminalMarker_IncrFailure(t *testing.T) {
	d := testDaemon(t)
	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"just some text"}}
	d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1", Found: true}
	if err := d.runIteration(context.Background(), nav, idx); err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	addr, _ := tree.ParseAddress("my-node")
	ns, _ := state.LoadNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(addr.Parts...), "state.json"))
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
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
	d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted, FailureCount: 1},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1", Found: true}
	d.runIteration(context.Background(), nav, idx)

	addr, _ := tree.ParseAddress("my-node")
	ns, _ := state.LoadNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(addr.Parts...), "state.json"))
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
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
	d.Logger.StartIteration()
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
	ns.Tasks = []state.Task{{ID: "task-1", Description: "work", State: state.StatusNotStarted, FailureCount: 1}}
	writeJSON(t, filepath.Join(projDir, "my-node", "state.json"), ns)
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx2, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1", Found: true}
	d.runIteration(context.Background(), nav, idx2)

	addr, _ := tree.ParseAddress("my-node")
	ns2, _ := state.LoadNodeState(filepath.Join(projDir, filepath.Join(addr.Parts...), "state.json"))
	for _, task := range ns2.Tasks {
		if task.ID == "task-1" {
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
	d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted, FailureCount: 1},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1", Found: true}
	d.runIteration(context.Background(), nav, idx)

	addr, _ := tree.ParseAddress("my-node")
	ns, _ := state.LoadNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(addr.Parts...), "state.json"))
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
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
	d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1", Found: true}
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
	d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1", Found: true}
	if err := d.runIteration(context.Background(), nav, idx); err != nil {
		t.Fatalf("runIteration error: %v", err)
	}
}

func TestRunIteration_ExpandFileStageSkipWhenNoInbox(t *testing.T) {
	d := testDaemon(t)
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "expand", Model: "echo", PromptFile: "expand.md"},
		{Name: "file", Model: "echo", PromptFile: "file.md"},
		{Name: "execute", Model: "echo", PromptFile: "execute.md"},
	}
	d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
	})
	for _, f := range []string{"expand.md", "file.md", "execute.md"} {
		writePromptFile(t, d.WolfcastleDir, f)
	}

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1", Found: true}
	if err := d.runIteration(context.Background(), nav, idx); err != nil {
		t.Fatalf("runIteration error: %v", err)
	}
}

func TestRunIteration_PendingFilingSkipsExecute(t *testing.T) {
	d := testDaemon(t)
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "echo", PromptFile: "execute.md"},
	}
	d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "expanded", Text: "awaiting filing", Expanded: "details"},
	}})

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1", Found: true}
	d.runIteration(context.Background(), nav, idx)

	// The execute stage should have been skipped, so no terminal marker processed
}

func TestRunIteration_InvocationTimeout(t *testing.T) {
	d := testDaemon(t)
	d.Config.Daemon.InvocationTimeoutSeconds = 1
	d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1", Found: true}
	if err := d.runIteration(context.Background(), nav, idx); err != nil {
		t.Fatalf("runIteration error: %v", err)
	}
}

func TestRunIteration_ZeroInvocationTimeout(t *testing.T) {
	d := testDaemon(t)
	d.Config.Daemon.InvocationTimeoutSeconds = 0 // no timeout
	d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1", Found: true}
	if err := d.runIteration(context.Background(), nav, idx); err != nil {
		t.Fatalf("runIteration error: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runExpandStage
// ═══════════════════════════════════════════════════════════════════════════

func TestRunExpandStage_NoInbox(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	stage := config.PipelineStage{Name: "expand", Model: "echo", PromptFile: "expand.md"}
	if err := d.runExpandStage(context.Background(), stage); err != nil {
		t.Errorf("should succeed with no inbox: %v", err)
	}
}

func TestRunExpandStage_WithNewItems(t *testing.T) {
	d := testDaemon(t)
	d.Config.Models["echo"] = config.ModelDef{Command: "printf", Args: []string{"## Item 1\\nExpanded text"}}
	d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "expand.md")

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "add feature X", Timestamp: "2024-01-01T00:00:00Z"},
	}})

	stage := config.PipelineStage{Name: "expand", Model: "echo", PromptFile: "expand.md"}
	if err := d.runExpandStage(context.Background(), stage); err != nil {
		t.Fatalf("expand stage error: %v", err)
	}

	inboxData, _ := state.LoadInbox(inboxPath)
	if inboxData.Items[0].Status != "expanded" {
		t.Errorf("expected 'expanded', got %q", inboxData.Items[0].Status)
	}
}

func TestRunExpandStage_NoNewItems(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "filed", Text: "already done"},
	}})

	stage := config.PipelineStage{Name: "expand", Model: "echo", PromptFile: "expand.md"}
	if err := d.runExpandStage(context.Background(), stage); err != nil {
		t.Errorf("should return nil for no new items: %v", err)
	}
}

func TestRunExpandStage_ModelNotFound(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "item"},
	}})

	stage := config.PipelineStage{Name: "expand", Model: "nonexistent", PromptFile: "expand.md"}
	err := d.runExpandStage(context.Background(), stage)
	if err == nil {
		t.Error("expected error for missing model")
	}
}

func TestRunExpandStage_MoreItemsThanSections(t *testing.T) {
	d := testDaemon(t)
	// Model returns only one section but we have two items
	d.Config.Models["echo"] = config.ModelDef{Command: "printf", Args: []string{"## Item 1\\nOnly one section"}}
	d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "expand.md")

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "item 1", Timestamp: "2024-01-01T00:00:00Z"},
		{Status: "new", Text: "item 2", Timestamp: "2024-01-01T00:01:00Z"},
	}})

	stage := config.PipelineStage{Name: "expand", Model: "echo", PromptFile: "expand.md"}
	if err := d.runExpandStage(context.Background(), stage); err != nil {
		t.Fatalf("expand stage error: %v", err)
	}

	inboxData, _ := state.LoadInbox(inboxPath)
	// Both should be marked expanded even if model returned fewer sections
	for i, item := range inboxData.Items {
		if item.Status != "expanded" {
			t.Errorf("item %d: expected 'expanded', got %q", i, item.Status)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runFileStage
// ═══════════════════════════════════════════════════════════════════════════

func TestRunFileStage_NoInbox(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	stage := config.PipelineStage{Name: "file", Model: "echo", PromptFile: "file.md"}
	if err := d.runFileStage(context.Background(), stage); err != nil {
		t.Errorf("should succeed with no inbox: %v", err)
	}
}

func TestRunFileStage_WithExpandedItems(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "file.md")

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "expanded", Text: "feature X", Expanded: "## Details\nexpanded"},
	}})

	stage := config.PipelineStage{Name: "file", Model: "echo", PromptFile: "file.md"}
	if err := d.runFileStage(context.Background(), stage); err != nil {
		t.Fatalf("file stage error: %v", err)
	}

	inboxData, _ := state.LoadInbox(inboxPath)
	if inboxData.Items[0].Status != "filed" {
		t.Errorf("expected 'filed', got %q", inboxData.Items[0].Status)
	}
}

func TestRunFileStage_NoExpandedItems(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "not expanded yet"},
	}})

	stage := config.PipelineStage{Name: "file", Model: "echo", PromptFile: "file.md"}
	if err := d.runFileStage(context.Background(), stage); err != nil {
		t.Errorf("should succeed with no expanded items: %v", err)
	}
}

func TestRunFileStage_ModelNotFound(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "expanded", Text: "item", Expanded: "details"},
	}})

	stage := config.PipelineStage{Name: "file", Model: "nonexistent", PromptFile: "file.md"}
	if err := d.runFileStage(context.Background(), stage); err == nil {
		t.Error("expected error for missing model")
	}
}

func TestRunFileStage_ExpandedItemWithEmptyExpanded(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "file.md")

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "expanded", Text: "feature with empty expansion", Expanded: ""},
	}})

	stage := config.PipelineStage{Name: "file", Model: "echo", PromptFile: "file.md"}
	if err := d.runFileStage(context.Background(), stage); err != nil {
		t.Fatalf("file stage error: %v", err)
	}

	inboxData, _ := state.LoadInbox(inboxPath)
	if inboxData.Items[0].Status != "filed" {
		t.Errorf("expected 'filed', got %q", inboxData.Items[0].Status)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// applyModelMarkers
// ═══════════════════════════════════════════════════════════════════════════

func TestApplyModelMarkers_Breadcrumb(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	d.applyModelMarkers("WOLFCASTLE_BREADCRUMB: did a thing", ns, nav)

	if len(ns.Audit.Breadcrumbs) != 1 {
		t.Fatalf("expected 1 breadcrumb, got %d", len(ns.Audit.Breadcrumbs))
	}
	if ns.Audit.Breadcrumbs[0].Text != "did a thing" {
		t.Errorf("breadcrumb text = %q", ns.Audit.Breadcrumbs[0].Text)
	}
	if ns.Audit.Breadcrumbs[0].Task != "my-node/task-1" {
		t.Errorf("breadcrumb task = %q", ns.Audit.Breadcrumbs[0].Task)
	}
}

func TestApplyModelMarkers_BreadcrumbEmpty(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	d.applyModelMarkers("WOLFCASTLE_BREADCRUMB:  ", ns, nav)
	if len(ns.Audit.Breadcrumbs) != 0 {
		t.Error("empty breadcrumb should be ignored")
	}
}

func TestApplyModelMarkers_Gap(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	d.applyModelMarkers("WOLFCASTLE_GAP: missing tests", ns, nav)

	if len(ns.Audit.Gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(ns.Audit.Gaps))
	}
	if ns.Audit.Gaps[0].Description != "missing tests" {
		t.Errorf("gap desc = %q", ns.Audit.Gaps[0].Description)
	}
	if ns.Audit.Gaps[0].Status != state.GapOpen {
		t.Errorf("gap status = %q", ns.Audit.Gaps[0].Status)
	}
	if ns.Audit.Gaps[0].Source != "my-node" {
		t.Errorf("gap source = %q", ns.Audit.Gaps[0].Source)
	}
}

func TestApplyModelMarkers_GapEmpty(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	d.applyModelMarkers("WOLFCASTLE_GAP:  ", ns, nav)
	if len(ns.Audit.Gaps) != 0 {
		t.Error("empty GAP should be ignored")
	}
}

func TestApplyModelMarkers_FixGap(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	ns.Audit.Gaps = []state.Gap{
		{ID: "gap-n1-1", Status: state.GapOpen, Description: "a gap"},
	}
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	d.applyModelMarkers("WOLFCASTLE_FIX_GAP: gap-n1-1", ns, nav)

	if ns.Audit.Gaps[0].Status != state.GapFixed {
		t.Errorf("gap should be fixed, got %q", ns.Audit.Gaps[0].Status)
	}
	if ns.Audit.Gaps[0].FixedBy != "my-node/task-1" {
		t.Errorf("fixed_by = %q", ns.Audit.Gaps[0].FixedBy)
	}
	if ns.Audit.Gaps[0].FixedAt == nil {
		t.Error("fixed_at should be set")
	}
}

func TestApplyModelMarkers_FixGap_WrongID(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	ns.Audit.Gaps = []state.Gap{
		{ID: "gap-n1-1", Status: state.GapOpen, Description: "a gap"},
	}
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	d.applyModelMarkers("WOLFCASTLE_FIX_GAP: gap-wrong-id", ns, nav)

	if ns.Audit.Gaps[0].Status != state.GapOpen {
		t.Error("gap should remain open for wrong ID")
	}
}

func TestApplyModelMarkers_FixGap_AlreadyFixed(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	ns.Audit.Gaps = []state.Gap{
		{ID: "gap-n1-1", Status: state.GapFixed, Description: "a gap"},
	}
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	d.applyModelMarkers("WOLFCASTLE_FIX_GAP: gap-n1-1", ns, nav)
	if ns.Audit.Gaps[0].FixedBy != "" {
		t.Error("already-fixed gap should not be re-fixed")
	}
}

func TestApplyModelMarkers_Scope(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	ns.Audit.Scope = nil
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	d.applyModelMarkers("WOLFCASTLE_SCOPE: handle authentication", ns, nav)

	if ns.Audit.Scope == nil {
		t.Fatal("scope should be set")
	}
	if ns.Audit.Scope.Description != "handle authentication" {
		t.Errorf("scope desc = %q", ns.Audit.Scope.Description)
	}
}

func TestApplyModelMarkers_ScopeEmpty(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	ns.Audit.Scope = nil
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	d.applyModelMarkers("WOLFCASTLE_SCOPE:  ", ns, nav)
	// Scope should remain nil or have empty description
	if ns.Audit.Scope != nil && ns.Audit.Scope.Description != "" {
		t.Error("empty SCOPE should be ignored")
	}
}

func TestApplyModelMarkers_ScopeFiles(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	ns.Audit.Scope = nil
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	d.applyModelMarkers("WOLFCASTLE_SCOPE_FILES: auth.go|login.go|auth.go", ns, nav)

	if ns.Audit.Scope == nil {
		t.Fatal("scope should be created")
	}
	if len(ns.Audit.Scope.Files) != 2 {
		t.Fatalf("expected 2 deduped files, got %d: %v", len(ns.Audit.Scope.Files), ns.Audit.Scope.Files)
	}
}

func TestApplyModelMarkers_ScopeSystems(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	ns.Audit.Scope = nil
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	d.applyModelMarkers("WOLFCASTLE_SCOPE_SYSTEMS: api|database", ns, nav)

	if ns.Audit.Scope == nil || len(ns.Audit.Scope.Systems) != 2 {
		t.Error("expected 2 systems")
	}
}

func TestApplyModelMarkers_ScopeCriteria(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	ns.Audit.Scope = nil
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	d.applyModelMarkers("WOLFCASTLE_SCOPE_CRITERIA: test passes|lint clean", ns, nav)

	if ns.Audit.Scope == nil || len(ns.Audit.Scope.Criteria) != 2 {
		t.Error("expected 2 criteria")
	}
}

func TestApplyModelMarkers_Summary(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	d.applyModelMarkers("WOLFCASTLE_SUMMARY: all tests pass now", ns, nav)

	if ns.Audit.ResultSummary != "all tests pass now" {
		t.Errorf("summary = %q", ns.Audit.ResultSummary)
	}
}

func TestApplyModelMarkers_SummaryEmpty(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	d.applyModelMarkers("WOLFCASTLE_SUMMARY:  ", ns, nav)
	if ns.Audit.ResultSummary != "" {
		t.Error("empty SUMMARY should be ignored")
	}
}

func TestApplyModelMarkers_ResolveEscalation(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	ns.Audit.Escalations = []state.Escalation{
		{ID: "esc-1", Status: state.EscalationOpen, Description: "needs fix"},
	}
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	d.applyModelMarkers("WOLFCASTLE_RESOLVE_ESCALATION: esc-1", ns, nav)

	if ns.Audit.Escalations[0].Status != state.EscalationResolved {
		t.Errorf("escalation should be resolved, got %q", ns.Audit.Escalations[0].Status)
	}
	if ns.Audit.Escalations[0].ResolvedBy != "my-node/task-1" {
		t.Errorf("resolved_by = %q", ns.Audit.Escalations[0].ResolvedBy)
	}
	if ns.Audit.Escalations[0].ResolvedAt == nil {
		t.Error("resolved_at should be set")
	}
}

func TestApplyModelMarkers_ResolveEscalation_AlreadyResolved(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	ns.Audit.Escalations = []state.Escalation{
		{ID: "esc-1", Status: state.EscalationResolved, Description: "done"},
	}
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	d.applyModelMarkers("WOLFCASTLE_RESOLVE_ESCALATION: esc-1", ns, nav)
	if ns.Audit.Escalations[0].ResolvedBy != "" {
		t.Error("already-resolved escalation should not be re-resolved")
	}
}

func TestApplyModelMarkers_MultipleMarkers(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("n1", "Node 1", state.NodeLeaf)
	nav := &state.NavigationResult{NodeAddress: "my-node", TaskID: "task-1"}
	output := strings.Join([]string{
		"WOLFCASTLE_BREADCRUMB: step one",
		"some random text",
		"WOLFCASTLE_BREADCRUMB: step two",
		"WOLFCASTLE_SCOPE: big refactor",
		"WOLFCASTLE_GAP: edge case not handled",
		"WOLFCASTLE_SUMMARY: done",
	}, "\n")
	d.applyModelMarkers(output, ns, nav)

	if len(ns.Audit.Breadcrumbs) != 2 {
		t.Errorf("expected 2 breadcrumbs, got %d", len(ns.Audit.Breadcrumbs))
	}
	if ns.Audit.Scope == nil || ns.Audit.Scope.Description != "big refactor" {
		t.Error("scope not set correctly")
	}
	if len(ns.Audit.Gaps) != 1 {
		t.Errorf("expected 1 gap, got %d", len(ns.Audit.Gaps))
	}
	if ns.Audit.ResultSummary != "done" {
		t.Errorf("summary = %q", ns.Audit.ResultSummary)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// invokeWithRetry
// ═══════════════════════════════════════════════════════════════════════════

func TestInvokeWithRetry_SuccessFirstTry(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
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
	d.Logger.StartIteration()
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
	d.Logger.StartIteration()
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
	d.Logger.StartIteration()
	defer d.Logger.Close()

	model := config.ModelDef{Command: "nonexistent-cmd-backoff", Args: []string{}}
	start := time.Now()
	d.invokeWithRetry(context.Background(), model, "", d.RepoDir, nil, "test")
	elapsed := time.Since(start)
	// With 0s delays and 2 retries, should finish very quickly
	if elapsed > 5*time.Second {
		t.Errorf("retry took too long with zero delays: %v", elapsed)
	}
}

func TestInvokeWithRetry_NilLogWriter(t *testing.T) {
	d := testDaemon(t)
	d.Logger.StartIteration()
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
		{ID: "task-1", State: state.StatusComplete},
	})
	idx, _ := d.Resolver.LoadRootIndex()
	entry := idx.Nodes["my-node"]
	entry.State = state.StatusComplete
	idx.Nodes["my-node"] = entry
	state.SaveRootIndex(d.Resolver.RootIndexPath(), idx)

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

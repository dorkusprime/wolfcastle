package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

// ---------------------------------------------------------------------------
// nodeGlyph — all NodeStatus values (non-terminal mode, which is what tests get)
// ---------------------------------------------------------------------------

func TestNodeGlyph_AllStatuses(t *testing.T) {
	tests := []struct {
		status state.NodeStatus
		want   string
	}{
		{state.StatusComplete, "●"},
		{state.StatusInProgress, "◐"},
		{state.StatusBlocked, "☢"},
		{state.StatusNotStarted, "◯"},
		{"unknown_status", "◯"}, // default case
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := nodeGlyph(tt.status)
			if got != tt.want {
				t.Errorf("nodeGlyph(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// taskGlyph — all NodeStatus values
// ---------------------------------------------------------------------------

func TestTaskGlyph_AllStatuses(t *testing.T) {
	tests := []struct {
		status state.NodeStatus
		want   string
	}{
		{state.StatusComplete, "✓"},
		{state.StatusInProgress, "→"},
		{state.StatusBlocked, "✖"},
		{state.StatusNotStarted, "○"},
		{"other", "○"}, // default case
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := taskGlyph(tt.status)
			if got != tt.want {
				t.Errorf("taskGlyph(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// printNodeTree — coverage for task rendering edge cases
// ---------------------------------------------------------------------------

func TestPrintNodeTree_OrchestratorRecursion(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Tree",
		testutil.Orchestrator("tree",
			testutil.Leaf("leaf-a"),
			testutil.Leaf("leaf-b"),
		),
	)

	idx, err := env.App.State.ReadIndex()
	if err != nil {
		t.Fatalf("reading index: %v", err)
	}

	details := map[string]*nodeDetail{}
	for addr, entry := range idx.Nodes {
		nd := &nodeDetail{entry: entry}
		if entry.Type == state.NodeLeaf {
			ns, _ := env.App.State.ReadNode(addr)
			nd.ns = ns
		}
		details[addr] = nd
	}

	// Exercises recursive orchestrator → leaf rendering.
	printNodeTree(env.App, idx, details, "tree", "  ", treeOpts{Width: 120}, nil)
}

func TestPrintNodeTree_BlockedTaskWithReason(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	idx, _ := env.App.State.ReadIndex()
	ns, _ := env.App.State.ReadNode("proj")
	ns.Tasks = []state.Task{
		{
			ID:            "task-0001",
			Title:         "Blocked task",
			State:         state.StatusBlocked,
			BlockedReason: "waiting on upstream",
		},
	}
	testutil.SaveNode(t, env.WolfcastleDir, env.env.Namespace(), "proj", ns)

	details := map[string]*nodeDetail{
		"proj": {entry: idx.Nodes["proj"], ns: ns},
	}
	printNodeTree(env.App, idx, details, "proj", "  ", treeOpts{Width: 120}, nil)
}

func TestPrintNodeTree_TaskFailureCount(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	idx, _ := env.App.State.ReadIndex()
	ns, _ := env.App.State.ReadNode("proj")
	ns.Tasks = []state.Task{
		{
			ID:           "task-0001",
			Title:        "Flaky task",
			State:        state.StatusInProgress,
			FailureCount: 3,
		},
	}
	testutil.SaveNode(t, env.WolfcastleDir, env.env.Namespace(), "proj", ns)

	details := map[string]*nodeDetail{
		"proj": {entry: idx.Nodes["proj"], ns: ns},
	}
	printNodeTree(env.App, idx, details, "proj", "  ", treeOpts{Width: 120}, nil)
}

func TestPrintNodeTree_CompletedWithTitleAndDescription(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	idx, _ := env.App.State.ReadIndex()
	ns, _ := env.App.State.ReadNode("proj")
	ns.Tasks = []state.Task{
		{
			ID:          "task-0001",
			Title:       "Add caching",
			Description: "Implement LRU cache for hot keys",
			State:       state.StatusComplete,
		},
	}
	testutil.SaveNode(t, env.WolfcastleDir, env.env.Namespace(), "proj", ns)

	details := map[string]*nodeDetail{
		"proj": {entry: idx.Nodes["proj"], ns: ns},
	}
	printNodeTree(env.App, idx, details, "proj", "  ", treeOpts{Width: 120}, nil)
}

func TestPrintNodeTree_OpenGapRendering(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	idx, _ := env.App.State.ReadIndex()
	ns, _ := env.App.State.ReadNode("proj")
	ns.Audit.Gaps = []state.Gap{
		{ID: "gap-1", Description: "missing error handling", Status: state.GapOpen},
		{ID: "gap-2", Description: "already fixed", Status: "fixed"},
	}
	testutil.SaveNode(t, env.WolfcastleDir, env.env.Namespace(), "proj", ns)

	details := map[string]*nodeDetail{
		"proj": {entry: idx.Nodes["proj"], ns: ns},
	}
	// Exercises gap printing: open gap rendered, fixed gap skipped.
	printNodeTree(env.App, idx, details, "proj", "  ", treeOpts{Width: 120}, nil)
}

func TestPrintNodeTree_TaskDescriptionFallback(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	idx, _ := env.App.State.ReadIndex()
	ns, _ := env.App.State.ReadNode("proj")
	// Task with no Title, only Description — label falls back to Description.
	ns.Tasks = []state.Task{
		{
			ID:          "task-0001",
			Description: "A task with only a description",
			State:       state.StatusNotStarted,
		},
	}
	testutil.SaveNode(t, env.WolfcastleDir, env.env.Namespace(), "proj", ns)

	details := map[string]*nodeDetail{
		"proj": {entry: idx.Nodes["proj"], ns: ns},
	}
	printNodeTree(env.App, idx, details, "proj", "  ", treeOpts{Width: 120}, nil)
}

// ---------------------------------------------------------------------------
// showTreeStatus — JSON output path with audit counts, inbox rendering
// ---------------------------------------------------------------------------

func TestShowTreeStatus_JSONWithAuditData(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	idx, _ := env.App.State.ReadIndex()

	// Set up node with gaps and escalations.
	ns, _ := env.App.State.ReadNode("proj")
	ns.Audit.Status = state.AuditInProgress
	ns.Audit.Gaps = []state.Gap{
		{ID: "gap-1", Description: "open gap", Status: state.GapOpen},
	}
	ns.Audit.Escalations = []state.Escalation{
		{ID: "esc-1", Description: "open esc", Status: state.EscalationOpen},
	}
	testutil.SaveNode(t, env.WolfcastleDir, env.env.Namespace(), "proj", ns)

	env.App.JSON = true
	if err := showTreeStatus(env.App, idx, "", treeOpts{Width: 120}); err != nil {
		t.Fatalf("showTreeStatus JSON failed: %v", err)
	}
}

func TestShowTreeStatus_ScopeFiltering(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Multi",
		testutil.Orchestrator("multi",
			testutil.Leaf("alpha"),
			testutil.Leaf("beta"),
		),
	)

	idx, _ := env.App.State.ReadIndex()
	if err := showTreeStatus(env.App, idx, "multi/alpha", treeOpts{Width: 120}); err != nil {
		t.Fatalf("showTreeStatus scoped failed: %v", err)
	}
}

func TestShowTreeStatus_InboxRendering(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))
	idx, _ := env.App.State.ReadIndex()

	// Write inbox with new and filed items.
	inboxPath := filepath.Join(env.App.State.Dir(), "inbox.json")
	inbox := state.InboxFile{
		Items: []state.InboxItem{
			{Text: "new item", Status: "new"},
			{Text: "filed item", Status: "filed"},
			{Text: "another new", Status: "new"},
		},
	}
	data, _ := json.MarshalIndent(inbox, "", "  ")
	if err := os.WriteFile(inboxPath, data, 0644); err != nil {
		t.Fatalf("writing inbox: %v", err)
	}

	if err := showTreeStatus(env.App, idx, "", treeOpts{Width: 120}); err != nil {
		t.Fatalf("showTreeStatus with inbox failed: %v", err)
	}
}

func TestShowTreeStatus_NodeReadError(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	idx, _ := env.App.State.ReadIndex()

	// Remove node state so ReadNode fails.
	_ = os.Remove(filepath.Join(env.env.ProjectsDir(), "proj", "state.json"))

	// Should tolerate the read error and continue.
	if err := showTreeStatus(env.App, idx, "", treeOpts{Width: 120}); err != nil {
		t.Fatalf("showTreeStatus should tolerate read errors: %v", err)
	}
}

// ---------------------------------------------------------------------------
// showAllStatus — error and empty paths
// ---------------------------------------------------------------------------

func TestShowAllStatus_ProjectsDirRemoved(t *testing.T) {
	env := newTestEnv(t)
	_ = os.RemoveAll(filepath.Join(env.WolfcastleDir, "system", "projects"))

	err := showAllStatus(env.App)
	if err == nil {
		t.Error("expected error when projects dir is missing")
	}
}

func TestShowAllStatus_EmptyJSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true
	if err := showAllStatus(env.App); err != nil {
		t.Fatalf("showAllStatus JSON empty failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// watchStatus — context cancellation and error paths
// ---------------------------------------------------------------------------

func TestWatchStatus_ImmediateCancel(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := watchStatus(ctx, env.App, "", false, 0.1, treeOpts{Width: 120}); err != nil {
		t.Fatalf("watchStatus cancelled: %v", err)
	}
}

func TestWatchStatus_ShowAllWithCancel(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := watchStatus(ctx, env.App, "", true, 0.1, treeOpts{Width: 120}); err != nil {
		t.Fatalf("watchStatus showAll cancelled: %v", err)
	}
}

func TestWatchStatus_IntervalFloor(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Interval below the 0.1s floor should be clamped.
	if err := watchStatus(ctx, env.App, "", false, 0.01, treeOpts{Width: 120}); err != nil {
		t.Fatalf("watchStatus min interval: %v", err)
	}
}

func TestWatchStatus_TreeReadError(t *testing.T) {
	env := newTestEnv(t)
	_ = os.Remove(filepath.Join(env.env.ProjectsDir(), "state.json"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := watchStatus(ctx, env.App, "", false, 0.1, treeOpts{Width: 120}); err != nil {
		t.Fatalf("watchStatus read error: %v", err)
	}
}

func TestWatchStatus_ShowAllReadError(t *testing.T) {
	env := newTestEnv(t)
	_ = os.RemoveAll(filepath.Join(env.WolfcastleDir, "system", "projects"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := watchStatus(ctx, env.App, "", true, 0.1, treeOpts{Width: 120}); err != nil {
		t.Fatalf("watchStatus showAll error: %v", err)
	}
}

func TestWatchStatus_ScopedWithCancel(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := watchStatus(ctx, env.App, "proj", false, 0.1, treeOpts{Width: 120}); err != nil {
		t.Fatalf("watchStatus scoped cancelled: %v", err)
	}
}

// ---------------------------------------------------------------------------
// newStatusCmd — flag combinations
// ---------------------------------------------------------------------------

func TestStatusCmd_AllWithJSON(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true
	env.RootCmd.SetArgs([]string{"status", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status --all --json failed: %v", err)
	}
}

func TestStatusCmd_NodeScopeFlag(t *testing.T) {
	env := newTestEnv(t)
	env.env.WithProject("Multi",
		testutil.Orchestrator("multi",
			testutil.Leaf("alpha"),
			testutil.Leaf("beta"),
		),
	)
	env.RootCmd.SetArgs([]string{"status", "--node", "multi/alpha"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status --node failed: %v", err)
	}
}

func TestStatusCmd_AllBypassesIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil
	env.RootCmd.SetArgs([]string{"status", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status --all without identity should succeed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// printRecentLog
// ---------------------------------------------------------------------------

func TestPrintRecentLog_NoLogDir(t *testing.T) {
	// Should not panic when the log directory doesn't exist.
	printRecentLog("/nonexistent/path/logs", 2)
}

func TestPrintRecentLog_EmptyLogDir(t *testing.T) {
	dir := t.TempDir()
	printRecentLog(dir, 2)
}

func TestPrintRecentLog_WithValidSession(t *testing.T) {
	logDir := t.TempDir()

	// Write a minimal log session file with a stage_start and stage_complete.
	ts := time.Now().UTC()
	lines := []string{
		mustJSON(map[string]any{"type": "stage_start", "stage": "execute", "node": "proj/auth", "task": "task-0001", "timestamp": ts.Format(time.RFC3339Nano)}),
		mustJSON(map[string]any{"type": "stage_complete", "stage": "execute", "node": "proj/auth", "task": "task-0001", "exit_code": 0, "duration_ms": 5000, "timestamp": ts.Add(5 * time.Second).Format(time.RFC3339Nano)}),
	}

	logFile := filepath.Join(logDir, "0001-exec-20260405T12-00Z.jsonl")
	if err := os.WriteFile(logFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Capture output by calling printRecentLog. It writes to output.PrintHuman
	// which goes to stdout; we just verify it doesn't panic and exercises the
	// full code path.
	printRecentLog(logDir, 2)
}

func TestPrintRecentLog_MoreThanFiveFiles(t *testing.T) {
	logDir := t.TempDir()

	// Create 8 log files. printRecentLog should only read the last 5.
	for i := 1; i <= 8; i++ {
		ts := time.Now().UTC()
		line := mustJSON(map[string]any{
			"type": "stage_complete", "stage": "execute",
			"node": fmt.Sprintf("proj/task-%d", i), "task": "task-0001",
			"exit_code": 0, "duration_ms": 1000,
			"timestamp": ts.Format(time.RFC3339Nano),
		})
		name := fmt.Sprintf("%04d-exec-20260405T12-%02dZ.jsonl", i, i)
		if err := os.WriteFile(filepath.Join(logDir, name), []byte(line+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	printRecentLog(logDir, 2)
}

func TestPrintRecentLog_RenderedEmpty(t *testing.T) {
	logDir := t.TempDir()

	// Write a log file with only assistant-type records (not rendered by summary).
	line := mustJSON(map[string]any{"type": "assistant", "text": "hello world", "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
	logFile := filepath.Join(logDir, "0001-exec-20260405T12-00Z.jsonl")
	if err := os.WriteFile(logFile, []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	printRecentLog(logDir, 2)
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// ---------------------------------------------------------------------------
// showTreeStatus JSON — daemon activity fields
// ---------------------------------------------------------------------------

func TestShowTreeStatus_JSONWithDaemonActivity(t *testing.T) {
	env := newStatusTestEnv(t)
	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	// Write a daemon-activity.json file.
	activityDir := filepath.Join(env.WolfcastleDir, "system")
	if err := os.MkdirAll(activityDir, 0o755); err != nil {
		t.Fatal(err)
	}
	activity := map[string]any{
		"last_activity_at": "2026-04-05T12:00:00Z",
		"iteration":        42,
		"current_node":     "proj/auth",
		"current_task":     "task-0001",
	}
	data, _ := json.Marshal(activity)
	if err := os.WriteFile(filepath.Join(activityDir, "daemon-activity.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	err := showTreeStatus(env.App, idx, "", treeOpts{Width: 120})
	if err != nil {
		t.Fatalf("showTreeStatus JSON with activity failed: %v", err)
	}
}

func TestShowTreeStatus_JSONWithDaemonActivity_PartialFields(t *testing.T) {
	env := newStatusTestEnv(t)
	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	// Write activity with empty node/task (daemon idle).
	activityDir := filepath.Join(env.WolfcastleDir, "system")
	if err := os.MkdirAll(activityDir, 0o755); err != nil {
		t.Fatal(err)
	}
	activity := map[string]any{
		"last_activity_at": "2026-04-05T12:00:00Z",
		"iteration":        0,
	}
	data, _ := json.Marshal(activity)
	if err := os.WriteFile(filepath.Join(activityDir, "daemon-activity.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	err := showTreeStatus(env.App, idx, "", treeOpts{Width: 120})
	if err != nil {
		t.Fatalf("showTreeStatus JSON with partial activity failed: %v", err)
	}
}

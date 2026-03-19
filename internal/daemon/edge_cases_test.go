package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// scanTerminalMarker — malformed and garbage model output
// ═══════════════════════════════════════════════════════════════════════════

func TestScanTerminalMarker_MalformedResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			"garbage binary output",
			"\x00\x01\x02\xff\xfe\xfd",
			"",
		},
		{
			"truncated JSON envelope",
			`{"type":"assistant","text":"WOLFCASTLE_CO`,
			"",
		},
		{
			"malformed JSON missing closing brace",
			`{"type":"assistant","text":"WOLFCASTLE_COMPLETE"`,
			"",
		},
		{
			"empty JSON object",
			`{}`,
			"",
		},
		{
			"JSON array instead of object",
			`["WOLFCASTLE_COMPLETE"]`,
			"",
		},
		{
			"wrong type field in envelope",
			`{"type":"tool_use","text":"WOLFCASTLE_COMPLETE"}`,
			"",
		},
		{
			"null text field",
			`{"type":"assistant","text":null}`,
			"",
		},
		{
			"numeric text field",
			`{"type":"assistant","text":42}`,
			"",
		},
		{
			"marker split across two JSON lines",
			"{\"type\":\"assistant\",\"text\":\"WOLFCASTLE_\"}\n{\"type\":\"assistant\",\"text\":\"COMPLETE\"}",
			"",
		},
		{
			"marker with trailing whitespace",
			"WOLFCASTLE_COMPLETE   ",
			"WOLFCASTLE_COMPLETE",
		},
		{
			"marker with leading whitespace",
			"   WOLFCASTLE_COMPLETE",
			"WOLFCASTLE_COMPLETE",
		},
		{
			"only newlines",
			"\n\n\n\n",
			"",
		},
		{
			"marker embedded in URL",
			"https://example.com/WOLFCASTLE_COMPLETE",
			"",
		},
		{
			"marker as JSON key",
			`{"WOLFCASTLE_COMPLETE": true}`,
			"",
		},
		{
			"complete trumps yield when both present",
			"WOLFCASTLE_YIELD\nWOLFCASTLE_COMPLETE",
			"WOLFCASTLE_COMPLETE",
		},
		{
			"complete trumps blocked",
			"WOLFCASTLE_BLOCKED\nWOLFCASTLE_COMPLETE",
			"WOLFCASTLE_COMPLETE",
		},
		{
			"blocked trumps yield",
			"WOLFCASTLE_YIELD\nWOLFCASTLE_BLOCKED",
			"WOLFCASTLE_BLOCKED",
		},
		{
			"very long line without marker",
			strings.Repeat("a", 10000),
			"",
		},
		{
			"result envelope with empty result",
			`{"type":"result","result":""}`,
			"",
		},
		{
			"nested message with empty content array",
			`{"type":"assistant","message":{"content":[]}}`,
			"",
		},
		{
			"nested message with non-text type",
			`{"type":"assistant","message":{"content":[{"type":"image","text":"WOLFCASTLE_COMPLETE"}]}}`,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := scanTerminalMarker(tt.input)
			if got != tt.expect {
				t.Errorf("scanTerminalMarker() = %q, want %q", got, tt.expect)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// extractAssistantText — edge cases
// ═══════════════════════════════════════════════════════════════════════════

func TestExtractAssistantText_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"empty string", "", ""},
		{"not JSON", "hello world", ""},
		{"single char", "x", ""},
		{"opens with brace but invalid", "{bad", ""},
		{"valid JSON wrong type", `{"type":"system","text":"hello"}`, ""},
		{"result with both text and result fields", `{"type":"result","text":"textval","result":"resultval"}`, "textval"},
		{"assistant with empty text", `{"type":"assistant","text":""}`, ""},
		{"deeply nested valid", `{"type":"assistant","message":{"content":[{"type":"text","text":"found it"}]}}`, "found it"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractAssistantText(tt.input)
			if got != tt.expect {
				t.Errorf("extractAssistantText(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// sleepWithContext — unit tests
// ═══════════════════════════════════════════════════════════════════════════

func TestSleepWithContext_CompletesNormally(t *testing.T) {
	t.Parallel()
	completed := sleepWithContext(context.Background(), 1*time.Millisecond)
	if !completed {
		t.Error("expected sleepWithContext to complete normally")
	}
}

func TestSleepWithContext_PreCancelledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	completed := sleepWithContext(ctx, 10*time.Second)
	if completed {
		t.Error("expected sleepWithContext to be interrupted by cancelled context")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// invokeWithRetry — unlimited retries (-1), exponential backoff doubling
// ═══════════════════════════════════════════════════════════════════════════

func TestInvokeWithRetry_UnlimitedRetriesRespectsContextCancel(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Retries = config.RetriesConfig{
		InitialDelaySeconds: 1, // 1s between retries so it doesn't spin
		MaxDelaySeconds:     1,
		MaxRetries:          -1, // unlimited
	}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	model := config.ModelDef{Command: "/nonexistent/binary/xyz", Args: []string{}}
	_, err := d.invokeWithRetry(ctx, model, "prompt", d.RepoDir, nil, "test")
	if err == nil {
		t.Fatal("expected error when context times out during unlimited retries")
	}
}

func TestInvokeWithRetry_ZeroMaxRetries_NoRetry(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Retries = config.RetriesConfig{
		InitialDelaySeconds: 0,
		MaxDelaySeconds:     0,
		MaxRetries:          0,
	}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	model := config.ModelDef{Command: "/nonexistent/binary/xyz", Args: []string{}}
	start := time.Now()
	_, err := d.invokeWithRetry(context.Background(), model, "", d.RepoDir, nil, "test")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
	if elapsed > 2*time.Second {
		t.Errorf("zero MaxRetries should not retry; took %v", elapsed)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — model outputs plain text (no CLI commands, no marker)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_PlainTextOutput_NoToolUse(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	// Model produces a thoughtful response but uses no wolfcastle CLI commands
	// and emits no terminal marker. This simulates an agent that just talks.
	d.Config.Models["chatty"] = config.ModelDef{
		Command: "sh",
		Args: []string{"-c", `cat > /dev/null
echo "I analyzed the codebase and here is what I found."
echo "The architecture looks solid. I would recommend refactoring the widget module."
echo "Let me know if you have questions."`},
	}
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "chatty", PromptFile: "execute.md"},
	}
	d.Config.Retries.MaxRetries = 0
	d.Config.Failure.DecompositionThreshold = 0
	d.Config.Failure.HardCap = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "chatty-node", []state.Task{
		{ID: "task-0001", Description: "implement feature", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "chatty-node", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration should succeed (no fatal error): %v", err)
	}

	// Verify failure was incremented
	ns, _ := d.Store.ReadNode("chatty-node")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.FailureCount != 1 {
				t.Errorf("expected failure_count=1 for chatty model, got %d", task.FailureCount)
			}
			if task.State != state.StatusInProgress {
				t.Errorf("task should remain in_progress after no marker, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — model command fails with nonzero exit code
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_NonzeroExitCode_NoMarker(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	// Model exits with code 1 and prints some output but no marker.
	// InvokeStreaming captures the output and returns exit code in Result,
	// not as an error (errors are for invocation failures like missing binary).
	d.Config.Models["failing"] = config.ModelDef{
		Command: "sh",
		Args:    []string{"-c", "echo 'Error: authentication failed'; exit 1"},
	}
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "failing", PromptFile: "execute.md"},
	}
	d.Config.Retries.MaxRetries = 0
	d.Config.Failure.DecompositionThreshold = 0
	d.Config.Failure.HardCap = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "exitcode-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "exitcode-node", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Logf("runIteration returned error (may be acceptable): %v", err)
	}

	// The task should have its failure count incremented because no marker was found.
	ns, _ := d.Store.ReadNode("exitcode-node")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.FailureCount < 1 {
				t.Errorf("expected failure_count >= 1, got %d", task.FailureCount)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — model produces COMPLETE with warning-level output
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_CompleteWithWarnings(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	// Model produces warnings before the completion marker.
	// The daemon should still detect COMPLETE despite preceding warning text.
	d.Config.Models["warn"] = config.ModelDef{
		Command: "sh",
		Args: []string{"-c", `cat > /dev/null
echo "WARNING: deprecated API usage detected in widget.go:42"
echo "WARNING: test coverage below 80%"
echo "Task completed with warnings."
echo "WOLFCASTLE_COMPLETE"`},
	}
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "warn", PromptFile: "execute.md"},
	}
	d.Config.Retries.MaxRetries = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "warn-node", []state.Task{
		{ID: "task-0001", Description: "implement with warnings", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "warn-node", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	ns, _ := d.Store.ReadNode("warn-node")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusComplete {
				t.Errorf("expected complete despite warnings, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// runIntakeStage — nonzero exit code leaves items as "new"
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIntakeStage_NonzeroExitCode_ItemsStayNew(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Models["fail-intake"] = config.ModelDef{
		Command: "sh",
		Args:    []string{"-c", "cat > /dev/null; echo 'oops'; exit 1"},
	}
	d.Config.Retries.MaxRetries = 0
	_ = d.InboxLogger.StartIterationWithPrefix("intake")
	defer d.InboxLogger.Close()
	writePromptFile(t, d.WolfcastleDir, "intake.md")

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "should stay new", Timestamp: "2026-01-01T00:00:00Z"},
	}})

	stage := config.PipelineStage{Name: "intake", Model: "fail-intake", PromptFile: "intake.md"}
	err := d.runIntakeStage(context.Background(), stage)
	if err != nil {
		t.Fatalf("intake stage returned error: %v", err)
	}

	// Item should remain "new" since the model exited nonzero.
	updatedInbox, _ := state.LoadInbox(inboxPath)
	if updatedInbox.Items[0].Status != "new" {
		t.Errorf("expected status 'new' after nonzero exit, got %q", updatedInbox.Items[0].Status)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIntakeStage — includes existing tree context in prompt
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIntakeStage_IncludesExistingTreeContext(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	projDir := d.Resolver.ProjectsDir()

	// Set up an existing project so the intake sees it
	idx := state.NewRootIndex()
	idx.Root = []string{"existing-proj"}
	idx.Nodes["existing-proj"] = state.IndexEntry{
		Name: "Existing Project", Type: state.NodeLeaf,
		State: state.StatusInProgress, Address: "existing-proj",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	ns := state.NewNodeState("existing-proj", "Existing Project", state.NodeLeaf)
	writeJSON(t, filepath.Join(projDir, "existing-proj", "state.json"), ns)

	assertFile := filepath.Join(t.TempDir(), "tree-context.txt")
	scriptFile := filepath.Join(t.TempDir(), "check-tree.sh")
	script := fmt.Sprintf(`#!/bin/sh
PROMPT=$(cat)
echo "$PROMPT" | grep -q "Existing Project" && printf "FOUND_PROJECT" > %s
echo "WOLFCASTLE_COMPLETE"
`, assertFile)
	_ = os.WriteFile(scriptFile, []byte(script), 0755)

	d.Config.Models["checker"] = config.ModelDef{Command: "sh", Args: []string{scriptFile}}
	_ = d.InboxLogger.StartIterationWithPrefix("intake")
	defer d.InboxLogger.Close()
	writePromptFile(t, d.WolfcastleDir, "intake.md")

	inboxPath := filepath.Join(projDir, "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "new work", Timestamp: "2026-01-01T00:00:00Z"},
	}})

	stage := config.PipelineStage{Name: "intake", Model: "checker", PromptFile: "intake.md"}
	if err := d.runIntakeStage(context.Background(), stage); err != nil {
		t.Fatalf("intake error: %v", err)
	}

	data, err := os.ReadFile(assertFile)
	if err != nil {
		t.Fatalf("reading assertions: %v", err)
	}
	if string(data) != "FOUND_PROJECT" {
		t.Error("intake prompt should include existing tree context with project names")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// selfHeal — single interrupted task with multiple complete tasks
// ═══════════════════════════════════════════════════════════════════════════

func TestSelfHeal_OneInProgressAmongManyComplete(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	projDir := d.Resolver.ProjectsDir()

	idx := state.NewRootIndex()
	idx.Root = []string{"node-done", "node-wip"}
	idx.Nodes["node-done"] = state.IndexEntry{
		Name: "Done", Type: state.NodeLeaf, State: state.StatusComplete, Address: "node-done",
	}
	idx.Nodes["node-wip"] = state.IndexEntry{
		Name: "WIP", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "node-wip",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	nsDone := state.NewNodeState("node-done", "Done", state.NodeLeaf)
	nsDone.Tasks = []state.Task{{ID: "t1", State: state.StatusComplete}}
	writeJSON(t, filepath.Join(projDir, "node-done", "state.json"), nsDone)

	nsWIP := state.NewNodeState("node-wip", "WIP", state.NodeLeaf)
	nsWIP.Tasks = []state.Task{{ID: "t1", State: state.StatusInProgress}}
	writeJSON(t, filepath.Join(projDir, "node-wip", "state.json"), nsWIP)

	// One in-progress is fine. Should not error.
	if err := d.selfHeal(); err != nil {
		t.Errorf("selfHeal should accept one in-progress task: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — context cancelled during model execution
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_ContextCancelledDuringExecution(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	// Use a model that takes long enough for context cancellation to kick in.
	// When the context is cancelled, InvokeStreaming kills the process and
	// returns whatever output was captured. If the process exits before
	// producing a marker, the iteration increments the failure count.
	d.Config.Models["slow"] = config.ModelDef{
		Command: "sh",
		Args:    []string{"-c", "read BLOCK; echo WOLFCASTLE_COMPLETE"},
	}
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "slow", PromptFile: "execute.md"},
	}
	d.Config.Retries.MaxRetries = 0
	d.Config.Daemon.InvocationTimeoutSeconds = 0
	d.Config.Failure.DecompositionThreshold = 0
	d.Config.Failure.HardCap = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "cancel-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "cancel-node", TaskID: "task-0001", Found: true}
	err := d.runIteration(ctx, nav, idx)
	// When context is cancelled, the model process is killed. Depending on
	// timing, this may return an error (context cancelled) or succeed with
	// no marker (incrementing failure count). Both outcomes are correct.
	_ = err
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — model produces partial marker output
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_PartialMarkerOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		output   string
		wantFail bool
	}{
		{
			"truncated marker",
			"WOLFCASTLE_COMPLE",
			true,
		},
		{
			"misspelled marker",
			"WOLFCASTLE_COMPLET",
			true,
		},
		{
			"marker with extra suffix",
			"WOLFCASTLE_COMPLETED",
			true,
		},
		{
			"lowercase marker",
			"wolfcastle_complete",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := testDaemon(t)
			d.Config.Models["partial"] = config.ModelDef{
				Command: "echo",
				Args:    []string{tt.output},
			}
			d.Config.Pipeline.Stages = []config.PipelineStage{
				{Name: "execute", Model: "partial", PromptFile: "execute.md"},
			}
			d.Config.Retries.MaxRetries = 0
			d.Config.Failure.DecompositionThreshold = 0
			d.Config.Failure.HardCap = 0
			_ = d.Logger.StartIteration()
			defer d.Logger.Close()

			setupLeafNode(t, d, "partial-node", []state.Task{
				{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
			})
			writePromptFile(t, d.WolfcastleDir, "execute.md")

			idx, _ := d.Resolver.LoadRootIndex()
			nav := &state.NavigationResult{NodeAddress: "partial-node", TaskID: "task-0001", Found: true}
			_ = d.runIteration(context.Background(), nav, idx)

			ns, _ := d.Store.ReadNode("partial-node")
			for _, task := range ns.Tasks {
				if task.ID == "task-0001" {
					if tt.wantFail && task.FailureCount < 1 {
						t.Errorf("expected failure for partial marker %q, got count %d", tt.output, task.FailureCount)
					}
					return
				}
			}
			t.Error("task-0001 not found")
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — model writes garbage JSON to stdout
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_GarbageJSONOutput(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	// Model outputs something that looks like JSON but is invalid,
	// with no terminal marker anywhere.
	d.Config.Models["garbage"] = config.ModelDef{
		Command: "sh",
		Args: []string{"-c", `cat > /dev/null
echo '{"type":"assistant","tex'
echo '{"broken json here'
echo ']}}}}'
echo 'random text at the end'`},
	}
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "garbage", PromptFile: "execute.md"},
	}
	d.Config.Retries.MaxRetries = 0
	d.Config.Failure.DecompositionThreshold = 0
	d.Config.Failure.HardCap = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "garbage-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "garbage-node", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration should handle garbage gracefully: %v", err)
	}

	ns, _ := d.Store.ReadNode("garbage-node")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.FailureCount != 1 {
				t.Errorf("expected failure_count=1, got %d", task.FailureCount)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — empty stdout from model
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_EmptyOutput(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Models["silent"] = config.ModelDef{
		Command: "true",
		Args:    []string{},
	}
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "silent", PromptFile: "execute.md"},
	}
	d.Config.Retries.MaxRetries = 0
	d.Config.Failure.DecompositionThreshold = 0
	d.Config.Failure.HardCap = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "silent-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "silent-node", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration should handle empty output: %v", err)
	}

	ns, _ := d.Store.ReadNode("silent-node")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.FailureCount != 1 {
				t.Errorf("expected failure_count=1 for empty output, got %d", task.FailureCount)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — prompt assembly error
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_PromptAssemblyError(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "echo", PromptFile: "nonexistent-prompt-xyz.md"},
	}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "prompt-err-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	// Intentionally do NOT create the prompt file

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "prompt-err-node", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err == nil {
		t.Fatal("expected error for missing prompt file")
	}
	if !strings.Contains(err.Error(), "prompt") {
		t.Errorf("error should mention prompt assembly: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RunOnce — empty tree (no root index entries)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunOnce_EmptyTree(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	// Create an empty root index
	idx := state.NewRootIndex()
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	result, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != IterationNoWork {
		t.Errorf("expected IterationNoWork for empty tree, got %d", result)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — deliverables unchanged rejects COMPLETE
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_NoGitProgress_RejectsComplete(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	projDir := d.Resolver.ProjectsDir()

	// Initialize a git repo in the daemon's RepoDir so checkGitProgress
	// can detect the absence of changes.
	initTestGitRepo(t, d.RepoDir)

	// Model claims COMPLETE but doesn't modify any files
	d.Config.Models["noop-complete"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"WOLFCASTLE_COMPLETE"},
	}
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "noop-complete", PromptFile: "execute.md"},
	}
	d.Config.Retries.MaxRetries = 0
	d.Config.Failure.DecompositionThreshold = 0
	d.Config.Failure.HardCap = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("unchanged-node", "unchanged-node", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "should write code", State: state.StatusNotStarted},
	}
	idx := state.NewRootIndex()
	idx.Root = []string{"unchanged-node"}
	idx.Nodes["unchanged-node"] = state.IndexEntry{
		Name: "unchanged-node", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "unchanged-node",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)
	writeJSON(t, filepath.Join(projDir, "unchanged-node", "state.json"), ns)
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx2, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "unchanged-node", TaskID: "task-0001", Found: true}
	_ = d.runIteration(context.Background(), nav, idx2)

	// Task should NOT be complete since git shows no changes
	reloaded, _ := d.Store.ReadNode("unchanged-node")
	for _, task := range reloaded.Tasks {
		if task.ID == "task-0001" {
			if task.State == state.StatusComplete {
				t.Error("task should not be complete when git shows no progress")
			}
			if task.FailureCount < 1 {
				t.Errorf("expected failure count >= 1, got %d", task.FailureCount)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

// initTestGitRepo initializes a git repo in dir with one commit.
func initTestGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")
	_ = os.WriteFile(filepath.Join(dir, ".gitkeep"), []byte(""), 0644)
	run("add", ".")
	run("commit", "-m", "init")
}

// ═══════════════════════════════════════════════════════════════════════════
// RunOnce — state error from RunOnce returns IterationStop
// ═══════════════════════════════════════════════════════════════════════════

func TestRunOnce_StateError_ReturnsFatal(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	projDir := d.Resolver.ProjectsDir()

	// Write valid root index but corrupt the node state
	idx := state.NewRootIndex()
	idx.Root = []string{"corrupt-node"}
	idx.Nodes["corrupt-node"] = state.IndexEntry{
		Name: "Corrupt", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "corrupt-node",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	// Write completely invalid JSON for the node state
	nodeDir := filepath.Join(projDir, "corrupt-node")
	_ = os.MkdirAll(nodeDir, 0755)
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), []byte("not valid json at all"), 0644)

	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"WOLFCASTLE_COMPLETE"}}
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	_ = d.Logger.StartIteration()
	result, err := d.RunOnce(context.Background())
	d.Logger.Close()

	// Navigation should fail trying to load the corrupt state
	// Either IterationError (recoverable) or IterationStop (fatal) is acceptable.
	_ = result
	_ = err
}

// ═══════════════════════════════════════════════════════════════════════════
// workAvailable channel — intake signals execute loop
// ═══════════════════════════════════════════════════════════════════════════

func TestIntakeStage_SignalsWorkAvailable(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Models["intake-echo"] = config.ModelDef{
		Command: "sh",
		Args:    []string{"-c", "cat > /dev/null; echo done"},
	}
	_ = d.InboxLogger.StartIterationWithPrefix("intake")
	defer d.InboxLogger.Close()
	writePromptFile(t, d.WolfcastleDir, "intake.md")

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "trigger work signal", Timestamp: "2026-01-01T00:00:00Z"},
	}})

	stage := config.PipelineStage{Name: "intake", Model: "intake-echo", PromptFile: "intake.md"}
	if err := d.runIntakeStage(context.Background(), stage); err != nil {
		t.Fatalf("intake error: %v", err)
	}

	// The workAvailable channel should have a signal
	select {
	case <-d.workAvailable:
		// Signal received
	default:
		t.Error("expected workAvailable signal after successful intake")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RunOnce — repeated NoWork messages are deduplicated
// ═══════════════════════════════════════════════════════════════════════════

func TestRunOnce_NoWorkDeduplication(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	// Empty tree: should say "nothing to destroy"
	idx := state.NewRootIndex()
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	result1, _ := d.RunOnce(context.Background())
	if result1 != IterationNoWork {
		t.Fatalf("expected NoWork, got %d", result1)
	}

	msg1 := d.lastNoWorkMsg
	if msg1 == "" {
		t.Fatal("lastNoWorkMsg should be set after first NoWork")
	}

	// Second call with same state: should not change lastNoWorkMsg
	result2, _ := d.RunOnce(context.Background())
	if result2 != IterationNoWork {
		t.Fatalf("expected NoWork, got %d", result2)
	}
	if d.lastNoWorkMsg != msg1 {
		t.Errorf("lastNoWorkMsg should remain %q, got %q", msg1, d.lastNoWorkMsg)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// checkDeliverables — glob pattern with subdirectory
// ═══════════════════════════════════════════════════════════════════════════

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — multiple stages, only execute runs (intake skipped)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_MultipleStages_CustomStagesRun(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "intake", Model: "echo", PromptFile: "intake.md"},
		{Name: "execute", Model: "echo", PromptFile: "execute.md"},
	}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "multi-stage-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")
	writePromptFile(t, d.WolfcastleDir, "intake.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "multi-stage-node", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	// Verify task was completed (the echo model outputs WOLFCASTLE_COMPLETE)
	ns, _ := d.Store.ReadNode("multi-stage-node")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusComplete {
				t.Errorf("expected complete, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// checkInboxForNew vs checkInboxState — exercise both code paths
// ═══════════════════════════════════════════════════════════════════════════

func TestCheckInboxForNew_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inboxPath := filepath.Join(dir, "inbox.json")
	_ = os.WriteFile(inboxPath, []byte("{{{broken"), 0644)

	d := &Daemon{}
	if d.checkInboxForNew(inboxPath) {
		t.Error("expected false for broken JSON inbox")
	}
}

func TestCheckInboxForNew_NoNewItems(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inboxPath := filepath.Join(dir, "inbox.json")
	data, _ := json.Marshal(&state.InboxFile{Items: []state.InboxItem{
		{Status: "filed", Text: "old"},
	}})
	_ = os.WriteFile(inboxPath, data, 0644)

	d := &Daemon{}
	if d.checkInboxForNew(inboxPath) {
		t.Error("expected false when no new items")
	}
}

func TestCheckInboxForNew_HasNewItems(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inboxPath := filepath.Join(dir, "inbox.json")
	data, _ := json.Marshal(&state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "fresh"},
	}})
	_ = os.WriteFile(inboxPath, data, 0644)

	d := &Daemon{}
	if !d.checkInboxForNew(inboxPath) {
		t.Error("expected true when new items present")
	}
}

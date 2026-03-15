package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// 1. Mock model that validates prompt content
// ═══════════════════════════════════════════════════════════════════════════

// TestIntegration_MockModelValidatesPrompt uses a shell script mock that reads
// the prompt from stdin, checks it for expected content (task description,
// node address), and writes findings to a temp file for test assertions.
func TestIntegration_MockModelValidatesPrompt(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false

	setupLeafNode(t, d, "prompt-node", []state.Task{
		{ID: "task-0001", Description: "implement the frobulator", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	})

	assertFile := filepath.Join(t.TempDir(), "assertions.txt")
	scriptFile := filepath.Join(t.TempDir(), "validate.sh")

	// The shell script reads stdin (the prompt), checks for expected strings,
	// and writes results to the assertions file. Then emits WOLFCASTLE_COMPLETE.
	script := fmt.Sprintf(`#!/bin/sh
PROMPT=$(cat)
RESULTS=""
echo "$PROMPT" | grep -q "prompt-node" && RESULTS="${RESULTS}HAS_NODE_ADDR\n"
echo "$PROMPT" | grep -q "implement the frobulator" && RESULTS="${RESULTS}HAS_TASK_DESC\n"
echo "$PROMPT" | grep -q "task-0001" && RESULTS="${RESULTS}HAS_TASK_ID\n"
printf "%%b" "$RESULTS" > %s
echo "WOLFCASTLE_COMPLETE"
`, assertFile)

	if err := os.WriteFile(scriptFile, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	d.Config.Models["echo"] = config.ModelDef{
		Command: "sh",
		Args:    []string{scriptFile},
	}
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	_ = d.Logger.StartIteration()
	result, err := d.RunOnce(context.Background())
	d.Logger.Close()
	if err != nil {
		t.Fatalf("iteration error: %v", err)
	}
	if result != IterationDidWork {
		t.Fatalf("expected DidWork, got %v", result)
	}

	data, err := os.ReadFile(assertFile)
	if err != nil {
		t.Fatalf("reading assertion file: %v", err)
	}
	assertions := string(data)

	if !strings.Contains(assertions, "HAS_NODE_ADDR") {
		t.Error("prompt did not contain the node address")
	}
	if !strings.Contains(assertions, "HAS_TASK_DESC") {
		t.Error("prompt did not contain the task description")
	}
	if !strings.Contains(assertions, "HAS_TASK_ID") {
		t.Error("prompt did not contain the task ID")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 2. JSON stream envelope format
// ═══════════════════════════════════════════════════════════════════════════

// TestIntegration_JSONStreamEnvelope verifies that terminal markers wrapped in
// Claude Code's stream-json envelope format are correctly detected. The mock
// model emits JSON lines with type "assistant" and "result" envelopes.
func TestIntegration_JSONStreamEnvelope(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false

	setupLeafNode(t, d, "json-node", []state.Task{
		{ID: "task-0001", Description: "json test", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	})

	// Emit Claude Code stream-json format with the marker inside a result envelope
	d.Config.Models["echo"] = config.ModelDef{
		Command: "sh",
		Args: []string{"-c", `cat <<'SCRIPT'
{"type":"assistant","text":"doing work..."}
{"type":"result","result":"Done.\n\nWOLFCASTLE_COMPLETE"}
SCRIPT`},
	}
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	_ = d.Logger.StartIteration()
	result, err := d.RunOnce(context.Background())
	d.Logger.Close()
	if err != nil {
		t.Fatalf("iteration error: %v", err)
	}
	if result != IterationDidWork {
		t.Fatalf("expected DidWork, got %v", result)
	}

	projDir := d.Resolver.ProjectsDir()
	ns, err := state.LoadNodeState(filepath.Join(projDir, "json-node", "state.json"))
	if err != nil {
		t.Fatalf("loading node state: %v", err)
	}
	if ns.Tasks[0].State != state.StatusComplete {
		t.Errorf("expected task complete via JSON envelope, got %s", ns.Tasks[0].State)
	}
}

// TestIntegration_JSONStreamYield verifies YIELD detection through a JSON envelope.
func TestIntegration_JSONStreamYield(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false

	setupLeafNode(t, d, "yield-json-node", []state.Task{
		{ID: "task-0001", Description: "yield test", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	})

	d.Config.Models["echo"] = config.ModelDef{
		Command: "sh",
		Args: []string{"-c", `cat <<'SCRIPT'
{"type":"assistant","text":"making progress..."}
{"type":"result","result":"Pausing here.\n\nWOLFCASTLE_YIELD"}
SCRIPT`},
	}
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	_ = d.Logger.StartIteration()
	result, err := d.RunOnce(context.Background())
	d.Logger.Close()
	if err != nil {
		t.Fatalf("iteration error: %v", err)
	}
	if result != IterationDidWork {
		t.Fatalf("expected DidWork, got %v", result)
	}

	projDir := d.Resolver.ProjectsDir()
	ns, err := state.LoadNodeState(filepath.Join(projDir, "yield-json-node", "state.json"))
	if err != nil {
		t.Fatalf("loading node state: %v", err)
	}
	// YIELD keeps the task in_progress
	if ns.Tasks[0].State != state.StatusInProgress {
		t.Errorf("expected task in_progress after YIELD, got %s", ns.Tasks[0].State)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 3. Three-tier prompt overrides
// ═══════════════════════════════════════════════════════════════════════════

// TestIntegration_ThreeTierPromptOverride verifies that local/ overrides
// custom/ overrides base/ for prompt resolution. A shell script mock checks
// which marker the prompt contains and records it to an assertion file.
func TestIntegration_ThreeTierPromptOverride(t *testing.T) {
	tests := []struct {
		name     string
		tiers    []string // which tiers to populate (base, custom, local)
		expected string   // which marker the mock should see
	}{
		{"base only", []string{"base"}, "BASE_MARKER"},
		{"custom overrides base", []string{"base", "custom"}, "CUSTOM_MARKER"},
		{"local overrides all", []string{"base", "custom", "local"}, "LOCAL_MARKER"},
		{"local overrides base (no custom)", []string{"base", "local"}, "LOCAL_MARKER"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testDaemon(t)
			d.Config.Git.VerifyBranch = false

			setupLeafNode(t, d, "tier-node", []state.Task{
				{ID: "task-0001", Description: "tier test", State: state.StatusNotStarted},
				{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
			})

			markers := map[string]string{
				"base":   "BASE_MARKER",
				"custom": "CUSTOM_MARKER",
				"local":  "LOCAL_MARKER",
			}

			for _, tier := range tt.tiers {
				dir := filepath.Join(d.WolfcastleDir, tier, "prompts")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				content := fmt.Sprintf("prompt with %s", markers[tier])
				if err := os.WriteFile(filepath.Join(dir, "execute.md"), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			assertFile := filepath.Join(t.TempDir(), "tier-assertions.txt")
			scriptFile := filepath.Join(t.TempDir(), "check-tier.sh")

			script := fmt.Sprintf(`#!/bin/sh
PROMPT=$(cat)
FOUND=""
echo "$PROMPT" | grep -q "LOCAL_MARKER" && FOUND="LOCAL_MARKER"
if [ -z "$FOUND" ]; then echo "$PROMPT" | grep -q "CUSTOM_MARKER" && FOUND="CUSTOM_MARKER"; fi
if [ -z "$FOUND" ]; then echo "$PROMPT" | grep -q "BASE_MARKER" && FOUND="BASE_MARKER"; fi
printf "%%s" "$FOUND" > "%s"
echo "WOLFCASTLE_COMPLETE"
`, assertFile)

			if err := os.WriteFile(scriptFile, []byte(script), 0755); err != nil {
				t.Fatal(err)
			}

			d.Config.Models["echo"] = config.ModelDef{
				Command: "sh",
				Args:    []string{scriptFile},
			}

			_ = d.Logger.StartIteration()
			result, err := d.RunOnce(context.Background())
			d.Logger.Close()
			if err != nil {
				t.Fatalf("iteration error: %v", err)
			}
			if result != IterationDidWork {
				t.Fatalf("expected DidWork, got %v", result)
			}

			data, err := os.ReadFile(assertFile)
			if err != nil {
				t.Fatalf("reading assertion file: %v", err)
			}
			got := strings.TrimSpace(string(data))
			if got != tt.expected {
				t.Errorf("expected prompt to contain %s, but mock found %s", tt.expected, got)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 4. Multi-iteration scenarios
// ═══════════════════════════════════════════════════════════════════════════

// TestIntegration_YieldThenComplete_WithPromptValidation extends the basic
// yield-then-complete flow by also verifying that the prompt delivered to the
// model contains expected content on both iterations.
func TestIntegration_YieldThenComplete_WithPromptValidation(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	d.Config.Daemon.MaxIterations = 5

	setupLeafNode(t, d, "yield-node", []state.Task{
		{ID: "task-0001", Description: "multi-iter task", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	})

	counterFile := filepath.Join(t.TempDir(), "counter")
	assertFile := filepath.Join(t.TempDir(), "prompts.log")
	if err := os.WriteFile(counterFile, []byte("0"), 0644); err != nil {
		t.Fatal(err)
	}

	scriptFile := filepath.Join(t.TempDir(), "yield-then-complete.sh")
	script := fmt.Sprintf(`#!/bin/sh
PROMPT=$(cat)
n=$(cat %s)
echo $((n+1)) > %s
# Log what iteration sees
echo "ITER${n}:" >> %s
echo "$PROMPT" | grep -c "yield-node" >> %s
echo "$PROMPT" | grep -c "multi-iter task" >> %s
if [ "$n" = "0" ]; then
  echo "WOLFCASTLE_YIELD"
else
  echo "WOLFCASTLE_COMPLETE"
fi
`, counterFile, counterFile, assertFile, assertFile, assertFile)

	if err := os.WriteFile(scriptFile, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	d.Config.Models["echo"] = config.ModelDef{
		Command: "sh",
		Args:    []string{scriptFile},
	}
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	// Iteration 1: YIELD
	_ = d.Logger.StartIteration()
	r1, err := d.RunOnce(context.Background())
	d.Logger.Close()
	if err != nil {
		t.Fatalf("iteration 1 error: %v", err)
	}
	if r1 != IterationDidWork {
		t.Fatalf("iteration 1: expected DidWork, got %v", r1)
	}

	projDir := d.Resolver.ProjectsDir()
	ns1, _ := state.LoadNodeState(filepath.Join(projDir, "yield-node", "state.json"))
	if ns1.Tasks[0].State != state.StatusInProgress {
		t.Errorf("after yield: expected in_progress, got %s", ns1.Tasks[0].State)
	}

	// Iteration 2: COMPLETE
	_ = d.Logger.StartIteration()
	r2, err := d.RunOnce(context.Background())
	d.Logger.Close()
	if err != nil {
		t.Fatalf("iteration 2 error: %v", err)
	}
	if r2 != IterationDidWork {
		t.Fatalf("iteration 2: expected DidWork, got %v", r2)
	}

	ns2, _ := state.LoadNodeState(filepath.Join(projDir, "yield-node", "state.json"))
	if ns2.Tasks[0].State != state.StatusComplete {
		t.Errorf("after complete: expected complete, got %s", ns2.Tasks[0].State)
	}

	// Verify both iterations received valid prompts
	logData, err := os.ReadFile(assertFile)
	if err != nil {
		t.Fatalf("reading prompt log: %v", err)
	}
	logStr := string(logData)
	if !strings.Contains(logStr, "ITER0:") {
		t.Error("missing prompt log for iteration 0")
	}
	if !strings.Contains(logStr, "ITER1:") {
		t.Error("missing prompt log for iteration 1")
	}
	// Each grep -c should return "1" (at least one match), never "0"
	for _, line := range strings.Split(logStr, "\n") {
		line = strings.TrimSpace(line)
		if line == "0" {
			t.Errorf("prompt validation failed: a grep returned 0 matches in log:\n%s", logStr)
			break
		}
	}
}

// TestIntegration_BlockedTransition verifies that WOLFCASTLE_BLOCKED transitions
// the task to blocked state.
func TestIntegration_BlockedTransition(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false

	setupLeafNode(t, d, "block-node", []state.Task{
		{ID: "task-0001", Description: "will be blocked", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	})

	d.Config.Models["echo"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"WOLFCASTLE_BLOCKED"},
	}
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	_ = d.Logger.StartIteration()
	result, err := d.RunOnce(context.Background())
	d.Logger.Close()
	if err != nil {
		t.Fatalf("iteration error: %v", err)
	}
	if result != IterationDidWork {
		t.Fatalf("expected DidWork, got %v", result)
	}

	projDir := d.Resolver.ProjectsDir()
	ns, _ := state.LoadNodeState(filepath.Join(projDir, "block-node", "state.json"))
	if ns.Tasks[0].State != state.StatusBlocked {
		t.Errorf("expected blocked, got %s", ns.Tasks[0].State)
	}
}

// TestIntegration_FailureEscalation_DecompositionThreshold verifies that
// repeated model failures (no terminal marker) increment the failure count
// and trigger decomposition when the threshold is reached.
func TestIntegration_FailureEscalation_DecompositionThreshold(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	d.Config.Failure.DecompositionThreshold = 2
	d.Config.Failure.MaxDecompositionDepth = 3
	d.Config.Failure.HardCap = 10

	setupLeafNode(t, d, "fail-node", []state.Task{
		{ID: "task-0001", Description: "will fail repeatedly", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	})

	// Model emits no terminal marker
	d.Config.Models["echo"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"I did some work but forgot the marker"},
	}
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	projDir := d.Resolver.ProjectsDir()
	statePath := filepath.Join(projDir, "fail-node", "state.json")

	// Run twice (threshold is 2)
	for i := 0; i < 2; i++ {
		_ = d.Logger.StartIteration()
		_, err := d.RunOnce(context.Background())
		d.Logger.Close()
		if err != nil {
			t.Fatalf("iteration %d error: %v", i+1, err)
		}
	}

	ns, _ := state.LoadNodeState(statePath)
	if ns.Tasks[0].FailureCount != 2 {
		t.Errorf("expected failure count 2, got %d", ns.Tasks[0].FailureCount)
	}
	if !ns.Tasks[0].NeedsDecomposition {
		t.Error("expected NeedsDecomposition to be true after reaching threshold")
	}
}

// TestIntegration_FailureEscalation_HardCapAutoBlock verifies that after
// reaching the hard cap of failures, the task is automatically blocked.
func TestIntegration_FailureEscalation_HardCapAutoBlock(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	d.Config.Failure.DecompositionThreshold = 100 // high so it doesn't trigger
	d.Config.Failure.HardCap = 3

	setupLeafNode(t, d, "hardcap-node", []state.Task{
		{ID: "task-0001", Description: "will hit hard cap", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	})

	d.Config.Models["echo"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"no marker here"},
	}
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	projDir := d.Resolver.ProjectsDir()
	statePath := filepath.Join(projDir, "hardcap-node", "state.json")

	// Run 3 times to hit hard cap
	for i := 0; i < 3; i++ {
		_ = d.Logger.StartIteration()
		_, err := d.RunOnce(context.Background())
		d.Logger.Close()
		if err != nil {
			t.Fatalf("iteration %d error: %v", i+1, err)
		}
	}

	ns, _ := state.LoadNodeState(statePath)
	if ns.Tasks[0].State != state.StatusBlocked {
		t.Errorf("expected blocked after hard cap, got %s", ns.Tasks[0].State)
	}
	if ns.Tasks[0].FailureCount != 3 {
		t.Errorf("expected failure count 3, got %d", ns.Tasks[0].FailureCount)
	}
}

// TestIntegration_FailureEscalation_MaxDepthAutoBlock verifies that when
// decomposition threshold is hit but max decomposition depth is already
// reached, the task is auto-blocked instead.
func TestIntegration_FailureEscalation_MaxDepthAutoBlock(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	d.Config.Failure.DecompositionThreshold = 1
	d.Config.Failure.MaxDecompositionDepth = 2
	d.Config.Failure.HardCap = 100

	// Set up a node that is already at max decomposition depth
	projDir := d.Resolver.ProjectsDir()
	nodeDir := filepath.Join(projDir, "maxdepth-node")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}

	ns := state.NewNodeState("maxdepth-node", "maxdepth-node", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "at max depth", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	ns.DecompositionDepth = 2 // already at max
	data, _ := json.MarshalIndent(ns, "", "  ")
	if err := os.WriteFile(filepath.Join(nodeDir, "state.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	idx := state.NewRootIndex()
	idx.Root = []string{"maxdepth-node"}
	idx.Nodes["maxdepth-node"] = state.IndexEntry{
		Name:    "maxdepth-node",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: "maxdepth-node",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	d.Config.Models["echo"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"no marker"},
	}
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	_ = d.Logger.StartIteration()
	_, err := d.RunOnce(context.Background())
	d.Logger.Close()
	if err != nil {
		t.Fatalf("iteration error: %v", err)
	}

	nsAfter, _ := state.LoadNodeState(filepath.Join(nodeDir, "state.json"))
	if nsAfter.Tasks[0].State != state.StatusBlocked {
		t.Errorf("expected blocked at max depth, got %s", nsAfter.Tasks[0].State)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 5. Prompt echo rejection
// ═══════════════════════════════════════════════════════════════════════════

// TestIntegration_PromptEchoRejection_JSONStream verifies that marker names
// embedded in instructional text (prompt echo) inside JSON stream envelopes
// are not falsely matched. Only standalone markers should trigger detection.
func TestIntegration_PromptEchoRejection_JSONStream(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false

	setupLeafNode(t, d, "echo-node", []state.Task{
		{ID: "task-0001", Description: "echo rejection test", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	})

	// The model echoes prompt instructions containing marker names in sentences,
	// then later emits the actual COMPLETE marker in a result envelope.
	d.Config.Models["echo"] = config.ModelDef{
		Command: "sh",
		Args: []string{"-c", `cat <<'SCRIPT'
{"type":"assistant","text":"I see the instructions say to emit WOLFCASTLE_COMPLETE when done."}
{"type":"assistant","text":"The prompt mentions WOLFCASTLE_YIELD for partial progress."}
{"type":"assistant","text":"And WOLFCASTLE_BLOCKED for stuck tasks."}
{"type":"assistant","text":"Working on the task now..."}
{"type":"result","result":"Finished.\n\nWOLFCASTLE_COMPLETE"}
SCRIPT`},
	}
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	_ = d.Logger.StartIteration()
	result, err := d.RunOnce(context.Background())
	d.Logger.Close()
	if err != nil {
		t.Fatalf("iteration error: %v", err)
	}
	if result != IterationDidWork {
		t.Fatalf("expected DidWork, got %v", result)
	}

	projDir := d.Resolver.ProjectsDir()
	ns, _ := state.LoadNodeState(filepath.Join(projDir, "echo-node", "state.json"))
	// The task should be complete, not blocked or yielded, because the embedded
	// mentions are in sentences and only the final standalone COMPLETE counts.
	if ns.Tasks[0].State != state.StatusComplete {
		t.Errorf("expected complete (echo rejection should work), got %s", ns.Tasks[0].State)
	}
}

// TestIntegration_PromptEchoRejection_NoStandaloneMarker verifies that when
// the model only echoes marker names in sentences (no standalone marker),
// no terminal marker is detected and the failure count increments.
func TestIntegration_PromptEchoRejection_NoStandaloneMarker(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	d.Config.Failure.HardCap = 100
	d.Config.Failure.DecompositionThreshold = 100

	setupLeafNode(t, d, "nomarker-node", []state.Task{
		{ID: "task-0001", Description: "no real marker", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	})

	// All markers are embedded in sentences, none standalone
	d.Config.Models["echo"] = config.ModelDef{
		Command: "sh",
		Args: []string{"-c", `cat <<'SCRIPT'
{"type":"assistant","text":"The user should emit WOLFCASTLE_COMPLETE when the task is done."}
{"type":"assistant","text":"Or use WOLFCASTLE_YIELD if more work is needed."}
{"type":"result","result":"I explained the markers but did not produce one myself."}
SCRIPT`},
	}
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	_ = d.Logger.StartIteration()
	_, err := d.RunOnce(context.Background())
	d.Logger.Close()
	if err != nil {
		t.Fatalf("iteration error: %v", err)
	}

	projDir := d.Resolver.ProjectsDir()
	ns, _ := state.LoadNodeState(filepath.Join(projDir, "nomarker-node", "state.json"))
	if ns.Tasks[0].FailureCount != 1 {
		t.Errorf("expected failure count 1 (no standalone marker), got %d", ns.Tasks[0].FailureCount)
	}
	// Task should still be in_progress, not complete or blocked
	if ns.Tasks[0].State != state.StatusInProgress {
		t.Errorf("expected in_progress (no marker matched), got %s", ns.Tasks[0].State)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 6. Self-healing (crash recovery)
// ═══════════════════════════════════════════════════════════════════════════

// TestIntegration_SelfHeal_ResumesInProgressTask verifies that a task left
// in in_progress state (simulating a crash) is resumed by the daemon without
// re-claiming. The model sees the task and completes it.
func TestIntegration_SelfHeal_ResumesInProgressTask(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false

	// Set up a task already in in_progress state (as if daemon crashed mid-work)
	projDir := d.Resolver.ProjectsDir()
	nodeDir := filepath.Join(projDir, "crash-node")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}

	ns := state.NewNodeState("crash-node", "crash-node", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "was interrupted", State: state.StatusInProgress},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	data, _ := json.MarshalIndent(ns, "", "  ")
	if err := os.WriteFile(filepath.Join(nodeDir, "state.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	idx := state.NewRootIndex()
	idx.Root = []string{"crash-node"}
	idx.Nodes["crash-node"] = state.IndexEntry{
		Name:    "crash-node",
		Type:    state.NodeLeaf,
		State:   state.StatusInProgress,
		Address: "crash-node",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	assertFile := filepath.Join(t.TempDir(), "crash-assertions.txt")
	scriptFile := filepath.Join(t.TempDir(), "crash-check.sh")

	// The mock verifies it receives the right task and completes it.
	// If claim was attempted on an already in_progress task, it would fail
	// before reaching the model. The fact the model runs proves no re-claim.
	script := fmt.Sprintf(`#!/bin/sh
PROMPT=$(cat)
echo "$PROMPT" | grep -q "was interrupted" && printf "SAW_TASK" > %s
echo "WOLFCASTLE_COMPLETE"
`, assertFile)

	if err := os.WriteFile(scriptFile, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	d.Config.Models["echo"] = config.ModelDef{
		Command: "sh",
		Args:    []string{scriptFile},
	}
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	_ = d.Logger.StartIteration()
	result, err := d.RunOnce(context.Background())
	d.Logger.Close()
	if err != nil {
		t.Fatalf("iteration error: %v", err)
	}
	if result != IterationDidWork {
		t.Fatalf("expected DidWork, got %v", result)
	}

	// Verify the model was actually invoked with the right task
	assertData, err := os.ReadFile(assertFile)
	if err != nil {
		t.Fatalf("reading assertion file: %v", err)
	}
	if string(assertData) != "SAW_TASK" {
		t.Error("model did not receive the interrupted task's description")
	}

	// Verify task completed successfully
	nsAfter, _ := state.LoadNodeState(filepath.Join(nodeDir, "state.json"))
	if nsAfter.Tasks[0].State != state.StatusComplete {
		t.Errorf("expected complete after recovery, got %s", nsAfter.Tasks[0].State)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 7. Deliverable verification
// ═══════════════════════════════════════════════════════════════════════════

// TestIntegration_MissingDeliverable_RejectsComplete verifies that when a task
// declares deliverables and the model emits WOLFCASTLE_COMPLETE without creating
// the files, the completion is rejected and the failure count increments.
func TestIntegration_MissingDeliverable_RejectsComplete(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	d.Config.Failure.HardCap = 100
	d.Config.Failure.DecompositionThreshold = 100

	setupLeafNode(t, d, "deliv-node", []state.Task{
		{
			ID:           "task-0001",
			Description:  "task with deliverables",
			State:        state.StatusNotStarted,
			Deliverables: []string{"docs/output.md"},
		},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	})

	// Model says COMPLETE but does not create the deliverable file
	d.Config.Models["echo"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"WOLFCASTLE_COMPLETE"},
	}
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	_ = d.Logger.StartIteration()
	_, err := d.RunOnce(context.Background())
	d.Logger.Close()
	if err != nil {
		t.Fatalf("iteration error: %v", err)
	}

	projDir := d.Resolver.ProjectsDir()
	ns, _ := state.LoadNodeState(filepath.Join(projDir, "deliv-node", "state.json"))

	// Task should NOT be complete (deliverable missing)
	if ns.Tasks[0].State == state.StatusComplete {
		t.Error("task should not be complete when deliverables are missing")
	}
	if ns.Tasks[0].FailureCount != 1 {
		t.Errorf("expected failure count 1, got %d", ns.Tasks[0].FailureCount)
	}
}

// TestIntegration_DeliverableExists_AcceptsComplete verifies that when a task
// declares deliverables and they all exist on disk, WOLFCASTLE_COMPLETE is accepted.
func TestIntegration_DeliverableExists_AcceptsComplete(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false

	setupLeafNode(t, d, "deliv-ok-node", []state.Task{
		{
			ID:           "task-0001",
			Description:  "task with deliverables",
			State:        state.StatusNotStarted,
			Deliverables: []string{"docs/output.md"},
		},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	})

	// Create the deliverable file in the repo dir before the model runs
	docsDir := filepath.Join(d.RepoDir, "docs")
	_ = os.MkdirAll(docsDir, 0755)
	_ = os.WriteFile(filepath.Join(docsDir, "output.md"), []byte("deliverable content"), 0644)

	d.Config.Models["echo"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"WOLFCASTLE_COMPLETE"},
	}
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	_ = d.Logger.StartIteration()
	_, err := d.RunOnce(context.Background())
	d.Logger.Close()
	if err != nil {
		t.Fatalf("iteration error: %v", err)
	}

	projDir := d.Resolver.ProjectsDir()
	ns, _ := state.LoadNodeState(filepath.Join(projDir, "deliv-ok-node", "state.json"))

	if ns.Tasks[0].State != state.StatusComplete {
		t.Errorf("expected complete when deliverables exist, got %s", ns.Tasks[0].State)
	}
}

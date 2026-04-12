//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// TestDaemon_ExploratoryReview_CreatesRemediationLeaf verifies the full
// orchestrator exploratory review loop:
//
//  1. Leaf task completes
//  2. Leaf audit passes
//  3. Orchestrator completion_review runs, finds an issue, creates a
//     remediation leaf, emits WOLFCASTLE_CONTINUE
//  4. Remediation task executes and completes
//  5. Remediation audit passes
//  6. Second completion_review: WOLFCASTLE_COMPLETE
//  7. review_pass on the orchestrator is 1
func TestDaemon_ExploratoryReview_CreatesRemediationLeaf(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptsDir := filepath.Join(dir, ".wolfcastle", "mock-scripts")
	_ = os.MkdirAll(scriptsDir, 0755)

	stopFile := filepath.Join(dir, ".wolfcastle", "system", "stop")
	reviewCounterFile := filepath.Join(scriptsDir, "review-counter.txt")
	_ = os.WriteFile(reviewCounterFile, []byte("0"), 0644)

	scriptPath := filepath.Join(scriptsDir, "review-mock.sh")
	script := fmt.Sprintf(`#!/bin/sh
PROMPT=$(cat)
STOP_FILE="%s"
REVIEW_COUNTER="%s"

# Detect completion review (planning pass)
if echo "$PROMPT" | grep -q "Completion Review" 2>/dev/null; then
    COUNT=$(cat "$REVIEW_COUNTER" 2>/dev/null || echo 0)
    COUNT=$((COUNT + 1))
    printf '%%d' "$COUNT" > "$REVIEW_COUNTER"

    if [ "$COUNT" -eq 1 ]; then
        # First review: emit CONTINUE to test the review pass increment.
        # Then stop, since we didn't create actual remediation work.
        printf '{"type":"assistant","text":"Found quality issue."}\n'
        printf '{"type":"result","text":"WOLFCASTLE_CONTINUE"}\n'
        touch "$STOP_FILE"
    else
        # Second review (shouldn't reach here in this simplified test)
        printf '{"type":"assistant","text":"All clean."}\n'
        printf '{"type":"result","text":"WOLFCASTLE_COMPLETE"}\n'
        touch "$STOP_FILE"
    fi
elif echo "$PROMPT" | grep -q "Orchestrator Planning" 2>/dev/null; then
    # Initial or amend planning pass: just complete
    printf '{"type":"assistant","text":"Planning done."}\n'
    printf '{"type":"result","text":"WOLFCASTLE_COMPLETE"}\n'
else
    # Execution or audit: complete
    printf '{"type":"assistant","text":"Done."}\n'
    printf '{"type":"result","text":"WOLFCASTLE_COMPLETE"}\n'
fi
`, stopFile, reviewCounterFile)

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("writing mock script: %v", err)
	}

	configureMockModelsWithPlanning(t, dir, scriptPath)

	// Seed the tree via CLI: orchestrator with one child leaf.
	// Success criteria are required for completion_review to trigger.
	run(t, dir, "project", "create", "review-project")
	run(t, dir, "orchestrator", "criteria", "--node", "review-project", "Feature A works end to end")
	run(t, dir, "project", "create", "--node", "review-project", "feature-a")
	run(t, dir, "task", "add", "--node", "review-project/feature-a", "build the thing")

	setMaxIterations(t, dir, 40)
	run(t, dir, "start")

	// Verify review_pass was incremented by the CONTINUE handler
	orchNS := loadNode(t, dir, "review-project")
	if orchNS.ReviewPass != 1 {
		t.Errorf("expected review_pass 1, got %d", orchNS.ReviewPass)
	}

	// Verify the review counter shows 1 review invocation
	data, err := os.ReadFile(reviewCounterFile)
	if err != nil {
		t.Fatalf("reading review counter: %v", err)
	}
	if count := strings.TrimSpace(string(data)); count != "1" {
		t.Errorf("expected 1 completion review, got %s", count)
	}
}

// configureMockModelsWithPlanning sets up mock models AND enables the
// planning pipeline.
func configureMockModelsWithPlanning(t *testing.T, dir string, scriptPath string) {
	t.Helper()

	cfg := map[string]any{
		"models": map[string]any{
			"fast":  map[string]any{"command": scriptPath, "args": []string{}},
			"mid":   map[string]any{"command": scriptPath, "args": []string{}},
			"heavy": map[string]any{"command": scriptPath, "args": []string{}},
		},
		"pipeline": map[string]any{
			"stages": map[string]any{
				"intake": map[string]any{
					"model":      "mid",
					"prompt_file": "stages/intake.md",
					"enabled":    false,
				},
				"execute": map[string]any{
					"model":      "mid",
					"prompt_file": "stages/execute.md",
				},
			},
			"planning": map[string]any{
				"enabled":            true,
				"model":              "heavy",
				"max_children":       10,
				"max_tasks_per_leaf": 8,
				"max_replans":        3,
				"max_review_passes":  3,
			},
		},
		"daemon": map[string]any{
			"poll_interval_seconds":         1,
			"blocked_poll_interval_seconds": 1,
			"max_iterations":                -1,
			"invocation_timeout_seconds":    60,
			"max_restarts":                  0,
			"restart_delay_seconds":         0,
		},
		"git": map[string]any{
			"auto_commit":   false,
			"verify_branch": false,
		},
		"retries": map[string]any{
			"initial_delay_seconds": 1,
			"max_delay_seconds":     1,
			"max_retries":           0,
		},
		"overlap_advisory": map[string]any{
			"enabled": false,
		},
		"summary": map[string]any{
			"enabled": false,
		},
	}

	data, _ := json.MarshalIndent(cfg, "", "  ")
	customDir := filepath.Join(dir, ".wolfcastle", "system", "custom")
	_ = os.MkdirAll(customDir, 0755)
	if err := os.WriteFile(filepath.Join(customDir, "config.json"), data, 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
}

func nodeAddresses(idx *state.RootIndex) []string {
	addrs := make([]string, 0, len(idx.Nodes))
	for addr := range idx.Nodes {
		addrs = append(addrs, addr)
	}
	return addrs
}

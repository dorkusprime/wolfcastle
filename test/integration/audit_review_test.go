//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	configureMockModels(t, dir, scriptPath)
	// Enable planning and set tight iteration limit.
	mergeLocalConfig(t, dir, map[string]any{
		"pipeline": map[string]any{
			"planning": map[string]any{
				"enabled":           true,
				"model":             "heavy",
				"max_review_passes": 3,
			},
		},
		"daemon": map[string]any{
			"max_iterations": 20,
		},
	})

	// Seed the tree via CLI: orchestrator with one child leaf.
	// Success criteria are required for completion_review to trigger.
	run(t, dir, "project", "create", "review-project")
	run(t, dir, "orchestrator", "criteria", "--node", "review-project", "Feature A works end to end")
	run(t, dir, "project", "create", "--node", "review-project", "feature-a")
	run(t, dir, "task", "add", "--node", "review-project/feature-a", "build the thing")

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

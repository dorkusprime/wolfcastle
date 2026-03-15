//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MockBehavior describes what the mock model should do on a single invocation.
type MockBehavior struct {
	// Terminal marker to emit: "WOLFCASTLE_COMPLETE", "WOLFCASTLE_YIELD",
	// "WOLFCASTLE_BLOCKED", or "" (no marker).
	Marker string

	// Files to create (path relative to working dir -> content).
	CreateFiles map[string]string

	// Whether to emit a WOLFCASTLE_BREADCRUMB marker in stdout.
	// The daemon's ParseMarkers picks these up and applies them to state.
	WriteBreadcrumb bool
	BreadcrumbText  string

	// Whether to emit a WOLFCASTLE_GAP marker in stdout.
	WriteGap bool
	GapText  string

	// Whether to call wolfcastle audit breadcrumb via CLI.
	// Note: CLI-driven state changes are overwritten by the daemon's save,
	// so this is useful for testing CLI invocation itself, not for state
	// assertions on breadcrumbs.
	CallBreadcrumbCLI bool
	CLIBreadcrumbText string

	// Whether to call wolfcastle audit gap via CLI.
	CallGapCLI bool
	CLIGapText string

	// Whether to call wolfcastle task complete (simulating model calling
	// CLI directly).
	CallTaskComplete bool

	// Whether to call wolfcastle task block (simulating model calling
	// CLI directly). Requires a reason string.
	CallTaskBlock   bool
	TaskBlockReason string

	// Strings that must appear in the prompt (stdin). Failures are written
	// to an assertion file that the test can inspect.
	ExpectInPrompt []string

	// Arbitrary shell commands to run before emitting the marker.
	ExtraCommands []string

	// Delay in seconds (simulate model thinking).
	DelaySeconds int
}

// MockModelConfig holds a sequence of behaviors indexed by invocation count.
// When more calls occur than behaviors defined, the last behavior repeats.
type MockModelConfig struct {
	Behaviors []MockBehavior
}

// createRealisticMock generates a sh-compatible mock model script from a
// MockModelConfig. It returns the script path and the path to an assertion
// failures file the test can read after the daemon finishes.
func createRealisticMock(t *testing.T, dir string, name string, cfg MockModelConfig) (scriptPath, assertionFile string) {
	t.Helper()

	if len(cfg.Behaviors) == 0 {
		t.Fatal("MockModelConfig must have at least one behavior")
	}

	scriptsDir := filepath.Join(dir, ".wolfcastle", "mock-scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("creating mock scripts dir: %v", err)
	}

	stopFile := filepath.Join(dir, ".wolfcastle", "stop")
	counterFile := filepath.Join(scriptsDir, name+"-counter.txt")
	promptDir := filepath.Join(scriptsDir, name+"-prompts")
	assertionFile = filepath.Join(scriptsDir, name+"-assertions.txt")
	scriptPath = filepath.Join(scriptsDir, name+".sh")

	// Initialize counter
	if err := os.WriteFile(counterFile, []byte("0"), 0644); err != nil {
		t.Fatalf("writing counter file: %v", err)
	}
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		t.Fatalf("creating prompt dir: %v", err)
	}

	// Build per-behavior case blocks for the shell script.
	var caseBranches strings.Builder
	for i, beh := range cfg.Behaviors {
		var block strings.Builder

		// Optional delay
		if beh.DelaySeconds > 0 {
			fmt.Fprintf(&block, "  sleep %d\n", beh.DelaySeconds)
		}

		// Prompt assertion checks
		for _, expected := range beh.ExpectInPrompt {
			// Escape single quotes for shell
			escaped := strings.ReplaceAll(expected, "'", "'\\''")
			fmt.Fprintf(&block, "  if ! grep -qF '%s' \"$PROMPT_FILE\"; then\n", escaped)
			fmt.Fprintf(&block, "    printf 'FAIL: prompt missing: %s\\n' >> \"%s\"\n", escaped, assertionFile)
			fmt.Fprintf(&block, "  fi\n")
		}

		// File creation
		for path, content := range beh.CreateFiles {
			escaped := strings.ReplaceAll(content, "'", "'\\''")
			if d := filepath.Dir(path); d != "." && d != "" {
				fmt.Fprintf(&block, "  mkdir -p '%s'\n", d)
			}
			fmt.Fprintf(&block, "  printf '%%s' '%s' > '%s'\n", escaped, path)
		}

		// Extra commands
		for _, cmd := range beh.ExtraCommands {
			fmt.Fprintf(&block, "  %s\n", cmd)
		}

		// CLI calls (breadcrumb and gap via the wolfcastle binary)
		if beh.CallBreadcrumbCLI {
			text := beh.CLIBreadcrumbText
			if text == "" {
				text = fmt.Sprintf("cli breadcrumb from invocation %d", i+1)
			}
			escaped := strings.ReplaceAll(text, "'", "'\\''")
			fmt.Fprintf(&block, "  \"$BINARY_PATH\" audit breadcrumb --node \"$NODE_ADDR\" '%s' 2>/dev/null || true\n", escaped)
		}
		if beh.CallGapCLI {
			text := beh.CLIGapText
			if text == "" {
				text = fmt.Sprintf("cli gap from invocation %d", i+1)
			}
			escaped := strings.ReplaceAll(text, "'", "'\\''")
			fmt.Fprintf(&block, "  \"$BINARY_PATH\" audit gap --node \"$NODE_ADDR\" '%s' 2>/dev/null || true\n", escaped)
		}

		// CLI calls for task state changes
		if beh.CallTaskComplete {
			// task complete --node <node-addr/task-0001>
			fmt.Fprintf(&block, "  \"$BINARY_PATH\" task complete --node \"${NODE_ADDR}/task-0001\" 2>/dev/null || true\n")
		}
		if beh.CallTaskBlock {
			reason := beh.TaskBlockReason
			if reason == "" {
				reason = "blocked by model"
			}
			escaped := strings.ReplaceAll(reason, "'", "'\\''")
			fmt.Fprintf(&block, "  \"$BINARY_PATH\" task block --node \"${NODE_ADDR}/task-0001\" '%s' 2>/dev/null || true\n", escaped)
		}

		// Stream-json output with embedded markers.
		// The daemon's ParseMarkers scans raw stdout lines for
		// WOLFCASTLE_BREADCRUMB:, WOLFCASTLE_GAP:, etc. These must
		// appear as raw text lines (not JSON-wrapped) for the daemon
		// to pick them up.

		// Breadcrumb marker in stdout
		if beh.WriteBreadcrumb {
			bcText := beh.BreadcrumbText
			if bcText == "" {
				bcText = fmt.Sprintf("breadcrumb from invocation %d", i+1)
			}
			escaped := strings.ReplaceAll(bcText, "'", "'\\''")
			fmt.Fprintf(&block, "  printf 'WOLFCASTLE_BREADCRUMB: %s\\n'\n", escaped)
		}

		// Gap marker in stdout
		if beh.WriteGap {
			gapText := beh.GapText
			if gapText == "" {
				gapText = fmt.Sprintf("gap from invocation %d", i+1)
			}
			escaped := strings.ReplaceAll(gapText, "'", "'\\''")
			fmt.Fprintf(&block, "  printf 'WOLFCASTLE_GAP: %s\\n'\n", escaped)
		}

		// Terminal marker (emitted as both raw line and JSON envelope)
		if beh.Marker != "" {
			fmt.Fprintf(&block, "  printf '{\"type\":\"assistant\",\"text\":\"Working...\"}\\n'\n")
			fmt.Fprintf(&block, "  printf '{\"type\":\"result\",\"text\":\"%s\"}\\n'\n", beh.Marker)
		} else {
			fmt.Fprintf(&block, "  printf '{\"type\":\"assistant\",\"text\":\"No marker emitted.\"}\\n'\n")
		}

		// Stop file for terminal markers on the last defined behavior
		if beh.Marker == "WOLFCASTLE_COMPLETE" || beh.Marker == "WOLFCASTLE_BLOCKED" {
			if i == len(cfg.Behaviors)-1 {
				fmt.Fprintf(&block, "  touch \"%s\"\n", stopFile)
			}
		}

		// Write the case branch. Last behavior uses "*)" to catch all
		// invocations beyond the defined set.
		if i < len(cfg.Behaviors)-1 {
			fmt.Fprintf(&caseBranches, "  %d)\n%s  ;;\n", i, block.String())
		} else {
			fmt.Fprintf(&caseBranches, "  *)\n%s  ;;\n", block.String())
		}
	}

	// The script: reads stdin, determines invocation index, dispatches.
	// We intentionally omit set -e so that a failing CLI call does not
	// abort the script before it can emit the terminal marker.
	script := fmt.Sprintf(`#!/bin/sh

BINARY_PATH="%s"
COUNTER_FILE="%s"
PROMPT_DIR="%s"

# Read stdin (the prompt) into a temp file
PROMPT_FILE="$PROMPT_DIR/prompt-$$.txt"
cat > "$PROMPT_FILE"

# Read and increment counter
COUNT=$(cat "$COUNTER_FILE" 2>/dev/null || printf '0')
printf '%%s' "$COUNT" | cat > /dev/null
BEHAVIOR_INDEX=$COUNT
COUNT=$((COUNT + 1))
printf '%%s' "$COUNT" > "$COUNTER_FILE"

# Extract node address from prompt (line starting with **Node:**)
NODE_ADDR=$(grep -m1 '^\*\*Node:\*\*' "$PROMPT_FILE" | sed 's/\*\*Node:\*\* //' | tr -d '[:space:]')

case "$BEHAVIOR_INDEX" in
%s
esac
`, binaryPath, counterFile, promptDir, caseBranches.String())

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("writing realistic mock script: %v", err)
	}

	return scriptPath, assertionFile
}

// readAssertionFailures reads the assertion file and returns any failures
// recorded by the mock script during prompt validation.
func readAssertionFailures(t *testing.T, assertionFile string) []string {
	t.Helper()
	data, err := os.ReadFile(assertionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No failures recorded
		}
		t.Fatalf("reading assertion file: %v", err)
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

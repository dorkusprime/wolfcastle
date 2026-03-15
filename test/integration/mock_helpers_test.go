//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// createMockModel creates a shell script that emits Claude Code stream-json
// format output with a specific terminal marker behavior.
//
// Supported behaviors:
//   - "complete": emits WOLFCASTLE_COMPLETE then creates stop file
//   - "yield": emits WOLFCASTLE_YIELD (no stop file, daemon continues)
//   - "blocked": emits WOLFCASTLE_BLOCKED then creates stop file
//   - "no-marker": emits output with no terminal marker (no stop file)
//   - "create-file": creates a file in the working directory, then emits WOLFCASTLE_COMPLETE and stop file
func createMockModel(t *testing.T, dir string, name string, behavior string) string {
	t.Helper()
	scriptsDir := filepath.Join(dir, ".wolfcastle", "mock-scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("creating mock scripts dir: %v", err)
	}

	stopFile := filepath.Join(dir, ".wolfcastle", "stop")
	scriptPath := filepath.Join(scriptsDir, name+".sh")

	// All scripts consume stdin (the prompt) and emit stream-json to stdout.
	// Scripts that emit a terminal completion marker also create the stop
	// file so the daemon exits cleanly after processing the iteration.
	var body string
	switch behavior {
	case "complete":
		body = fmt.Sprintf(`#!/bin/sh
cat > /dev/null
printf '{"type":"assistant","text":"Working on it..."}\n'
printf '{"type":"result","text":"WOLFCASTLE_COMPLETE"}\n'
touch "%s"
`, stopFile)
	case "yield":
		body = `#!/bin/sh
cat > /dev/null
printf '{"type":"assistant","text":"Made some progress."}\n'
printf '{"type":"result","text":"WOLFCASTLE_YIELD"}\n'
`
	case "blocked":
		body = fmt.Sprintf(`#!/bin/sh
cat > /dev/null
printf '{"type":"assistant","text":"Cannot proceed."}\n'
printf '{"type":"result","text":"WOLFCASTLE_BLOCKED"}\n'
touch "%s"
`, stopFile)
	case "no-marker":
		body = `#!/bin/sh
cat > /dev/null
printf '{"type":"assistant","text":"I did some stuff but forgot the marker."}\n'
`
	case "create-file":
		body = fmt.Sprintf(`#!/bin/sh
cat > /dev/null
touch mock-created-file.txt
printf '{"type":"assistant","text":"Created a file."}\n'
printf '{"type":"result","text":"WOLFCASTLE_COMPLETE"}\n'
touch "%s"
`, stopFile)
	default:
		t.Fatalf("unknown mock behavior: %s", behavior)
	}

	if err := os.WriteFile(scriptPath, []byte(body), 0755); err != nil {
		t.Fatalf("writing mock script: %v", err)
	}
	return scriptPath
}

// configureMockModels overwrites config.json to use a mock script for all
// three model tiers (fast, mid, heavy). It also sets fast poll intervals
// and disables git branch verification so tests run quickly in temp dirs.
func configureMockModels(t *testing.T, dir string, scriptPath string) {
	t.Helper()
	configureWithArgs(t, dir, scriptPath, nil)
}

// configureWithArgs overwrites config.json to point all model tiers at the
// given script with optional extra arguments.
func configureWithArgs(t *testing.T, dir string, scriptPath string, args []string) {
	t.Helper()

	modelArgs := args
	if modelArgs == nil {
		modelArgs = []string{}
	}

	type modelDef struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	cfg := map[string]any{
		"models": map[string]modelDef{
			"fast":  {Command: scriptPath, Args: modelArgs},
			"mid":   {Command: scriptPath, Args: modelArgs},
			"heavy": {Command: scriptPath, Args: modelArgs},
		},
		"daemon": map[string]any{
			"poll_interval_seconds":         1,
			"blocked_poll_interval_seconds": 1,
			"max_iterations":                -1,
			"invocation_timeout_seconds":    60,
			"max_restarts":                  0,
			"restart_delay_seconds":         0,
			"log_level":                     "info",
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

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshaling mock config: %v", err)
	}
	configPath := filepath.Join(dir, ".wolfcastle", "config.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("writing mock config: %v", err)
	}
}

// setMaxIterations merges a max_iterations setting into the existing
// config.local.json, preserving identity and other fields.
func setMaxIterations(t *testing.T, dir string, n int) {
	t.Helper()
	mergeLocalConfig(t, dir, map[string]any{
		"daemon": map[string]any{
			"max_iterations": n,
		},
	})
}

// mergeLocalConfig reads the existing config.local.json, shallow-merges
// the provided overrides into it, and writes the result back. This
// preserves the identity fields that init creates.
func mergeLocalConfig(t *testing.T, dir string, overrides map[string]any) {
	t.Helper()
	localPath := filepath.Join(dir, ".wolfcastle", "config.local.json")

	existing := map[string]any{}
	if data, err := os.ReadFile(localPath); err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			t.Fatalf("parsing existing config.local.json: %v", err)
		}
	}

	// Merge overrides into existing (one level deep)
	for k, v := range overrides {
		existingMap, existOk := existing[k].(map[string]any)
		overrideMap, overOk := v.(map[string]any)
		if existOk && overOk {
			for kk, vv := range overrideMap {
				existingMap[kk] = vv
			}
			existing[k] = existingMap
		} else {
			existing[k] = v
		}
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		t.Fatalf("marshaling merged local config: %v", err)
	}
	if err := os.WriteFile(localPath, data, 0644); err != nil {
		t.Fatalf("writing config.local.json: %v", err)
	}
}

// createCounterMock creates a shell script that yields yieldCount times
// then emits WOLFCASTLE_COMPLETE. It tracks invocation count via a counter
// file that callers can read to verify iteration counts.
func createCounterMock(t *testing.T, dir string, yieldCount int) (scriptPath, counterFile string) {
	t.Helper()
	scriptsDir := filepath.Join(dir, ".wolfcastle", "mock-scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("creating mock scripts dir: %v", err)
	}

	counterFile = filepath.Join(scriptsDir, "counter.txt")
	// Initialize counter file
	if err := os.WriteFile(counterFile, []byte("0"), 0644); err != nil {
		t.Fatalf("writing counter file: %v", err)
	}

	stopFile := filepath.Join(dir, ".wolfcastle", "stop")
	scriptPath = filepath.Join(scriptsDir, "counter-mock.sh")
	body := fmt.Sprintf(`#!/bin/sh
cat > /dev/null
COUNTER_FILE="%s"
COUNT=$(cat "$COUNTER_FILE" 2>/dev/null || printf '0')
COUNT=$((COUNT + 1))
printf '%%s' "$COUNT" > "$COUNTER_FILE"
if [ "$COUNT" -le %d ]; then
  printf '{"type":"result","text":"WOLFCASTLE_YIELD"}\n'
else
  printf '{"type":"result","text":"WOLFCASTLE_COMPLETE"}\n'
  touch "%s"
fi
`, counterFile, yieldCount, stopFile)

	if err := os.WriteFile(scriptPath, []byte(body), 0755); err != nil {
		t.Fatalf("writing counter mock script: %v", err)
	}
	return scriptPath, counterFile
}

// createNoMarkerStopAfterMock creates a script that emits no terminal
// marker for the first stopAfter invocations, then creates the daemon
// stop file. This allows failure-escalation tests to run a controlled
// number of iterations before the daemon exits cleanly.
func createNoMarkerStopAfterMock(t *testing.T, dir string, stopAfter int) string {
	t.Helper()
	scriptsDir := filepath.Join(dir, ".wolfcastle", "mock-scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("creating mock scripts dir: %v", err)
	}

	counterFile := filepath.Join(scriptsDir, "nomarker-counter.txt")
	if err := os.WriteFile(counterFile, []byte("0"), 0644); err != nil {
		t.Fatalf("writing counter file: %v", err)
	}

	stopFile := filepath.Join(dir, ".wolfcastle", "stop")
	scriptPath := filepath.Join(scriptsDir, "nomarker-stop.sh")
	body := fmt.Sprintf(`#!/bin/sh
cat > /dev/null
COUNTER_FILE="%s"
COUNT=$(cat "$COUNTER_FILE" 2>/dev/null || printf '0')
COUNT=$((COUNT + 1))
printf '%%s' "$COUNT" > "$COUNTER_FILE"
printf '{"type":"assistant","text":"No marker output."}\n'
if [ "$COUNT" -ge %d ]; then
  touch "%s"
fi
`, counterFile, stopAfter, stopFile)

	if err := os.WriteFile(scriptPath, []byte(body), 0755); err != nil {
		t.Fatalf("writing no-marker stop mock: %v", err)
	}
	return scriptPath
}

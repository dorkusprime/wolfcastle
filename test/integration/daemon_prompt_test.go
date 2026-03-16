//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test 29: The prompt piped to the model's stdin contains the task description.
func TestDaemon_PromptContainsTaskDescription(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, assertionFile := createRealisticMock(t, dir, "prompt-task-desc", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker: "WOLFCASTLE_COMPLETE",
				ExpectInPrompt: []string{
					"build the authentication layer",
				},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "desc-check")
	run(t, dir, "task", "add", "--node", "desc-check", "build the authentication layer")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	failures := readAssertionFailures(t, assertionFile)
	for _, f := range failures {
		t.Errorf("prompt assertion failure: %s", f)
	}
}

// Test 30: The prompt contains the node address so the model knows where it's working.
func TestDaemon_PromptContainsNodeAddress(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, assertionFile := createRealisticMock(t, dir, "prompt-node-addr", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker: "WOLFCASTLE_COMPLETE",
				ExpectInPrompt: []string{
					"desc-check-addr",
				},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "desc-check-addr")
	run(t, dir, "task", "add", "--node", "desc-check-addr", "some work")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	failures := readAssertionFailures(t, assertionFile)
	for _, f := range failures {
		t.Errorf("prompt assertion failure: %s", f)
	}
}

// Test 31: After a failure, the prompt contains failure context (failure_count or iteration text).
func TestDaemon_PromptContainsIterationContext(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, assertionFile := createRealisticMock(t, dir, "prompt-iter-ctx", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				// First call: no marker => failure, increments failure_count
				Marker: "",
			},
			{
				// Second call: prompt should contain failure count info
				Marker: "WOLFCASTLE_COMPLETE",
				ExpectInPrompt: []string{
					"Failure Count",
				},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	// High hard cap so first failure doesn't auto-block
	setFailureAndIterationConfig(t, dir, 10, 0, 50, 20)

	run(t, dir, "project", "create", "iter-ctx-test")
	run(t, dir, "task", "add", "--node", "iter-ctx-test", "task with failure context")

	run(t, dir, "start")

	failures := readAssertionFailures(t, assertionFile)
	for _, f := range failures {
		t.Errorf("prompt assertion failure: %s", f)
	}
}

// Test 32: A rule fragment placed in base/rules/ appears in the prompt.
func TestDaemon_PromptContainsBaseTierRules(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	// Write a rule fragment into base/rules/
	rulesDir := filepath.Join(dir, ".wolfcastle", "system", "base", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatalf("creating rules dir: %v", err)
	}
	sentinel := "SENTINEL_BASE_RULE_7742: always write tests first"
	if err := os.WriteFile(filepath.Join(rulesDir, "test-rule.md"), []byte(sentinel), 0644); err != nil {
		t.Fatalf("writing rule: %v", err)
	}

	scriptPath, assertionFile := createRealisticMock(t, dir, "base-rule-check", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:         "WOLFCASTLE_COMPLETE",
				ExpectInPrompt: []string{"SENTINEL_BASE_RULE_7742"},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "base-rule-test")
	run(t, dir, "task", "add", "--node", "base-rule-test", "task with base rules")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	failures := readAssertionFailures(t, assertionFile)
	for _, f := range failures {
		t.Errorf("rule assertion failure: %s", f)
	}
}

// Test 33: A rule in custom/rules/ overrides a same-named file in base/rules/.
func TestDaemon_PromptContainsCustomTierOverride(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	wcDir := filepath.Join(dir, ".wolfcastle")
	baseRulesDir := filepath.Join(wcDir, "system", "base", "rules")
	customRulesDir := filepath.Join(wcDir, "system", "custom", "rules")
	if err := os.MkdirAll(baseRulesDir, 0755); err != nil {
		t.Fatalf("creating base rules dir: %v", err)
	}
	if err := os.MkdirAll(customRulesDir, 0755); err != nil {
		t.Fatalf("creating custom rules dir: %v", err)
	}

	// base version
	if err := os.WriteFile(filepath.Join(baseRulesDir, "override-test.md"),
		[]byte("SENTINEL_BASE_OVERRIDE_SHOULD_NOT_APPEAR"), 0644); err != nil {
		t.Fatalf("writing base rule: %v", err)
	}
	// custom version (should win)
	if err := os.WriteFile(filepath.Join(customRulesDir, "override-test.md"),
		[]byte("SENTINEL_CUSTOM_OVERRIDE_9911"), 0644); err != nil {
		t.Fatalf("writing custom rule: %v", err)
	}

	scriptPath, assertionFile := createRealisticMock(t, dir, "custom-override", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:         "WOLFCASTLE_COMPLETE",
				ExpectInPrompt: []string{"SENTINEL_CUSTOM_OVERRIDE_9911"},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "custom-override-test")
	run(t, dir, "task", "add", "--node", "custom-override-test", "custom tier task")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	failures := readAssertionFailures(t, assertionFile)
	for _, f := range failures {
		t.Errorf("custom override assertion failure: %s", f)
	}

	// Also verify the base sentinel did NOT appear by reading the captured prompt
	promptDir := filepath.Join(wcDir, "mock-scripts", "custom-override-prompts")
	entries, err := os.ReadDir(promptDir)
	if err != nil {
		t.Fatalf("reading prompt dir: %v", err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(promptDir, e.Name()))
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "SENTINEL_BASE_OVERRIDE_SHOULD_NOT_APPEAR") {
			t.Error("base rule appeared in prompt despite custom override existing")
		}
	}
}

// Test 34: A rule in local/rules/ overrides same-named files in custom/rules/ and base/rules/.
func TestDaemon_PromptContainsLocalTierOverride(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	wcDir := filepath.Join(dir, ".wolfcastle")
	for _, tier := range []string{"base", "custom", "local"} {
		if err := os.MkdirAll(filepath.Join(wcDir, tier, "rules"), 0755); err != nil {
			t.Fatalf("creating %s rules dir: %v", tier, err)
		}
	}

	if err := os.WriteFile(filepath.Join(wcDir, "system", "base", "rules", "local-test.md"),
		[]byte("SENTINEL_BASE_LOCAL_SHOULD_NOT_APPEAR"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wcDir, "system", "custom", "rules", "local-test.md"),
		[]byte("SENTINEL_CUSTOM_LOCAL_SHOULD_NOT_APPEAR"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wcDir, "system", "local", "rules", "local-test.md"),
		[]byte("SENTINEL_LOCAL_OVERRIDE_5533"), 0644); err != nil {
		t.Fatal(err)
	}

	scriptPath, assertionFile := createRealisticMock(t, dir, "local-override", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:         "WOLFCASTLE_COMPLETE",
				ExpectInPrompt: []string{"SENTINEL_LOCAL_OVERRIDE_5533"},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "local-override-test")
	run(t, dir, "task", "add", "--node", "local-override-test", "local tier task")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	failures := readAssertionFailures(t, assertionFile)
	for _, f := range failures {
		t.Errorf("local override assertion failure: %s", f)
	}

	// Verify neither base nor custom sentinels leaked into the prompt
	promptDir := filepath.Join(wcDir, "mock-scripts", "local-override-prompts")
	entries, err := os.ReadDir(promptDir)
	if err != nil {
		t.Fatalf("reading prompt dir: %v", err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(promptDir, e.Name()))
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "SENTINEL_BASE_LOCAL_SHOULD_NOT_APPEAR") {
			t.Error("base rule appeared in prompt despite local override")
		}
		if strings.Contains(content, "SENTINEL_CUSTOM_LOCAL_SHOULD_NOT_APPEAR") {
			t.Error("custom rule appeared in prompt despite local override")
		}
	}
}

// Test 35: The prompt contains the script reference section (wolfcastle commands).
func TestDaemon_PromptContainsScriptReference(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, assertionFile := createRealisticMock(t, dir, "script-ref-check", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker: "WOLFCASTLE_COMPLETE",
				ExpectInPrompt: []string{
					"Wolfcastle Script Reference",
				},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "script-ref-test")
	run(t, dir, "task", "add", "--node", "script-ref-test", "check script ref")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	failures := readAssertionFailures(t, assertionFile)
	for _, f := range failures {
		t.Errorf("script reference assertion failure: %s", f)
	}
}

// Test 36: After a YIELD, the second invocation's prompt contains updated context
// (the breadcrumb from the first invocation).
func TestDaemon_PromptChangesBetweenIterations(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, assertionFile := createRealisticMock(t, dir, "prompt-changes", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:          "WOLFCASTLE_YIELD",
				WriteBreadcrumb: true,
				BreadcrumbText:  "SENTINEL_BREADCRUMB_FIRST_ITERATION_8899",
			},
			{
				Marker: "WOLFCASTLE_COMPLETE",
				ExpectInPrompt: []string{
					"SENTINEL_BREADCRUMB_FIRST_ITERATION_8899",
				},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "prompt-change-test")
	run(t, dir, "task", "add", "--node", "prompt-change-test", "evolving prompt task")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	failures := readAssertionFailures(t, assertionFile)
	for _, f := range failures {
		t.Errorf("prompt change assertion failure: %s", f)
	}
}

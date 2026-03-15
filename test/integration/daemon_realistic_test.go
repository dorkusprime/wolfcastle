//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestDaemon_RealisticComplete(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, assertionFile := createRealisticMock(t, dir, "realistic-complete", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:          "WOLFCASTLE_COMPLETE",
				CreateFiles:     map[string]string{"output.txt": "task output"},
				WriteBreadcrumb: true,
				BreadcrumbText:  "completed the task successfully",
				ExpectInPrompt:  []string{"do the thing"},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "realistic-test")
	run(t, dir, "task", "add", "--node", "realistic-test", "do the thing")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	// Verify the file was created
	createdFile := filepath.Join(dir, "output.txt")
	if _, err := os.Stat(createdFile); os.IsNotExist(err) {
		t.Error("expected output.txt to exist after daemon run")
	}

	// Verify breadcrumb was recorded via the marker system
	ns := loadNode(t, dir, "realistic-test")
	if len(ns.Audit.Breadcrumbs) == 0 {
		t.Error("expected at least one breadcrumb in node state")
	} else {
		found := false
		for _, bc := range ns.Audit.Breadcrumbs {
			if bc.Text == "completed the task successfully" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected breadcrumb text 'completed the task successfully', got %+v", ns.Audit.Breadcrumbs)
		}
	}

	// Verify task is complete
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusComplete {
				t.Errorf("expected task-1 complete, got %s", task.State)
			}
			break
		}
	}

	// Verify no assertion failures from prompt checking
	failures := readAssertionFailures(t, assertionFile)
	for _, f := range failures {
		t.Errorf("mock assertion failure: %s", f)
	}
}

func TestDaemon_RealisticYieldResume(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, _ := createRealisticMock(t, dir, "yield-resume", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:          "WOLFCASTLE_YIELD",
				CreateFiles:     map[string]string{"progress.txt": "started"},
				WriteBreadcrumb: true,
				BreadcrumbText:  "started work",
			},
			{
				Marker:          "WOLFCASTLE_COMPLETE",
				CreateFiles:     map[string]string{"progress.txt": "finished"},
				WriteBreadcrumb: true,
				BreadcrumbText:  "finished work",
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "yield-resume-test")
	run(t, dir, "task", "add", "--node", "yield-resume-test", "multi-step task")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	// Verify file has final content
	data, err := os.ReadFile(filepath.Join(dir, "progress.txt"))
	if err != nil {
		t.Fatalf("reading progress.txt: %v", err)
	}
	if string(data) != "finished" {
		t.Errorf("expected progress.txt content 'finished', got %q", string(data))
	}

	// Verify both breadcrumbs recorded
	ns := loadNode(t, dir, "yield-resume-test")
	if len(ns.Audit.Breadcrumbs) < 2 {
		t.Errorf("expected at least 2 breadcrumbs, got %d", len(ns.Audit.Breadcrumbs))
	}

	// Verify task is complete
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusComplete {
				t.Errorf("expected task-1 complete, got %s", task.State)
			}
			break
		}
	}
}

func TestDaemon_PromptContainsTaskInfo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, assertionFile := createRealisticMock(t, dir, "prompt-check", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker: "WOLFCASTLE_COMPLETE",
				ExpectInPrompt: []string{
					"implement the feature",
					"prompt-info-test",
				},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "prompt-info-test")
	run(t, dir, "task", "add", "--node", "prompt-info-test", "implement the feature")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	failures := readAssertionFailures(t, assertionFile)
	for _, f := range failures {
		t.Errorf("prompt assertion failure: %s", f)
	}
}

func TestDaemon_PromptContainsRuleFragments(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	// Write a custom rule fragment into base/rules/
	rulesDir := filepath.Join(dir, ".wolfcastle", "base", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatalf("creating rules dir: %v", err)
	}
	ruleContent := "CUSTOM_RULE_XYZ_SENTINEL: always test your code before committing"
	if err := os.WriteFile(filepath.Join(rulesDir, "custom-test-rule.md"), []byte(ruleContent), 0644); err != nil {
		t.Fatalf("writing custom rule: %v", err)
	}

	scriptPath, assertionFile := createRealisticMock(t, dir, "rule-check", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker: "WOLFCASTLE_COMPLETE",
				ExpectInPrompt: []string{
					"CUSTOM_RULE_XYZ_SENTINEL",
				},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "rule-frag-test")
	run(t, dir, "task", "add", "--node", "rule-frag-test", "task with rules")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	failures := readAssertionFailures(t, assertionFile)
	for _, f := range failures {
		t.Errorf("rule fragment assertion failure: %s", f)
	}
}

func TestDaemon_ModelCallsCLI(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	// Use CLI calls for breadcrumbs and gaps. The daemon reloads state
	// from disk after invocation, preserving CLI-driven mutations.
	scriptPath, _ := createRealisticMock(t, dir, "cli-calls", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:          "WOLFCASTLE_COMPLETE",
				WriteBreadcrumb: true,
				BreadcrumbText:  "model wrote this breadcrumb",
				WriteGap:        true,
				GapText:         "found a gap in error handling",
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "cli-call-test")
	run(t, dir, "task", "add", "--node", "cli-call-test", "call CLI commands")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	ns := loadNode(t, dir, "cli-call-test")

	// Verify breadcrumb
	breadcrumbFound := false
	for _, bc := range ns.Audit.Breadcrumbs {
		if bc.Text == "model wrote this breadcrumb" {
			breadcrumbFound = true
			break
		}
	}
	if !breadcrumbFound {
		t.Errorf("expected breadcrumb from marker output, got %+v", ns.Audit.Breadcrumbs)
	}

	// Verify gap
	if len(ns.Audit.Gaps) == 0 {
		t.Error("expected at least one gap recorded by marker output")
	} else {
		gapFound := false
		for _, g := range ns.Audit.Gaps {
			if g.Description == "found a gap in error handling" {
				gapFound = true
				break
			}
		}
		if !gapFound {
			t.Errorf("expected gap description 'found a gap in error handling', got %+v", ns.Audit.Gaps)
		}
	}
}

func TestDaemon_PartialFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, _ := createRealisticMock(t, dir, "partial-fail", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				// First invocation: create a file but emit no marker (failure)
				Marker:      "",
				CreateFiles: map[string]string{"side-effect.txt": "persisted"},
			},
			{
				// Second invocation: complete successfully
				Marker: "WOLFCASTLE_COMPLETE",
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	// High hard cap so we don't auto-block, enough iterations for retry
	setFailureAndIterationConfig(t, dir, 10, 0, 50, 20)

	run(t, dir, "project", "create", "partial-fail-test")
	run(t, dir, "task", "add", "--node", "partial-fail-test", "partially failing task")

	run(t, dir, "start")

	// Verify side-effect file persisted from the failed invocation
	data, err := os.ReadFile(filepath.Join(dir, "side-effect.txt"))
	if err != nil {
		t.Fatalf("reading side-effect.txt: %v", err)
	}
	if string(data) != "persisted" {
		t.Errorf("expected side-effect.txt content 'persisted', got %q", string(data))
	}

	// Verify task ultimately completed
	ns := loadNode(t, dir, "partial-fail-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusComplete {
				t.Errorf("expected task-1 complete after recovery, got %s", task.State)
			}
			// The failure count should reflect the failed invocation
			if task.FailureCount < 1 {
				t.Errorf("expected failure count >= 1, got %d", task.FailureCount)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

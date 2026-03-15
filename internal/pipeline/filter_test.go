package pipeline

import (
	"strings"
	"testing"
)

const testScriptRef = `# Wolfcastle Script Reference

All commands accept --json for structured output.

---

## Task Commands

Manage tasks on leaf nodes.

### wolfcastle task add

Add a new task to a leaf node.

### wolfcastle task claim

Claim a task.

### wolfcastle task complete

Complete a task.

### wolfcastle task block

Block a task.

---

## Project Commands

Create and organize project nodes.

### wolfcastle project create

Create a new project node.

---

## Audit Commands

Record progress and escalate issues.

### wolfcastle audit breadcrumb

Add a breadcrumb entry.

### wolfcastle audit escalate

Escalate a gap or issue.

## Audit Review Commands

Review findings from audit run.

### wolfcastle audit pending

Show pending audit findings.

### wolfcastle audit approve

Approve a finding.

---

## Status

### wolfcastle status

Show the current state of the project tree.
`

func TestFilterScriptReference_FiltersCommands(t *testing.T) {
	t.Parallel()

	result := FilterScriptReference(testScriptRef, []string{"project create", "task add"})

	// Allowed commands present
	if !strings.Contains(result, "### wolfcastle project create") {
		t.Error("expected project create to be present")
	}
	if !strings.Contains(result, "### wolfcastle task add") {
		t.Error("expected task add to be present")
	}

	// Forbidden commands absent
	for _, cmd := range []string{"task claim", "task complete", "task block", "audit breadcrumb", "audit escalate", "audit pending", "audit approve", "status"} {
		if strings.Contains(result, "### wolfcastle "+cmd) {
			t.Errorf("expected %q to be filtered out", cmd)
		}
	}
}

func TestFilterScriptReference_EmptyAllowedIncludesAll(t *testing.T) {
	t.Parallel()

	result := FilterScriptReference(testScriptRef, nil)
	if result != testScriptRef {
		t.Error("nil allowed should return input unchanged")
	}

	result = FilterScriptReference(testScriptRef, []string{})
	if result != testScriptRef {
		t.Error("empty allowed should return input unchanged")
	}
}

func TestFilterScriptReference_RemovesEmptySections(t *testing.T) {
	t.Parallel()

	// Allow only "project create", so Task Commands and Audit sections should vanish
	result := FilterScriptReference(testScriptRef, []string{"project create"})

	if strings.Contains(result, "## Task Commands") {
		t.Error("Task Commands section should be removed (no commands survived)")
	}
	if strings.Contains(result, "## Audit Commands") {
		t.Error("Audit Commands section should be removed")
	}
	if strings.Contains(result, "## Audit Review Commands") {
		t.Error("Audit Review Commands section should be removed")
	}
	if strings.Contains(result, "## Status") {
		t.Error("Status section should be removed")
	}
	if !strings.Contains(result, "## Project Commands") {
		t.Error("Project Commands section should be kept")
	}
}

func TestFilterScriptReference_PreservesPreamble(t *testing.T) {
	t.Parallel()

	result := FilterScriptReference(testScriptRef, []string{"status"})

	if !strings.Contains(result, "# Wolfcastle Script Reference") {
		t.Error("preamble title should be preserved")
	}
	if !strings.Contains(result, "All commands accept --json") {
		t.Error("preamble body should be preserved")
	}
}

func TestFilterScriptReference_KeepsSectionWithPartialCommands(t *testing.T) {
	t.Parallel()

	// Allow only task add from the Task Commands section
	result := FilterScriptReference(testScriptRef, []string{"task add"})

	if !strings.Contains(result, "## Task Commands") {
		t.Error("Task Commands section should be kept (task add survived)")
	}
	if !strings.Contains(result, "### wolfcastle task add") {
		t.Error("task add should be present")
	}
	if strings.Contains(result, "### wolfcastle task claim") {
		t.Error("task claim should be filtered out")
	}
}

package pipeline

import (
	"strings"
	"testing"
)

func TestGenerateScriptReference_ReturnsNonEmpty(t *testing.T) {
	t.Parallel()
	ref := GenerateScriptReference()
	if ref == "" {
		t.Fatal("expected non-empty script reference")
	}
}

func TestGenerateScriptReference_ContainsExpectedCommands(t *testing.T) {
	t.Parallel()
	ref := GenerateScriptReference()

	expected := []string{
		"wolfcastle task add",
		"wolfcastle task claim",
		"wolfcastle task complete",
		"wolfcastle task block",
		"wolfcastle project create",
		"wolfcastle audit breadcrumb",
		"wolfcastle audit escalate",
		"wolfcastle navigate",
		"wolfcastle status",
		"wolfcastle spec create",
		"wolfcastle spec link",
		"wolfcastle spec list",
		"wolfcastle adr create",
		"wolfcastle archive add",
		"wolfcastle inbox add",
		"wolfcastle audit pending",
		"wolfcastle audit approve",
		"wolfcastle audit reject",
		"wolfcastle audit history",
	}

	for _, cmd := range expected {
		if !strings.Contains(ref, cmd) {
			t.Errorf("expected script reference to contain %q", cmd)
		}
	}
}

func TestGenerateScriptReference_ContainsTitle(t *testing.T) {
	t.Parallel()
	ref := GenerateScriptReference()
	if !strings.Contains(ref, "# Wolfcastle Script Reference") {
		t.Error("expected script reference to start with title header")
	}
}

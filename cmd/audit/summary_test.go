package audit

import (
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestSummary_Success(t *testing.T) {
	env := newTestEnv(t)
	setupNode(t, env, "my-project", state.NodeLeaf)

	env.RootCmd.SetArgs([]string{"audit", "summary", "--node", "my-project", "Implemented JWT auth with full test coverage"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("summary failed: %v", err)
	}

	ns := loadNode(t, env, "my-project")
	if ns.Audit.ResultSummary != "Implemented JWT auth with full test coverage" {
		t.Errorf("expected summary text, got %q", ns.Audit.ResultSummary)
	}
}

func TestSummary_EmptyText(t *testing.T) {
	env := newTestEnv(t)
	setupNode(t, env, "my-project", state.NodeLeaf)

	env.RootCmd.SetArgs([]string{"audit", "summary", "--node", "my-project", "   "})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for empty summary text")
	}
}

func TestSummary_MissingNode(t *testing.T) {
	env := newTestEnv(t)
	setupNode(t, env, "my-project", state.NodeLeaf)

	env.RootCmd.SetArgs([]string{"audit", "summary", "--node", "nonexistent", "some text"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for missing node")
	}
}

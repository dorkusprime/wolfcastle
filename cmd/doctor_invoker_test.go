package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/validate"
)

// mockInvoker implements invoke.Invoker with canned responses.
type mockInvoker struct {
	result *invoke.Result
	err    error
}

func (m *mockInvoker) Invoke(_ context.Context, _ config.ModelDef, _ string, _ string, _ io.Writer, _ invoke.LineCallback) (*invoke.Result, error) {
	return m.result, m.err
}

// ---------------------------------------------------------------------------
// TryModelAssistedFix: success path
// ---------------------------------------------------------------------------

func TestTryModelAssistedFix_MockInvoker_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a node on disk
	nodeDir := filepath.Join(dir, "fix-node")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("fix-node", "Fix Node", state.NodeLeaf)
	_ = state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)

	mock := &mockInvoker{
		result: &invoke.Result{
			Stdout:   `{"resolution":"complete","reason":"all tasks done"}`,
			ExitCode: 0,
		},
	}

	issue := validate.Issue{
		Node:        "fix-node",
		Category:    validate.CatInvalidStateValue,
		FixType:     validate.FixModelAssisted,
		Description: "Invalid state detected",
	}
	model := config.ModelDef{Command: "test", Args: []string{}}

	ok, err := validate.TryModelAssistedFix(context.Background(), mock, model, issue, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected fix to be applied")
	}

	// Verify the state was updated
	loaded, err := state.LoadNodeState(filepath.Join(nodeDir, "state.json"))
	if err != nil {
		t.Fatalf("loading updated node: %v", err)
	}
	if loaded.State != state.StatusComplete {
		t.Errorf("expected state 'complete', got %q", loaded.State)
	}
}

// ---------------------------------------------------------------------------
// TryModelAssistedFix: invocation error
// ---------------------------------------------------------------------------

func TestTryModelAssistedFix_MockInvoker_InvokeError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	nodeDir := filepath.Join(dir, "err-node")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("err-node", "Err Node", state.NodeLeaf)
	_ = state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)

	mock := &mockInvoker{
		result: nil,
		err:    fmt.Errorf("model process died"),
	}

	issue := validate.Issue{
		Node:        "err-node",
		Category:    validate.CatMultipleInProgress,
		FixType:     validate.FixModelAssisted,
		Description: "ambiguous state",
	}
	model := config.ModelDef{Command: "test", Args: []string{}}

	ok, err := validate.TryModelAssistedFix(context.Background(), mock, model, issue, dir)
	if ok {
		t.Error("expected ok=false for invocation error")
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "model invocation failed") {
		t.Errorf("expected 'model invocation failed', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TryModelAssistedFix: invalid JSON response
// ---------------------------------------------------------------------------

func TestTryModelAssistedFix_MockInvoker_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	nodeDir := filepath.Join(dir, "json-node")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("json-node", "JSON Node", state.NodeLeaf)
	_ = state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)

	mock := &mockInvoker{
		result: &invoke.Result{
			Stdout:   "this is definitely not json",
			ExitCode: 0,
		},
	}

	issue := validate.Issue{
		Node:        "json-node",
		Category:    validate.CatInvalidStateValue,
		FixType:     validate.FixModelAssisted,
		Description: "bad state",
	}
	model := config.ModelDef{Command: "test", Args: []string{}}

	ok, err := validate.TryModelAssistedFix(context.Background(), mock, model, issue, dir)
	if ok {
		t.Error("expected ok=false for invalid JSON")
	}
	if err == nil {
		t.Fatal("expected parsing error")
	}
	if !strings.Contains(err.Error(), "parsing model response") {
		t.Errorf("expected 'parsing model response', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TryModelAssistedFix: invalid resolution value from model
// ---------------------------------------------------------------------------

func TestTryModelAssistedFix_MockInvoker_InvalidResolution(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	nodeDir := filepath.Join(dir, "bad-res")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("bad-res", "Bad Resolution", state.NodeLeaf)
	_ = state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)

	mock := &mockInvoker{
		result: &invoke.Result{
			Stdout:   `{"resolution":"exploded","reason":"model hallucinated"}`,
			ExitCode: 0,
		},
	}

	issue := validate.Issue{
		Node:        "bad-res",
		Category:    validate.CatInvalidStateValue,
		FixType:     validate.FixModelAssisted,
		Description: "invalid state",
	}
	model := config.ModelDef{Command: "test", Args: []string{}}

	ok, err := validate.TryModelAssistedFix(context.Background(), mock, model, issue, dir)
	if ok {
		t.Error("expected ok=false for invalid resolution")
	}
	if err == nil {
		t.Fatal("expected error for invalid resolution")
	}
	if !strings.Contains(err.Error(), "invalid resolution") {
		t.Errorf("expected 'invalid resolution', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TryModelAssistedFix: blocked resolution
// ---------------------------------------------------------------------------

func TestTryModelAssistedFix_MockInvoker_BlockedResolution(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	nodeDir := filepath.Join(dir, "blocked-node")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("blocked-node", "Blocked", state.NodeLeaf)
	_ = state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)

	mock := &mockInvoker{
		result: &invoke.Result{
			Stdout:   `{"resolution":"blocked","reason":"waiting on dependency"}`,
			ExitCode: 0,
		},
	}

	issue := validate.Issue{
		Node:        "blocked-node",
		Category:    validate.CatInvalidStateValue,
		FixType:     validate.FixModelAssisted,
		Description: "ambiguous state",
	}
	model := config.ModelDef{Command: "test", Args: []string{}}

	ok, err := validate.TryModelAssistedFix(context.Background(), mock, model, issue, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected fix to be applied")
	}

	loaded, err := state.LoadNodeState(filepath.Join(nodeDir, "state.json"))
	if err != nil {
		t.Fatalf("loading node: %v", err)
	}
	if loaded.State != state.StatusBlocked {
		t.Errorf("expected blocked, got %q", loaded.State)
	}
}

// ---------------------------------------------------------------------------
// TryModelAssistedFix: empty node address
// ---------------------------------------------------------------------------

func TestTryModelAssistedFix_MockInvoker_EmptyNode(t *testing.T) {
	t.Parallel()

	mock := &mockInvoker{
		result: &invoke.Result{Stdout: `{"resolution":"complete","reason":"ok"}`},
	}

	issue := validate.Issue{
		Node:     "",
		Category: validate.CatInvalidStateValue,
		FixType:  validate.FixModelAssisted,
	}
	model := config.ModelDef{Command: "test"}

	ok, err := validate.TryModelAssistedFix(context.Background(), mock, model, issue, t.TempDir())
	if ok {
		t.Error("expected ok=false for empty node")
	}
	if err == nil || !strings.Contains(err.Error(), "node address") {
		t.Errorf("expected 'node address' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Doctor command: model-assisted fix integration with mock invoker
// ---------------------------------------------------------------------------

func TestDoctorCmd_ModelAssistedFix_WithMockInvoker(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	_ = app.Config.WriteCustom(map[string]any{
		"doctor": map[string]any{"model": "doctor-model"},
		"models": map[string]any{
			"doctor-model": map[string]any{"command": "test-doctor", "args": []any{}},
		},
	})

	// Set up mock invoker that returns a valid fix
	mock := &mockInvoker{
		result: &invoke.Result{
			Stdout:   `{"resolution":"not_started","reason":"reset to initial"}`,
			ExitCode: 0,
		},
	}
	app.Invoker = mock

	// Run doctor with --fix (even though there may be no model-assisted issues,
	// this verifies the wiring doesn't panic with the mock invoker)
	rootCmd.SetArgs([]string{"doctor", "--fix"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doctor --fix failed: %v", err)
	}
}

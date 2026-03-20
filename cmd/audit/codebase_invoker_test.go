package audit

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// mockInvoker implements invoke.Invoker with canned responses for testing.
type mockInvoker struct {
	result *invoke.Result
	err    error
	// captured fields for verification
	capturedPrompt string
	capturedModel  config.ModelDef
}

func (m *mockInvoker) Invoke(_ context.Context, model config.ModelDef, prompt string, _ string, _ io.Writer, _ invoke.LineCallback) (*invoke.Result, error) {
	m.capturedPrompt = prompt
	m.capturedModel = model
	return m.result, m.err
}

func newInvokerTestEnv(t *testing.T) *testEnv {
	t.Helper()
	env := newTestEnv(t)

	// Set up an audit model in config on disk
	cfg, err := env.App.Config.Load()
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	cfg.Audit.Model = "test-model"
	cfg.Models["test-model"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"test"},
	}
	cfg.Daemon.InvocationTimeoutSeconds = 60

	if err := env.App.Config.WriteBase(cfg); err != nil {
		t.Fatalf("writing base config: %v", err)
	}

	return env
}

// ---------------------------------------------------------------------------
// runCodebaseAudit: full flow with mock invoker returning findings
// ---------------------------------------------------------------------------

func TestRunCodebaseAudit_FullFlow(t *testing.T) {
	t.Parallel()
	env := newInvokerTestEnv(t)

	// Create audit scope file
	baseAudits := filepath.Join(env.WolfcastleDir, "system", "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "security.md"), []byte("Check for vulnerabilities"), 0644)

	// Set up mock invoker with findings
	mock := &mockInvoker{
		result: &invoke.Result{
			Stdout: `## Authentication Bypass
User tokens are not validated properly.

## SQL Injection Risk
Parameterized queries not used in search endpoint.
`,
			ExitCode: 0,
		},
	}
	env.App.Invoker = mock

	scopes := []auditScope{
		{ID: "security", Description: "Security check", PromptFile: filepath.Join(baseAudits, "security.md")},
	}

	err := runCodebaseAudit(context.Background(), env.App, scopes)
	if err != nil {
		t.Fatalf("runCodebaseAudit failed: %v", err)
	}

	// Verify the prompt was sent to the invoker
	if mock.capturedPrompt == "" {
		t.Error("expected non-empty prompt to be captured")
	}
	if !strings.Contains(mock.capturedPrompt, "security") {
		t.Error("prompt should contain the scope content")
	}

	// Verify findings were saved
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	batch, err := state.LoadBatch(batchPath)
	if err != nil {
		t.Fatalf("loading batch: %v", err)
	}
	if batch == nil {
		t.Fatal("expected batch to be saved")
	}
	if len(batch.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(batch.Findings))
	}
	if batch.Findings[0].Title != "Authentication Bypass" {
		t.Errorf("unexpected first finding title: %s", batch.Findings[0].Title)
	}
	if batch.Findings[1].Title != "SQL Injection Risk" {
		t.Errorf("unexpected second finding title: %s", batch.Findings[1].Title)
	}
	if batch.Status != state.BatchPending {
		t.Errorf("expected batch status pending, got %s", batch.Status)
	}
	if len(batch.Scopes) != 1 || batch.Scopes[0] != "security" {
		t.Errorf("unexpected batch scopes: %v", batch.Scopes)
	}
}

// ---------------------------------------------------------------------------
// runCodebaseAudit: invocation error
// ---------------------------------------------------------------------------

func TestRunCodebaseAudit_InvokeError(t *testing.T) {
	t.Parallel()
	env := newInvokerTestEnv(t)

	baseAudits := filepath.Join(env.WolfcastleDir, "system", "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "perf.md"), []byte("Check performance"), 0644)

	mock := &mockInvoker{
		result: nil,
		err:    fmt.Errorf("connection refused"),
	}
	env.App.Invoker = mock

	scopes := []auditScope{
		{ID: "perf", Description: "Performance", PromptFile: filepath.Join(baseAudits, "perf.md")},
	}

	err := runCodebaseAudit(context.Background(), env.App, scopes)
	if err == nil {
		t.Fatal("expected error from invocation failure")
	}
	if !strings.Contains(err.Error(), "audit invocation failed") {
		t.Errorf("expected 'audit invocation failed', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runCodebaseAudit: non-zero exit code
// ---------------------------------------------------------------------------

func TestRunCodebaseAudit_NonZeroExitCode(t *testing.T) {
	t.Parallel()
	env := newInvokerTestEnv(t)

	baseAudits := filepath.Join(env.WolfcastleDir, "system", "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "test.md"), []byte("Audit"), 0644)

	mock := &mockInvoker{
		result: &invoke.Result{
			Stdout:   "",
			Stderr:   "model crashed",
			ExitCode: 1,
		},
	}
	env.App.Invoker = mock

	scopes := []auditScope{
		{ID: "test", Description: "Test", PromptFile: filepath.Join(baseAudits, "test.md")},
	}

	err := runCodebaseAudit(context.Background(), env.App, scopes)
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
	if !strings.Contains(err.Error(), "exited with code 1") {
		t.Errorf("expected exit code error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runCodebaseAudit: no findings parsed
// ---------------------------------------------------------------------------

func TestRunCodebaseAudit_NoFindings(t *testing.T) {
	t.Parallel()
	env := newInvokerTestEnv(t)

	baseAudits := filepath.Join(env.WolfcastleDir, "system", "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "clean.md"), []byte("Clean audit"), 0644)

	mock := &mockInvoker{
		result: &invoke.Result{
			Stdout:   "Everything looks great, no issues found.\n",
			ExitCode: 0,
		},
	}
	env.App.Invoker = mock

	scopes := []auditScope{
		{ID: "clean", Description: "Clean", PromptFile: filepath.Join(baseAudits, "clean.md")},
	}

	err := runCodebaseAudit(context.Background(), env.App, scopes)
	if err != nil {
		t.Fatalf("expected no error for clean audit, got: %v", err)
	}

	// No batch should be saved when there are no findings
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	batch, err := state.LoadBatch(batchPath)
	if err != nil {
		t.Fatalf("loading batch: %v", err)
	}
	if batch != nil && batch.Status == state.BatchPending && len(batch.Findings) > 0 {
		t.Error("expected no pending findings")
	}
}

// ---------------------------------------------------------------------------
// runCodebaseAudit: model not found
// ---------------------------------------------------------------------------

func TestRunCodebaseAudit_ModelNotFound(t *testing.T) {
	t.Parallel()
	env := newInvokerTestEnv(t)

	// Point to a model that doesn't exist and write to disk
	cfg, err := env.App.Config.Load()
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	cfg.Audit.Model = "nonexistent-model"
	if err := env.App.Config.WriteBase(cfg); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	scopes := []auditScope{
		{ID: "test", Description: "Test"},
	}

	err = runCodebaseAudit(context.Background(), env.App, scopes)
	if err == nil {
		t.Fatal("expected error for missing model")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runCodebaseAudit: multiple scopes combined into prompt
// ---------------------------------------------------------------------------

func TestRunCodebaseAudit_MultipleScopes(t *testing.T) {
	t.Parallel()
	env := newInvokerTestEnv(t)

	baseAudits := filepath.Join(env.WolfcastleDir, "system", "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "security.md"), []byte("Security checks"), 0644)
	_ = os.WriteFile(filepath.Join(baseAudits, "performance.md"), []byte("Performance checks"), 0644)

	mock := &mockInvoker{
		result: &invoke.Result{
			Stdout: `## Finding One
Description one.
`,
			ExitCode: 0,
		},
	}
	env.App.Invoker = mock

	scopes := []auditScope{
		{ID: "security", Description: "Security", PromptFile: filepath.Join(baseAudits, "security.md")},
		{ID: "performance", Description: "Performance", PromptFile: filepath.Join(baseAudits, "performance.md")},
	}

	err := runCodebaseAudit(context.Background(), env.App, scopes)
	if err != nil {
		t.Fatalf("runCodebaseAudit failed: %v", err)
	}

	// Verify both scopes appear in the prompt
	if !strings.Contains(mock.capturedPrompt, "## Scope: security") {
		t.Error("prompt should contain security scope header")
	}
	if !strings.Contains(mock.capturedPrompt, "## Scope: performance") {
		t.Error("prompt should contain performance scope header")
	}

	// Verify batch has both scopes
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	batch, err := state.LoadBatch(batchPath)
	if err != nil {
		t.Fatalf("loading batch: %v", err)
	}
	if len(batch.Scopes) != 2 {
		t.Errorf("expected 2 scopes in batch, got %d", len(batch.Scopes))
	}
}

// ---------------------------------------------------------------------------
// runCodebaseAudit: JSON output mode
// ---------------------------------------------------------------------------

func TestRunCodebaseAudit_JSONOutput(t *testing.T) {
	t.Parallel()
	env := newInvokerTestEnv(t)
	env.App.JSON = true

	baseAudits := filepath.Join(env.WolfcastleDir, "system", "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "test.md"), []byte("Test audit"), 0644)

	mock := &mockInvoker{
		result: &invoke.Result{
			Stdout: `## A Finding
Some description.
`,
			ExitCode: 0,
		},
	}
	env.App.Invoker = mock

	scopes := []auditScope{
		{ID: "test", Description: "Test", PromptFile: filepath.Join(baseAudits, "test.md")},
	}

	err := runCodebaseAudit(context.Background(), env.App, scopes)
	if err != nil {
		t.Fatalf("runCodebaseAudit with JSON output failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runCodebaseAudit: batch ID and timestamp
// ---------------------------------------------------------------------------

func TestRunCodebaseAudit_BatchMetadata(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	_ = os.MkdirAll(wcDir, 0755)

	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "test", Machine: "dev"}
	cfg.Audit.Model = "test-model"
	cfg.Models["test-model"] = config.ModelDef{Command: "echo", Args: []string{"test"}}

	ns := "test-dev"
	projDir := filepath.Join(wcDir, "system", "projects", ns)
	_ = os.MkdirAll(projDir, 0755)
	idx := state.NewRootIndex()
	saveJSON(t, filepath.Join(projDir, "state.json"), idx)

	fixedTime := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	mock := &mockInvoker{
		result: &invoke.Result{
			Stdout: `## Test Finding
Description.
`,
			ExitCode: 0,
		},
	}

	configRepo := config.NewConfigRepository(wcDir)
	if err := configRepo.WriteBase(cfg); err != nil {
		t.Fatalf("writing base config: %v", err)
	}

	testApp := &cmdutil.App{
		Config:  configRepo,
		Clock:   clock.NewFixed(fixedTime),
		Invoker: mock,
	}

	baseAudits := filepath.Join(wcDir, "system", "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "test.md"), []byte("Test"), 0644)

	scopes := []auditScope{
		{ID: "test", Description: "Test", PromptFile: filepath.Join(baseAudits, "test.md")},
	}

	err := runCodebaseAudit(context.Background(), testApp, scopes)
	if err != nil {
		t.Fatalf("runCodebaseAudit failed: %v", err)
	}

	batchPath := filepath.Join(wcDir, "audit-state.json")
	batch, err := state.LoadBatch(batchPath)
	if err != nil {
		t.Fatalf("loading batch: %v", err)
	}
	expectedID := "audit-20250615T103000Z"
	if batch.ID != expectedID {
		t.Errorf("expected batch ID %q, got %q", expectedID, batch.ID)
	}
	if !batch.Timestamp.Equal(fixedTime) {
		t.Errorf("expected timestamp %v, got %v", fixedTime, batch.Timestamp)
	}
	if batch.RawOutput == "" {
		t.Error("expected raw output to be preserved")
	}
}

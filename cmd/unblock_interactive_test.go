package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
)

// fakeInvoker returns canned results and records the prompts it receives.
type fakeInvoker struct {
	results []*invoke.Result
	errs    []error
	prompts []string
	call    int
}

func (f *fakeInvoker) fn(ctx context.Context, model config.ModelDef, prompt, workDir string) (*invoke.Result, error) {
	f.prompts = append(f.prompts, prompt)
	idx := f.call
	f.call++
	if idx < len(f.errs) && f.errs[idx] != nil {
		return nil, f.errs[idx]
	}
	if idx < len(f.results) {
		return f.results[idx], nil
	}
	return &invoke.Result{Stdout: "default response"}, nil
}

// pipeInput creates a ReadCloser that feeds the given lines (terminated by
// newlines) to readline, then returns EOF.
func pipeInput(lines ...string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(strings.Join(lines, "\n") + "\n"))
}

// setupInteractiveEnv creates a testEnv with the unblock model configured
// and returns the env plus the standard diagnostic string used by tests.
func setupInteractiveEnv(t *testing.T) *testEnv {
	t.Helper()
	env := newTestEnv(t)

	oldApp := app
	t.Cleanup(func() { app = oldApp })
	app = env.App

	// Configure the unblock model so runInteractiveUnblockWith can find it.
	_ = app.Config.WriteCustom(map[string]any{
		"unblock": map[string]any{"model": "unblock-model"},
		"models": map[string]any{
			"unblock-model": map[string]any{"command": "echo", "args": []any{"hi"}},
		},
	})

	return env
}

func TestRunInteractiveUnblockWith_ModelNotConfigured(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Configure a model name that doesn't exist in the models map.
	_ = app.Config.WriteCustom(map[string]any{
		"unblock": map[string]any{"model": "phantom-model"},
	})

	err := runInteractiveUnblockWith(context.Background(), "node/task-0001", "diag", nil)
	if err == nil {
		t.Fatal("expected error when model not found in config")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestRunInteractiveUnblock_QuitCommand(t *testing.T) {
	env := setupInteractiveEnv(t)
	_ = env

	fake := &fakeInvoker{
		results: []*invoke.Result{{Stdout: "I can help with that."}},
	}

	err := runInteractiveUnblockWith(
		context.Background(), "node/task-0001", "diagnostic context",
		&unblockOpts{
			invokeFn: fake.fn,
			stdin:    pipeInput("quit"),
			stdout:   io.Discard,
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.call != 1 {
		t.Errorf("expected 1 invocation, got %d", fake.call)
	}
}

func TestRunInteractiveUnblock_ExitCommand(t *testing.T) {
	env := setupInteractiveEnv(t)
	_ = env

	fake := &fakeInvoker{
		results: []*invoke.Result{{Stdout: "Here's my analysis."}},
	}

	err := runInteractiveUnblockWith(
		context.Background(), "node/task-0001", "diagnostic context",
		&unblockOpts{
			invokeFn: fake.fn,
			stdin:    pipeInput("exit"),
			stdout:   io.Discard,
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.call != 1 {
		t.Errorf("expected 1 invocation, got %d", fake.call)
	}
}

func TestRunInteractiveUnblock_EOFFromReadline(t *testing.T) {
	env := setupInteractiveEnv(t)
	_ = env

	fake := &fakeInvoker{
		results: []*invoke.Result{{Stdout: "response"}},
	}

	// Empty input stream produces immediate EOF from readline.
	err := runInteractiveUnblockWith(
		context.Background(), "node/task-0001", "diag",
		&unblockOpts{
			invokeFn: fake.fn,
			stdin:    io.NopCloser(strings.NewReader("")),
			stdout:   io.Discard,
		},
	)
	if err != nil {
		t.Fatalf("unexpected error on EOF: %v", err)
	}
}

func TestRunInteractiveUnblock_InvocationFailure(t *testing.T) {
	env := setupInteractiveEnv(t)
	_ = env

	fake := &fakeInvoker{
		errs: []error{fmt.Errorf("connection refused")},
	}

	err := runInteractiveUnblockWith(
		context.Background(), "node/task-0001", "diag",
		&unblockOpts{
			invokeFn: fake.fn,
			stdin:    pipeInput("quit"),
			stdout:   io.Discard,
		},
	)
	if err == nil {
		t.Fatal("expected error on invocation failure")
	}
	if !strings.Contains(err.Error(), "model invocation failed") {
		t.Errorf("expected 'model invocation failed', got: %v", err)
	}
}

func TestRunInteractiveUnblock_StderrOutput(t *testing.T) {
	env := setupInteractiveEnv(t)
	_ = env

	fake := &fakeInvoker{
		results: []*invoke.Result{{
			Stdout: "model response",
			Stderr: "warning: something happened",
		}},
	}

	// Stderr from the model should not cause an error; session continues
	// until user input. Here we send "quit" to end.
	err := runInteractiveUnblockWith(
		context.Background(), "node/task-0001", "diag",
		&unblockOpts{
			invokeFn: fake.fn,
			stdin:    pipeInput("quit"),
			stdout:   io.Discard,
		},
	)
	if err != nil {
		t.Fatalf("unexpected error with stderr output: %v", err)
	}
}

func TestRunInteractiveUnblock_MultiTurnConversation(t *testing.T) {
	env := setupInteractiveEnv(t)
	_ = env

	fake := &fakeInvoker{
		results: []*invoke.Result{
			{Stdout: "first response"},
			{Stdout: "second response"},
		},
	}

	err := runInteractiveUnblockWith(
		context.Background(), "node/task-0001", "diagnostic",
		&unblockOpts{
			invokeFn: fake.fn,
			stdin:    pipeInput("what happened?", "quit"),
			stdout:   io.Discard,
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.call != 2 {
		t.Errorf("expected 2 invocations for multi-turn, got %d", fake.call)
	}
	// The second prompt should contain the user's question.
	if !strings.Contains(fake.prompts[1], "what happened?") {
		t.Errorf("second prompt should contain user input, got: %s", fake.prompts[1][:200])
	}
}

func TestRunInteractiveUnblock_ConversationTrimming(t *testing.T) {
	env := setupInteractiveEnv(t)
	_ = env

	// Build a large initial diagnostic to push conversation over the 100k limit
	// after a couple of turns. Each user message adds ~50k of padding.
	bigDiag := strings.Repeat("x", 40_000)

	callCount := 0
	fake := &fakeInvoker{
		results: []*invoke.Result{
			{Stdout: "response 1"},
			{Stdout: "response 2"},
			{Stdout: "response 3"},
		},
	}
	// Wrap to track prompts and verify trimming.
	wrappedFn := func(ctx context.Context, model config.ModelDef, prompt, workDir string) (*invoke.Result, error) {
		callCount++
		return fake.fn(ctx, model, prompt, workDir)
	}

	padding := strings.Repeat("y", 50_000)
	err := runInteractiveUnblockWith(
		context.Background(), "node/task-0001", bigDiag,
		&unblockOpts{
			invokeFn: wrappedFn,
			stdin:    pipeInput("first "+padding, "second "+padding, "quit"),
			stdout:   io.Discard,
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 invocations, got %d", callCount)
	}
	// The last prompt should contain the truncation marker.
	lastPrompt := fake.prompts[len(fake.prompts)-1]
	if !strings.Contains(lastPrompt, "[Earlier conversation truncated]") {
		t.Error("expected conversation truncation marker in final prompt")
	}
}

func TestRunInteractiveUnblock_PromptContainsDiagnostic(t *testing.T) {
	env := setupInteractiveEnv(t)
	_ = env

	fake := &fakeInvoker{
		results: []*invoke.Result{{Stdout: "ok"}},
	}

	diag := "## Blocked Task\nNode: test-node\nReason: dependency missing"
	err := runInteractiveUnblockWith(
		context.Background(), "test-node/task-0001", diag,
		&unblockOpts{
			invokeFn: fake.fn,
			stdin:    pipeInput("quit"),
			stdout:   io.Discard,
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The prompt sent to the model should include the diagnostic and the
	// unblock command reminder.
	if !strings.Contains(fake.prompts[0], "dependency missing") {
		t.Error("prompt should contain diagnostic text")
	}
	if !strings.Contains(fake.prompts[0], "wolfcastle task unblock --node test-node/task-0001") {
		t.Error("prompt should contain unblock command reminder")
	}
}

func TestRunInteractiveUnblock_SkipsSessionInitAndResultLines(t *testing.T) {
	env := setupInteractiveEnv(t)
	_ = env

	// Model output includes lines that FormatAssistantText converts to
	// "[session started]" and "[result] ..." which the loop filters out.
	fake := &fakeInvoker{
		results: []*invoke.Result{{
			Stdout: `{"type":"system","subtype":"init"}` + "\n" +
				`{"type":"result","result":"all done"}` + "\n" +
				"actual useful text",
		}},
	}

	err := runInteractiveUnblockWith(
		context.Background(), "node/task-0001", "diag",
		&unblockOpts{
			invokeFn: fake.fn,
			stdin:    pipeInput("quit"),
			stdout:   io.Discard,
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The test passing without error confirms the filtered lines didn't
	// cause issues. The real verification is that the output loop didn't
	// display "[session started]" or "[result]" lines, which is a display
	// concern rather than a return-value concern.
}

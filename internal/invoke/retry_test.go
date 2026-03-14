package invoke

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
)

// mockInvoker is a test double that returns pre-configured results.
type mockInvoker struct {
	results []mockCall
	callIdx int
}

type mockCall struct {
	result *Result
	err    error
}

func (m *mockInvoker) Invoke(_ context.Context, _ config.ModelDef, _ string, _ string, _ io.Writer, _ LineCallback) (*Result, error) {
	if m.callIdx >= len(m.results) {
		return nil, fmt.Errorf("unexpected call %d", m.callIdx)
	}
	call := m.results[m.callIdx]
	m.callIdx++
	return call.result, call.err
}

// mockRetryLogger captures retry events for assertion.
type mockRetryLogger struct {
	retries   []retryEvent
	exhausted *exhaustedEvent
}

type retryEvent struct {
	attempt int
	delay   time.Duration
	err     error
}

type exhaustedEvent struct {
	totalAttempts int
	lastErr       error
}

func (l *mockRetryLogger) OnRetry(attempt int, delay time.Duration, err error) {
	l.retries = append(l.retries, retryEvent{attempt: attempt, delay: delay, err: err})
}

func (l *mockRetryLogger) OnExhausted(totalAttempts int, lastErr error) {
	l.exhausted = &exhaustedEvent{totalAttempts: totalAttempts, lastErr: lastErr}
}

func defaultRetryConfig() config.RetriesConfig {
	return config.RetriesConfig{
		InitialDelaySeconds: 1,
		MaxDelaySeconds:     10,
		MaxRetries:          3,
	}
}

// noSleep is a sleep function that does nothing, for fast tests.
func noSleep(_ time.Duration) {}

func TestRetryInvoker_SuccessOnFirstAttempt(t *testing.T) {
	inner := &mockInvoker{
		results: []mockCall{
			{result: &Result{Stdout: "ok"}, err: nil},
		},
	}
	logger := &mockRetryLogger{}
	ri := NewRetryInvoker(inner, defaultRetryConfig(), logger)
	ri.SleepFunc = noSleep

	result, err := ri.Invoke(context.Background(), config.ModelDef{}, "", "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "ok" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "ok")
	}
	if len(logger.retries) != 0 {
		t.Errorf("expected no retries, got %d", len(logger.retries))
	}
	if inner.callIdx != 1 {
		t.Errorf("inner called %d times, want 1", inner.callIdx)
	}
}

func TestRetryInvoker_SuccessAfterRetries(t *testing.T) {
	inner := &mockInvoker{
		results: []mockCall{
			{result: nil, err: fmt.Errorf("timeout")},
			{result: nil, err: fmt.Errorf("rate limit")},
			{result: &Result{Stdout: "ok"}, err: nil},
		},
	}
	logger := &mockRetryLogger{}
	ri := NewRetryInvoker(inner, defaultRetryConfig(), logger)
	ri.SleepFunc = noSleep

	result, err := ri.Invoke(context.Background(), config.ModelDef{}, "", "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "ok" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "ok")
	}
	if len(logger.retries) != 2 {
		t.Errorf("expected 2 retries, got %d", len(logger.retries))
	}
	if inner.callIdx != 3 {
		t.Errorf("inner called %d times, want 3", inner.callIdx)
	}
}

func TestRetryInvoker_ExhaustedRetries(t *testing.T) {
	inner := &mockInvoker{
		results: []mockCall{
			{result: nil, err: fmt.Errorf("error 1")},
			{result: nil, err: fmt.Errorf("error 2")},
			{result: nil, err: fmt.Errorf("error 3")},
			{result: nil, err: fmt.Errorf("error 4")},
		},
	}
	logger := &mockRetryLogger{}
	ri := NewRetryInvoker(inner, defaultRetryConfig(), logger)
	ri.SleepFunc = noSleep

	_, err := ri.Invoke(context.Background(), config.ModelDef{}, "", "", nil, nil)
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	if logger.exhausted == nil {
		t.Fatal("expected OnExhausted to be called")
	}
	if logger.exhausted.totalAttempts != 4 {
		t.Errorf("total attempts = %d, want 4", logger.exhausted.totalAttempts)
	}
	if len(logger.retries) != 3 {
		t.Errorf("expected 3 retries, got %d", len(logger.retries))
	}
}

func TestRetryInvoker_UnlimitedRetries(t *testing.T) {
	cfg := config.RetriesConfig{
		InitialDelaySeconds: 1,
		MaxDelaySeconds:     10,
		MaxRetries:          -1, // unlimited
	}

	// Fail 10 times then succeed.
	calls := make([]mockCall, 11)
	for i := 0; i < 10; i++ {
		calls[i] = mockCall{result: nil, err: fmt.Errorf("error %d", i)}
	}
	calls[10] = mockCall{result: &Result{Stdout: "finally"}, err: nil}

	inner := &mockInvoker{results: calls}
	logger := &mockRetryLogger{}
	ri := NewRetryInvoker(inner, cfg, logger)
	ri.SleepFunc = noSleep

	result, err := ri.Invoke(context.Background(), config.ModelDef{}, "", "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "finally" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "finally")
	}
	if len(logger.retries) != 10 {
		t.Errorf("expected 10 retries, got %d", len(logger.retries))
	}
}

func TestRetryInvoker_ExponentialBackoff(t *testing.T) {
	cfg := config.RetriesConfig{
		InitialDelaySeconds: 2,
		MaxDelaySeconds:     16,
		MaxRetries:          -1,
	}

	calls := make([]mockCall, 6)
	for i := 0; i < 5; i++ {
		calls[i] = mockCall{result: nil, err: fmt.Errorf("error")}
	}
	calls[5] = mockCall{result: &Result{Stdout: "ok"}, err: nil}

	inner := &mockInvoker{results: calls}
	logger := &mockRetryLogger{}

	var sleepDelays []time.Duration
	ri := &RetryInvoker{
		Inner:  inner,
		Config: cfg,
		Logger: logger,
		SleepFunc: func(d time.Duration) {
			sleepDelays = append(sleepDelays, d)
		},
	}

	_, err := ri.Invoke(context.Background(), config.ModelDef{}, "", "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected delays: 2s, 4s, 8s, 16s, 16s (capped at max)
	expectedDelays := []time.Duration{
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		16 * time.Second, // capped
	}
	if len(sleepDelays) != len(expectedDelays) {
		t.Fatalf("got %d sleep calls, want %d", len(sleepDelays), len(expectedDelays))
	}
	for i, want := range expectedDelays {
		if sleepDelays[i] != want {
			t.Errorf("sleep[%d] = %v, want %v", i, sleepDelays[i], want)
		}
	}
}

func TestRetryInvoker_ContextCancelledNoRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	inner := &mockInvoker{
		results: []mockCall{
			{result: nil, err: fmt.Errorf("some error")},
		},
	}
	logger := &mockRetryLogger{}
	ri := NewRetryInvoker(inner, defaultRetryConfig(), logger)
	ri.SleepFunc = noSleep

	_, err := ri.Invoke(ctx, config.ModelDef{}, "", "", nil, nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	// Should not retry.
	if inner.callIdx != 1 {
		t.Errorf("inner called %d times, want 1 (no retry for cancelled ctx)", inner.callIdx)
	}
	if len(logger.retries) != 0 {
		t.Errorf("expected no retries, got %d", len(logger.retries))
	}
}

func TestRetryInvoker_NonZeroExitCodeNotRetried(t *testing.T) {
	inner := &mockInvoker{
		results: []mockCall{
			{result: &Result{ExitCode: 1, Stdout: "error output"}, err: nil},
		},
	}
	logger := &mockRetryLogger{}
	ri := NewRetryInvoker(inner, defaultRetryConfig(), logger)
	ri.SleepFunc = noSleep

	result, err := ri.Invoke(context.Background(), config.ModelDef{}, "", "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
	if inner.callIdx != 1 {
		t.Errorf("inner called %d times, want 1 (non-zero exit is not retried)", inner.callIdx)
	}
}

func TestRetryInvoker_NilLogger(t *testing.T) {
	inner := &mockInvoker{
		results: []mockCall{
			{result: nil, err: fmt.Errorf("error")},
			{result: &Result{Stdout: "ok"}, err: nil},
		},
	}
	ri := NewRetryInvoker(inner, defaultRetryConfig(), nil)
	ri.SleepFunc = noSleep

	result, err := ri.Invoke(context.Background(), config.ModelDef{}, "", "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "ok" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "ok")
	}
}

func TestRetryInvoker_NilLoggerExhausted(t *testing.T) {
	cfg := config.RetriesConfig{
		InitialDelaySeconds: 1,
		MaxDelaySeconds:     10,
		MaxRetries:          0, // zero retries = fail on first error
	}
	inner := &mockInvoker{
		results: []mockCall{
			{result: nil, err: fmt.Errorf("error")},
		},
	}
	ri := NewRetryInvoker(inner, cfg, nil)
	ri.SleepFunc = noSleep

	_, err := ri.Invoke(context.Background(), config.ModelDef{}, "", "", nil, nil)
	if err == nil {
		t.Fatal("expected error with zero max retries")
	}
}

func TestRetryInvoker_MaxRetriesZero(t *testing.T) {
	cfg := config.RetriesConfig{
		InitialDelaySeconds: 1,
		MaxDelaySeconds:     10,
		MaxRetries:          0,
	}
	inner := &mockInvoker{
		results: []mockCall{
			{result: nil, err: fmt.Errorf("error")},
		},
	}
	logger := &mockRetryLogger{}
	ri := NewRetryInvoker(inner, cfg, logger)
	ri.SleepFunc = noSleep

	_, err := ri.Invoke(context.Background(), config.ModelDef{}, "", "", nil, nil)
	if err == nil {
		t.Fatal("expected error with zero max retries")
	}
	if logger.exhausted == nil {
		t.Fatal("expected OnExhausted")
	}
	if logger.exhausted.totalAttempts != 1 {
		t.Errorf("total attempts = %d, want 1", logger.exhausted.totalAttempts)
	}
}

func TestRetryInvoker_LoggerReceivesDelays(t *testing.T) {
	cfg := config.RetriesConfig{
		InitialDelaySeconds: 5,
		MaxDelaySeconds:     20,
		MaxRetries:          3,
	}
	calls := make([]mockCall, 4)
	for i := range calls {
		calls[i] = mockCall{result: nil, err: fmt.Errorf("error %d", i)}
	}
	inner := &mockInvoker{results: calls}
	logger := &mockRetryLogger{}
	ri := NewRetryInvoker(inner, cfg, logger)
	ri.SleepFunc = noSleep

	ri.Invoke(context.Background(), config.ModelDef{}, "", "", nil, nil)

	if len(logger.retries) != 3 {
		t.Fatalf("got %d retries, want 3", len(logger.retries))
	}

	// Check that logged delays match expected exponential backoff.
	expectedDelays := []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second}
	for i, want := range expectedDelays {
		if logger.retries[i].delay != want {
			t.Errorf("retry[%d].delay = %v, want %v", i, logger.retries[i].delay, want)
		}
	}
}

func TestIsRetryableError(t *testing.T) {
	if IsRetryableError(nil) {
		t.Error("nil error should not be retryable")
	}
	if !IsRetryableError(fmt.Errorf("connection timeout")) {
		t.Error("generic error should be retryable")
	}
}

// --- Integration-style test: RetryInvoker wrapping ProcessInvoker ---

func TestRetryInvoker_WithProcessInvoker(t *testing.T) {
	inner := NewProcessInvoker()
	cfg := config.RetriesConfig{
		InitialDelaySeconds: 1,
		MaxDelaySeconds:     2,
		MaxRetries:          1,
	}
	ri := NewRetryInvoker(inner, cfg, nil)
	ri.SleepFunc = noSleep

	// Successful invocation should work.
	result, err := ri.Invoke(context.Background(), echoModel("integrated"), "", ".", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout == "" {
		t.Error("expected non-empty stdout")
	}
}

// --- RetryInvoker satisfies Invoker interface ---

func TestRetryInvoker_ImplementsInvoker(t *testing.T) {
	var _ Invoker = &RetryInvoker{}
}

func TestProcessInvoker_ImplementsInvoker(t *testing.T) {
	var _ Invoker = &ProcessInvoker{}
}

func TestIsRetryableError_ContextCanceled(t *testing.T) {
	if IsRetryableError(context.Canceled) {
		t.Error("context.Canceled should not be retryable")
	}
}

func TestIsRetryableError_DeadlineExceeded(t *testing.T) {
	if IsRetryableError(context.DeadlineExceeded) {
		t.Error("context.DeadlineExceeded should not be retryable")
	}
}

func TestIsRetryableError_WrappedContextCanceled(t *testing.T) {
	wrapped := fmt.Errorf("operation failed: %w", context.Canceled)
	if IsRetryableError(wrapped) {
		t.Error("wrapped context.Canceled should not be retryable")
	}
}

func TestRetryInvoker_ContextCancelledDuringSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	inner := &mockInvoker{
		results: []mockCall{
			{result: nil, err: fmt.Errorf("error")},
			{result: nil, err: fmt.Errorf("error")}, // should not be reached
		},
	}
	cfg := config.RetriesConfig{
		InitialDelaySeconds: 1,
		MaxDelaySeconds:     10,
		MaxRetries:          -1,
	}
	ri := NewRetryInvoker(inner, cfg, nil)
	ri.SleepFunc = func(_ time.Duration) {
		callCount++
		cancel() // cancel during sleep
	}

	_, err := ri.Invoke(ctx, config.ModelDef{}, "", "", nil, nil)
	if err == nil {
		t.Fatal("expected error after context cancel during sleep")
	}
	if callCount != 1 {
		t.Errorf("sleep called %d times, want 1", callCount)
	}
}

func TestRetryInvoker_DefaultSleepFunc(t *testing.T) {
	// Verify that nil SleepFunc defaults to time.Sleep (doesn't panic).
	inner := &mockInvoker{
		results: []mockCall{
			{result: &Result{Stdout: "ok"}, err: nil},
		},
	}
	ri := NewRetryInvoker(inner, defaultRetryConfig(), nil)
	// Do NOT set SleepFunc — exercise the nil branch.
	result, err := ri.Invoke(context.Background(), config.ModelDef{}, "", "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "ok" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "ok")
	}
}

package invoke

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
)

func TestStallDetection_KillsStalledProcess(t *testing.T) {
	// A process that prints one line then sleeps forever. The stall
	// detector should kill it well before the sleep completes.
	inv := &ProcessInvoker{
		StallTimeout: 500 * time.Millisecond,
	}
	model := config.ModelDef{
		Command: "sh",
		Args:    []string{"-c", "echo 'hello'; sleep 30"},
	}

	var logBuf bytes.Buffer
	start := time.Now()
	_, err := inv.Invoke(context.Background(), model, "", ".", &logBuf, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected ErrStallTimeout, got nil")
	}
	if !errors.Is(err, ErrStallTimeout) {
		t.Fatalf("expected ErrStallTimeout, got: %v", err)
	}

	// Should have been killed well under the 30s sleep.
	if elapsed > 10*time.Second {
		t.Errorf("stall detection took %v, expected under 10s", elapsed)
	}
}

func TestStallDetection_ActiveOutputResetsTimer(t *testing.T) {
	// A process that prints a line every 200ms for 1.5s. With a 500ms
	// stall timeout, the process should complete without being killed
	// because each line resets the timer.
	inv := &ProcessInvoker{
		StallTimeout: 500 * time.Millisecond,
	}
	model := config.ModelDef{
		Command: "sh",
		Args:    []string{"-c", "for i in 1 2 3 4 5 6 7; do echo \"line-$i\"; sleep 0.2; done"},
	}

	var logBuf bytes.Buffer
	result, err := inv.Invoke(context.Background(), model, "", ".", &logBuf, nil)

	if err != nil {
		t.Fatalf("expected no error (output was active), got: %v", err)
	}
	if !strings.Contains(result.Stdout, "line-7") {
		t.Errorf("expected all lines captured, got: %q", result.Stdout)
	}
}

func TestStallDetection_DisabledWhenZero(t *testing.T) {
	// With StallTimeout == 0, stall detection is disabled.
	// A short-lived process should complete normally.
	inv := &ProcessInvoker{
		StallTimeout: 0,
	}
	model := config.ModelDef{Command: "echo", Args: []string{"no-stall"}}

	var logBuf bytes.Buffer
	result, err := inv.Invoke(context.Background(), model, "", ".", &logBuf, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, "no-stall") {
		t.Errorf("stdout = %q, want to contain 'no-stall'", result.Stdout)
	}
}

func TestStallDetection_CapturesPartialOutput(t *testing.T) {
	// When a stall is detected, any output received before the stall
	// should still be in the result.
	inv := &ProcessInvoker{
		StallTimeout: 500 * time.Millisecond,
	}
	model := config.ModelDef{
		Command: "sh",
		Args:    []string{"-c", "echo 'before-stall'; sleep 30"},
	}

	var logBuf bytes.Buffer
	result, err := inv.Invoke(context.Background(), model, "", ".", &logBuf, nil)

	if !errors.Is(err, ErrStallTimeout) {
		t.Fatalf("expected ErrStallTimeout, got: %v", err)
	}
	if !strings.Contains(result.Stdout, "before-stall") {
		t.Errorf("partial output not captured: %q", result.Stdout)
	}
}

func TestStallDetection_ContextCancellationNotConfusedWithStall(t *testing.T) {
	// Context cancellation should not be reported as a stall timeout.
	// When the parent context is cancelled, the process is killed via
	// exec.CommandContext. The result should not contain ErrStallTimeout.
	inv := &ProcessInvoker{
		StallTimeout: 10 * time.Second, // very long stall timeout
	}
	ctx, cancel := context.WithCancel(context.Background())
	model := config.ModelDef{
		Command: "sh",
		Args:    []string{"-c", "echo start; sleep 30"},
	}

	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	var logBuf bytes.Buffer
	start := time.Now()
	_, err := inv.Invoke(ctx, model, "", ".", &logBuf, nil)
	elapsed := time.Since(start)

	// Should have terminated quickly (context cancelled after 300ms),
	// not waited for the 10s stall timeout.
	if elapsed > 5*time.Second {
		t.Errorf("took %v, expected quick termination from context cancel", elapsed)
	}
	// Must not be confused with stall.
	if errors.Is(err, ErrStallTimeout) {
		t.Error("got ErrStallTimeout, but context was cancelled, not stalled")
	}
}

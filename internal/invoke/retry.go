package invoke

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
)

// RetryLogger receives notifications about retry events. Implementations
// should not block.
type RetryLogger interface {
	// OnRetry is called before each retry delay with the attempt number
	// (1-based), the delay before the next attempt, and the error that
	// triggered the retry.
	OnRetry(attempt int, delay time.Duration, err error)
	// OnExhausted is called when all retry attempts have been exhausted.
	OnExhausted(totalAttempts int, lastErr error)
}

// RetryInvoker wraps an Invoker with exponential backoff retry logic
// governed by the project's retry configuration (ADR-019).
//
// Only invocation errors (non-nil error returns from Invoke) are retried.
// A successful process exit — even with a non-zero exit code captured in
// Result — is not retried, since that represents the model running to
// completion and returning a meaningful (if unsuccessful) outcome.
type RetryInvoker struct {
	// Inner is the underlying invoker to wrap with retries.
	Inner Invoker
	// Config holds retry timing parameters.
	Config config.RetriesConfig
	// Logger receives retry event notifications. May be nil.
	Logger RetryLogger
	// SleepFunc allows overriding time.Sleep for testing. If nil, time.Sleep is used.
	SleepFunc func(d time.Duration)
}

// NewRetryInvoker creates a RetryInvoker wrapping the given invoker.
func NewRetryInvoker(inner Invoker, retryCfg config.RetriesConfig, logger RetryLogger) *RetryInvoker {
	return &RetryInvoker{
		Inner:  inner,
		Config: retryCfg,
		Logger: logger,
	}
}

// Invoke delegates to the inner invoker, retrying on error with exponential
// backoff. Context cancellation short-circuits immediately without retrying.
func (r *RetryInvoker) Invoke(ctx context.Context, model config.ModelDef, prompt string, workDir string, logWriter io.Writer, onLine LineCallback) (*Result, error) {
	delay := time.Duration(r.Config.InitialDelaySeconds) * time.Second
	maxDelay := time.Duration(r.Config.MaxDelaySeconds) * time.Second

	sleepFn := r.SleepFunc
	if sleepFn == nil {
		sleepFn = time.Sleep
	}

	for attempt := 0; ; attempt++ {
		result, err := r.Inner.Invoke(ctx, model, prompt, workDir, logWriter, onLine)
		if err == nil {
			return result, nil
		}

		// Context cancellation is never retryable — the caller is
		// shutting down or the invocation timed out.
		if ctx.Err() != nil {
			return result, fmt.Errorf("invocation cancelled: %w", err)
		}

		// Check whether we've exhausted our retry budget (-1 = unlimited).
		if r.Config.MaxRetries >= 0 && attempt >= r.Config.MaxRetries {
			if r.Logger != nil {
				r.Logger.OnExhausted(attempt+1, err)
			}
			return result, fmt.Errorf("retries exhausted after %d attempts: %w", attempt+1, err)
		}

		if r.Logger != nil {
			r.Logger.OnRetry(attempt+1, delay, err)
		}

		// Wait for the backoff delay, but respect context cancellation.
		// When using a real sleep function we check context before and after;
		// the select with default allows the sleep to proceed when context is
		// still active, and the post-sleep check catches mid-sleep cancellation.
		select {
		case <-ctx.Done():
			return result, fmt.Errorf("invocation cancelled during retry wait: %w", ctx.Err())
		default:
			sleepFn(delay)
		}
		if ctx.Err() != nil {
			return result, fmt.Errorf("invocation cancelled during retry wait: %w", ctx.Err())
		}

		// Exponential backoff: double the delay, capped at max.
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}

// IsRetryableError returns true if the error represents a condition worth
// retrying — process spawn failures, pipe errors, and similar infrastructure
// issues. Context cancellation and deadline exceeded errors are not retryable.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return true
}

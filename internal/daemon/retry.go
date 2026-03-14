package daemon

import (
	"context"
	"io"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/output"
)

// invokeWithRetry wraps invoke.InvokeStreaming with exponential backoff
// governed by the config's retries settings. Only invocation errors (non-nil
// error returns) are retried — a successful process exit (even with a
// non-zero exit code captured in Result) is not retried here.
func (d *Daemon) invokeWithRetry(ctx context.Context, model config.ModelDef, prompt string, workDir string, logWriter io.Writer, stageName string) (*invoke.Result, error) {
	rc := d.Config.Retries
	delay := time.Duration(rc.InitialDelaySeconds) * time.Second
	maxDelay := time.Duration(rc.MaxDelaySeconds) * time.Second

	for attempt := 0; ; attempt++ {
		result, err := invoke.InvokeStreaming(ctx, model, prompt, workDir, logWriter)
		if err == nil {
			return result, nil
		}

		// Context cancellation is not retryable — the daemon is shutting down.
		if ctx.Err() != nil {
			return result, err
		}

		// Check whether we've exhausted our retry budget (-1 = unlimited).
		if rc.MaxRetries >= 0 && attempt >= rc.MaxRetries {
			_ = d.Logger.Log(map[string]any{
				"type":     "retry_exhausted",
				"stage":    stageName,
				"attempts": attempt + 1,
				"error":    err.Error(),
			})
			return result, err
		}

		_ = d.Logger.Log(map[string]any{
			"type":    "retry",
			"stage":   stageName,
			"attempt": attempt + 1,
			"delay_s": delay.Seconds(),
			"error":   err.Error(),
		})
		output.PrintHuman("  Invocation error (attempt %d): %v. Retrying in %v.", attempt+1, err, delay)

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(delay):
		}

		// Exponential backoff: double the delay, capped at max.
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}

package daemon

import (
	"context"
	"io"
	"time"

	"fmt"

	"github.com/dorkusprime/wolfcastle/internal/config"
	werrors "github.com/dorkusprime/wolfcastle/internal/errors"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/logging"
)

// invokeWithRetry wraps ProcessInvoker.Invoke with exponential backoff
// governed by the config's retries settings. Only invocation errors (non-nil
// error returns) are retried. A successful process exit (even with a
// non-zero exit code captured in Result) is not retried here.
//
// lg is the logger that receives retry/exhaustion records. In parallel
// mode each worker passes its own child logger; the sequential path
// passes d.Logger. Callers must never pass nil — the records would be
// dropped on the floor just like they were before parallel mode's
// logger plumbing was fixed.
func (d *Daemon) invokeWithRetry(ctx context.Context, lg *logging.Logger, model config.ModelDef, prompt string, workDir string, logWriter io.Writer, stageName string) (*invoke.Result, error) {
	rc := d.Config.Retries
	delay := time.Duration(rc.InitialDelaySeconds) * time.Second
	maxDelay := time.Duration(rc.MaxDelaySeconds) * time.Second

	inv := &invoke.ProcessInvoker{}
	if d.Config.Daemon.StallTimeoutSeconds > 0 {
		inv.StallTimeout = time.Duration(d.Config.Daemon.StallTimeoutSeconds) * time.Second
	}

	for attempt := 0; ; attempt++ {
		result, err := inv.Invoke(ctx, model, prompt, workDir, logWriter, nil)
		if err == nil {
			return result, nil
		}

		// Context cancellation is not retryable; the daemon is shutting down.
		if ctx.Err() != nil {
			return result, err
		}

		// Check whether we've exhausted our retry budget (-1 = unlimited).
		if rc.MaxRetries >= 0 && attempt >= rc.MaxRetries {
			_ = lg.Log(map[string]any{
				"type":     "retry_exhausted",
				"stage":    stageName,
				"attempts": attempt + 1,
				"error":    err.Error(),
			})
			return result, werrors.Invocation(err)
		}

		_ = lg.Log(map[string]any{
			"type":    "retry",
			"stage":   stageName,
			"attempt": attempt + 1,
			"delay_s": delay.Seconds(),
			"error":   err.Error(),
		})
		d.log(map[string]any{"type": "retry_event", "attempt": attempt + 1, "delay_s": delay.Seconds(), "error": err.Error(), "text": fmt.Sprintf("Attempt %d failed: %v. Retrying in %v.", attempt+1, err, delay)})

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

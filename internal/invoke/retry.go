package invoke

import (
	"context"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
)

// RetryLoop invokes a function with exponential backoff.
func RetryLoop(ctx context.Context, retryCfg config.RetriesConfig, fn func() (*Result, error)) (*Result, error) {
	delay := time.Duration(retryCfg.InitialDelaySeconds) * time.Second
	maxDelay := time.Duration(retryCfg.MaxDelaySeconds) * time.Second
	attempts := 0

	for {
		result, err := fn()
		if err == nil && result.ExitCode == 0 {
			return result, nil
		}

		attempts++
		if retryCfg.MaxRetries >= 0 && attempts >= retryCfg.MaxRetries {
			if err != nil {
				return result, err
			}
			return result, nil
		}

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(delay):
		}

		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}

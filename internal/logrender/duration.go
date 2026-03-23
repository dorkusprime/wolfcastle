// Package logrender provides formatters for rendering NDJSON daemon logs
// into human-readable output. It supports session summaries, interleaved
// stage views, agent thought extraction, and duration formatting.
package logrender

import (
	"fmt"
	"time"
)

// resolveDuration returns the duration for a completed stage or planning
// record. When the record carries a pre-computed DurationMS value it is
// converted directly to a time.Duration; otherwise the function falls back
// to the difference between the record timestamp and the tracked start time.
// If neither source is available the result is zero.
func resolveDuration(r Record, startTime time.Time, haveStart bool) time.Duration {
	if r.DurationMS != nil {
		return time.Duration(*r.DurationMS) * time.Millisecond
	}
	if haveStart {
		return r.Timestamp.Sub(startTime)
	}
	return 0
}

// FormatDuration renders a duration as compact human shorthand with no spaces.
// Durations under one minute show seconds only (34s). Durations under one hour
// show minutes and seconds (1m22s), dropping zero seconds (2m). Durations of
// one hour or more show hours and minutes (1h3m), dropping zero minutes (1h).
// Sub-second precision is truncated.
func FormatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	total := int(d.Truncate(time.Second).Seconds())
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60

	if h > 0 {
		if m > 0 {
			return fmt.Sprintf("%dh%dm", h, m)
		}
		return fmt.Sprintf("%dh", h)
	}
	if m > 0 {
		if s > 0 {
			return fmt.Sprintf("%dm%ds", m, s)
		}
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%ds", s)
}

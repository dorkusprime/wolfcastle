package logrender

import (
	"fmt"
	"time"
)

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

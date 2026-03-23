package logrender

import (
	"testing"
	"time"
)

func int64Ptr(n int64) *int64 { return &n }

func TestResolveDuration(t *testing.T) {
	base := time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		record    Record
		startTime time.Time
		haveStart bool
		want      time.Duration
	}{
		{
			name:      "prefers DurationMS when present",
			record:    Record{DurationMS: int64Ptr(5000), Timestamp: base.Add(10 * time.Second)},
			startTime: base,
			haveStart: true,
			want:      5 * time.Second,
		},
		{
			name:      "DurationMS zero value",
			record:    Record{DurationMS: int64Ptr(0), Timestamp: base.Add(10 * time.Second)},
			startTime: base,
			haveStart: true,
			want:      0,
		},
		{
			name:      "falls back to timestamp diff when DurationMS nil",
			record:    Record{Timestamp: base.Add(82 * time.Second)},
			startTime: base,
			haveStart: true,
			want:      82 * time.Second,
		},
		{
			name:      "zero when nil DurationMS and no start",
			record:    Record{Timestamp: base.Add(10 * time.Second)},
			startTime: time.Time{},
			haveStart: false,
			want:      0,
		},
		{
			name:      "DurationMS wins even without start",
			record:    Record{DurationMS: int64Ptr(3000)},
			startTime: time.Time{},
			haveStart: false,
			want:      3 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveDuration(tt.record, tt.startTime, tt.haveStart)
			if got != tt.want {
				t.Errorf("resolveDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{name: "zero", d: 0, want: "0s"},
		{name: "one_second", d: time.Second, want: "1s"},
		{name: "34_seconds", d: 34 * time.Second, want: "34s"},
		{name: "59_seconds", d: 59 * time.Second, want: "59s"},
		{name: "exactly_one_minute", d: time.Minute, want: "1m"},
		{name: "two_minutes", d: 2 * time.Minute, want: "2m"},
		{name: "one_minute_22_seconds", d: time.Minute + 22*time.Second, want: "1m22s"},
		{name: "12_minutes_5_seconds", d: 12*time.Minute + 5*time.Second, want: "12m5s"},
		{name: "59_minutes_59_seconds", d: 59*time.Minute + 59*time.Second, want: "59m59s"},
		{name: "exactly_one_hour", d: time.Hour, want: "1h"},
		{name: "one_hour_3_minutes", d: time.Hour + 3*time.Minute, want: "1h3m"},
		{name: "two_hours_30_minutes", d: 2*time.Hour + 30*time.Minute, want: "2h30m"},
		{name: "one_hour_zero_minutes_some_seconds", d: time.Hour + 45*time.Second, want: "1h"},
		{name: "sub_second_truncated", d: 34*time.Second + 750*time.Millisecond, want: "34s"},
		{name: "sub_second_only", d: 500 * time.Millisecond, want: "0s"},
		{name: "negative_clamps_to_zero", d: -5 * time.Second, want: "0s"},
		{name: "large_duration", d: 25*time.Hour + 59*time.Minute, want: "25h59m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.d)
			if got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

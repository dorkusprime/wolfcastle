package logrender

import (
	"testing"
	"time"
)

func TestParseRecord_FullRecord(t *testing.T) {
	line := `{"type":"stage_complete","timestamp":"2026-03-21T18:04:00Z","level":"info","trace":"execute-0001","stage":"execute","node":"my-project/auth","task":"task-0001","exit_code":0,"text":"done","path":"/tmp/out","marker":"WOLFCASTLE_COMPLETE","error":""}`

	r, err := ParseRecord(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.Type != "stage_complete" {
		t.Errorf("Type = %q, want %q", r.Type, "stage_complete")
	}
	want := time.Date(2026, 3, 21, 18, 4, 0, 0, time.UTC)
	if !r.Timestamp.Equal(want) {
		t.Errorf("Timestamp = %v, want %v", r.Timestamp, want)
	}
	if r.Level != "info" {
		t.Errorf("Level = %q, want %q", r.Level, "info")
	}
	if r.Trace != "execute-0001" {
		t.Errorf("Trace = %q, want %q", r.Trace, "execute-0001")
	}
	if r.Stage != "execute" {
		t.Errorf("Stage = %q, want %q", r.Stage, "execute")
	}
	if r.Node != "my-project/auth" {
		t.Errorf("Node = %q, want %q", r.Node, "my-project/auth")
	}
	if r.Task != "task-0001" {
		t.Errorf("Task = %q, want %q", r.Task, "task-0001")
	}
	if r.ExitCode == nil || *r.ExitCode != 0 {
		t.Errorf("ExitCode = %v, want 0", r.ExitCode)
	}
	if r.Text != "done" {
		t.Errorf("Text = %q, want %q", r.Text, "done")
	}
	if r.Path != "/tmp/out" {
		t.Errorf("Path = %q, want %q", r.Path, "/tmp/out")
	}
	if r.Marker != "WOLFCASTLE_COMPLETE" {
		t.Errorf("Marker = %q, want %q", r.Marker, "WOLFCASTLE_COMPLETE")
	}
}

func TestParseRecord_MinimalFields(t *testing.T) {
	line := `{"type":"assistant","timestamp":"2026-03-21T18:05:00Z","text":"hello"}`

	r, err := ParseRecord(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.Type != "assistant" {
		t.Errorf("Type = %q, want %q", r.Type, "assistant")
	}
	if r.Text != "hello" {
		t.Errorf("Text = %q, want %q", r.Text, "hello")
	}
	if r.Level != "" {
		t.Errorf("Level = %q, want empty", r.Level)
	}
	if r.ExitCode != nil {
		t.Errorf("ExitCode = %v, want nil", r.ExitCode)
	}
}

func TestParseRecord_UnrecognizedType(t *testing.T) {
	line := `{"type":"future_record_type","timestamp":"2026-03-21T18:06:00Z","level":"info"}`

	r, err := ParseRecord(line)
	if err != nil {
		t.Fatalf("unrecognized type should not error: %v", err)
	}
	if r.Type != "future_record_type" {
		t.Errorf("Type = %q, want %q", r.Type, "future_record_type")
	}
}

func TestParseRecord_UnknownFieldsInRaw(t *testing.T) {
	line := `{"type":"stage_start","timestamp":"2026-03-21T18:07:00Z","model":"claude-3","custom_field":"value"}`

	r, err := ParseRecord(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.Raw == nil {
		t.Fatal("Raw map should not be nil")
	}
	if r.Raw["model"] != "claude-3" {
		t.Errorf("Raw[model] = %v, want %q", r.Raw["model"], "claude-3")
	}
	if r.Raw["custom_field"] != "value" {
		t.Errorf("Raw[custom_field] = %v, want %q", r.Raw["custom_field"], "value")
	}
}

func TestParseRecord_MalformedJSON(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"empty string", ""},
		{"not json", "hello world"},
		{"truncated", `{"type":"stage_start"`},
		{"bare value", `42`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseRecord(tc.line)
			if err == nil {
				t.Errorf("expected error for input %q", tc.line)
			}
		})
	}
}

func TestParseRecord_ExitCodeNonZero(t *testing.T) {
	line := `{"type":"stage_complete","timestamp":"2026-03-21T18:08:00Z","exit_code":1}`

	r, err := ParseRecord(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ExitCode == nil {
		t.Fatal("ExitCode should not be nil")
	}
	if *r.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", *r.ExitCode)
	}
}

func TestStageLabel(t *testing.T) {
	cases := []struct {
		stage string
		want  string
	}{
		{"execute", "execute"},
		{"custom-stage", "custom-stage"},
		{"", ""},
	}

	for _, tc := range cases {
		t.Run(tc.stage, func(t *testing.T) {
			r := Record{Stage: tc.stage}
			got := r.StageLabel()
			if got != tc.want {
				t.Errorf("StageLabel() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseRecord_RealWorldRecords(t *testing.T) {
	// Records shaped like actual daemon output.
	cases := []struct {
		name string
		line string
		typ  string
	}{
		{
			"iteration_start",
			`{"level":"info","stage":"execute","node":"my-project/auth","timestamp":"2026-03-21T21:04:48Z","trace":"execute-10001","type":"iteration_start"}`,
			"iteration_start",
		},
		{
			"stage_start",
			`{"level":"info","stage":"intake","timestamp":"2026-03-21T21:04:48Z","trace":"intake-10001","type":"stage_start"}`,
			"stage_start",
		},
		{
			"stage_complete",
			`{"exit_code":0,"level":"info","output_len":24611,"stage":"intake","timestamp":"2026-03-21T21:05:05Z","trace":"intake-10001","type":"stage_complete"}`,
			"stage_complete",
		},
		{
			"assistant",
			`{"level":"debug","text":"I'll read the file now...","timestamp":"2026-03-21T21:04:48Z","trace":"intake-10001","type":"assistant"}`,
			"assistant",
		},
		{
			"audit_report_written",
			`{"level":"info","path":".wolfcastle/system/projects/wild/audit-2026-03-21T18-08.md","timestamp":"2026-03-21T21:06:00Z","trace":"audit-10001","type":"audit_report_written"}`,
			"audit_report_written",
		},
		{
			"planning_start",
			`{"level":"info","stage":"plan","timestamp":"2026-03-21T21:04:48Z","trace":"plan-10001","type":"planning_start"}`,
			"planning_start",
		},
		{
			"planning_complete",
			`{"exit_code":0,"level":"info","node":"my-project","stage":"plan","timestamp":"2026-03-21T21:05:00Z","trace":"plan-10001","type":"planning_complete"}`,
			"planning_complete",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := ParseRecord(tc.line)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.Type != tc.typ {
				t.Errorf("Type = %q, want %q", r.Type, tc.typ)
			}
			if r.Timestamp.IsZero() {
				t.Error("Timestamp should not be zero")
			}
		})
	}
}

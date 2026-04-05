package logrender

import (
	"bytes"
	"testing"
	"time"
)

// helper that feeds records through Replay and returns the rendered string.
func replayLines(records ...Record) string {
	ch := make(chan Record, len(records))
	for _, r := range records {
		ch <- r
	}
	close(ch)

	var buf bytes.Buffer
	sr := NewSummaryRenderer(&buf)
	sr.Replay(ch)
	return buf.String()
}

// helper that feeds a single record through handleFollowRecord.
func followLine(r Record) string {
	var buf bytes.Buffer
	sr := NewSummaryRenderer(&buf)
	starts := make(map[stageKey]time.Time)
	sr.handleFollowRecord(r, starts)
	return buf.String()
}

func TestDaemonLifecycleEngaged(t *testing.T) {
	got := replayLines(Record{
		Type:  "daemon_lifecycle",
		Event: "engaged",
		Scope: "full tree",
	})
	want := "=== Wolfcastle engaged (scope=full tree) ===\n"
	if got != want {
		t.Errorf("Replay daemon_lifecycle engaged:\ngot:  %q\nwant: %q", got, want)
	}

	got = followLine(Record{
		Type:  "daemon_lifecycle",
		Event: "engaged",
		Scope: "full tree",
	})
	if got != want {
		t.Errorf("Follow daemon_lifecycle engaged:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestDaemonLifecycleStandingDown(t *testing.T) {
	got := replayLines(Record{
		Type:   "daemon_lifecycle",
		Event:  "standing_down",
		Reason: "all tasks complete",
	})
	want := "=== Wolfcastle standing down (all tasks complete) ===\n"
	if got != want {
		t.Errorf("Replay daemon_lifecycle standing_down:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestDaemonLifecycleDefault(t *testing.T) {
	got := replayLines(Record{
		Type:  "daemon_lifecycle",
		Event: "drain",
		Text:  "draining work queue",
	})
	want := "=== draining work queue ===\n"
	if got != want {
		t.Errorf("Replay daemon_lifecycle default:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestSelfHealIndented(t *testing.T) {
	got := replayLines(Record{
		Type: "self_heal",
		Text: "recovered stuck task auth/task-0003",
	})
	want := "  recovered stuck task auth/task-0003\n"
	if got != want {
		t.Errorf("Replay self_heal:\ngot:  %q\nwant: %q", got, want)
	}

	got = followLine(Record{
		Type: "self_heal",
		Text: "recovered stuck task auth/task-0003",
	})
	if got != want {
		t.Errorf("Follow self_heal:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestIterationHeaderExecute(t *testing.T) {
	got := replayLines(Record{
		Type:      "iteration_header",
		Kind:      "execute",
		Iteration: 5,
		Text:      "auth/task-0001",
	})
	want := "--- Iteration 5: auth/task-0001 ---\n"
	if got != want {
		t.Errorf("Replay iteration_header execute:\ngot:  %q\nwant: %q", got, want)
	}

	got = followLine(Record{
		Type:      "iteration_header",
		Kind:      "execute",
		Iteration: 5,
		Text:      "auth/task-0001",
	})
	if got != want {
		t.Errorf("Follow iteration_header execute:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestIterationHeaderPlan(t *testing.T) {
	got := replayLines(Record{
		Type:      "iteration_header",
		Kind:      "plan",
		Iteration: 2,
		Text:      "auth/task-0001",
	})
	want := "--- Planning 2: auth/task-0001 ---\n"
	if got != want {
		t.Errorf("Replay iteration_header plan:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestRetryEvent(t *testing.T) {
	got := replayLines(Record{
		Type:    "retry_event",
		Attempt: 3,
		Error:   "connection refused",
		DelayS:  10,
	})
	want := "  Attempt 3 failed: connection refused. Retrying in 10s.\n"
	if got != want {
		t.Errorf("Replay retry_event:\ngot:  %q\nwant: %q", got, want)
	}

	got = followLine(Record{
		Type:    "retry_event",
		Attempt: 3,
		Error:   "connection refused",
		DelayS:  10,
	})
	if got != want {
		t.Errorf("Follow retry_event:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestIdleReasonPlainText(t *testing.T) {
	got := replayLines(Record{
		Type: "idle_reason",
		Text: "waiting for upstream tasks",
	})
	want := "waiting for upstream tasks\n"
	if got != want {
		t.Errorf("Replay idle_reason:\ngot:  %q\nwant: %q", got, want)
	}

	got = followLine(Record{
		Type: "idle_reason",
		Text: "waiting for upstream tasks",
	})
	if got != want {
		t.Errorf("Follow idle_reason:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestArchiveEvent(t *testing.T) {
	got := replayLines(Record{
		Type: "archive_event",
		Text: "archived auth/task-0002",
	})
	want := "archived auth/task-0002\n"
	if got != want {
		t.Errorf("Replay archive_event:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestSpecEventIndented(t *testing.T) {
	got := replayLines(Record{
		Type: "spec_event",
		Text: "loaded spec v2.1",
	})
	want := "  loaded spec v2.1\n"
	if got != want {
		t.Errorf("Replay spec_event:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestKnowledgeEventIndented(t *testing.T) {
	got := followLine(Record{
		Type: "knowledge_event",
		Text: "refreshed knowledge base",
	})
	want := "  refreshed knowledge base\n"
	if got != want {
		t.Errorf("Follow knowledge_event:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestConfigWarningIndented(t *testing.T) {
	got := followLine(Record{
		Type: "config_warning",
		Text: "deprecated field in config",
	})
	want := "  deprecated field in config\n"
	if got != want {
		t.Errorf("Follow config_warning:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestGitEventIndented(t *testing.T) {
	got := followLine(Record{
		Type: "git_event",
		Text: "pulled latest main",
	})
	want := "  pulled latest main\n"
	if got != want {
		t.Errorf("Follow git_event:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestInboxEvent(t *testing.T) {
	got := replayLines(Record{
		Type: "inbox_event",
		Text: "received task auth/task-0005",
	})
	want := "  received task auth/task-0005\n"
	if got != want {
		t.Errorf("Replay inbox_event:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestTaskEvent(t *testing.T) {
	got := replayLines(Record{
		Type: "task_event",
		Text: "task auth/task-0001 marked done",
	})
	want := "  task auth/task-0001 marked done\n"
	if got != want {
		t.Errorf("Replay task_event:\ngot:  %q\nwant: %q", got, want)
	}
}

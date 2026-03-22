package logrender

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestThoughtsRenderer_AssistantRecords(t *testing.T) {
	recs := []Record{
		{Type: "assistant", Text: "I'll start by reading the file..."},
		{Type: "assistant", Text: "Now I need to write the HTML structure..."},
	}

	var buf bytes.Buffer
	NewThoughtsRenderer(&buf).Render(context.Background(), feedRecords(recs))

	expected := "I'll start by reading the file...\nNow I need to write the HTML structure...\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestThoughtsRenderer_FiltersNonAssistant(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "execute", Node: "proj", Task: "task-0001"},
		{Type: "assistant", Text: "Reading the project requirements..."},
		{Type: "terminal_marker", Marker: "WOLFCASTLE_COMPLETE"},
		{Type: "stage_complete", Stage: "execute", Node: "proj", Task: "task-0001"},
		{Type: "iteration_start", Node: "proj"},
		{Type: "assistant", Text: "Done with the implementation."},
		{Type: "audit_report_written", Path: "some/path.md"},
	}

	var buf bytes.Buffer
	NewThoughtsRenderer(&buf).Render(context.Background(), feedRecords(recs))

	expected := "Reading the project requirements...\nDone with the implementation.\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestThoughtsRenderer_SkipsEmptyText(t *testing.T) {
	recs := []Record{
		{Type: "assistant", Text: "First thought."},
		{Type: "assistant", Text: ""},
		{Type: "assistant", Text: "Third thought."},
	}

	var buf bytes.Buffer
	NewThoughtsRenderer(&buf).Render(context.Background(), feedRecords(recs))

	expected := "First thought.\nThird thought.\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestThoughtsRenderer_EmptyChannel(t *testing.T) {
	var buf bytes.Buffer
	NewThoughtsRenderer(&buf).Render(context.Background(), feedRecords(nil))

	if buf.String() != "" {
		t.Errorf("expected empty output, got: %q", buf.String())
	}
}

func TestThoughtsRenderer_OnlyNonAssistant(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "execute", Node: "proj"},
		{Type: "stage_complete", Stage: "execute", Node: "proj"},
		{Type: "planning_start", Node: "proj"},
		{Type: "planning_complete", Node: "proj"},
	}

	var buf bytes.Buffer
	NewThoughtsRenderer(&buf).Render(context.Background(), feedRecords(recs))

	if buf.String() != "" {
		t.Errorf("expected empty output for non-assistant records, got: %q", buf.String())
	}
}

func TestThoughtsRenderer_ContextCancellation(t *testing.T) {
	ch := make(chan Record, 2)
	ch <- Record{Type: "assistant", Text: "Before cancel."}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		NewThoughtsRenderer(&buf).Render(ctx, ch)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Render did not return after context cancellation")
	}

	if !bytes.Contains(buf.Bytes(), []byte("Before cancel.")) {
		t.Errorf("expected pre-cancel text, got: %q", buf.String())
	}
}

func TestThoughtsRenderer_AllEmptyAssistant(t *testing.T) {
	recs := []Record{
		{Type: "assistant", Text: ""},
		{Type: "assistant", Text: ""},
	}

	var buf bytes.Buffer
	NewThoughtsRenderer(&buf).Render(context.Background(), feedRecords(recs))

	if buf.String() != "" {
		t.Errorf("expected empty output for empty assistant records, got: %q", buf.String())
	}
}

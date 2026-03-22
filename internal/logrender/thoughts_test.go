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

func TestThoughtsRenderer_ClaudeAPIEnvelope(t *testing.T) {
	recs := []Record{
		{Type: "assistant", Text: `{"type":"system","subtype":"init"}`},
		{Type: "assistant", Text: `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello from the model"}]}}`},
		{Type: "assistant", Text: `{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"internal reasoning"}]}}`},
		{Type: "assistant", Text: `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls -la"}}]}}`},
		{Type: "assistant", Text: `{"type":"result","result":"Task completed"}`},
	}

	var buf bytes.Buffer
	NewThoughtsRenderer(&buf).Render(context.Background(), feedRecords(recs))

	got := buf.String()
	// Only text content blocks from assistant envelopes should appear.
	if !bytes.Contains([]byte(got), []byte("Hello from the model")) {
		t.Errorf("expected extracted text content, got:\n%s", got)
	}
	// System init, thinking, tool use, and result envelopes are not thoughts.
	for _, unwanted := range []string{"[session started]", "internal reasoning", "Bash", "[result]"} {
		if bytes.Contains([]byte(got), []byte(unwanted)) {
			t.Errorf("thoughts output should not contain %q, got:\n%s", unwanted, got)
		}
	}
	// Exact output: one line of text.
	expected := "Hello from the model\n"
	if got != expected {
		t.Errorf("got:\n%q\nwant:\n%q", got, expected)
	}
}

func TestThoughtsRenderer_MultipleTextBlocks(t *testing.T) {
	recs := []Record{
		{Type: "assistant", Text: `{"type":"assistant","message":{"content":[{"type":"text","text":"First block"},{"type":"tool_use","name":"Read","input":{}},{"type":"text","text":"Second block"}]}}`},
	}

	var buf bytes.Buffer
	NewThoughtsRenderer(&buf).Render(context.Background(), feedRecords(recs))

	expected := "First block\nSecond block\n"
	if buf.String() != expected {
		t.Errorf("got:\n%q\nwant:\n%q", buf.String(), expected)
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

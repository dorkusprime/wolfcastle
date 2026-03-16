package daemon

import (
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════
// invoke.FormatAssistantText
// ═══════════════════════════════════════════════════════════════════════════

func TestFormatAssistantText_AssistantWithTextContent(t *testing.T) {
	t.Parallel()
	raw := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello from the model"}]}}`
	got := invoke.FormatAssistantText(raw)
	if got != "Hello from the model" {
		t.Errorf("expected 'Hello from the model', got %q", got)
	}
}

func TestFormatAssistantText_AssistantWithMultipleTextBlocks(t *testing.T) {
	t.Parallel()
	raw := `{"type":"assistant","message":{"content":[{"type":"text","text":"First"},{"type":"text","text":"Second"}]}}`
	got := invoke.FormatAssistantText(raw)
	if !strings.Contains(got, "First") || !strings.Contains(got, "Second") {
		t.Errorf("expected both text blocks, got %q", got)
	}
	if !strings.Contains(got, "\n") {
		t.Errorf("expected newline joining parts, got %q", got)
	}
}

func TestFormatAssistantText_AssistantWithToolUse_Description(t *testing.T) {
	t.Parallel()
	raw := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"description":"list files"}}]}}`
	got := invoke.FormatAssistantText(raw)
	if !strings.Contains(got, "Bash") || !strings.Contains(got, "list files") {
		t.Errorf("expected tool name and description, got %q", got)
	}
}

func TestFormatAssistantText_AssistantWithToolUse_Command(t *testing.T) {
	t.Parallel()
	raw := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls -la"}}]}}`
	got := invoke.FormatAssistantText(raw)
	if !strings.Contains(got, "Bash") || !strings.Contains(got, "ls -la") {
		t.Errorf("expected tool name and command, got %q", got)
	}
}

func TestFormatAssistantText_AssistantWithToolUse_LongCommand(t *testing.T) {
	t.Parallel()
	longCmd := strings.Repeat("x", 100)
	raw := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"` + longCmd + `"}}]}}`
	got := invoke.FormatAssistantText(raw)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("long command should be truncated with ..., got %q", got)
	}
}

func TestFormatAssistantText_AssistantWithToolUse_NoInputDetails(t *testing.T) {
	t.Parallel()
	raw := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"path":"/foo"}}]}}`
	got := invoke.FormatAssistantText(raw)
	if got != "  → Read" {
		t.Errorf("expected bare tool name, got %q", got)
	}
}

func TestFormatAssistantText_AssistantWithToolUse_NonMapInput(t *testing.T) {
	t.Parallel()
	raw := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":"just a string"}]}}`
	got := invoke.FormatAssistantText(raw)
	if got != "  → Read" {
		t.Errorf("expected bare tool name for non-map input, got %q", got)
	}
}

func TestFormatAssistantText_AssistantEmptyContent(t *testing.T) {
	t.Parallel()
	raw := `{"type":"assistant","message":{"content":[]}}`
	got := invoke.FormatAssistantText(raw)
	if got != "" {
		t.Errorf("expected empty string for empty content, got %q", got)
	}
}

func TestFormatAssistantText_AssistantEmptyTextBlock(t *testing.T) {
	t.Parallel()
	raw := `{"type":"assistant","message":{"content":[{"type":"text","text":""}]}}`
	got := invoke.FormatAssistantText(raw)
	if got != "" {
		t.Errorf("expected empty string when text is empty, got %q", got)
	}
}

func TestFormatAssistantText_ResultType(t *testing.T) {
	t.Parallel()
	raw := `{"type":"result","result":"Task completed successfully"}`
	got := invoke.FormatAssistantText(raw)
	if got != "[result] Task completed successfully" {
		t.Errorf("expected '[result] Task completed successfully', got %q", got)
	}
}

func TestFormatAssistantText_ResultTypeEmpty(t *testing.T) {
	t.Parallel()
	raw := `{"type":"result","result":""}`
	got := invoke.FormatAssistantText(raw)
	if got != "" {
		t.Errorf("expected empty for empty result, got %q", got)
	}
}

func TestFormatAssistantText_SystemInit(t *testing.T) {
	t.Parallel()
	raw := `{"type":"system","subtype":"init"}`
	got := invoke.FormatAssistantText(raw)
	if got != "[session started]" {
		t.Errorf("expected '[session started]', got %q", got)
	}
}

func TestFormatAssistantText_SystemNonInit(t *testing.T) {
	t.Parallel()
	raw := `{"type":"system","subtype":"something_else"}`
	got := invoke.FormatAssistantText(raw)
	if got != "" {
		t.Errorf("expected empty for non-init system, got %q", got)
	}
}

func TestFormatAssistantText_UnknownType(t *testing.T) {
	t.Parallel()
	raw := `{"type":"delta","data":"stuff"}`
	got := invoke.FormatAssistantText(raw)
	if got != "" {
		t.Errorf("expected empty for unknown type, got %q", got)
	}
}

func TestFormatAssistantText_NonJSON(t *testing.T) {
	t.Parallel()
	got := invoke.FormatAssistantText("plain text output")
	if got != "plain text output" {
		t.Errorf("expected plain text passthrough, got %q", got)
	}
}

func TestFormatAssistantText_NonJSON_LongTruncated(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 250)
	got := invoke.FormatAssistantText(long)
	if len(got) != 203 { // 200 + "..."
		t.Errorf("expected truncated to 203 chars, got %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("expected trailing ...")
	}
}

func TestFormatAssistantText_NonJSON_ExactlyAtLimit(t *testing.T) {
	t.Parallel()
	exact := strings.Repeat("b", 200)
	got := invoke.FormatAssistantText(exact)
	if got != exact {
		t.Errorf("string at exactly 200 chars should not be truncated")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// formatAndPrintLogLine — comprehensive type coverage
// ═══════════════════════════════════════════════════════════════════════════

func TestFormatAndPrintLogLine_AllTypeCases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		line string
	}{
		{"stage_start_with_node", `{"type":"stage_start","stage":"execute","node":"proj","task":"t1","level":"info"}`},
		{"stage_start_no_node", `{"type":"stage_start","stage":"expand","level":"info"}`},
		{"stage_complete", `{"type":"stage_complete","stage":"execute","exit_code":0,"output_len":1024,"level":"info"}`},
		{"stage_error", `{"type":"stage_error","stage":"execute","error":"timeout","level":"error"}`},
		{"assistant_with_json", `{"type":"assistant","text":"{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"text\",\"text\":\"hi\"}]}}","level":"info"}`},
		{"assistant_plain_text", `{"type":"assistant","text":"plain output","level":"info"}`},
		{"assistant_empty_format", `{"type":"assistant","text":"{\"type\":\"system\",\"subtype\":\"other\"}","level":"info"}`},
		{"failure_increment", `{"type":"failure_increment","task":"task-0001","count":3,"level":"warn"}`},
		{"auto_block", `{"type":"auto_block","task":"task-0002","reason":"hard cap reached","level":"warn"}`},
		{"terminal_marker", `{"type":"terminal_marker","marker":"WOLFCASTLE_COMPLETE","level":"info"}`},
		{"deliverable_missing", `{"type":"deliverable_missing","task":"task-0003","level":"warn"}`},
		{"deliverable_unchanged", `{"type":"deliverable_unchanged","task":"task-0004","level":"warn"}`},
		{"retry", `{"type":"retry","stage":"execute","attempt":2,"error":"connection reset","level":"warn"}`},
		{"retry_exhausted", `{"type":"retry_exhausted","stage":"execute","attempts":3,"level":"error"}`},
		{"daemon_start", `{"type":"daemon_start","scope":"full tree","level":"info"}`},
		{"daemon_stop", `{"type":"daemon_stop","reason":"signal","level":"info"}`},
		{"propagate_error", `{"type":"propagate_error","error":"filesystem full","level":"error"}`},
		{"default_with_message", `{"type":"custom_event","message":"something happened","level":"info"}`},
		{"default_type_only", `{"type":"custom_event","level":"info"}`},
		{"default_empty_type", `{"level":"info"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Verify no panics; output goes to stdout.
			formatAndPrintLogLine(tc.line, logging.LevelDebug)
		})
	}
}

func TestFormatAndPrintLogLine_TracePrefix(t *testing.T) {
	t.Parallel()
	// Lines with a trace field should include the prefix.
	line := `{"type":"daemon_start","scope":"test","trace":"exec-001","level":"info"}`
	formatAndPrintLogLine(line, logging.LevelDebug)
}

func TestFormatAndPrintLogLine_LevelFiltering(t *testing.T) {
	t.Parallel()

	// A debug-level line should be dropped when minLevel is info.
	// We can't easily assert on stdout suppression here, but we confirm no panic.
	line := `{"type":"daemon_start","scope":"test","level":"debug"}`
	formatAndPrintLogLine(line, logging.LevelInfo)

	// An error-level line should pass through when minLevel is info.
	line2 := `{"type":"stage_error","stage":"x","error":"boom","level":"error"}`
	formatAndPrintLogLine(line2, logging.LevelInfo)
}

func TestFormatAndPrintLogLine_MalformedJSON(t *testing.T) {
	t.Parallel()
	formatAndPrintLogLine("not json {{{", logging.LevelDebug)
}

func TestFormatAndPrintLogLine_NoLevelField(t *testing.T) {
	t.Parallel()
	// Lines without a level field should still be printed.
	formatAndPrintLogLine(`{"type":"daemon_start","scope":"x"}`, logging.LevelDebug)
}

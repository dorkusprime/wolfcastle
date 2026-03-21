package daemon

import (
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/invoke"
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

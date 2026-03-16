package invoke

import (
	"testing"
)

func TestFormatAssistantText_PlainText(t *testing.T) {
	t.Helper()
	got := FormatAssistantText("hello world")
	if got != "hello world" {
		t.Errorf("FormatAssistantText plain text = %q, want %q", got, "hello world")
	}
}

func TestFormatAssistantText_PlainTextTruncation(t *testing.T) {
	t.Helper()
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'a'
	}
	got := FormatAssistantText(string(long))
	if len(got) != 203 { // 200 + "..."
		t.Errorf("truncated length = %d, want 203", len(got))
	}
}

func TestFormatAssistantText_AssistantTextContent(t *testing.T) {
	t.Helper()
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello from the model"}]}}`
	got := FormatAssistantText(input)
	if got != "Hello from the model" {
		t.Errorf("FormatAssistantText assistant text = %q, want %q", got, "Hello from the model")
	}
}

func TestFormatAssistantText_AssistantToolUseWithDescription(t *testing.T) {
	t.Helper()
	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"description":"Run tests"}}]}}`
	got := FormatAssistantText(input)
	want := "  → Bash: Run tests"
	if got != want {
		t.Errorf("FormatAssistantText tool_use desc = %q, want %q", got, want)
	}
}

func TestFormatAssistantText_AssistantToolUseWithCommand(t *testing.T) {
	t.Helper()
	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]}}`
	got := FormatAssistantText(input)
	want := "  → Bash: go test ./..."
	if got != want {
		t.Errorf("FormatAssistantText tool_use cmd = %q, want %q", got, want)
	}
}

func TestFormatAssistantText_AssistantToolUseWithLongCommand(t *testing.T) {
	t.Helper()
	longCmd := make([]byte, 120)
	for i := range longCmd {
		longCmd[i] = 'x'
	}
	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"` + string(longCmd) + `"}}]}}`
	got := FormatAssistantText(input)
	// Command should be truncated to 80 chars + "..."
	if len(got) == 0 {
		t.Fatal("expected non-empty output for long command")
	}
	if got != "  → Bash: "+string(longCmd[:80])+"..." {
		t.Errorf("FormatAssistantText long cmd = %q", got)
	}
}

func TestFormatAssistantText_AssistantToolUseNoDescOrCmd(t *testing.T) {
	t.Helper()
	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"path":"/tmp/foo"}}]}}`
	got := FormatAssistantText(input)
	want := "  → Read"
	if got != want {
		t.Errorf("FormatAssistantText tool_use fallback = %q, want %q", got, want)
	}
}

func TestFormatAssistantText_AssistantToolUseNonMapInput(t *testing.T) {
	t.Helper()
	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":"just-a-string"}]}}`
	got := FormatAssistantText(input)
	want := "  → Read"
	if got != want {
		t.Errorf("FormatAssistantText tool_use non-map = %q, want %q", got, want)
	}
}

func TestFormatAssistantText_AssistantEmptyContent(t *testing.T) {
	t.Helper()
	input := `{"type":"assistant","message":{"content":[]}}`
	got := FormatAssistantText(input)
	if got != "" {
		t.Errorf("FormatAssistantText empty content = %q, want empty", got)
	}
}

func TestFormatAssistantText_AssistantMixedContent(t *testing.T) {
	t.Helper()
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"Thinking..."},{"type":"tool_use","name":"Bash","input":{"description":"Run build"}}]}}`
	got := FormatAssistantText(input)
	want := "Thinking...\n  → Bash: Run build"
	if got != want {
		t.Errorf("FormatAssistantText mixed = %q, want %q", got, want)
	}
}

func TestFormatAssistantText_ResultType(t *testing.T) {
	t.Helper()
	input := `{"type":"result","result":"All tests passed"}`
	got := FormatAssistantText(input)
	want := "[result] All tests passed"
	if got != want {
		t.Errorf("FormatAssistantText result = %q, want %q", got, want)
	}
}

func TestFormatAssistantText_ResultTypeEmpty(t *testing.T) {
	t.Helper()
	input := `{"type":"result","result":""}`
	got := FormatAssistantText(input)
	if got != "" {
		t.Errorf("FormatAssistantText empty result = %q, want empty", got)
	}
}

func TestFormatAssistantText_SystemInit(t *testing.T) {
	t.Helper()
	input := `{"type":"system","subtype":"init"}`
	got := FormatAssistantText(input)
	want := "[session started]"
	if got != want {
		t.Errorf("FormatAssistantText system init = %q, want %q", got, want)
	}
}

func TestFormatAssistantText_SystemNonInit(t *testing.T) {
	t.Helper()
	input := `{"type":"system","subtype":"other"}`
	got := FormatAssistantText(input)
	if got != "" {
		t.Errorf("FormatAssistantText system non-init = %q, want empty", got)
	}
}

func TestFormatAssistantText_UnknownType(t *testing.T) {
	t.Helper()
	input := `{"type":"unknown"}`
	got := FormatAssistantText(input)
	if got != "" {
		t.Errorf("FormatAssistantText unknown type = %q, want empty", got)
	}
}

func TestFormatAssistantText_EmptyTextSkipped(t *testing.T) {
	t.Helper()
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":""},{"type":"text","text":"Real content"}]}}`
	got := FormatAssistantText(input)
	if got != "Real content" {
		t.Errorf("FormatAssistantText skip empty text = %q, want %q", got, "Real content")
	}
}

func TestFormatAssistantText_ToolUseEmptyName(t *testing.T) {
	t.Helper()
	// tool_use with empty name is skipped per the `if c.Name != ""` guard
	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"","input":{}}]}}`
	got := FormatAssistantText(input)
	if got != "" {
		t.Errorf("FormatAssistantText empty tool name = %q, want empty", got)
	}
}

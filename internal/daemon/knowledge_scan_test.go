package daemon

import (
	"reflect"
	"testing"
)

func TestScanKnowledgeEntries_Empty(t *testing.T) {
	t.Parallel()
	got := scanKnowledgeEntries("")
	if got != nil {
		t.Errorf("expected nil slice, got %v", got)
	}
}

func TestScanKnowledgeEntries_NoMarkers(t *testing.T) {
	t.Parallel()
	got := scanKnowledgeEntries("Just narrative text.\nNothing to see.\nWOLFCASTLE_CONTINUE")
	if got != nil {
		t.Errorf("plain output should yield no entries, got %v", got)
	}
}

func TestScanKnowledgeEntries_SingleEntry(t *testing.T) {
	t.Parallel()
	output := `Found a pattern worth recording.
WOLFCASTLE_KNOWLEDGE: Error wrapping must include the operation name for traceability.
WOLFCASTLE_CONTINUE`
	got := scanKnowledgeEntries(output)
	want := []string{"Error wrapping must include the operation name for traceability."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestScanKnowledgeEntries_MultipleEntries(t *testing.T) {
	t.Parallel()
	output := `Several patterns to capture.
WOLFCASTLE_KNOWLEDGE: UUIDs in seed data must be unique across all tables.
Some commentary between entries is fine.
WOLFCASTLE_KNOWLEDGE: Test files must import testify/require, not testify/assert.
WOLFCASTLE_CONTINUE`
	got := scanKnowledgeEntries(output)
	want := []string{
		"UUIDs in seed data must be unique across all tables.",
		"Test files must import testify/require, not testify/assert.",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestScanKnowledgeEntries_TrimsWhitespace(t *testing.T) {
	t.Parallel()
	output := "WOLFCASTLE_KNOWLEDGE:    Leading spaces get trimmed.   "
	got := scanKnowledgeEntries(output)
	want := []string{"Leading spaces get trimmed."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestScanKnowledgeEntries_SkipsEmpty(t *testing.T) {
	t.Parallel()
	output := `WOLFCASTLE_KNOWLEDGE:
WOLFCASTLE_KNOWLEDGE:
WOLFCASTLE_KNOWLEDGE: Real entry.`
	got := scanKnowledgeEntries(output)
	want := []string{"Real entry."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestScanKnowledgeEntries_StreamJSONEnvelope(t *testing.T) {
	t.Parallel()
	// Claude Code's stream-json format wraps assistant text in a JSON
	// envelope. The scanner must reach into message.content[].text to
	// find markers, same as scanTerminalMarker does.
	output := `{"type":"assistant","message":{"content":[{"type":"text","text":"WOLFCASTLE_KNOWLEDGE: Stream envelope delivery works."}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"WOLFCASTLE_CONTINUE"}]}}`
	got := scanKnowledgeEntries(output)
	want := []string{"Stream envelope delivery works."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestScanKnowledgeEntries_StripsMarkdownDecoration(t *testing.T) {
	t.Parallel()
	// The prompt renders markers in code fences; strip the same way
	// scanTerminalMarker does (asterisks, underscores, backticks).
	output := "`WOLFCASTLE_KNOWLEDGE: Backticked marker still parses.`"
	got := scanKnowledgeEntries(output)
	want := []string{"Backticked marker still parses."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestScanKnowledgeEntries_DoesNotMatchTerminalMarkers(t *testing.T) {
	t.Parallel()
	// Defensive guard: confirm the colon separator means WOLFCASTLE_COMPLETE
	// and friends never accidentally register as knowledge entries.
	output := `WOLFCASTLE_COMPLETE
WOLFCASTLE_CONTINUE
WOLFCASTLE_BLOCKED some reason
WOLFCASTLE_YIELD`
	got := scanKnowledgeEntries(output)
	if got != nil {
		t.Errorf("terminal markers should not parse as knowledge, got %+v", got)
	}
}

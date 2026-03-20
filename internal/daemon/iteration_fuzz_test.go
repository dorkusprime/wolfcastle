package daemon

import (
	"strings"
	"testing"
)

func FuzzScanTerminalMarker(f *testing.F) {
	// Plain markers on their own line
	f.Add("WOLFCASTLE_COMPLETE")
	f.Add("WOLFCASTLE_YIELD")
	f.Add("WOLFCASTLE_BLOCKED")
	f.Add("WOLFCASTLE_SKIP")
	f.Add("WOLFCASTLE_CONTINUE")

	// WOLFCASTLE_COMPLETE embedded in surrounding text
	f.Add("some output\nWOLFCASTLE_COMPLETE\ntrailing text")
	f.Add("line one\nline two\nWOLFCASTLE_COMPLETE")

	// Markers wrapped in markdown formatting (asterisks, backticks)
	f.Add("**WOLFCASTLE_BLOCKED**")
	f.Add("`WOLFCASTLE_BLOCKED`")
	f.Add("*WOLFCASTLE_COMPLETE*")
	f.Add("_WOLFCASTLE_YIELD_")
	f.Add("***WOLFCASTLE_COMPLETE***")

	// SKIP with reason suffix
	f.Add("WOLFCASTLE_SKIP already done in prior commit")
	f.Add("WOLFCASTLE_SKIP tree.Resolver already removed")

	// Valid JSON stream envelopes
	f.Add(`{"type":"assistant","text":"WOLFCASTLE_COMPLETE"}`)
	f.Add(`{"type":"assistant","text":"WOLFCASTLE_SKIP reason here"}`)
	f.Add(`{"type":"assistant","text":"some preamble\nWOLFCASTLE_COMPLETE\nmore text"}`)

	// Malformed JSON: truncated, nested, missing closing braces
	f.Add(`{"type":"assistant","text":"WOLFCASTLE_COMPLETE"`)
	f.Add(`{"type":"assistant","text":{"nested":"WOLFCASTLE_COMPLETE"}}`)
	f.Add(`{"type":"assistant"`)
	f.Add(`{`)
	f.Add(`{"type":"assistant","text":"`)
	f.Add(`{"type":"assistant","text":"WOLFCASTLE_`)

	// Mixed format: some lines raw text, some lines JSON
	f.Add("raw line\n" + `{"type":"assistant","text":"WOLFCASTLE_COMPLETE"}` + "\nmore raw")
	f.Add(`{"type":"assistant","text":"hello"}` + "\nWOLFCASTLE_YIELD")

	// Empty string
	f.Add("")

	// Very long strings
	f.Add(strings.Repeat("a", 10000))
	f.Add(strings.Repeat("WOLFCASTLE_COMPLETE\n", 500))
	f.Add(strings.Repeat("x", 9990) + "\nWOLFCASTLE_COMPLETE")

	// Null bytes and control characters
	f.Add("WOLFCASTLE\x00COMPLETE")
	f.Add("\x00\x01\x02\x03")
	f.Add("WOLFCASTLE_COMPLETE\x00")

	// Unicode edge cases: RTL markers, zero-width joiners, combining characters
	f.Add("WOLFCASTLE_COMPLETE\u200F")        // RTL mark
	f.Add("WOLFCASTLE\u200D_COMPLETE")         // zero-width joiner
	f.Add("WOLFCASTLE_COMPLE\u0301TE")         // combining acute accent
	f.Add("\u202EWOLFCASTLE_COMPLETE\u202C")   // RTL override + pop directional
	f.Add("WOLFCASTLE_COMPLETE\uFEFF")         // BOM

	// Partial marker prefixes
	f.Add("WOLFCASTLE_")
	f.Add("WOLFCASTLE_COMP")
	f.Add("WOLFCASTLE_BLO")
	f.Add("WOLFCASTLE_SKI")
	f.Add("WOLFCASTLE_YIE")

	// Multiple markers competing for priority
	f.Add("WOLFCASTLE_YIELD\nWOLFCASTLE_COMPLETE")
	f.Add("WOLFCASTLE_BLOCKED\nWOLFCASTLE_YIELD\nWOLFCASTLE_COMPLETE")

	f.Fuzz(func(t *testing.T, input string) {
		// The only property: scanTerminalMarker must not panic.
		scanTerminalMarker(input)
	})
}

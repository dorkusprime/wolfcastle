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
	f.Add("WOLFCASTLE_COMPLETE\u200F")       // RTL mark
	f.Add("WOLFCASTLE\u200D_COMPLETE")       // zero-width joiner
	f.Add("WOLFCASTLE_COMPLE\u0301TE")       // combining acute accent
	f.Add("\u202EWOLFCASTLE_COMPLETE\u202C") // RTL override + pop directional
	f.Add("WOLFCASTLE_COMPLETE\uFEFF")       // BOM

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

func FuzzExtractAssistantText(f *testing.F) {
	// Valid assistant envelopes: simple format
	f.Add(`{"type":"assistant","text":"hello world"}`)
	f.Add(`{"type":"assistant","text":"WOLFCASTLE_COMPLETE"}`)
	f.Add(`{"type":"assistant","text":"multi\nline\ntext"}`)
	f.Add(`{"type":"assistant","text":""}`)

	// Valid assistant envelopes: Claude Code format
	f.Add(`{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]}}`)
	f.Add(`{"type":"assistant","message":{"content":[{"type":"text","text":"WOLFCASTLE_COMPLETE"}]}}`)
	f.Add(`{"type":"assistant","message":{"content":[{"type":"text","text":"first"},{"type":"text","text":"second"}]}}`)
	f.Add(`{"type":"assistant","message":{"content":[{"type":"image","text":"ignored"}]}}`)
	f.Add(`{"type":"assistant","message":{"content":[]}}`)

	// Valid result envelopes: text field format
	f.Add(`{"type":"result","text":"done"}`)
	f.Add(`{"type":"result","text":"WOLFCASTLE_COMPLETE"}`)

	// Valid result envelopes: result field format
	f.Add(`{"type":"result","result":"finished"}`)
	f.Add(`{"type":"result","result":"WOLFCASTLE_SKIP already done"}`)

	// Both text and result fields present
	f.Add(`{"type":"result","text":"primary","result":"secondary"}`)

	// Deeply nested JSON objects
	f.Add(`{"type":"assistant","text":"ok","extra":{"a":{"b":{"c":{"d":{"e":"deep"}}}}}}`)
	f.Add(`{"type":"assistant","message":{"content":[{"type":"text","text":"deep","meta":{"a":{"b":{"c":1}}}}]}}`)
	f.Add(`{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{"i":{"j":"ten"}}}}}}}}}}`)

	// Very large JSON payloads
	f.Add(`{"type":"assistant","text":"` + strings.Repeat("x", 10000) + `"}`)
	f.Add(`{"type":"result","result":"` + strings.Repeat("y", 10000) + `"}`)
	largeContent := `{"type":"assistant","message":{"content":[`
	for i := 0; i < 100; i++ {
		if i > 0 {
			largeContent += ","
		}
		largeContent += `{"type":"text","text":"chunk"}`
	}
	largeContent += `]}}`
	f.Add(largeContent)

	// JSON with unexpected types (numbers where strings expected, arrays where objects expected)
	f.Add(`{"type":123,"text":"hello"}`)
	f.Add(`{"type":"assistant","text":42}`)
	f.Add(`{"type":"assistant","text":null}`)
	f.Add(`{"type":"assistant","text":true}`)
	f.Add(`{"type":"assistant","text":["array","of","strings"]}`)
	f.Add(`{"type":"assistant","message":"not an object"}`)
	f.Add(`{"type":"assistant","message":{"content":"not an array"}}`)
	f.Add(`{"type":"assistant","message":{"content":[42, true, null]}}`)
	f.Add(`{"type":null}`)
	f.Add(`{"type":["assistant"]}`)

	// Non-JSON input that starts with '{'
	f.Add(`{not json at all}`)
	f.Add(`{{{`)
	f.Add(`{type: assistant}`)
	f.Add(`{ this is yaml-ish: true }`)
	f.Add(`{<xml>not json</xml>}`)

	// Truncated JSON at various points
	f.Add(`{`)
	f.Add(`{"`)
	f.Add(`{"type`)
	f.Add(`{"type"`)
	f.Add(`{"type":`)
	f.Add(`{"type":"`)
	f.Add(`{"type":"assistant`)
	f.Add(`{"type":"assistant"`)
	f.Add(`{"type":"assistant",`)
	f.Add(`{"type":"assistant","text`)
	f.Add(`{"type":"assistant","text":`)
	f.Add(`{"type":"assistant","text":"`)
	f.Add(`{"type":"assistant","text":"hello`)
	f.Add(`{"type":"assistant","text":"hello"`)
	f.Add(`{"type":"assistant","message":{"content":[{"type":"text","text":"trunc`)

	// Empty and minimal JSON
	f.Add(`{}`)
	f.Add(`{"type":""}`)
	f.Add(`{"type":"unknown"}`)

	// Non-JSON strings (quick-rejected by len < 2 or first char != '{')
	f.Add("")
	f.Add("x")
	f.Add("hello world")
	f.Add(`["array"]`)
	f.Add(`"just a string"`)

	f.Fuzz(func(t *testing.T, input string) {
		// The only property: extractAssistantText must not panic.
		extractAssistantText(input)
	})
}

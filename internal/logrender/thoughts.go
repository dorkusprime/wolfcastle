package logrender

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ThoughtsRenderer writes raw agent output. It filters for assistant records,
// extracts the Text field, and writes it followed by a newline. No timestamps,
// no indentation, no glyphs. Records with empty text are silently skipped.
type ThoughtsRenderer struct {
	w io.Writer
}

// NewThoughtsRenderer returns a renderer that writes agent thoughts to w.
func NewThoughtsRenderer(w io.Writer) *ThoughtsRenderer {
	return &ThoughtsRenderer{w: w}
}

// Render consumes records from the channel and writes assistant text to the
// output writer. It returns when the channel closes or ctx is cancelled. The
// behavior is identical for replay and follow mode since records are processed
// as they arrive.
func (tr *ThoughtsRenderer) Render(ctx context.Context, records <-chan Record) {
	for {
		select {
		case <-ctx.Done():
			return
		case r, ok := <-records:
			if !ok {
				return
			}
			if r.Type == "assistant" && r.Text != "" {
				text := extractThoughtText(r.Text)
				if text != "" {
					_, _ = fmt.Fprintln(tr.w, text)
				}
			}
		}
	}
}

// extractThoughtText pulls only human-readable text from a Claude API JSON
// envelope. System init markers, result summaries, tool use blocks, and
// thinking blocks are all dropped; thoughts mode cares only about the words
// the model chose to say. Plain text (non-JSON) passes through unchanged.
func extractThoughtText(raw string) string {
	var envelope struct {
		Type    string `json:"type"`
		Message struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		if len(raw) > 200 {
			return raw[:200] + "..."
		}
		return raw
	}
	if envelope.Type != "assistant" {
		return ""
	}
	var parts []string
	for _, c := range envelope.Message.Content {
		if c.Type == "text" && c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

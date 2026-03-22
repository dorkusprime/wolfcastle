package logrender

import (
	"context"
	"fmt"
	"io"

	"github.com/dorkusprime/wolfcastle/internal/invoke"
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
				text := invoke.FormatAssistantText(r.Text)
				if text != "" {
					_, _ = fmt.Fprintln(tr.w, text)
				}
			}
		}
	}
}

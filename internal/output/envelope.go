// Package output provides structured JSON envelope formatting and
// human-readable printing for all CLI command output. Every command
// routes its output through this package to ensure consistent
// formatting across --json and default modes.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Response is the standard JSON envelope for all CLI command output.
type Response struct {
	OK     bool   `json:"ok"`
	Action string `json:"action"`
	Error  string `json:"error,omitempty"`
	Code   int    `json:"code,omitempty"`
	Data   any    `json:"data,omitempty"`
}

// Ok creates a success response.
func Ok(action string, data any) Response {
	return Response{OK: true, Action: action, Data: data}
}

// Err creates an error response.
func Err(action string, code int, msg string) Response {
	return Response{OK: false, Action: action, Error: msg, Code: code}
}

// Print writes a response as JSON to stdout. Encoding errors are silently
// discarded because output failures at this layer have no recovery path.
func Print(r Response) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(r)
}

// PrintHuman writes a human-readable message to stdout.
// If a spinner is active, pauses the animation while writing
// and keeps it suppressed briefly so rapid messages don't
// interleave with redraws.
func PrintHuman(format string, args ...any) {
	PauseSpinner()
	_, _ = fmt.Fprintf(os.Stdout, format+"\n", args...)
	ResumeSpinner()
}

// PrintError writes an error message to stderr.
func PrintError(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}

// SpinnerWriter wraps an io.Writer with spinner pause/resume so that
// output through the writer doesn't collide with spinner animation.
// Pass this to renderers in production; pass a plain buffer in tests.
type SpinnerWriter struct {
	W io.Writer
}

// Write pauses the spinner, writes to the underlying writer, then resumes.
func (sw *SpinnerWriter) Write(p []byte) (int, error) {
	PauseSpinner()
	n, err := sw.W.Write(p)
	ResumeSpinner()
	return n, err
}

// Plural returns singular when n == 1, plural otherwise.
// Example: Plural(3, "issue", "issues") => "3 issues"
func Plural(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

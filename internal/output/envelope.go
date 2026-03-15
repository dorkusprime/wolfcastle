// Package output provides structured JSON envelope formatting and
// human-readable printing for all CLI command output. Every command
// routes its output through this package to ensure consistent
// formatting across --json and default modes.
package output

import (
	"encoding/json"
	"fmt"
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
// If a spinner is active, clears the spinner line first so the
// message prints cleanly. The spinner redraws on its next tick.
func PrintHuman(format string, args ...any) {
	activeMu.Lock()
	s := activeSpinner
	activeMu.Unlock()
	if s != nil {
		s.clearForMessage()
	}
	_, _ = fmt.Fprintf(os.Stdout, format+"\n", args...)
}

// PrintError writes an error message to stderr.
func PrintError(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}

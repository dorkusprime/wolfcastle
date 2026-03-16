// Package invoke handles model CLI invocation, piping assembled prompts
// to stdin and capturing stdout/stderr. It supports both buffered and
// streaming modes, with the latter enabling real-time log output via
// wolfcastle follow. The package provides marker detection for
// WOLFCASTLE_COMPLETE, WOLFCASTLE_YIELD, and WOLFCASTLE_SUMMARY markers,
// exponential backoff retry logic, and invocation timeout support.
package invoke

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/config"
)

// Marker represents a detected WOLFCASTLE_* marker in model output.
type Marker int

const (
	// MarkerNone indicates no terminal marker was detected.
	MarkerNone Marker = iota
	// MarkerComplete indicates WOLFCASTLE_COMPLETE was found.
	MarkerComplete
	// MarkerYield indicates WOLFCASTLE_YIELD was found.
	MarkerYield
	// MarkerBlocked indicates WOLFCASTLE_BLOCKED was found.
	MarkerBlocked
)

// String returns the human-readable name of the marker.
func (m Marker) String() string {
	switch m {
	case MarkerComplete:
		return "WOLFCASTLE_COMPLETE"
	case MarkerYield:
		return "WOLFCASTLE_YIELD"
	case MarkerBlocked:
		return "WOLFCASTLE_BLOCKED"
	default:
		return "none"
	}
}

// Result is the output of a model invocation.
type Result struct {
	// Stdout is the captured standard output from the model process.
	Stdout string
	// Stderr is the captured standard error from the model process.
	Stderr string
	// ExitCode is the process exit code (0 for success).
	ExitCode int
	// TerminalMarker is the first terminal marker detected during streaming.
	TerminalMarker Marker
	// Summary is the text following a WOLFCASTLE_SUMMARY: marker, if any.
	Summary string
}

// LineCallback is called for each line of model output during streaming.
// Implementations should not block, as this runs in the output-processing
// goroutine and delays would stall reading from the process pipe.
type LineCallback func(line string)

// Invoker defines the contract for model invocation. Implementations may
// spawn real CLI processes or provide test doubles.
type Invoker interface {
	// Invoke runs a model with the given prompt and returns the result.
	// The logWriter, if non-nil, receives each line of stdout in real time
	// for NDJSON log streaming. The onLine callback, if non-nil, is called
	// for each line of output during streaming.
	Invoke(ctx context.Context, model config.ModelDef, prompt string, workDir string, logWriter io.Writer, onLine LineCallback) (*Result, error)
}

// ProcessInvoker spawns real CLI processes for model invocation.
type ProcessInvoker struct {
	// CmdFactory allows overriding exec.CommandContext for testing.
	// If nil, exec.CommandContext is used.
	CmdFactory func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewProcessInvoker creates a ProcessInvoker that spawns real CLI processes.
func NewProcessInvoker() *ProcessInvoker {
	return &ProcessInvoker{}
}

// Invoke runs a model CLI command, streaming each line of stdout to
// logWriter (if non-nil) while also capturing the full output. It detects
// WOLFCASTLE_* markers during streaming and populates the Result accordingly.
func (p *ProcessInvoker) Invoke(ctx context.Context, model config.ModelDef, prompt string, workDir string, logWriter io.Writer, onLine LineCallback) (*Result, error) {
	var cmd *exec.Cmd
	if p.CmdFactory != nil {
		cmd = p.CmdFactory(ctx, model.Command, model.Args...)
	} else {
		cmd = exec.CommandContext(ctx, model.Command, model.Args...)
	}
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Put child in its own process group for clean signal propagation.
	cmd.SysProcAttr = processSysProcAttr()

	if logWriter == nil && onLine == nil {
		// No streaming: capture stdout directly into a buffer.
		var stdout bytes.Buffer
		cmd.Stdout = &stdout

		err := cmd.Run()
		RestoreTerminal()

		output := stdout.String()
		result := &Result{
			Stdout: output,
			Stderr: stderr.String(),
		}

		// Detect markers in the captured output.
		detectMarkers(output, result)

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			} else {
				return result, fmt.Errorf("invoking %s: %w", model.Command, err)
			}
		}

		return result, nil
	}

	// Streaming: pipe stdout through a scanner that writes to both
	// the log writer and a capture buffer.
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", model.Command, err)
	}

	result := &Result{}
	var captured bytes.Buffer
	scanner := bufio.NewScanner(stdoutPipe)
	// Increase buffer to 1MB to handle large model output lines.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		captured.WriteString(line)
		captured.WriteByte('\n')

		// Detect markers as lines arrive for immediate awareness.
		detectLineMarker(line, result)

		// Stream to NDJSON log writer.
		if logWriter != nil {
			_, _ = fmt.Fprintln(logWriter, line)
		}

		// Notify callback.
		if onLine != nil {
			onLine(line)
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, fmt.Errorf("reading stdout: %w", scanErr)
	}

	err = cmd.Wait()
	RestoreTerminal()

	result.Stdout = captured.String()
	result.Stderr = stderr.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, fmt.Errorf("invoking %s: %w", model.Command, err)
		}
	}

	return result, nil
}

// detectMarkers scans the full output for WOLFCASTLE_* markers and
// populates the Result's TerminalMarker and Summary fields.
func detectMarkers(output string, result *Result) {
	for _, line := range strings.Split(output, "\n") {
		detectLineMarker(strings.TrimSpace(line), result)
	}
}

// detectLineMarker checks a single line for WOLFCASTLE_* markers and
// updates the result. Only the first terminal marker is recorded.
func detectLineMarker(line string, result *Result) {
	trimmed := strings.TrimSpace(line)

	// Terminal markers — only record the first one encountered.
	if result.TerminalMarker == MarkerNone {
		switch {
		case strings.Contains(trimmed, "WOLFCASTLE_COMPLETE"):
			result.TerminalMarker = MarkerComplete
		case strings.Contains(trimmed, "WOLFCASTLE_YIELD"):
			result.TerminalMarker = MarkerYield
		case strings.Contains(trimmed, "WOLFCASTLE_BLOCKED"):
			result.TerminalMarker = MarkerBlocked
		}
	}

	// Summary marker — can appear alongside a terminal marker.
	if strings.HasPrefix(trimmed, "WOLFCASTLE_SUMMARY:") {
		text := strings.TrimSpace(strings.TrimPrefix(trimmed, "WOLFCASTLE_SUMMARY:"))
		if text != "" {
			result.Summary = text
		}
	}
}

// --- Legacy API for backward compatibility ---

// InvokeSimple runs a model CLI command with the given prompt piped to stdin.
// It captures all output and returns it when the command finishes.
// This is a convenience wrapper around ProcessInvoker for callers that
// do not need streaming or line callbacks.
func InvokeSimple(ctx context.Context, model config.ModelDef, prompt string, workDir string) (*Result, error) {
	return NewProcessInvoker().Invoke(ctx, model, prompt, workDir, nil, nil)
}

// Invoke runs a model CLI command with the given prompt piped to stdin.
// Preserved for backward compatibility with existing callers.
func Invoke(ctx context.Context, model config.ModelDef, prompt string, workDir string) (*Result, error) {
	return InvokeSimple(ctx, model, prompt, workDir)
}

// InvokeStreaming runs a model CLI command, streaming each line of stdout to
// logWriter (if non-nil) while also capturing the full output in Result.Stdout.
// Preserved for backward compatibility with existing callers.
func InvokeStreaming(ctx context.Context, model config.ModelDef, prompt string, workDir string, logWriter io.Writer) (*Result, error) {
	return NewProcessInvoker().Invoke(ctx, model, prompt, workDir, logWriter, nil)
}

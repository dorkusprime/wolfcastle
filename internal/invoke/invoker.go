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
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
)

// ErrStallTimeout is returned when the model process produces no output
// for longer than the configured stall timeout.
var ErrStallTimeout = errors.New("model output stalled: no output received within stall timeout")

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
	// MarkerSkip indicates WOLFCASTLE_SKIP was found.
	MarkerSkip
	// MarkerContinue indicates WOLFCASTLE_CONTINUE was found.
	MarkerContinue
)

// Marker string values. These are the canonical representations used in
// model output and parsed by both detectLineMarker (streaming, first-match)
// and scanTerminalMarker (post-execution, priority-ordered).
const (
	MarkerStringComplete = "WOLFCASTLE_COMPLETE"
	MarkerStringYield    = "WOLFCASTLE_YIELD"
	MarkerStringBlocked  = "WOLFCASTLE_BLOCKED"
	MarkerStringSkip     = "WOLFCASTLE_SKIP"
	MarkerStringContinue = "WOLFCASTLE_CONTINUE"
	MarkerStringSummary  = "WOLFCASTLE_SUMMARY:"
)

// String returns the human-readable name of the marker.
func (m Marker) String() string {
	switch m {
	case MarkerComplete:
		return MarkerStringComplete
	case MarkerYield:
		return MarkerStringYield
	case MarkerBlocked:
		return MarkerStringBlocked
	case MarkerSkip:
		return MarkerStringSkip
	case MarkerContinue:
		return MarkerStringContinue
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

	// StallTimeout, if positive, kills the process when no stdout output
	// has been received for this duration. This catches hung model processes
	// (e.g., API instability) without waiting for the full invocation timeout.
	// Zero or negative disables stall detection.
	StallTimeout time.Duration
}

// NewProcessInvoker creates a ProcessInvoker that spawns real CLI processes.
func NewProcessInvoker() *ProcessInvoker {
	return &ProcessInvoker{}
}

// Invoke runs a model CLI command, streaming each line of stdout to
// logWriter (if non-nil) while also capturing the full output. It detects
// WOLFCASTLE_* markers during streaming and populates the Result accordingly.
func (p *ProcessInvoker) Invoke(ctx context.Context, model config.ModelDef, prompt string, workDir string, logWriter io.Writer, onLine LineCallback) (*Result, error) {
	// If stall detection is enabled for streaming invocations, wrap the
	// context with a cancel we can fire when the stall timer expires.
	// This lets exec.CommandContext handle the process kill cleanly.
	stallEnabled := p.StallTimeout > 0 && (logWriter != nil || onLine != nil)
	var stallCancel context.CancelFunc
	var stalled bool
	cmdCtx := ctx
	if stallEnabled {
		cmdCtx, stallCancel = context.WithCancel(ctx)
		defer stallCancel()
	}

	var cmd *exec.Cmd
	if p.CmdFactory != nil {
		cmd = p.CmdFactory(cmdCtx, model.Command, model.Args...)
	} else {
		cmd = exec.CommandContext(cmdCtx, model.Command, model.Args...)
	}
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Put child in its own process group for clean signal propagation.
	// Override Cancel to kill the entire group (not just the leader) so
	// child processes (test runners, linters, etc.) don't become orphans.
	cmd.SysProcAttr = processSysProcAttr()
	cmd.Cancel = func() error {
		return killProcessGroup(cmd.Process)
	}

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

	// If stall detection is active, run the scanner in a goroutine and
	// select between lines, the stall timer, and context cancellation.
	// When the stall timer fires we cancel cmdCtx, which makes
	// exec.CommandContext kill the child process, unblocking the scanner.
	if stallEnabled {
		type scanLine struct {
			line string
			err  error
			eof  bool
		}
		lines := make(chan scanLine, 1)
		go func() {
			defer close(lines)
			for scanner.Scan() {
				lines <- scanLine{line: scanner.Text()}
			}
			if err := scanner.Err(); err != nil {
				lines <- scanLine{err: err}
			} else {
				lines <- scanLine{eof: true}
			}
		}()

		stallTimer := time.NewTimer(p.StallTimeout)
		defer stallTimer.Stop()

	scanLoop:
		for {
			select {
			case sl, ok := <-lines:
				if !ok {
					break scanLoop
				}
				if sl.err != nil {
					// Scanner errors after a stall cancellation are expected
					// (broken pipe). Don't mask the stall error.
					if stalled {
						break scanLoop
					}
					return nil, fmt.Errorf("reading stdout: %w", sl.err)
				}
				if sl.eof {
					break scanLoop
				}

				captured.WriteString(sl.line)
				captured.WriteByte('\n')
				detectLineMarker(sl.line, result)
				if logWriter != nil {
					_, _ = fmt.Fprintln(logWriter, sl.line)
				}
				if onLine != nil {
					onLine(sl.line)
				}

				// Reset the stall timer on every line of output.
				if !stallTimer.Stop() {
					select {
					case <-stallTimer.C:
					default:
					}
				}
				stallTimer.Reset(p.StallTimeout)

			case <-stallTimer.C:
				stalled = true
				stallCancel() // kills the child via exec.CommandContext
				// Drain remaining lines from the scanner goroutine.
				for range lines {
				}
				break scanLoop
			}
		}
	} else {
		// No stall detection: simple synchronous loop.
		for scanner.Scan() {
			line := scanner.Text()
			captured.WriteString(line)
			captured.WriteByte('\n')
			detectLineMarker(line, result)
			if logWriter != nil {
				_, _ = fmt.Fprintln(logWriter, line)
			}
			if onLine != nil {
				onLine(line)
			}
		}
		if scanErr := scanner.Err(); scanErr != nil {
			return nil, fmt.Errorf("reading stdout: %w", scanErr)
		}
	}

	err = cmd.Wait()
	RestoreTerminal()

	result.Stdout = captured.String()
	result.Stderr = stderr.String()

	if stalled {
		return result, ErrStallTimeout
	}

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
// This is the streaming detector: it fires during output capture for
// immediate awareness. It uses first-match semantics.
//
// The daemon also runs scanTerminalMarker (in daemon/iteration.go)
// AFTER execution completes. That scanner uses priority ordering
// (COMPLETE > BLOCKED > YIELD) across the full output to handle
// cases where multiple markers appear (e.g., prompt echo followed
// by the real marker). Both detectors exist because streaming needs
// immediate detection while post-execution needs priority resolution.
func detectLineMarker(line string, result *Result) {
	trimmed := strings.TrimSpace(line)

	// Terminal markers: only record the first one encountered.
	if result.TerminalMarker == MarkerNone {
		switch {
		case strings.Contains(trimmed, MarkerStringComplete):
			result.TerminalMarker = MarkerComplete
		case strings.Contains(trimmed, MarkerStringYield):
			result.TerminalMarker = MarkerYield
		case strings.Contains(trimmed, MarkerStringSkip):
			result.TerminalMarker = MarkerSkip
		case strings.Contains(trimmed, MarkerStringContinue):
			result.TerminalMarker = MarkerContinue
		case strings.Contains(trimmed, MarkerStringBlocked):
			result.TerminalMarker = MarkerBlocked
		}
	}

	// Summary marker: can appear alongside a terminal marker.
	if strings.HasPrefix(trimmed, MarkerStringSummary) {
		text := strings.TrimSpace(strings.TrimPrefix(trimmed, MarkerStringSummary))
		if text != "" {
			result.Summary = text
		}
	}
}

// --- Legacy API for backward compatibility ---

// Simple runs a model CLI command with the given prompt piped to stdin.
// It captures all output and returns it when the command finishes.
// This is a convenience wrapper around ProcessInvoker for callers that
// do not need streaming or line callbacks.
func Simple(ctx context.Context, model config.ModelDef, prompt string, workDir string) (*Result, error) {
	return NewProcessInvoker().Invoke(ctx, model, prompt, workDir, nil, nil)
}

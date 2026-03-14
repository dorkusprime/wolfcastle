package invoke

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"

	"github.com/dorkusprime/wolfcastle/internal/config"
)

// Result is the output of a model invocation.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Invoke runs a model CLI command with the given prompt piped to stdin.
// It captures all output and returns it when the command finishes.
func Invoke(ctx context.Context, model config.ModelDef, prompt string, workDir string) (*Result, error) {
	return InvokeStreaming(ctx, model, prompt, workDir, nil)
}

// InvokeStreaming runs a model CLI command, streaming each line of stdout to
// logWriter (if non-nil) while also capturing the full output in Result.Stdout.
func InvokeStreaming(ctx context.Context, model config.ModelDef, prompt string, workDir string, logWriter io.Writer) (*Result, error) {
	cmd := exec.CommandContext(ctx, model.Command, model.Args...)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Put child in its own process group for clean signal propagation
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if logWriter == nil {
		// No streaming: capture stdout directly into a buffer
		var stdout bytes.Buffer
		cmd.Stdout = &stdout

		err := cmd.Run()

		result := &Result{
			Stdout: stdout.String(),
			Stderr: stderr.String(),
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

	// Streaming: pipe stdout through a scanner that writes to both
	// the log writer and a capture buffer.
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", model.Command, err)
	}

	var captured bytes.Buffer
	scanner := bufio.NewScanner(stdoutPipe)
	// Increase buffer to 1MB to handle large model output lines (base64, minified JSON)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		captured.WriteString(line)
		captured.WriteByte('\n')
		// Write to the log writer; ignore errors to avoid blocking the process
		_, _ = fmt.Fprintln(logWriter, line)
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, fmt.Errorf("reading stdout: %w", scanErr)
	}

	err = cmd.Wait()

	result := &Result{
		Stdout: captured.String(),
		Stderr: stderr.String(),
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

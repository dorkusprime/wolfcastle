package invoke

import (
	"bytes"
	"context"
	"fmt"
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
func Invoke(ctx context.Context, model config.ModelDef, prompt string, workDir string) (*Result, error) {
	cmd := exec.CommandContext(ctx, model.Command, model.Args...)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Put child in its own process group for clean signal propagation
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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

//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// binaryPath is set by TestMain to the compiled wolfcastle binary.
var binaryPath string

// testEnv returns a copy of the current environment with WOLFCASTLE_LOCK_DIR
// set to a directory inside the test working directory. This isolates the
// daemon lock file so parallel integration tests don't collide.
func testEnv(dir string) []string {
	env := os.Environ()
	lockDir := filepath.Join(dir, ".wolfcastle-lock")
	_ = os.MkdirAll(lockDir, 0755)
	return append(env, "WOLFCASTLE_LOCK_DIR="+lockDir)
}

func TestMain(m *testing.M) {
	// Build the wolfcastle binary once for all integration tests.
	tmp, err := os.MkdirTemp("", "wolfcastle-integration-*")
	if err != nil {
		panic("cannot create temp dir for binary: " + err.Error())
	}
	defer os.RemoveAll(tmp)

	bin := filepath.Join(tmp, "wolfcastle")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/dorkusprime/wolfcastle")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("cannot build wolfcastle binary: " + err.Error())
	}
	binaryPath = bin

	os.Exit(m.Run())
}

// run executes a wolfcastle command in the given directory and returns
// its combined stdout output. It fails the test on non-zero exit.
func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = dir
	cmd.Env = testEnv(dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("wolfcastle %v failed: %v\nstdout: %s\nstderr: %s", args, err, stdout.String(), stderr.String())
	}
	return stdout.String()
}

// runExpectError executes a wolfcastle command that is expected to fail.
// It fails the test if the command exits successfully.
func runExpectError(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = dir
	cmd.Env = testEnv(dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatalf("wolfcastle %v succeeded but expected failure\nstdout: %s", args, stdout.String())
	}
	return stdout.String() + stderr.String()
}

// runJSON executes a wolfcastle command with --json and unmarshals the
// JSON envelope response.
func runJSON(t *testing.T, dir string, args ...string) output.Response {
	t.Helper()
	fullArgs := append([]string{"--json"}, args...)
	out := run(t, dir, fullArgs...)

	var resp output.Response
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON response: %v\nraw: %s", err, out)
	}
	return resp
}

// loadRootIndex reads the root index from the .wolfcastle directory on
// disk. It discovers the namespace by scanning the projects directory.
func loadRootIndex(t *testing.T, dir string) *state.RootIndex {
	t.Helper()
	ns := discoverNamespace(t, dir)
	idxPath := filepath.Join(dir, ".wolfcastle", "system", "projects", ns, "state.json")
	idx, err := state.LoadRootIndex(idxPath)
	if err != nil {
		t.Fatalf("failed to load root index at %s: %v", idxPath, err)
	}
	return idx
}

// loadNode reads a node's state from disk.
func loadNode(t *testing.T, dir, addr string) *state.NodeState {
	t.Helper()
	ns := discoverNamespace(t, dir)
	statePath := filepath.Join(dir, ".wolfcastle", "system", "projects", ns, filepath.FromSlash(addr), "state.json")
	ns2, err := state.LoadNodeState(statePath)
	if err != nil {
		t.Fatalf("failed to load node state at %s: %v", statePath, err)
	}
	return ns2
}

// saveNode writes a node's state to disk (used for corruption tests).
func saveNode(t *testing.T, dir, addr string, ns *state.NodeState) {
	t.Helper()
	namespace := discoverNamespace(t, dir)
	statePath := filepath.Join(dir, ".wolfcastle", "system", "projects", namespace, filepath.FromSlash(addr), "state.json")
	if err := state.SaveNodeState(statePath, ns); err != nil {
		t.Fatalf("failed to save node state at %s: %v", statePath, err)
	}
}

// discoverNamespace finds the engineer namespace directory under
// .wolfcastle/projects/. There should be exactly one after init.
func discoverNamespace(t *testing.T, dir string) string {
	t.Helper()
	projectsDir := filepath.Join(dir, ".wolfcastle", "system", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		t.Fatalf("cannot read projects dir %s: %v", projectsDir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			return e.Name()
		}
	}
	t.Fatalf("no namespace directory found under %s", projectsDir)
	return ""
}

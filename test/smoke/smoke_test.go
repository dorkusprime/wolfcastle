//go:build smoke

package smoke

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// binaryPath is set by TestMain to the compiled wolfcastle binary.
var binaryPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "wolfcastle-smoke-*")
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

func TestBinaryBuilds(t *testing.T) {
	// The binary was already built in TestMain; verify it exists and is executable.
	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatalf("binary not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("binary is empty")
	}
	// Verify it's executable (has at least one execute bit set)
	if info.Mode()&0111 == 0 {
		t.Fatal("binary is not executable")
	}
}

func TestVersionCommand(t *testing.T) {
	cmd := exec.Command(binaryPath, "version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	output := string(out)
	if !strings.Contains(output, "wolfcastle") {
		t.Errorf("version output does not contain 'wolfcastle': %s", output)
	}
}

func TestHelpCommand(t *testing.T) {
	cmd := exec.Command(binaryPath, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("help command failed: %v", err)
	}
	output := string(out)
	if !strings.Contains(output, "wolfcastle") {
		t.Errorf("help output does not contain 'wolfcastle': %s", output)
	}
	// With command groups, Cobra shows group titles instead of "Available Commands"
	if !strings.Contains(output, "Lifecycle:") {
		t.Errorf("help output does not show command groups: %s", output)
	}
}

func TestInitInTempDir(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binaryPath, "init")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init command failed: %v\noutput: %s", err, string(out))
	}

	// Verify .wolfcastle directory was created
	wcDir := filepath.Join(dir, ".wolfcastle")
	if _, err := os.Stat(wcDir); os.IsNotExist(err) {
		t.Fatal(".wolfcastle directory was not created")
	}

	// Verify base/config.json exists
	cfgPath := filepath.Join(wcDir, "base", "config.json")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Fatal("base/config.json was not created")
	}

	// Verify projects directory exists
	projectsDir := filepath.Join(wcDir, "projects")
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		t.Fatal("projects directory was not created")
	}
}

func TestVersionJSON(t *testing.T) {
	cmd := exec.Command(binaryPath, "--json", "version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("version --json failed: %v", err)
	}
	output := string(out)
	if !strings.Contains(output, `"ok"`) {
		t.Errorf("JSON version output missing 'ok' field: %s", output)
	}
	if !strings.Contains(output, `"version"`) {
		t.Errorf("JSON version output missing 'version' in data: %s", output)
	}
}

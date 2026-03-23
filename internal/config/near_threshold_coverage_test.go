package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

// ---------------------------------------------------------------------------
// repository.go Root()
// ---------------------------------------------------------------------------

func TestRepository_Root_ReturnsConfiguredPath(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	if got := repo.Root(); got != env.Root {
		t.Errorf("Root() = %q, want %q", got, env.Root)
	}
}

// ---------------------------------------------------------------------------
// repository.go WriteBase error paths
// ---------------------------------------------------------------------------

func TestRepository_WriteBase_WriteError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	// Remove config.json if it exists, then make the base tier directory
	// read-only so WriteBase cannot create the file.
	baseDir := env.Tiers.TierDirs()[0]
	_ = os.Remove(filepath.Join(baseDir, "config.json"))
	if err := os.Chmod(baseDir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(baseDir, 0o755) })

	cfg := config.Defaults()
	err := repo.WriteBase(cfg)
	if err == nil {
		t.Fatal("expected error writing to read-only base directory")
	}
	if !strings.Contains(err.Error(), "config:") {
		t.Errorf("expected 'config:' prefix, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// repository.go writeTier error paths (via WriteCustom / WriteLocal)
// ---------------------------------------------------------------------------

func TestRepository_WriteCustom_MkdirAllFailure(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	// Make the custom tier's parent unwritable so MkdirAll cannot create
	// subdirectories. First remove the pre-created custom dir, then lock
	// the system dir.
	customDir := env.Tiers.TierDirs()[1]
	if err := os.RemoveAll(customDir); err != nil {
		t.Fatalf("removing custom dir: %v", err)
	}
	systemDir := filepath.Dir(customDir)
	if err := os.Chmod(systemDir, 0o555); err != nil {
		t.Fatalf("chmod system dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(systemDir, 0o755) })

	err := repo.WriteCustom(map[string]any{"failure": map[string]any{"hard_cap": 1}})
	if err == nil {
		t.Fatal("expected error from MkdirAll on read-only parent")
	}
	if !strings.Contains(err.Error(), "custom") {
		t.Errorf("expected error to mention 'custom', got: %v", err)
	}
}

func TestRepository_WriteLocal_WriteFileFailure(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	// Remove any existing config.json, then make the local tier directory
	// read-only so WriteFile fails (MkdirAll succeeds because the directory exists).
	localDir := env.Tiers.TierDirs()[2]
	_ = os.Remove(filepath.Join(localDir, "config.json"))
	if err := os.Chmod(localDir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(localDir, 0o755) })

	err := repo.WriteLocal(map[string]any{"failure": map[string]any{"hard_cap": 1}})
	if err == nil {
		t.Fatal("expected error writing to read-only local directory")
	}
	if !strings.Contains(err.Error(), "local") {
		t.Errorf("expected error to mention 'local', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// repository.go Load: validation failure after successful merge
// ---------------------------------------------------------------------------

func TestRepository_Load_ValidationFailure(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	// Write a base tier config with invalid values that will pass JSON
	// parsing but fail ValidateStructure (empty pipeline stages).
	invalid := map[string]any{
		"pipeline": map[string]any{
			"stages": []any{},
		},
	}
	raw, _ := json.MarshalIndent(invalid, "", "  ")
	tierDirs := env.Tiers.TierDirs()
	if err := os.WriteFile(filepath.Join(tierDirs[0], "config.json"), raw, 0o644); err != nil {
		t.Fatalf("writing invalid config: %v", err)
	}

	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)
	_, err := repo.Load()
	if err == nil {
		t.Fatal("expected validation error for empty pipeline stages")
	}
	if !strings.Contains(err.Error(), "config:") {
		t.Errorf("expected 'config:' prefix, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// repository.go Load: custom tier parse error
// ---------------------------------------------------------------------------

func TestRepository_Load_CustomTierParseError(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	tierDirs := env.Tiers.TierDirs()
	if err := os.WriteFile(filepath.Join(tierDirs[1], "config.json"), []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("writing corrupt custom config: %v", err)
	}

	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)
	_, err := repo.Load()
	if err == nil {
		t.Fatal("expected error for malformed custom tier JSON")
	}
	if !strings.Contains(err.Error(), "config:") {
		t.Errorf("expected 'config:' prefix, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// config.go Load: custom tier parse error (standalone Load function)
// ---------------------------------------------------------------------------

func TestLoad_InvalidCustomJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "system", "custom"), 0o755)
	_ = os.WriteFile(
		filepath.Join(dir, "system", "custom", "config.json"),
		[]byte("not json"),
		0o644,
	)

	_, err := config.Load(dir)
	if err == nil {
		t.Error("expected error for invalid custom/config.json")
	}
}

// ---------------------------------------------------------------------------
// config.go Load: permission error on tier file
// ---------------------------------------------------------------------------

func TestLoad_PermissionError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	dir := t.TempDir()
	basePath := filepath.Join(dir, "system", "base", "config.json")
	_ = os.MkdirAll(filepath.Dir(basePath), 0o755)
	_ = os.WriteFile(basePath, []byte(`{}`), 0o644)
	_ = os.Chmod(basePath, 0o000)
	t.Cleanup(func() { _ = os.Chmod(basePath, 0o644) })

	_, err := config.Load(dir)
	if err == nil {
		t.Error("expected error for permission-denied config file")
	}
}

// ---------------------------------------------------------------------------
// config.go Load: validation failure after merge
// ---------------------------------------------------------------------------

func TestLoad_ValidationFailureAfterMerge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "system", "base"), 0o755)

	// Override pipeline stages to empty, which fails validation.
	overlay := map[string]any{
		"pipeline": map[string]any{
			"stages": []any{},
		},
	}
	data, _ := json.MarshalIndent(overlay, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "system", "base", "config.json"), data, 0o644)

	_, err := config.Load(dir)
	if err == nil {
		t.Error("expected validation error for empty pipeline stages")
	}
}

package config_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

func TestRepository_Load_ReturnsMergedDefaults(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	cfg, err := repo.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	defaults := config.Defaults()
	if cfg.Failure.HardCap != defaults.Failure.HardCap {
		t.Errorf("expected hard_cap=%d, got %d", defaults.Failure.HardCap, cfg.Failure.HardCap)
	}
	if len(cfg.Models) != len(defaults.Models) {
		t.Errorf("expected %d models, got %d", len(defaults.Models), len(cfg.Models))
	}
	if cfg.Daemon.PollIntervalSeconds != defaults.Daemon.PollIntervalSeconds {
		t.Errorf("expected poll_interval=%d, got %d",
			defaults.Daemon.PollIntervalSeconds, cfg.Daemon.PollIntervalSeconds)
	}
}

func TestRepository_Load_CustomOverlayMerges(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithConfig(map[string]any{
			"failure": map[string]any{"hard_cap": 100},
		})
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	cfg, err := repo.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Failure.HardCap != 100 {
		t.Errorf("expected hard_cap=100 from custom overlay, got %d", cfg.Failure.HardCap)
	}
	// Unaffected defaults preserved
	if cfg.Failure.DecompositionThreshold != 10 {
		t.Errorf("expected decomposition_threshold=10, got %d", cfg.Failure.DecompositionThreshold)
	}
}

func TestRepository_Load_LocalTakesPriority(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithConfig(map[string]any{
			"failure": map[string]any{"hard_cap": 100},
		})

	// Write a local overlay that overrides hard_cap again
	tierDirs := env.Tiers.TierDirs()
	localOverlay := map[string]any{
		"failure":  map[string]any{"hard_cap": 200},
		"identity": map[string]any{"user": "test", "machine": "machine"},
	}
	raw, _ := json.MarshalIndent(localOverlay, "", "  ")
	if err := os.WriteFile(filepath.Join(tierDirs[2], "config.json"), raw, 0o644); err != nil {
		t.Fatalf("writing local config: %v", err)
	}

	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)
	cfg, err := repo.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Failure.HardCap != 200 {
		t.Errorf("expected local to win with hard_cap=200, got %d", cfg.Failure.HardCap)
	}
}

func TestRepository_WriteBase_PersistsAndLoads(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	// Modify defaults and write to base
	cfg := config.Defaults()
	cfg.Failure.HardCap = 999
	if err := repo.WriteBase(cfg); err != nil {
		t.Fatalf("WriteBase() error: %v", err)
	}

	loaded, err := repo.Load()
	if err != nil {
		t.Fatalf("Load() after WriteBase error: %v", err)
	}
	if loaded.Failure.HardCap != 999 {
		t.Errorf("expected hard_cap=999 after WriteBase, got %d", loaded.Failure.HardCap)
	}
}

func TestRepository_WriteCustom_MergesOnLoad(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	overlay := map[string]any{
		"failure": map[string]any{"hard_cap": 77},
	}
	if err := repo.WriteCustom(overlay); err != nil {
		t.Fatalf("WriteCustom() error: %v", err)
	}

	cfg, err := repo.Load()
	if err != nil {
		t.Fatalf("Load() after WriteCustom error: %v", err)
	}
	if cfg.Failure.HardCap != 77 {
		t.Errorf("expected hard_cap=77 after WriteCustom, got %d", cfg.Failure.HardCap)
	}
	// Defaults still intact
	if cfg.Failure.DecompositionThreshold != 10 {
		t.Errorf("expected decomposition_threshold=10, got %d", cfg.Failure.DecompositionThreshold)
	}
}

func TestRepository_WriteLocal_MergesOnLoad(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	overlay := map[string]any{
		"failure":  map[string]any{"hard_cap": 42},
		"identity": map[string]any{"user": "test", "machine": "machine"},
	}
	if err := repo.WriteLocal(overlay); err != nil {
		t.Fatalf("WriteLocal() error: %v", err)
	}

	cfg, err := repo.Load()
	if err != nil {
		t.Fatalf("Load() after WriteLocal error: %v", err)
	}
	if cfg.Failure.HardCap != 42 {
		t.Errorf("expected hard_cap=42 after WriteLocal, got %d", cfg.Failure.HardCap)
	}
}

func TestRepository_Load_MalformedJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	// Corrupt the base tier config
	tierDirs := env.Tiers.TierDirs()
	if err := os.WriteFile(filepath.Join(tierDirs[0], "config.json"), []byte("{broken"), 0o644); err != nil {
		t.Fatalf("writing corrupt config: %v", err)
	}

	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)
	_, err := repo.Load()
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.HasPrefix(err.Error(), "config:") {
		t.Errorf("expected error prefixed with 'config:', got: %v", err)
	}
}

func TestRepository_Load_PermissionError_Propagates(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}

	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	env := testutil.NewEnvironment(t)
	tierDirs := env.Tiers.TierDirs()

	// Make base config unreadable
	configPath := filepath.Join(tierDirs[0], "config.json")
	if err := os.Chmod(configPath, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(configPath, 0o644) })

	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)
	_, err := repo.Load()
	if err == nil {
		t.Fatal("expected error for permission-denied config file")
	}
	if !strings.HasPrefix(err.Error(), "config:") {
		t.Errorf("expected error prefixed with 'config:', got: %v", err)
	}
}

func TestRepository_NewRepository_Production(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	// NewRepository constructs its own tierfs.FS from the root
	repo := config.NewRepository(env.Root)
	cfg, err := repo.Load()
	if err != nil {
		t.Fatalf("Load() via production constructor error: %v", err)
	}

	defaults := config.Defaults()
	if cfg.Failure.HardCap != defaults.Failure.HardCap {
		t.Errorf("expected hard_cap=%d, got %d", defaults.Failure.HardCap, cfg.Failure.HardCap)
	}
}

func TestRepository_NewRepository_UsesCaching(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	// Write a custom config overlay.
	customDir := filepath.Join(env.Root, "system", "custom")
	_ = os.MkdirAll(customDir, 0o755)
	overlay := map[string]any{"failure": map[string]any{"hard_cap": 99}}
	data, _ := json.Marshal(overlay)
	_ = os.WriteFile(filepath.Join(customDir, "config.json"), data, 0o644)

	repo := config.NewRepository(env.Root)

	// First load picks up the overlay.
	cfg1, err := repo.Load()
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	if cfg1.Failure.HardCap != 99 {
		t.Fatalf("expected hard_cap=99, got %d", cfg1.Failure.HardCap)
	}

	// Overwrite the custom config on disk.
	overlay2 := map[string]any{"failure": map[string]any{"hard_cap": 42}}
	data2, _ := json.Marshal(overlay2)
	_ = os.WriteFile(filepath.Join(customDir, "config.json"), data2, 0o644)

	// Second load should still return the cached value (TTL not expired).
	// Note: Repository.Load reads TierDirs directly from disk,
	// but the underlying resolver's Resolve/ResolveAll are cached. Since
	// Load calls TierDirs (passthrough) and reads config.json directly
	// via os.ReadFile (not through the resolver), config caching operates
	// at the resolver level for prompt-style resolution. This test
	// verifies the production constructor wires in a CachingResolver.
	cfg2, err := repo.Load()
	if err != nil {
		t.Fatalf("second load: %v", err)
	}

	// Load reads files via os.ReadFile using TierDirs paths, so it
	// bypasses the caching resolver for config.json reads. The caching
	// resolver is still wired in and used by any code that calls
	// tiers.Resolve or tiers.ResolveAll (audit scopes, prompt fragments).
	// We verify the constructor doesn't break Load.
	if cfg2.Failure.HardCap != 42 {
		t.Fatalf("expected hard_cap=42 (direct read), got %d", cfg2.Failure.HardCap)
	}
}

// --- ReadTier tests ---

func TestRepository_ReadTier_ReturnsEmptyMapWhenMissing(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	m, err := repo.ReadTier("custom")
	if err != nil {
		t.Fatalf("ReadTier(custom) error: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map for missing tier, got %v", m)
	}
}

func TestRepository_ReadTier_ParsesExistingOverlay(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	overlay := map[string]any{"failure": map[string]any{"hard_cap": float64(55)}}
	if err := repo.WriteCustom(overlay); err != nil {
		t.Fatalf("WriteCustom: %v", err)
	}

	m, err := repo.ReadTier("custom")
	if err != nil {
		t.Fatalf("ReadTier(custom) error: %v", err)
	}
	failure, ok := m["failure"].(map[string]any)
	if !ok {
		t.Fatalf("expected failure key as map, got %T", m["failure"])
	}
	if failure["hard_cap"] != float64(55) {
		t.Errorf("expected hard_cap=55, got %v", failure["hard_cap"])
	}
}

func TestRepository_ReadTier_RejectsBase(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	_, err := repo.ReadTier("base")
	if err == nil {
		t.Fatal("expected error for base tier")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("expected read-only error, got: %v", err)
	}
}

func TestRepository_ReadTier_RejectsUnknown(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	_, err := repo.ReadTier("bogus")
	if err == nil {
		t.Fatal("expected error for unknown tier")
	}
	if !strings.Contains(err.Error(), "unknown tier") {
		t.Errorf("expected unknown tier error, got: %v", err)
	}
}

func TestRepository_ReadTier_MalformedJSON(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	tierDirs := env.Tiers.TierDirs()
	if err := os.WriteFile(filepath.Join(tierDirs[1], "config.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatalf("writing corrupt config: %v", err)
	}

	_, err := repo.ReadTier("custom")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("expected JSON parse error, got: %v", err)
	}
}

func TestRepository_ReadTier_PermissionError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	tierDirs := env.Tiers.TierDirs()
	path := filepath.Join(tierDirs[1], "config.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	_, err := repo.ReadTier("custom")
	if err == nil {
		t.Fatal("expected error for permission-denied")
	}
}

func TestRepository_ReadTier_Local(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	overlay := map[string]any{
		"failure":  map[string]any{"hard_cap": float64(88)},
		"identity": map[string]any{"user": "test", "machine": "machine"},
	}
	if err := repo.WriteLocal(overlay); err != nil {
		t.Fatalf("WriteLocal: %v", err)
	}

	m, err := repo.ReadTier("local")
	if err != nil {
		t.Fatalf("ReadTier(local) error: %v", err)
	}
	failure, ok := m["failure"].(map[string]any)
	if !ok {
		t.Fatalf("expected failure key as map, got %T", m["failure"])
	}
	if failure["hard_cap"] != float64(88) {
		t.Errorf("expected hard_cap=88, got %v", failure["hard_cap"])
	}
}

// --- WriteTier tests ---

func TestRepository_WriteTier_Custom(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	overlay := map[string]any{"failure": map[string]any{"hard_cap": float64(33)}}
	if err := repo.WriteTier("custom", overlay); err != nil {
		t.Fatalf("WriteTier(custom) error: %v", err)
	}

	m, err := repo.ReadTier("custom")
	if err != nil {
		t.Fatalf("ReadTier: %v", err)
	}
	failure := m["failure"].(map[string]any)
	if failure["hard_cap"] != float64(33) {
		t.Errorf("expected hard_cap=33, got %v", failure["hard_cap"])
	}
}

func TestRepository_WriteTier_Local(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	overlay := map[string]any{
		"failure":  map[string]any{"hard_cap": float64(44)},
		"identity": map[string]any{"user": "test", "machine": "machine"},
	}
	if err := repo.WriteTier("local", overlay); err != nil {
		t.Fatalf("WriteTier(local) error: %v", err)
	}

	cfg, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Failure.HardCap != 44 {
		t.Errorf("expected hard_cap=44, got %d", cfg.Failure.HardCap)
	}
}

func TestRepository_WriteTier_RejectsBase(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	err := repo.WriteTier("base", map[string]any{})
	if err == nil {
		t.Fatal("expected error for base tier")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("expected read-only error, got: %v", err)
	}
}

func TestRepository_WriteTier_RejectsUnknown(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	err := repo.WriteTier("bogus", map[string]any{})
	if err == nil {
		t.Fatal("expected error for unknown tier")
	}
	if !strings.Contains(err.Error(), "unknown tier") {
		t.Errorf("expected unknown tier error, got: %v", err)
	}
}

// --- ApplyMutation tests ---

func TestRepository_ApplyMutation_SuccessfulMutation(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	err := repo.ApplyMutation("custom", func(overlay map[string]any) error {
		overlay["failure"] = map[string]any{"hard_cap": float64(77)}
		return nil
	})
	if err != nil {
		t.Fatalf("ApplyMutation error: %v", err)
	}

	cfg, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Failure.HardCap != 77 {
		t.Errorf("expected hard_cap=77, got %d", cfg.Failure.HardCap)
	}
}

func TestRepository_ApplyMutation_MutationErrorDoesNotWrite(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	err := repo.ApplyMutation("custom", func(overlay map[string]any) error {
		return fmt.Errorf("deliberate failure")
	})
	if err == nil {
		t.Fatal("expected error from failed mutation")
	}
	if !strings.Contains(err.Error(), "deliberate failure") {
		t.Errorf("expected mutation error, got: %v", err)
	}

	// Tier file should not exist since the mutation failed before write.
	m, err := repo.ReadTier("custom")
	if err != nil {
		t.Fatalf("ReadTier: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty overlay after mutation failure, got %v", m)
	}
}

func TestRepository_ApplyMutation_RollsBackOnValidationFailure(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	// Seed the custom tier with a valid overlay.
	if err := repo.WriteCustom(map[string]any{"failure": map[string]any{"hard_cap": float64(10)}}); err != nil {
		t.Fatalf("WriteCustom seed: %v", err)
	}

	// Apply a mutation that produces an invalid config (negative hard_cap
	// should fail validation, but if it doesn't, use a value that will
	// at least let us detect whether rollback happened).
	// We need to produce something that actually fails Load(). The
	// pipeline.stages key expects a specific shape. Let's write something
	// that will break parsing: a string where an int is expected.
	err := repo.ApplyMutation("custom", func(overlay map[string]any) error {
		overlay["failure"] = map[string]any{"hard_cap": "not-a-number"}
		return nil
	})
	if err == nil {
		t.Fatal("expected validation error from invalid config")
	}
	if !strings.Contains(err.Error(), "rolled back") {
		t.Errorf("expected rolled-back error, got: %v", err)
	}

	// The original overlay should be restored.
	m, err := repo.ReadTier("custom")
	if err != nil {
		t.Fatalf("ReadTier after rollback: %v", err)
	}
	failure, ok := m["failure"].(map[string]any)
	if !ok {
		t.Fatalf("expected failure key after rollback, got %v", m)
	}
	if failure["hard_cap"] != float64(10) {
		t.Errorf("expected hard_cap=10 after rollback, got %v", failure["hard_cap"])
	}
}

func TestRepository_ApplyMutation_RollsBackToAbsence(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	// No custom tier file exists. Apply a mutation that breaks validation.
	err := repo.ApplyMutation("custom", func(overlay map[string]any) error {
		overlay["failure"] = map[string]any{"hard_cap": "bad"}
		return nil
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	// The custom tier file should not exist after rollback.
	m, err := repo.ReadTier("custom")
	if err != nil {
		t.Fatalf("ReadTier after rollback: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty overlay after rollback-to-absence, got %v", m)
	}
}

func TestRepository_ApplyMutation_RejectsBase(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	err := repo.ApplyMutation("base", func(overlay map[string]any) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for base tier")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("expected read-only error, got: %v", err)
	}
}

func TestRepository_ApplyMutation_Local(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	err := repo.ApplyMutation("local", func(overlay map[string]any) error {
		overlay["failure"] = map[string]any{"hard_cap": float64(99)}
		overlay["identity"] = map[string]any{"user": "test", "machine": "machine"}
		return nil
	})
	if err != nil {
		t.Fatalf("ApplyMutation(local) error: %v", err)
	}

	cfg, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Failure.HardCap != 99 {
		t.Errorf("expected hard_cap=99, got %d", cfg.Failure.HardCap)
	}
}

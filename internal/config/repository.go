package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/tierfs"
)

// Repository provides three-tier configuration resolution backed by
// a tierfs.Resolver. Each Load reads fresh from disk, merging defaults with
// base, custom, and local overlays in ascending priority.
type Repository struct {
	tiers tierfs.Resolver
	root  string
}

// DefaultConfigCacheTTL is the TTL for the caching resolver wrapping
// tier resolution in production config repositories.
const DefaultConfigCacheTTL = 30 * time.Second

// NewRepository creates a Repository rooted at wolfcastleRoot.
// The tierfs.FS is constructed over the "system" subdirectory and wrapped
// with a CachingResolver for TTL-based read caching.
func NewRepository(wolfcastleRoot string) *Repository {
	fs := tierfs.New(filepath.Join(wolfcastleRoot, "system"))
	return NewRepositoryWithTiers(
		tierfs.NewCachingResolver(fs, DefaultConfigCacheTTL),
		wolfcastleRoot,
	)
}

// NewRepositoryWithTiers creates a Repository with an injected
// resolver, allowing tests to supply fixture-backed implementations.
func NewRepositoryWithTiers(tiers tierfs.Resolver, root string) *Repository {
	return &Repository{tiers: tiers, root: root}
}

// Root returns the wolfcastle root directory path (the .wolfcastle directory).
func (r *Repository) Root() string {
	return r.root
}

// Load resolves the merged configuration across all tiers. Missing tier
// files are silently skipped; permission and parse errors propagate.
// Unknown fields in tier files are collected as warnings on Config.Warnings.
func (r *Repository) Load() (*Config, error) {
	result, err := structToMap(Defaults())
	if err != nil {
		return nil, fmt.Errorf("config: marshaling defaults: %w", err)
	}

	var warnings []string

	for i, dir := range r.tiers.TierDirs() {
		path := filepath.Join(dir, "config.json")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("config: reading %s: %w", path, err)
		}

		// Check this tier's raw JSON for unknown fields
		tierLabel := tierfs.TierNames[i] + "/config.json"
		warnings = append(warnings, checkUnknownFields(data, tierLabel)...)

		var overlay map[string]any
		if err := json.Unmarshal(data, &overlay); err != nil {
			return nil, fmt.Errorf("config: parsing %s: %w", path, err)
		}
		result = DeepMerge(result, overlay)
	}

	// Apply schema migrations if the merged config is behind CurrentVersion.
	migrated, migrationDescs, migErr := MigrateConfig(result)
	if migErr != nil {
		return nil, fmt.Errorf("config: migration: %w", migErr)
	}
	result = migrated
	for _, desc := range migrationDescs {
		warnings = append(warnings, "config migrated: "+desc)
	}

	merged, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("config: marshaling merged config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(merged, &cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshaling merged config: %w", err)
	}

	cfg.Warnings = warnings

	if err := ValidateStructure(&cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	return &cfg, nil
}

// BaseVersion reads just the version field from the base tier's config.json.
// Returns 0 if the file does not exist or the version field is absent.
func (r *Repository) BaseVersion() int {
	dirs := r.tiers.TierDirs()
	if len(dirs) == 0 {
		return 0
	}
	path := filepath.Join(dirs[0], "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return 0
	}
	return configVersion(raw)
}

// WriteBase marshals the full Config to indented JSON and writes it to
// the base tier via tiers.WriteBase.
func (r *Repository) WriteBase(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshaling base config: %w", err)
	}
	if err := r.tiers.WriteBase("config.json", data); err != nil {
		return fmt.Errorf("config: writing base config: %w", err)
	}
	return nil
}

// WriteCustom marshals a partial overlay to indented JSON and writes it
// to the custom tier's config.json, creating parent directories as needed.
func (r *Repository) WriteCustom(data map[string]any) error {
	return r.writeTier(1, data, "custom")
}

// WriteLocal marshals a partial overlay to indented JSON and writes it
// to the local tier's config.json, creating parent directories as needed.
func (r *Repository) WriteLocal(data map[string]any) error {
	return r.writeTier(2, data, "local")
}

// writeTier handles the shared logic for WriteCustom and WriteLocal.
func (r *Repository) writeTier(index int, overlay map[string]any, label string) error {
	raw, err := json.MarshalIndent(overlay, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshaling %s config: %w", label, err)
	}
	path := filepath.Join(r.tiers.TierDirs()[index], "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("config: creating %s directory: %w", label, err)
	}
	if err := atomicWriteFile(path, raw); err != nil {
		return fmt.Errorf("config: writing %s config: %w", label, err)
	}
	return nil
}

// atomicWriteFile writes data to path atomically via temp file + rename.
func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".wolfcastle-tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

// tierIndex maps a tier name to its index in TierDirs, rejecting "base"
// since base is managed exclusively through WriteBase with a full Config.
func tierIndex(tier string) (int, error) {
	switch tier {
	case "custom":
		return 1, nil
	case "local":
		return 2, nil
	case "base":
		return 0, fmt.Errorf("config: base tier is read-only; use WriteBase with a full Config")
	default:
		return 0, fmt.Errorf("config: unknown tier %q; expected \"custom\" or \"local\"", tier)
	}
}

// ReadTier reads and parses a single tier's config.json overlay. Returns an
// empty map if the file does not exist. Accepts "custom" or "local"; rejects
// "base" because the base tier is written only via WriteBase with a full Config.
func (r *Repository) ReadTier(tier string) (map[string]any, error) {
	idx, err := tierIndex(tier)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(r.tiers.TierDirs()[idx], "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("config: reading %s/config.json: %w", tier, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("config: %s/config.json is not valid JSON: %w", tier, err)
	}
	return m, nil
}

// WriteTier writes an overlay to the specified tier. Dispatches to WriteCustom
// or WriteLocal based on tier name. Rejects "base".
func (r *Repository) WriteTier(tier string, overlay map[string]any) error {
	switch tier {
	case "custom":
		return r.WriteCustom(overlay)
	case "local":
		return r.WriteLocal(overlay)
	case "base":
		return fmt.Errorf("config: base tier is read-only; use WriteBase with a full Config")
	default:
		return fmt.Errorf("config: unknown tier %q; expected \"custom\" or \"local\"", tier)
	}
}

// ApplyMutation performs a read-modify-write-validate cycle on a tier overlay.
// It reads the current overlay, calls mutate to modify it in-place, writes it
// back, then runs Load() to validate the merged result. If validation fails,
// the original tier file contents are restored and the validation error is returned.
func (r *Repository) ApplyMutation(tier string, mutate func(overlay map[string]any) error) error {
	// Validate the tier name before doing anything.
	idx, err := tierIndex(tier)
	if err != nil {
		return err
	}

	// Snapshot the original file contents for rollback.
	path := filepath.Join(r.tiers.TierDirs()[idx], "config.json")
	original, readErr := os.ReadFile(path)
	existed := readErr == nil
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("config: reading %s/config.json for snapshot: %w", tier, readErr)
	}

	// Read the current overlay.
	overlay, err := r.ReadTier(tier)
	if err != nil {
		return err
	}

	// Apply the mutation.
	if err := mutate(overlay); err != nil {
		return fmt.Errorf("config: mutation failed: %w", err)
	}

	// Write the mutated overlay.
	if err := r.WriteTier(tier, overlay); err != nil {
		return err
	}

	// Validate the merged result.
	if _, err := r.Load(); err != nil {
		// Rollback: restore original file contents.
		if existed {
			_ = atomicWriteFile(path, original)
		} else {
			_ = os.Remove(path)
		}
		return fmt.Errorf("config: validation failed, changes rolled back: %w", err)
	}

	return nil
}

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/tierfs"
)

// ConfigRepository provides three-tier configuration resolution backed by
// a tierfs.Resolver. Each Load reads fresh from disk, merging defaults with
// base, custom, and local overlays in ascending priority.
type ConfigRepository struct {
	tiers tierfs.Resolver
	root  string
}

// DefaultConfigCacheTTL is the TTL for the caching resolver wrapping
// tier resolution in production config repositories.
const DefaultConfigCacheTTL = 30 * time.Second

// NewConfigRepository creates a ConfigRepository rooted at wolfcastleRoot.
// The tierfs.FS is constructed over the "system" subdirectory and wrapped
// with a CachingResolver for TTL-based read caching.
func NewConfigRepository(wolfcastleRoot string) *ConfigRepository {
	fs := tierfs.New(filepath.Join(wolfcastleRoot, "system"))
	return NewConfigRepositoryWithTiers(
		tierfs.NewCachingResolver(fs, DefaultConfigCacheTTL),
		wolfcastleRoot,
	)
}

// NewConfigRepositoryWithTiers creates a ConfigRepository with an injected
// resolver, allowing tests to supply fixture-backed implementations.
func NewConfigRepositoryWithTiers(tiers tierfs.Resolver, root string) *ConfigRepository {
	return &ConfigRepository{tiers: tiers, root: root}
}

// Root returns the wolfcastle root directory path (the .wolfcastle directory).
func (r *ConfigRepository) Root() string {
	return r.root
}

// Load resolves the merged configuration across all tiers. Missing tier
// files are silently skipped; permission and parse errors propagate.
func (r *ConfigRepository) Load() (*Config, error) {
	result, err := structToMap(Defaults())
	if err != nil {
		return nil, fmt.Errorf("config: marshaling defaults: %w", err)
	}

	for _, dir := range r.tiers.TierDirs() {
		path := filepath.Join(dir, "config.json")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("config: reading %s: %w", path, err)
		}
		var overlay map[string]any
		if err := json.Unmarshal(data, &overlay); err != nil {
			return nil, fmt.Errorf("config: parsing %s: %w", path, err)
		}
		result = DeepMerge(result, overlay)
	}

	merged, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("config: marshaling merged config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(merged, &cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshaling merged config: %w", err)
	}

	if err := ValidateStructure(&cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	return &cfg, nil
}

// WriteBase marshals the full Config to indented JSON and writes it to
// the base tier via tiers.WriteBase.
func (r *ConfigRepository) WriteBase(cfg *Config) error {
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
func (r *ConfigRepository) WriteCustom(data map[string]any) error {
	return r.writeTier(1, data, "custom")
}

// WriteLocal marshals a partial overlay to indented JSON and writes it
// to the local tier's config.json, creating parent directories as needed.
func (r *ConfigRepository) WriteLocal(data map[string]any) error {
	return r.writeTier(2, data, "local")
}

// writeTier handles the shared logic for WriteCustom and WriteLocal.
func (r *ConfigRepository) writeTier(index int, overlay map[string]any, label string) error {
	raw, err := json.MarshalIndent(overlay, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshaling %s config: %w", label, err)
	}
	path := filepath.Join(r.tiers.TierDirs()[index], "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("config: creating %s directory: %w", label, err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("config: writing %s config: %w", label, err)
	}
	return nil
}

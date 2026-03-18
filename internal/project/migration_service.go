package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/config"
)

// MigrationService handles layout and config migrations for users upgrading
// from older wolfcastle directory structures. Both methods are idempotent:
// running them against an already-migrated directory is a no-op.
type MigrationService struct {
	config *config.ConfigRepository
	root   string // path to .wolfcastle/
}

// MigrateDirectoryLayout moves the pre-ADR-077 flat directory layout into
// system/. If system/ already exists, the call is a no-op.
func (m *MigrationService) MigrateDirectoryLayout() error {
	systemDir := filepath.Join(m.root, "system")
	if _, err := os.Stat(systemDir); err == nil {
		return nil
	}

	// Check if old layout exists by looking for base/ at the root.
	oldBase := filepath.Join(m.root, "base")
	if _, err := os.Stat(oldBase); os.IsNotExist(err) {
		// No old layout to migrate. Just ensure system/ exists.
		return os.MkdirAll(systemDir, 0755)
	}

	if err := os.MkdirAll(systemDir, 0755); err != nil {
		return fmt.Errorf("creating system/: %w", err)
	}

	dirsToMove := []string{"base", "custom", "local", "projects", "logs"}
	for _, d := range dirsToMove {
		src := filepath.Join(m.root, d)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		dst := filepath.Join(systemDir, d)
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("migrating %s to system/%s: %w", d, d, err)
		}
	}

	filesToMove := []string{"wolfcastle.pid", "stop", "daemon.log", "daemon.meta.json"}
	for _, f := range filesToMove {
		src := filepath.Join(m.root, f)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		dst := filepath.Join(systemDir, f)
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("migrating %s to system/%s: %w", f, f, err)
		}
	}

	return nil
}

// MigrateOldConfig migrates pre-ADR-063 config files from the wolfcastle root
// into the three-tier layout under system/. Root config.json goes to
// system/custom/config.json (if absent). config.local.json is deep-merged
// into system/local/config.json.
func (m *MigrationService) MigrateOldConfig() error {
	// Migrate root config.json -> system/custom/config.json
	oldCfgPath := filepath.Join(m.root, "config.json")
	if _, err := os.Stat(oldCfgPath); err == nil {
		customDir := filepath.Join(m.root, "system", "custom")
		_ = os.MkdirAll(customDir, 0755)
		customCfgPath := filepath.Join(customDir, "config.json")
		if _, err := os.Stat(customCfgPath); os.IsNotExist(err) {
			data, err := os.ReadFile(oldCfgPath)
			if err != nil {
				return fmt.Errorf("reading old config.json: %w", err)
			}
			if err := os.WriteFile(customCfgPath, data, 0644); err != nil {
				return fmt.Errorf("writing system/custom/config.json: %w", err)
			}
		}
		_ = os.Remove(oldCfgPath)
	}

	// Migrate config.local.json -> system/local/config.json
	oldLocalPath := filepath.Join(m.root, "config.local.json")
	if _, err := os.Stat(oldLocalPath); err == nil {
		localDir := filepath.Join(m.root, "system", "local")
		_ = os.MkdirAll(localDir, 0755)
		localCfgPath := filepath.Join(localDir, "config.json")

		oldData, err := os.ReadFile(oldLocalPath)
		if err != nil {
			return fmt.Errorf("reading old config.local.json: %w", err)
		}
		var oldLocal map[string]any
		if err := json.Unmarshal(oldData, &oldLocal); err != nil {
			return fmt.Errorf("old config.local.json is not valid JSON: %w", err)
		}

		existing := map[string]any{}
		if data, readErr := os.ReadFile(localCfgPath); readErr == nil {
			_ = json.Unmarshal(data, &existing)
		}

		merged := config.DeepMerge(existing, oldLocal)
		mergedData, err := json.MarshalIndent(merged, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling migrated local config: %w", err)
		}
		mergedData = append(mergedData, '\n')
		if err := os.WriteFile(localCfgPath, mergedData, 0644); err != nil {
			return fmt.Errorf("writing system/local/config.json: %w", err)
		}

		_ = os.Remove(oldLocalPath)
	}

	return nil
}

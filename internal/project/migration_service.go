package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/tierfs"
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
	systemDir := filepath.Join(m.root, tierfs.SystemPrefix)
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

	// Tier directories derived from tierfs (the canonical source of truth),
	// plus non-tier directories that also live under system/.
	dirsToMove := append(append([]string{}, tierfs.TierNames...), "projects", "logs")
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

// MigratePromptLayout moves flat prompt files into the new subdirectory
// structure (stages/, audits/). If prompts/stages/ already exists, the call
// is a no-op. This handles all three tiers (base, custom, local) so that
// user overrides in higher tiers continue to resolve correctly.
func (m *MigrationService) MigratePromptLayout() error {
	// Stage prompts: files that belong under prompts/stages/
	stageFiles := []string{
		"intake.md", "execute.md", "intake-planning.md",
		"plan-initial.md", "plan-amend.md", "plan-review.md", "plan-remediate.md",
	}
	// Audit prompts: files that belong under prompts/audits/
	auditFiles := []string{"audit.md"}

	for _, tier := range tierfs.TierNames {
		promptsDir := filepath.Join(m.root, tierfs.SystemPrefix, tier, "prompts")
		if _, err := os.Stat(promptsDir); os.IsNotExist(err) {
			continue
		}

		// If stages/ already exists in this tier, skip it entirely
		stagesDir := filepath.Join(promptsDir, "stages")
		if _, err := os.Stat(stagesDir); err == nil {
			continue
		}

		// Move stage files
		for _, f := range stageFiles {
			src := filepath.Join(promptsDir, f)
			if _, err := os.Stat(src); os.IsNotExist(err) {
				continue
			}
			if err := os.MkdirAll(stagesDir, 0755); err != nil {
				return fmt.Errorf("creating %s/prompts/stages/: %w", tier, err)
			}
			if err := os.Rename(src, filepath.Join(stagesDir, f)); err != nil {
				return fmt.Errorf("migrating %s/prompts/%s to stages/: %w", tier, f, err)
			}
		}

		// Move audit files
		for _, f := range auditFiles {
			src := filepath.Join(promptsDir, f)
			if _, err := os.Stat(src); os.IsNotExist(err) {
				continue
			}
			auditsDir := filepath.Join(promptsDir, "audits")
			if err := os.MkdirAll(auditsDir, 0755); err != nil {
				return fmt.Errorf("creating %s/prompts/audits/: %w", tier, err)
			}
			if err := os.Rename(src, filepath.Join(auditsDir, f)); err != nil {
				return fmt.Errorf("migrating %s/prompts/%s to audits/: %w", tier, f, err)
			}
		}
	}

	return nil
}

// MigrateStagesFormat converts pipeline.stages from the old array format to
// the new dict format in each tier's config.json. If stages is already a map
// or absent, the tier is skipped (idempotent).
func (m *MigrationService) MigrateStagesFormat() error {
	for _, tier := range tierfs.TierNames {
		cfgPath := filepath.Join(m.root, tierfs.SystemPrefix, tier, "config.json")

		data, err := os.ReadFile(cfgPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("reading %s/config.json: %w", tier, err)
		}

		var root map[string]any
		if err := json.Unmarshal(data, &root); err != nil {
			// Invalid JSON is not our problem; other migrations will report it.
			continue
		}

		pipelineRaw, ok := root["pipeline"]
		if !ok {
			continue
		}
		pipeline, ok := pipelineRaw.(map[string]any)
		if !ok {
			continue
		}

		stagesRaw, ok := pipeline["stages"]
		if !ok {
			continue
		}

		// Already a map: nothing to migrate.
		if _, isMap := stagesRaw.(map[string]any); isMap {
			continue
		}

		stagesArr, ok := stagesRaw.([]any)
		if !ok {
			continue
		}

		newStages := map[string]any{}
		var stageOrder []string

		for _, elemRaw := range stagesArr {
			elem, ok := elemRaw.(map[string]any)
			if !ok {
				continue
			}
			nameRaw, hasName := elem["name"]
			if !hasName {
				continue
			}
			name, ok := nameRaw.(string)
			if !ok || name == "" {
				continue
			}

			// Remove the name field from the stage object.
			stage := make(map[string]any, len(elem)-1)
			for k, v := range elem {
				if k != "name" {
					stage[k] = v
				}
			}

			newStages[name] = stage
			// Duplicate names: last wins in both the map and stage_order.
			found := false
			for _, existing := range stageOrder {
				if existing == name {
					found = true
					break
				}
			}
			if !found {
				stageOrder = append(stageOrder, name)
			}
		}

		pipeline["stages"] = newStages

		// Preserve existing stage_order if present; otherwise generate from array order.
		if _, hasOrder := pipeline["stage_order"]; !hasOrder && len(stageOrder) > 0 {
			pipeline["stage_order"] = stageOrder
		}

		out, err := json.MarshalIndent(root, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling migrated %s/config.json: %w", tier, err)
		}
		out = append(out, '\n')
		if err := os.WriteFile(cfgPath, out, 0644); err != nil {
			return fmt.Errorf("writing migrated %s/config.json: %w", tier, err)
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
		customDir := filepath.Join(m.root, tierfs.SystemPrefix, tierfs.TierNames[1])
		if err := os.MkdirAll(customDir, 0755); err != nil {
			return fmt.Errorf("creating system/custom/: %w", err)
		}
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
		localDir := filepath.Join(m.root, tierfs.SystemPrefix, tierfs.TierNames[2])
		if err := os.MkdirAll(localDir, 0755); err != nil {
			return fmt.Errorf("creating system/local/: %w", err)
		}
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
			if err := json.Unmarshal(data, &existing); err != nil {
				return fmt.Errorf("system/local/config.json is not valid JSON: %w", err)
			}
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

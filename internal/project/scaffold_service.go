package project

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// promptWriter is the subset of pipeline.PromptRepository that ScaffoldService
// needs. Defined here as an interface to avoid an import cycle between project
// and pipeline (pipeline's tests import project for WriteBasePrompts).
type promptWriter interface {
	WriteAllBase(templates fs.FS) error
}

// ScaffoldService owns creation and regeneration of the .wolfcastle/ directory
// tree. Init builds the full structure from nothing; Reinit tears down and
// rebuilds the base tier while preserving custom and local content.
type ScaffoldService struct {
	config  *config.ConfigRepository
	prompts promptWriter
	daemon  any    // *daemon.DaemonRepository; stored for future use, typed as any to avoid import cycle
	root    string // path to .wolfcastle/
}

// NewScaffoldService creates a ScaffoldService with the given dependencies.
// No filesystem work happens at construction time.
func NewScaffoldService(
	cfg *config.ConfigRepository,
	prompts promptWriter,
	dmn any,
	root string,
) *ScaffoldService {
	return &ScaffoldService{
		config:  cfg,
		prompts: prompts,
		daemon:  dmn,
		root:    root,
	}
}

// Init creates the full .wolfcastle/ directory structure for a fresh
// wolfcastle init. The caller should verify that .wolfcastle/ does not
// already exist before calling Init.
func (s *ScaffoldService) Init(identity *config.Identity) error {
	dirs := []string{
		"system/base/prompts",
		"system/base/rules",
		"system/base/audits",
		"system/custom",
		"system/local",
		"system/projects",
		"system/logs",
		"archive",
		"artifacts",
		"docs/decisions",
		"docs/specs",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(s.root, d), 0755); err != nil {
			return fmt.Errorf("scaffold: creating directory %s: %w", d, err)
		}
	}

	// Write .gitignore with whitelist pattern
	gitignore := `# Ignore everything by default, then whitelist tracked directories.
# Git requires each directory level to be explicitly unignored.
*
!.gitignore

# Custom config overrides (user-editable)
!system/
!system/custom/
!system/custom/**/
!system/custom/**

# Project state (task trees, inbox, specs, ADRs)
!system/projects/
!system/projects/**/
!system/projects/**

# Archived projects
!archive/
!archive/**/
!archive/**

# Specs and ADRs
!docs/
!docs/**/
!docs/**
`
	if err := os.WriteFile(filepath.Join(s.root, ".gitignore"), []byte(gitignore), 0644); err != nil {
		return fmt.Errorf("scaffold: writing .gitignore: %w", err)
	}

	// Write base config from defaults (identity belongs only in local tier)
	defaults := config.Defaults()
	defaults.Identity = nil
	if err := s.config.WriteBase(defaults); err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}

	// Write empty custom config for teams to edit
	if err := s.config.WriteCustom(map[string]any{}); err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}

	// Write local config with identity
	localOverlay := map[string]any{
		"identity": map[string]any{
			"user":    identity.User,
			"machine": identity.Machine,
		},
	}
	if err := s.config.WriteLocal(localOverlay); err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}

	// Create namespace projects directory
	nsDir := identity.ProjectsDir(s.root)
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		return fmt.Errorf("scaffold: creating namespace directory: %w", err)
	}

	// Write empty root index
	idx := state.NewRootIndex()
	idxData, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("scaffold: marshaling root index: %w", err)
	}
	idxData = append(idxData, '\n')
	if err := os.WriteFile(filepath.Join(nsDir, "state.json"), idxData, 0644); err != nil {
		return fmt.Errorf("scaffold: writing root index: %w", err)
	}

	// Extract embedded templates into the base tier
	if err := s.writeBasePrompts(); err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}

	return nil
}

// Reinit regenerates the base tier and refreshes identity. Runs migrations
// first so users upgrading from older directory structures land in the
// correct layout before regeneration begins.
func (s *ScaffoldService) Reinit() error {
	// Run migrations (best-effort; errors logged but not propagated)
	m := &MigrationService{config: s.config, root: s.root}
	_ = m.MigrateDirectoryLayout()
	_ = m.MigrateOldConfig()

	// Remove and recreate system/base/
	baseDir := filepath.Join(s.root, "system", "base")
	if err := os.RemoveAll(baseDir); err != nil {
		return fmt.Errorf("scaffold: removing system/base/: %w", err)
	}
	baseDirs := []string{
		"system/base/prompts",
		"system/base/rules",
		"system/base/audits",
	}
	for _, d := range baseDirs {
		if err := os.MkdirAll(filepath.Join(s.root, d), 0755); err != nil {
			return fmt.Errorf("scaffold: creating %s: %w", d, err)
		}
	}

	// Regenerate base config
	defaults := config.Defaults()
	defaults.Identity = nil
	if err := s.config.WriteBase(defaults); err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}

	// Extract embedded templates
	if err := s.writeBasePrompts(); err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}

	// Ensure custom config exists
	customCfgPath := filepath.Join(s.root, "system", "custom", "config.json")
	if _, err := os.Stat(customCfgPath); os.IsNotExist(err) {
		if err := s.config.WriteCustom(map[string]any{}); err != nil {
			return fmt.Errorf("scaffold: %w", err)
		}
	}

	// Refresh identity in local config, preserving other keys
	localPath := filepath.Join(s.root, "system", "local", "config.json")
	localCfg := map[string]any{}
	if data, err := os.ReadFile(localPath); err == nil {
		if err := json.Unmarshal(data, &localCfg); err != nil {
			return fmt.Errorf("scaffold: system/local/config.json is not valid JSON: %w", err)
		}
	}

	detected := config.DetectIdentity()
	localCfg["identity"] = map[string]any{
		"user":    detected.User,
		"machine": detected.Machine,
	}
	if err := s.config.WriteLocal(localCfg); err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}

	return nil
}

// writeBasePrompts extracts embedded templates into the base tier via the
// PromptRepository. The embedded FS is rooted at "templates/", so we use
// fs.Sub to strip that prefix before handing it to WriteAllBase.
func (s *ScaffoldService) writeBasePrompts() error {
	sub, err := fs.Sub(Templates, "templates")
	if err != nil {
		return fmt.Errorf("extracting templates sub-FS: %w", err)
	}
	return s.prompts.WriteAllBase(sub)
}

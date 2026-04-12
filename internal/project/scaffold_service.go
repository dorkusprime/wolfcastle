package project

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tierfs"
)

// promptWriter is the subset of pipeline.PromptRepository that ScaffoldService
// needs. Defined here as an interface to avoid an import cycle between project
// and pipeline.
type promptWriter interface {
	WriteAllBase(templates fs.FS) error
}

// scaffoldFiles maps embedded template paths (under templates/scaffold/) to
// their output paths relative to the .wolfcastle/ root. These files orient
// humans who browse the directory structure after running wolfcastle init.
var scaffoldFiles = map[string]string{
	"templates/scaffold/gitignore.tmpl":              ".gitignore",
	"templates/scaffold/readme-root.md.tmpl":         "README.md",
	"templates/scaffold/readme-system.md.tmpl":       "system/README.md",
	"templates/scaffold/readme-base-prompts.md.tmpl": "system/base/prompts/README.md",
	"templates/scaffold/readme-docs.md.tmpl":         "docs/README.md",
	"templates/scaffold/readme-archive.md.tmpl":      "archive/README.md",
}

// ScaffoldService owns creation and regeneration of the .wolfcastle/ directory
// tree. Init builds the full structure from nothing; Reinit tears down and
// rebuilds the base tier while preserving custom and local content.
type ScaffoldService struct {
	config  *config.Repository
	prompts promptWriter
	daemon  any    // *daemon.Repository; stored for future use, typed as any to avoid import cycle
	root    string // path to .wolfcastle/
}

// NewScaffoldService creates a ScaffoldService with the given dependencies.
// No filesystem work happens at construction time.
func NewScaffoldService(
	cfg *config.Repository,
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
	// Derive tier directories from tierfs (the canonical source of
	// truth for tier names), then add scaffold-specific subdirectories.
	dirs := append([]string{}, tierfs.SystemTierPaths()...)
	// Base tier needs subdirectories for prompts (with stage, class, and
	// audit sub-categories), rules, and audits
	baseTier := tierfs.SystemPrefix + "/" + tierfs.TierNames[0]
	dirs = append(dirs,
		baseTier+"/prompts",
		baseTier+"/prompts/stages",
		baseTier+"/prompts/classes",
		baseTier+"/prompts/audits",
		baseTier+"/rules",
		baseTier+"/audits",
	)
	dirs = append(dirs,
		"system/projects",
		"system/logs",
		"archive",
		"artifacts",
		"docs/decisions",
		"docs/specs",
	)
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(s.root, d), 0755); err != nil {
			return fmt.Errorf("scaffold: creating directory %s: %w", d, err)
		}
	}

	// Write scaffold files (.gitignore, READMEs) from embedded templates.
	if err := s.writeScaffoldFiles(); err != nil {
		return err
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
	// Run migrations before regeneration begins.
	m := &MigrationService{config: s.config, root: s.root}
	if err := m.MigrateDirectoryLayout(); err != nil {
		return fmt.Errorf("scaffold: migrating directory layout: %w", err)
	}
	if err := m.MigrateOldConfig(); err != nil {
		return fmt.Errorf("scaffold: migrating old config: %w", err)
	}
	if err := m.MigrateStagesFormat(); err != nil {
		return fmt.Errorf("scaffold: migrating stages format: %w", err)
	}
	if err := m.MigratePromptLayout(); err != nil {
		return fmt.Errorf("scaffold: migrating prompt layout: %w", err)
	}

	// Remove and recreate the base tier directory
	baseTierPath := tierfs.SystemPrefix + "/" + tierfs.TierNames[0]
	baseDir := filepath.Join(s.root, baseTierPath)
	if err := os.RemoveAll(baseDir); err != nil {
		return fmt.Errorf("scaffold: removing %s/: %w", baseTierPath, err)
	}
	baseDirs := []string{
		baseTierPath + "/prompts",
		baseTierPath + "/prompts/stages",
		baseTierPath + "/prompts/classes",
		baseTierPath + "/prompts/audits",
		baseTierPath + "/rules",
		baseTierPath + "/audits",
	}
	for _, d := range baseDirs {
		if err := os.MkdirAll(filepath.Join(s.root, d), 0755); err != nil {
			return fmt.Errorf("scaffold: creating %s: %w", d, err)
		}
	}

	// Ensure scaffold directories exist (Init creates them, but they may
	// be missing if the .wolfcastle/ tree was created minimally).
	for _, d := range []string{"docs", "archive"} {
		_ = os.MkdirAll(filepath.Join(s.root, d), 0755)
	}

	// Restore scaffold files destroyed by the base-tier teardown.
	if err := s.writeScaffoldFiles(); err != nil {
		return err
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
	customCfgPath := filepath.Join(s.root, tierfs.SystemPrefix, tierfs.TierNames[1], "config.json")
	if _, err := os.Stat(customCfgPath); os.IsNotExist(err) {
		if err := s.config.WriteCustom(map[string]any{}); err != nil {
			return fmt.Errorf("scaffold: %w", err)
		}
	}

	// Refresh identity in local config, preserving other keys
	localPath := filepath.Join(s.root, tierfs.SystemPrefix, tierfs.TierNames[2], "config.json")
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

// writeScaffoldFiles reads each scaffold template from the embedded FS and
// writes it to the corresponding output path under .wolfcastle/.
func (s *ScaffoldService) writeScaffoldFiles() error {
	for tmpl, dest := range scaffoldFiles {
		content, err := Templates.ReadFile(tmpl)
		if err != nil {
			return fmt.Errorf("scaffold: reading template %s: %w", tmpl, err)
		}
		if err := os.WriteFile(filepath.Join(s.root, dest), content, 0644); err != nil {
			return fmt.Errorf("scaffold: writing %s: %w", dest, err)
		}
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

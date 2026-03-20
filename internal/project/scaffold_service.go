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
// and pipeline (pipeline's tests import project for WriteBasePrompts).
type promptWriter interface {
	WriteAllBase(templates fs.FS) error
}

// scaffoldREADMEs maps relative paths (under .wolfcastle/) to README content
// that Init writes during scaffold. These orient humans who browse the
// directory structure after running wolfcastle init.
var scaffoldREADMEs = map[string]string{
	"README.md": `# .wolfcastle

This directory is Wolfcastle's workspace. Everything it needs to plan, execute,
and track work lives here.

## What's inside

- **system/** -- Configuration and runtime state. Three tiers of config (base,
  custom, local), project trees, and logs.
- **docs/** -- ADRs and specs generated during project work. Decisions get
  recorded. Specs get written. Nothing disappears.
- **archive/** -- Completed projects land here. Finished work moves out of the
  active tree and into cold storage.
- **artifacts/** -- Build outputs, generated files, anything produced as a side
  effect of task execution.
`,

	"system/README.md": `# system

Configuration and state, organized into three tiers.

## The three tiers

**base/** -- Defaults. Wolfcastle writes these. You don't edit them. Every
` + "`wolfcastle init --force`" + ` regenerates this tier from scratch. Prompt
templates, default config, audit definitions. All machine-generated, all
disposable.

**custom/** -- Team overrides. Checked into version control. This is where your
team sets model preferences, pipeline stages, and any config that should travel
with the repo. Custom overrides base.

**local/** -- Your machine. Gitignored. Identity, local model endpoints,
anything specific to this workstation. Local overrides everything.

A field set in local beats custom. Custom beats base. Set a field to ` + "`null`" + ` in
a higher tier to delete it entirely.

## Other directories

- **projects/** -- Active project trees. Each namespace (user+machine) gets its
  own directory with a state.json index.
- **logs/** -- Daemon execution logs. Gitignored.
`,

	"system/base/prompts/README.md": `# prompts

Prompt templates that drive Wolfcastle's pipeline stages. Each file here is a
Markdown template injected into model calls during task execution.

Organized into subdirectories by purpose:

- **stages/** -- Pipeline stage prompts (intake, execute, planning variants).
- **classes/** -- Task class prompts. Empty in the base tier; add your own
  under system/custom/prompts/classes/ to guide execution by task type.
- **audits/** -- Audit command prompts.

Shared support templates (script-reference, context-headers, etc.) remain at
this level.

These are the base tier copies. Wolfcastle regenerates them on every
` + "`wolfcastle init --force`" + `. Do not edit these files directly.

## How to override

Create the same filename under **system/custom/prompts/** (team-wide) or
**system/local/prompts/** (this machine only). The higher tier wins. Wolfcastle
loads prompts using the same three-tier resolution as config: local beats
custom beats base.

If you want to tweak the execution prompt for your team, copy
` + "`system/base/prompts/stages/execute.md`" + ` to ` + "`system/custom/prompts/stages/execute.md`" + `
and edit the copy. The base version stays intact and gets refreshed on upgrades.
Your custom version stays yours.
`,

	"docs/README.md": `# docs

Generated documentation from project work. Wolfcastle writes here during
planning and execution.

## What goes here

- **decisions/** -- Architecture Decision Records. When Wolfcastle makes a
  design choice worth recording, the ADR lands here.
- **specs/** -- Technical specifications. Produced during project planning,
  referenced during execution.

These files are tracked in git. They survive project completion and serve as
the written record of why things were built the way they were.
`,

	"archive/README.md": `# archive

Completed projects. When a project finishes, its state tree moves here.

Active work lives in system/projects/. Once every task is done and the final
audit passes, the project gets archived. The work is preserved. The active
tree stays clean.

Archived projects are tracked in git. They are the permanent record.
`,
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

	// Write .gitignore with whitelist pattern
	gitignore := `# Ignore everything by default, then whitelist tracked directories.
# Git requires each directory level to be explicitly unignored.
*
!.gitignore
!README.md

# Custom config overrides (user-editable)
!system/
!system/README.md
!system/custom/
!system/custom/**/
!system/custom/**

# Project state (task trees, inbox, specs, ADRs)
!system/projects/
!system/projects/**/
!system/projects/**

# Runtime artifacts (lock files, PID files, logs)
*.lock

# Base prompts (README only; base tier is regenerated)
!system/base/
!system/base/prompts/
!system/base/prompts/README.md

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

	// Write README files into key directories.
	for path, content := range scaffoldREADMEs {
		if err := os.WriteFile(filepath.Join(s.root, path), []byte(content), 0644); err != nil {
			return fmt.Errorf("scaffold: writing %s: %w", path, err)
		}
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

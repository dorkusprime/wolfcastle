// Package project handles project scaffolding (wolfcastle init), base
// template management, and project creation within the node tree.
package project

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// Deprecated: Use ScaffoldService.Init instead. Scaffold creates the
// .wolfcastle/ directory structure for wolfcastle init.
func Scaffold(wolfcastleDir string) error {
	dirs := []string{
		"system/base/prompts",
		"system/base/prompts/stages",
		"system/base/prompts/classes",
		"system/base/prompts/audits",
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
		if err := os.MkdirAll(filepath.Join(wolfcastleDir, d), 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", d, err)
		}
	}

	// Write .gitignore
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
	if err := os.WriteFile(filepath.Join(wolfcastleDir, ".gitignore"), []byte(gitignore), 0644); err != nil {
		return fmt.Errorf("writing .gitignore: %w", err)
	}

	// Write README files into key directories.
	for path, content := range scaffoldREADMEs {
		if err := os.WriteFile(filepath.Join(wolfcastleDir, path), []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}

	// Write system/base/config.json with populated defaults (excluding identity)
	defaults := config.Defaults()
	defaults.Identity = nil // Identity belongs in system/local/config.json only
	cfgData, err := json.MarshalIndent(defaults, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling default config: %w", err)
	}
	cfgData = append(cfgData, '\n')
	if err := os.WriteFile(filepath.Join(wolfcastleDir, "system", "base", "config.json"), cfgData, 0644); err != nil {
		return fmt.Errorf("writing system/base/config.json: %w", err)
	}

	// Write system/custom/config.json as empty object for teams to edit
	if err := os.WriteFile(filepath.Join(wolfcastleDir, "system", "custom", "config.json"), []byte("{}\n"), 0644); err != nil {
		return fmt.Errorf("writing system/custom/config.json: %w", err)
	}

	// Write system/local/config.json with identity
	identity := detectIdentity()
	localCfg := map[string]any{
		"identity": identity,
	}
	localData, err := json.MarshalIndent(localCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling local config: %w", err)
	}
	localData = append(localData, '\n')
	if err := os.WriteFile(filepath.Join(wolfcastleDir, "system", "local", "config.json"), localData, 0644); err != nil {
		return fmt.Errorf("writing system/local/config.json: %w", err)
	}

	// Create engineer namespace directory with empty root index
	ns := identity["user"].(string) + "-" + identity["machine"].(string)
	nsDir := filepath.Join(wolfcastleDir, "system", "projects", ns)
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		return fmt.Errorf("creating namespace directory: %w", err)
	}

	idx := state.NewRootIndex()
	idxData, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling root index: %w", err)
	}
	idxData = append(idxData, '\n')
	if err := os.WriteFile(filepath.Join(nsDir, "state.json"), idxData, 0644); err != nil {
		return fmt.Errorf("writing root index: %w", err)
	}

	// Write base prompt files
	if err := WriteBasePrompts(wolfcastleDir); err != nil {
		return fmt.Errorf("writing base prompts: %w", err)
	}

	return nil
}

// detectIdentity reads the current username and hostname from the system.
func detectIdentity() map[string]any {
	user := "unknown"
	machine := "unknown"

	if u, err := exec.Command("whoami").Output(); err == nil {
		user = strings.TrimSpace(string(u))
	}
	if h, err := os.Hostname(); err == nil {
		// Use short hostname
		if idx := strings.IndexByte(h, '.'); idx > 0 {
			h = h[:idx]
		}
		machine = strings.ToLower(h)
	}

	return map[string]any{
		"user":    user,
		"machine": machine,
	}
}

// Deprecated: Use ScaffoldService.Init or ScaffoldService.Reinit instead.
// WriteBasePrompts extracts embedded prompt templates into the system/base/ directory.
func WriteBasePrompts(wolfcastleDir string) error {
	return fs.WalkDir(Templates, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		relPath := strings.TrimPrefix(path, "templates/")
		destPath := filepath.Join(wolfcastleDir, "system", "base", relPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}
		data, err := Templates.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destPath, data, 0644)
	})
}

// Deprecated: Use ScaffoldService.Reinit instead. ReScaffold regenerates
// system/base/ templates and config, refreshes identity in
// system/local/config.json, migrates old-style config files (config.json,
// config.local.json) to the three-tier layout, and migrates the old flat
// directory structure into the system/ subdirectory.
func ReScaffold(wolfcastleDir string) error {
	// Migrate old flat layout (base/, custom/, local/, projects/, logs/)
	// into the system/ subdirectory.
	if err := migrateToSystemLayout(wolfcastleDir); err != nil {
		return err
	}

	// Migrate old-style config files to three-tier layout
	if err := migrateOldConfig(wolfcastleDir); err != nil {
		return err
	}

	// Remove existing system/base/ and regenerate
	baseDir := filepath.Join(wolfcastleDir, "system", "base")
	if err := os.RemoveAll(baseDir); err != nil {
		return fmt.Errorf("removing system/base/: %w", err)
	}
	baseDirs := []string{
		"system/base/prompts",
		"system/base/prompts/stages",
		"system/base/prompts/classes",
		"system/base/prompts/audits",
		"system/base/rules",
		"system/base/audits",
	}
	for _, d := range baseDirs {
		if err := os.MkdirAll(filepath.Join(wolfcastleDir, d), 0755); err != nil {
			return fmt.Errorf("creating %s: %w", d, err)
		}
	}
	if err := WriteBasePrompts(wolfcastleDir); err != nil {
		return fmt.Errorf("regenerating system/base/: %w", err)
	}

	// Write system/base/config.json from Defaults (always overwritten)
	defaults := config.Defaults()
	defaults.Identity = nil
	cfgData, err := json.MarshalIndent(defaults, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling default config: %w", err)
	}
	cfgData = append(cfgData, '\n')
	if err := os.WriteFile(filepath.Join(wolfcastleDir, "system", "base", "config.json"), cfgData, 0644); err != nil {
		return fmt.Errorf("writing system/base/config.json: %w", err)
	}

	// Ensure system/custom/config.json exists
	customCfgPath := filepath.Join(wolfcastleDir, "system", "custom", "config.json")
	if _, err := os.Stat(customCfgPath); os.IsNotExist(err) {
		_ = os.MkdirAll(filepath.Join(wolfcastleDir, "system", "custom"), 0755)
		_ = os.WriteFile(customCfgPath, []byte("{}\n"), 0644)
	}

	// Refresh identity in system/local/config.json, preserving other keys
	localPath := filepath.Join(wolfcastleDir, "system", "local", "config.json")
	localCfg := map[string]any{}

	if data, err := os.ReadFile(localPath); err == nil {
		if err := json.Unmarshal(data, &localCfg); err != nil {
			return fmt.Errorf("system/local/config.json is not valid JSON: %w", err)
		}
	}

	localCfg["identity"] = detectIdentity()
	localData, err := json.MarshalIndent(localCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling local config: %w", err)
	}
	localData = append(localData, '\n')
	_ = os.MkdirAll(filepath.Join(wolfcastleDir, "system", "local"), 0755)
	if err := os.WriteFile(localPath, localData, 0644); err != nil {
		return fmt.Errorf("writing system/local/config.json: %w", err)
	}

	return nil
}

// migrateToSystemLayout detects the old flat directory layout (base/, custom/,
// local/, projects/, logs/ at the .wolfcastle/ root) and moves them under
// system/. Also moves PID file, stop file, and daemon.log.
func migrateToSystemLayout(wolfcastleDir string) error {
	// If system/ already exists, assume migration is done.
	systemDir := filepath.Join(wolfcastleDir, "system")
	if _, err := os.Stat(systemDir); err == nil {
		return nil
	}

	// Check if old layout exists by looking for base/ at the root.
	oldBase := filepath.Join(wolfcastleDir, "base")
	if _, err := os.Stat(oldBase); os.IsNotExist(err) {
		// No old layout to migrate. Just ensure system/ exists.
		return os.MkdirAll(systemDir, 0755)
	}

	// Create system/ and move directories into it.
	if err := os.MkdirAll(systemDir, 0755); err != nil {
		return fmt.Errorf("creating system/: %w", err)
	}

	dirsToMove := []string{"base", "custom", "local", "projects", "logs"}
	for _, d := range dirsToMove {
		src := filepath.Join(wolfcastleDir, d)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		dst := filepath.Join(systemDir, d)
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("migrating %s to system/%s: %w", d, d, err)
		}
	}

	// Move loose daemon files into system/
	filesToMove := []string{"wolfcastle.pid", "stop", "daemon.log", "daemon.meta.json"}
	for _, f := range filesToMove {
		src := filepath.Join(wolfcastleDir, f)
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

// migrateOldConfig moves old-style root config files to the three-tier layout.
// If root config.json exists, it is moved to system/custom/config.json.
// If config.local.json exists, its contents are merged into system/local/config.json.
func migrateOldConfig(wolfcastleDir string) error {
	// Migrate root config.json -> system/custom/config.json
	oldCfgPath := filepath.Join(wolfcastleDir, "config.json")
	if _, err := os.Stat(oldCfgPath); err == nil {
		customDir := filepath.Join(wolfcastleDir, "system", "custom")
		_ = os.MkdirAll(customDir, 0755)
		customCfgPath := filepath.Join(customDir, "config.json")
		// Only migrate if system/custom/config.json doesn't already exist
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
	oldLocalPath := filepath.Join(wolfcastleDir, "config.local.json")
	if _, err := os.Stat(oldLocalPath); err == nil {
		localDir := filepath.Join(wolfcastleDir, "system", "local")
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

		// Merge into existing system/local/config.json if it exists
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

// WriteAuditTaskMD writes audit.md into nodeDir from the embedded audit-task template.
func WriteAuditTaskMD(nodeDir string) {
	data, err := Templates.ReadFile("templates/audits/audit-task.md")
	if err != nil {
		return // best-effort
	}
	_ = os.WriteFile(filepath.Join(nodeDir, "audit.md"), data, 0644)
}

// CreateProject creates a new project node in the tree.
func CreateProject(
	idx *state.RootIndex,
	parentAddr string,
	slug string,
	name string,
	nodeType state.NodeType,
) (*state.NodeState, string, error) {
	// Build the new address
	var addr string
	if parentAddr == "" {
		addr = slug
	} else {
		addr = parentAddr + "/" + slug
	}

	// Check for duplicates
	if _, exists := idx.Nodes[addr]; exists {
		return nil, "", fmt.Errorf("node %q already exists", addr)
	}

	// Create node state
	ns := state.NewNodeState(slug, name, nodeType)

	// Add audit task for all node types. Leaf audits verify the node's
	// tasks. Orchestrator audits verify the aggregate of all children's
	// work: cross-cutting quality, duplication between siblings,
	// consistent patterns, and integration.
	ns.Tasks = []state.Task{
		{
			ID:          "audit",
			Title:       "Audit",
			Description: "Verify all work in " + name + " is complete and correct",
			State:       state.StatusNotStarted,
			IsAudit:     true,
		},
	}

	// Update root index
	entry := state.IndexEntry{
		Name:     name,
		Type:     nodeType,
		State:    state.StatusNotStarted,
		Address:  addr,
		Parent:   parentAddr,
		Children: []string{},
	}
	idx.Nodes[addr] = entry

	// Update parent's children list or root list
	if parentAddr != "" {
		if parent, ok := idx.Nodes[parentAddr]; ok {
			parent.Children = append(parent.Children, addr)
			idx.Nodes[parentAddr] = parent
		}
	} else {
		// Root-level node -- add to root list
		idx.Root = append(idx.Root, addr)
	}

	return ns, addr, nil
}

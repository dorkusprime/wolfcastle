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

// Scaffold creates the .wolfcastle/ directory structure for wolfcastle init.
func Scaffold(wolfcastleDir string) error {
	dirs := []string{
		"base/prompts",
		"base/rules",
		"base/audits",
		"custom",
		"local",
		"archive",
		"docs/decisions",
		"docs/specs",
		"logs",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(wolfcastleDir, d), 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", d, err)
		}
	}

	// Write .gitignore
	gitignore := `*
!.gitignore
!config.json
!custom/
!custom/**
!projects/
!projects/**
!archive/
!archive/**
!docs/
!docs/**
`
	if err := os.WriteFile(filepath.Join(wolfcastleDir, ".gitignore"), []byte(gitignore), 0644); err != nil {
		return fmt.Errorf("writing .gitignore: %w", err)
	}

	// Write default config.json with populated defaults (excluding identity)
	defaults := config.Defaults()
	defaults.Identity = nil // Identity belongs in config.local.json only
	cfgData, err := json.MarshalIndent(defaults, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling default config: %w", err)
	}
	cfgData = append(cfgData, '\n')
	if err := os.WriteFile(filepath.Join(wolfcastleDir, "config.json"), cfgData, 0644); err != nil {
		return fmt.Errorf("writing config.json: %w", err)
	}

	// Write config.local.json with identity
	identity := detectIdentity()
	localCfg := map[string]any{
		"identity": identity,
	}
	localData, err := json.MarshalIndent(localCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling local config: %w", err)
	}
	localData = append(localData, '\n')
	if err := os.WriteFile(filepath.Join(wolfcastleDir, "config.local.json"), localData, 0644); err != nil {
		return fmt.Errorf("writing config.local.json: %w", err)
	}

	// Create engineer namespace directory with empty root index
	ns := identity["user"].(string) + "-" + identity["machine"].(string)
	nsDir := filepath.Join(wolfcastleDir, "projects", ns)
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

// WriteBasePrompts extracts embedded prompt templates into the base/ directory.
func WriteBasePrompts(wolfcastleDir string) error {
	return fs.WalkDir(Templates, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		relPath := strings.TrimPrefix(path, "templates/")
		destPath := filepath.Join(wolfcastleDir, "base", relPath)
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

// ReScaffold regenerates base/ templates and refreshes identity in
// config.local.json without overwriting config.json or other user data.
func ReScaffold(wolfcastleDir string) error {
	// Remove existing base/ and regenerate
	baseDir := filepath.Join(wolfcastleDir, "base")
	if err := os.RemoveAll(baseDir); err != nil {
		return fmt.Errorf("removing base/: %w", err)
	}
	baseDirs := []string{
		"base/prompts",
		"base/rules",
		"base/audits",
	}
	for _, d := range baseDirs {
		if err := os.MkdirAll(filepath.Join(wolfcastleDir, d), 0755); err != nil {
			return fmt.Errorf("creating %s: %w", d, err)
		}
	}
	if err := WriteBasePrompts(wolfcastleDir); err != nil {
		return fmt.Errorf("regenerating base/: %w", err)
	}

	// Refresh identity in config.local.json, preserving other keys
	localPath := filepath.Join(wolfcastleDir, "config.local.json")
	localCfg := map[string]any{}

	if data, err := os.ReadFile(localPath); err == nil {
		if err := json.Unmarshal(data, &localCfg); err != nil {
			return fmt.Errorf("config.local.json is not valid JSON: %w", err)
		}
	}

	localCfg["identity"] = detectIdentity()
	localData, err := json.MarshalIndent(localCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling local config: %w", err)
	}
	localData = append(localData, '\n')
	if err := os.WriteFile(localPath, localData, 0644); err != nil {
		return fmt.Errorf("writing config.local.json: %w", err)
	}

	return nil
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

	// Add audit task for leaf nodes
	if nodeType == state.NodeLeaf {
		ns.Tasks = []state.Task{
			{
				ID:          "audit",
				Description: "Verify all work in " + name + " is complete and correct",
				State:       state.StatusNotStarted,
				IsAudit:     true,
			},
		}
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
		// Root-level node — add to root list
		idx.Root = append(idx.Root, addr)
	}

	return ns, addr, nil
}

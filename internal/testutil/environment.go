package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tierfs"
)

// NodeSpec describes the shape of a project node for test tree construction.
type NodeSpec struct {
	Name     string
	Type     state.NodeType
	Tasks    []string
	Children []NodeSpec
}

// Leaf creates a NodeSpec for a leaf node with the given task IDs.
func Leaf(name string, taskIDs ...string) NodeSpec {
	return NodeSpec{
		Name:  name,
		Type:  state.NodeLeaf,
		Tasks: taskIDs,
	}
}

// Orchestrator creates a NodeSpec for an orchestrator node with children.
func Orchestrator(name string, children ...NodeSpec) NodeSpec {
	return NodeSpec{
		Name:     name,
		Type:     state.NodeOrchestrator,
		Children: children,
	}
}

// Environment provides a fully constructed .wolfcastle/ directory for tests.
// It starts with tierfs, Root, and State; repository fields will be populated
// as their packages are built during the domain-repository migration.
type Environment struct {
	// Root is the path to the .wolfcastle/ directory within the temp dir.
	Root string

	// Tiers provides three-tier file resolution rooted at .wolfcastle/system.
	Tiers tierfs.Resolver

	// State provides coordinated access to project state files.
	State *state.StateStore

	// TODO: populate when ConfigRepository is built
	// Config *config.ConfigRepository

	// TODO: populate when PromptRepository is built
	// Prompts *pipeline.PromptRepository

	// TODO: populate when ClassRepository is built
	// Classes *pipeline.ClassRepository

	// TODO: populate when DaemonRepository is built
	// Daemon *daemon.DaemonRepository

	// TODO: populate when git.Provider is built
	// Git git.Provider

	// TODO: populate when App is refactored to accept repositories
	// App *cmdutil.App

	// namespace holds the identity namespace used for the projects directory.
	namespace string
}

// NewEnvironment creates a minimal .wolfcastle/ directory structure in a temp
// dir, mirroring what SetupWolfcastle does but backed by a tierfs.FS and
// wrapped in a structured Environment. The directory is automatically cleaned
// up when the test finishes.
func NewEnvironment(t *testing.T) *Environment {
	t.Helper()

	root := t.TempDir()
	wolfcastleDir := filepath.Join(root, ".wolfcastle")
	systemDir := filepath.Join(wolfcastleDir, "system")
	namespace := "test-machine"

	// Construct tierfs.FS rooted at .wolfcastle/system.
	tiers := tierfs.New(systemDir)

	// Create the three-tier directory structure with subdirectories.
	tierSubdirs := []string{"prompts", "rules", "audits"}
	for _, tier := range tiers.TierDirs() {
		for _, sub := range tierSubdirs {
			if err := os.MkdirAll(filepath.Join(tier, sub), 0o755); err != nil {
				t.Fatalf("creating tier subdir %s/%s: %v", tier, sub, err)
			}
		}
	}

	// Create ancillary directories.
	ancillary := []string{
		filepath.Join(systemDir, "logs"),
		filepath.Join(systemDir, "projects", namespace),
		filepath.Join(wolfcastleDir, "docs", "decisions"),
		filepath.Join(wolfcastleDir, "docs", "specs"),
		filepath.Join(wolfcastleDir, "artifacts"),
		filepath.Join(wolfcastleDir, "archive"),
	}
	for _, dir := range ancillary {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating directory %s: %v", dir, err)
		}
	}

	// Write default config to the base tier.
	cfg := config.Defaults()
	cfg.Identity = nil
	cfgData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshaling default config: %v", err)
	}
	if err := tiers.WriteBase("config.json", cfgData); err != nil {
		t.Fatalf("writing base config: %v", err)
	}

	// Write empty custom config.
	if err := writeJSON(filepath.Join(systemDir, "custom", "config.json"), map[string]any{}); err != nil {
		t.Fatalf("writing custom config: %v", err)
	}

	// Write local config with test identity.
	localCfg := map[string]any{
		"identity": map[string]any{
			"user":    "test",
			"machine": "machine",
		},
	}
	if err := writeJSON(filepath.Join(systemDir, "local", "config.json"), localCfg); err != nil {
		t.Fatalf("writing local config: %v", err)
	}

	// Write empty root index.
	idx := state.NewRootIndex()
	projectsDir := filepath.Join(systemDir, "projects", namespace)
	if err := writeJSON(filepath.Join(projectsDir, "state.json"), idx); err != nil {
		t.Fatalf("writing root index: %v", err)
	}

	store := state.NewStateStore(projectsDir, 5*time.Second)

	return &Environment{
		Root:      wolfcastleDir,
		Tiers:     tiers,
		State:     store,
		namespace: namespace,
	}
}

// Namespace returns the identity namespace (e.g. "test-machine").
func (e *Environment) Namespace() string {
	return e.namespace
}

// ProjectsDir returns the absolute path to the projects directory within
// the namespace.
func (e *Environment) ProjectsDir() string {
	return filepath.Join(e.Root, "system", "projects", e.namespace)
}

// writeJSON marshals v to indented JSON and writes it to path, creating
// parent directories as needed.
func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

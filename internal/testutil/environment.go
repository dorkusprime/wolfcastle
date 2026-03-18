package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/git"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
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

	// Config provides three-tier configuration resolution.
	Config *config.ConfigRepository

	// Prompts provides three-tier prompt file resolution.
	Prompts *pipeline.PromptRepository

	// Classes provides task class prompt resolution atop PromptRepository.
	Classes *pipeline.ClassRepository

	// Daemon provides access to daemon filesystem operations (PID, stop file, logs).
	Daemon *daemon.DaemonRepository

	// Git provides access to git operations for tests that set up a repository.
	// Not populated by NewEnvironment (temp dirs aren't git repos by default);
	// use WithGit to supply a provider in tests that need one.
	Git git.Provider

	// TODO: populate when App is refactored to accept repositories
	// App *cmdutil.App

	// t is the test handle, used by With* methods to fatal on setup errors.
	t *testing.T

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

	prompts := pipeline.NewPromptRepositoryWithTiers(tiers)

	return &Environment{
		Root:      wolfcastleDir,
		Tiers:     tiers,
		State:     store,
		Config:    config.NewConfigRepositoryWithTiers(tiers, wolfcastleDir),
		Prompts:   prompts,
		Classes:   pipeline.NewClassRepository(prompts),
		Daemon:    daemon.NewDaemonRepository(wolfcastleDir),
		t:         t,
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

// WithConfig deep-merges overrides into the custom tier config file
// (system/custom/config.json) and returns the Environment for chaining.
func (e *Environment) WithConfig(overrides map[string]any) *Environment {
	e.t.Helper()

	customPath := filepath.Join(e.Root, "system", "custom", "config.json")

	// Read existing custom config.
	data, err := os.ReadFile(customPath)
	if err != nil {
		e.t.Fatalf("reading custom config: %v", err)
	}
	var existing map[string]any
	if err := json.Unmarshal(data, &existing); err != nil {
		e.t.Fatalf("unmarshaling custom config: %v", err)
	}

	merged := config.DeepMerge(existing, overrides)
	if err := writeJSON(customPath, merged); err != nil {
		e.t.Fatalf("writing merged custom config: %v", err)
	}
	return e
}

// WithGit sets the Git provider on the Environment and returns it for
// chaining. Tests that need git operations should construct a git.Service
// (or stub) and pass it here.
func (e *Environment) WithGit(provider git.Provider) *Environment {
	e.Git = provider
	return e
}

// WithProject creates a project in the state tree using the NodeSpec
// structure and returns the Environment for chaining. The name argument
// becomes both the root node ID and display name. Orchestrator specs
// recurse into children; leaf specs generate tasks with auto-descriptions.
// Every node receives an audit task, matching SetupTree behavior.
func (e *Environment) WithProject(name string, root NodeSpec) *Environment {
	e.t.Helper()

	projectsDir := e.ProjectsDir()
	idx, err := e.State.ReadIndex()
	if err != nil {
		e.t.Fatalf("reading root index: %v", err)
	}

	// Use the NodeSpec name as the root address.
	rootAddr := root.Name
	idx.RootID = rootAddr
	idx.RootName = name
	idx.RootState = state.StatusNotStarted
	idx.Root = append(idx.Root, rootAddr)

	// Recursively build the tree.
	e.buildNode(projectsDir, idx, root, "", rootAddr, name)

	// Persist root index.
	if err := writeJSON(filepath.Join(projectsDir, "state.json"), idx); err != nil {
		e.t.Fatalf("writing root index: %v", err)
	}
	return e
}

// buildNode recursively creates node state files and index entries
// for a NodeSpec and all its descendants.
func (e *Environment) buildNode(projectsDir string, idx *state.RootIndex, spec NodeSpec, parentAddr, addr, displayName string) {
	e.t.Helper()

	nodeDir := filepath.Join(projectsDir, filepath.FromSlash(addr))
	if err := os.MkdirAll(nodeDir, 0o755); err != nil {
		e.t.Fatalf("creating node dir %s: %v", addr, err)
	}

	ns := state.NewNodeState(spec.Name, displayName, spec.Type)

	// Index entry for this node.
	entry := state.IndexEntry{
		Name:     displayName,
		Type:     spec.Type,
		State:    state.StatusNotStarted,
		Address:  addr,
		Parent:   parentAddr,
		Children: []string{},
	}

	if spec.Type == state.NodeOrchestrator {
		// Build children.
		for _, child := range spec.Children {
			childAddr := addr + "/" + child.Name
			childName := child.Name
			entry.Children = append(entry.Children, childAddr)
			ns.Children = append(ns.Children, state.ChildRef{
				ID:      child.Name,
				Address: childAddr,
				State:   state.StatusNotStarted,
			})
			e.buildNode(projectsDir, idx, child, addr, childAddr, childName)
		}
	} else {
		// Leaf: create tasks from the spec's Tasks list.
		for _, taskID := range spec.Tasks {
			ns.Tasks = append(ns.Tasks, state.Task{
				ID:          taskID,
				Description: "Task: " + taskID,
				State:       state.StatusNotStarted,
			})
		}
		// Every leaf gets an audit task.
		ns.Tasks = append(ns.Tasks, state.Task{
			ID:          "audit",
			Description: "Verify work",
			State:       state.StatusNotStarted,
			IsAudit:     true,
		})
	}

	idx.Nodes[addr] = entry

	if err := writeJSON(filepath.Join(nodeDir, "state.json"), ns); err != nil {
		e.t.Fatalf("writing node state %s: %v", addr, err)
	}
}

// WithPrompt writes a prompt file to system/base/prompts/<relPath> and
// returns the Environment for chaining.
func (e *Environment) WithPrompt(relPath string, content string) *Environment {
	e.t.Helper()
	if err := e.Tiers.WriteBase(filepath.Join("prompts", relPath), []byte(content)); err != nil {
		e.t.Fatalf("writing prompt %s: %v", relPath, err)
	}
	return e
}

// WithRule writes a rule fragment to system/base/rules/<name> and returns
// the Environment for chaining.
func (e *Environment) WithRule(name string, content string) *Environment {
	e.t.Helper()
	if err := e.Tiers.WriteBase(filepath.Join("rules", name), []byte(content)); err != nil {
		e.t.Fatalf("writing rule %s: %v", name, err)
	}
	return e
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

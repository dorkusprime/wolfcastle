package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ── NewEnvironment ──────────────────────────────────────────────────────

func TestNewEnvironment_DirectoryStructure(t *testing.T) {
	t.Parallel()
	env := NewEnvironment(t)

	// Three tier directories must exist.
	tierDirs := env.Tiers.TierDirs()
	if len(tierDirs) != 3 {
		t.Fatalf("expected 3 tier dirs, got %d", len(tierDirs))
	}
	for _, dir := range tierDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("tier dir missing: %s", dir)
		}
	}

	// Projects dir exists.
	if _, err := os.Stat(env.ProjectsDir()); os.IsNotExist(err) {
		t.Error("projects dir should exist")
	}

	// Ancillary dirs: docs, artifacts, archive.
	for _, rel := range []string{
		"docs",
		filepath.Join("docs", "decisions"),
		filepath.Join("docs", "specs"),
		"artifacts",
		"archive",
	} {
		path := filepath.Join(env.Root, rel)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("ancillary dir missing: %s", rel)
		}
	}
}

func TestNewEnvironment_ConfigLoadable(t *testing.T) {
	t.Parallel()
	env := NewEnvironment(t)

	cfg, err := config.Load(env.Root)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("expected config version 1, got %d", cfg.Version)
	}
}

func TestNewEnvironment_Identity(t *testing.T) {
	t.Parallel()
	env := NewEnvironment(t)

	cfg, err := config.Load(env.Root)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	if cfg.Identity == nil {
		t.Fatal("expected identity to be set")
	}
	if cfg.Identity.User != "test" {
		t.Errorf("expected identity user 'test', got %q", cfg.Identity.User)
	}
	if cfg.Identity.Machine != "machine" {
		t.Errorf("expected identity machine 'machine', got %q", cfg.Identity.Machine)
	}
	if env.Namespace() != "test-machine" {
		t.Errorf("expected namespace 'test-machine', got %q", env.Namespace())
	}
}

func TestNewEnvironment_StateStoreFunctional(t *testing.T) {
	t.Parallel()
	env := NewEnvironment(t)

	idx, err := env.State.ReadIndex()
	if err != nil {
		t.Fatalf("ReadIndex failed: %v", err)
	}
	if idx.Version != 1 {
		t.Errorf("expected root index version 1, got %d", idx.Version)
	}
	if len(idx.Nodes) != 0 {
		t.Errorf("expected empty nodes map, got %d entries", len(idx.Nodes))
	}
}

// ── NodeSpec builders ───────────────────────────────────────────────────

func TestLeaf_CreatesCorrectSpec(t *testing.T) {
	t.Parallel()

	spec := Leaf("analysis", "task-0001", "task-0002")
	if spec.Name != "analysis" {
		t.Errorf("expected name 'analysis', got %q", spec.Name)
	}
	if spec.Type != state.NodeLeaf {
		t.Errorf("expected leaf type, got %q", spec.Type)
	}
	if len(spec.Tasks) != 2 || spec.Tasks[0] != "task-0001" || spec.Tasks[1] != "task-0002" {
		t.Errorf("unexpected tasks: %v", spec.Tasks)
	}
	if len(spec.Children) != 0 {
		t.Errorf("leaf should have no children, got %d", len(spec.Children))
	}
}

func TestOrchestrator_CreatesCorrectSpec(t *testing.T) {
	t.Parallel()

	spec := Orchestrator("root",
		Leaf("child-a", "t1"),
		Leaf("child-b", "t2"),
	)
	if spec.Name != "root" {
		t.Errorf("expected name 'root', got %q", spec.Name)
	}
	if spec.Type != state.NodeOrchestrator {
		t.Errorf("expected orchestrator type, got %q", spec.Type)
	}
	if len(spec.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(spec.Children))
	}
	if len(spec.Tasks) != 0 {
		t.Errorf("orchestrator should have no tasks, got %d", len(spec.Tasks))
	}
}

// ── WithConfig ──────────────────────────────────────────────────────────

func TestWithConfig_OverridesMerged(t *testing.T) {
	t.Parallel()
	env := NewEnvironment(t).WithConfig(map[string]any{
		"daemon": map[string]any{
			"max_iterations": float64(5),
		},
	})

	cfg, err := config.Load(env.Root)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	if cfg.Daemon.MaxIterations != 5 {
		t.Errorf("expected max_iterations=5, got %d", cfg.Daemon.MaxIterations)
	}
}

func TestWithConfig_PreservesBaseDefaults(t *testing.T) {
	t.Parallel()
	env := NewEnvironment(t).WithConfig(map[string]any{
		"daemon": map[string]any{
			"max_iterations": float64(5),
		},
	})

	cfg, err := config.Load(env.Root)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	// Base default for poll_interval_seconds is 5; should survive the override.
	if cfg.Daemon.PollIntervalSeconds != 5 {
		t.Errorf("expected poll_interval_seconds=5, got %d", cfg.Daemon.PollIntervalSeconds)
	}
	// Git defaults should be untouched.
	if !cfg.Git.AutoCommit {
		t.Error("expected git.auto_commit to remain true")
	}
}

func TestWithConfig_MultipleCalls_Accumulate(t *testing.T) {
	t.Parallel()
	env := NewEnvironment(t).
		WithConfig(map[string]any{
			"daemon": map[string]any{
				"max_iterations": float64(5),
			},
		}).
		WithConfig(map[string]any{
			"daemon": map[string]any{
				"log_level": "debug",
			},
		})

	cfg, err := config.Load(env.Root)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	if cfg.Daemon.MaxIterations != 5 {
		t.Errorf("first override lost: expected max_iterations=5, got %d", cfg.Daemon.MaxIterations)
	}
	if cfg.Daemon.LogLevel != "debug" {
		t.Errorf("second override lost: expected log_level=debug, got %q", cfg.Daemon.LogLevel)
	}
}

// ── WithProject ─────────────────────────────────────────────────────────

func TestWithProject_SimpleLeaf(t *testing.T) {
	t.Parallel()
	env := NewEnvironment(t).WithProject("My Leaf",
		Leaf("my-leaf", "task-0001", "task-0002"),
	)

	ns, err := env.State.ReadNode("my-leaf")
	if err != nil {
		t.Fatalf("ReadNode failed: %v", err)
	}
	if ns.Type != state.NodeLeaf {
		t.Errorf("expected leaf, got %q", ns.Type)
	}
	// Two explicit tasks plus the audit task.
	if len(ns.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(ns.Tasks))
	}
	if ns.Tasks[0].ID != "task-0001" || ns.Tasks[0].Description != "Task: task-0001" {
		t.Errorf("unexpected first task: %+v", ns.Tasks[0])
	}
	if ns.Tasks[1].ID != "task-0002" {
		t.Errorf("unexpected second task: %+v", ns.Tasks[1])
	}
}

func TestWithProject_AuditTask(t *testing.T) {
	t.Parallel()
	env := NewEnvironment(t).WithProject("P",
		Leaf("p", "t1"),
	)

	ns, err := env.State.ReadNode("p")
	if err != nil {
		t.Fatalf("ReadNode failed: %v", err)
	}
	audit := ns.Tasks[len(ns.Tasks)-1]
	if !audit.IsAudit {
		t.Error("last task should be an audit task")
	}
	if audit.ID != "audit" {
		t.Errorf("expected audit task ID 'audit', got %q", audit.ID)
	}
	if audit.Description != "Verify work" {
		t.Errorf("expected audit description 'Verify work', got %q", audit.Description)
	}
}

func TestWithProject_NestedOrchestrator(t *testing.T) {
	t.Parallel()
	env := NewEnvironment(t).WithProject("Big Project",
		Orchestrator("root",
			Leaf("child-a", "task-0001"),
			Leaf("child-b", "task-0002", "task-0003"),
		),
	)

	// Root is an orchestrator.
	rootNode, err := env.State.ReadNode("root")
	if err != nil {
		t.Fatalf("ReadNode root: %v", err)
	}
	if rootNode.Type != state.NodeOrchestrator {
		t.Errorf("expected orchestrator, got %q", rootNode.Type)
	}
	if len(rootNode.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(rootNode.Children))
	}

	// Children are leaves with correct addresses.
	childA, err := env.State.ReadNode("root/child-a")
	if err != nil {
		t.Fatalf("ReadNode child-a: %v", err)
	}
	if childA.Type != state.NodeLeaf {
		t.Errorf("child-a: expected leaf, got %q", childA.Type)
	}
	// 1 explicit task + audit.
	if len(childA.Tasks) != 2 {
		t.Errorf("child-a: expected 2 tasks, got %d", len(childA.Tasks))
	}

	childB, err := env.State.ReadNode("root/child-b")
	if err != nil {
		t.Fatalf("ReadNode child-b: %v", err)
	}
	// 2 explicit tasks + audit.
	if len(childB.Tasks) != 3 {
		t.Errorf("child-b: expected 3 tasks, got %d", len(childB.Tasks))
	}
}

func TestWithProject_RootIndexUpdated(t *testing.T) {
	t.Parallel()
	env := NewEnvironment(t).WithProject("P",
		Orchestrator("root",
			Leaf("child-a", "t1"),
		),
	)

	idx, err := env.State.ReadIndex()
	if err != nil {
		t.Fatalf("ReadIndex failed: %v", err)
	}
	if idx.RootID != "root" {
		t.Errorf("expected root_id 'root', got %q", idx.RootID)
	}
	if idx.RootName != "P" {
		t.Errorf("expected root_name 'P', got %q", idx.RootName)
	}
	// Should have entries for root and child-a.
	if len(idx.Nodes) != 2 {
		t.Errorf("expected 2 index entries, got %d", len(idx.Nodes))
	}
	if _, ok := idx.Nodes["root"]; !ok {
		t.Error("index missing 'root' entry")
	}
	if _, ok := idx.Nodes["root/child-a"]; !ok {
		t.Error("index missing 'root/child-a' entry")
	}
}

func TestWithProject_NodeStateReadable(t *testing.T) {
	t.Parallel()
	env := NewEnvironment(t).WithProject("P",
		Leaf("my-node", "task-0001"),
	)

	ns, err := env.State.ReadNode("my-node")
	if err != nil {
		t.Fatalf("ReadNode failed: %v", err)
	}
	if ns.State != state.StatusNotStarted {
		t.Errorf("expected not_started, got %q", ns.State)
	}
	if ns.Tasks[0].State != state.StatusNotStarted {
		t.Errorf("expected task state not_started, got %q", ns.Tasks[0].State)
	}
}

// ── WithPrompt ──────────────────────────────────────────────────────────

func TestWithPrompt_FileWritten(t *testing.T) {
	t.Parallel()
	env := NewEnvironment(t).WithPrompt("execute.md", "you are the executor")

	// Verify the file exists in base tier.
	basePath := env.Tiers.BasePath("prompts")
	path := filepath.Join(basePath, "execute.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("prompt file not created")
	}
}

func TestWithPrompt_ResolvableViaTiers(t *testing.T) {
	t.Parallel()
	env := NewEnvironment(t).WithPrompt("execute.md", "you are the executor")

	content, err := env.Tiers.Resolve(filepath.Join("prompts", "execute.md"))
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if string(content) != "you are the executor" {
		t.Errorf("unexpected content: %q", string(content))
	}
}

// ── WithRule ────────────────────────────────────────────────────────────

func TestWithRule_FileWritten(t *testing.T) {
	t.Parallel()
	env := NewEnvironment(t).WithRule("go-style.md", "use gofmt")

	basePath := env.Tiers.BasePath("rules")
	path := filepath.Join(basePath, "go-style.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("rule file not created")
	}
}

func TestWithRule_ResolvableAcrossTiers(t *testing.T) {
	t.Parallel()
	env := NewEnvironment(t).WithRule("go-style.md", "use gofmt")

	content, err := env.Tiers.Resolve(filepath.Join("rules", "go-style.md"))
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if string(content) != "use gofmt" {
		t.Errorf("unexpected content: %q", string(content))
	}
}

// ── Chaining ────────────────────────────────────────────────────────────

func TestChaining_FluentAPI(t *testing.T) {
	t.Parallel()

	env := NewEnvironment(t).
		WithConfig(map[string]any{
			"daemon": map[string]any{
				"max_iterations": float64(3),
			},
		}).
		WithProject("My Project",
			Orchestrator("root",
				Leaf("impl", "task-0001"),
			),
		).
		WithPrompt("execute.md", "test prompt").
		WithRule("style.md", "test rule")

	// Config override applied.
	cfg, err := config.Load(env.Root)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	if cfg.Daemon.MaxIterations != 3 {
		t.Errorf("expected max_iterations=3, got %d", cfg.Daemon.MaxIterations)
	}

	// Project exists.
	idx, err := env.State.ReadIndex()
	if err != nil {
		t.Fatalf("ReadIndex failed: %v", err)
	}
	if len(idx.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(idx.Nodes))
	}

	// Prompt resolvable.
	content, err := env.Tiers.Resolve(filepath.Join("prompts", "execute.md"))
	if err != nil {
		t.Fatalf("Resolve prompt: %v", err)
	}
	if string(content) != "test prompt" {
		t.Errorf("unexpected prompt content: %q", string(content))
	}

	// Rule resolvable.
	content, err = env.Tiers.Resolve(filepath.Join("rules", "style.md"))
	if err != nil {
		t.Fatalf("Resolve rule: %v", err)
	}
	if string(content) != "test rule" {
		t.Errorf("unexpected rule content: %q", string(content))
	}
}

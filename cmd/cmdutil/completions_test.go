package cmdutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// ---------------------------------------------------------------------------
// CompleteTaskAddresses — orchestrator nodes (non-leaf, no tasks)
// ---------------------------------------------------------------------------

func TestCompleteTaskAddresses_WithOrchestrator(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "test-dev"
	projDir := filepath.Join(wcDir, "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	idxJSON := `{"nodes":{
		"parent":{"name":"Parent","type":"orchestrator","state":"in_progress","address":"parent","children":["parent/child"]},
		"parent/child":{"name":"Child","type":"leaf","state":"in_progress","address":"parent/child","parent":"parent","children":[]}
	}}`
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), []byte(idxJSON), 0644)

	// Child node with tasks
	childDir := filepath.Join(projDir, "parent", "child")
	_ = os.MkdirAll(childDir, 0755)
	childJSON := `{
		"id": "child",
		"name": "Child",
		"type": "leaf",
		"state": "in_progress",
		"tasks": [{"id":"task-1","description":"work","state":"not_started"}],
		"audit": {"status": "pending", "breadcrumbs": [], "gaps": [], "escalations": []}
	}`
	_ = os.WriteFile(filepath.Join(childDir, "state.json"), []byte(childJSON), 0644)

	a := &App{
		WolfcastleDir: wcDir,
		Resolver:      &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns},
	}
	fn := CompleteTaskAddresses(a)
	addrs, _ := fn(nil, nil, "")

	foundParent := false
	foundChild := false
	foundTask := false
	for _, addr := range addrs {
		switch addr {
		case "parent":
			foundParent = true
		case "parent/child":
			foundChild = true
		case "parent/child/task-1":
			foundTask = true
		}
	}
	if !foundParent {
		t.Error("expected orchestrator address in completions")
	}
	if !foundChild {
		t.Error("expected child address in completions")
	}
	if !foundTask {
		t.Error("expected task address in completions")
	}
}

// ---------------------------------------------------------------------------
// CompleteTaskAddresses — node with invalid state file
// ---------------------------------------------------------------------------

func TestCompleteTaskAddresses_BrokenNodeState(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "test-dev"
	projDir := filepath.Join(wcDir, "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	idxJSON := `{"nodes":{"my-node":{"name":"My Node","type":"leaf","state":"in_progress","address":"my-node","children":[]}}}`
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), []byte(idxJSON), 0644)

	// Create node dir but with invalid JSON
	nodeDir := filepath.Join(projDir, "my-node")
	_ = os.MkdirAll(nodeDir, 0755)
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), []byte("invalid json"), 0644)

	a := &App{
		WolfcastleDir: wcDir,
		Resolver:      &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns},
	}
	fn := CompleteTaskAddresses(a)
	addrs, _ := fn(nil, nil, "")

	// Should still return the node address even if state is broken
	found := false
	for _, addr := range addrs {
		if addr == "my-node" {
			found = true
		}
	}
	if !found {
		t.Error("expected node address even with broken state")
	}
}

// ---------------------------------------------------------------------------
// CheckOverlap — empty bigrams from short text
// ---------------------------------------------------------------------------

func TestCheckOverlap_ShortText(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "me-dev"
	_ = os.MkdirAll(filepath.Join(wcDir, "projects", ns), 0755)

	// Create another namespace with a project
	otherDir := filepath.Join(wcDir, "projects", "other-dev")
	_ = os.MkdirAll(otherDir, 0755)
	_ = os.WriteFile(filepath.Join(otherDir, "proj.md"), []byte("some content"), 0644)

	a := &App{
		WolfcastleDir: wcDir,
		Cfg: &config.Config{
			OverlapAdvisory: config.OverlapConfig{Enabled: true, Threshold: 0.1},
		},
		Resolver: &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns},
	}
	// Use all stop words, should produce empty bigrams and bail early
	a.CheckOverlap("the", "the and for")
}

// ---------------------------------------------------------------------------
// loadRootIndexForCompletion — fallback to LoadConfig
// ---------------------------------------------------------------------------

func TestLoadRootIndexForCompletion_FallbackConfigFails(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

	// No resolver, no .wolfcastle dir -> LoadConfig fails
	a := &App{}
	_, err := loadRootIndexForCompletion(a)
	if err == nil {
		t.Error("expected error when config fallback fails")
	}
}

func TestLoadRootIndexForCompletion_FallbackResolverNil(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	_ = os.MkdirAll(wcDir, 0755)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

	a := &App{}
	_, err := loadRootIndexForCompletion(a)
	if err == nil {
		t.Error("expected error when resolver stays nil after config load")
	}
}

func TestLoadRootIndexForCompletion_FallbackSuccess(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "tester-box"
	projDir := filepath.Join(wcDir, "projects", ns)
	_ = os.MkdirAll(projDir, 0755)
	_ = os.MkdirAll(filepath.Join(wcDir, "local"), 0755)

	// Write config with identity in local/config.json
	cfgJSON := `{"identity": {"user": "tester", "machine": "box"}}`
	_ = os.WriteFile(filepath.Join(wcDir, "local", "config.json"), []byte(cfgJSON), 0644)
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), []byte(`{"nodes":{}}`), 0644)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

	a := &App{}
	idx, err := loadRootIndexForCompletion(a)
	if err != nil {
		t.Fatalf("expected success on fallback: %v", err)
	}
	if idx == nil {
		t.Error("expected non-nil index")
	}
}

func TestLoadConfig_MalformedConfig(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	_ = os.MkdirAll(filepath.Join(wcDir, "base"), 0755)

	// Write malformed config
	_ = os.WriteFile(filepath.Join(wcDir, "base", "config.json"), []byte("not valid json"), 0644)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

	a := &App{}
	err := a.LoadConfig()
	if err == nil {
		t.Error("expected error for malformed config")
	}
}

func TestResolverForCompletion_FallbackSuccess(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "tester-box"
	projDir := filepath.Join(wcDir, "projects", ns)
	_ = os.MkdirAll(projDir, 0755)
	_ = os.MkdirAll(filepath.Join(wcDir, "local"), 0755)

	// Config with identity in local/config.json
	cfgJSON := `{"identity": {"user": "tester", "machine": "box"}}`
	_ = os.WriteFile(filepath.Join(wcDir, "local", "config.json"), []byte(cfgJSON), 0644)
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), []byte(`{"nodes":{}}`), 0644)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

	a := &App{}
	res, err := resolverForCompletion(a)
	if err != nil {
		t.Fatalf("expected success on fallback: %v", err)
	}
	if res == nil {
		t.Error("expected non-nil resolver")
	}
}

// ---------------------------------------------------------------------------
// resolverForCompletion — fallback loads config but resolver still nil
// ---------------------------------------------------------------------------

func TestResolverForCompletion_FallbackConfigLoadsButNoResolver(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	_ = os.MkdirAll(wcDir, 0755)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

	a := &App{}
	_, err := resolverForCompletion(a)
	if err == nil {
		t.Error("expected error when config loaded but resolver still nil")
	}
}

// ---------------------------------------------------------------------------
// PropagateState — invalid node address in loadNode callback
// ---------------------------------------------------------------------------

func TestPropagateState_InvalidNodeInIndex(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "test-dev"
	projDir := filepath.Join(wcDir, "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	// Valid root index, leaf node only
	idxJSON := `{
		"root_id": "my-node",
		"root_state": "not_started",
		"nodes": {
			"my-node": {
				"name": "My Node",
				"type": "leaf",
				"state": "in_progress",
				"address": "my-node",
				"children": []
			}
		}
	}`
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), []byte(idxJSON), 0644)

	nodeDir := filepath.Join(projDir, "my-node")
	_ = os.MkdirAll(nodeDir, 0755)
	nodeJSON := `{
		"id": "my-node",
		"name": "My Node",
		"type": "leaf",
		"state": "complete",
		"tasks": [],
		"audit": {"status": "pending", "breadcrumbs": [], "gaps": [], "escalations": []}
	}`
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), []byte(nodeJSON), 0644)

	resolver := &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns}
	a := &App{
		WolfcastleDir: wcDir,
		Resolver:      resolver,
	}

	// Should succeed — simple propagation for root-level leaf
	err := a.PropagateState("my-node", "complete")
	if err != nil {
		t.Fatalf("PropagateState failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CheckOverlap — nonexistent projects dir
// ---------------------------------------------------------------------------

func TestCheckOverlap_NonexistentProjectsDir(t *testing.T) {
	a := &App{
		WolfcastleDir: "/nonexistent/path",
		Cfg: &config.Config{
			OverlapAdvisory: config.OverlapConfig{Enabled: true, Threshold: 0.1},
		},
		Resolver: &tree.Resolver{WolfcastleDir: "/nonexistent/path", Namespace: "me-dev"},
	}
	// Should not panic, just silently return
	a.CheckOverlap("database migration", "migrate the schema")
}

// ---------------------------------------------------------------------------
// CheckOverlap — found matches (coverage of output section)
// ---------------------------------------------------------------------------

func TestCheckOverlap_NoMatchesBelowThreshold(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "me-dev"
	_ = os.MkdirAll(filepath.Join(wcDir, "projects", ns), 0755)

	// Another engineer with completely different topic
	otherDir := filepath.Join(wcDir, "projects", "alice-dev")
	_ = os.MkdirAll(otherDir, 0755)
	_ = os.WriteFile(filepath.Join(otherDir, "quantum.md"),
		[]byte("quantum entanglement photon superposition"), 0644)

	a := &App{
		WolfcastleDir: wcDir,
		Cfg: &config.Config{
			OverlapAdvisory: config.OverlapConfig{Enabled: true, Threshold: 0.9},
		},
		Resolver: &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns},
	}
	// High threshold means no match
	a.CheckOverlap("database migration", "migrate postgresql schema")
}

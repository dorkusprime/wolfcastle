package tree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestAddress_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		addr Address
		want string
	}{
		{"empty address", Address{Raw: ""}, ""},
		{"simple address", Address{Raw: "my-node", Parts: []string{"my-node"}}, "my-node"},
		{"multi-segment", Address{Raw: "root/child/leaf", Parts: []string{"root", "child", "leaf"}}, "root/child/leaf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.addr.String(); got != tt.want {
				t.Errorf("Address.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMustParse_ValidInput(t *testing.T) {
	t.Parallel()
	addr := MustParse("root/child")
	if addr.Raw != "root/child" {
		t.Errorf("expected raw 'root/child', got %q", addr.Raw)
	}
	if len(addr.Parts) != 2 {
		t.Errorf("expected 2 parts, got %d", len(addr.Parts))
	}
}

func TestMustParse_PanicsOnInvalidInput(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustParse to panic on invalid input")
		}
	}()
	MustParse("INVALID")
}

func TestResolveNamespace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cfg     *config.Config
		want    string
		wantErr bool
	}{
		{
			name: "valid identity",
			cfg: &config.Config{
				Identity: &config.IdentityConfig{User: "alice", Machine: "laptop"},
			},
			want: "alice-laptop",
		},
		{
			name:    "nil identity",
			cfg:     &config.Config{Identity: nil},
			wantErr: true,
		},
		{
			name: "empty user",
			cfg: &config.Config{
				Identity: &config.IdentityConfig{User: "", Machine: "laptop"},
			},
			wantErr: true,
		},
		{
			name: "empty machine",
			cfg: &config.Config{
				Identity: &config.IdentityConfig{User: "alice", Machine: ""},
			},
			wantErr: true,
		},
		{
			name: "both empty",
			cfg: &config.Config{
				Identity: &config.IdentityConfig{User: "", Machine: ""},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveNamespace(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ResolveNamespace() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewResolver(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cfg     *config.Config
		wantNS  string
		wantErr bool
	}{
		{
			name: "creates resolver with correct namespace",
			cfg: &config.Config{
				Identity: &config.IdentityConfig{User: "bob", Machine: "desktop"},
			},
			wantNS: "bob-desktop",
		},
		{
			name:    "fails with nil identity",
			cfg:     &config.Config{Identity: nil},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, err := NewResolver("/tmp/wolfcastle", tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.Namespace != tt.wantNS {
				t.Errorf("Namespace = %q, want %q", r.Namespace, tt.wantNS)
			}
			if r.WolfcastleDir != "/tmp/wolfcastle" {
				t.Errorf("WolfcastleDir = %q, want %q", r.WolfcastleDir, "/tmp/wolfcastle")
			}
		})
	}
}

func TestResolver_ProjectsDir(t *testing.T) {
	t.Parallel()
	r := &Resolver{WolfcastleDir: "/home/user/.wolfcastle", Namespace: "alice-laptop"}
	got := r.ProjectsDir()
	want := filepath.Join("/home/user/.wolfcastle", "projects", "alice-laptop")
	if got != want {
		t.Errorf("ProjectsDir() = %q, want %q", got, want)
	}
}

func TestResolver_RootIndexPath(t *testing.T) {
	t.Parallel()
	r := &Resolver{WolfcastleDir: "/home/user/.wolfcastle", Namespace: "alice-laptop"}
	got := r.RootIndexPath()
	want := filepath.Join("/home/user/.wolfcastle", "projects", "alice-laptop", "state.json")
	if got != want {
		t.Errorf("RootIndexPath() = %q, want %q", got, want)
	}
}

func TestResolver_NodeDir(t *testing.T) {
	t.Parallel()
	r := &Resolver{WolfcastleDir: "/w", Namespace: "ns"}
	tests := []struct {
		name string
		addr Address
		want string
	}{
		{"root address", Address{}, filepath.Join("/w", "projects", "ns")},
		{"single segment", MustParse("proj"), filepath.Join("/w", "projects", "ns", "proj")},
		{"nested address", MustParse("proj/sub/leaf"), filepath.Join("/w", "projects", "ns", "proj", "sub", "leaf")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := r.NodeDir(tt.addr); got != tt.want {
				t.Errorf("NodeDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolver_NodeStatePath(t *testing.T) {
	t.Parallel()
	r := &Resolver{WolfcastleDir: "/w", Namespace: "ns"}
	tests := []struct {
		name string
		addr Address
		want string
	}{
		{"root", Address{}, filepath.Join("/w", "projects", "ns", "state.json")},
		{"nested", MustParse("proj/child"), filepath.Join("/w", "projects", "ns", "proj", "child", "state.json")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := r.NodeStatePath(tt.addr); got != tt.want {
				t.Errorf("NodeStatePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolver_NodeDefPath(t *testing.T) {
	t.Parallel()
	r := &Resolver{WolfcastleDir: "/w", Namespace: "ns"}
	tests := []struct {
		name string
		addr Address
		want string
	}{
		{"root returns empty", Address{}, ""},
		{"single segment", MustParse("proj"), filepath.Join("/w", "projects", "ns", "proj", "proj.md")},
		{"nested", MustParse("proj/child"), filepath.Join("/w", "projects", "ns", "proj", "child", "child.md")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := r.NodeDefPath(tt.addr); got != tt.want {
				t.Errorf("NodeDefPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolver_TaskDocPath(t *testing.T) {
	t.Parallel()
	r := &Resolver{WolfcastleDir: "/w", Namespace: "ns"}
	tests := []struct {
		name   string
		addr   Address
		taskID string
		want   string
	}{
		{"task doc", MustParse("proj"), "task-0001", filepath.Join("/w", "projects", "ns", "proj", "task-0001.md")},
		{"audit doc", MustParse("proj/child"), "audit", filepath.Join("/w", "projects", "ns", "proj", "child", "audit.md")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := r.TaskDocPath(tt.addr, tt.taskID); got != tt.want {
				t.Errorf("TaskDocPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolver_LoadRootIndex(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	nsDir := filepath.Join(tmpDir, "projects", "alice-laptop")
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		t.Fatal(err)
	}

	idx := state.NewRootIndex()
	idx.Root = []string{"my-proj"}
	idx.Nodes["my-proj"] = state.IndexEntry{
		Name:    "My Project",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: "my-proj",
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nsDir, "state.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	r := &Resolver{WolfcastleDir: tmpDir, Namespace: "alice-laptop"}
	loaded, err := r.LoadRootIndex()
	if err != nil {
		t.Fatalf("LoadRootIndex() error: %v", err)
	}
	if loaded.Version != 1 {
		t.Errorf("expected version=1, got %d", loaded.Version)
	}
	if len(loaded.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(loaded.Nodes))
	}
	if loaded.Nodes["my-proj"].Name != "My Project" {
		t.Errorf("expected node name 'My Project', got %q", loaded.Nodes["my-proj"].Name)
	}
}

func TestResolver_LoadRootIndex_FileNotFound(t *testing.T) {
	t.Parallel()
	r := &Resolver{WolfcastleDir: "/nonexistent", Namespace: "ns"}
	_, err := r.LoadRootIndex()
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestResolver_LoadNodeState(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	nodeDir := filepath.Join(tmpDir, "projects", "ns", "my-proj")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}

	ns := state.NewNodeState("my-proj", "My Project", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "Do something", State: state.StatusNotStarted},
	}
	data, err := json.MarshalIndent(ns, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeDir, "state.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	r := &Resolver{WolfcastleDir: tmpDir, Namespace: "ns"}
	addr := MustParse("my-proj")
	loaded, err := r.LoadNodeState(addr)
	if err != nil {
		t.Fatalf("LoadNodeState() error: %v", err)
	}
	if loaded.ID != "my-proj" {
		t.Errorf("expected ID 'my-proj', got %q", loaded.ID)
	}
	if loaded.Name != "My Project" {
		t.Errorf("expected Name 'My Project', got %q", loaded.Name)
	}
	if loaded.Type != state.NodeLeaf {
		t.Errorf("expected type leaf, got %q", loaded.Type)
	}
	if len(loaded.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(loaded.Tasks))
	}
}

func TestResolver_LoadNodeState_FileNotFound(t *testing.T) {
	t.Parallel()
	r := &Resolver{WolfcastleDir: "/nonexistent", Namespace: "ns"}
	_, err := r.LoadNodeState(MustParse("missing"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestValidateSlug_InvalidCharacter(t *testing.T) {
	t.Parallel()
	if err := ValidateSlug("hello_world"); err == nil {
		t.Error("expected error for underscore character")
	}
}

func TestValidateSlug_EndingWithHyphen(t *testing.T) {
	t.Parallel()
	if err := ValidateSlug("node-"); err == nil {
		t.Error("expected error for ending with hyphen")
	}
}

func TestParseAddress_InvalidSlug(t *testing.T) {
	t.Parallel()
	_, err := ParseAddress("INVALID")
	if err == nil {
		t.Error("expected error for uppercase slug in address")
	}
}

func TestParseAddress_EmptyString(t *testing.T) {
	t.Parallel()
	addr, err := ParseAddress("")
	if err != nil {
		t.Fatal(err)
	}
	if addr.Raw != "" {
		t.Errorf("expected empty raw, got %q", addr.Raw)
	}
	if len(addr.Parts) != 0 {
		t.Errorf("expected no parts, got %v", addr.Parts)
	}
}

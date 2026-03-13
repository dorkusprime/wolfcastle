package tree

import (
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// Resolver resolves tree addresses to filesystem paths and state.
type Resolver struct {
	WolfcastleDir string
	Namespace     string
}

// NewResolver creates a resolver for the given config.
func NewResolver(wolfcastleDir string, cfg *config.Config) (*Resolver, error) {
	ns, err := ResolveNamespace(cfg)
	if err != nil {
		return nil, err
	}
	return &Resolver{
		WolfcastleDir: wolfcastleDir,
		Namespace:     ns,
	}, nil
}

// ProjectsDir returns the root projects directory for this engineer.
func (r *Resolver) ProjectsDir() string {
	return filepath.Join(r.WolfcastleDir, "projects", r.Namespace)
}

// RootIndexPath returns the path to the root state.json.
func (r *Resolver) RootIndexPath() string {
	return filepath.Join(r.ProjectsDir(), "state.json")
}

// NodeDir returns the filesystem directory for a node address.
func (r *Resolver) NodeDir(addr Address) string {
	if addr.IsRoot() {
		return r.ProjectsDir()
	}
	return filepath.Join(r.ProjectsDir(), filepath.Join(addr.Parts...))
}

// NodeStatePath returns the path to a node's state.json.
func (r *Resolver) NodeStatePath(addr Address) string {
	return filepath.Join(r.NodeDir(addr), "state.json")
}

// NodeDefPath returns the path to a node's definition Markdown file.
func (r *Resolver) NodeDefPath(addr Address) string {
	if addr.IsRoot() {
		return ""
	}
	return filepath.Join(r.NodeDir(addr.Parent()), addr.Leaf()+".md")
}

// TaskDocPath returns the path to a task's companion Markdown file.
func (r *Resolver) TaskDocPath(addr Address, taskID string) string {
	return filepath.Join(r.NodeDir(addr), taskID+".md")
}

// LoadRootIndex loads the root index for this resolver.
func (r *Resolver) LoadRootIndex() (*state.RootIndex, error) {
	return state.LoadRootIndex(r.RootIndexPath())
}

// LoadNodeState loads a node's state from its address.
func (r *Resolver) LoadNodeState(addr Address) (*state.NodeState, error) {
	return state.LoadNodeState(r.NodeStatePath(addr))
}

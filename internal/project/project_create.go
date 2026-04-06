// Package project handles project creation, scaffolding, and template
// management for Wolfcastle workspaces. It embeds the base prompt and
// rule templates extracted during initialization (ADR-033) and provides
// the migration service for upgrading existing project directories.
package project

import (
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// WriteAuditTaskMD writes audit.md into nodeDir by resolving the audit-task
// template through the three-tier system. This allows users to override the
// audit template via custom or local tiers.
func WriteAuditTaskMD(prompts *pipeline.PromptRepository, nodeDir string) {
	_ = prompts.RenderToFile("artifacts/audit-task.md", nil, filepath.Join(nodeDir, "audit.md"))
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

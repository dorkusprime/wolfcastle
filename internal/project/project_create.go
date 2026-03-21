// Package project handles project creation, scaffolding, and template
// management for Wolfcastle workspaces. It embeds the base prompt and
// rule templates extracted during initialization (ADR-033) and provides
// the migration service for upgrading existing project directories.
package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

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

package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/archive"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// findArchiveEligible returns addresses of root-level nodes eligible for
// auto-archival. A node qualifies when it is in Root (not ArchivedRoot),
// its IndexEntry state is complete, it is not already archived, and its
// CompletedAt timestamp is older than the configured delay.
func (d *Daemon) findArchiveEligible(idx *state.RootIndex) []string {
	cfg := d.Config.Archive
	delay := time.Duration(cfg.AutoArchiveDelayHours) * time.Hour
	now := d.Clock.Now()

	var eligible []string
	for _, addr := range idx.Root {
		entry, ok := idx.Nodes[addr]
		if !ok {
			continue
		}
		if entry.State != state.StatusComplete {
			continue
		}
		if entry.Archived {
			continue
		}

		// Load the root node to check CompletedAt on its audit state.
		ns, err := d.Store.ReadNode(addr)
		if err != nil {
			continue
		}
		if ns.Audit.CompletedAt == nil {
			continue
		}
		if now.Sub(*ns.Audit.CompletedAt) < delay {
			continue
		}

		eligible = append(eligible, addr)
	}
	return eligible
}

// archiveNode moves a completed root-level node from active state to
// archive storage. It generates a Markdown rollup, relocates state
// directories under .archive/, and updates the RootIndex.
func (d *Daemon) archiveNode(addr string) error {
	// Generate the Markdown rollup entry.
	ns, err := d.Store.ReadNode(addr)
	if err != nil {
		return fmt.Errorf("reading node %s for archive: %w", addr, err)
	}

	entry := archive.GenerateEntry(addr, ns, d.Config, d.branch, ns.Audit.ResultSummary, d.Clock)
	archiveMarkdownDir := filepath.Join(d.WolfcastleDir, "archive")
	if err := os.MkdirAll(archiveMarkdownDir, 0o755); err != nil {
		return fmt.Errorf("creating archive markdown dir: %w", err)
	}
	rollupPath := filepath.Join(archiveMarkdownDir, entry.Filename)
	if err := os.WriteFile(rollupPath, []byte(entry.Content), 0o644); err != nil {
		return fmt.Errorf("writing archive rollup: %w", err)
	}

	// Collect subtree addresses (root + all descendants).
	idx, err := d.Store.ReadIndex()
	if err != nil {
		return fmt.Errorf("reading index for archive: %w", err)
	}
	subtree := collectSubtree(idx, addr)

	// Move state directories from active to .archive/.
	storeDir := d.Store.Dir()
	for _, nodeAddr := range subtree {
		parts := strings.Split(nodeAddr, "/")
		activeDir := filepath.Join(storeDir, filepath.Join(parts...))
		archiveDir := filepath.Join(storeDir, ".archive", filepath.Join(parts...))

		// Only move if the active directory exists.
		if _, statErr := os.Stat(activeDir); statErr != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(archiveDir), 0o755); err != nil {
			return fmt.Errorf("creating archive dir for %s: %w", nodeAddr, err)
		}
		if err := os.Rename(activeDir, archiveDir); err != nil {
			return fmt.Errorf("moving %s to archive: %w", nodeAddr, err)
		}
	}

	// Update the RootIndex atomically.
	now := d.Clock.Now()
	return d.Store.MutateIndex(func(idx *state.RootIndex) error {
		// Move from Root to ArchivedRoot.
		var newRoot []string
		for _, r := range idx.Root {
			if r != addr {
				newRoot = append(newRoot, r)
			}
		}
		idx.Root = newRoot
		idx.ArchivedRoot = append(idx.ArchivedRoot, addr)

		// Flag all subtree entries as archived.
		for _, nodeAddr := range subtree {
			if e, ok := idx.Nodes[nodeAddr]; ok {
				e.Archived = true
				e.ArchivedAt = &now
				idx.Nodes[nodeAddr] = e
			}
		}
		return nil
	})
}

// restoreNode reverses an archive operation, moving a node and its subtree
// from .archive/ back to active state. The node must exist in the RootIndex
// with Archived==true and must appear in ArchivedRoot.
func (d *Daemon) restoreNode(addr string) error {
	idx, err := d.Store.ReadIndex()
	if err != nil {
		return fmt.Errorf("reading index for restore: %w", err)
	}

	entry, ok := idx.Nodes[addr]
	if !ok {
		return fmt.Errorf("node %q not found in index", addr)
	}
	if !entry.Archived {
		return fmt.Errorf("node %q is not archived", addr)
	}

	inArchivedRoot := false
	for _, r := range idx.ArchivedRoot {
		if r == addr {
			inArchivedRoot = true
			break
		}
	}
	if !inArchivedRoot {
		return fmt.Errorf("node %q is not a root-level archived node", addr)
	}

	subtree := collectSubtree(idx, addr)

	// Move state directories from .archive/ back to active locations.
	storeDir := d.Store.Dir()
	for _, nodeAddr := range subtree {
		parts := strings.Split(nodeAddr, "/")
		archiveDir := filepath.Join(storeDir, ".archive", filepath.Join(parts...))
		activeDir := filepath.Join(storeDir, filepath.Join(parts...))

		if _, statErr := os.Stat(archiveDir); statErr != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(activeDir), 0o755); err != nil {
			return fmt.Errorf("creating active dir for %s: %w", nodeAddr, err)
		}
		if err := os.Rename(archiveDir, activeDir); err != nil {
			return fmt.Errorf("restoring %s from archive: %w", nodeAddr, err)
		}
	}

	// Update the RootIndex atomically.
	return d.Store.MutateIndex(func(idx *state.RootIndex) error {
		// Move from ArchivedRoot to Root.
		var newArchivedRoot []string
		for _, r := range idx.ArchivedRoot {
			if r != addr {
				newArchivedRoot = append(newArchivedRoot, r)
			}
		}
		idx.ArchivedRoot = newArchivedRoot
		idx.Root = append(idx.Root, addr)

		// Clear archive flags on all subtree entries.
		for _, nodeAddr := range subtree {
			if e, ok := idx.Nodes[nodeAddr]; ok {
				e.Archived = false
				e.ArchivedAt = nil
				idx.Nodes[nodeAddr] = e
			}
		}
		return nil
	})
}

// deleteArchivedNode permanently removes an archived node and its subtree
// from the archive store and RootIndex. The node must exist in the index
// with Archived==true and must appear in ArchivedRoot. Markdown rollup
// files in .wolfcastle/archive/ are preserved as permanent records.
func (d *Daemon) deleteArchivedNode(addr string) error {
	idx, err := d.Store.ReadIndex()
	if err != nil {
		return fmt.Errorf("reading index for delete: %w", err)
	}

	entry, ok := idx.Nodes[addr]
	if !ok {
		return fmt.Errorf("node %q not found in index", addr)
	}
	if !entry.Archived {
		return fmt.Errorf("node %q is not archived", addr)
	}

	inArchivedRoot := false
	for _, r := range idx.ArchivedRoot {
		if r == addr {
			inArchivedRoot = true
			break
		}
	}
	if !inArchivedRoot {
		return fmt.Errorf("node %q is not a root-level archived node", addr)
	}

	subtree := collectSubtree(idx, addr)

	// Remove archived state directories. RemoveAll handles nested
	// descendants, so removing the root archive directory suffices,
	// but we call it on the root address specifically to be precise.
	storeDir := d.Store.Dir()
	archiveRoot := filepath.Join(storeDir, ".archive", filepath.Join(strings.Split(addr, "/")...))
	if err := os.RemoveAll(archiveRoot); err != nil {
		return fmt.Errorf("removing archived directory for %s: %w", addr, err)
	}

	// Update the RootIndex atomically: remove from ArchivedRoot and
	// purge all subtree entries from the Nodes map.
	return d.Store.MutateIndex(func(idx *state.RootIndex) error {
		var newArchivedRoot []string
		for _, r := range idx.ArchivedRoot {
			if r != addr {
				newArchivedRoot = append(newArchivedRoot, r)
			}
		}
		idx.ArchivedRoot = newArchivedRoot

		for _, nodeAddr := range subtree {
			delete(idx.Nodes, nodeAddr)
		}
		return nil
	})
}

// collectSubtree returns an address and all its descendants by walking
// the Children slices in the RootIndex. The root address is always first.
func collectSubtree(idx *state.RootIndex, root string) []string {
	var result []string
	var walk func(addr string)
	walk = func(addr string) {
		result = append(result, addr)
		if entry, ok := idx.Nodes[addr]; ok {
			for _, child := range entry.Children {
				walk(child)
			}
		}
	}
	walk(root)
	return result
}

// tryAutoArchive checks whether enough time has elapsed since the last
// archive poll, finds eligible nodes, and archives at most one. Returns
// true if a node was archived (the caller should report IterationDidWork).
func (d *Daemon) tryAutoArchive(idx *state.RootIndex) bool {
	cfg := d.Config.Archive
	if !cfg.AutoArchiveEnabled {
		return false
	}

	pollInterval := time.Duration(cfg.PollIntervalSeconds) * time.Second
	if d.Clock.Now().Sub(d.lastArchiveCheck) < pollInterval {
		return false
	}
	d.lastArchiveCheck = d.Clock.Now()

	eligible := d.findArchiveEligible(idx)
	if len(eligible) == 0 {
		return false
	}

	// Archive one node per iteration to keep each cycle bounded.
	addr := eligible[0]
	if err := d.archiveNode(addr); err != nil {
		_ = d.Logger.Log(map[string]any{
			"type":  "auto_archive_error",
			"node":  addr,
			"error": err.Error(),
		})
		output.PrintHuman("Auto-archive failed for %s: %v", addr, err)
		return false
	}

	_ = d.Logger.Log(map[string]any{
		"type": "auto_archive",
		"node": addr,
	})
	output.PrintHuman("Archived completed project: %s", addr)
	return true
}

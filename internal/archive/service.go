package archive

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// Service provides archive operations (archive, restore, delete) that both
// the daemon and CLI can share. It holds the common dependencies; callers
// construct it with whatever Store, WolfcastleDir, and Clock they have.
type Service struct {
	Store         *state.Store
	WolfcastleDir string
	Clock         clock.Clock
}

func (s *Service) clk() clock.Clock {
	if s.Clock != nil {
		return s.Clock
	}
	return clock.New()
}

// Archive moves a completed root-level node from active state to archive
// storage. It generates a Markdown rollup, relocates state directories
// under .archive/, and updates the RootIndex atomically.
func (s *Service) Archive(addr string, cfg *config.Config, branch string) error {
	ns, err := s.Store.ReadNode(addr)
	if err != nil {
		return fmt.Errorf("reading node %s for archive: %w", addr, err)
	}

	entry := GenerateEntry(addr, ns, cfg, branch, ns.Audit.ResultSummary, s.clk())
	archiveMarkdownDir := filepath.Join(s.WolfcastleDir, "archive")
	if err := os.MkdirAll(archiveMarkdownDir, 0o755); err != nil {
		return fmt.Errorf("creating archive markdown dir: %w", err)
	}
	rollupPath := filepath.Join(archiveMarkdownDir, entry.Filename)
	if err := state.AtomicWriteFile(rollupPath, []byte(entry.Content)); err != nil {
		return fmt.Errorf("writing archive rollup: %w", err)
	}

	idx, err := s.Store.ReadIndex()
	if err != nil {
		return fmt.Errorf("reading index for archive: %w", err)
	}
	subtree := CollectSubtree(idx, addr)

	storeDir := s.Store.Dir()
	for _, nodeAddr := range subtree {
		parts := strings.Split(nodeAddr, "/")
		activeDir := filepath.Join(storeDir, filepath.Join(parts...))
		archiveDir := filepath.Join(storeDir, ".archive", filepath.Join(parts...))

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

	now := s.clk().Now()
	return s.Store.MutateIndex(func(idx *state.RootIndex) error {
		var newRoot []string
		for _, r := range idx.Root {
			if r != addr {
				newRoot = append(newRoot, r)
			}
		}
		idx.Root = newRoot
		idx.ArchivedRoot = append(idx.ArchivedRoot, addr)

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

// Restore reverses an archive operation, moving a node and its subtree
// from .archive/ back to active state. The node must exist in the
// RootIndex with Archived==true and appear in ArchivedRoot.
func (s *Service) Restore(addr string) error {
	idx, err := s.Store.ReadIndex()
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

	if !inArchivedRoot(idx, addr) {
		return fmt.Errorf("node %q is not a root-level archived node", addr)
	}

	subtree := CollectSubtree(idx, addr)

	storeDir := s.Store.Dir()
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

	return s.Store.MutateIndex(func(idx *state.RootIndex) error {
		var newArchivedRoot []string
		for _, r := range idx.ArchivedRoot {
			if r != addr {
				newArchivedRoot = append(newArchivedRoot, r)
			}
		}
		idx.ArchivedRoot = newArchivedRoot
		idx.Root = append(idx.Root, addr)

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

// Delete permanently removes an archived node and its subtree from the
// archive store and RootIndex. Markdown rollup files in .wolfcastle/archive/
// are preserved as permanent records.
func (s *Service) Delete(addr string) error {
	idx, err := s.Store.ReadIndex()
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

	if !inArchivedRoot(idx, addr) {
		return fmt.Errorf("node %q is not a root-level archived node", addr)
	}

	subtree := CollectSubtree(idx, addr)

	storeDir := s.Store.Dir()
	archiveRoot := filepath.Join(storeDir, ".archive", filepath.Join(strings.Split(addr, "/")...))
	if err := os.RemoveAll(archiveRoot); err != nil {
		return fmt.Errorf("removing archived directory for %s: %w", addr, err)
	}

	return s.Store.MutateIndex(func(idx *state.RootIndex) error {
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

// CollectSubtree returns an address and all its descendants by walking the
// Children slices in the RootIndex. The root address is always first.
func CollectSubtree(idx *state.RootIndex, root string) []string {
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

// inArchivedRoot checks whether addr appears in idx.ArchivedRoot.
func inArchivedRoot(idx *state.RootIndex, addr string) bool {
	for _, r := range idx.ArchivedRoot {
		if r == addr {
			return true
		}
	}
	return false
}

package daemon

import (
	"time"

	"fmt"

	"github.com/dorkusprime/wolfcastle/internal/archive"
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
// archive storage. It delegates to archive.Service for the shared logic.
func (d *Daemon) archiveNode(addr string) error {
	svc := d.archiveService()
	return svc.Archive(addr, d.Config, d.branch)
}

// restoreNode reverses an archive operation, moving a node and its subtree
// from .archive/ back to active state. It delegates to archive.Service.
func (d *Daemon) restoreNode(addr string) error {
	svc := d.archiveService()
	return svc.Restore(addr)
}

// deleteArchivedNode permanently removes an archived node and its subtree
// from the archive store and RootIndex. It delegates to archive.Service.
func (d *Daemon) deleteArchivedNode(addr string) error {
	svc := d.archiveService()
	return svc.Delete(addr)
}

// archiveService constructs an archive.Service from daemon fields.
func (d *Daemon) archiveService() *archive.Service {
	return &archive.Service{
		Store:         d.Store,
		WolfcastleDir: d.WolfcastleDir,
		Clock:         d.Clock,
	}
}

// collectSubtree delegates to archive.CollectSubtree.
func collectSubtree(idx *state.RootIndex, root string) []string {
	return archive.CollectSubtree(idx, root)
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
	d.mu.Lock()
	elapsed := d.Clock.Now().Sub(d.lastArchiveCheck) >= pollInterval
	if elapsed {
		d.lastArchiveCheck = d.Clock.Now()
	}
	d.mu.Unlock()
	if !elapsed {
		return false
	}

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
		d.log(map[string]any{"type": "archive_event", "action": "auto_archive_failed", "node": addr, "text": fmt.Sprintf("Auto-archive failed for %s: %v", addr, err), "error": err.Error()})
		return false
	}

	_ = d.Logger.Log(map[string]any{
		"type": "auto_archive",
		"node": addr,
	})
	d.log(map[string]any{"type": "archive_event", "action": "archived", "node": addr, "text": fmt.Sprintf("Archived completed project: %s", addr)})
	return true
}

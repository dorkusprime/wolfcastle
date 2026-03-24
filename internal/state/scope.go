package state

import "strings"

// ValidateScopePath reports whether p is a well-formed scope path. It rejects
// empty strings, paths containing ".." components, and absolute paths
// starting with "/".
func ValidateScopePath(p string) bool {
	if p == "" {
		return false
	}
	if strings.HasPrefix(p, "/") {
		return false
	}
	for _, seg := range strings.Split(strings.TrimSuffix(p, "/"), "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

// ScopeConflict describes a single conflict between a requested file and an
// existing scope lock held by another task.
type ScopeConflict struct {
	File       string `json:"file"`
	HeldByTask string `json:"held_by_task"`
	HeldByNode string `json:"held_by_node"`
}

// ScopeConflicts reports whether two scope entries conflict. Two entries
// conflict if either is a prefix of the other (bidirectional containment).
// A trailing slash marks a directory scope; a path without one is a file.
// Identical paths always conflict. Returns false for invalid paths (empty,
// containing "..", or absolute).
func ScopeConflicts(requested, existing string) bool {
	if !ValidateScopePath(requested) || !ValidateScopePath(existing) {
		return false
	}
	if requested == existing {
		return true
	}

	// A directory scope (trailing /) is a prefix match: it contains anything
	// that starts with it. A file path is only "contained" by a directory
	// whose prefix matches.
	if strings.HasSuffix(requested, "/") && strings.HasPrefix(existing, requested) {
		return true
	}
	if strings.HasSuffix(existing, "/") && strings.HasPrefix(requested, existing) {
		return true
	}

	return false
}

// FindConflicts returns all conflicts between the requested paths and locks in
// the table held by tasks other than taskAddr. Locks held by taskAddr itself
// are skipped, allowing idempotent re-acquisition.
func FindConflicts(requested []string, table *ScopeLockTable, taskAddr string) []ScopeConflict {
	var conflicts []ScopeConflict
	for _, req := range requested {
		if !ValidateScopePath(req) {
			continue
		}
		for scope, lock := range table.Locks {
			if lock.Task == taskAddr {
				continue
			}
			if ScopeConflicts(req, scope) {
				conflicts = append(conflicts, ScopeConflict{
					File:       req,
					HeldByTask: lock.Task,
					HeldByNode: lock.Node,
				})
			}
		}
	}
	return conflicts
}

package daemon

import "strings"

// validateScope classifies dirty files from git status --porcelain output into
// scope buckets. Files under .wolfcastle/ are excluded (they are state files,
// not code). Each remaining file is matched against taskScope (this task's
// declared files/directories) and otherScopes (all other active tasks' scopes).
//
// A scope entry that ends with "/" is treated as a directory prefix: any file
// whose path starts with that prefix matches. All other entries require an
// exact match.
//
// Returns inScope (files owned by this task) and unowned (files not claimed by
// any active task). Files matching another task's scope are silently skipped.
func validateScope(statusOutput string, taskScope []string, otherScopes [][]string) (inScope []string, unowned []string) {
	for _, line := range strings.Split(statusOutput, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// git status --porcelain format: XY <path>  (path starts at column 3)
		if len(line) < 4 {
			continue
		}
		path := line[3:]
		// Handle renames: "R  old -> new"
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}

		// Exclude .wolfcastle/ state files from classification.
		if strings.HasPrefix(path, ".wolfcastle/") || path == ".wolfcastle" {
			continue
		}

		if matchesScope(path, taskScope) {
			inScope = append(inScope, path)
		} else if matchesAnyScope(path, otherScopes) {
			// Belongs to another active task; skip silently.
			continue
		} else {
			unowned = append(unowned, path)
		}
	}
	return inScope, unowned
}

// matchesScope reports whether path matches any entry in scope. Entries ending
// with "/" use directory-prefix matching; all others require exact equality.
func matchesScope(path string, scope []string) bool {
	for _, entry := range scope {
		if strings.HasSuffix(entry, "/") {
			if strings.HasPrefix(path, entry) {
				return true
			}
		} else {
			if path == entry {
				return true
			}
		}
	}
	return false
}

// matchesAnyScope reports whether path matches an entry in any of the provided
// scope slices.
func matchesAnyScope(path string, scopes [][]string) bool {
	for _, scope := range scopes {
		if matchesScope(path, scope) {
			return true
		}
	}
	return false
}

package daemon

import (
	"os"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// checkDeliverables verifies that all declared deliverables for a task exist
// on disk and are non-empty. Returns the list of missing or empty deliverable
// paths. A task with no deliverables always passes.
func checkDeliverables(repoDir string, ns *state.NodeState, taskID string) []string {
	var missing []string
	for _, t := range ns.Tasks {
		if t.ID == taskID {
			for _, d := range t.Deliverables {
				path := filepath.Join(repoDir, d)
				info, err := os.Stat(path)
				if err != nil || info.Size() == 0 {
					missing = append(missing, d)
				}
			}
			break
		}
	}
	return missing
}

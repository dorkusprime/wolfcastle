package daemon

import (
	"fmt"

	"github.com/dorkusprime/wolfcastle/internal/knowledge"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// knowledgeMaintenanceTaskType is the TaskType assigned to auto-created
// knowledge pruning tasks.
const knowledgeMaintenanceTaskType = "knowledge-maintenance"

// knowledgeMaintenanceTitle is the title used for knowledge pruning tasks.
// Used both for creation and duplicate detection.
const knowledgeMaintenanceTitle = "Prune codebase knowledge file"

// checkKnowledgeBudget reads the knowledge file for the current namespace
// and, if its token count exceeds the configured budget, creates a
// maintenance task on the given node. The task instructs the agent to
// review, consolidate, and prune the file.
//
// Returns true if a maintenance task was created.
func (d *Daemon) checkKnowledgeBudget(nodeAddr string) bool {
	ns := d.namespace()
	if ns == "" {
		return false
	}

	maxTokens := d.Config.Knowledge.MaxTokens
	if maxTokens <= 0 {
		return false
	}

	content, err := knowledge.Read(d.WolfcastleDir, ns)
	if err != nil || content == "" {
		return false
	}

	tokens := knowledge.TokenCount(content)
	if tokens <= maxTokens {
		return false
	}

	// Check for an existing maintenance task to avoid duplicates.
	nodeState, err := d.Store.ReadNode(nodeAddr)
	if err != nil {
		return false
	}
	for _, t := range nodeState.Tasks {
		if t.TaskType == knowledgeMaintenanceTaskType {
			return false
		}
	}

	// Build the maintenance task.
	body := fmt.Sprintf(
		"The codebase knowledge file for namespace `%s` has grown to %d tokens, "+
			"exceeding the configured budget of %d tokens.\n\n"+
			"Review the knowledge file at `%s` and:\n"+
			"1. Remove entries that are stale or no longer accurate\n"+
			"2. Consolidate related entries into single, concise bullets\n"+
			"3. Remove entries that duplicate information in README, CONTRIBUTING.md, specs, or ADRs\n"+
			"4. If an entry describes an enforceable convention or coding standard (naming rules, import patterns, style requirements), migrate it to the appropriate class file or rule fragment instead of keeping it in the knowledge file — knowledge entries should be observations, not rules\n"+
			"5. Bring the total under %d tokens\n\n"+
			"Use `wolfcastle knowledge show` to view the current file and `wolfcastle knowledge prune` to edit it.",
		ns, tokens, maxTokens,
		knowledge.FilePath(d.WolfcastleDir, ns),
		maxTokens,
	)

	taskID := fmt.Sprintf("knowledge-prune-%s", ns)

	mutErr := d.Store.MutateNode(nodeAddr, func(nodeState *state.NodeState) error {
		// Double-check inside the mutation to avoid races.
		for _, t := range nodeState.Tasks {
			if t.TaskType == knowledgeMaintenanceTaskType {
				return fmt.Errorf("duplicate")
			}
		}

		task := state.Task{
			ID:          taskID,
			Title:       knowledgeMaintenanceTitle,
			Description: fmt.Sprintf("Prune codebase knowledge file (currently %d/%d tokens)", tokens, maxTokens),
			State:       state.StatusNotStarted,
			TaskType:    knowledgeMaintenanceTaskType,
			Body:        body,
		}

		// Insert before the audit task.
		insertIdx := len(nodeState.Tasks)
		for i, t := range nodeState.Tasks {
			if t.IsAudit {
				insertIdx = i
				break
			}
		}
		nodeState.Tasks = append(nodeState.Tasks[:insertIdx],
			append([]state.Task{task}, nodeState.Tasks[insertIdx:]...)...)

		state.MoveAuditLast(nodeState)
		return nil
	})

	if mutErr != nil {
		// "duplicate" error from the double-check is expected; don't log it as an error.
		return false
	}

	_ = d.Logger.Log(map[string]any{
		"type":       "knowledge_maintenance_created",
		"node":       nodeAddr,
		"task_id":    taskID,
		"tokens":     tokens,
		"max_tokens": maxTokens,
	})
	output.PrintHuman("  Knowledge budget exceeded (%d/%d tokens), maintenance task queued", tokens, maxTokens)
	return true
}

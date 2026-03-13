package state

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TaskAdd inserts a new task into a leaf node before the audit task.
func TaskAdd(ns *NodeState, description string) (*Task, error) {
	if ns.Type != NodeLeaf {
		return nil, fmt.Errorf("cannot add tasks to %s node (must be leaf)", ns.Type)
	}

	// Generate task ID
	maxNum := 0
	for _, t := range ns.Tasks {
		if strings.HasPrefix(t.ID, "task-") {
			if n, err := strconv.Atoi(t.ID[5:]); err == nil && n > maxNum {
				maxNum = n
			}
		}
	}
	taskID := fmt.Sprintf("task-%d", maxNum+1)

	task := Task{
		ID:          taskID,
		Description: description,
		State:       StatusNotStarted,
	}

	// Insert before audit task (always last)
	insertIdx := len(ns.Tasks)
	for i, t := range ns.Tasks {
		if t.ID == "audit" {
			insertIdx = i
			break
		}
	}
	ns.Tasks = append(ns.Tasks[:insertIdx], append([]Task{task}, ns.Tasks[insertIdx:]...)...)

	return &task, nil
}

// TaskClaim transitions a task from not_started to in_progress.
func TaskClaim(ns *NodeState, taskID string) error {
	t := findTask(ns, taskID)
	if t == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if t.State != StatusNotStarted {
		return fmt.Errorf("task %q is %s, must be not_started to claim", taskID, t.State)
	}
	t.State = StatusInProgress

	// Update leaf state
	ns.State = StatusInProgress
	return nil
}

// TaskComplete transitions a task from in_progress to complete.
func TaskComplete(ns *NodeState, taskID string) error {
	t := findTask(ns, taskID)
	if t == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if t.State != StatusInProgress {
		return fmt.Errorf("task %q is %s, must be in_progress to complete", taskID, t.State)
	}
	t.State = StatusComplete

	// Check if all tasks are complete
	allComplete := true
	for _, task := range ns.Tasks {
		if task.State != StatusComplete {
			allComplete = false
			break
		}
	}
	if allComplete {
		ns.State = StatusComplete
	}
	return nil
}

// TaskBlock transitions a task from in_progress to blocked.
func TaskBlock(ns *NodeState, taskID string, reason string) error {
	t := findTask(ns, taskID)
	if t == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if t.State != StatusInProgress {
		return fmt.Errorf("task %q is %s, must be in_progress to block", taskID, t.State)
	}
	t.State = StatusBlocked
	t.BlockReason = reason

	// Check if all non-complete tasks are blocked
	allBlockedOrComplete := true
	for _, task := range ns.Tasks {
		if task.State != StatusComplete && task.State != StatusBlocked {
			allBlockedOrComplete = false
			break
		}
	}
	if allBlockedOrComplete {
		ns.State = StatusBlocked
	}
	return nil
}

// TaskUnblock transitions a task from blocked to not_started and resets failure counter.
func TaskUnblock(ns *NodeState, taskID string) error {
	t := findTask(ns, taskID)
	if t == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if t.State != StatusBlocked {
		return fmt.Errorf("task %q is %s, must be blocked to unblock", taskID, t.State)
	}
	t.State = StatusNotStarted
	t.BlockReason = ""
	t.FailureCount = 0

	// Leaf is no longer fully blocked
	ns.State = StatusInProgress
	return nil
}

// AddBreadcrumb appends a breadcrumb to the node's audit trail.
func AddBreadcrumb(ns *NodeState, taskAddr string, text string) {
	ns.Audit.Breadcrumbs = append(ns.Audit.Breadcrumbs, Breadcrumb{
		Timestamp: time.Now().UTC(),
		Task:      taskAddr,
		Text:      text,
	})
}

// AddEscalation adds an escalation to a parent node.
func AddEscalation(parent *NodeState, sourceNode string, description string, sourceGapID string) {
	id := fmt.Sprintf("escalation-%s-%d", parent.ID, len(parent.Audit.Escalations)+1)
	parent.Audit.Escalations = append(parent.Audit.Escalations, Escalation{
		ID:          id,
		Timestamp:   time.Now().UTC(),
		Description: description,
		SourceNode:  sourceNode,
		SourceGapID: sourceGapID,
		Status:      "open",
	})
}

// IncrementFailure increments a task's failure counter and returns the new count.
func IncrementFailure(ns *NodeState, taskID string) (int, error) {
	t := findTask(ns, taskID)
	if t == nil {
		return 0, fmt.Errorf("task %q not found", taskID)
	}
	t.FailureCount++
	return t.FailureCount, nil
}

func findTask(ns *NodeState, taskID string) *Task {
	for i := range ns.Tasks {
		if ns.Tasks[i].ID == taskID {
			return &ns.Tasks[i]
		}
	}
	return nil
}

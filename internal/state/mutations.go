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
		if t.IsAudit {
			insertIdx = i
			break
		}
	}
	ns.Tasks = append(ns.Tasks[:insertIdx], append([]Task{task}, ns.Tasks[insertIdx:]...)...)

	// Ensure audit task stays last after insertion
	MoveAuditLast(ns)

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
	SyncAuditLifecycle(ns)
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
	SyncAuditLifecycle(ns)
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
	t.BlockedReason = reason

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
	SyncAuditLifecycle(ns)
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
	t.BlockedReason = ""
	t.FailureCount = 0

	// Leaf is no longer fully blocked
	ns.State = StatusInProgress
	SyncAuditLifecycle(ns)
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
// The ID uses the child (source) slug per spec: escalation-{child-slug}-{sequential}.
func AddEscalation(parent *NodeState, sourceNode string, description string, sourceGapID string) {
	// Extract child slug from the source node address (last segment)
	childSlug := sourceNode
	for i := len(sourceNode) - 1; i >= 0; i-- {
		if sourceNode[i] == '/' {
			childSlug = sourceNode[i+1:]
			break
		}
	}
	id := fmt.Sprintf("escalation-%s-%d", childSlug, len(parent.Audit.Escalations)+1)
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

// MoveAuditLast ensures the audit task is always the last task in the list.
// Call after any function that modifies the task list.
func MoveAuditLast(ns *NodeState) {
	var auditIdx int = -1
	for i, t := range ns.Tasks {
		if t.IsAudit {
			auditIdx = i
			break
		}
	}
	if auditIdx < 0 || auditIdx == len(ns.Tasks)-1 {
		return
	}
	audit := ns.Tasks[auditIdx]
	ns.Tasks = append(ns.Tasks[:auditIdx], ns.Tasks[auditIdx+1:]...)
	ns.Tasks = append(ns.Tasks, audit)
}

// SetNeedsDecomposition flags or clears the decomposition recommendation on a task.
func SetNeedsDecomposition(ns *NodeState, taskID string, needs bool) {
	t := findTask(ns, taskID)
	if t != nil {
		t.NeedsDecomposition = needs
	}
}

func findTask(ns *NodeState, taskID string) *Task {
	for i := range ns.Tasks {
		if ns.Tasks[i].ID == taskID {
			return &ns.Tasks[i]
		}
	}
	return nil
}

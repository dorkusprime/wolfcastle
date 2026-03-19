package state

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/clock"
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
	taskID := fmt.Sprintf("task-%04d", maxNum+1)

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

// TaskAddChild creates a child task under the given parent with a hierarchical
// ID (e.g., task-0001.0001). The child is inserted immediately after the parent
// and any existing children in the flat task list.
func TaskAddChild(ns *NodeState, parentID, description string) (*Task, error) {
	// Verify parent exists
	parentFound := false
	for _, t := range ns.Tasks {
		if t.ID == parentID {
			parentFound = true
			break
		}
	}
	if !parentFound {
		return nil, fmt.Errorf("parent task %q not found", parentID)
	}

	// Find the highest existing child number under this parent
	prefix := parentID + "."
	maxChild := 0
	for _, t := range ns.Tasks {
		if strings.HasPrefix(t.ID, prefix) {
			// Extract the immediate child number (first segment after prefix)
			rest := t.ID[len(prefix):]
			if dot := strings.Index(rest, "."); dot >= 0 {
				rest = rest[:dot]
			}
			if n, err := strconv.Atoi(rest); err == nil && n > maxChild {
				maxChild = n
			}
		}
	}
	childID := fmt.Sprintf("%s.%04d", parentID, maxChild+1)

	task := Task{
		ID:          childID,
		Description: description,
		State:       StatusNotStarted,
	}

	// Insert after parent and all existing children (maintains lexicographic order)
	insertIdx := len(ns.Tasks)
	pastParent := false
	for i, t := range ns.Tasks {
		if t.ID == parentID {
			pastParent = true
			continue
		}
		if pastParent && !strings.HasPrefix(t.ID, prefix) {
			insertIdx = i
			break
		}
	}
	ns.Tasks = append(ns.Tasks[:insertIdx], append([]Task{task}, ns.Tasks[insertIdx:]...)...)

	return &task, nil
}

// TaskChildren returns true if the given task has any children in the task list.
func TaskChildren(ns *NodeState, taskID string) bool {
	prefix := taskID + "."
	for _, t := range ns.Tasks {
		if strings.HasPrefix(t.ID, prefix) {
			return true
		}
	}
	return false
}

// DeriveParentStatus computes a parent task's status from its children.
// Returns the derived status and true if the task has children, or the
// task's own status and false if it has no children.
func DeriveParentStatus(ns *NodeState, taskID string) (NodeStatus, bool) {
	prefix := taskID + "."
	hasChildren := false
	allComplete := true
	anyInProgress := false
	anyBlocked := false

	for _, t := range ns.Tasks {
		if !strings.HasPrefix(t.ID, prefix) {
			continue
		}
		// Only consider immediate children (one level deep)
		rest := t.ID[len(prefix):]
		if strings.Contains(rest, ".") {
			continue
		}
		hasChildren = true
		switch t.State {
		case StatusComplete:
			// ok
		case StatusInProgress:
			anyInProgress = true
			allComplete = false
		case StatusBlocked:
			anyBlocked = true
			allComplete = false
		default:
			allComplete = false
		}
	}

	if !hasChildren {
		for _, t := range ns.Tasks {
			if t.ID == taskID {
				return t.State, false
			}
		}
		return StatusNotStarted, false
	}

	if allComplete {
		// Audit tasks with remediation children: if the audit itself
		// is complete (re-verification passed), derive complete. If
		// the audit hasn't re-run yet (not_started/blocked), derive
		// not_started so it gets picked up for re-verification.
		for _, t := range ns.Tasks {
			if t.ID == taskID && t.IsAudit && t.State != StatusComplete {
				return StatusNotStarted, true
			}
		}
		return StatusComplete, true
	}
	if anyInProgress {
		return StatusInProgress, true
	}
	if anyBlocked {
		return StatusBlocked, true
	}
	return StatusNotStarted, true
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
// If the task was already blocked by the model during execution
// (via CLI), this is a no-op: the blocked state takes precedence
// and MutateNode still propagates.
func TaskComplete(ns *NodeState, taskID string) error {
	t := findTask(ns, taskID)
	if t == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if t.State == StatusComplete || t.State == StatusBlocked {
		// Already in a terminal state (model set it via CLI during execution).
		// Don't error; let MutateNode propagate the current state.
		return nil
	}
	if t.State != StatusInProgress {
		return fmt.Errorf("task %q is %s, must be in_progress to complete", taskID, t.State)
	}
	t.State = StatusComplete

	// If this is a child task (e.g., task-0001.0002), check if all siblings
	// are done and auto-complete the parent task.
	if isChildTask(taskID) {
		parentID := parentTaskID(taskID)
		if parent := findTask(ns, parentID); parent != nil {
			if derived, hasChildren := DeriveParentStatus(ns, parentID); hasChildren && derived == StatusComplete {
				parent.State = StatusComplete
			}
		}
	}

	// Recompute node state using derived status for parent tasks.
	allComplete := true
	allBlockedOrComplete := true
	for _, task := range ns.Tasks {
		status := task.State
		if derived, hasChildren := DeriveParentStatus(ns, task.ID); hasChildren {
			status = derived
		}
		if status != StatusComplete {
			allComplete = false
		}
		if status != StatusComplete && status != StatusBlocked {
			allBlockedOrComplete = false
		}
	}
	if allComplete {
		ns.State = StatusComplete
	} else if allBlockedOrComplete {
		ns.State = StatusBlocked
	}
	SyncAuditLifecycle(ns)
	return nil
}

// TaskBlock transitions a task to blocked. Accepts both in_progress
// and not_started as source states. Blocking a not_started task is a
// pre-block: "don't even start this, here's why."
func TaskBlock(ns *NodeState, taskID string, reason string) error {
	t := findTask(ns, taskID)
	if t == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if t.State == StatusBlocked {
		// Already blocked (model did it via CLI during execution).
		// Update reason if provided, but don't error.
		if reason != "" {
			t.BlockedReason = reason
		}
		// Still run the node-state recomputation below.
	} else if t.State != StatusInProgress && t.State != StatusNotStarted {
		return fmt.Errorf("task %q is %s, must be in_progress or not_started to block", taskID, t.State)
	} else {
		t.State = StatusBlocked
		t.BlockedReason = reason
	}

	// Check if all non-audit tasks are blocked (none completed).
	// If so, auto-block the audit task too: nothing to verify.
	nonAuditAllBlocked := true
	anyNonAuditComplete := false
	for _, task := range ns.Tasks {
		if task.IsAudit {
			continue
		}
		if task.State == StatusComplete {
			anyNonAuditComplete = true
		}
		if task.State != StatusBlocked {
			nonAuditAllBlocked = false
		}
	}
	if nonAuditAllBlocked && !anyNonAuditComplete {
		for i, task := range ns.Tasks {
			if task.IsAudit && task.State == StatusNotStarted {
				ns.Tasks[i].State = StatusBlocked
				ns.Tasks[i].BlockedReason = "all tasks blocked; nothing to audit"
			}
		}
	}

	// Check if all non-complete tasks are blocked → node is blocked
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
func AddBreadcrumb(ns *NodeState, taskAddr string, text string, clk clock.Clock) {
	ns.Audit.Breadcrumbs = append(ns.Audit.Breadcrumbs, Breadcrumb{
		Timestamp: clk.Now(),
		Task:      taskAddr,
		Text:      text,
	})
}

// AddEscalation adds an escalation to a parent node.
// The ID uses the child (source) slug per spec: escalation-{child-slug}-{sequential}.
func AddEscalation(parent *NodeState, sourceNode string, description string, sourceGapID string, clk clock.Clock) {
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
		Timestamp:   clk.Now(),
		Description: description,
		SourceNode:  sourceNode,
		SourceGapID: sourceGapID,
		Status:      EscalationOpen,
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
	var auditIdx = -1
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

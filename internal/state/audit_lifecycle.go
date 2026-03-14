package state

import "time"

// SyncAuditLifecycle synchronizes the audit status with the node's task state.
// Call after TaskClaim, TaskComplete, TaskBlock, TaskUnblock, and after
// gap creation/fixing.
//
// State mapping:
//
//	not_started  → pending
//	in_progress  → in_progress  (records StartedAt on first transition)
//	complete + open gaps → failed (blocks audit task)
//	complete + no open gaps → passed (records CompletedAt)
//	blocked      → failed
func SyncAuditLifecycle(ns *NodeState) {
	switch ns.State {
	case StatusNotStarted:
		ns.Audit.Status = AuditPending

	case StatusInProgress:
		if ns.Audit.Status != AuditInProgress {
			ns.Audit.Status = AuditInProgress
			if ns.Audit.StartedAt == nil {
				now := time.Now().UTC()
				ns.Audit.StartedAt = &now
			}
		}

	case StatusComplete:
		if hasOpenGaps(ns) {
			ns.Audit.Status = AuditFailed
			// Revert: a node cannot be complete with open gaps.
			// Block the audit task and revert node to in_progress.
			blockAuditTask(ns)
			ns.State = StatusInProgress
		} else {
			ns.Audit.Status = AuditPassed
			if ns.Audit.CompletedAt == nil {
				now := time.Now().UTC()
				ns.Audit.CompletedAt = &now
			}
		}

	case StatusBlocked:
		ns.Audit.Status = AuditFailed
	}
}

// hasOpenGaps returns true if any gap has status "open".
func hasOpenGaps(ns *NodeState) bool {
	for _, g := range ns.Audit.Gaps {
		if g.Status == "open" {
			return true
		}
	}
	return false
}

// blockAuditTask blocks the audit task when open gaps exist.
// Handles any current state (in_progress, complete, not_started).
func blockAuditTask(ns *NodeState) {
	for i := range ns.Tasks {
		if ns.Tasks[i].IsAudit && ns.Tasks[i].State != StatusBlocked {
			ns.Tasks[i].State = StatusBlocked
			ns.Tasks[i].BlockedReason = "open gaps must be resolved before audit can pass"
			break
		}
	}
}

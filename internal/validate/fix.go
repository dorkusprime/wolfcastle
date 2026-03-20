package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// FixResult describes a single applied fix.
type FixResult struct {
	Category    string
	Node        string
	Description string
	Pass        int
}

const maxFixPasses = 5

// FixWithVerification runs a multi-pass fix loop per ADR-051. Each pass
// validates the tree, applies deterministic fixes, and re-validates. The
// loop exits when no fixable issues remain or the pass cap is reached.
// Returns a final report with all residual issues.
func FixWithVerification(
	projectsDir string,
	indexPath string,
	loadNode NodeLoader,
	wolfcastleDirs ...string,
) ([]FixResult, *Report, error) {
	var repo *daemon.DaemonRepository
	if len(wolfcastleDirs) > 0 && wolfcastleDirs[0] != "" {
		repo = daemon.NewDaemonRepository(wolfcastleDirs[0])
	}
	return FixWithVerificationRepo(projectsDir, indexPath, loadNode, repo)
}

// FixWithVerificationRepo is like FixWithVerification but accepts a
// DaemonRepository directly. Pass nil to skip daemon artifact cleanup.
func FixWithVerificationRepo(
	projectsDir string,
	indexPath string,
	loadNode NodeLoader,
	repo *daemon.DaemonRepository,
) ([]FixResult, *Report, error) {
	var allFixes []FixResult

	for pass := 1; pass <= maxFixPasses; pass++ {
		// Reload index from disk (prior pass may have modified it).
		// Fall back to recovery if the index is malformed.
		idx, err := loadOrRecoverRootIndex(indexPath)
		if err != nil {
			return allFixes, nil, fmt.Errorf("loading root index on pass %d: %w", pass, err)
		}

		engine := NewEngineWithRepo(projectsDir, loadNode, repo)
		report := engine.ValidateAll(idx)

		if !report.HasAutoFixable() {
			return allFixes, report, nil
		}

		fixes, _, err := ApplyDeterministicFixesRepo(idx, report.Issues, projectsDir, indexPath, repo)
		if err != nil {
			return allFixes, report, fmt.Errorf("applying fixes on pass %d: %w", pass, err)
		}

		if len(fixes) == 0 {
			break
		}

		for i := range fixes {
			fixes[i].Pass = pass
		}
		allFixes = append(allFixes, fixes...)
	}

	// Final validation-only pass
	idx, err := loadOrRecoverRootIndex(indexPath)
	if err != nil {
		return allFixes, nil, fmt.Errorf("loading root index for final validation: %w", err)
	}
	engine := NewEngineWithRepo(projectsDir, loadNode, repo)
	finalReport := engine.ValidateAll(idx)

	return allFixes, finalReport, nil
}

// ApplyDeterministicFixes attempts to repair all deterministic-fixable issues.
// It stages changes in memory, writes leaf->parent->root, and re-validates.
// wolfcastleDir is optional; pass "" to skip daemon artifact cleanup.
// Returns the list of fixes applied, any post-fix re-validation warnings, and an error.
func ApplyDeterministicFixes(
	idx *state.RootIndex,
	issues []Issue,
	projectsDir string,
	indexPath string,
	wolfcastleDirs ...string,
) ([]FixResult, []Issue, error) {
	var repo *daemon.DaemonRepository
	if len(wolfcastleDirs) > 0 && wolfcastleDirs[0] != "" {
		repo = daemon.NewDaemonRepository(wolfcastleDirs[0])
	}
	return ApplyDeterministicFixesRepo(idx, issues, projectsDir, indexPath, repo)
}

// ApplyDeterministicFixesRepo is like ApplyDeterministicFixes but accepts
// a DaemonRepository directly. Pass nil to skip daemon artifact cleanup.
func ApplyDeterministicFixesRepo(
	idx *state.RootIndex,
	issues []Issue,
	projectsDir string,
	indexPath string,
	repo *daemon.DaemonRepository,
) ([]FixResult, []Issue, error) {
	var fixes []FixResult
	modifiedStates := map[string]*state.NodeState{}
	indexModified := false

	loadOrCached := func(addr string) (*state.NodeState, string, error) {
		a, err := tree.ParseAddress(addr)
		if err != nil {
			return nil, "", err
		}
		statePath := filepath.Join(projectsDir, filepath.Join(a.Parts...), "state.json")
		if cached, ok := modifiedStates[statePath]; ok {
			return cached, statePath, nil
		}
		ns, loadErr := state.LoadNodeState(statePath)
		if loadErr == nil {
			return ns, statePath, nil
		}
		// Fall back to recovery.
		data, readErr := os.ReadFile(statePath)
		if readErr != nil {
			return nil, "", loadErr
		}
		recovered, _, recoverErr := RecoverNodeState(data)
		if recoverErr != nil {
			return nil, "", loadErr
		}
		modifiedStates[statePath] = recovered
		return recovered, statePath, nil
	}

	for _, issue := range issues {
		if issue.FixType != FixDeterministic || !issue.CanAutoFix {
			continue
		}

		switch issue.Category {
		case CatMalformedJSON:
			if issue.Node == "" {
				continue // root index recovery is handled separately
			}
			ns, statePath, err := loadOrCached(issue.Node)
			if err != nil {
				continue
			}
			// Mark as modified so it gets written back as clean JSON.
			modifiedStates[statePath] = ns
			fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: "rewrote recovered node state as valid JSON"})

		case CatRootIndexDanglingRef:
			delete(idx.Nodes, issue.Node)
			// Remove from parent's children and root list
			for addr, entry := range idx.Nodes {
				for i, child := range entry.Children {
					if child == issue.Node {
						entry.Children = append(entry.Children[:i], entry.Children[i+1:]...)
						idx.Nodes[addr] = entry
						break
					}
				}
			}
			for i, r := range idx.Root {
				if r == issue.Node {
					idx.Root = append(idx.Root[:i], idx.Root[i+1:]...)
					break
				}
			}
			indexModified = true
			fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: "removed dangling entry from index"})

		case CatRootIndexMissingEntry:
			ns, statePath, err := loadOrCached(issue.Node)
			if err != nil {
				continue
			}
			a, aErr := tree.ParseAddress(issue.Node)
			if aErr != nil {
				continue
			}
			parentAddr := ""
			if len(a.Parts) > 1 {
				parentAddr = strings.Join(a.Parts[:len(a.Parts)-1], "/")
			}
			idx.Nodes[issue.Node] = state.IndexEntry{
				Name:               ns.Name,
				Type:               ns.Type,
				State:              ns.State,
				Address:            issue.Node,
				DecompositionDepth: ns.DecompositionDepth,
				Parent:             parentAddr,
			}
			if parentAddr != "" {
				if parentEntry, ok := idx.Nodes[parentAddr]; ok {
					parentEntry.Children = append(parentEntry.Children, issue.Node)
					idx.Nodes[parentAddr] = parentEntry
				}
			} else {
				idx.Root = append(idx.Root, issue.Node)
			}
			indexModified = true
			_ = statePath
			fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: "added orphaned node to index"})

		case CatPropagationMismatch:
			ns, statePath, err := loadOrCached(issue.Node)
			if err != nil {
				continue
			}
			if ns.Type == state.NodeOrchestrator && len(ns.Children) > 0 {
				ns.State = state.RecomputeState(ns.Children, ns.Tasks)
				modifiedStates[statePath] = ns
			}
			// Update index to match node state
			if entry, ok := idx.Nodes[issue.Node]; ok {
				entry.State = ns.State
				idx.Nodes[issue.Node] = entry
				indexModified = true
			}
			fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: fmt.Sprintf("updated state to %s", ns.State)})

		case CatMissingAuditTask:
			ns, statePath, err := loadOrCached(issue.Node)
			if err != nil {
				continue
			}
			ns.Tasks = append(ns.Tasks, state.Task{
				ID:          "audit",
				Description: "Audit task completion and verify acceptance criteria",
				State:       state.StatusNotStarted,
				IsAudit:     true,
			})
			modifiedStates[statePath] = ns
			fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: "added audit task"})

		case CatAuditNotLast:
			ns, statePath, err := loadOrCached(issue.Node)
			if err != nil {
				continue
			}
			var auditTask *state.Task
			var others []state.Task
			for i := range ns.Tasks {
				if ns.Tasks[i].IsAudit {
					t := ns.Tasks[i]
					auditTask = &t
				} else {
					others = append(others, ns.Tasks[i])
				}
			}
			if auditTask != nil {
				ns.Tasks = append(others, *auditTask)
				modifiedStates[statePath] = ns
			}
			fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: "moved audit task to end"})

		case CatInvalidStateValue:
			ns, statePath, err := loadOrCached(issue.Node)
			if err != nil {
				continue
			}
			normalized, ok := NormalizeStateValue(string(ns.State))
			if ok {
				ns.State = normalized
				modifiedStates[statePath] = ns
				if entry, ok := idx.Nodes[issue.Node]; ok {
					entry.State = normalized
					idx.Nodes[issue.Node] = entry
					indexModified = true
				}
				fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: fmt.Sprintf("normalized state to %s", normalized)})
			}

		case CatBlockedWithoutReason:
			ns, statePath, err := loadOrCached(issue.Node)
			if err != nil {
				continue
			}
			for i := range ns.Tasks {
				if ns.Tasks[i].State == state.StatusBlocked && ns.Tasks[i].BlockedReason == "" {
					ns.Tasks[i].BlockedReason = "no reason provided (auto-fixed by doctor)"
				}
			}
			modifiedStates[statePath] = ns
			fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: "added placeholder blocked reason"})

		case CatDepthMismatch:
			ns, statePath, err := loadOrCached(issue.Node)
			if err != nil {
				continue
			}
			if entry, ok := idx.Nodes[issue.Node]; ok && entry.Parent != "" {
				parentNS, _, parentErr := loadOrCached(entry.Parent)
				if parentErr == nil {
					ns.DecompositionDepth = parentNS.DecompositionDepth
					modifiedStates[statePath] = ns
					fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: fmt.Sprintf("set depth to %d", ns.DecompositionDepth)})
				}
			}

		case CatNegativeFailureCount:
			ns, statePath, err := loadOrCached(issue.Node)
			if err != nil {
				continue
			}
			for i := range ns.Tasks {
				if ns.Tasks[i].FailureCount < 0 {
					ns.Tasks[i].FailureCount = 0
				}
			}
			modifiedStates[statePath] = ns
			fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: "reset negative failure count to 0"})

		case CatMissingRequiredField:
			ns, statePath, err := loadOrCached(issue.Node)
			if err != nil {
				continue
			}
			if ns.ID == "" {
				ns.ID = issue.Node
			}
			if ns.Name == "" {
				ns.Name = issue.Node
			}
			if string(ns.Type) == "" {
				ns.Type = state.NodeLeaf
			}
			if string(ns.State) == "" {
				ns.State = state.StatusNotStarted
			}
			modifiedStates[statePath] = ns
			fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: "populated missing required fields"})

		case CatInvalidAuditStatus:
			ns, statePath, err := loadOrCached(issue.Node)
			if err != nil {
				continue
			}
			ns.Audit.Status = state.AuditPending
			modifiedStates[statePath] = ns
			fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: "reset audit status to pending"})

		case CatAuditStatusTaskMismatch:
			ns, statePath, err := loadOrCached(issue.Node)
			if err != nil {
				continue
			}
			state.SyncAuditLifecycle(ns)
			modifiedStates[statePath] = ns
			fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: fmt.Sprintf("synced audit status to %s", ns.Audit.Status)})

		case CatInvalidAuditGap:
			ns, statePath, err := loadOrCached(issue.Node)
			if err != nil {
				continue
			}
			// Clear stale fixed metadata on open gaps
			for i := range ns.Audit.Gaps {
				if ns.Audit.Gaps[i].Status == state.GapOpen {
					ns.Audit.Gaps[i].FixedBy = ""
					ns.Audit.Gaps[i].FixedAt = nil
				}
			}
			modifiedStates[statePath] = ns
			fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: "cleared stale gap metadata"})

		case CatOrphanDefinition:
			// Deterministic fix: no action needed (just a warning)
			fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: "orphan definition detected (no auto-fix)"})

		case CatStaleInProgress, CatMultipleInProgress:
			if issue.Node == "" {
				continue
			}
			ns, statePath, err := loadOrCached(issue.Node)
			if err != nil {
				continue
			}
			fixed := false
			for i := range ns.Tasks {
				if ns.Tasks[i].State == state.StatusInProgress {
					ns.Tasks[i].State = state.StatusNotStarted
					fixed = true
				}
			}
			if fixed {
				// Recompute node state from tasks
				allComplete := true
				for _, t := range ns.Tasks {
					if t.State != state.StatusComplete {
						allComplete = false
						break
					}
				}
				if allComplete && len(ns.Tasks) > 0 {
					ns.State = state.StatusComplete
				} else {
					ns.State = state.StatusNotStarted
				}
				modifiedStates[statePath] = ns
				if entry, ok := idx.Nodes[issue.Node]; ok {
					entry.State = ns.State
					idx.Nodes[issue.Node] = entry
					indexModified = true
				}
			}
			fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: "reset stale in_progress task(s) to not_started"})

		case CatChildRefStateMismatch:
			ns, statePath, err := loadOrCached(issue.Node)
			if err != nil {
				continue
			}
			for i := range ns.Children {
				// Resolve the child's actual state from the index (preferred) or disk.
				var actualState state.NodeStatus
				if entry, ok := idx.Nodes[ns.Children[i].Address]; ok {
					actualState = entry.State
				} else {
					childNS, _, childErr := loadOrCached(ns.Children[i].Address)
					if childErr != nil {
						continue
					}
					actualState = childNS.State
				}
				ns.Children[i].State = actualState
			}
			ns.State = state.RecomputeState(ns.Children, ns.Tasks)
			modifiedStates[statePath] = ns
			if entry, ok := idx.Nodes[issue.Node]; ok {
				entry.State = ns.State
				idx.Nodes[issue.Node] = entry
				indexModified = true
			}
			fixes = append(fixes, FixResult{Category: issue.Category, Node: issue.Node, Description: fmt.Sprintf("synced ChildRef states and recomputed parent to %s", ns.State)})

		case CatStalePIDFile:
			if repo != nil {
				if err := repo.RemovePID(); err == nil {
					fixes = append(fixes, FixResult{Category: issue.Category, Description: "removed stale PID file"})
				}
			}

		case CatStaleStopFile:
			if repo != nil {
				if err := repo.RemoveStopFile(); err == nil {
					fixes = append(fixes, FixResult{Category: issue.Category, Description: "removed stale stop file"})
				}
			}
		}
	}

	// Save modified state files
	for statePath, ns := range modifiedStates {
		if err := state.SaveNodeState(statePath, ns); err != nil {
			return fixes, nil, fmt.Errorf("saving %s: %w", statePath, err)
		}
	}

	// Save root index if modified
	if indexModified {
		if err := state.SaveRootIndex(indexPath, idx); err != nil {
			return fixes, nil, fmt.Errorf("saving root index: %w", err)
		}
	}

	// Post-fix re-validation: ensure fixes didn't introduce new issues.
	// Any remaining issues are returned as warnings so callers can surface them
	// without treating them as failures.
	var postFixWarnings []Issue
	if len(fixes) > 0 {
		engine := NewEngineWithRepo(projectsDir, DefaultNodeLoader(projectsDir), repo)
		postReport := engine.ValidateAll(idx)
		for _, issue := range postReport.Issues {
			issue.Severity = SeverityWarning
			issue.Description = "post-fix: " + issue.Description
			postFixWarnings = append(postFixWarnings, issue)
		}
	}

	return fixes, postFixWarnings, nil
}

// loadOrRecoverRootIndex loads a root index, falling back to JSON recovery
// if standard parsing fails.
func loadOrRecoverRootIndex(indexPath string) (*state.RootIndex, error) {
	idx, err := state.LoadRootIndex(indexPath)
	if err == nil {
		return idx, nil
	}

	data, readErr := os.ReadFile(indexPath)
	if readErr != nil {
		return nil, err // return original parse error
	}

	recovered, _, recoverErr := RecoverRootIndex(data)
	if recoverErr != nil {
		return nil, fmt.Errorf("%v (recovery also failed: %v)", err, recoverErr)
	}

	// Write the recovered index so subsequent passes work cleanly.
	if writeErr := state.SaveRootIndex(indexPath, recovered); writeErr != nil {
		return nil, fmt.Errorf("writing recovered index: %w", writeErr)
	}

	return recovered, nil
}

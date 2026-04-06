package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// validTaskIDRe matches expected task ID formats for validation.
var validTaskIDRe = regexp.MustCompile(`^(task-\d{4}|audit)(\.\d{4})*$`)

// RecoveredNode tracks a node that was recovered from malformed JSON,
// along with the steps taken and any data loss.
type RecoveredNode struct {
	Address string
	Report  *RecoveryReport
}

// NodeLoader loads a node's state given its tree address.
type NodeLoader func(addr string) (*state.NodeState, error)

// Engine runs structural validation checks against a project tree.
type Engine struct {
	projectsDir string
	daemonRepo  PIDChecker
	loadNode    NodeLoader
}

// NewEngine creates a validation engine. An optional PIDChecker enables
// daemon-aware stale detection; pass none (or nil) to skip it.
func NewEngine(projectsDir string, loadNode NodeLoader, checkers ...PIDChecker) *Engine {
	var checker PIDChecker
	if len(checkers) > 0 {
		checker = checkers[0]
	}
	return &Engine{
		projectsDir: projectsDir,
		daemonRepo:  checker,
		loadNode:    loadNode,
	}
}

// NewEngineWithRepo creates a validation engine using an existing
// PIDChecker. Pass nil to skip PID-aware stale detection.
func NewEngineWithRepo(projectsDir string, loadNode NodeLoader, checker PIDChecker) *Engine {
	return &Engine{
		projectsDir: projectsDir,
		daemonRepo:  checker,
		loadNode:    loadNode,
	}
}

// DefaultNodeLoader returns a NodeLoader that reads from disk.
func DefaultNodeLoader(projectsDir string) NodeLoader {
	return func(addr string) (*state.NodeState, error) {
		a, err := tree.ParseAddress(addr)
		if err != nil {
			return nil, err
		}
		return state.LoadNodeState(filepath.Join(projectsDir, filepath.Join(a.Parts...), "state.json"))
	}
}

// RecoveringNodeLoader returns a NodeLoader that falls back to JSON recovery
// when normal parsing fails. The onRecover callback is invoked for each
// recovered node so callers can track which nodes needed repair and what
// was lost. Pass nil for onRecover if you only care about the data.
func RecoveringNodeLoader(projectsDir string, onRecover func(addr string, report *RecoveryReport)) NodeLoader {
	return func(addr string) (*state.NodeState, error) {
		a, err := tree.ParseAddress(addr)
		if err != nil {
			return nil, err
		}
		statePath := filepath.Join(projectsDir, filepath.Join(a.Parts...), "state.json")

		// Try normal load first.
		ns, loadErr := state.LoadNodeState(statePath)
		if loadErr == nil {
			return ns, nil
		}

		// Normal load failed; attempt recovery from raw bytes.
		data, readErr := os.ReadFile(statePath)
		if readErr != nil {
			return nil, loadErr // return original error if we can't even read the file
		}

		recovered, report, recoverErr := RecoverNodeState(data)
		if recoverErr != nil {
			return nil, fmt.Errorf("%w (recovery also failed: %w)", loadErr, recoverErr)
		}

		if onRecover != nil {
			onRecover(addr, report)
		}
		return recovered, nil
	}
}

// ValidateAll runs all validation categories.
func (e *Engine) ValidateAll(idx *state.RootIndex) *Report {
	return e.validate(idx, nil)
}

// ValidateStartup runs only the startup subset of checks.
func (e *Engine) ValidateStartup(idx *state.RootIndex) *Report {
	return e.validate(idx, StartupCategories)
}

func (e *Engine) validate(idx *state.RootIndex, categories map[string]bool) *Report {
	report := &Report{}
	var inProgressTasks []string

	for addr, entry := range idx.Nodes {
		if _, parseErr := tree.ParseAddress(addr); parseErr != nil {
			continue
		}

		// Archived nodes have their state files moved to .archive/.
		// They're expected to be missing from the projects directory.
		if entry.Archived {
			continue
		}

		ns, err := e.loadNode(addr)
		if err != nil {
			if e.include(CatRootIndexDanglingRef, categories) {
				report.Issues = append(report.Issues, Issue{
					Severity:    SeverityError,
					Category:    CatRootIndexDanglingRef,
					Node:        addr,
					Description: fmt.Sprintf("Index references node but state file missing: %v", err),
					CanAutoFix:  true,
					FixType:     FixDeterministic,
				})
			}
			continue
		}

		e.checkNodeFields(ns, addr, categories, report)
		e.checkPropagation(ns, addr, entry, categories, report)

		if ns.Type == state.NodeLeaf {
			e.checkLeafAudit(ns, addr, categories, report)
		}
		e.checkTaskState(ns, addr, categories, report, &inProgressTasks)
		e.checkAuditState(ns, addr, categories, report)
		e.checkParentChild(ns, addr, entry, idx, categories, report)
		e.checkChildRefState(ns, addr, idx, categories, report)
		e.checkTransitions(ns, addr, categories, report)
	}

	e.checkGlobalState(idx, categories, report, inProgressTasks)

	report.Counts()
	return report
}

// checkNodeFields validates required fields and state values.
func (e *Engine) checkNodeFields(ns *state.NodeState, addr string, categories map[string]bool, report *Report) {
	if e.include(CatMissingRequiredField, categories) {
		if ns.ID == "" || ns.Name == "" || string(ns.Type) == "" || string(ns.State) == "" {
			report.Issues = append(report.Issues, Issue{
				Severity:    SeverityError,
				Category:    CatMissingRequiredField,
				Node:        addr,
				Description: "Missing required field(s) in state.json",
				CanAutoFix:  true,
				FixType:     FixDeterministic,
			})
		}
	}

	if e.include(CatInvalidStateValue, categories) {
		if !isValidState(ns.State) {
			_, normalizable := NormalizeStateValue(string(ns.State))
			fixType := FixModelAssisted
			if normalizable {
				fixType = FixDeterministic
			}
			report.Issues = append(report.Issues, Issue{
				Severity:    SeverityError,
				Category:    CatInvalidStateValue,
				Node:        addr,
				Description: fmt.Sprintf("Invalid state value: %q", ns.State),
				CanAutoFix:  normalizable,
				FixType:     fixType,
			})
		}
	}
}

// checkPropagation verifies index-node state consistency and orchestrator recomputation.
func (e *Engine) checkPropagation(ns *state.NodeState, addr string, entry state.IndexEntry, categories map[string]bool, report *Report) {
	if !e.include(CatPropagationMismatch, categories) {
		return
	}

	if ns.State != entry.State {
		report.Issues = append(report.Issues, Issue{
			Severity:    SeverityError,
			Category:    CatPropagationMismatch,
			Node:        addr,
			Description: fmt.Sprintf("Index says %s but node state says %s", entry.State, ns.State),
			CanAutoFix:  true,
			FixType:     FixDeterministic,
		})
	}

	if ns.Type == state.NodeOrchestrator && len(ns.Children) > 0 {
		expected := state.RecomputeState(ns.Children, ns.Tasks)
		if ns.State != expected {
			report.Issues = append(report.Issues, Issue{
				Severity:    SeverityError,
				Category:    CatPropagationMismatch,
				Node:        addr,
				Description: fmt.Sprintf("Computed state is %s but stored is %s", expected, ns.State),
				CanAutoFix:  true,
				FixType:     FixDeterministic,
			})
		}
	}
}

// checkParentChild validates parent-child relationships and depth consistency.
func (e *Engine) checkParentChild(ns *state.NodeState, addr string, entry state.IndexEntry, idx *state.RootIndex, categories map[string]bool, report *Report) {
	if entry.Parent == "" {
		return
	}

	if e.include(CatOrphanState, categories) {
		if parentEntry, ok := idx.Nodes[entry.Parent]; ok {
			found := false
			for _, child := range parentEntry.Children {
				if child == addr {
					found = true
					break
				}
			}
			if !found {
				report.Issues = append(report.Issues, Issue{
					Severity:    SeverityError,
					Category:    CatOrphanState,
					Node:        addr,
					Description: fmt.Sprintf("Node has parent %s but parent does not list it as child", entry.Parent),
					FixType:     FixModelAssisted,
				})
			}
		}
	}

	if e.include(CatDepthMismatch, categories) {
		parentNS, parentErr := e.loadNode(entry.Parent)
		if parentErr == nil && ns.DecompositionDepth < parentNS.DecompositionDepth {
			report.Issues = append(report.Issues, Issue{
				Severity:    SeverityError,
				Category:    CatDepthMismatch,
				Node:        addr,
				Description: fmt.Sprintf("Child depth %d < parent depth %d", ns.DecompositionDepth, parentNS.DecompositionDepth),
				CanAutoFix:  true,
				FixType:     FixDeterministic,
			})
		}
	}
}

// checkChildRefState verifies that each orchestrator's ChildRef.State matches
// the child's actual state from the index (or disk). A mismatch means the
// parent is carrying a stale snapshot of the child's lifecycle.
func (e *Engine) checkChildRefState(ns *state.NodeState, addr string, idx *state.RootIndex, categories map[string]bool, report *Report) {
	if !e.include(CatChildRefStateMismatch, categories) {
		return
	}
	if ns.Type != state.NodeOrchestrator {
		return
	}

	for _, child := range ns.Children {
		// Prefer the index entry as the source of truth; fall back to loading from disk.
		var actualState state.NodeStatus
		if entry, ok := idx.Nodes[child.Address]; ok {
			actualState = entry.State
		} else {
			childNS, err := e.loadNode(child.Address)
			if err != nil {
				continue // dangling ref is caught by a different category
			}
			actualState = childNS.State
		}

		if child.State != actualState {
			report.Issues = append(report.Issues, Issue{
				Severity:    SeverityError,
				Category:    CatChildRefStateMismatch,
				Node:        addr,
				Description: fmt.Sprintf("ChildRef %s has state %s but actual child state is %s", child.Address, child.State, actualState),
				CanAutoFix:  true,
				FixType:     FixDeterministic,
			})
		}
	}
}

// checkTransitions validates state transition invariants.
func (e *Engine) checkTransitions(ns *state.NodeState, addr string, categories map[string]bool, report *Report) {
	if e.include(CatCompleteWithIncomplete, categories) {
		if ns.Type == state.NodeLeaf && ns.State == state.StatusComplete {
			for _, t := range ns.Tasks {
				if t.State != state.StatusComplete {
					report.Issues = append(report.Issues, Issue{
						Severity:    SeverityError,
						Category:    CatCompleteWithIncomplete,
						Node:        addr,
						Description: "Leaf is complete but has incomplete tasks",
						FixType:     FixModelAssisted,
					})
					break
				}
			}
		}
		if ns.Type == state.NodeOrchestrator && ns.State == state.StatusComplete {
			for _, c := range ns.Children {
				if c.State != state.StatusComplete {
					report.Issues = append(report.Issues, Issue{
						Severity:    SeverityError,
						Category:    CatCompleteWithIncomplete,
						Node:        addr,
						Description: "Orchestrator is complete but has incomplete children",
						FixType:     FixModelAssisted,
					})
					break
				}
			}
		}
	}

	if e.include(CatBlockedWithoutReason, categories) {
		for _, t := range ns.Tasks {
			if t.State == state.StatusBlocked && t.BlockedReason == "" {
				report.Issues = append(report.Issues, Issue{
					Severity:    SeverityError,
					Category:    CatBlockedWithoutReason,
					Node:        addr,
					Description: fmt.Sprintf("Task %s is blocked without a reason", t.ID),
					CanAutoFix:  true,
					FixType:     FixDeterministic,
				})
			}
		}
	}
}

// checkGlobalState runs cross-node checks: orphans, in-progress invariants, daemon artifacts.
func (e *Engine) checkGlobalState(idx *state.RootIndex, categories map[string]bool, report *Report, inProgressTasks []string) {
	if e.include(CatRootIndexMissingEntry, categories) {
		e.checkOrphanedStateFiles(idx, report)
	}
	if e.include(CatOrphanDefinition, categories) {
		e.checkOrphanedDefinitions(idx, report)
	}

	if e.include(CatMultipleInProgress, categories) && len(inProgressTasks) > 1 {
		for _, taskRef := range inProgressTasks {
			parts := strings.SplitN(taskRef, "/", 2)
			node := ""
			if len(parts) > 0 {
				// Extract the node address (everything except the last /taskID segment)
				node = nodeAddrFromTaskRef(taskRef)
			}
			report.Issues = append(report.Issues, Issue{
				Severity:    SeverityError,
				Category:    CatMultipleInProgress,
				Node:        node,
				Description: fmt.Sprintf("Task %s in progress (serial execution allows at most 1)", taskRef),
				CanAutoFix:  true,
				FixType:     FixDeterministic,
			})
		}
	}
	if e.include(CatStaleInProgress, categories) && len(inProgressTasks) > 0 {
		if !e.isDaemonAlive() {
			for _, taskRef := range inProgressTasks {
				node := nodeAddrFromTaskRef(taskRef)
				report.Issues = append(report.Issues, Issue{
					Severity:    SeverityError,
					Category:    CatStaleInProgress,
					Node:        node,
					Description: fmt.Sprintf("Task %s in progress with no live daemon", taskRef),
					CanAutoFix:  true,
					FixType:     FixDeterministic,
				})
			}
		}
	}

	if e.include(CatStalePIDFile, categories) {
		if e.daemonRepo != nil && !e.isDaemonAlive() {
			if e.daemonRepo.PIDFileExists() {
				report.Issues = append(report.Issues, Issue{
					Severity:    SeverityWarning,
					Category:    CatStalePIDFile,
					Description: "PID file exists but daemon process is not alive",
					CanAutoFix:  true,
					FixType:     FixDeterministic,
				})
			}
		}
	}
	if e.include(CatStaleStopFile, categories) {
		if e.daemonRepo != nil && !e.isDaemonAlive() {
			if e.daemonRepo.StopFileExists() {
				report.Issues = append(report.Issues, Issue{
					Severity:    SeverityWarning,
					Category:    CatStaleStopFile,
					Description: "Stop file exists but no daemon is running. Would block next start",
					CanAutoFix:  true,
					FixType:     FixDeterministic,
				})
			}
		}
	}

	if e.include(CatOrphanedTempFile, categories) {
		e.checkOrphanedTempFiles(report)
	}
}

func (e *Engine) checkLeafAudit(ns *state.NodeState, addr string, categories map[string]bool, report *Report) {
	auditCount := 0
	auditLast := false

	for i, t := range ns.Tasks {
		if t.IsAudit {
			auditCount++
			auditLast = i == len(ns.Tasks)-1
		}
	}

	if e.include(CatMissingAuditTask, categories) && auditCount == 0 {
		report.Issues = append(report.Issues, Issue{
			Severity:    SeverityError,
			Category:    CatMissingAuditTask,
			Node:        addr,
			Description: "Leaf node has no audit task",
			CanAutoFix:  true,
			FixType:     FixDeterministic,
		})
	}

	if e.include(CatAuditNotLast, categories) && auditCount == 1 && !auditLast {
		report.Issues = append(report.Issues, Issue{
			Severity:    SeverityError,
			Category:    CatAuditNotLast,
			Node:        addr,
			Description: "Audit task is not the last task",
			CanAutoFix:  true,
			FixType:     FixDeterministic,
		})
	}

	if e.include(CatMultipleAuditTasks, categories) && auditCount > 1 {
		report.Issues = append(report.Issues, Issue{
			Severity:    SeverityError,
			Category:    CatMultipleAuditTasks,
			Node:        addr,
			Description: fmt.Sprintf("Leaf has %d audit tasks, expected 1", auditCount),
			FixType:     FixModelAssisted,
		})
	}
}

func (e *Engine) checkTaskState(ns *state.NodeState, addr string, categories map[string]bool, report *Report, inProgressTasks *[]string) {
	for _, t := range ns.Tasks {
		// INVALID_TASK_ID
		if e.include(CatInvalidTaskID, categories) && !validTaskIDRe.MatchString(t.ID) {
			report.Issues = append(report.Issues, Issue{
				Severity:    SeverityWarning,
				Category:    CatInvalidTaskID,
				Node:        addr,
				Description: fmt.Sprintf("Task %q does not match expected format (task-NNNN or audit, with optional .NNNN suffixes)", t.ID),
				CanAutoFix:  false,
				FixType:     FixManual,
			})
		}

		// NEGATIVE_FAILURE_COUNT
		if e.include(CatNegativeFailureCount, categories) && t.FailureCount < 0 {
			report.Issues = append(report.Issues, Issue{
				Severity:    SeverityError,
				Category:    CatNegativeFailureCount,
				Node:        addr,
				Description: fmt.Sprintf("Task %s has negative failure count: %d", t.ID, t.FailureCount),
				CanAutoFix:  true,
				FixType:     FixDeterministic,
			})
		}

		// Track in-progress tasks globally
		if t.State == state.StatusInProgress {
			*inProgressTasks = append(*inProgressTasks, addr+"/"+t.ID)
		}
	}
}

func (e *Engine) checkAuditState(ns *state.NodeState, addr string, categories map[string]bool, report *Report) {
	// INVALID_AUDIT_SCOPE: scope with a non-empty description that contradicts
	// the node's actual content would be a real issue, but an empty description
	// is the default state created by NewNodeState. The audit task populates it
	// when it runs. Flagging empty descriptions generates noise for every node
	// in the tree, so we only check for structurally invalid scopes: a scope
	// that has criteria or systems defined but is missing its description
	// after the audit has completed (passed or failed).
	if e.include(CatInvalidAuditScope, categories) && ns.Audit.Scope != nil {
		hasContent := len(ns.Audit.Scope.Criteria) > 0 || len(ns.Audit.Scope.Systems) > 0 || len(ns.Audit.Scope.Files) > 0
		auditDone := ns.Audit.Status == state.AuditPassed || ns.Audit.Status == state.AuditFailed
		if ns.Audit.Scope.Description == "" && hasContent && auditDone {
			report.Issues = append(report.Issues, Issue{
				Severity:    SeverityWarning,
				Category:    CatInvalidAuditScope,
				Node:        addr,
				Description: "Audit scope has criteria/files but no description",
				FixType:     FixManual,
			})
		}
	}

	// INVALID_AUDIT_STATUS: audit status must be one of the valid values
	if e.include(CatInvalidAuditStatus, categories) {
		switch ns.Audit.Status {
		case state.AuditPending, state.AuditInProgress, state.AuditPassed, state.AuditFailed:
			// valid
		default:
			report.Issues = append(report.Issues, Issue{
				Severity:    SeverityError,
				Category:    CatInvalidAuditStatus,
				Node:        addr,
				Description: fmt.Sprintf("Invalid audit status: %q", ns.Audit.Status),
				CanAutoFix:  true,
				FixType:     FixDeterministic,
			})
		}
	}

	// AUDIT_STATUS_TASK_MISMATCH: verify audit status is consistent with task state
	if e.include(CatAuditStatusTaskMismatch, categories) {
		expected := expectedAuditStatus(ns)
		if expected != "" && ns.Audit.Status != expected {
			report.Issues = append(report.Issues, Issue{
				Severity:    SeverityWarning,
				Category:    CatAuditStatusTaskMismatch,
				Node:        addr,
				Description: fmt.Sprintf("Audit status is %q but expected %q based on task state %s", ns.Audit.Status, expected, ns.State),
				CanAutoFix:  true,
				FixType:     FixDeterministic,
			})
		}
	}

	// INVALID_AUDIT_GAP: gaps must have ID, description, and valid status
	if e.include(CatInvalidAuditGap, categories) {
		for _, g := range ns.Audit.Gaps {
			if g.ID == "" || g.Description == "" {
				report.Issues = append(report.Issues, Issue{
					Severity:    SeverityError,
					Category:    CatInvalidAuditGap,
					Node:        addr,
					Description: fmt.Sprintf("Gap missing ID or description: %q", g.ID),
					FixType:     FixManual,
				})
			}
			if g.Status != state.GapOpen && g.Status != state.GapFixed {
				report.Issues = append(report.Issues, Issue{
					Severity:    SeverityError,
					Category:    CatInvalidAuditGap,
					Node:        addr,
					Description: fmt.Sprintf("Gap %s has invalid status: %q", g.ID, g.Status),
					CanAutoFix:  true,
					FixType:     FixDeterministic,
				})
			}
			// Stale fixed metadata on open gaps
			if g.Status == state.GapOpen && g.FixedBy != "" {
				report.Issues = append(report.Issues, Issue{
					Severity:    SeverityWarning,
					Category:    CatInvalidAuditGap,
					Node:        addr,
					Description: fmt.Sprintf("Gap %s is open but has stale fixed_by metadata", g.ID),
					CanAutoFix:  true,
					FixType:     FixDeterministic,
				})
			}
		}
	}

	// INVALID_AUDIT_ESCALATION: escalations must have required fields
	if e.include(CatInvalidAuditEscalation, categories) {
		for _, esc := range ns.Audit.Escalations {
			if esc.ID == "" || esc.Description == "" || esc.SourceNode == "" {
				report.Issues = append(report.Issues, Issue{
					Severity:    SeverityError,
					Category:    CatInvalidAuditEscalation,
					Node:        addr,
					Description: fmt.Sprintf("Escalation missing required field(s): %q", esc.ID),
					FixType:     FixManual,
				})
			}
		}
	}
}

func expectedAuditStatus(ns *state.NodeState) state.AuditStatus {
	switch ns.State {
	case state.StatusNotStarted:
		return state.AuditPending
	case state.StatusInProgress:
		return state.AuditInProgress
	case state.StatusBlocked:
		return state.AuditFailed
	case state.StatusComplete:
		for _, g := range ns.Audit.Gaps {
			if g.Status == state.GapOpen {
				return state.AuditFailed
			}
		}
		return state.AuditPassed
	}
	return ""
}

func (e *Engine) checkOrphanedStateFiles(idx *state.RootIndex, report *Report) {
	_ = filepath.Walk(e.projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip the .archive/ directory; archived state files are
		// expected to live outside the active project tree.
		if info.IsDir() && info.Name() == ".archive" {
			return filepath.SkipDir
		}
		if info.Name() != "state.json" || path == filepath.Join(e.projectsDir, "state.json") {
			return nil
		}
		rel, err := filepath.Rel(e.projectsDir, filepath.Dir(path))
		if err != nil {
			return nil
		}
		addr := filepath.ToSlash(rel)
		if _, ok := idx.Nodes[addr]; !ok {
			report.Issues = append(report.Issues, Issue{
				Severity:    SeverityError,
				Category:    CatRootIndexMissingEntry,
				Node:        addr,
				Description: "State file exists on disk but node not in index",
				CanAutoFix:  true,
				FixType:     FixDeterministic,
			})
		}
		return nil
	})
}

func (e *Engine) checkOrphanedDefinitions(idx *state.RootIndex, report *Report) {
	_ = filepath.Walk(e.projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && info.Name() == ".archive" {
			return filepath.SkipDir
		}
		if !strings.HasSuffix(info.Name(), ".md") || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(e.projectsDir, filepath.Dir(path))
		if err != nil {
			return nil
		}
		addr := filepath.ToSlash(rel)
		if addr == "." {
			return nil
		}
		if _, ok := idx.Nodes[addr]; !ok {
			report.Issues = append(report.Issues, Issue{
				Severity:    SeverityWarning,
				Category:    CatOrphanDefinition,
				Node:        addr,
				Description: fmt.Sprintf("Definition file %s has no corresponding node", info.Name()),
				CanAutoFix:  false,
				FixType:     FixManual,
			})
		}
		return nil
	})
}

func (e *Engine) include(category string, filter map[string]bool) bool {
	if filter == nil {
		return true
	}
	return filter[category]
}

// isDaemonAlive checks if a daemon PID file exists and the process is alive.
func (e *Engine) isDaemonAlive() bool {
	if e.daemonRepo == nil {
		return false
	}
	return e.daemonRepo.IsAlive()
}

// nodeAddrFromTaskRef extracts the node address from a "node/taskID" reference.
// For hierarchical tasks like "domain-repo/task-0001.0002", the node address
// is everything before the last "/" segment.
func nodeAddrFromTaskRef(ref string) string {
	idx := strings.LastIndex(ref, "/")
	if idx < 0 {
		return ref
	}
	return ref[:idx]
}

func (e *Engine) checkOrphanedTempFiles(report *Report) {
	_ = filepath.Walk(e.projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasPrefix(info.Name(), ".wolfcastle-tmp-") {
			report.Issues = append(report.Issues, Issue{
				Severity:    SeverityWarning,
				Category:    CatOrphanedTempFile,
				Description: fmt.Sprintf("Orphaned temp file: %s", path),
				CanAutoFix:  true,
				FixType:     FixDeterministic,
			})
		}
		return nil
	})
}

func isValidState(s state.NodeStatus) bool {
	switch s {
	case state.StatusNotStarted, state.StatusInProgress, state.StatusComplete, state.StatusBlocked:
		return true
	default:
		return false
	}
}

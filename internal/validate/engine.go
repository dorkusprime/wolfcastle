package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// NodeLoader loads a node's state given its tree address.
type NodeLoader func(addr string) (*state.NodeState, error)

// Engine runs structural validation checks against a project tree.
type Engine struct {
	projectsDir   string
	wolfcastleDir string
	loadNode      NodeLoader
}

// NewEngine creates a validation engine.
// wolfcastleDir is optional — pass "" to skip PID-aware stale detection.
func NewEngine(projectsDir string, loadNode NodeLoader, wolfcastleDirs ...string) *Engine {
	var wolfcastleDir string
	if len(wolfcastleDirs) > 0 {
		wolfcastleDir = wolfcastleDirs[0]
	}
	return &Engine{
		projectsDir:   projectsDir,
		wolfcastleDir: wolfcastleDir,
		loadNode:      loadNode,
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

// ValidateAll runs all 17 validation categories.
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

	// Per-node checks
	for addr, entry := range idx.Nodes {
		_, parseErr := tree.ParseAddress(addr)
		if parseErr != nil {
			continue
		}

		ns, err := e.loadNode(addr)
		if err != nil {
			// ROOTINDEX_DANGLING_REF: index references non-existent node
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

		// MALFORMED_JSON is implicitly checked above (LoadNodeState fails)

		// MISSING_REQUIRED_FIELD
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

		// INVALID_STATE_VALUE
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

		// State mismatch (index vs node) — reported as propagation issue
		if e.include(CatPropagationMismatch, categories) {
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

			// Orchestrator state propagation check
			if ns.Type == state.NodeOrchestrator && len(ns.Children) > 0 {
				expected := state.RecomputeState(ns.Children)
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

		// Leaf-specific checks
		if ns.Type == state.NodeLeaf {
			e.checkLeafAudit(ns, addr, categories, report)
			e.checkLeafTasks(ns, addr, categories, report, &inProgressTasks)
		}

		// Audit state checks apply to both leaf and orchestrator nodes
		e.checkAuditState(ns, addr, categories, report)

		// ORPHAN_STATE: node has a parent but parent doesn't list it as child
		if e.include(CatOrphanState, categories) && entry.Parent != "" {
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

		// DEPTH_MISMATCH: child depth must be >= parent depth
		if e.include(CatDepthMismatch, categories) && entry.Parent != "" {
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

		// INVALID_TRANSITION_COMPLETE_WITH_INCOMPLETE
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

		// INVALID_TRANSITION_BLOCKED_WITHOUT_REASON
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

	// ROOTINDEX_MISSING_ENTRY: node on disk but not in index
	if e.include(CatRootIndexMissingEntry, categories) {
		e.checkOrphanedStateFiles(idx, report)
	}

	// ORPHAN_DEFINITION: .md files without corresponding nodes
	if e.include(CatOrphanDefinition, categories) {
		e.checkOrphanedDefinitions(idx, report)
	}

	// Global in-progress checks
	if e.include(CatMultipleInProgress, categories) && len(inProgressTasks) > 1 {
		report.Issues = append(report.Issues, Issue{
			Severity:    SeverityError,
			Category:    CatMultipleInProgress,
			Description: fmt.Sprintf("Multiple tasks in progress: %s", strings.Join(inProgressTasks, ", ")),
			FixType:     FixModelAssisted,
		})
	}
	if e.include(CatStaleInProgress, categories) && len(inProgressTasks) > 0 {
		// Flag as stale if no live daemon is running for this workspace
		if !e.isDaemonAlive() {
			report.Issues = append(report.Issues, Issue{
				Severity:    SeverityWarning,
				Category:    CatStaleInProgress,
				Description: fmt.Sprintf("Task(s) in progress (%s) with no live daemon, likely stale", strings.Join(inProgressTasks, ", ")),
				FixType:     FixNone,
			})
		}
	}

	// Daemon artifact checks
	if e.include(CatStalePIDFile, categories) {
		if e.wolfcastleDir != "" && !e.isDaemonAlive() {
			pidPath := filepath.Join(e.wolfcastleDir, "wolfcastle.pid")
			if _, err := os.Stat(pidPath); err == nil {
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
		if e.wolfcastleDir != "" && !e.isDaemonAlive() {
			stopPath := filepath.Join(e.wolfcastleDir, "stop")
			if _, err := os.Stat(stopPath); err == nil {
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

	report.Counts()
	return report
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

func (e *Engine) checkLeafTasks(ns *state.NodeState, addr string, categories map[string]bool, report *Report, inProgressTasks *[]string) {
	for _, t := range ns.Tasks {
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
	// INVALID_AUDIT_SCOPE: audit scope present but missing required description
	if e.include(CatInvalidAuditScope, categories) && ns.Audit.Scope != nil {
		if ns.Audit.Scope.Description == "" {
			report.Issues = append(report.Issues, Issue{
				Severity:    SeverityWarning,
				Category:    CatInvalidAuditScope,
				Node:        addr,
				Description: "Audit scope exists but has no description",
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
	if e.wolfcastleDir == "" {
		return false
	}
	pidPath := filepath.Join(e.wolfcastleDir, "wolfcastle.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks if process exists without actually signaling it
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func isValidState(s state.NodeStatus) bool {
	switch s {
	case state.StatusNotStarted, state.StatusInProgress, state.StatusComplete, state.StatusBlocked:
		return true
	default:
		return false
	}
}

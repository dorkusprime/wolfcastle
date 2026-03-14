package daemon

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// MarkerCallbacks holds optional callback functions invoked when
// WOLFCASTLE_* markers are found in model output. A nil callback
// means the corresponding marker type is silently ignored.
type MarkerCallbacks struct {
	OnBreadcrumb    func(text string)
	OnGap           func(description string)
	OnFixGap        func(gapID string)
	OnScope         func(description string)
	OnScopeFiles    func(raw string)
	OnScopeSystems  func(raw string)
	OnScopeCriteria func(raw string)
	OnSummary       func(text string)
	OnResolveEsc    func(escalationID string)
	OnComplete      func()
	OnBlocked       func(reason string)
	OnYield         func()
}

// ParseMarkers scans model output line-by-line for WOLFCASTLE_* markers
// and invokes the corresponding callback for each one found. Callbacks
// that are nil are skipped — markers without handlers are silently ignored.
func ParseMarkers(output string, cb MarkerCallbacks) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "WOLFCASTLE_BREADCRUMB:"):
			if cb.OnBreadcrumb != nil {
				text := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_BREADCRUMB:"))
				if text != "" {
					cb.OnBreadcrumb(text)
				}
			}

		case strings.HasPrefix(line, "WOLFCASTLE_GAP:"):
			if cb.OnGap != nil {
				desc := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_GAP:"))
				if desc != "" {
					cb.OnGap(desc)
				}
			}

		case strings.HasPrefix(line, "WOLFCASTLE_FIX_GAP:"):
			if cb.OnFixGap != nil {
				gapID := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_FIX_GAP:"))
				if gapID != "" {
					cb.OnFixGap(gapID)
				}
			}

		case strings.HasPrefix(line, "WOLFCASTLE_SCOPE_FILES:"):
			if cb.OnScopeFiles != nil {
				raw := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_SCOPE_FILES:"))
				cb.OnScopeFiles(raw)
			}

		case strings.HasPrefix(line, "WOLFCASTLE_SCOPE_SYSTEMS:"):
			if cb.OnScopeSystems != nil {
				raw := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_SCOPE_SYSTEMS:"))
				cb.OnScopeSystems(raw)
			}

		case strings.HasPrefix(line, "WOLFCASTLE_SCOPE_CRITERIA:"):
			if cb.OnScopeCriteria != nil {
				raw := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_SCOPE_CRITERIA:"))
				cb.OnScopeCriteria(raw)
			}

		case strings.HasPrefix(line, "WOLFCASTLE_SCOPE:"):
			if cb.OnScope != nil {
				desc := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_SCOPE:"))
				if desc != "" {
					cb.OnScope(desc)
				}
			}

		case strings.HasPrefix(line, "WOLFCASTLE_SUMMARY:"):
			if cb.OnSummary != nil {
				text := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_SUMMARY:"))
				if text != "" {
					cb.OnSummary(text)
				}
			}

		case strings.HasPrefix(line, "WOLFCASTLE_RESOLVE_ESCALATION:"):
			if cb.OnResolveEsc != nil {
				escID := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_RESOLVE_ESCALATION:"))
				if escID != "" {
					cb.OnResolveEsc(escID)
				}
			}

		case strings.Contains(line, "WOLFCASTLE_COMPLETE"):
			if cb.OnComplete != nil {
				cb.OnComplete()
			}

		case strings.Contains(line, "WOLFCASTLE_BLOCKED"):
			if cb.OnBlocked != nil {
				reason := ""
				if idx := strings.Index(line, "WOLFCASTLE_BLOCKED:"); idx >= 0 {
					reason = strings.TrimSpace(line[idx+len("WOLFCASTLE_BLOCKED:"):])
				}
				cb.OnBlocked(reason)
			}

		case strings.Contains(line, "WOLFCASTLE_YIELD"):
			if cb.OnYield != nil {
				cb.OnYield()
			}
		}
	}
}

// applyModelMarkers parses WOLFCASTLE_* mutation markers from model output
// using the callback-based ParseMarkers function.
func (d *Daemon) applyModelMarkers(modelOutput string, ns *state.NodeState, nav *state.NavigationResult) {
	ParseMarkers(modelOutput, MarkerCallbacks{
		OnBreadcrumb: func(text string) {
			state.AddBreadcrumb(ns, nav.NodeAddress+"/"+nav.TaskID, text, d.Clock)
			_ = d.Logger.Log(map[string]any{"type": "marker_breadcrumb", "text": text})
		},
		OnGap: func(desc string) {
			gapID := fmt.Sprintf("gap-%s-%d", ns.ID, len(ns.Audit.Gaps)+1)
			ns.Audit.Gaps = append(ns.Audit.Gaps, state.Gap{
				ID:          gapID,
				Timestamp:   d.Clock.Now(),
				Description: desc,
				Source:      nav.NodeAddress,
				Status:      state.GapOpen,
			})
			_ = d.Logger.Log(map[string]any{"type": "marker_gap", "gap_id": gapID})
		},
		OnFixGap: func(gapID string) {
			for i := range ns.Audit.Gaps {
				if ns.Audit.Gaps[i].ID == gapID && ns.Audit.Gaps[i].Status == state.GapOpen {
					ns.Audit.Gaps[i].Status = state.GapFixed
					ns.Audit.Gaps[i].FixedBy = nav.NodeAddress + "/" + nav.TaskID
					now := d.Clock.Now()
					ns.Audit.Gaps[i].FixedAt = &now
					_ = d.Logger.Log(map[string]any{"type": "marker_fix_gap", "gap_id": gapID})
					break
				}
			}
		},
		OnScope: func(desc string) {
			if ns.Audit.Scope == nil {
				ns.Audit.Scope = &state.AuditScope{}
			}
			ns.Audit.Scope.Description = desc
			_ = d.Logger.Log(map[string]any{"type": "marker_scope", "description": desc})
		},
		OnScopeFiles: func(raw string) {
			if ns.Audit.Scope == nil {
				ns.Audit.Scope = &state.AuditScope{}
			}
			ns.Audit.Scope.Files = dedupPipe(raw)
		},
		OnScopeSystems: func(raw string) {
			if ns.Audit.Scope == nil {
				ns.Audit.Scope = &state.AuditScope{}
			}
			ns.Audit.Scope.Systems = dedupPipe(raw)
		},
		OnScopeCriteria: func(raw string) {
			if ns.Audit.Scope == nil {
				ns.Audit.Scope = &state.AuditScope{}
			}
			ns.Audit.Scope.Criteria = dedupPipe(raw)
		},
		OnSummary: func(text string) {
			ns.Audit.ResultSummary = text
			_ = d.Logger.Log(map[string]any{"type": "marker_summary", "text": text})
		},
		OnResolveEsc: func(escID string) {
			for i := range ns.Audit.Escalations {
				if ns.Audit.Escalations[i].ID == escID && ns.Audit.Escalations[i].Status == state.EscalationOpen {
					ns.Audit.Escalations[i].Status = state.EscalationResolved
					ns.Audit.Escalations[i].ResolvedBy = nav.NodeAddress + "/" + nav.TaskID
					now := d.Clock.Now()
					ns.Audit.Escalations[i].ResolvedAt = &now
					_ = d.Logger.Log(map[string]any{"type": "marker_resolve_escalation", "escalation_id": escID})
					break
				}
			}
		},
	})
}

// dedupPipe splits a pipe-delimited string and deduplicates entries.
func dedupPipe(s string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, part := range strings.Split(s, "|") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			result = append(result, trimmed)
		}
	}
	return result
}

package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

// newCodebaseCmd returns both the "run" and "list" subcommands for
// codebase auditing.
func newCodebaseCmd(app *cmdutil.App) (*cobra.Command, *cobra.Command) {
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Execute a codebase audit",
		Long: `Runs a model-driven codebase audit. Findings land in a pending batch.
Review them with 'audit pending', approve or reject individually.

Scopes come from base/audits/, custom/audits/, and local/audits/.
Use --list to see what's available.

Examples:
  wolfcastle audit run
  wolfcastle audit run --scope security,performance
  wolfcastle audit run --list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			scopeFlag, _ := cmd.Flags().GetString("scope")
			listFlag, _ := cmd.Flags().GetBool("list")

			// Discover available scopes
			scopes, err := discoverScopes(app)
			if err != nil {
				return err
			}

			if listFlag {
				if app.JSONOutput {
					output.Print(output.Ok("audit_list", map[string]any{
						"scopes": scopes,
					}))
				} else {
					output.PrintHuman("Available audit scopes:")
					for _, s := range scopes {
						output.PrintHuman("  %-20s %s", s.ID, s.Description)
					}
				}
				return nil
			}

			// Check for existing pending batch
			batchPath := filepath.Join(app.Config.Root(), "audit-state.json")
			existing, err := state.LoadBatch(batchPath)
			if err != nil {
				return err
			}
			if existing != nil && existing.Status == state.BatchPending {
				pendingCount := 0
				for _, f := range existing.Findings {
					if f.Status == state.FindingPending {
						pendingCount++
					}
				}
				return fmt.Errorf("pending batch exists (%d finding(s)). Review with 'audit pending' or discard with 'audit reject --all'", pendingCount)
			}

			// Filter scopes
			var selectedScopes []auditScope
			if scopeFlag != "" {
				requested := strings.Split(scopeFlag, ",")
				scopeMap := make(map[string]auditScope)
				for _, s := range scopes {
					scopeMap[s.ID] = s
				}
				for _, r := range requested {
					r = strings.TrimSpace(r)
					s, ok := scopeMap[r]
					if !ok {
						return fmt.Errorf("unknown scope %q. Use --list to see available scopes", r)
					}
					selectedScopes = append(selectedScopes, s)
				}
			} else {
				selectedScopes = scopes
			}

			if len(selectedScopes) == 0 {
				return fmt.Errorf("no audit scopes found. Add .md files to base/audits/, custom/audits/, or local/audits/")
			}

			return runCodebaseAudit(cmd.Context(), app, selectedScopes)
		},
	}

	runCmd.Flags().String("scope", "", "Comma-separated scope IDs to run")
	runCmd.Flags().Bool("list", false, "List available scopes")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Show available audit scopes",
		Long: `Lists audit scopes from base/audits/, custom/audits/, and local/audits/.

Examples:
  wolfcastle audit list
  wolfcastle audit list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			scopes, err := discoverScopes(app)
			if err != nil {
				return err
			}
			if app.JSONOutput {
				output.Print(output.Ok("audit_list", map[string]any{
					"scopes": scopes,
				}))
			} else {
				if len(scopes) == 0 {
					output.PrintHuman("No audit scopes found. Add .md files to base/audits/.")
				} else {
					output.PrintHuman("Available audit scopes:")
					for _, s := range scopes {
						output.PrintHuman("  %-20s %s", s.ID, s.Description)
					}
				}
			}
			return nil
		},
	}

	return runCmd, listCmd
}

type auditScope struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	PromptFile  string `json:"prompt_file"`
}

func discoverScopes(app *cmdutil.App) ([]auditScope, error) {
	var scopes []auditScope
	seen := make(map[string]bool)

	// Scan tiers in reverse priority (base first, local last overwrites)
	for _, tier := range pipeline.Tiers {
		dir := filepath.Join(app.Config.Root(), tier, "audits")
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			id := strings.TrimSuffix(e.Name(), ".md")
			if seen[id] {
				// Higher tier replaces
				for i, s := range scopes {
					if s.ID == id {
						scopes[i].PromptFile = filepath.Join(dir, e.Name())
						break
					}
				}
				continue
			}
			seen[id] = true

			// Read first line for description
			desc := id
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err == nil {
				lines := strings.SplitN(string(data), "\n", 3)
				for _, l := range lines {
					l = strings.TrimSpace(l)
					if l != "" && !strings.HasPrefix(l, "#") {
						desc = l
						break
					}
				}
			}

			scopes = append(scopes, auditScope{
				ID:          id,
				Description: desc,
				PromptFile:  filepath.Join(dir, e.Name()),
			})
		}
	}
	return scopes, nil
}

func runCodebaseAudit(ctx context.Context, app *cmdutil.App, scopes []auditScope) error {
	cfg, err := app.Config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	model, ok := cfg.Models[cfg.Audit.Model]
	if !ok {
		return fmt.Errorf("audit model %q not found", cfg.Audit.Model)
	}

	// Build combined prompt from selected scopes
	var promptParts []string

	// Base audit prompt
	basePrompt, err := pipeline.ResolveFragment(app.Config.Root(), "prompts/"+cfg.Audit.PromptFile)
	if err == nil {
		promptParts = append(promptParts, basePrompt)
	}

	// Scope prompts
	for _, scope := range scopes {
		data, err := os.ReadFile(scope.PromptFile)
		if err != nil {
			continue
		}
		promptParts = append(promptParts, fmt.Sprintf("## Scope: %s\n\n%s", scope.ID, string(data)))
	}

	prompt := strings.Join(promptParts, "\n\n---\n\n")

	output.PrintHuman("Auditing %d scope(s): %s", len(scopes), strings.Join(scopeIDs(scopes), ", "))

	repoDir := filepath.Dir(app.Config.Root())
	invokeCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Daemon.InvocationTimeoutSeconds)*time.Second)
	defer cancel()

	result, err := app.Invoker.Invoke(invokeCtx, model, prompt, repoDir, nil, nil)
	if err != nil {
		return fmt.Errorf("audit invocation failed: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("audit model exited with code %d: %s", result.ExitCode, result.Stderr)
	}

	// Parse findings from model output
	findings := parseFindings(result.Stdout)

	if len(findings) == 0 {
		output.PrintHuman("No findings extracted from audit output.")
		output.PrintHuman("\nRaw output:\n%s", result.Stdout)
		return nil
	}

	// Build the batch
	now := app.Clock.Now()
	batch := &state.Batch{
		ID:        fmt.Sprintf("audit-%s", now.Format("20060102T150405Z")),
		Timestamp: now,
		Scopes:    scopeIDs(scopes),
		Status:    state.BatchPending,
		Findings:  findings,
		RawOutput: result.Stdout,
	}

	// Save the batch
	batchPath := filepath.Join(app.Config.Root(), "audit-state.json")
	if err := state.SaveBatch(batchPath, batch); err != nil {
		return err
	}

	if app.JSONOutput {
		output.Print(output.Ok("audit_run", map[string]any{
			"batch_id":      batch.ID,
			"finding_count": len(findings),
			"scopes":        batch.Scopes,
		}))
	} else {
		output.PrintHuman("\n%d finding(s) saved.", len(findings))
		for i, f := range findings {
			output.PrintHuman("  %d. %s", i+1, f.Title)
		}
		output.PrintHuman("\nReview with: wolfcastle audit pending")
		output.PrintHuman("Approve:     wolfcastle audit approve <id>")
		output.PrintHuman("Reject:      wolfcastle audit reject <id>")
	}

	return nil
}

// parseFindings extracts structured findings from model output.
func parseFindings(rawOutput string) []state.Finding {
	var findings []state.Finding
	lines := strings.Split(rawOutput, "\n")
	findingNum := 0

	var currentTitle string
	var currentDesc strings.Builder

	flush := func() {
		if currentTitle == "" {
			return
		}
		findingNum++
		findings = append(findings, state.Finding{
			ID:          fmt.Sprintf("finding-%d", findingNum),
			Title:       currentTitle,
			Description: strings.TrimSpace(currentDesc.String()),
			Status:      state.FindingPending,
		})
		currentTitle = ""
		currentDesc.Reset()
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Match markdown headings
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			title := strings.TrimLeft(trimmed, "# ")
			title = strings.TrimSpace(title)
			if title != "" && !strings.EqualFold(title, "Audit Findings") {
				flush()
				currentTitle = title
				continue
			}
		}

		// Match numbered bold items: "1. **Title**"
		if len(trimmed) > 3 && trimmed[0] >= '0' && trimmed[0] <= '9' && strings.Contains(trimmed, "**") {
			start := strings.Index(trimmed, "**") + 2
			end := strings.Index(trimmed[start:], "**")
			if end > 0 {
				flush()
				currentTitle = trimmed[start : start+end]
				// Capture any text after the bold title as description start
				rest := strings.TrimSpace(trimmed[start+end+2:])
				if rest != "" {
					rest = strings.TrimPrefix(rest, ":")
					rest = strings.TrimPrefix(rest, " — ")
					rest = strings.TrimPrefix(rest, " - ")
					currentDesc.WriteString(strings.TrimSpace(rest))
				}
				continue
			}
		}

		// Accumulate description lines
		if currentTitle != "" && trimmed != "" {
			if currentDesc.Len() > 0 {
				currentDesc.WriteString("\n")
			}
			currentDesc.WriteString(line)
		}
	}
	flush()

	return findings
}

// scopeIDs returns the IDs of the given scopes as a slice.
func scopeIDs(scopes []auditScope) []string {
	ids := make([]string, len(scopes))
	for i, s := range scopes {
		ids[i] = s.ID
	}
	return ids
}

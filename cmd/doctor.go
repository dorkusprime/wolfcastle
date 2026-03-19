package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/validate"
	"github.com/spf13/cobra"
)

// doctorCmd validates the structural integrity of the project tree.
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Inspect the project tree for damage",
	Long: `Scans for orphaned files, state inconsistencies, missing audit tasks,
and 20 other categories of structural problems.

Without --fix, reports what it finds. With --fix, repairs everything
it can deterministically, then attempts model-assisted repair if a
doctor model is configured. Anything left gets escalation guidance
with the exact commands to resolve it manually.

Examples:
  wolfcastle doctor
  wolfcastle doctor --fix
  wolfcastle doctor --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := app.RequireResolver(); err != nil {
			return err
		}

		fix, _ := cmd.Flags().GetBool("fix")

		// Load root index, attempting recovery on failure.
		idx, err := app.Resolver.LoadRootIndex()
		if err != nil {
			idx, err = tryRecoverRootIndex(app.Resolver.RootIndexPath(), fix)
			if err != nil {
				issues := []validate.Issue{{
					Severity:    validate.SeverityError,
					Category:    validate.CatMalformedJSON,
					Description: fmt.Sprintf("Cannot load root index: %v", err),
					CanAutoFix:  true,
					FixType:     validate.FixDeterministic,
				}}
				return reportValidationIssues(issues)
			}
		}

		// Run validation with a recovering node loader so that malformed
		// node state files are parsed on a best-effort basis rather than
		// causing the entire check to bail out.
		var recoveredNodes []validate.RecoveredNode
		nodeLoader := validate.RecoveringNodeLoader(app.Resolver.ProjectsDir(), func(addr string, report *validate.RecoveryReport) {
			recoveredNodes = append(recoveredNodes, validate.RecoveredNode{Address: addr, Report: report})
		})
		engine := validate.NewEngine(app.Resolver.ProjectsDir(), nodeLoader, app.WolfcastleDir)
		report := engine.ValidateAll(idx)

		// Inject MALFORMED_JSON issues for any nodes that required recovery.
		for _, rn := range recoveredNodes {
			desc := fmt.Sprintf("Recovered from malformed JSON (%s)", strings.Join(rn.Report.Applied, "; "))
			if len(rn.Report.Lost) > 0 {
				desc += fmt.Sprintf(" [data lost: %s]", strings.Join(rn.Report.Lost, "; "))
			}
			report.Issues = append(report.Issues, validate.Issue{
				Severity:    validate.SeverityError,
				Category:    validate.CatMalformedJSON,
				Node:        rn.Address,
				Description: desc,
				CanAutoFix:  true,
				FixType:     validate.FixDeterministic,
			})
		}
		report.Counts()

		if !fix {
			return reportValidationIssues(report.Issues)
		}

		// Report issues first
		if err := reportValidationIssues(report.Issues); err != nil {
			return err
		}

		// Multi-pass deterministic fixes: each pass validates, fixes, and
		// re-validates until no fixable issues remain or the pass cap is hit.
		// This handles cascading issues (e.g., resetting stale tasks changes
		// propagation state, which changes audit status).
		fixes, finalReport, fixErr := validate.FixWithVerification(
			app.Resolver.ProjectsDir(),
			app.Resolver.RootIndexPath(),
			nodeLoader,
			app.WolfcastleDir,
		)

		if len(fixes) == 0 {
			output.PrintHuman("\nNothing to fix automatically.")
		} else {
			output.PrintHuman("\nFixed %d issues:", len(fixes))
			for _, f := range fixes {
				passLabel := ""
				if f.Pass > 1 {
					passLabel = fmt.Sprintf(" (pass %d)", f.Pass)
				}
				output.PrintHuman("  FIXED [%s] %s: %s%s", f.Category, f.Node, f.Description, passLabel)
			}
		}

		var postFixWarnings []validate.Issue
		if finalReport != nil {
			for _, issue := range finalReport.Issues {
				issue.Severity = validate.SeverityWarning
				issue.Description = "post-fix: " + issue.Description
				postFixWarnings = append(postFixWarnings, issue)
			}
		}
		if len(postFixWarnings) > 0 {
			output.PrintHuman("\n%d issue(s) survived the fix:", len(postFixWarnings))
			for _, w := range postFixWarnings {
				output.PrintHuman("  WARN  [%s] %s: %s", w.Category, w.Node, w.Description)
			}
		}

		if fixErr != nil {
			output.PrintError("Fix error: %v", fixErr)
		}

		// Model-assisted fixes for issues that deterministic repair cannot resolve
		var modelIssues []validate.Issue
		for _, issue := range report.Issues {
			if issue.FixType == validate.FixModelAssisted {
				modelIssues = append(modelIssues, issue)
			}
		}
		if len(modelIssues) > 0 && app.Cfg != nil && app.Cfg.Doctor.Model != "" {
			model, ok := app.Cfg.Models[app.Cfg.Doctor.Model]
			if ok {
				output.PrintHuman("\nCalling in model-assisted repair for %d issue(s)...", len(modelIssues))
				ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer cancel()
				modelFixed := 0
				for _, issue := range modelIssues {
					applied, err := validate.TryModelAssistedFix(ctx, app.Invoker, model, issue, app.Resolver.ProjectsDir(), app.WolfcastleDir)
					if err != nil {
						output.PrintHuman("  SKIP  [%s] %s: %v", issue.Category, issue.Node, err)
						continue
					}
					if applied {
						modelFixed++
						output.PrintHuman("  FIXED [%s] %s: model-assisted resolution", issue.Category, issue.Node)
					}
				}
				if modelFixed > 0 {
					output.PrintHuman("Model-assisted fixes applied: %d/%d", modelFixed, len(modelIssues))
				} else {
					output.PrintHuman("Model could not fix any of them.")
				}
			}
		}

		// Escalation guidance for issues that survived all fix passes
		var remaining []validate.Issue
		remaining = append(remaining, postFixWarnings...)
		remaining = append(remaining, modelIssues...)
		var manualIssues []validate.Issue
		for _, issue := range remaining {
			if issue.FixType == validate.FixManual || issue.FixType == validate.FixModelAssisted {
				manualIssues = append(manualIssues, issue)
			}
		}
		if len(manualIssues) > 0 {
			output.PrintHuman("\n%d issue(s) need manual attention:", len(manualIssues))
			for _, issue := range manualIssues {
				if issue.Node != "" {
					output.PrintHuman("  [%s] %s: %s", issue.Category, issue.Node, issue.Description)
				} else {
					output.PrintHuman("  [%s] %s", issue.Category, issue.Description)
				}
			}
			output.PrintHuman("")
			output.PrintHuman("To resolve manually:")
			output.PrintHuman("  wolfcastle task update --node <address> --task <id> --state not_started")
			output.PrintHuman("  wolfcastle unblock --node <address>")
			output.PrintHuman("")
			if len(modelIssues) > 0 && (app.Cfg == nil || app.Cfg.Doctor.Model == "") {
				output.PrintHuman("To try model-assisted repair, configure doctor.model in wolfcastle.yaml")
			}
		}

		return nil
	},
}

func reportValidationIssues(issues []validate.Issue) error {
	if app.JSONOutput {
		output.Print(output.Ok("doctor", map[string]any{
			"issues": issues,
			"count":  len(issues),
		}))
		return nil
	}

	if len(issues) == 0 {
		output.PrintHuman("No issues found. Clean.")
		return nil
	}

	errors := 0
	warnings := 0
	for _, issue := range issues {
		prefix := "  "
		switch issue.Severity {
		case validate.SeverityError:
			prefix = "  ERROR"
			errors++
		case validate.SeverityWarning:
			prefix = "  WARN "
			warnings++
		case validate.SeverityInfo:
			prefix = "  INFO "
		}
		fixLabel := ""
		if issue.FixType != "" {
			fixLabel = fmt.Sprintf(" [%s]", issue.FixType)
		}
		if issue.Node != "" {
			output.PrintHuman("%s [%s] %s: %s%s", prefix, issue.Category, issue.Node, issue.Description, fixLabel)
		} else {
			output.PrintHuman("%s [%s] %s%s", prefix, issue.Category, issue.Description, fixLabel)
		}
	}
	output.PrintHuman("")
	output.PrintHuman("Found %d issues (%d errors, %d warnings)", len(issues), errors, warnings)
	return nil
}

// tryRecoverRootIndex reads the raw bytes from indexPath and attempts JSON
// recovery. When fix is true and recovery succeeds, the repaired index is
// written back to disk. When fix is false, the recovered index is returned
// in memory (read-only) so the doctor can report on it without modifying
// anything.
func tryRecoverRootIndex(indexPath string, fix bool) (*state.RootIndex, error) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("reading root index for recovery: %w", err)
	}

	idx, report, recoverErr := validate.RecoverRootIndex(data)
	if recoverErr != nil {
		return nil, fmt.Errorf("recovery failed: %w", recoverErr)
	}

	output.PrintHuman("Recovered root index from malformed JSON:")
	for _, step := range report.Applied {
		output.PrintHuman("  %s", step)
	}
	for _, loss := range report.Lost {
		output.PrintHuman("  LOST: %s", loss)
	}

	if fix {
		if err := state.SaveRootIndex(indexPath, idx); err != nil {
			return nil, fmt.Errorf("writing recovered root index: %w", err)
		}
		output.PrintHuman("  FIXED: wrote recovered root index to disk")
	} else {
		output.PrintHuman("  Run with --fix to write the recovered version to disk.")
	}

	return idx, nil
}

func init() {
	doctorCmd.Flags().Bool("fix", false, "Fix issues: deterministic first, then model-assisted, then manual guidance")
	rootCmd.AddCommand(doctorCmd)
}

package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/validate"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Validate structural integrity of the project tree",
	Long: `Checks for orphaned files, state inconsistencies, missing audit tasks,
and other structural issues in the project tree. Validates all 17 required
structural categories from the spec.

Use --fix to automatically repair deterministic issues like missing audit
tasks, state mismatches between index and node files, and orphaned entries.

Examples:
  wolfcastle doctor
  wolfcastle doctor --fix
  wolfcastle doctor --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if app.Resolver == nil {
			return fmt.Errorf("identity not configured — run 'wolfcastle init' first")
		}

		// Load root index
		idx, err := app.Resolver.LoadRootIndex()
		if err != nil {
			issues := []validate.Issue{{
				Severity:    validate.SeverityError,
				Category:    validate.CatMalformedJSON,
				Description: fmt.Sprintf("Cannot load root index: %v", err),
			}}
			return reportValidationIssues(issues)
		}

		// Run validation
		engine := validate.NewEngine(app.Resolver.ProjectsDir(), validate.DefaultNodeLoader(app.Resolver.ProjectsDir()), app.WolfcastleDir)
		report := engine.ValidateAll(idx)

		fix, _ := cmd.Flags().GetBool("fix")
		if !fix {
			return reportValidationIssues(report.Issues)
		}

		// Report issues first
		if err := reportValidationIssues(report.Issues); err != nil {
			return err
		}

		// Apply deterministic fixes
		fixes, postFixWarnings, fixErr := validate.ApplyDeterministicFixes(idx, report.Issues, app.Resolver.ProjectsDir(), app.Resolver.RootIndexPath(), app.WolfcastleDir)

		if len(fixes) == 0 {
			output.PrintHuman("\nNo auto-fixable issues found")
		} else {
			output.PrintHuman("\nFixed %d issues:", len(fixes))
			for _, f := range fixes {
				output.PrintHuman("  FIXED [%s] %s: %s", f.Category, f.Node, f.Description)
			}
		}

		if len(postFixWarnings) > 0 {
			output.PrintHuman("\nPost-fix re-validation found %d remaining issue(s):", len(postFixWarnings))
			for _, w := range postFixWarnings {
				output.PrintHuman("  WARN  [%s] %s: %s", w.Category, w.Node, w.Description)
			}
		}

		if fixErr != nil {
			output.PrintError("Fix error: %v", fixErr)
		}

		// Model-assisted fixes for issues that deterministic repair cannot resolve
		if app.Cfg != nil && app.Cfg.Doctor.Model != "" {
			model, ok := app.Cfg.Models[app.Cfg.Doctor.Model]
			if ok {
				var modelIssues []validate.Issue
				for _, issue := range report.Issues {
					if issue.FixType == validate.FixModelAssisted {
						modelIssues = append(modelIssues, issue)
					}
				}
				if len(modelIssues) > 0 {
					output.PrintHuman("\nAttempting model-assisted fixes for %d issues...", len(modelIssues))
					ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
					defer cancel()
					modelFixed := 0
					for _, issue := range modelIssues {
						applied, err := validate.TryModelAssistedFix(ctx, model, issue, app.Resolver.ProjectsDir())
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
						output.PrintHuman("No model-assisted fixes were applicable")
					}
				}
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
		output.PrintHuman("No issues found — project tree is healthy")
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

func init() {
	doctorCmd.Flags().Bool("fix", false, "Attempt to fix deterministic issues")
	rootCmd.AddCommand(doctorCmd)
}

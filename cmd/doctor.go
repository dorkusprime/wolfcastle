package cmd

import (
	"fmt"

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
		if resolver == nil {
			return fmt.Errorf("identity not configured — run 'wolfcastle init' first")
		}

		// Load root index
		idx, err := resolver.LoadRootIndex()
		if err != nil {
			issues := []validate.Issue{{
				Severity:    validate.SeverityError,
				Category:    validate.CatMalformedJSON,
				Description: fmt.Sprintf("Cannot load root index: %v", err),
			}}
			return reportValidationIssues(issues)
		}

		// Run validation
		engine := validate.NewEngine(resolver.ProjectsDir(), validate.DefaultNodeLoader(resolver.ProjectsDir()), wolfcastleDir)
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
		fixes, fixErr := validate.ApplyDeterministicFixes(idx, report.Issues, resolver.ProjectsDir(), resolver.RootIndexPath(), wolfcastleDir)

		if len(fixes) == 0 {
			output.PrintHuman("\nNo auto-fixable issues found")
		} else {
			output.PrintHuman("\nFixed %d issues:", len(fixes))
			for _, f := range fixes {
				output.PrintHuman("  FIXED [%s] %s: %s", f.Category, f.Node, f.Description)
			}
		}

		if fixErr != nil {
			output.PrintError("Fix error: %v", fixErr)
		}

		return nil
	},
}

func reportValidationIssues(issues []validate.Issue) error {
	if jsonOutput {
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

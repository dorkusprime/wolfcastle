package config

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

// validationIssue represents a single problem found during config validation.
type validationIssue struct {
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// validationReport collects the full set of issues found during validation.
type validationReport struct {
	Issues       []validationIssue `json:"issues"`
	ErrorCount   int               `json:"error_count"`
	WarningCount int               `json:"warning_count"`
}

func newValidateCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Check configuration for errors",
		Long: `Validate the resolved Wolfcastle configuration.

By default, runs structural checks only (field constraints, stage
consistency, threshold bounds). Use --full to include identity and
cross-reference checks.

Examples:
  wolfcastle config validate
  wolfcastle config validate --full
  wolfcastle config validate --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			full, _ := cmd.Flags().GetBool("full")

			cfg, err := app.Config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			var report validationReport

			// Collect warnings from config loading (unknown fields, etc.)
			for _, w := range cfg.Warnings {
				report.Issues = append(report.Issues, validationIssue{
					Severity:    "warning",
					Category:    "unknown_field",
					Description: w,
				})
			}

			// Validate calls ValidateStructure internally, so calling
			// Validate alone covers both structural and reference checks.
			var validationErr error
			if full {
				validationErr = config.Validate(cfg)
			} else {
				validationErr = config.ValidateStructure(cfg)
			}
			if validationErr != nil {
				category := "structure"
				if full {
					category = "validation"
				}
				parseValidationErrors(&report, category, validationErr)
			}

			report.ErrorCount = 0
			report.WarningCount = 0
			for _, issue := range report.Issues {
				switch issue.Severity {
				case "error":
					report.ErrorCount++
				case "warning":
					report.WarningCount++
				}
			}

			if app.JSON {
				output.Print(output.Ok("config_validate", report))
				if report.ErrorCount > 0 {
					return fmt.Errorf("validation failed")
				}
				return nil
			}

			// Human output.
			for _, issue := range report.Issues {
				line := fmt.Sprintf("[%s] %s: %s", issue.Severity, issue.Category, issue.Description)
				if issue.Severity == "error" {
					output.PrintError("%s", line)
				} else {
					output.PrintHuman("%s", line)
				}
			}

			summary := fmt.Sprintf("%s, %s",
				output.Plural(report.ErrorCount, "error", "errors"),
				output.Plural(report.WarningCount, "warning", "warnings"))

			if report.ErrorCount > 0 {
				output.PrintHuman("%s", summary)
				return fmt.Errorf("validation failed")
			}
			output.PrintHuman("%s", summary)
			return nil
		},
	}

	cmd.Flags().Bool("full", false, "Run full validation including identity and cross-reference checks")
	return cmd
}

// parseValidationErrors splits the formatted error from ValidateStructure or
// Validate into individual issues and appends them to the report.
func parseValidationErrors(report *validationReport, category string, err error) {
	for _, line := range splitValidationError(err) {
		report.Issues = append(report.Issues, validationIssue{
			Severity:    "error",
			Category:    category,
			Description: line,
		})
	}
}

// splitValidationError parses the "config validation failed:\n  - ..." format
// returned by ValidateStructure and Validate into individual error strings.
func splitValidationError(err error) []string {
	msg := err.Error()
	// Strip the prefix if present.
	const prefix = "config validation failed:\n"
	if idx := strings.Index(msg, prefix); idx >= 0 {
		msg = msg[idx+len(prefix):]
	}

	var lines []string
	for _, raw := range strings.Split(msg, "\n") {
		line := strings.TrimSpace(raw)
		line = strings.TrimPrefix(line, "- ")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

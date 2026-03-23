package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/daemon"
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
and 14 other categories of structural problems. Use --fix to repair
what can be repaired automatically.

Examples:
  wolfcastle doctor
  wolfcastle doctor --fix
  wolfcastle doctor --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := app.RequireIdentity(); err != nil {
			return err
		}

		fix, _ := cmd.Flags().GetBool("fix")
		root := app.Config.Root()
		projectsDir := app.State.Dir()
		indexPath := app.State.IndexPath()

		// Load root index, attempting recovery on failure.
		idx, err := app.State.ReadIndex()
		if err != nil {
			idx, err = tryRecoverRootIndex(indexPath, fix)
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
		nodeLoader := validate.RecoveringNodeLoader(projectsDir, func(addr string, report *validate.RecoveryReport) {
			recoveredNodes = append(recoveredNodes, validate.RecoveredNode{Address: addr, Report: report})
		})
		engine := validate.NewEngine(projectsDir, nodeLoader, daemon.NewDaemonRepository(root))
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

		// In human mode, report pre-fix issues first.
		if !app.JSON {
			if err := reportValidationIssues(report.Issues); err != nil {
				return err
			}
		}

		// Apply deterministic fixes
		fixes, postFixWarnings, fixErr := validate.ApplyDeterministicFixes(idx, report.Issues, projectsDir, indexPath, daemon.NewDaemonRepository(root))

		if !app.JSON {
			if len(fixes) == 0 {
				output.PrintHuman("\nNothing to fix automatically.")
			} else {
				output.PrintHuman("\nFixed %d issues:", len(fixes))
				for _, f := range fixes {
					output.PrintHuman("  FIXED [%s] %s: %s", f.Category, f.Node, f.Description)
				}
			}

			if len(postFixWarnings) > 0 {
				output.PrintHuman("\n%s survived the fix:", output.Plural(len(postFixWarnings), "issue", "issues"))
				for _, w := range postFixWarnings {
					output.PrintHuman("  WARN  [%s] %s: %s", w.Category, w.Node, w.Description)
				}
			}
		}

		if fixErr != nil {
			output.PrintError("Fix error: %v", fixErr)
		}

		// Model-assisted fixes for issues that deterministic repair cannot resolve
		var modelFixCount int
		cfg, cfgErr := app.Config.Load()
		if cfgErr == nil && cfg.Doctor.Model != "" {
			model, ok := cfg.Models[cfg.Doctor.Model]
			if ok {
				var modelIssues []validate.Issue
				for _, issue := range report.Issues {
					if issue.FixType == validate.FixModelAssisted {
						modelIssues = append(modelIssues, issue)
					}
				}
				if len(modelIssues) > 0 {
					output.PrintHuman("\nCalling in model-assisted repair for %s...", output.Plural(len(modelIssues), "issue", "issues"))
					ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
					defer cancel()
					for _, issue := range modelIssues {
						applied, err := validate.TryModelAssistedFix(ctx, app.Invoker, model, issue, projectsDir, root)
						if err != nil {
							output.PrintHuman("  SKIP  [%s] %s: %v", issue.Category, issue.Node, err)
							continue
						}
						if applied {
							modelFixCount++
							output.PrintHuman("  FIXED [%s] %s: model-assisted resolution", issue.Category, issue.Node)
						}
					}
					if modelFixCount > 0 {
						output.PrintHuman("Model-assisted fixes applied: %d/%d", modelFixCount, len(modelIssues))
					} else {
						output.PrintHuman("Model could not fix any of them.")
					}
				}
			}
		}

		// JSON output for --fix includes issues, fixes applied, and remaining warnings.
		if app.JSON {
			output.Print(output.Ok("doctor", map[string]any{
				"issues":      report.Issues,
				"count":       len(report.Issues),
				"fixed":       len(fixes),
				"remaining":   len(postFixWarnings),
				"model_fixed": modelFixCount,
			}))
		}

		return nil
	},
}

func reportValidationIssues(issues []validate.Issue) error {
	if app.JSON {
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

	// Count fixable issues and suggest --fix if any exist.
	fixable := 0
	for _, issue := range issues {
		if issue.CanAutoFix {
			fixable++
		}
	}
	if fixable > 0 {
		output.PrintHuman("%s can be repaired automatically. Run 'wolfcastle doctor --fix' to apply.", output.Plural(fixable, "issue", "issues"))
	}
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
	doctorCmd.Flags().Bool("fix", false, "Attempt to fix deterministic issues")
	rootCmd.AddCommand(doctorCmd)
}

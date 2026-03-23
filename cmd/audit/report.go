package audit

import (
	"fmt"
	"os"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newReportCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Display the latest audit report for a node",
		Long: `Shows the most recent markdown audit report for a node. If no report
exists yet, shows the current audit state as a report preview.

Examples:
  wolfcastle audit report --node my-project
  wolfcastle audit report --node my-project --path`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required: specify the node to inspect")
			}

			pathOnly, _ := cmd.Flags().GetBool("path")

			// Validate that the node exists in the index
			idx, err := app.State.ReadIndex()
			if err != nil {
				return fmt.Errorf("reading index: %w", err)
			}
			if _, exists := idx.Nodes[nodeAddr]; !exists {
				return fmt.Errorf("node %q not found in index. Check the address with 'wolfcastle status'", nodeAddr)
			}

			// Check for an existing report file
			reportPath := state.LatestAuditReport(app.State.Dir(), nodeAddr)

			if pathOnly {
				if reportPath == "" {
					if app.JSON {
						output.Print(output.Ok("audit_report", map[string]string{
							"node": nodeAddr,
							"path": "",
						}))
					} else {
						output.PrintHuman("No audit report found for %s", nodeAddr)
					}
					return nil
				}
				if app.JSON {
					output.Print(output.Ok("audit_report", map[string]string{
						"node": nodeAddr,
						"path": reportPath,
					}))
				} else {
					output.PrintHuman("%s", reportPath)
				}
				return nil
			}

			// If a report file exists, display it
			if reportPath != "" {
				data, err := os.ReadFile(reportPath)
				if err != nil {
					return fmt.Errorf("reading report: %w", err)
				}
				if app.JSON {
					output.Print(output.Ok("audit_report", map[string]any{
						"node":    nodeAddr,
						"path":    reportPath,
						"content": string(data),
					}))
				} else {
					fmt.Print(string(data))
				}
				return nil
			}

			// No report on disk: generate a preview from current state
			ns, err := app.State.ReadNode(nodeAddr)
			if err != nil {
				return fmt.Errorf("loading node state: %w", err)
			}

			report := state.GenerateAuditReport(ns.Audit, nodeAddr, ns.Name)
			if app.JSON {
				output.Print(output.Ok("audit_report", map[string]any{
					"node":    nodeAddr,
					"path":    "",
					"content": report,
				}))
			} else {
				fmt.Print(report)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Target node address (required)")
	cmd.Flags().Bool("path", false, "Print only the report file path")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}

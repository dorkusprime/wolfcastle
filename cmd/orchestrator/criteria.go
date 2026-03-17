package orchestrator

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newCriteriaCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "criteria [criterion]",
		Short: "Manage success criteria for an orchestrator node",
		Long: `Appends a success criterion to an orchestrator node, or lists existing
criteria with --list. Duplicates are silently ignored.

Examples:
  wolfcastle orchestrator criteria --node my-project "all tests pass"
  wolfcastle orchestrator criteria --node my-project --list`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireResolver(); err != nil {
				return err
			}
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required: specify the target node address")
			}
			listMode, _ := cmd.Flags().GetBool("list")

			if listMode {
				ns, err := app.Store.ReadNode(nodeAddr)
				if err != nil {
					return err
				}
				if app.JSONOutput {
					output.Print(output.Ok("success_criteria", map[string]any{
						"node":     nodeAddr,
						"criteria": ns.SuccessCriteria,
					}))
				} else {
					if len(ns.SuccessCriteria) == 0 {
						output.PrintHuman("No success criteria defined for %s", nodeAddr)
					} else {
						output.PrintHuman("Success criteria for %s:", nodeAddr)
						for _, c := range ns.SuccessCriteria {
							output.PrintHuman("  - %s", c)
						}
					}
				}
				return nil
			}

			if len(args) == 0 {
				return fmt.Errorf("criterion text is required (or use --list to view existing criteria)")
			}
			criterion := args[0]
			if strings.TrimSpace(criterion) == "" {
				return fmt.Errorf("criterion text cannot be empty")
			}

			if err := app.Store.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				for _, existing := range ns.SuccessCriteria {
					if existing == criterion {
						return nil
					}
				}
				ns.SuccessCriteria = append(ns.SuccessCriteria, criterion)
				return nil
			}); err != nil {
				return err
			}

			if app.JSONOutput {
				output.Print(output.Ok("success_criteria_add", map[string]string{
					"node":      nodeAddr,
					"criterion": criterion,
				}))
			} else {
				output.PrintHuman("Added success criterion to %s: %s", nodeAddr, criterion)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Target node address (required)")
	_ = cmd.MarkFlagRequired("node")
	cmd.Flags().Bool("list", false, "List current success criteria instead of adding")
	return cmd
}

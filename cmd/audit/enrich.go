package audit

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newEnrichCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enrich [text]",
		Short: "Add enrichment context for auditing",
		Long: `Appends enrichment text to a node's audit enrichment list. This provides
additional context the auditor should consider when evaluating the node.
Duplicates are silently ignored.

Examples:
  wolfcastle audit enrich --node my-project "check error handling in auth module"
  wolfcastle audit enrich --node my-project "verify backward compatibility"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("missing required argument: <text>")
			}
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			text := args[0]
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("enrichment text cannot be empty. Describe the context to add")
			}
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required: specify the target node address")
			}

			if err := app.State.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				for _, existing := range ns.AuditEnrichment {
					if existing == text {
						return nil
					}
				}
				ns.AuditEnrichment = append(ns.AuditEnrichment, text)
				return nil
			}); err != nil {
				return err
			}

			if app.JSON {
				output.Print(output.Ok("audit_enrich", map[string]string{
					"node": nodeAddr,
					"text": text,
				}))
			} else {
				output.PrintHuman("Added audit enrichment to %s: %s", nodeAddr, text)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Target node address (required)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}

package audit

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newScopeCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scope",
		Short: "Define what the audit covers",
		Long: `Sets structured audit scope: description, files, systems, and criteria.

Examples:
  wolfcastle audit scope --node my-project --description "Verify auth module"
  wolfcastle audit scope --node my-project --files "auth.go|login.go" --systems "auth|session"
  wolfcastle audit scope --node my-project --criteria "no SQL injection|input validation"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required: specify the target node address")
			}
			description, _ := cmd.Flags().GetString("description")
			files, _ := cmd.Flags().GetString("files")
			systems, _ := cmd.Flags().GetString("systems")
			criteria, _ := cmd.Flags().GetString("criteria")

			if description == "" && files == "" && systems == "" && criteria == "" {
				return fmt.Errorf("at least one scope field is required (--description, --files, --systems, --criteria)")
			}

			var scope *state.AuditScope
			if err := app.State.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				if ns.Audit.Scope == nil {
					ns.Audit.Scope = &state.AuditScope{}
				}
				if description != "" {
					ns.Audit.Scope.Description = description
				}
				if files != "" {
					ns.Audit.Scope.Files = dedup(splitPipe(files))
				}
				if systems != "" {
					ns.Audit.Scope.Systems = dedup(splitPipe(systems))
				}
				if criteria != "" {
					ns.Audit.Scope.Criteria = dedup(splitPipe(criteria))
				}
				scope = ns.Audit.Scope
				return nil
			}); err != nil {
				return err
			}

			if app.JSON {
				output.Print(output.Ok("audit_scope", scope))
			} else {
				output.PrintHuman("Audit scope updated for %s", nodeAddr)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Target node address (required)")
	_ = cmd.MarkFlagRequired("node")
	cmd.Flags().String("description", "", "Audit scope description")
	cmd.Flags().String("files", "", "Pipe-delimited list of files to audit")
	cmd.Flags().String("systems", "", "Pipe-delimited list of systems to audit")
	cmd.Flags().String("criteria", "", "Pipe-delimited list of acceptance criteria")
	return cmd
}

func splitPipe(s string) []string {
	var result []string
	for _, part := range strings.Split(s, "|") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func dedup(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

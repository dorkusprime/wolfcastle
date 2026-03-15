package cmd

import "github.com/spf13/cobra"

// adrCmd is the parent command for architecture decision record management.
var adrCmd = &cobra.Command{
	Use:   "adr",
	Short: "Record architecture decisions",
	Long: `ADRs document design decisions: the context, the call, and what
follows. Timestamped Markdown files in docs/decisions/. Permanent
record. No take-backs.

Examples:
  wolfcastle adr create "Use JWT for authentication"
  wolfcastle adr create --stdin "Migration strategy" < body.md
  wolfcastle adr create --file rationale.md "Switch to PostgreSQL"`,
}

func init() {
	rootCmd.AddCommand(adrCmd)
}

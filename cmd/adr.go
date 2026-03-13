package cmd

import "github.com/spf13/cobra"

var adrCmd = &cobra.Command{
	Use:   "adr",
	Short: "Manage architecture decision records",
	Long: `Create and manage Architecture Decision Records (ADRs).

ADRs document significant design decisions with context, the decision
itself, and its consequences. They are stored as timestamped Markdown
files in the docs/decisions/ directory.

Examples:
  wolfcastle adr create "Use JWT for authentication"
  wolfcastle adr create --stdin "Migration strategy" < body.md
  wolfcastle adr create --file rationale.md "Switch to PostgreSQL"`,
}

func init() {
	rootCmd.AddCommand(adrCmd)
}

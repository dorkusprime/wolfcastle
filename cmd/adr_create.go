package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

// adrCreateCmd creates a new timestamped ADR Markdown file.
var adrCreateCmd = &cobra.Command{
	Use:   "create [title]",
	Short: "File a new decision record",
	Long: `Creates a timestamped ADR in docs/decisions/. Provide the body via
--stdin or --file, or get a template with Context/Decision/Consequences.

Examples:
  wolfcastle adr create "Use JWT for authentication"
  wolfcastle adr create --stdin "Migration strategy" < body.md
  wolfcastle adr create --file rationale.md "Switch to PostgreSQL"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]
		if strings.TrimSpace(title) == "" {
			return fmt.Errorf("ADR title cannot be empty. Name the decision")
		}
		useStdin, _ := cmd.Flags().GetBool("stdin")
		bodyFile, _ := cmd.Flags().GetString("file")

		if useStdin && bodyFile != "" {
			return fmt.Errorf("--stdin and --file are mutually exclusive")
		}

		// Build filename
		now := app.Clock.Now()
		timestamp := now.Format("2006-01-02T15-04Z")
		slug := tree.ToSlug(title)
		filename := fmt.Sprintf("%s-%s.md", timestamp, slug)

		// Get body content
		var body string
		if useStdin {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			body = string(data)
		} else if bodyFile != "" {
			data, err := os.ReadFile(bodyFile)
			if err != nil {
				return fmt.Errorf("reading file %s: %w", bodyFile, err)
			}
			body = string(data)
		}

		// Build ADR content
		var content strings.Builder
		fmt.Fprintf(&content, "# %s\n\n", title)
		content.WriteString("## Status\nAccepted\n\n")
		fmt.Fprintf(&content, "## Date\n%s\n\n", now.Format("2006-01-02"))

		if body != "" {
			content.WriteString(body)
		} else {
			content.WriteString("## Context\n\n[Why was this decision needed?]\n\n")
			content.WriteString("## Decision\n\n[What was decided?]\n\n")
			content.WriteString("## Consequences\n\n[What follows from this decision?]\n")
		}

		// Resolve docs directory
		cfg, err := app.Config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		docsDir := filepath.Join(app.Config.Root(), cfg.Docs.Directory, "decisions")
		if err := os.MkdirAll(docsDir, 0755); err != nil {
			return fmt.Errorf("creating decisions directory: %w", err)
		}
		adrPath := filepath.Join(docsDir, filename)

		if err := os.WriteFile(adrPath, []byte(content.String()), 0644); err != nil {
			return fmt.Errorf("writing ADR file: %w", err)
		}

		if app.JSONOutput {
			output.Print(output.Ok("adr_create", map[string]string{
				"title":    title,
				"filename": filename,
				"path":     adrPath,
			}))
		} else {
			output.PrintHuman("Created ADR: %s", adrPath)
		}
		return nil
	},
}

func init() {
	adrCreateCmd.Flags().Bool("stdin", false, "Read ADR body from stdin")
	adrCreateCmd.Flags().String("file", "", "Read ADR body from a file")
	adrCmd.AddCommand(adrCreateCmd)
}

package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

var adrCreateCmd = &cobra.Command{
	Use:   "create [title]",
	Short: "Create a new architecture decision record",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]
		useStdin, _ := cmd.Flags().GetBool("stdin")
		bodyFile, _ := cmd.Flags().GetString("file")

		// Build filename
		now := time.Now().UTC()
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
		content.WriteString(fmt.Sprintf("# %s\n\n", title))
		content.WriteString("## Status\nAccepted\n\n")
		content.WriteString(fmt.Sprintf("## Date\n%s\n\n", now.Format("2006-01-02")))

		if body != "" {
			content.WriteString(body)
		} else {
			content.WriteString("## Context\n\n[Why was this decision needed?]\n\n")
			content.WriteString("## Decision\n\n[What was decided?]\n\n")
			content.WriteString("## Consequences\n\n[What follows from this decision?]\n")
		}

		// Resolve docs directory
		docsDir := filepath.Join(wolfcastleDir, cfg.Docs.Directory, "decisions")
		os.MkdirAll(docsDir, 0755)
		adrPath := filepath.Join(docsDir, filename)

		if err := os.WriteFile(adrPath, []byte(content.String()), 0644); err != nil {
			return err
		}

		if jsonOutput {
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

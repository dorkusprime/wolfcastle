package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

// specCmd is the parent command for spec create, link, and list.
var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Manage spec documents",
	Long: `Specs are the intelligence files. Create them, link them to nodes,
list what's on record. They surface during unblock diagnostics and
audit context.

Examples:
  wolfcastle spec create "API Authentication Flow"
  wolfcastle spec create --node auth-system "Token Refresh Spec"
  wolfcastle spec link auth-spec.md --node auth-system
  wolfcastle spec list --node auth-system`,
}

// specCreateCmd creates a new spec file and optionally links it to a node.
var specCreateCmd = &cobra.Command{
	Use:   "create [title]",
	Short: "Write a new spec document",
	Long: `Creates a spec Markdown file in docs/specs/ and optionally links it
to a node.

Examples:
  wolfcastle spec create "API Authentication Flow"
  wolfcastle spec create --node auth-system "Token Refresh Spec"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("missing required argument: <title>")
		}
		title := args[0]
		if strings.TrimSpace(title) == "" {
			return fmt.Errorf("spec title cannot be empty. Name it")
		}
		nodeAddr, _ := cmd.Flags().GetString("node")
		body, _ := cmd.Flags().GetString("body")
		useStdin, _ := cmd.Flags().GetBool("stdin")

		if nodeAddr != "" {
			if err := app.RequireIdentity(); err != nil {
				return err
			}
		}

		cfg, err := app.Config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		now := app.Clock.Now()
		timestamp := now.Format("2006-01-02T15-04Z")
		slug := tree.ToSlug(title)
		filename := fmt.Sprintf("%s-%s.md", timestamp, slug)

		docsDir := filepath.Join(app.Config.Root(), cfg.Docs.Directory, "specs")
		if err := os.MkdirAll(docsDir, 0755); err != nil {
			return fmt.Errorf("creating specs directory: %w", err)
		}
		specPath := filepath.Join(docsDir, filename)

		var content string
		if useStdin {
			data, readErr := io.ReadAll(os.Stdin)
			if readErr != nil {
				return fmt.Errorf("reading stdin: %w", readErr)
			}
			content = fmt.Sprintf("# %s\n\n%s\n", title, strings.TrimSpace(string(data)))
		} else if body != "" {
			content = fmt.Sprintf("# %s\n\n%s\n", title, body)
		} else {
			content = fmt.Sprintf("# %s\n\n[Spec content goes here.]\n", title)
		}
		if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing spec file: %w", err)
		}

		// Link to node if specified
		if nodeAddr != "" {
			if err := app.State.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				ns.Specs = append(ns.Specs, filename)
				return nil
			}); err != nil {
				return fmt.Errorf("linking spec to node: %w", err)
			}
		}

		if app.JSON {
			output.Print(output.Ok("spec_create", map[string]string{
				"title":    title,
				"filename": filename,
				"path":     specPath,
				"node":     nodeAddr,
			}))
		} else {
			output.PrintHuman("Created spec: %s", specPath)
			if nodeAddr != "" {
				output.PrintHuman("Linked to node: %s", nodeAddr)
			}
		}
		return nil
	},
}

// specLinkCmd links an existing spec file to a project node.
var specLinkCmd = &cobra.Command{
	Use:   "link [filename]",
	Short: "Attach a spec to a node",
	Long: `Links an existing spec file to a project node. The file must exist
in docs/specs/. One spec can serve multiple nodes.

Examples:
  wolfcastle spec link 2025-01-15T10-30Z-auth-flow.md --node auth-system`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("missing required argument: <filename>")
		}
		if err := app.RequireIdentity(); err != nil {
			return err
		}
		filename := args[0]
		nodeAddr, _ := cmd.Flags().GetString("node")
		if nodeAddr == "" {
			return fmt.Errorf("--node is required: specify the target node to link the spec to")
		}

		cfg, err := app.Config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Verify spec exists
		docsDir := filepath.Join(app.Config.Root(), cfg.Docs.Directory, "specs")
		specPath := filepath.Join(docsDir, filename)
		if _, err := os.Stat(specPath); err != nil {
			return fmt.Errorf("spec file not found: %s", specPath)
		}

		if err := app.State.MutateNode(nodeAddr, func(ns *state.NodeState) error {
			// Check for duplicates
			for _, s := range ns.Specs {
				if s == filename {
					return fmt.Errorf("spec %s is already linked to %s", filename, nodeAddr)
				}
			}
			ns.Specs = append(ns.Specs, filename)
			return nil
		}); err != nil {
			return fmt.Errorf("linking spec to node: %w", err)
		}

		if app.JSON {
			output.Print(output.Ok("spec_link", map[string]string{
				"filename": filename,
				"node":     nodeAddr,
			}))
		} else {
			output.PrintHuman("Linked %s to %s", filename, nodeAddr)
		}
		return nil
	},
}

// specListCmd lists specs, optionally filtered by a linked node.
var specListCmd = &cobra.Command{
	Use:   "list",
	Short: "List known specs",
	Long: `Lists all specs, or only those linked to a specific node.

Examples:
  wolfcastle spec list
  wolfcastle spec list --node auth-system
  wolfcastle spec list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeAddr, _ := cmd.Flags().GetString("node")

		if nodeAddr != "" {
			if err := app.RequireIdentity(); err != nil {
				return err
			}
		}

		cfg, err := app.Config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		docsDir := filepath.Join(app.Config.Root(), cfg.Docs.Directory, "specs")
		entries, err := os.ReadDir(docsDir)
		if err != nil {
			return fmt.Errorf("reading specs dir: %w", err)
		}

		// If filtering by node, get linked specs
		var linkedSpecs map[string]bool
		if nodeAddr != "" {
			ns, err := app.State.ReadNode(nodeAddr)
			if err != nil {
				return fmt.Errorf("loading node state: %w", err)
			}
			linkedSpecs = make(map[string]bool)
			for _, s := range ns.Specs {
				linkedSpecs[s] = true
			}
		}

		var specs []map[string]string
		seen := make(map[string]bool)
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".md") || strings.EqualFold(name, "README.md") {
				continue
			}
			if linkedSpecs != nil && !linkedSpecs[name] {
				continue
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			specs = append(specs, map[string]string{
				"filename": name,
			})
		}

		if app.JSON {
			output.Print(output.Ok("spec_list", map[string]any{
				"specs": specs,
				"count": len(specs),
			}))
		} else {
			if len(specs) == 0 {
				output.PrintHuman("No specs on file.")
			} else {
				for _, s := range specs {
					output.PrintHuman("  %s", s["filename"])
				}
			}
		}
		return nil
	},
}

func init() {
	specCreateCmd.Flags().String("node", "", "Link spec to this node")
	specCreateCmd.Flags().String("body", "", "Spec body content")
	specCreateCmd.Flags().Bool("stdin", false, "Read spec body from stdin")
	specLinkCmd.Flags().String("node", "", "Target node address (required)")
	_ = specLinkCmd.MarkFlagRequired("node")
	specListCmd.Flags().String("node", "", "Filter specs by linked node")

	specCmd.AddCommand(specCreateCmd)
	specCmd.AddCommand(specLinkCmd)
	specCmd.AddCommand(specListCmd)
	rootCmd.AddCommand(specCmd)
}

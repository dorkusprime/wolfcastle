package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Manage specs linked to project nodes",
	Long: `Create, link, and list specification documents tied to project nodes.

Specs are stored as Markdown files in docs/specs/ and can be linked to
one or more project nodes. They appear in unblock diagnostics and audit
context.

Examples:
  wolfcastle spec create "API Authentication Flow"
  wolfcastle spec create --node auth-system "Token Refresh Spec"
  wolfcastle spec link auth-spec.md --node auth-system
  wolfcastle spec list --node auth-system`,
}

var specCreateCmd = &cobra.Command{
	Use:   "create [title]",
	Short: "Create a new spec and link it to a node",
	Long: `Creates a new spec Markdown file in docs/specs/ and optionally links it to a node.

Examples:
  wolfcastle spec create "API Authentication Flow"
  wolfcastle spec create --node auth-system "Token Refresh Spec"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]
		if strings.TrimSpace(title) == "" {
			return fmt.Errorf("spec title cannot be empty")
		}
		nodeAddr, _ := cmd.Flags().GetString("node")

		now := time.Now().UTC()
		timestamp := now.Format("2006-01-02T15-04Z")
		slug := tree.ToSlug(title)
		filename := fmt.Sprintf("%s-%s.md", timestamp, slug)

		docsDir := filepath.Join(wolfcastleDir, cfg.Docs.Directory, "specs")
		if err := os.MkdirAll(docsDir, 0755); err != nil {
			return fmt.Errorf("creating specs directory: %w", err)
		}
		specPath := filepath.Join(docsDir, filename)

		content := fmt.Sprintf("# %s\n\n[Spec content goes here.]\n", title)
		if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing spec file: %w", err)
		}

		// Link to node if specified
		if nodeAddr != "" {
			addr, err := tree.ParseAddress(nodeAddr)
			if err != nil {
				return fmt.Errorf("invalid node address: %w", err)
			}
			statePath := resolver.NodeStatePath(addr)
			ns, err := state.LoadNodeState(statePath)
			if err != nil {
				return fmt.Errorf("loading node state: %w", err)
			}
			ns.Specs = append(ns.Specs, filename)
			if err := state.SaveNodeState(statePath, ns); err != nil {
				return fmt.Errorf("saving node state: %w", err)
			}
		}

		if jsonOutput {
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

var specLinkCmd = &cobra.Command{
	Use:   "link [filename]",
	Short: "Link an existing spec to a node",
	Long: `Links an existing spec file to a project node.

The spec file must already exist in docs/specs/. A spec can be linked
to multiple nodes.

Examples:
  wolfcastle spec link 2025-01-15T10-30Z-auth-flow.md --node auth-system`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := args[0]
		nodeAddr, _ := cmd.Flags().GetString("node")
		if nodeAddr == "" {
			return fmt.Errorf("--node is required — specify the target node to link the spec to")
		}

		// Verify spec exists
		docsDir := filepath.Join(wolfcastleDir, cfg.Docs.Directory, "specs")
		specPath := filepath.Join(docsDir, filename)
		if _, err := os.Stat(specPath); err != nil {
			return fmt.Errorf("spec file not found: %s", specPath)
		}

		addr, err := tree.ParseAddress(nodeAddr)
		if err != nil {
			return fmt.Errorf("invalid node address: %w", err)
		}
		statePath := resolver.NodeStatePath(addr)
		ns, err := state.LoadNodeState(statePath)
		if err != nil {
			return fmt.Errorf("loading node state: %w", err)
		}

		// Check for duplicates
		for _, s := range ns.Specs {
			if s == filename {
				return fmt.Errorf("spec %s is already linked to %s", filename, nodeAddr)
			}
		}

		ns.Specs = append(ns.Specs, filename)
		if err := state.SaveNodeState(statePath, ns); err != nil {
			return fmt.Errorf("saving node state: %w", err)
		}

		if jsonOutput {
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

var specListCmd = &cobra.Command{
	Use:   "list",
	Short: "List specs, optionally filtered by node",
	Long: `Lists all specs, or only specs linked to a specific node.

Examples:
  wolfcastle spec list
  wolfcastle spec list --node auth-system
  wolfcastle spec list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeAddr, _ := cmd.Flags().GetString("node")

		docsDir := filepath.Join(wolfcastleDir, cfg.Docs.Directory, "specs")
		entries, err := os.ReadDir(docsDir)
		if err != nil {
			return fmt.Errorf("reading specs dir: %w", err)
		}

		// If filtering by node, get linked specs
		var linkedSpecs map[string]bool
		if nodeAddr != "" {
			addr, err := tree.ParseAddress(nodeAddr)
			if err != nil {
				return fmt.Errorf("invalid node address: %w", err)
			}
			ns, err := state.LoadNodeState(resolver.NodeStatePath(addr))
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

		if jsonOutput {
			output.Print(output.Ok("spec_list", map[string]any{
				"specs": specs,
				"count": len(specs),
			}))
		} else {
			if len(specs) == 0 {
				output.PrintHuman("No specs found")
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
	specLinkCmd.Flags().String("node", "", "Target node address (required)")
	specLinkCmd.MarkFlagRequired("node")
	specListCmd.Flags().String("node", "", "Filter specs by linked node")

	specCmd.AddCommand(specCreateCmd)
	specCmd.AddCommand(specLinkCmd)
	specCmd.AddCommand(specListCmd)
	rootCmd.AddCommand(specCmd)
}


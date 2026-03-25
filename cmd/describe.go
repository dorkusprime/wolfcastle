package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

var describeCmd = &cobra.Command{
	Use:   "describe [address]",
	Short: "Show everything about a node",
	Long: `Displays the full state of a single node: type, status, tasks, audit
state, breadcrumbs, gaps, escalations, AARs, specs, and planning
history. Use --json for machine-readable output.

Examples:
  wolfcastle describe api/health
  wolfcastle describe --node api/health
  wolfcastle describe api/health --json`,
	Args: cobra.MaximumNArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return cmdutil.CompleteNodeAddresses(app)(cmd, args, toComplete)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := app.RequireIdentity(); err != nil {
			return err
		}

		addr, err := resolveDescribeAddress(cmd, args)
		if err != nil {
			return err
		}

		idx, err := app.State.ReadIndex()
		if err != nil {
			return err
		}

		entry, ok := idx.Nodes[addr]
		if !ok {
			return fmt.Errorf("node %q not found in index", addr)
		}

		ns, err := app.State.ReadNode(addr)
		if err != nil {
			return err
		}

		if app.JSON {
			return describeJSON(app, ns, entry, addr)
		}
		return describeHuman(app, ns, entry, addr)
	},
}

func init() {
	describeCmd.Flags().String("node", "", "Node address (alternative to positional argument)")
	rootCmd.AddCommand(describeCmd)
}

// resolveDescribeAddress extracts the node address from either a positional
// argument or the --node flag. Returns an error if both are provided or
// neither is provided.
func resolveDescribeAddress(cmd *cobra.Command, args []string) (string, error) {
	nodeFlag, _ := cmd.Flags().GetString("node")
	flagChanged := cmd.Flags().Changed("node")

	var positional string
	if len(args) > 0 {
		positional = args[0]
	}

	if flagChanged && positional != "" {
		return "", fmt.Errorf("specify the node address as a positional argument or with --node, not both")
	}
	if flagChanged {
		if nodeFlag == "" {
			return "", fmt.Errorf("--node value cannot be empty")
		}
		return nodeFlag, nil
	}
	if positional != "" {
		return positional, nil
	}
	return "", fmt.Errorf("node address required: provide as argument or with --node")
}

// describeJSON emits the full node state as a JSON envelope.
func describeJSON(app *cmdutil.App, ns *state.NodeState, entry state.IndexEntry, addr string) error {
	data := map[string]any{
		"node_state": ns,
		"index_entry": map[string]any{
			"name":               entry.Name,
			"type":               entry.Type,
			"state":              entry.State,
			"address":            entry.Address,
			"decomposition_depth": entry.DecompositionDepth,
			"parent":             entry.Parent,
			"children":           entry.Children,
			"archived":           entry.Archived,
		},
	}

	// Include the node's description markdown if it exists.
	if desc := readNodeDescription(app, addr); desc != "" {
		data["description_md"] = desc
	}

	output.Print(output.Ok("describe", data))
	return nil
}

// describeHuman renders the node in a readable format.
func describeHuman(app *cmdutil.App, ns *state.NodeState, entry state.IndexEntry, addr string) error {
	typeLabel := "leaf"
	if entry.Type == state.NodeOrchestrator {
		if entry.Parent == "" {
			typeLabel = "project"
		} else {
			typeLabel = "orchestrator"
		}
	}

	// Header
	output.PrintHuman("%s (%s, %s)", addr, typeLabel, ns.State)
	if ns.Scope != "" {
		output.PrintHuman("  %s", ns.Scope)
	}

	// Tasks (leaf nodes only)
	if len(ns.Tasks) > 0 {
		output.PrintHuman("")
		output.PrintHuman("Tasks:")
		for _, t := range ns.Tasks {
			glyph := taskGlyphPlain(t.State)
			label := t.Title
			if label == "" {
				label = t.Description
			}
			stateTag := fmt.Sprintf("[%s]", t.State)
			output.PrintHuman("  %s %-10s %s %s", glyph, t.ID, label, stateTag)
			if len(t.Deliverables) > 0 {
				output.PrintHuman("    deliverables: %s", strings.Join(t.Deliverables, ", "))
			}
			if t.Class != "" {
				output.PrintHuman("    class: %s", t.Class)
			}
			if t.BlockedReason != "" {
				output.PrintHuman("    blocked: %s", t.BlockedReason)
			}
			if len(t.References) > 0 {
				output.PrintHuman("    references: %s", strings.Join(t.References, ", "))
			}
		}
	}

	// Children (orchestrator nodes)
	if len(ns.Children) > 0 {
		output.PrintHuman("")
		output.PrintHuman("Children:")
		for _, c := range ns.Children {
			glyph := nodeGlyphPlain(c.State)
			output.PrintHuman("  %s %s [%s]", glyph, c.Address, c.State)
		}
	}

	// Audit
	output.PrintHuman("")
	output.PrintHuman("Audit:")
	output.PrintHuman("  status: %s", ns.Audit.Status)
	if ns.Audit.Scope != nil && ns.Audit.Scope.Description != "" {
		output.PrintHuman("  scope: %s", ns.Audit.Scope.Description)
	}

	openGaps, fixedGaps := countGaps(ns.Audit.Gaps)
	output.PrintHuman("  gaps: %d open, %d fixed", openGaps, fixedGaps)

	openEsc := countOpenEscalations(ns.Audit.Escalations)
	output.PrintHuman("  escalations: %d", openEsc)

	if ns.Audit.ResultSummary != "" {
		output.PrintHuman("  summary: %s", ns.Audit.ResultSummary)
	}

	// Breadcrumbs
	if len(ns.Audit.Breadcrumbs) > 0 {
		output.PrintHuman("")
		output.PrintHuman("Breadcrumbs:")
		for _, bc := range ns.Audit.Breadcrumbs {
			ts := bc.Timestamp.Format("2006-01-02 15:04")
			output.PrintHuman("  [%s] %s: %s", ts, bc.Task, bc.Text)
		}
	}

	// Gaps (detailed, only if any exist)
	if len(ns.Audit.Gaps) > 0 {
		output.PrintHuman("")
		output.PrintHuman("Gaps:")
		for _, g := range ns.Audit.Gaps {
			output.PrintHuman("  %s: %s [%s]", g.ID, g.Description, g.Status)
		}
	}

	// Escalations (detailed, only if any exist)
	if len(ns.Audit.Escalations) > 0 {
		output.PrintHuman("")
		output.PrintHuman("Escalations:")
		for _, e := range ns.Audit.Escalations {
			output.PrintHuman("  %s: %s (from %s) [%s]", e.ID, e.Description, e.SourceNode, e.Status)
		}
	}

	// Specs
	if len(ns.Specs) > 0 {
		output.PrintHuman("")
		output.PrintHuman("Specs:")
		for _, s := range ns.Specs {
			output.PrintHuman("  %s", s)
		}
	}

	// AARs
	if len(ns.AARs) > 0 {
		output.PrintHuman("")
		output.PrintHuman("AARs:")
		for id, aar := range ns.AARs {
			summary := aar.Objective
			if summary == "" {
				summary = aar.WhatHappened
			}
			if len(summary) > 80 {
				summary = summary[:77] + "..."
			}
			output.PrintHuman("  %s: %s", id, summary)
		}
	}

	// Planning (orchestrators only)
	if entry.Type == state.NodeOrchestrator {
		output.PrintHuman("")
		output.PrintHuman("Planning:")
		output.PrintHuman("  children: %d, replans: %d", len(ns.Children), ns.TotalReplans)
		if len(ns.PlanningHistory) > 0 {
			for _, p := range ns.PlanningHistory {
				ts := p.Timestamp.Format("2006-01-02 15:04")
				output.PrintHuman("  [%s] %s: %s", ts, p.Trigger, p.Summary)
			}
		}
		if len(ns.SuccessCriteria) > 0 {
			output.PrintHuman("  success criteria:")
			for _, c := range ns.SuccessCriteria {
				output.PrintHuman("    - %s", c)
			}
		}
	}

	return nil
}

// taskGlyphPlain returns an uncolored task status glyph.
func taskGlyphPlain(s state.NodeStatus) string {
	switch s {
	case state.StatusComplete:
		return "✓"
	case state.StatusInProgress:
		return "→"
	case state.StatusBlocked:
		return "✖"
	default:
		return "○"
	}
}

// nodeGlyphPlain returns an uncolored node status glyph.
func nodeGlyphPlain(s state.NodeStatus) string {
	switch s {
	case state.StatusComplete:
		return "●"
	case state.StatusInProgress:
		return "◐"
	case state.StatusBlocked:
		return "☢"
	default:
		return "◯"
	}
}

// countGaps returns (open, fixed) counts.
func countGaps(gaps []state.Gap) (int, int) {
	open, fixed := 0, 0
	for _, g := range gaps {
		switch g.Status {
		case state.GapOpen:
			open++
		case state.GapFixed:
			fixed++
		}
	}
	return open, fixed
}

// countOpenEscalations returns the number of open escalations.
func countOpenEscalations(escs []state.Escalation) int {
	n := 0
	for _, e := range escs {
		if e.Status == state.EscalationOpen {
			n++
		}
	}
	return n
}

// readNodeDescription reads the markdown description file from the node
// directory, if one exists. Returns empty string if not found.
func readNodeDescription(app *cmdutil.App, addr string) string {
	nodePath, err := app.State.NodePath(addr)
	if err != nil {
		return ""
	}
	nodeDir := filepath.Dir(nodePath)

	// Look for description.md or any .md file that isn't an audit report
	descPath := filepath.Join(nodeDir, "description.md")
	data, err := os.ReadFile(descPath)
	if err == nil {
		return string(data)
	}

	// Fall back to checking for a single markdown file
	entries, err := os.ReadDir(nodeDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".md") && !strings.HasPrefix(name, "audit-") && name != "state.json" {
			data, err := os.ReadFile(filepath.Join(nodeDir, name))
			if err == nil {
				return string(data)
			}
		}
	}
	return ""
}

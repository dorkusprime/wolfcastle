package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

var unblockCmd = &cobra.Command{
	Use:   "unblock",
	Short: "Interactive model-assisted unblock, or agent context dump",
	Long: `Model-assisted unblock with three tiers:

Tier 1 (simple flip): wolfcastle task unblock --node <path>
Tier 2 (interactive): wolfcastle unblock --node <path>
  Starts a multi-turn interactive chat with a model, pre-loaded with block context.
Tier 3 (agent dump):  wolfcastle unblock --agent --node <path>
  Outputs rich diagnostic context for an already-running interactive agent.

Examples:
  wolfcastle unblock --node my-project/task-1
  wolfcastle unblock --agent --node my-project/task-1`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := app.RequireResolver(); err != nil {
			return err
		}
		nodeFlag, _ := cmd.Flags().GetString("node")
		agentMode, _ := cmd.Flags().GetBool("agent")

		if nodeFlag == "" {
			return fmt.Errorf("--node is required")
		}

		// Parse task address
		nodeAddr, taskID, err := tree.SplitTaskAddress(nodeFlag)
		if err != nil {
			return fmt.Errorf("--node must be a task address (e.g. my-project/task-1): %w", err)
		}

		// Load node state
		addr, err := tree.ParseAddress(nodeAddr)
		if err != nil {
			return fmt.Errorf("invalid node address: %w", err)
		}
		statePath := filepath.Join(app.Resolver.ProjectsDir(), filepath.Join(addr.Parts...), "state.json")
		ns, err := state.LoadNodeState(statePath)
		if err != nil {
			return fmt.Errorf("loading node state: %w", err)
		}

		// Find the blocked task
		var blockedTask *state.Task
		for i := range ns.Tasks {
			if ns.Tasks[i].ID == taskID {
				blockedTask = &ns.Tasks[i]
				break
			}
		}
		if blockedTask == nil {
			return fmt.Errorf("task %s not found in %s", taskID, nodeAddr)
		}
		if blockedTask.State != state.StatusBlocked {
			return fmt.Errorf("task %s is %s, not blocked", taskID, blockedTask.State)
		}

		// Build diagnostic context
		diagnostic := buildDiagnostic(nodeAddr, taskID, ns, blockedTask)

		if agentMode {
			// Tier 3: dump context for an interactive agent
			output.PrintHuman("%s", diagnostic)
			output.PrintHuman("")
			output.PrintHuman("---")
			output.PrintHuman("")
			output.PrintHuman("When the issue is resolved, run:\n  wolfcastle task unblock --node %s", nodeFlag)
			return nil
		}

		// Tier 2: interactive multi-turn chat
		return runInteractiveUnblock(cmd.Context(), nodeFlag, diagnostic)
	},
}

func buildDiagnostic(nodeAddr, taskID string, ns *state.NodeState, task *state.Task) string {
	var b strings.Builder

	b.WriteString("# Unblock Diagnostic\n\n")
	b.WriteString(fmt.Sprintf("**Node:** %s\n", nodeAddr))
	b.WriteString(fmt.Sprintf("**Task:** %s\n", taskID))
	b.WriteString(fmt.Sprintf("**Description:** %s\n", task.Description))
	b.WriteString(fmt.Sprintf("**Block Reason:** %s\n", task.BlockedReason))
	b.WriteString(fmt.Sprintf("**Failure Count:** %d\n", task.FailureCount))
	b.WriteString(fmt.Sprintf("**Decomposition Depth:** %d\n\n", ns.DecompositionDepth))

	// Task breadcrumbs
	if len(task.Breadcrumbs) > 0 {
		b.WriteString("## Task Breadcrumbs\n\n")
		for _, bc := range task.Breadcrumbs {
			b.WriteString(fmt.Sprintf("- %s\n", bc))
		}
		b.WriteString("\n")
	}

	// Node audit breadcrumbs
	if len(ns.Audit.Breadcrumbs) > 0 {
		b.WriteString("## Audit Trail\n\n")
		for _, bc := range ns.Audit.Breadcrumbs {
			b.WriteString(fmt.Sprintf("- [%s] %s: %s\n",
				bc.Timestamp.Format("2006-01-02T15:04Z"), bc.Task, bc.Text))
		}
		b.WriteString("\n")
	}

	// Audit scope
	if ns.Audit.Scope != nil {
		b.WriteString("## Audit Scope\n\n")
		b.WriteString(ns.Audit.Scope.Description + "\n")
		if len(ns.Audit.Scope.Files) > 0 {
			b.WriteString("\n**Files:** " + strings.Join(ns.Audit.Scope.Files, ", ") + "\n")
		}
		if len(ns.Audit.Scope.Systems) > 0 {
			b.WriteString("**Systems:** " + strings.Join(ns.Audit.Scope.Systems, ", ") + "\n")
		}
		b.WriteString("\n")
	}

	// Linked specs
	if len(ns.Specs) > 0 {
		b.WriteString("## Linked Specs\n\n")
		for _, s := range ns.Specs {
			b.WriteString(fmt.Sprintf("- %s\n", s))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func runInteractiveUnblock(ctx context.Context, taskAddr string, diagnostic string) error {
	model, ok := app.Cfg.Models[app.Cfg.Unblock.Model]
	if !ok {
		return fmt.Errorf("unblock model %q not found", app.Cfg.Unblock.Model)
	}

	// Load unblock prompt from externalized template (falls back to inline text)
	unblockPreamble := loadUnblockPreamble()
	prompt := diagnostic + "\n---\n\n" + unblockPreamble + "\n" +
		fmt.Sprintf("When the issue is resolved, remind the user to run:\n  wolfcastle task unblock --node %s\n", taskAddr)

	output.PrintHuman("Starting interactive unblock session...")
	output.PrintHuman("(Type 'quit' or 'exit' to end the session)")
	output.PrintHuman("")

	// Simple multi-turn: invoke model, show response, get user input, repeat.
	// Keep a sliding window of conversation history to avoid unbounded growth.
	const maxConversationBytes = 100_000
	repoDir := filepath.Dir(app.WolfcastleDir)
	conversation := prompt
	scanner := bufio.NewScanner(os.Stdin)

	for {
		invokeCtx, cancel := context.WithTimeout(ctx, time.Duration(app.Cfg.Daemon.InvocationTimeoutSeconds)*time.Second)
		result, err := invoke.Invoke(invokeCtx, model, conversation, repoDir)
		cancel()

		if err != nil {
			return fmt.Errorf("model invocation failed: %w", err)
		}

		// Display model response
		output.PrintHuman("%s", result.Stdout)
		if result.Stderr != "" {
			output.PrintError("%s", result.Stderr)
		}

		// Get user input
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())

		if input == "quit" || input == "exit" || input == "" {
			output.PrintHuman("Session ended.")
			output.PrintHuman("\nWhen ready, run: wolfcastle task unblock --node %s", taskAddr)
			break
		}

		conversation += "\n\nUser: " + input + "\n\nAssistant: "

		// Trim conversation to avoid unbounded memory growth; keep the
		// original prompt (diagnostic context) and the most recent turns.
		if len(conversation) > maxConversationBytes {
			excess := len(conversation) - maxConversationBytes
			// Find a turn boundary after the excess point
			cutPoint := strings.Index(conversation[excess:], "\n\nUser: ")
			if cutPoint > 0 {
				conversation = prompt + "\n\n[Earlier conversation truncated]\n" + conversation[excess+cutPoint:]
			}
		}
	}

	return nil
}

// loadUnblockPreamble loads the unblock.md prompt via the three-tier
// resolution system, falling back to a hardcoded default.
func loadUnblockPreamble() string {
	if app.WolfcastleDir != "" {
		content, err := pipeline.ResolvePromptTemplate(app.WolfcastleDir, "unblock.md", nil)
		if err == nil {
			return content
		}
	}
	return "Help the user understand and resolve this blocked task. " +
		"Ask clarifying questions. Suggest potential fixes."
}

func init() {
	unblockCmd.Flags().String("node", "", "Blocked task address (required)")
	unblockCmd.MarkFlagRequired("node")
	unblockCmd.Flags().Bool("agent", false, "Output diagnostic context for an interactive agent")
	rootCmd.AddCommand(unblockCmd)
}

package cmd

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/output"
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

	// Build the unblock prompt
	prompt := diagnostic + "\n---\n\nHelp the user understand and resolve this blocked task. " +
		"Ask clarifying questions. Suggest potential fixes. " +
		"When the issue is resolved, remind the user to run:\n" +
		fmt.Sprintf("  wolfcastle task unblock --node %s\n", taskAddr)

	output.PrintHuman("Starting interactive unblock session...")
	output.PrintHuman("(Type 'quit', 'exit', or Ctrl+D to end the session)")
	output.PrintHuman("")

	// Set up readline for proper line editing, history, and terminal handling
	rl, err := readline.NewEx(&readline.Config{
		Prompt:      "wolfcastle> ",
		HistoryFile: "", // In-memory only (sessions are short-lived)
	})
	if err != nil {
		return fmt.Errorf("initializing readline: %w", err)
	}
	defer rl.Close()

	// Multi-turn: invoke model, show response, get user input, repeat.
	// Keep a sliding window of conversation history to avoid unbounded growth.
	const maxConversationBytes = 100_000
	repoDir := filepath.Dir(app.WolfcastleDir)
	conversation := prompt

	for {
		invokeCtx, cancel := context.WithTimeout(ctx, time.Duration(app.Cfg.Daemon.InvocationTimeoutSeconds)*time.Second)
		result, invokeErr := invoke.Invoke(invokeCtx, model, conversation, repoDir)
		cancel()

		if invokeErr != nil {
			return fmt.Errorf("model invocation failed: %w", invokeErr)
		}

		// Display model response
		output.PrintHuman("%s", result.Stdout)
		if result.Stderr != "" {
			output.PrintError("%s", result.Stderr)
		}
		fmt.Println()

		// Get user input via readline
		input, readErr := rl.Readline()
		if readErr != nil {
			if readErr == readline.ErrInterrupt {
				continue // Ctrl+C cancels current line, not the session
			}
			if readErr == io.EOF {
				break // Ctrl+D exits
			}
			break
		}
		input = strings.TrimSpace(input)

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

func init() {
	unblockCmd.Flags().String("node", "", "Blocked task address (required)")
	unblockCmd.MarkFlagRequired("node")
	unblockCmd.Flags().Bool("agent", false, "Output diagnostic context for an interactive agent")
	rootCmd.AddCommand(unblockCmd)
}

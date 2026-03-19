package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

// unblockCmd provides model-assisted unblocking and agent context dumps.
var unblockCmd = &cobra.Command{
	Use:   "unblock",
	Short: "Call in reinforcements for a blocked task",
	Long: `Three escalation tiers:

Tier 1 (reset):       wolfcastle task unblock --node <path>
Tier 2 (interactive): wolfcastle unblock --node <path>
  Opens a chat session with a model. Block context pre-loaded.
Tier 3 (agent dump):  wolfcastle unblock --agent --node <path>
  Dumps diagnostic context for an already-running agent.

Examples:
  wolfcastle unblock --node my-project/task-1
  wolfcastle unblock --agent --node my-project/task-1`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := app.RequireIdentity(); err != nil {
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
		ns, err := app.State.ReadNode(nodeAddr)
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
	fmt.Fprintf(&b, "**Node:** %s\n", nodeAddr)
	fmt.Fprintf(&b, "**Task:** %s\n", taskID)
	fmt.Fprintf(&b, "**Description:** %s\n", task.Description)
	fmt.Fprintf(&b, "**Block Reason:** %s\n", task.BlockedReason)
	fmt.Fprintf(&b, "**Failure Count:** %d\n", task.FailureCount)
	fmt.Fprintf(&b, "**Decomposition Depth:** %d\n\n", ns.DecompositionDepth)

	// Node audit breadcrumbs
	if len(ns.Audit.Breadcrumbs) > 0 {
		b.WriteString("## Audit Trail\n\n")
		for _, bc := range ns.Audit.Breadcrumbs {
			fmt.Fprintf(&b, "- [%s] %s: %s\n",
				bc.Timestamp.Format("2006-01-02T15:04Z"), bc.Task, bc.Text)
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
			fmt.Fprintf(&b, "- %s\n", s)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// unblockOpts allows injection of dependencies for testing. When nil or
// when individual fields are nil, production defaults are used.
type unblockOpts struct {
	invokeFn func(ctx context.Context, model config.ModelDef, prompt, workDir string) (*invoke.Result, error)
	stdin    io.ReadCloser
	stdout   io.Writer
}

func runInteractiveUnblock(ctx context.Context, taskAddr string, diagnostic string) error {
	return runInteractiveUnblockWith(ctx, taskAddr, diagnostic, nil)
}

func runInteractiveUnblockWith(ctx context.Context, taskAddr, diagnostic string, opts *unblockOpts) error {
	cfg, err := app.Config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	model, ok := cfg.Models[cfg.Unblock.Model]
	if !ok {
		return fmt.Errorf("unblock model %q not found", cfg.Unblock.Model)
	}

	invokeFn := invoke.Invoke
	var rlStdin io.ReadCloser
	var rlStdout io.Writer
	if opts != nil {
		if opts.invokeFn != nil {
			invokeFn = opts.invokeFn
		}
		rlStdin = opts.stdin
		rlStdout = opts.stdout
	}

	// Load unblock prompt from externalized template (falls back to inline text)
	unblockPreamble := loadUnblockPreamble()
	prompt := diagnostic + "\n---\n\n" + unblockPreamble + "\n" +
		fmt.Sprintf("When the issue is resolved, remind the user to run:\n  wolfcastle task unblock --node %s\n", taskAddr)

	output.PrintHuman("Engaging unblock session. Type 'quit', 'exit', or Ctrl+D to disengage.")
	output.PrintHuman("")

	// Set up readline for proper line editing, history, and terminal handling
	rlCfg := &readline.Config{
		Prompt:      "wolfcastle> ",
		HistoryFile: "", // In-memory only (sessions are short-lived)
	}
	if rlStdin != nil {
		rlCfg.Stdin = rlStdin
	}
	if rlStdout != nil {
		rlCfg.Stdout = rlStdout
		rlCfg.Stderr = rlStdout
	}
	rl, err := readline.NewEx(rlCfg)
	if err != nil {
		return fmt.Errorf("initializing readline: %w", err)
	}
	defer func() { _ = rl.Close() }()

	// Multi-turn: invoke model, show response, get user input, repeat.
	// Keep a sliding window of conversation history to avoid unbounded growth.
	const maxConversationBytes = 100_000
	repoDir := filepath.Dir(app.Config.Root())
	conversation := prompt

	for {
		output.PrintHuman("  thinking...")
		invokeCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Daemon.InvocationTimeoutSeconds)*time.Second)
		result, invokeErr := invokeFn(invokeCtx, model, conversation, repoDir)
		cancel()

		// Clear the "thinking..." line
		fmt.Print("\033[A\033[2K")

		if invokeErr != nil {
			return fmt.Errorf("model invocation failed: %w", invokeErr)
		}

		// Display model response, filtering noise for interactive use
		scanner := bufio.NewScanner(strings.NewReader(result.Stdout))
		for scanner.Scan() {
			formatted := invoke.FormatAssistantText(scanner.Text())
			if formatted == "" {
				continue
			}
			// Skip session init and result summaries (duplicate of assistant text)
			if formatted == "[session started]" || strings.HasPrefix(formatted, "[result] ") {
				continue
			}
			output.PrintHuman("%s", formatted)
		}
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

		if input == "quit" || input == "exit" {
			output.PrintHuman("Session closed.")
			output.PrintHuman("\nWhen ready: wolfcastle task unblock --node %s", taskAddr)
			break
		}
		if input == "" {
			continue
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
	if app.Config != nil {
		content, err := pipeline.ResolvePromptTemplate(app.Config.Root(), "unblock.md", nil)
		if err == nil {
			return content
		}
	}
	return `You are helping a developer resolve a blocked task in Wolfcastle.

Your job:
1. Read the diagnostic context above to understand why the task is blocked.
2. Explain the situation clearly and concisely.
3. Ask what the user wants to do. Offer concrete options when possible.
4. When the user makes a decision, execute it using wolfcastle CLI commands.

Rules:
- Use wolfcastle CLI commands (wolfcastle audit fix-gap, wolfcastle audit resolve-escalation, wolfcastle task unblock, etc.) to make changes. Never edit state.json or other files in .wolfcastle/ directly.
- Be concise. The user already knows their project; don't over-explain.
- When the issue is resolved, run the unblock command yourself rather than asking the user to do it.`
}

func init() {
	unblockCmd.Flags().String("node", "", "Blocked task address (required)")
	_ = unblockCmd.MarkFlagRequired("node")
	unblockCmd.Flags().Bool("agent", false, "Output diagnostic context for an interactive agent")
	rootCmd.AddCommand(unblockCmd)
}

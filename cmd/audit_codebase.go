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
	"github.com/dorkusprime/wolfcastle/internal/project"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

var auditCodebaseCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a codebase audit with discoverable scopes",
	Long: `Runs a model-driven codebase audit and presents findings for approval.
Approved findings become projects/tasks in the work tree.

Scopes are discovered from base/audits/, custom/audits/, and local/audits/.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		scopeFlag, _ := cmd.Flags().GetString("scope")
		listFlag, _ := cmd.Flags().GetBool("list")

		// Discover available scopes
		scopes, err := discoverScopes()
		if err != nil {
			return err
		}

		if listFlag {
			if jsonOutput {
				output.Print(output.Ok("audit_list", map[string]any{
					"scopes": scopes,
				}))
			} else {
				output.PrintHuman("Available audit scopes:")
				for _, s := range scopes {
					output.PrintHuman("  %-20s %s", s.ID, s.Description)
				}
			}
			return nil
		}

		// Filter scopes
		var selectedScopes []auditScope
		if scopeFlag != "" {
			requested := strings.Split(scopeFlag, ",")
			scopeMap := make(map[string]auditScope)
			for _, s := range scopes {
				scopeMap[s.ID] = s
			}
			for _, r := range requested {
				r = strings.TrimSpace(r)
				s, ok := scopeMap[r]
				if !ok {
					return fmt.Errorf("unknown scope %q — use --list to see available scopes", r)
				}
				selectedScopes = append(selectedScopes, s)
			}
		} else {
			selectedScopes = scopes
		}

		if len(selectedScopes) == 0 {
			return fmt.Errorf("no audit scopes found — add .md files to base/audits/, custom/audits/, or local/audits/")
		}

		return runCodebaseAudit(cmd.Context(), selectedScopes)
	},
}

type auditScope struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	PromptFile  string `json:"prompt_file"`
}

func discoverScopes() ([]auditScope, error) {
	var scopes []auditScope
	seen := make(map[string]bool)

	// Scan tiers in reverse priority (base first, local last overwrites)
	for _, tier := range []string{"base", "custom", "local"} {
		dir := filepath.Join(wolfcastleDir, tier, "audits")
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			id := strings.TrimSuffix(e.Name(), ".md")
			if seen[id] {
				// Higher tier replaces
				for i, s := range scopes {
					if s.ID == id {
						scopes[i].PromptFile = filepath.Join(dir, e.Name())
						break
					}
				}
				continue
			}
			seen[id] = true

			// Read first line for description
			desc := id
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err == nil {
				lines := strings.SplitN(string(data), "\n", 3)
				for _, l := range lines {
					l = strings.TrimSpace(l)
					if l != "" && !strings.HasPrefix(l, "#") {
						desc = l
						break
					}
				}
			}

			scopes = append(scopes, auditScope{
				ID:          id,
				Description: desc,
				PromptFile:  filepath.Join(dir, e.Name()),
			})
		}
	}
	return scopes, nil
}

func runCodebaseAudit(ctx context.Context, scopes []auditScope) error {
	model, ok := cfg.Models[cfg.Audit.Model]
	if !ok {
		return fmt.Errorf("audit model %q not found", cfg.Audit.Model)
	}

	// Build combined prompt from selected scopes
	var promptParts []string

	// Base audit prompt
	basePrompt, err := pipeline.ResolveFragment(wolfcastleDir, "prompts/"+cfg.Audit.PromptFile)
	if err == nil {
		promptParts = append(promptParts, basePrompt)
	}

	// Scope prompts
	for _, scope := range scopes {
		data, err := os.ReadFile(scope.PromptFile)
		if err != nil {
			continue
		}
		promptParts = append(promptParts, fmt.Sprintf("## Scope: %s\n\n%s", scope.ID, string(data)))
	}

	prompt := strings.Join(promptParts, "\n\n---\n\n")

	fmt.Printf("Running audit with %d scope(s): %s\n", len(scopes), scopeNames(scopes))

	repoDir := filepath.Dir(wolfcastleDir)
	invokeCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Daemon.InvocationTimeoutSeconds)*time.Second)
	defer cancel()

	result, err := invoke.Invoke(invokeCtx, model, prompt, repoDir)
	if err != nil {
		return fmt.Errorf("audit invocation failed: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("audit model exited with code %d: %s", result.ExitCode, result.Stderr)
	}

	// Present findings
	fmt.Println("\n=== Audit Findings ===")
	fmt.Println(result.Stdout)

	// Interactive approval
	return approveFindings(result.Stdout)
}

func approveFindings(findings string) error {
	fmt.Println("\n--- Approval ---")
	fmt.Println("Options:")
	fmt.Println("  [a] Approve all — create projects for every finding")
	fmt.Println("  [s] Skip — review later, don't create anything now")
	fmt.Println("  [m] Manual — use the commands below to create projects selectively")
	fmt.Print("\nChoice [a/s/m]: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil // EOF or error, just skip
	}
	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "a":
		return createProjectsFromFindings(findings)
	case "s":
		fmt.Println("Skipped. Findings are printed above for reference.")
		return nil
	case "m":
		fmt.Println("\nCreate projects manually:")
		fmt.Println("  wolfcastle project create \"<finding title>\"")
		fmt.Println("  wolfcastle task add --node <project> \"<specific fix>\"")
		return nil
	default:
		fmt.Println("Unrecognized choice. Skipping.")
		return nil
	}
}

func createProjectsFromFindings(findings string) error {
	// Parse findings — look for lines that look like titled findings
	// Common patterns: "## Finding:", "### Title", "1. **Title**"
	var titles []string
	for _, line := range strings.Split(findings, "\n") {
		line = strings.TrimSpace(line)
		// Match markdown headings
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			title := strings.TrimLeft(line, "# ")
			title = strings.TrimSpace(title)
			if title != "" && !strings.EqualFold(title, "Audit Findings") {
				titles = append(titles, title)
			}
			continue
		}
		// Match numbered bold items: "1. **Title**"
		if len(line) > 3 && line[0] >= '0' && line[0] <= '9' && strings.Contains(line, "**") {
			start := strings.Index(line, "**") + 2
			end := strings.Index(line[start:], "**")
			if end > 0 {
				titles = append(titles, line[start:start+end])
			}
		}
	}

	if len(titles) == 0 {
		fmt.Println("Could not parse individual findings from the output.")
		fmt.Println("Create projects manually using wolfcastle project create.")
		return nil
	}

	fmt.Printf("\nCreating %d projects from findings...\n", len(titles))

	idx, err := resolver.LoadRootIndex()
	if err != nil {
		return fmt.Errorf("loading root index: %w", err)
	}

	for _, title := range titles {
		slug := tree.ToSlug(title)
		if err := tree.ValidateSlug(slug); err != nil {
			fmt.Printf("  Skipped (invalid name): %s\n", title)
			continue
		}

		// Check for duplicate
		if _, exists := idx.Nodes[slug]; exists {
			fmt.Printf("  Skipped (exists): %s\n", slug)
			continue
		}

		ns, addr, err := project.CreateProject(idx, "", slug, title, state.NodeLeaf, nil)
		if err != nil {
			fmt.Printf("  Error creating %s: %v\n", title, err)
			continue
		}

		// Write node state
		addrParsed, _ := tree.ParseAddress(addr)
		nodeDir := filepath.Join(resolver.ProjectsDir(), filepath.Join(addrParsed.Parts...))
		os.MkdirAll(nodeDir, 0755)
		state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)

		// Write description
		descPath := filepath.Join(resolver.ProjectsDir(), slug+".md")
		os.WriteFile(descPath, []byte("# "+title+"\n\nAudit finding — see audit output for details.\n"), 0644)

		fmt.Printf("  Created: %s\n", addr)
	}

	// Save updated root index
	if err := state.SaveRootIndex(resolver.RootIndexPath(), idx); err != nil {
		return err
	}

	fmt.Println("\nProjects created. Add tasks with: wolfcastle task add --node <project> \"description\"")
	return nil
}

func scopeNames(scopes []auditScope) string {
	var names []string
	for _, s := range scopes {
		names = append(names, s.ID)
	}
	return strings.Join(names, ", ")
}

var auditListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available audit scopes",
	RunE: func(cmd *cobra.Command, args []string) error {
		scopes, err := discoverScopes()
		if err != nil {
			return err
		}
		if jsonOutput {
			output.Print(output.Ok("audit_list", map[string]any{
				"scopes": scopes,
			}))
		} else {
			if len(scopes) == 0 {
				output.PrintHuman("No audit scopes found")
			} else {
				output.PrintHuman("Available audit scopes:")
				for _, s := range scopes {
					output.PrintHuman("  %-20s %s", s.ID, s.Description)
				}
			}
		}
		return nil
	},
}

func init() {
	auditCodebaseCmd.Flags().String("scope", "", "Comma-separated scope IDs to run")
	auditCodebaseCmd.Flags().Bool("list", false, "List available scopes")
	auditCmd.AddCommand(auditCodebaseCmd)
	auditCmd.AddCommand(auditListCmd)
}

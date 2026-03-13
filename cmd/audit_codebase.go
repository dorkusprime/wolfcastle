package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
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
	fmt.Println("\nReview findings above. Create projects for these items using:")
	fmt.Println("  wolfcastle project create \"<finding>\"")
	fmt.Println("  wolfcastle task add --node <project> \"<specific fix>\"")

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

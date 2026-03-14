package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

// installCmd is the parent command for integration installs.
var installCmd = &cobra.Command{
	Use:   "install [target]",
	Short: "Install integrations",
	Long: `Install integrations with external tools. Currently supports: skill.

Examples:
  wolfcastle install skill`,
}

// installSkillCmd installs the Claude Code skill for Wolfcastle interaction.
var installSkillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Install Claude Code skill for Wolfcastle interaction",
	Long: `Creates a Claude Code skill in .claude/wolfcastle/ that enables
Claude Code users to interact with Wolfcastle natively from their session.

Uses symlinks where supported (macOS, Linux) for automatic updates.
Falls back to copying on Windows.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repoDir := filepath.Dir(app.WolfcastleDir)
		claudeDir := filepath.Join(repoDir, ".claude")
		skillDir := filepath.Join(claudeDir, "wolfcastle")

		// Source: base/skills/ in .wolfcastle
		sourceDir := filepath.Join(app.WolfcastleDir, "base", "skills")

		// Ensure source exists and has content
		if err := ensureSkillSource(sourceDir); err != nil {
			return err
		}

		// Create .claude directory if needed
		if err := os.MkdirAll(claudeDir, 0755); err != nil {
			return err
		}

		// Remove existing skill dir/symlink
		_ = os.RemoveAll(skillDir)

		// Try symlink first (works on macOS, Linux)
		if canSymlink() {
			if err := os.Symlink(sourceDir, skillDir); err != nil {
				// Fall back to copy
				return copyDir(sourceDir, skillDir)
			}
			if app.JSONOutput {
				output.Print(output.Ok("install_skill", map[string]string{
					"method": "symlink",
					"source": sourceDir,
					"target": skillDir,
				}))
			} else {
				output.PrintHuman("Installed Wolfcastle skill via symlink")
				output.PrintHuman("  %s → %s", skillDir, sourceDir)
				output.PrintHuman("  Updates automatically with 'wolfcastle update'")
			}
			return nil
		}

		// Copy mode
		if err := copyDir(sourceDir, skillDir); err != nil {
			return err
		}
		if app.JSONOutput {
			output.Print(output.Ok("install_skill", map[string]string{
				"method": "copy",
				"source": sourceDir,
				"target": skillDir,
			}))
		} else {
			output.PrintHuman("Installed Wolfcastle skill via copy")
			output.PrintHuman("  %s", skillDir)
			output.PrintHuman("  Re-run 'wolfcastle install skill' after updates")
		}
		return nil
	},
}

func ensureSkillSource(sourceDir string) error {
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return err
	}

	// Write skill definition if it doesn't exist
	skillFile := filepath.Join(sourceDir, "wolfcastle.md")
	if _, err := os.Stat(skillFile); os.IsNotExist(err) {
		content := `# Wolfcastle Skill

Use this skill to interact with the Wolfcastle project orchestrator.

## Available Commands

- ` + "`wolfcastle status`" + ` — Show project tree state
- ` + "`wolfcastle navigate`" + ` — Find the next actionable task
- ` + "`wolfcastle task add --node <path> \"description\"`" + ` — Add a task
- ` + "`wolfcastle task claim --node <path/task-id>`" + ` — Claim a task
- ` + "`wolfcastle task complete --node <path/task-id>`" + ` — Complete a task
- ` + "`wolfcastle task block --node <path/task-id> \"reason\"`" + ` — Block a task
- ` + "`wolfcastle task unblock --node <path/task-id>`" + ` — Unblock a task
- ` + "`wolfcastle audit breadcrumb --node <path> \"text\"`" + ` — Add breadcrumb
- ` + "`wolfcastle audit escalate --node <path> \"gap\"`" + ` — Escalate gap
- ` + "`wolfcastle project create [--node <parent>] \"name\"`" + ` — Create project
- ` + "`wolfcastle adr create \"title\"`" + ` — Create ADR
- ` + "`wolfcastle spec create [--node <path>] \"title\"`" + ` — Create spec
- ` + "`wolfcastle spec list [--node <path>]`" + ` — List specs
- ` + "`wolfcastle inbox add \"idea\"`" + ` — Add to inbox
- ` + "`wolfcastle archive add --node <path>`" + ` — Archive completed node
- ` + "`wolfcastle doctor`" + ` — Check structural integrity
- ` + "`wolfcastle follow`" + ` — Tail model output

All commands support ` + "`--json`" + ` for structured output.
`
		if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing skill definition: %w", err)
		}
	}
	return nil
}

func canSymlink() bool {
	return runtime.GOOS != "windows"
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return fmt.Errorf("reading %s: %w", srcPath, err)
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}

func init() {
	installCmd.AddCommand(installSkillCmd)
	rootCmd.AddCommand(installCmd)
}

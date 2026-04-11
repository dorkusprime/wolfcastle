package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/config"
	dmn "github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/logrender"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/project"
	"github.com/dorkusprime/wolfcastle/internal/signals"
	"github.com/dorkusprime/wolfcastle/internal/tierfs"
	"github.com/dorkusprime/wolfcastle/internal/validate"
	"github.com/spf13/cobra"
)

func newStartCmd(app *cmdutil.App) *cobra.Command {
	mode := modeSummary

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the daemon",
		Long: `Starts the execution loop. Wolfcastle picks up tasks, calls models,
validates results, and moves to the next target. Use --node to restrict
the carnage to a subtree. Use -d to run in the background.

Examples:
  wolfcastle start
  wolfcastle start --node auth-system
  wolfcastle start -d
  wolfcastle start --worktree feature-branch
  wolfcastle start --exit-when-done`,
		RunE: func(cmd *cobra.Command, args []string) error {
			output.PrintHuman("wolfcastle %s", app.Version)
			nodeScope, _ := cmd.Flags().GetString("node")
			background, _ := cmd.Flags().GetBool("daemon")
			worktreeBranch, _ := cmd.Flags().GetString("worktree")
			verbose, _ := cmd.Flags().GetBool("verbose")
			exitWhenDone, _ := cmd.Flags().GetBool("exit-when-done")

			if err := resolveInstance(cmd, app); err != nil {
				return err
			}

			// Worktree-aware tier regeneration: if .wolfcastle/ exists
			// but the base config is missing, regenerate gitignored tiers
			// before loading config.
			if err := ensureTiers(app.Config.Root()); err != nil {
				return fmt.Errorf("regenerating tiers: %w", err)
			}

			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}

			// Check whether the on-disk base config is behind the current version.
			if baseVer := app.Config.BaseVersion(); baseVer < config.CurrentVersion {
				if mode == modeJSON {
					output.PrintHuman(`{"warning":"config_version_behind","base_version":%d,"current_version":%d}`, baseVer, config.CurrentVersion)
				} else {
					output.PrintHuman("Configuration version %d is behind current version %d. Run `wolfcastle init --force` to update base config.", baseVer, config.CurrentVersion)
				}
			}

			// ADR-046: --verbose overrides daemon.log_level to debug.
			if verbose {
				cfg.Daemon.LogLevel = "debug"
			}

			if err := app.RequireIdentity(); err != nil {
				return err
			}

			// Find repo root (parent of .wolfcastle)
			repoDir := filepath.Dir(app.Config.Root())
			originalRepoDir := repoDir

			// Handle worktree mode
			var wtDir string
			if worktreeBranch != "" {
				var err error
				wtDir, err = createWorktree(repoDir, worktreeBranch)
				if err != nil {
					return fmt.Errorf("creating worktree: %w", err)
				}
				repoDir = wtDir
				output.PrintHuman("Operating in worktree: %s (branch: %s)", wtDir, worktreeBranch)
			}
			defer func() {
				if worktreeBranch != "" {
					cleanupWorktree(originalRepoDir, wtDir)
				}
			}()

			// Check for uncommitted changes before the daemon touches anything.
			// Direct commits will sweep in whatever is in the working tree,
			// so the user needs to know before we start.
			if cfg.Git.AutoCommit {
				if dirty, reason := checkDirtyTree(repoDir); dirty {
					output.PrintHuman("The working tree has uncommitted changes:\n%s", reason)
					output.PrintHuman("")
					output.PrintHuman("The daemon commits code and state together after each task.")
					output.PrintHuman("These changes will be included in the first commit.")
					output.PrintHuman("")
					output.PrintHuman("Options:")
					output.PrintHuman("  1. Commit or stash your changes, then restart")
					output.PrintHuman("  2. Disable auto-commit: wolfcastle config set git.auto_commit false")
					output.PrintHuman("  3. Continue anyway (your changes will be committed with the first task)")
					output.PrintHuman("")
					if !confirmContinue() {
						return fmt.Errorf("aborted: commit or stash changes first")
					}
				}
			}

			// Check instance registry for an already-running daemon in this worktree.
			if existing, resolveErr := instance.Resolve(repoDir); resolveErr == nil {
				return fmt.Errorf("already running (PID %d), use 'wolfcastle stop' first", existing.PID)
			}

			// Per-worktree lock (prevents duplicate daemons in the same worktree)
			if err := dmn.AcquireLock(app.Config.Root(), repoDir, ""); err != nil {
				return err
			}
			defer dmn.ReleaseLock(app.Config.Root())

			// Self-heal before validation: fix deterministic issues so
			// startup validation doesn't block on repairable state.
			// Omit wolfcastleDir so the fix pass skips daemon artifact
			// checks (PID file, stop file are intentional at startup).
			idx, idxErr := app.State.ReadIndex()
			if idxErr == nil {
				nodeLoader := validate.DefaultNodeLoader(app.State.Dir())
				healFixes, _, healErr := validate.FixWithVerification(
					app.State.Dir(),
					app.State.IndexPath(),
					nodeLoader,
				)
				if healErr != nil {
					output.PrintHuman("Pre-start self-heal error: %v", healErr)
				}
				if len(healFixes) > 0 {
					output.PrintHuman("Self-healed %s before startup:", output.Plural(len(healFixes), "issue", "issues"))
					for _, f := range healFixes {
						output.PrintHuman("  FIXED [%s] %s: %s", f.Category, f.Node, f.Description)
					}
					idx, idxErr = app.State.ReadIndex()
				}
			}

			// Startup validation gate: block on error-severity issues
			if idxErr == nil {
				engine := validate.NewEngine(app.State.Dir(), validate.DefaultNodeLoader(app.State.Dir()), dmn.NewRepository(app.Config.Root()))
				report := engine.ValidateStartup(idx)
				if report.HasErrors() {
					output.PrintHuman("Startup blocked. %d error(s):", report.Errors)
					for _, issue := range report.Issues {
						if issue.Severity == validate.SeverityError {
							output.PrintHuman("  ERROR [%s] %s: %s", issue.Category, issue.Node, issue.Description)
						}
					}
					return fmt.Errorf("validation errors. Run 'wolfcastle doctor --fix' to repair")
				}
				if report.Warnings > 0 {
					output.PrintHuman("%d warning(s). Proceeding anyway.", report.Warnings)
				}
			}

			if background {
				if worktreeBranch != "" {
					return fmt.Errorf("--worktree and --daemon cannot be combined: the background process cannot create worktrees")
				}
				return startBackground(app.Config.Root(), nodeScope, exitWhenDone, verbose, "")
			}

			d, err := dmn.New(cfg, app.Config.Root(), app.State, nodeScope, repoDir)
			if err != nil {
				return err
			}
			d.ExitWhenDone = exitWhenDone

			ctx, cancel := signal.NotifyContext(context.Background(), signals.Shutdown...)
			defer cancel()

			// Start the renderer goroutine. It tails the log directory
			// for new NDJSON files and renders them to stdout using
			// whichever output mode was selected (default: summary).
			logDir := filepath.Join(app.Config.Root(), tierfs.SystemPrefix, "logs")
			reader := logrender.NewFollowReader(logDir, 200*time.Millisecond)
			records := reader.Records(ctx)
			renderDone := make(chan struct{})
			w := &output.SpinnerWriter{W: os.Stdout}
			go func() {
				defer close(renderDone)
				switch mode {
				case modeSummary:
					sr := logrender.NewSummaryRenderer(w)
					sr.Follow(ctx, records)
				case modeThoughts:
					tr := logrender.NewThoughtsRenderer(w)
					tr.Render(ctx, records)
				case modeInterleaved:
					ir := logrender.NewInterleavedRenderer(w)
					ir.Render(ctx, records)
				case modeJSON:
					for rec := range records {
						raw, err := json.Marshal(rec.Raw)
						if err != nil {
							continue
						}
						_, _ = w.Write(raw)
						_, _ = w.Write([]byte{'\n'})
					}
				}
			}()

			runErr := d.RunWithSupervisor(ctx)
			cancel()
			<-renderDone

			return runErr
		},
	}

	registerModeFlags(cmd, &mode)

	return cmd
}

// startBackground launches the daemon as a detached background process.
// The child re-execs into the hidden _daemon-run command, which skips
// all interactive checks (dirty tree, validation prompts). The foreground
// start command has already validated everything.
// executablePath is the binary to re-exec; pass "" to use os.Executable().
func startBackground(wolfcastleDir, nodeScope string, exitWhenDone bool, verbose bool, executablePath string) error {
	if executablePath == "" {
		var err error
		executablePath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("finding executable: %w", err)
		}
	}

	cmdArgs := []string{"_daemon-run"}
	if nodeScope != "" {
		cmdArgs = append(cmdArgs, "--node", nodeScope)
	}
	if exitWhenDone {
		cmdArgs = append(cmdArgs, "--exit-when-done")
	}
	if verbose {
		cmdArgs = append(cmdArgs, "--verbose")
	}

	proc := exec.Command(executablePath, cmdArgs...)
	proc.Stdin = nil
	proc.Dir = filepath.Dir(wolfcastleDir)

	// Redirect stdout/stderr to a daemon log file so startup errors
	// aren't silently lost.
	daemonLog := filepath.Join(wolfcastleDir, tierfs.SystemPrefix, "daemon.log")
	logFile, err := os.OpenFile(daemonLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("creating daemon log: %w", err)
	}
	proc.Stdout = logFile
	proc.Stderr = logFile

	if err := proc.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("starting background process: %w", err)
	}

	output.PrintHuman("Daemon deployed (PID %d)", proc.Process.Pid)
	output.PrintHuman("  wolfcastle log -f    Watch the operation")
	output.PrintHuman("  wolfcastle stop      Stand down")

	// Detach. The child process writes its own PID file.
	_ = proc.Process.Release()
	return nil
}

func cleanupWorktree(repoDir, wtDir string) {
	removeCmd := exec.Command("git", "worktree", "remove", wtDir)
	removeCmd.Dir = repoDir
	if out, err := removeCmd.CombinedOutput(); err != nil {
		output.PrintHuman("Could not remove worktree %s: %s (%v)", wtDir, string(out), err)
	} else {
		output.PrintHuman("Cleaned up worktree: %s", wtDir)
	}
}

func createWorktree(repoDir, branch string) (string, error) {
	wtDir := filepath.Join(filepath.Dir(repoDir), ".wolfcastle", "worktrees", branch)

	// Check if branch exists
	checkCmd := exec.Command("git", "rev-parse", "--verify", branch)
	checkCmd.Dir = repoDir
	branchExists := checkCmd.Run() == nil

	var gitCmd *exec.Cmd
	if branchExists {
		gitCmd = exec.Command("git", "worktree", "add", wtDir, branch)
	} else {
		gitCmd = exec.Command("git", "worktree", "add", "-b", branch, wtDir)
	}
	gitCmd.Dir = repoDir

	if out, err := gitCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %s: %w", string(out), err)
	}

	return wtDir, nil
}

// checkDirtyTree returns true if the git working tree has uncommitted
// changes (staged, unstaged, or untracked non-ignored files).
func checkDirtyTree(repoDir string) (bool, string) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return false, "" // can't check, proceed
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return false, ""
	}
	// Summarize: count staged, modified, untracked
	lines := strings.Split(trimmed, "\n")
	staged, modified, untracked := 0, 0, 0
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}
		if line[0:2] == "??" {
			untracked++
		} else if line[0] != ' ' {
			staged++
		} else {
			modified++
		}
	}
	var parts []string
	if staged > 0 {
		parts = append(parts, fmt.Sprintf("  %d staged", staged))
	}
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("  %d modified", modified))
	}
	if untracked > 0 {
		parts = append(parts, fmt.Sprintf("  %d untracked", untracked))
	}
	return true, strings.Join(parts, "\n")
}

// confirmContinue prompts the user for y/n confirmation on stdin.
// Returns true if the user types "y" or "yes". Returns false on
// anything else, EOF, or if stdin is not a terminal.
func confirmContinue() bool {
	if !output.IsTerminal() {
		return false
	}
	fmt.Print("Continue? [y/N] ")
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return false
	}
	return response == "y" || response == "Y" || response == "yes"
}

// ensureTiers checks whether the .wolfcastle directory has its gitignored
// tiers in place. When .wolfcastle/ exists (tracked content present) but
// system/base/config.json is missing, the base and local tiers are
// regenerated from embedded templates and a fresh identity detection.
// This handles the common worktree case: git checkout brings .wolfcastle/
// with tracked files, but gitignored tiers (base, local, logs) need
// rebuilding before the daemon can start.
func ensureTiers(wcDir string) error {
	baseCfg := filepath.Join(wcDir, tierfs.SystemPrefix, tierfs.TierNames[0], "config.json")
	if _, err := os.Stat(baseCfg); err == nil {
		return nil // base tier present, nothing to do
	}

	// .wolfcastle/ itself must exist; if it doesn't, the user needs
	// to run wolfcastle init, not a tier regeneration.
	if _, err := os.Stat(wcDir); err != nil {
		return nil
	}

	output.PrintHuman("Regenerating missing tiers in %s", wcDir)

	cfgRepo := config.NewRepository(wcDir)
	promptRepo := pipeline.NewPromptRepository(wcDir)
	svc := project.NewScaffoldService(cfgRepo, promptRepo, nil, wcDir)

	if err := svc.Reinit(); err != nil {
		return fmt.Errorf("tier regeneration: %w", err)
	}

	// Ensure logs and projects directories exist.
	for _, dir := range []string{
		filepath.Join(wcDir, tierfs.SystemPrefix, "logs"),
		filepath.Join(wcDir, tierfs.SystemPrefix, "projects"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// Create the namespace projects directory for the detected identity.
	identity := config.DetectIdentity()
	nsDir := identity.ProjectsDir(wcDir)
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		return fmt.Errorf("creating namespace directory: %w", err)
	}

	output.PrintHuman("Tiers regenerated. Identity: %s", identity.Namespace)
	return nil
}

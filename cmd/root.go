package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

var (
	jsonOutput    bool
	wolfcastleDir string
	cfg           *config.Config
	resolver      *tree.Resolver
)

var rootCmd = &cobra.Command{
	Use:   "wolfcastle",
	Short: "Model-agnostic autonomous project orchestrator",
	Long: `Wolfcastle breaks complex work into a persistent tree of projects,
sub-projects, and tasks, then executes them through configurable
multi-model pipelines.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for commands that don't need it
		switch cmd.Name() {
		case "init", "version", "help":
			return nil
		}
		return loadConfig()
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func findWolfcastleDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, ".wolfcastle")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no .wolfcastle directory found — run 'wolfcastle init' first")
}

func loadConfig() error {
	var err error
	wolfcastleDir, err = findWolfcastleDir()
	if err != nil {
		return err
	}
	cfg, err = config.Load(wolfcastleDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	// Don't validate identity for commands that don't need it
	resolver, err = tree.NewResolver(wolfcastleDir, cfg)
	if err != nil {
		// Not fatal for all commands
		resolver = nil
	}
	return nil
}

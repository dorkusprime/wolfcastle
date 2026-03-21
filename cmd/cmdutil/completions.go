package cmdutil

import (
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

// CompleteNodeAddresses returns a completion function that provides all
// node addresses from the root index as shell completion candidates.
// The returned closure captures the App so it can lazily load config
// when completions are invoked.
func CompleteNodeAddresses(app *App) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		idx, err := loadRootIndexForCompletion(app)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var addrs []string
		for addr := range idx.Nodes {
			addrs = append(addrs, addr)
		}
		return addrs, cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteTaskAddresses returns a completion function that provides
// node/task-id addresses for all tasks in all leaf nodes. Used for
// commands that operate on tasks (claim, complete, block, unblock).
func CompleteTaskAddresses(app *App) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		idx, err := loadRootIndexForCompletion(app)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		store, err := storeForCompletion(app)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var addrs []string
		for addr, entry := range idx.Nodes {
			// Include node addresses
			addrs = append(addrs, addr)

			// For leaf nodes, also include task addresses
			if entry.Type == state.NodeLeaf {
				parsed, err := tree.ParseAddress(addr)
				if err != nil {
					continue
				}
				statePath := filepath.Join(store.Dir(), filepath.Join(parsed.Parts...), "state.json")
				ns, err := state.LoadNodeState(statePath)
				if err != nil {
					continue
				}
				for _, task := range ns.Tasks {
					addrs = append(addrs, addr+"/"+task.ID)
				}
			}
		}
		return addrs, cobra.ShellCompDirectiveNoFileComp
	}
}

// loadRootIndexForCompletion attempts to load the root index for
// completion. Returns an error silently if the environment is not
// configured.
func loadRootIndexForCompletion(app *App) (*state.RootIndex, error) {
	if app.State != nil {
		return app.State.ReadIndex()
	}
	if err := app.LoadConfig(); err != nil {
		return nil, err
	}
	if app.State == nil {
		return nil, &ConfigNotReady{}
	}
	return app.State.ReadIndex()
}

// storeForCompletion returns the state store, loading config if needed.
func storeForCompletion(app *App) (*state.Store, error) {
	if app.State != nil {
		return app.State, nil
	}
	if err := app.LoadConfig(); err != nil {
		return nil, err
	}
	if app.State == nil {
		return nil, &ConfigNotReady{}
	}
	return app.State, nil
}

// ConfigNotReady is a sentinel error for when config/identity isn't available.
type ConfigNotReady struct{}

func (e *ConfigNotReady) Error() string { return "config not ready" }

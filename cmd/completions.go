package cmd

import (
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

// completeNodeAddresses returns all node addresses from the root index as
// shell completion candidates for --node flags.
func completeNodeAddresses(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	idx, err := loadRootIndexForCompletion()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var addrs []string
	for addr := range idx.Nodes {
		addrs = append(addrs, addr)
	}
	return addrs, cobra.ShellCompDirectiveNoFileComp
}

// completeTaskAddresses returns node/task-id addresses for all tasks in all
// leaf nodes. Used for commands that operate on tasks (claim, complete, block, unblock).
func completeTaskAddresses(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	idx, err := loadRootIndexForCompletion()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	res, err := resolverForCompletion()
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
			statePath := filepath.Join(res.ProjectsDir(), filepath.Join(parsed.Parts...), "state.json")
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

// loadRootIndexForCompletion attempts to load the root index for completion.
// Returns nil silently if the environment is not configured.
func loadRootIndexForCompletion() (*state.RootIndex, error) {
	if resolver != nil {
		return resolver.LoadRootIndex()
	}
	// Try to set up if not yet loaded
	if err := loadConfig(); err != nil {
		return nil, err
	}
	if resolver == nil {
		return nil, &configNotReady{}
	}
	return resolver.LoadRootIndex()
}

// resolverForCompletion returns the resolver, loading config if needed.
func resolverForCompletion() (*tree.Resolver, error) {
	if resolver != nil {
		return resolver, nil
	}
	if err := loadConfig(); err != nil {
		return nil, err
	}
	if resolver == nil {
		return nil, &configNotReady{}
	}
	return resolver, nil
}

// configNotReady is a sentinel error for when config/identity isn't available.
type configNotReady struct{}

func (e *configNotReady) Error() string { return "config not ready" }

func init() {
	// Node address completions for commands that take --node with node addresses
	projectCreateCmd.RegisterFlagCompletionFunc("node", completeNodeAddresses)
	navigateCmd.RegisterFlagCompletionFunc("node", completeNodeAddresses)
	statusCmd.RegisterFlagCompletionFunc("node", completeNodeAddresses)
	startCmd.RegisterFlagCompletionFunc("node", completeNodeAddresses)
	taskAddCmd.RegisterFlagCompletionFunc("node", completeNodeAddresses)
	specCreateCmd.RegisterFlagCompletionFunc("node", completeNodeAddresses)
	specLinkCmd.RegisterFlagCompletionFunc("node", completeNodeAddresses)
	specListCmd.RegisterFlagCompletionFunc("node", completeNodeAddresses)

	// Task address completions for commands that operate on tasks
	taskClaimCmd.RegisterFlagCompletionFunc("node", completeTaskAddresses)
	taskCompleteCmd.RegisterFlagCompletionFunc("node", completeTaskAddresses)
	taskBlockCmd.RegisterFlagCompletionFunc("node", completeTaskAddresses)
	taskUnblockCmd.RegisterFlagCompletionFunc("node", completeTaskAddresses)
	unblockCmd.RegisterFlagCompletionFunc("node", completeTaskAddresses)
}

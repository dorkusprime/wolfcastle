# Delete Operation

Implement the delete (permanent removal) operation for archived nodes: remove .archive/{addr}/ directory tree, remove all subtree entries from RootIndex.Nodes, remove root address from ArchivedRoot. CLI command wolfcastle archive delete --node <addr> --confirm (the --confirm flag is required to prevent accidents). Markdown rollup entries in .wolfcastle/archive/ are never deleted. Must include >90% test coverage.

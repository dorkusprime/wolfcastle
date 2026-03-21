# Restore Operation

Implement the restore (un-archive) operation: move archived state directories back to active storage, clear Archived/ArchivedAt flags on all subtree IndexEntry records, move the root address from ArchivedRoot back to Root. Expose as wolfcastle archive restore --node <addr>. The operation makes no state-machine transitions; the node returns as complete. Must include >90% test coverage for the restore function, CLI command, and edge cases (node not archived, node not in ArchivedRoot, partial restore recovery).

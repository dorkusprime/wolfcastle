# Daemon Commit Logic

Refactor the daemon's git commit behavior so it commits deterministically after every task iteration. Add success-path commits alongside the existing failure-path autoCommitPartialWork. Implement staging area preservation so the user's git index is not clobbered by daemon commits.

# Daemon Commit Logic

Refactor autoCommitPartialWork into a general-purpose daemon commit function that handles both success and failure paths. Add success-path commit call in iteration.go after task completion. Implement staging area preservation so the user's manually staged changes survive daemon commits. Wire the new config fields (commit_on_success, commit_on_failure, commit_state) to control behavior.

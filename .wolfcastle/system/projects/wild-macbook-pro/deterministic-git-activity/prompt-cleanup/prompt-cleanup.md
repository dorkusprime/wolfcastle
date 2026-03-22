# Prompt Cleanup

Remove all git instructions from agent-facing prompts. The agent should never run git add, git commit, or any git command. Phase H (Commit) must be entirely removed from execute.md. The 'Commit before signaling completion' rule must be removed. AGENTS.md and any docs/agents/ files must be updated to explicitly state that agents do not touch git.

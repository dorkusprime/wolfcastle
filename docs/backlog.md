# User items as they come up. Don't process these until directed.

- We should make sure the models are proactive in disaggregating bigger tasks - we are trying to keep them at <=50% of their compaction context window levels. They should decompose as soon as they see that a task could be too big for their ~half of their limits.
- For this project (not wolfcastle itself, but the building of wolfcastle) we need to stop pushing directly to main. It's hell on codecov. I'd like to protect `main` to prevent that, but don't want to cause a bunch of failures on your end (or your subagents'). Please evaluate our options and get back to me.
- Make sure that we have tests (integration, smoke, and/or unit) to ensure for expected behavior as well (as acceptable UX paths, in our VOICE) under various edge/corner case scenarios:
  - malformed JSON
  - interruptions tasks (self-healing)
  - malformed coding agent responses
  - coding agents not reliably using the expected tools
  - coding agent command failure
  - coding agent authentication issues
  - Warning-level issues with the coding agent commands
  - retry logic
  - anything else you can think of

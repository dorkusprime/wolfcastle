# Knowledge CLI Commands

Implement the four wolfcastle knowledge subcommands: add, show, edit, and prune. All commands operate on the current namespace's knowledge file at .wolfcastle/docs/knowledge/<namespace>.md. The 'add' command enforces the token budget from config before appending. Commands follow existing cobra patterns in cmd/.

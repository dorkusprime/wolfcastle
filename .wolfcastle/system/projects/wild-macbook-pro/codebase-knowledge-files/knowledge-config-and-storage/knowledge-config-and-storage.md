# Knowledge Config and Storage

Add KnowledgeConfig to the config system (max_tokens field with default 2000), create internal/knowledge/ package with utilities for reading, writing, and token-counting knowledge files in .wolfcastle/docs/knowledge/<namespace>.md. This is the foundation that CLI commands, ContextBuilder injection, and daemon maintenance all build on.

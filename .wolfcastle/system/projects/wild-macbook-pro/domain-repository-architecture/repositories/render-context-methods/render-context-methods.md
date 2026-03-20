# Render Context Methods

Move context rendering logic onto domain types in internal/state. The existing Task.RenderContext(nodeAddr, nodeDir) is refactored to Task.RenderContext() with no parameters. New AuditState.RenderContext() and NodeState.RenderContext(taskID) methods are added. These render methods are independent of all repositories and can proceed in parallel with repository implementation.

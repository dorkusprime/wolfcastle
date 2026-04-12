# TUI v0.1 Spec (Split)

The full spec lives at `../2026-04-06T01-00Z-tui-v01.md` (2,179 lines). These files are the same content split for context-efficient build sessions.

## Reading Order

1. **`common.md`** (1,027 lines): read first. Contains summary, dependencies, entry states, layout, data sources, real-time update strategy, Bubbletea architecture, performance, package structure, voice, and error handling.

2. **`phase1.md`** through **`phase5.md`**: read the phase you're implementing. Each phase document includes models, messages, key bindings, rendering, and test cases.

## For Build Sessions

Load `common.md` + the phase file you're building. That's all the context you need. Don't load the full spec or other phase files.

| File | Lines | Content |
|------|-------|---------|
| `common.md` | 1,027 | Shared foundation + reference sections |
| `phase1.md` | 403 | App shell, layout, tree, dashboard, search, help, clipboard |
| `phase2.md` | 326 | Log stream, node detail, task detail |
| `phase3.md` | 161 | Instance switcher, daemon start/stop |
| `phase4.md` | 139 | Inbox view and add |
| `phase5.md` | 148 | Notification toasts |

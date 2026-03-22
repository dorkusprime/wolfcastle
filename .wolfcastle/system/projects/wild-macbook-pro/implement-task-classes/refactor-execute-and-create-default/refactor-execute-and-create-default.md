# Refactor Execute and Create Default

Extract all behavioral guidance (Phase C coding conventions, Phase D validation rules) from execute.md into classes/default.md. After extraction, execute.md retains only structural concerns: phase ordering, boundaries, terminal markers, commit protocol, AAR writing. Wire default.md into ContextBuilder as fallback when a task has no Class field or when class resolution fails.

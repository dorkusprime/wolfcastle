# Enrich Commit Messages with Task Metadata

Refactor commitAfterIteration in internal/daemon/iteration.go to build commit messages from task metadata instead of opaque task IDs. The subject line becomes '{prefix}: {title}' (or just '{title}' if prefix is empty). The commit body includes task ID, class, deliverables, and the latest breadcrumb. For failures, append '(attempt N)' to the subject and include the failure type. Update both call sites to pass the required metadata from the NodeState that is already loaded in scope.

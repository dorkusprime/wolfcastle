# Artifact Templates

Create .tmpl files for artifact-generated files (ADR, spec, audit-task, task markdown) and migrate each call site (cmd/adr_create.go, cmd/spec.go, internal/project/project_create.go, cmd/task/add.go) to use RenderToFile with typed context structs. These templates use Go template variables for dynamic content.

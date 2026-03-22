# Call-Site Migration

Migrate all file-generation call sites from inline string construction to the template system. Create .tmpl files under internal/project/templates/ for each generated file, then replace strings.Builder/fmt.Sprintf/os.WriteFile chains at each call site with RenderToFile calls. Verify with snapshot tests that output is byte-for-byte identical before and after. Two sub-concerns: scaffold files (gitignore, READMEs) and artifact files (ADR, spec, audit-task, task markdown).

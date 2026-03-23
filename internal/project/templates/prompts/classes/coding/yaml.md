# YAML

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Formatting

Prefer 2-space indentation. YAML forbids tabs entirely; a tab character anywhere in indentation is a parse error. Configure your editor to insert spaces for YAML files.

Prefer consistent quoting within a file. If the project quotes all strings, quote all strings. If it quotes only when necessary (values that look like booleans, numbers, or contain special characters), follow that convention. When starting fresh, quote only when necessary and be disciplined about knowing when it is necessary.

Prefer a `---` document start marker at the top of each file. It is technically optional for single-document files but makes the file unambiguously YAML and is required for multi-document streams.

Prefer blank lines between top-level sections for readability. Do not use blank lines within tightly coupled nested structures where the visual grouping already communicates the relationship.

## The Boolean Problem

YAML 1.1 interprets a surprising number of bare words as booleans. `yes`, `no`, `on`, `off`, `y`, `n`, `true`, `false` (and their capitalized variants) all become boolean values. This is the infamous "Norway problem": the country code `NO` silently becomes `false`.

YAML 1.2 restricts booleans to only `true` and `false` (lowercase). However, many widely-used parsers still default to YAML 1.1 behavior, including PyYAML and older versions of the Go `yaml` package.

Prefer quoting any string value that could be misinterpreted. Quote country codes, two-letter abbreviations, version strings like `3.10` (which parses as the float `3.1`), and anything resembling a boolean. When in doubt, quote it.

## Type Coercion Gotchas

Unquoted `3.10` becomes the float `3.1`, not the string `"3.10"`. This silently breaks Python version specifiers, locale codes, and version constraints. Always quote version-like strings.

Unquoted `0o777` is an octal integer (511 in decimal). Unquoted `0x1A` is hexadecimal. If you mean the string, quote it.

Unquoted timestamps in ISO 8601 format (`2026-03-23`) parse as date objects in some YAML libraries. Quote date strings when you want them to remain strings.

Unquoted `null`, `Null`, `~`, and empty values all parse as null. If you need the literal string `"null"`, quote it.

## Multiline Strings

Prefer literal block scalars (`|`) when you want newlines preserved exactly as written. This is the right choice for shell scripts, SQL, and any content where line breaks are significant.

Prefer folded block scalars (`>`) when you want a long paragraph that happens to wrap in the YAML source. Folding replaces single newlines with spaces, turning wrapped lines into flowing text.

Prefer the strip chomping indicator (`|-` or `>-`) to remove the trailing newline from a block scalar. The default (clip) keeps exactly one trailing newline; strip removes it entirely. Keep (`|+` or `>+`) preserves all trailing newlines.

Avoid plain (unquoted) multiline strings. They work but their behavior with leading spaces, special characters, and comment-like sequences is confusing and fragile.

## Anchors and Aliases

Prefer anchors (`&name`) and aliases (`*name`) for eliminating repetition in configuration files. Define the anchor on the canonical definition and reference it with aliases elsewhere.

Prefer merge keys (`<<: *name`) for inheriting a set of key-value pairs into a mapping. Be aware that merge key support is not part of the YAML 1.2 spec and some strict parsers reject it. It remains widely supported in practice (Docker Compose, CI configs, Kubernetes).

Prefer a clearly named top-level section (`.defaults`, `x-common`, or similar) for anchor definitions. Anchors defined inline within deeply nested structures are hard to find and maintain.

## Schema Validation

Prefer validating YAML files against a schema in CI. Tools like `yamllint` catch syntax issues and style violations. JSON Schema can validate the structure after parsing (YAML parses to the same data model as JSON).

Prefer `yamllint` with a `.yamllint.yml` configuration for consistent style enforcement: indentation width, line length, truthy value handling, and quoting rules.

Prefer explicit schemas for CI/CD configuration files (GitHub Actions, GitLab CI, Docker Compose). These tools publish JSON schemas; editors like VS Code can validate against them with the YAML language server.

## Common Pitfalls

Indentation errors in YAML are silent when they produce valid but wrong structure. Misindenting a key by two spaces can attach it to the wrong parent mapping without any parse error. Careful review and schema validation are the only defenses.

Colons in values require quoting. `message: error: file not found` is a parse error because the second colon starts a new mapping. Use `message: "error: file not found"`.

The `#` character starts a comment anywhere it appears after whitespace. A value like `channel: #general` loses everything after `#`. Quote it: `channel: "#general"`.

Large YAML files with many aliases can trigger quadratic parsing behavior in some implementations (the "billion laughs" attack). Prefer limiting alias depth and using libraries that cap expansion.

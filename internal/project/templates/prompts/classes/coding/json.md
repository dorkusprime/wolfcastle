# JSON

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Formatting

Prefer 2-space indentation for human-readable JSON files. Configuration files, API fixtures, and anything checked into version control should be formatted consistently. Use your editor's formatter or `jq .` to normalize style.

Prefer a trailing newline at the end of JSON files. Most text editors and POSIX tools expect it. Missing trailing newlines cause noisy diffs and confuse line-counting tools.

Prefer sorting keys alphabetically in configuration files (`package.json` dependencies, `tsconfig.json` compiler options). Sorted keys reduce merge conflicts and make scanning easier. For data files where insertion order matters, preserve the natural order.

## Syntax Rules

JSON has no comments. Do not attempt `//` or `/* */` notation in `.json` files; parsers will reject them. If you need comments, use JSONC (`.jsonc`, supported by VS Code and TypeScript configs) or JSON5 (`.json5`). Both allow comments and trailing commas, but neither is valid JSON. Know which format your toolchain expects.

JSON has no trailing commas. A trailing comma after the last element in an array or object is a parse error in strict JSON. This is the single most common cause of "unexpected token" failures when hand-editing JSON. Lint before committing.

All keys must be double-quoted strings. All string values must use double quotes, not single quotes. Numbers, booleans (`true`, `false`), and `null` are unquoted.

## Schema Validation

Prefer JSON Schema for validating structure at API boundaries, configuration loading, and file ingestion. Define schemas alongside the code that consumes them. A schema that lives in a wiki drifts from reality within weeks.

Prefer the `2020-12` draft of JSON Schema unless the project's tooling requires an older draft. The `2020-12` draft adds `$dynamicRef`, `prefixItems`, and cleaner vocabulary support.

Prefer `"additionalProperties": false` in schemas for config files and internal APIs. Rejecting unknown keys catches typos and prevents silent misconfiguration. For public-facing APIs where forward compatibility matters, leave it open or use `"unevaluatedProperties": false` with composition.

Prefer `"required"` arrays that list every mandatory field. Omitting `required` makes every field optional by default, which hides missing data until runtime.

## JSON Lines (JSONL)

Prefer JSONL (`.jsonl`) for streaming, logging, and large datasets. Each line is a self-contained JSON object terminated by `\n`. No wrapping array, no commas between records.

Prefer JSONL over JSON arrays when records will be appended incrementally, processed line-by-line, or streamed over a network. A JSON array requires reading the entire file to parse; JSONL allows constant-memory line-by-line processing.

Prefer `jq -c` to produce compact single-line JSON suitable for JSONL output. Prefer `jq -s` (slurp) to read a JSONL file into a JSON array for batch processing.

Do not pretty-print JSONL. Each record must occupy exactly one line. Newlines inside string values must be escaped as `\n`.

## jq Patterns

Prefer `jq` for command-line JSON manipulation. It is the standard tool and available on every major platform.

Prefer `jq '.field'` for simple extraction, `jq '.[] | select(.active)'` for filtering arrays, and `jq '{name, email}'` for reshaping objects. Compose filters with pipes inside the jq expression rather than piping between multiple `jq` invocations.

Prefer `jq -e` when using jq in shell conditionals. The `-e` flag sets the exit code based on the output value: `null` and `false` produce a nonzero exit, making `if jq -e '.enabled' config.json` work correctly.

Prefer `jq -r` (raw output) when extracting string values for shell use. Without `-r`, strings include surrounding double quotes.

## Common Pitfalls

Large integers lose precision in JSON when parsed by JavaScript. IEEE 754 double-precision floats cannot represent integers beyond 2^53 exactly. If your data includes 64-bit IDs or timestamps as integers, prefer transmitting them as strings.

Unicode escape sequences (`\uXXXX`) are valid in JSON strings but easy to get wrong for characters outside the Basic Multilingual Plane. Supplementary characters require surrogate pairs (`\uD800\uDC00` style). Prefer using the literal UTF-8 character when the file encoding is UTF-8, which it almost always should be.

Duplicate keys in a JSON object are technically allowed by RFC 8259 but their behavior is undefined. Some parsers keep the first value, others keep the last, and others reject the document. Prefer unique keys and lint for duplicates.

Empty objects (`{}`) and empty arrays (`[]`) are valid JSON documents. `null` is also a valid top-level JSON value. Not all consumers handle these correctly; validate assumptions at the boundary.

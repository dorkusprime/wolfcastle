# ADR-056: Cobra Dependency Evaluation

## Status
Accepted

## Date
2026-03-14

## Context
Wolfcastle currently has one external dependency: `github.com/spf13/cobra` (with transitive deps pflag and mousetrap). For an autonomous tool running unattended, every dependency is a potential CVE surface and a maintenance liability. Good engineering practice calls for periodic evaluation of external dependencies to ensure their benefits continue to justify their costs.

This ADR records a deliberate audit of the Cobra dependency and the resulting decision.

## Decision

**Retain Cobra, but document the justification and establish a review trigger.**

### Features Cobra Provides That We Use

| Feature | Cobra | Hand-rolled cost |
|---------|-------|-----------------|
| Auto-generated `--help` for all commands | Built-in | ~200 lines of help text maintenance |
| Shell completions (bash/zsh/fish) | Built-in + custom completers | ~300 lines, manual maintenance when commands change |
| Persistent flags (`--json` on root) | Built-in | ~30 lines of flag threading |
| Required flag enforcement | `MarkFlagRequired()` | ~50 lines of validation per command |
| Subcommand tree structure | Built-in | Switch dispatch (~100 lines) |
| Fuzzy command suggestions | Built-in | Not replicated |
| Unknown flag detection | Built-in | Not replicated |
| Flag type validation | Built-in | ~20 lines per flag type |

### Cost of Replacing Cobra

Estimated ~700-1000 lines of hand-rolled CLI infrastructure, plus ongoing maintenance as commands are added/changed. Shell completions alone are ~300 lines and must be manually kept in sync.

### Cost of Keeping Cobra

- 3 dependencies in go.sum (cobra, pflag, mousetrap)
- mousetrap is Windows-only, zero-risk on Unix
- Cobra has no history of CVEs in its 10+ year lifespan
- pflag is a well-audited, stable library

### Decision

The features Cobra provides (auto-help, completions, persistent flags, subcommand trees, fuzzy matching) are genuinely valuable for a CLI with 47+ commands. Replacing it would trade ~3 well-audited dependencies for ~800 lines of hand-maintained infrastructure. The CVE risk is theoretical. Cobra has had no security issues in over a decade.

### Review Trigger

Re-evaluate if any of these conditions become true:
- A CVE is filed against cobra, pflag, or mousetrap
- The command surface shrinks below 15 commands (Cobra's overhead exceeds its value)
- Go stdlib adds a native command framework that covers our needs
- Wolfcastle is deployed in an environment where any external dependency is prohibited by policy

## Consequences
- Cobra remains the CLI framework: no migration effort
- The dependency is explicitly justified, not just inherited
- A review trigger prevents the decision from becoming stale
- Contributors understand why the dependency exists and when to reconsider

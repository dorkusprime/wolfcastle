# ADR-010: Wolfcastle-Managed Documentation (ADRs and Specs)

## Status
Accepted

## Date
2026-03-12

## Context
Ralph's ADR integration was a key strength: agents checked existing decisions before acting and filed new ones when making architectural choices. Wolfcastle needs this as a first-class feature, not just for its own development but for projects that use it. Additionally, specs (living system documentation) should also be maintained by Wolfcastle as it works.

## Decision

### Documentation Structure
Wolfcastle manages a `docs/` subtree within `.wolfcastle/`:

```
.wolfcastle/
  docs/
    decisions/       # ADRs: created via deterministic script
    specs/           # System specs: model writes Markdown directly
```

The user can override the docs directory location via `config.json`.

### ADRs
- Created via deterministic script: `wolfcastle adr create "title"`
- The model calls this script during execution when it makes architectural decisions
- ADRs follow the format established in ADR-001 (Status, Date, Context, Decision, Consequences)

### Specs
- Written and updated by the model as direct Markdown
- Living documents that the model keeps in sync as the system evolves
- No script intermediary: specs are prose-heavy and context-dependent

### Two Categories of Model Output
This refines ADR-003 (deterministic scripts):
- **State**. Always through deterministic scripts (JSON). No exceptions.
- **Documentation** (ADRs, specs). Model writes Markdown directly. ADR creation is script-assisted (for filename/template), but content is model-written.

## Consequences
- Wolfcastle agents check existing ADRs before acting and file new ones when making decisions
- Specs stay current as a natural byproduct of execution, not a separate maintenance burden
- ADR creation is reliable (script handles filename, template) while content remains flexible
- Projects using Wolfcastle get decision tracking out of the box

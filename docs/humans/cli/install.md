# wolfcastle install skill

Installs the Wolfcastle skill for Claude Code.

## What It Does

Checks that `.wolfcastle/system/base/skills/` exists and has content. Creates `.claude/` if needed. Then installs the skill using the best method available:

**Symlink mode** (preferred, on macOS and Linux): creates `.claude/wolfcastle/` as a symlink pointing to `.wolfcastle/system/base/skills/`. When you run [`wolfcastle update`](update.md), the skills update automatically because the symlink still points to the regenerated `base/`.

**Copy mode** (fallback, on platforms without symlink support): copies the skill files to `.claude/wolfcastle/`. You will need to re-run `install skill` after updates to get new skill content.

If the destination already exists, checks whether it's the correct symlink or a stale copy, and replaces it if needed.

## Usage

```
wolfcastle install skill
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Installed (or already installed correctly). |
| 1 | Not initialized. |
| 2 | Unknown install target. |
| 3 | No skill source found in `base/`. |
| 4 | Permission denied. |

## Consequences

- Creates or replaces `.claude/wolfcastle/` (symlink or directory).
- Enables Wolfcastle commands from within Claude Code sessions.

## See Also

- [`wolfcastle update`](update.md) for how skill updates propagate.
- [`wolfcastle init`](init.md) which must run first.

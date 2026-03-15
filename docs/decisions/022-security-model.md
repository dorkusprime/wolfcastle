# ADR-022: Security Model — User-Configured, Wolfcastle-Transparent

## Status
Accepted

## Date
2026-03-12

## Context
Wolfcastle shells out to AI CLI tools that can read/write files, execute commands, and interact with the filesystem. Ralph used `--dangerously-skip-permissions` for full autonomy. Different users and environments have different security requirements — a developer on a personal machine may want full autonomy, while a CI environment may need strict sandboxing.

## Decision

### Security Is the User's Responsibility
Wolfcastle does not implement its own sandboxing, filesystem boundaries, or permission system. The executing model's capabilities are determined entirely by the CLI flags configured in the `models` dictionary (ADR-013).

### Wolfcastle Makes Security Posture Explicit
Permission flags are configured in `custom/config.json` as part of the model args, visible and auditable:

```json
{
  "models": {
    "heavy": {
      "command": "claude",
      "args": ["--dangerously-skip-permissions", "..."]
    }
  }
}
```

There are no hidden defaults — what's in the config is what the model gets. Teams can review and enforce permission levels through their normal config review process.

### No Wolfcastle-Level Sandboxing
Wolfcastle does not restrict what the model can do beyond what the CLI tool enforces. It does not filter commands, block file paths, or intercept model actions. Adding a Wolfcastle-specific security layer would create a false sense of safety and duplicate work better handled by the CLI tools themselves.

## Consequences
- Users choose their own security posture via CLI flags in config
- Permission levels are visible, auditable, and version-controlled in `custom/config.json`
- Teams can enforce stricter permissions by committing tighter args in shared config
- Individual engineers can loosen permissions in `local/config.json` (gitignored) at their own risk
- Documentation should clearly explain the implications of different permission levels
- Wolfcastle stays simple — no auth, no sandboxing, no permission logic

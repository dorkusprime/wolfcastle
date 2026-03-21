# Security Policy

## Reporting a Vulnerability

If you find a security vulnerability in Wolfcastle, please report it privately. Do not open a public issue.

**Email:** security@dorkusprime.com

**GitHub:** Use [private vulnerability reporting](https://github.com/dorkusprime/wolfcastle/security/advisories/new) if available.

Include:
- Description of the vulnerability
- Steps to reproduce
- Impact assessment
- Suggested fix (if you have one)

You should receive a response within 72 hours. We will coordinate disclosure timing with you.

## Scope

Wolfcastle's security model (ADR-022) explicitly trusts the configured AI model with full filesystem access within the repository directory. The model executes as a subprocess with inherited environment variables. This is by design.

Vulnerabilities in scope:
- State file corruption that bypasses the file locking mechanism
- Path traversal in node addressing that escapes `.wolfcastle/`
- Injection through marker parsing or config loading
- Privilege escalation beyond the configured model's intended access

Out of scope:
- Model behavior (the model is trusted by design)
- Attacks requiring local filesystem access (the attacker already has everything Wolfcastle has)
- Denial of service through large project trees

## Filesystem Requirements

Wolfcastle relies on `flock(2)` advisory locks to serialize access to `.wolfcastle/` state files. These locks are enforced by the local kernel and are not propagated over network filesystems.

Running Wolfcastle against a `.wolfcastle/` directory on NFS, CIFS, or any other network-mounted filesystem means the locking mechanism silently becomes a no-op. Concurrent processes (parallel daemon runs, multiple engineers sharing a mount) will read and write state without mutual exclusion. The result is silent state corruption: no error, no warning, no crash, just quietly inconsistent data.

Operators must ensure that `.wolfcastle/` resides on a local filesystem (ext4, APFS, etc.). This is not a recommendation; it is a hard requirement for data integrity. The "state file corruption bypassing flock mechanism" threat listed above is exactly what happens when this constraint is violated.

## Subprocess Environment Inheritance

Model CLIs invoked by the daemon inherit all environment variables from the parent process. This means API keys, credentials, and any other sensitive env vars present in the shell that launched `wolfcastle start` will be visible to the model subprocess. This is intentional: models need API keys to authenticate with their provider. Operators who run Wolfcastle in environments with sensitive env vars beyond what the model needs should consider launching the daemon from a sanitized shell or using a process wrapper that filters the environment.

## Supported Versions

Only the latest release receives security updates. Upgrade to the latest version before reporting.

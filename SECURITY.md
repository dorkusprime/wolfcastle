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

## Supported Versions

Only the latest release receives security updates. Upgrade to the latest version before reporting.

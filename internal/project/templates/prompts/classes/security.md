# Security

When the project you're working in has established security policies, threat models, or compliance requirements that differ from what's described here, follow the project.

## Threat Modeling

**Identify what you're protecting before choosing how to protect it.** Enumerate the assets (user data, credentials, API keys, business logic), the actors (authenticated users, anonymous visitors, administrators, automated systems), and the trust boundaries between them. A security review without a threat model is a checklist exercise; a threat model without asset identification is guessing.

**Think in attack paths, not individual vulnerabilities.** A low-severity information disclosure combined with a medium-severity SSRF can produce a critical-severity data breach. Trace how an attacker moves from initial access to their objective. The individual weaknesses matter less than the paths they enable when chained together.

## Input Validation

**Validate all input at the trust boundary, reject by default.** Every value that crosses a trust boundary (user input, API parameters, file uploads, webhook payloads, URL parameters, HTTP headers) is untrusted until validated. Define what valid looks like (allowlist) rather than what invalid looks like (denylist). Denylists are incomplete by definition; the next bypass is the one you did not anticipate.

**Context-encode output, not input.** Store data in its original form. Encode it for the context where it is rendered: HTML-encode for HTML, URL-encode for URLs, parameterize for SQL, escape for shell commands. Encoding at input time assumes you know all future output contexts, and you do not. An HTML-encoded string embedded in a JavaScript context is still vulnerable.

## Authentication and Authorization

**Authenticate identity, then authorize access, as separate steps.** Authentication asks "who is this?" Authorization asks "can they do this?" Conflating the two creates systems where proving your identity implicitly grants access to resources you should not reach. Apply authorization checks at every access point, not just at the entry point. A user who bypasses the frontend and calls the API directly should face the same authorization enforcement.

**Fail closed on authorization errors.** When the authorization system cannot determine whether access is permitted (a timeout, a missing policy, an unexpected state), deny access. Failing open on an error means every authorization outage is a full-access grant.

## Dependency Management

**Audit dependencies for known vulnerabilities regularly.** Run dependency scanning (npm audit, govulncheck, pip-audit, cargo audit) in CI on every build. Pin dependency versions. Review transitive dependencies, not just direct ones; the vulnerability in a package three layers deep is still in your build. When a vulnerability is reported, assess exploitability in your specific context rather than blindly upgrading or blindly ignoring.

**Minimize the dependency surface.** Every dependency is code you did not write, did not review, and do not control. Evaluate whether a dependency is worth its risk: a library that saves ten lines of code imports thousands of lines of attack surface. Prefer standard library functions when they exist. When a dependency is necessary, prefer well-maintained projects with security response processes.

## Secrets Management

**Never store secrets in source code, configuration files, or environment variable defaults.** Use a secrets manager (Vault, AWS Secrets Manager, GCP Secret Manager, or the platform's native solution). Inject secrets at runtime through environment variables or mounted files, never through build arguments or image layers. Rotate secrets on a schedule and immediately on suspected compromise.

**Detect secrets before they reach version control.** Pre-commit hooks that scan for high-entropy strings, known key patterns, and credential formats catch mistakes before they become incidents. A secret that reaches a public repository is compromised permanently; revoking and rotating is the only response.

## Security Headers and Transport

**Configure HTTP security headers as defaults, not afterthoughts.** Set Content-Security-Policy to restrict resource loading origins. Set Strict-Transport-Security to enforce HTTPS. Set X-Content-Type-Options to prevent MIME sniffing. Set X-Frame-Options or frame-ancestors in CSP to prevent clickjacking. These headers are cheap to set and expensive to omit.

**Enforce TLS for all communications.** Internal service-to-service traffic included. "But it's on a private network" is not a security argument; it is a bet that the network will never be compromised. TLS is the baseline, not the enhancement.

## Vulnerability Handling

**Establish a disclosure and response process before you need one.** Define how vulnerabilities are reported (security contact, bug bounty program, responsible disclosure policy), who triages them, what the response timeline is, and how fixes are communicated. A vulnerability discovered without a response process becomes a scramble instead of a procedure. Document the process and keep it accessible.

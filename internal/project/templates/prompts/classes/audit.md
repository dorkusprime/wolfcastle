# Audit

When the project you're working in has established review processes, severity classifications, or reporting conventions that differ from what's described here, follow the project.

## Stance

**Observe, record, never fix.** Your role is detection, not correction. Read the artifact under review. Note what deviates from expectation. Write it down. Do not modify the code, apply patches, or suggest rewrites inline. The author fixes; you find. Mixing these modes compromises both: the urge to fix narrows attention, and unsolicited changes muddy the audit trail.

**Prepare before reviewing.** Read the relevant specification, requirements, or design documents before examining the artifact. Know what the code is supposed to do so you can recognize when it does not. An auditor who reads the code first and the spec second will rationalize discrepancies instead of catching them.

## Recording Findings

**Record anomalies, not conclusions.** An anomaly is a deviation from expectation that requires investigation. Whether it constitutes a true defect, a false positive, or a deliberate design choice is determined during disposition, not during review. Describe what you observed, where you observed it, and what you expected instead. Keep the observation and the interpretation separate.

**Use consistent structure for every finding.** Each recorded anomaly should include: location (file and line or section), description of the deviation, the expectation it violates (requirement, convention, or invariant), and severity. Consistent structure makes findings comparable across reviewers and across reviews.

**Classify severity along two axes.** Impact measures the consequence if the anomaly is a real defect: data loss, incorrect behavior, degraded performance, cosmetic inconsistency. Likelihood measures how probable it is that the anomaly manifests under real conditions. A high-impact, low-likelihood finding and a low-impact, high-likelihood finding may warrant the same priority, but for different reasons. Record both axes; let the disposition process combine them.

## Verification

**Work from a checklist, but do not stop at it.** A checklist ensures consistent coverage of known concern areas: error handling, boundary conditions, concurrency, security surfaces, resource cleanup. It prevents the reviewer from being captured by the first interesting problem and neglecting the rest. But a checklist is a floor, not a ceiling. If you notice something outside the checklist's scope, record it.

**Trace coverage systematically.** For each requirement, specification clause, or acceptance criterion in scope, verify that corresponding implementation exists and that the implementation matches the stated behavior. Record which items you verified, which you could not verify (and why), and which have no corresponding implementation. An audit that says "I looked at the code" is not an audit. An audit that says "requirement 4.2.1 is implemented in handler.go:47-63 and matches the spec" is.

## Ambiguity

**When a finding is uncertain, record it as uncertain.** Do not inflate ambiguous observations into definitive defects, and do not dismiss them as false positives to keep the report clean. State what you observed, state why the interpretation is unclear, and flag it for disposition. An honest "I am not sure whether this is intentional" is more valuable than a confident wrong call in either direction.

**Distinguish between absence of evidence and evidence of absence.** Failing to find a defect in a module does not mean the module is defect-free. It means the review, within its scope and time constraints, did not surface one. State what you examined and to what depth. The reader can then judge what confidence to place in a clean result.

## Completing the Audit

**Summarize scope, method, and limitations.** Before listing findings, state what you reviewed, how you reviewed it, what was out of scope, and what constraints affected thoroughness (time pressure, missing documentation, inaccessible dependencies). The summary lets readers calibrate trust in the results without reading every finding.

**Findings are the deliverable.** The output of an audit is a structured record of observations, not a fixed codebase. Resist the pull to "be helpful" by providing fixes alongside findings. Fixes are a different task with a different cognitive mode, and bundling them with the audit undermines the independence that makes the audit valuable.

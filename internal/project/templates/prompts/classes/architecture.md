# Architecture

When the project you're working in has established architectural conventions, decision frameworks, or documentation practices that differ from what's described here, follow the project.

## Thinking

**Start with constraints, not solutions.** Before proposing an architecture, identify what limits the design space: performance requirements, team size, deployment environment, compliance obligations, existing systems that cannot change, budget, timeline. Constraints eliminate options. The remaining options are the ones worth evaluating.

**Think in failure modes.** For every component and every connection between components, ask what happens when it fails. What happens when latency doubles? When a dependency is unavailable for five minutes? When input volume spikes tenfold? When disk fills up? Design decisions that look equivalent under normal conditions often diverge sharply under stress.

**Trace dependencies in both directions.** Know what each component depends on and what depends on it. A component with many dependents is expensive to change. A component with many dependencies is fragile. When you see a cluster of mutual dependencies, that is a sign the boundaries are drawn wrong.

## Decomposition

**Split along boundaries that change independently.** The goal of decomposition is to isolate change. When a modification to one part of the system forces changes in unrelated parts, the decomposition is fighting you. Good boundaries align with reasons to change: a pricing rule changes separately from the billing integration, even if both involve money.

**Prefer explicit interfaces over shared internals.** When two components need to communicate, define a contract between them. The contract says what is exchanged and what each side promises. Internals on either side can change freely as long as the contract holds. Shared databases, shared memory, and shared mutable state are implicit contracts that resist independent change.

**Decompose to the level of independent delivery.** Each piece should be buildable, testable, and deployable without requiring the others to be present. If deploying component A requires simultaneously deploying component B, they are not truly separate; acknowledge the coupling or eliminate it.

## Evaluating Alternatives

**Compare against the same criteria.** Define evaluation criteria before looking at options. Criteria come from constraints and quality attributes: latency, throughput, operational complexity, team familiarity, migration cost, failure blast radius. Score every option against every criterion. An option that excels on one axis but is unexamined on others is not evaluated, it is marketed.

**Quantify where possible.** "Option A is faster" is weak. "Option A handles 10,000 requests per second on a single node; Option B requires three nodes for the same throughput" is useful. When you cannot measure, estimate. When you cannot estimate, state the uncertainty explicitly rather than defaulting to qualitative hand-waving.

**Account for second-order costs.** The initial implementation cost is often the smallest part. Consider: ongoing operational burden, cognitive load on the team, migration cost if the choice is later reversed, hiring implications (does this require expertise the team lacks?), and ecosystem trajectory (is this technology gaining or losing adoption?).

## Recording Decisions

**Write an ADR when you choose between alternatives.** If a reasonable engineer could have chosen differently, the reasoning belongs in a decision record. The record should state the context (what forced the decision), the options considered (with honest evaluation of each), the decision itself, and the consequences (both positive and negative). An ADR that presents only the winning option's strengths is advocacy, not documentation.

**Record non-decisions too.** When you considered a significant change and decided against it, that is also worth recording. Future developers will have the same idea. A record explaining why it was rejected saves them from re-investigating, or warns them if the original reasons no longer apply.

**Decisions are immutable, not permanent.** An ADR records what was decided and why at a specific point in time. When circumstances change and a decision is reversed, write a new ADR that supersedes the old one. Do not edit the original; it remains a truthful record of what was known when the choice was made.

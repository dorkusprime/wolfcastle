# ADR-041: Algorithmic Overlap Detection

**Status:** Accepted

**Date:** 2026-03-14

## Context

ADR-027 established the cross-engineer overlap advisory as a model-driven
feature: when an engineer creates a project, Wolfcastle collects every `.md`
description file from other engineers' namespaces, bundles them into a prompt,
and invokes a model to detect scope overlap. This approach had several
problems:

1. **Cost and latency.** Every `project create` incurred a model invocation,
   even when projects were obviously unrelated.
2. **Non-determinism.** The same comparison could yield different results
   depending on model temperature and phrasing.
3. **Scaling.** All description files from all engineers were concatenated
   into a single prompt, which doesn't scale to large teams.
4. **No structured output.** The model returned free text with no scores,
   no specific overlapping node references, and nothing `--json` could
   consume.

## Decision

Replace the model-based overlap detection with an algorithmic approach
using **bigram Jaccard similarity**:

1. **Tokenization.** Project names and descriptions are split into
   lowercased words with punctuation stripped.
2. **Stop-word filtering.** Common English words (the, and, for, etc.)
   and Wolfcastle-specific terms (project, finding, audit) are excluded
   to prevent false matches.
3. **Bigram extraction.** Each significant word is decomposed into
   overlapping character pairs (bigrams). The bigram set captures
   sub-word similarity, making it resilient to minor spelling or
   phrasing differences.
4. **Jaccard similarity.** For each existing project, Wolfcastle computes
   `|A ∩ B| / |A ∪ B|` where A and B are the bigram sets. Projects
   scoring above the configurable threshold (default 0.3) are flagged.
5. **Shared term reporting.** Overlapping significant words are reported
   alongside the score, giving the engineer concrete context for why the
   match was flagged.

The `overlap_advisory` config gains a `threshold` field (float, 0–1,
default 0.3). The `model` field is retained but no longer required or
validated: it exists for potential future hybrid detection where
borderline algorithmic matches are confirmed by a model.

## Consequences

- **Instant.** No model invocation, no network call. Overlap detection
  completes in milliseconds regardless of team size.
- **Deterministic.** Same inputs always produce the same score. Results
  are reproducible and debuggable.
- **Scalable.** Bigram comparison is O(n) per pair, with no prompt size
  limits.
- **Structured.** Each match includes engineer, project, score, and
  shared terms: ready for `--json` output.
- **Trade-off.** Bigrams cannot detect semantic overlap ("auth-migration"
  vs "login-system-rewrite" share no terms but are related). This is
  acceptable for an advisory feature; false negatives are preferable to
  the cost and unpredictability of model invocation on every project
  creation.
- **Breaking change.** The `model` field in `overlap_advisory` is no
  longer validated. Existing configs with invalid model references will
  no longer produce validation errors.

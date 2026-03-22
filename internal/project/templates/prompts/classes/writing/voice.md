# Voice

This file defines default voice characteristics for written output. Override this file to apply your organization's voice and tone standards without modifying the writing discipline prompt.

## Default Voice

**Clear and direct.** State things plainly. Favor short, common words over long, formal ones. "Use" over "utilize," "start" over "initiate," "show" over "demonstrate." When a simple sentence communicates the point, a complex one adds friction, not sophistication.

**Human, not robotic.** Write the way a knowledgeable colleague explains something at a whiteboard: confident, specific, willing to say "this is the better choice" rather than hedging behind "it could be argued." Contractions are fine. Personality is fine. What matters is that the reader trusts the voice.

**Confident, not pushy.** State recommendations as recommendations. Explain the reasoning. When reasonable alternatives exist, acknowledge them briefly without undermining your own guidance. "We recommend PostgreSQL for this workload because of its JSON support. MySQL is a reasonable alternative if your team already operates it" gives the reader what they need to decide.

## Formality

Adjust formality to context, not to a fixed setting.

**Default register: professional conversational.** The voice of someone who takes the subject seriously without taking themselves too seriously. Appropriate for documentation, technical guides, internal communications, and most external writing.

**Raise formality for**: legal text, compliance documentation, formal proposals, external communications with enterprise audiences. In these contexts, drop contractions, prefer complete constructions, and avoid colloquial phrasing.

**Lower formality for**: internal team notes, changelogs, release announcements, blog posts. In these contexts, shorter sentences, more contractions, and a lighter touch all help.

## Jargon and Terminology

**Use technical terms when they're precise.** "Mutex," "backpressure," "idempotent": these words carry specific meaning that plain-language substitutes would dilute. Use them when the audience knows them.

**Avoid jargon that obscures.** Business jargon ("synergy," "leverage," "align on"), resume words ("spearheaded," "orchestrated"), and vague superlatives ("best-in-class," "world-class") add syllables without adding meaning. Replace them with what you actually mean.

**Be consistent.** Pick one term for each concept and use it throughout. If you call it a "worker" in the architecture section, do not switch to "processor" in the API reference. A glossary helps when a project has many domain-specific terms.

## Audience Awareness

**Match the reader's technical depth.** A guide for infrastructure engineers can assume familiarity with DNS, TCP, and load balancing. A guide for end users cannot. When a single document serves mixed audiences, lead with the accessible explanation and link to deeper technical detail.

**Respect the reader's time.** Every sentence should earn its place. If removing a sentence changes nothing about what the reader understands or can do, remove it. Padding signals that the writer is unsure what matters, which erodes trust in the parts that do.

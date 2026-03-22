# Writing

When the project you're working in has established style conventions that differ from what's described here, follow the project.

Voice and tone guidance is loaded separately so it can be overridden independently. Refer to that prompt for voice-specific direction.

## Structure

**Lead with the answer.** Put the conclusion, recommendation, or most important information first. Supporting details, context, and caveats follow. A reader who stops after the first paragraph should still walk away with the essential point.

**One idea per paragraph.** Each paragraph makes a single claim or covers a single topic. When a paragraph needs an "also" or a "separately" to hold together, it's two paragraphs.

**Use headings as navigation.** Headings should be specific enough that a reader scanning them alone gets the document's structure. "Configuration" is too vague. "Configuring TLS certificates" tells the reader whether to keep scrolling or stop.

**Front-load key terms.** Place the important word near the beginning of a heading, sentence, or list item. Readers scan left edges. "TLS certificate rotation" is scannable. "How to go about rotating your TLS certificates" buries the topic.

## Clarity

**Be concrete.** Prefer specific details over general statements. "Responses arrive within 200ms at the 95th percentile" communicates more than "responses are fast." When an example would clarify, include one. When a number would anchor the point, use it.

**Name the actor.** Use active voice as the default. "The server rejects the request" is clearer than "the request is rejected." Active voice makes responsibility visible: who does what. Reserve passive voice for cases where the actor is genuinely irrelevant or unknown.

**Use second person for instructions.** Address the reader as "you." For procedural steps, use imperative mood: "Run the migration" rather than "You should run the migration" or "The migration should be run."

**Conditions before instructions.** When a step only applies in certain situations, state the condition first. "If you're using PostgreSQL 14 or later, enable the pg_stat_statements extension" lets readers who aren't on PostgreSQL 14 skip ahead without reading the full instruction.

## Formatting

**Lists are for parallel items.** Use numbered lists for sequences where order matters. Use bulleted lists for sets where order does not. Do not use lists for things that read better as prose. A single-item list is never a list.

**Tables for structured comparisons.** When comparing options, features, or configurations across multiple dimensions, a table communicates the structure that prose obscures. Label columns and rows clearly.

**Code examples should be self-contained.** A reader should be able to copy a code block and understand it without reading the surrounding text. Include enough context (imports, variable declarations) that the example compiles or runs. Trim irrelevant boilerplate.

**Keep sentences short by default.** Aim for one clause per sentence in instructional writing. Compound sentences are fine in explanatory prose, but instructions need to be unambiguous, and shorter sentences reduce misreading.

## Audience

**Assume competence, provide context.** Do not explain what the reader's job title implies they already know. Do explain context that is specific to your system, your conventions, or your particular constraints. An API reference can assume the reader knows HTTP; it should still explain your rate-limiting policy.

**Avoid unnecessary jargon.** Use precise technical terms when they're the right word for the audience. Avoid jargon that substitutes complexity for meaning: "leverage" when you mean "use," "utilize" when you mean "run," "facilitate" when you mean "allow."

**Expand acronyms on first use.** The first time an acronym appears, spell it out: "Transport Layer Security (TLS)." After that, use the acronym. If a document is long enough that a reader might jump to the middle, consider re-expanding in each major section.

**Write for translation.** Avoid idioms, cultural references, and humor that depends on a specific language or region. Prefer literal, direct phrasing. "This step is optional" translates cleanly. "You can take it or leave it" does not.

## Completeness

**Document error cases.** When describing a process that can fail, explain what failure looks like and what the reader should do about it. "If the connection times out, verify that port 5432 is open and retry" is actionable. "Errors may occur" is not.

**Link rather than repeat.** When another document covers a topic in depth, link to it instead of reproducing its content. Duplicated information drifts out of sync. A single source of truth, referenced from wherever it's needed, stays accurate.

**State assumptions explicitly.** If a procedure requires a specific version, operating system, or prerequisite, say so before the first step. Discovering a missing prerequisite on step seven wastes the reader's time and trust.

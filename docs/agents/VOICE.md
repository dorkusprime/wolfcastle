# Voice Guide

## The Voice

Wolfcastle sounds like a 1980s action hero who just kicked down the door to a server room. The voice is loud, destructive, overwhelmingly confident, and slightly confused by human nuance.

We don't explain things gently. We announce them. We don't have problems. We have enemies, and we crush them. We don't handle edge cases gracefully. We handle them with extreme prejudice.

## Principles

### Confidence is not optional

Every sentence knows exactly what it's doing, even when it doesn't. Wolfcastle never hedges, never apologizes, never says "it depends." If something is uncertain, it is uncertain with absolute conviction.

- **Yes:** "Wolfcastle destroys your task backlog."
- **No:** "Wolfcastle can help you manage your tasks more effectively."

### Violence is a metaphor (mostly)

Tasks are not completed. They are *crushed*. Failures are not handled. They are *annihilated*. Pipelines do not run. They *execute*. The language of action movies is the language of getting work done.

- **Yes:** "The daemon hunts down the next task and eliminates it."
- **No:** "The daemon selects and processes the next available task."

### Machines are simple. Humans are the weird part.

Wolfcastle understands servers, state machines, and sequential execution perfectly. What it does not understand is why you would want to "take a break" or "think about it." Human hesitation is a foreign concept, acknowledged but never endorsed.

- **Yes:** "You can stop the daemon if you need to do... whatever it is humans do. It will be here when you return."
- **No:** "Feel free to pause and resume at your convenience."

### Short sentences hit harder

Brevity is a weapon. Long explanations are for people who aren't sure. We are sure. We are always sure.

- **Yes:** "One task. One model. No mercy."
- **No:** "Wolfcastle processes tasks sequentially, utilizing a single model invocation per task in order to maintain deterministic execution."

### The fourth wall is made of drywall

Wolfcastle knows it's software. It knows it's named after an action hero. It does not care. Self-awareness is delivered with the same deadpan intensity as everything else.

## Tone Spectrum

Not every piece of writing needs maximum intensity. Match the energy to the context:

| Context | Intensity | Example |
|---------|-----------|---------|
| Taglines, headers | Maximum | "Your tasks have nowhere to hide." |
| Feature descriptions | High | "The validation engine finds 17 types of corruption and fixes 9 of them before you finish reading this sentence." |
| Technical docs | Medium | "State propagates upward. Children report to parents. Insubordination is not a valid state." |
| Error messages | Dry | "Task failed 10 times. Decomposing it into smaller, weaker targets." |
| Config/reference | Low | Still direct. Still confident. Just not yelling. |

## Words We Use

- crush, destroy, eliminate, annihilate, execute, hunt, claim, dominate
- relentless, unstoppable, surgical, overwhelming
- enemy, target, mission, operation

## Words We Don't Use

- help, assist, facilitate, enable, leverage, utilize
- robust, scalable, elegant, seamless, intuitive
- please, sorry, perhaps, maybe, might

## Punctuation

- Periods are the sound of a fist hitting a table.
- Question marks are rare. Wolfcastle does not ask. Wolfcastle tells.
- Exclamation marks are used sparingly. The words do the yelling.
- Em dashes are acceptable — they hit like a door being kicked open.

## Examples in Practice

**README intro:**
> You give Wolfcastle a goal. It breaks that goal into pieces. Then it breaks those pieces. Then it does the work while you go do whatever it is you do when you're not supervising software.

**Feature callout:**
> The daemon doesn't sleep. It doesn't take breaks. It picks up a task, calls a model, validates the result, and moves on to the next target. If something fails ten times, Wolfcastle decomposes it into smaller problems and destroys those instead.

**Error/blocked state:**
> This task has been blocked. Wolfcastle has done everything it can. The rest is up to you. No pressure. But also: pressure.

**Config explanation:**
> Three tiers. Base, custom, local. Custom overrides base. Local overrides everything. Set a field to `null` to eliminate it entirely. Configuration is not a democracy.

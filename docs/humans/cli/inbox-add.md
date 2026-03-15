# wolfcastle inbox add

Throws an idea at Wolfcastle. It catches it.

## What It Does

Appends a new entry to `projects/{identity}/inbox.json` (creates the file if it does not exist). The entry gets a timestamp and a status of `new`.

The daemon's [intake stage](../how-it-works.md#the-pipeline) picks up new inbox items in a parallel goroutine, decomposes them into tasks using a model, and files them into the [project tree](../how-it-works.md#the-project-tree). You do not need to specify where the work belongs. Wolfcastle figures that out.

## Usage

```
wolfcastle inbox add "Support OAuth2 PKCE flow"
```

## Arguments

| Argument | Description |
|----------|-------------|
| `idea` | **(Required)** What you want done. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Item added to inbox. |
| 1 | Not initialized. |
| 2 | Empty idea. |

## Consequences

- Mutates `inbox.json` in your [namespace](../collaboration.md#engineer-namespacing).
- The item will be picked up and decomposed on the next daemon iteration (if running).

## See Also

- [`wolfcastle inbox list`](inbox-list.md) to see what's in the inbox.
- [`wolfcastle inbox clear`](inbox-clear.md) to clean up processed items.
- [`wolfcastle task add`](task-add.md) if you know exactly where the task belongs.
- [Getting Work In](../how-it-works.md#getting-work-in) for all three entry points.

# Phoenix

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Contexts and Domain Boundaries

Prefer Phoenix contexts as the public API between your web layer and business logic. Each context module (`Accounts`, `Catalog`, `Billing`) exposes domain operations as functions; controllers and LiveViews call contexts, never Repo directly. Keep context functions focused: if a context accumulates more than a dozen public functions spanning unrelated concerns, split it into smaller contexts rather than letting it become a god module. Prefer Ecto schemas as internal to their owning context; other contexts reference by ID, not by reaching into another context's schema.

## Schemas and Changesets

Prefer `Ecto.Schema` with explicit `field`, `belongs_to`, `has_many` declarations over schemaless changesets for persistent domain objects. Use `cast/3` before `validate_required/2` and other validators; cast filters and converts incoming params, then validations run on the casted data. Prefer separate changeset functions for distinct operations (`registration_changeset`, `profile_changeset`) over a single changeset with conditional logic. Use `Ecto.Multi` for operations that must succeed or fail together across multiple schemas; Multi composes named steps and rolls back all changes on any failure.

## Router and Pipelines

Prefer `pipeline` blocks to group plugs by concern (`:browser`, `:api`, `:authenticated`). Use `scope` to apply pipelines to route groups. Prefer `resources` macro for standard CRUD, `live` macro for LiveView routes. In Phoenix 1.7+, prefer verified routes (`~p"/users/#{user}"`) over route helpers; verified routes are compile-time checked and fail fast on typos.

## LiveView

Prefer `mount/3` for initial state, `handle_params/3` for URL-driven state changes, and `handle_event/3` for user interactions. Keep assigns minimal: store IDs and lightweight data, not large structs or full query results that persist in process memory across events. Prefer streams (`stream/3`, `stream_insert/3`, `stream_delete/3`) for rendering collections; streams avoid holding the entire list in assigns and send only diffs to the client. Extract reusable UI into function components (`attr`/`slot` declarations); use live components (`live_component`) only when the component needs its own state or event handling. Prefer `assign_async/3` and `start_async/3` for data that loads after mount, keeping the initial render fast.

## Ecto and Repo

Prefer query composition with `from` and `Ecto.Query` pipelines over building raw SQL. Use `Repo.preload/2` for loading associations on already-fetched structs; use `join` with `preload` in the query when you need to filter on the association. Confusing these two patterns leads to either excess queries (preload after the fact on a large list) or missing data (joining without selecting). Prefer `Repo.insert_all/3` and `Repo.update_all/3` for bulk operations; they bypass changesets and validations, so use them for batch administrative work, not user-facing mutations. Use `Repo.transaction/1` with `Ecto.Multi` for multi-step operations rather than nesting `Repo.transaction` calls.

## PubSub and Channels

Prefer `Phoenix.PubSub` for intra-cluster real-time messaging. Subscribe in `mount/3`, handle messages in `handle_info/3`. Use topic namespacing (`"user:#{user_id}"`) to scope broadcasts. Do not assume PubSub message ordering across nodes or under high load; design handlers to be idempotent. Prefer Phoenix Channels with `socket` and `channel` modules when clients need bidirectional communication over WebSockets; use Presence for tracking connected users. For server-push-only updates to LiveViews, `PubSub` alone is sufficient without Channels.

## Background Jobs

Prefer Oban for persistent, retryable background work. Define workers as modules using `Oban.Worker` with `@impl true` on `perform/1`. Configure queues with concurrency limits per queue. Use `Oban.insert/1` to enqueue; pass serializable args (maps with string keys, not structs). Prefer unique job constraints (`unique: [period: 60]`) to prevent duplicate enqueuing. Use `Oban.Testing` helpers (`assert_enqueued`, `perform_job`) in tests rather than hitting the database.

## Testing

Prefer `ConnTest` for controller and LiveView integration tests, `DataCase` for context and schema tests. Use `async: true` on all test modules that do not share mutable state beyond the Ecto sandbox. Prefer Mox for mocking external dependencies; define behaviours on boundary modules and `Mox.defmock/2` in `test_helper.exs`. Use `Ecto.Adapters.SQL.Sandbox` in shared mode only when async tests interact with processes that run queries (like LiveView pids); default to exclusive mode otherwise. Prefer `live/2` from `Phoenix.LiveViewTest` for LiveView tests; assert on rendered HTML and push events with `render_click/2`, `render_submit/2`. Use `Wallaby` or similar only for true end-to-end browser testing where LiveViewTest cannot reach.

## Common Pitfalls

Context boundaries that start clean erode into god modules when every new feature adds functions to the nearest existing context. Watch for contexts with mixed concerns (user preferences, billing, and notifications in `Accounts`). Split early; the refactor cost grows with every caller.

LiveView processes hold their assigns in memory for the lifetime of the connection. Storing large datasets, file contents, or unbounded lists in assigns causes per-connection memory bloat that scales with concurrent users. Use streams for collections and external storage for large objects.

Ecto `preload` after fetch and `join`-based preload solve different problems. Preload after fetch issues one query per association (fine for small result sets). Join-based preload uses a single query but can produce large cartesian products. Choosing wrong produces either N+1 queries or bloated result sets.

PubSub delivers messages asynchronously with no ordering guarantees across processes or nodes. Handlers that assume messages arrive in publication order, or that process sequential broadcasts atomically, will produce subtle race conditions under load.

Changeset pipelines silently drop fields not included in `cast/3`. Calling `validate_required/2` before `cast/3`, or casting a subset of fields while validating the full set, produces changesets that always fail validation on the uncast fields. Always cast first, validate second, and verify the field lists are consistent.

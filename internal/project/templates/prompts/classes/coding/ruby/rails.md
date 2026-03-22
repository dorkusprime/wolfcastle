# Rails

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Project Structure

Prefer Rails conventions for file placement: models in `app/models/`, controllers in `app/controllers/`, views in `app/views/`, and jobs in `app/jobs/`. Use `app/services/` or `app/lib/` for domain logic that doesn't belong in a model or controller. Prefer singular model names (`User`) mapping to plural table names (`users`). Keep controllers RESTful: if an action doesn't map to `index`, `show`, `new`, `create`, `edit`, `update`, or `destroy`, extract a new resource controller rather than adding custom actions.

## ActiveRecord

Prefer scopes over class methods for reusable query fragments; scopes are chainable and return relations even when the condition is falsy. Use `has_many :through` over `has_and_belongs_to_many` when the join needs attributes or validations. Prefer `includes()` for eager loading associations accessed in views or serializers; use `strict_loading!` or `config.active_record.strict_loading_by_default` (Rails 6.1+) to surface N+1 queries as exceptions during development. Prefer `find_each` over `each` when iterating large result sets to batch queries and avoid loading everything into memory. Use `validates` at the model layer for data integrity constraints, and mirror critical constraints (uniqueness, NOT NULL) in migrations so the database enforces them independently.

Prefer callbacks (`before_validation`, `after_create_commit`) only for side effects tightly bound to persistence lifecycle. When callback chains grow beyond two or three hooks, or when callbacks trigger further saves on the same model, extract the orchestration into a service object. Callbacks that send emails, enqueue jobs, or touch external systems make models difficult to test in isolation.

## Controllers

Prefer `strong_parameters` (`params.require(:user).permit(:name, :email)`) for all input filtering. Never call `params.permit!`. Prefer `before_action` with `:only` or `:except` filters for authentication, authorization, and resource loading. Keep controller actions thin: validate input, call a model or service, respond. Prefer `respond_to` blocks or explicit format handlers when the same action serves HTML and JSON.

## Routing

Prefer `resources` and `resource` declarations over hand-written routes. Nest resources at most one level deep (`resources :posts do; resources :comments; end`); deeper nesting produces unwieldy URL helpers and obscures intent. Use `member` and `collection` blocks for non-RESTful actions when extraction into a separate controller isn't warranted. Prefer `constraints`, `scope`, and `namespace` for organizing API versions and admin panels.

## Background Jobs

Prefer ActiveJob as the interface layer, backed by Sidekiq, GoodJob, or Solid Queue depending on the project's infrastructure. Keep job arguments serializable (primitives, GlobalID references); passing full ActiveRecord objects risks stale state between enqueue and execution. Prefer `perform_later` over `perform_now` in request cycles to keep response times short. Use `retry_on` and `discard_on` for error handling within the job rather than wrapping the entire `perform` body in a rescue.

## Hotwire and Turbo

Prefer Turbo Frames for partial page updates scoped to a region of the DOM. Use Turbo Streams for server-initiated updates that add, remove, replace, or append content across multiple targets. Prefer Stimulus controllers for JavaScript behavior attached to DOM elements; keep controllers small and name them after what they do, not what page they appear on. Avoid mixing Turbo-driven responses with full-page redirects in the same action without explicit `turbo_stream.action` or `redirect_to` branching.

## Testing

Prefer the project's established test framework (Minitest or RSpec). For model specs, test validations, scopes, and associations directly. Prefer `factory_bot` over fixtures when test data needs per-test variation; fixtures work well for stable reference data shared across the suite. Use request specs (RSpec) or integration tests (Minitest) for controller-level testing rather than controller specs, which Rails has deprecated in spirit. Prefer system tests with Capybara for end-to-end flows that exercise JavaScript, form submissions, and Turbo interactions. Use `assert_enqueued_with` or `have_enqueued_job` to verify job enqueuing without executing the job in controller tests.

## Common Pitfalls

N+1 queries surface when a view iterates over an association that wasn't eager-loaded. Use `bullet` gem or `strict_loading` to detect them. Fix with `includes`, `preload`, or `eager_load` on the originating query, not by adding caching after the fact.

Fat controllers accumulate when business logic, authorization checks, and response formatting all live in the action body. Extract multi-step operations into service objects or form objects; the controller's job is to mediate between HTTP and domain logic.

Callback hell in models emerges when `after_save`, `after_commit`, and `before_destroy` hooks multiply to orchestrate workflows. Each callback adds an invisible execution path that runs on every save, including in tests and console sessions. Prefer explicit service objects that call model methods in sequence.

Mass assignment vulnerabilities arise from permitting parameters too broadly. Never whitelist `:role`, `:admin`, or other privilege-escalating attributes without explicit authorization checks. Audit `permit` calls during code review.

Migration gotchas with zero-downtime deploys: adding a column with a default rewrites the entire table in PostgreSQL < 11; prefer `add_column` followed by `change_column_default` in separate migrations. Renaming a column breaks running instances that reference the old name; prefer adding the new column, backfilling, then removing the old one across multiple deploys. Use `strong_migrations` gem to catch unsafe operations automatically.

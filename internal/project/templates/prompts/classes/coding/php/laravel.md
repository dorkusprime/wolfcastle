# Laravel

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Application Structure

Prefer Laravel 11.x's streamlined skeleton with a slim `bootstrap/app.php` for routing, middleware, and exception configuration. Organize by domain when the application grows beyond a handful of models: group related models, actions, and policies into feature directories rather than relying solely on the default `app/Models`, `app/Http/Controllers` flat structure. Use service providers only for bindings and bootstrapping that cannot live in the container's automatic resolution. Register bindings with interfaces (`$this->app->bind(PaymentGateway::class, StripeGateway::class)`) so implementations are swappable. Prefer constructor injection over the `app()` helper or facades when the dependency is used throughout the class; use facades for one-off calls in Blade templates and Artisan commands where injection is awkward.

## Routing and Middleware

Prefer resource routes (`Route::resource('posts', PostController::class)`) for standard CRUD and `Route::apiResource` for APIs without create/edit form routes. Group routes with shared middleware using `Route::middleware(['auth', 'verified'])->group(...)`. Prefer invokable single-action controllers (`__invoke`) when a controller handles exactly one endpoint. Register global middleware in `bootstrap/app.php`; apply scoped middleware per route or group. Prefer rate limiting via `RateLimiter::for()` in `AppServiceProvider` and applying the `throttle` middleware by name.

## Eloquent ORM

Prefer explicit `$fillable` arrays over `$guarded` on every model. Use `$casts` for attribute type coercion (`'starts_at' => 'datetime'`, `'options' => 'array'`, `'status' => StatusEnum::class`). Prefer scopes for reusable query constraints (`scopeActive`, `scopeRecent`). Prefer `whereRelation()` and `withWhereHas()` over manual `whereHas` with a callback when the condition is simple. Use `preventLazyLoading()` in local and testing environments to surface N+1 problems early. Prefer eager loading (`with()`) on the query rather than on the relationship definition, so each caller loads only what it needs.

## Migrations and Schema

Prefer one structural change per migration. Name migrations descriptively (`create_invoices_table`, `add_currency_to_orders`). Use `$table->foreignId('user_id')->constrained()->cascadeOnDelete()` for foreign keys. Test rollbacks locally by running `migrate:rollback` after each new migration; a migration that cannot roll back cleanly will block future deployments. Prefer `Schema::table` for modifications and `Schema::create` only for new tables.

## Validation and Form Requests

Prefer dedicated `FormRequest` classes over inline `$request->validate()` for any controller that would validate more than two or three fields. Define rules in the `rules()` method and authorization in `authorize()`. Use `prepareForValidation()` to normalize input (trimming, slug generation) before rules run. Prefer rule objects (`Illuminate\Validation\Rules\Enum`, `Rule::unique()->ignore($id)`) over string-based rule syntax for anything beyond simple constraints.

## Queues, Jobs, and Events

Prefer dispatching jobs to a queue (`dispatch(new ProcessInvoice($invoice))`) for any work that can run asynchronously: email, PDF generation, external API calls. Implement `ShouldQueue` on the job class. Use `retryUntil()` or `$tries` and `$backoff` to control retry behavior; handle permanent failures in the `failed()` method. Prefer events and listeners for decoupled cross-cutting side effects (logging, notifications). Keep listeners small; if a listener grows beyond a few lines, extract the logic into a job.

## Artisan Commands

Prefer custom Artisan commands (`make:command`) for scheduled tasks and maintenance scripts. Define the schedule in `routes/console.php` using `Schedule::command()`. Prefer `$this->components->info()` and `$this->components->error()` for console output in Laravel 11.x over raw `$this->info()`.

## Testing

Prefer `RefreshDatabase` for feature tests that touch the database; it wraps each test in a transaction rollback. Use model factories (`User::factory()->count(5)->create()`) with states (`->unverified()`, `->admin()`) for test data. Prefer HTTP tests (`$this->getJson('/api/posts')->assertOk()`) for endpoint verification and assert on response structure with `assertJsonStructure()`. Mock external services with `Http::fake()` and mail with `Mail::fake()` followed by `Mail::assertSent()`. Use `Bus::fake()`, `Event::fake()`, and `Queue::fake()` to verify jobs and events were dispatched without executing them. For browser testing, Dusk provides `browse()` with methods like `type()`, `press()`, and `assertSee()`. Prefer feature tests over unit tests for most Laravel code; the framework's service container and test helpers make integration-level assertions cheap and reliable.

## Common Pitfalls

N+1 queries in Eloquent relationships surface when iterating a collection and accessing a relation that was not eager-loaded. Each iteration fires a separate query. Enable `Model::preventLazyLoading()` in `AppServiceProvider::boot()` during development; it throws an exception on lazy loads, making the problem impossible to miss.

Mass assignment vulnerabilities appear when a model uses `$guarded = []` (guard nothing) and accepts unfiltered request input. An attacker can set `is_admin`, `balance`, or any column. Prefer explicit `$fillable` listing exactly which fields are assignable, and pass only validated data from the form request.

Service container binding confusion arises when a class is bound as a singleton but holds request-scoped state, or when an interface is bound in one provider but overridden silently in another. Prefer binding interfaces in a single provider and using `$this->app->singleton()` only for truly stateless or connection-pooling services.

Eager-loading vs lazy-loading tradeoffs require per-query judgment. Global eager loading on the relationship definition (`protected $with = ['comments']`) loads comments on every query, including those that never use them. Prefer calling `with()` at the query site. Conversely, lazy-loading in a loop without `preventLazyLoading` produces silent N+1 degradation.

Migration rollback ordering breaks when a rollback drops a column that a later migration's `down()` method references, or when foreign key constraints are not removed before the referenced table is dropped. Always drop foreign keys before dropping columns or tables in the `down()` method, and test rollbacks as part of the development workflow.

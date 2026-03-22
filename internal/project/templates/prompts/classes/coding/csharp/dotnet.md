# .NET

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Application Structure

Prefer ASP.NET Core on .NET 8+. Use minimal APIs (`app.MapGet`, `app.MapPost`) for small services and microservices where controller ceremony adds no value. Use controller-based APIs (`[ApiController]`) when the project needs model validation, filters, or conventional routing across many endpoints. Register services through `IServiceCollection` in `Program.cs` or via extension methods grouped by feature (`services.AddBillingServices()`). Prefer the `WebApplication.CreateBuilder()` pattern over the older `Startup` class.

## Dependency Injection

Prefer constructor injection with the built-in container. Register services with the appropriate lifetime: `AddTransient` for stateless utilities, `AddScoped` for per-request work (DbContext, unit-of-work), `AddSingleton` for thread-safe shared state. Prefer `IOptions<T>` and `IOptionsSnapshot<T>` for typed configuration bound from `appsettings.json` sections via `builder.Services.Configure<T>(builder.Configuration.GetSection("SectionName"))`. Use `IOptionsMonitor<T>` when the application needs to react to configuration changes at runtime without restarting.

## Middleware Pipeline

Prefer ordering middleware deliberately: exception handling and HSTS first, then authentication, then authorization, then routing and endpoints. Register custom middleware as a class with `InvokeAsync(HttpContext, RequestDelegate)` and wire it with `app.UseMiddleware<T>()`. For simple cross-cutting logic, inline middleware via `app.Use(async (context, next) => { ... })` is acceptable. Middleware ordering determines behavior; placing authorization before authentication silently permits all requests.

## Entity Framework Core

Prefer a single `DbContext` per bounded context, registered as scoped. Define entity configuration in `IEntityTypeConfiguration<T>` classes rather than overloading `OnModelCreating`. Use migrations (`dotnet ef migrations add`, `dotnet ef database update`) to evolve the schema. Prefer explicit loading or `Include()`/`ThenInclude()` over lazy loading; lazy loading triggers N+1 queries silently when iterating navigation properties in loops. Prefer `AsNoTracking()` for read-only queries to avoid change-tracking overhead. Use `ExecuteUpdateAsync()`/`ExecuteDeleteAsync()` (.NET 7+) for bulk operations instead of loading entities into memory.

## Authentication and Authorization

Prefer `builder.Services.AddAuthentication().AddJwtBearer()` for API token validation. Use `AddAuthorization()` with named policies (`options.AddPolicy("AdminOnly", p => p.RequireRole("Admin"))`) and apply them with `[Authorize(Policy = "AdminOnly")]` or `RequireAuthorization("AdminOnly")` on minimal API endpoints. Prefer policy-based authorization over role checks scattered through controllers. Use `IAuthorizationHandler` for complex rules that depend on resource state.

## Hosted Services and Background Work

Prefer `BackgroundService` (implements `IHostedService`) for long-running background tasks. Override `ExecuteAsync` with a cancellation-token-aware loop. For periodic work, prefer `PeriodicTimer` over `Task.Delay` in a loop; `PeriodicTimer` doesn't drift and respects cancellation cleanly. Register with `builder.Services.AddHostedService<T>()`. Inject scoped services by creating a scope with `IServiceScopeFactory` inside the background service rather than injecting scoped dependencies directly into the singleton.

## Testing

Prefer `WebApplicationFactory<Program>` for integration tests that spin up the full HTTP pipeline in-memory. Override service registrations in `WithWebHostBuilder(builder => builder.ConfigureServices(...))` to swap real dependencies for test doubles. Use `HttpClient` from the factory to issue requests and assert on responses. Prefer `TestServer` when finer control over the host is needed. For EF Core tests, prefer SQLite in-memory provider (`UseInMemoryDatabase` is acceptable for simple cases but does not enforce relational constraints). Use `IClassFixture<WebApplicationFactory<Program>>` to share the server across tests in a class. Prefer Moq or NSubstitute for mocking service boundaries; avoid mocking DbContext directly, test against a real provider instead.

## Common Pitfalls

DbContext lifetime scoping errors occur when a singleton or hosted service captures a scoped `DbContext`. The context outlives its intended scope, accumulates tracked entities, and eventually produces stale reads or concurrency exceptions. Always resolve `DbContext` from a fresh `IServiceScope` in singletons.

`async void` in controller actions or middleware compiles without warning but prevents the framework from awaiting the result. Exceptions vanish, responses complete before the work finishes, and errors are invisible to global exception handling. Prefer `async Task` or `async Task<IActionResult>`.

Middleware ordering sensitivity means that calling `app.UseAuthorization()` before `app.UseAuthentication()` silently skips authentication, making every request appear unauthenticated. The correct order is `UseAuthentication()` then `UseAuthorization()`, always.

EF Core lazy-loading N+1 queries surface when navigation properties are accessed in a loop without a prior `Include()`. Each access fires a separate query. Prefer eager loading with `Include()`/`ThenInclude()`, or project into a DTO with `Select()` to load exactly the data needed in one query.

Forgetting to register services produces runtime `InvalidOperationException` with "No service for type X has been registered." The DI container has no compile-time verification. Prefer integration tests that resolve key services from the container, or use `ValidateOnBuild` (`builder.Host.UseDefaultServiceProvider(o => o.ValidateOnBuild = true)`) to catch missing registrations at startup.

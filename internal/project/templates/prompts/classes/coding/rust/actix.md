# Actix Web

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Community Context

Actix Web 4.x is the current major version. It remains one of the fastest Rust web frameworks. Axum has gained significant community adoption for new projects due to its simpler middleware model and tighter Tower/Tokio integration. For existing Actix Web codebases, stay with Actix Web. For new projects, evaluate both; choose Actix Web when raw throughput matters most, Axum when middleware composability and ecosystem breadth are priorities.

## Application Structure and Routing

Prefer building the `App` with `web::scope` to group related routes under a shared prefix and middleware set. Prefer explicit method guards (`web::resource("/items").route(web::get().to(list)).route(web::post().to(create))`) over catch-all handlers. Prefer handler functions as `async fn` with typed extractors in the signature; Actix resolves `Path<T>`, `Query<T>`, `Json<T>`, and `web::Data<T>` from the request automatically based on position. Keep route registration in a `configure` function (`pub fn configure(cfg: &mut web::ServiceConfig)`) per module so the main `App` builder stays clean.

## Application State

Prefer `web::Data<T>` for shared application state (database pools, configuration, caches). Wrap mutable shared state in `web::Data<Arc<Mutex<T>>>` or `web::Data<Arc<RwLock<T>>>` depending on access patterns. State passed via `.app_data(web::Data::new(pool))` is cloned per worker thread; because `web::Data` wraps an inner `Arc`, all workers share the same underlying value. Prefer initializing state outside the `HttpServer::new` closure and moving clones in, rather than constructing state inside the closure where it would be duplicated per worker.

## Error Handling

Prefer implementing the `ResponseError` trait on custom error types. This gives each error variant control over its HTTP status code and response body. Prefer an application-level error enum with `#[from]` derives (via `thiserror`) to convert from library errors (`sqlx::Error`, `serde_json::Error`) into your domain error type. Avoid returning raw `actix_web::error::Error` from handlers; a concrete error type makes the API contract visible and testable.

## Middleware

Prefer `actix_web::middleware::Logger` for request logging and `actix_cors::Cors` for CORS. For custom middleware, prefer the `from_fn` middleware for simple cases; it takes an async function, supports extractors as leading parameters, and avoids the full `Transform`/`Service` trait boilerplate. Reserve manual `Transform` + `Service` implementations for middleware that needs to inspect or buffer the response body. Be careful with extractors that consume the request body in middleware; handlers will not be able to read it again unless you put the body back into the request.

## Database Integration

Prefer async database access with `sqlx` and a `PgPool` (or `SqlitePool`) passed as `web::Data<PgPool>`. Queries run on the async runtime without blocking worker threads. If using Diesel or another synchronous driver, wrap all database calls in `actix_web::web::block` to move them onto a threadpool; calling synchronous database code directly in an async handler blocks the worker's event loop. Prefer running migrations at startup before binding the server, not lazily on first request.

## WebSocket Actors

Prefer `actix_web_actors::ws::WebsocketContext` for WebSocket connections. Each connection becomes an Actix actor with `Handler` implementations for incoming frames. Prefer `ctx.text()` and `ctx.binary()` for sending, and implement `StreamHandler<Result<ws::Message, ws::ProtocolError>>` for receiving. Use `ctx.run_interval` for periodic heartbeat pings; connections without heartbeats accumulate silently when clients disconnect uncleanly.

## Testing

Prefer `actix_web::test::init_service` with `actix_web::test::TestRequest` for unit-style handler tests. Build a test app with the same `configure` function used in production so route configuration stays in sync. Use `actix_web::test::call_service` to send a `TestRequest` and assert on the response status and body. Prefer `actix_web::test::TestServer` for integration tests that need a real HTTP listener (WebSocket tests, redirect chains, TLS). Override `web::Data` in the test app to inject mock pools or test doubles.

## Common Pitfalls

Route handler closures passed to `.route()` must satisfy `Send + Sync + 'static` because Actix clones the `App` across worker threads. A handler that captures a non-`Send` type (e.g., `Rc<RefCell<T>>`) compiles in single-threaded tests but fails in production builds. Use `Arc` and `Mutex`/`RwLock` for shared mutable state.

Actix Web and the Actix actor framework are separate crates with different async runtimes. Importing `actix::prelude::*` alongside `actix_web` and mixing their spawn functions causes confusing runtime errors. Use `actix_web::rt` for web-related async work; only pull in the actor system if you need WebSocket actors or other actor-based features.

Payload size limits default to 256 KiB for JSON and 256 KiB for URL-encoded forms. Requests exceeding the limit return 413 with no body, which clients often misinterpret. Configure limits explicitly with `web::JsonConfig::default().limit(1_048_576)` registered via `.app_data()` and document the limits in your API contract.

Calling `.await` on a blocking operation (synchronous file I/O, CPU-heavy computation, synchronous database query) inside an async handler starves the tokio worker threads that Actix Web runs on. A single blocked handler under load cascades into request timeouts across unrelated endpoints. Move blocking work into `web::block` or a dedicated `spawn_blocking` task.

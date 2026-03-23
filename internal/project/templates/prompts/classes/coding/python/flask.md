# Flask

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Application Structure

Prefer the application factory pattern (`create_app()` returning a configured `Flask` instance) over a module-level `app = Flask(__name__)` global. The factory makes testing with different configurations straightforward and avoids circular imports between modules that need the app object. Register blueprints, extensions, and error handlers inside the factory. Prefer blueprints for organizing routes by domain; each blueprint owns its templates, static files, and URL prefix. Call `app.register_blueprint()` explicitly in the factory, and keep blueprints importable without triggering app creation.

## Configuration

Prefer `app.config.from_object()` with a config class per environment (`DevelopmentConfig`, `ProductionConfig`) over flat dictionaries or scattered `app.config["KEY"]` assignments. Use `app.config.from_prefixed_env()` (Flask 2.2+) to load environment variables with a consistent prefix. Keep secrets (database URIs, API keys) out of config classes; load them from environment variables or a `.env` file with `python-dotenv`. Access config values through `current_app.config` inside request handlers, not by importing the config module directly.

## Request and Application Context

Prefer `current_app` over importing the app instance when accessing configuration or extensions inside views and helpers. Use `g` for storing per-request state (database connections, authenticated user), not for data that should survive across requests. When writing CLI commands or background workers that run outside a request, push an application context manually with `with app.app_context():`. Prefer passing dependencies explicitly to utility functions rather than relying on `current_app` deep in the call stack, which couples library code to Flask's context machinery.

## Database Integration

Prefer Flask-SQLAlchemy's `db.init_app(app)` pattern in the factory, with models defined against a module-level `db = SQLAlchemy()` instance. Use Flask-Migrate (Alembic wrapper) for schema changes; generate with `flask db migrate`, review the output, then apply with `flask db upgrade`. Prefer scoped sessions (Flask-SQLAlchemy's default) that bind session lifetime to the request. Call `db.session.commit()` explicitly rather than relying on teardown hooks for writes; teardown should handle cleanup, not business logic.

## Templates

Prefer Jinja2 template inheritance (`{% extends "base.html" %}` with `{% block %}`) over duplicating layouts. Use `url_for()` in templates for all URL references, including static files (`url_for('static', filename='style.css')`). Prefer passing explicit context variables to `render_template()` over injecting globals; use `@app.context_processor` only for values genuinely needed on every page (current user, site name). Keep logic out of templates; complex filtering or formatting belongs in the view or a custom Jinja2 filter registered with `@app.template_filter()`.

## Error Handling and Middleware

Prefer `@app.errorhandler(404)` and `@app.errorhandler(Exception)` for centralized error responses. For API applications, register error handlers that return JSON with appropriate status codes rather than rendering HTML templates. Prefer `@app.before_request` and `@app.after_request` hooks for cross-cutting concerns (authentication checks, response headers, timing). Use `@app.teardown_appcontext` for resource cleanup (closing connections) rather than `after_request`, which doesn't run when an unhandled exception occurs.

## Async Support

Flask 3.x supports async views (`async def`), async error handlers, and async before/after request hooks. Async views work under both WSGI and ASGI, but WSGI mode runs them in a thread executor with overhead. For applications that are primarily async, prefer Quart (an ASGI reimplementation of Flask with the same API) or consider FastAPI. Flask's async support is adequate for occasional async handlers in an otherwise sync application.

## Testing

Prefer the `app.test_client()` fixture for HTTP-level integration tests. Create a test-specific factory configuration (`create_app(testing=True)` or `create_app(config_class=TestConfig)`) with `TESTING=True`, an in-memory or disposable database, and disabled CSRF protection. Use `app.test_request_context()` when testing functions that depend on `request`, `g`, or `current_app` without making a full HTTP request. Prefer `pytest` fixtures that yield an app, a client, and a database session scoped to each test, rolling back transactions between tests. Use `client.get()`, `client.post()` and assert on `response.status_code` and `response.get_json()`.

## Common Pitfalls

Accessing `current_app`, `request`, `g`, or `session` outside a request or application context raises a `RuntimeError` ("Working outside of application context"). This surfaces in CLI commands, Celery tasks, background threads, and test helper functions called without a context fixture. Push an application context explicitly, or restructure the code to receive dependencies as arguments.

Circular imports arise when a module that defines routes imports the app instance, which in turn imports the routes module at creation time. The application factory pattern solves this: routes live in blueprints that import only `flask.Blueprint`, and the factory imports and registers them. If extensions need the app, use `ext.init_app(app)` in the factory rather than passing `app` to the constructor at module level.

Flask's `g`, `request`, and `session` are thread-local proxies, not true globals. They are safe in threaded WSGI servers (each request gets its own context), but sharing state between requests through module-level variables is a race condition under concurrent load. For cross-request state, use a database, cache, or external store.

Forgetting to call `app.register_blueprint(bp)` after defining a blueprint is a silent failure: routes simply don't exist, and requests return 404 with no error at startup. Verify blueprint registration by checking `app.url_map` in a shell or test.

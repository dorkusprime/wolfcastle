# FastAPI

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Path Operations and Routing

Prefer organizing endpoints into `APIRouter` instances grouped by domain, then mounting them on the app with `app.include_router(router, prefix="/v1/items")`. Prefer explicit HTTP method decorators (`@router.get`, `@router.post`) over catch-all `@router.api_route`. Prefer returning Pydantic models directly from path operations; FastAPI handles serialization and generates OpenAPI schemas from the return type annotation. Use `status_code` on the decorator and `Response` model in the signature to keep the contract visible without reading the function body.

## Pydantic Models

Prefer Pydantic v2 `model_validator` and `field_validator` over the v1 `@validator`/`@root_validator` decorators. Use `model_config = ConfigDict(from_attributes=True)` (not the v1 `orm_mode = True`) when constructing models from ORM objects. Prefer separate request and response schemas even when the fields overlap; shared base classes are fine, but a single model serving both input and output couples validation rules to serialization shape. Use `Annotated[str, Field(min_length=1)]` for field constraints rather than bare `Field()` defaults, so the annotation carries the metadata cleanly.

## Dependency Injection

Prefer `Depends()` for shared logic (database sessions, authentication, pagination parameters) over manual calls inside the path operation. Prefer `Annotated[Session, Depends(get_db)]` type aliases for dependencies used across many endpoints; define these once and import them. Use `yield` dependencies for resources that need teardown (sessions, file handles, locks). Prefer request-scoped dependencies (the default) over caching with `use_cache=False` unless the dependency is intentionally stateless.

## Async vs Sync

Prefer `async def` for path operations that perform I/O through async-native libraries (httpx, asyncpg, aiofiles). Use plain `def` for CPU-bound or sync-only I/O; FastAPI runs sync path operations in a threadpool automatically. Never call blocking I/O (synchronous `requests.get`, `time.sleep`, sync database drivers) inside an `async def` path operation; it blocks the entire event loop. If a sync library is unavoidable, use `run_in_executor` or keep the path operation synchronous.

## Middleware and Exception Handlers

Prefer `@app.exception_handler(CustomError)` for domain-specific error responses. Prefer ASGI middleware (`BaseHTTPMiddleware` or pure ASGI callables) for cross-cutting concerns like request timing, correlation IDs, or CORS. Register middleware in reverse priority order; the last added middleware is the outermost wrapper.

## Background Tasks and WebSockets

Prefer FastAPI's `BackgroundTasks` parameter for fire-and-forget work that doesn't need monitoring (sending emails, flushing analytics). For work that needs retries, progress tracking, or persistence across restarts, prefer an external task queue (Celery, arq, Dramatiq). For WebSocket endpoints, prefer `@app.websocket` with an explicit receive/send loop and structured exception handling around `WebSocketDisconnect`.

## Database Integration

Prefer async SQLAlchemy sessions (`AsyncSession` from `sqlalchemy.ext.asyncio`) with a `yield` dependency that commits on success and rolls back on exception. Prefer Alembic for migrations; generate them with `alembic revision --autogenerate`, review the generated SQL, then apply. Keep the Alembic `env.py` configured to use the same async engine factory as the application.

## Testing

Prefer `httpx.AsyncClient` with `ASGITransport` (or the `async_client` fixture pattern) over the older `TestClient` for async applications. Use `app.dependency_overrides` to swap real dependencies (database sessions, external API clients) with test doubles. Prefer `pytest-asyncio` with `asyncio_mode = "auto"` in `pyproject.toml` so async tests run without manual `@pytest.mark.asyncio` on every function; the default mode is `strict`, which requires explicit markers. Use a separate test database; create and drop tables per session or per test via fixtures, not shared mutable state.

## Common Pitfalls

Calling `requests.get()`, `open()`, or any synchronous blocking call inside an `async def` endpoint freezes the event loop for all concurrent requests. The application appears to hang under load with no obvious error. Use async equivalents or move the operation to a sync `def` endpoint.

Pydantic v2 changed validation semantics: `@validator` is `@field_validator` with `mode="before"` or `mode="after"`, `@root_validator` is `@model_validator`, and `Config` inner classes are replaced by `model_config = ConfigDict(...)`. Mixing v1 and v2 APIs produces silent validation failures or confusing deprecation errors at startup. FastAPI has dropped Pydantic v1 support entirely; the minimum is now `pydantic>=2.7.0`. Use `model_validate()` and `model_dump()` instead of the v1 `.parse_obj()` and `.dict()` methods.

Dependencies using `yield` must not catch and suppress exceptions before the `yield` point; FastAPI relies on exception propagation to trigger proper HTTP error responses. A bare `except Exception` around the `yield` in a database session dependency silently swallows request errors and returns 500s with no traceback.

`Depends()` caches the return value per-request by default. Two parameters both depending on `get_db` receive the same session instance. This is usually correct for database sessions but surprising for dependencies that should return fresh values on each injection. Use `use_cache=False` when each injection point needs its own instance.

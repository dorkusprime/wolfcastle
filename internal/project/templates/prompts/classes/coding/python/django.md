# Django

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Project Structure

Prefer small, focused apps organized around a single domain concept. Each app owns its models, views, URLs, and tests. Wire apps into the project through `INSTALLED_APPS` and a root `urls.py` that includes each app's URL module under a clear prefix. Keep `settings.py` split by environment (`base.py`, `dev.py`, `prod.py`) using a `settings/` package when the project outgrows a single file.

## ORM

Prefer querysets over raw SQL. Chain `.filter()`, `.exclude()`, and `.annotate()` to build queries declaratively. Use `select_related()` for foreign key and one-to-one relationships fetched in the same query, and `prefetch_related()` for many-to-many and reverse foreign key relationships fetched as a second query. Use `F()` expressions for database-level field references in updates and annotations, and `Q()` objects for complex lookups with `|` (OR) and `~` (NOT).

Prefer `bulk_create()` and `bulk_update()` over loops of `.save()` calls when operating on multiple rows. Use `iterator()` on large querysets to avoid loading the entire result set into memory.

Prefer writing migrations with `makemigrations` and reviewing the generated SQL with `sqlmigrate` before applying. Use `RunPython` operations sparingly in data migrations; they run in a transaction on databases that support DDL transactions (PostgreSQL) but not on others (MySQL).

## Views

Prefer class-based views when the project uses them consistently, and function-based views when simplicity matters more than inheritance. For API work, prefer Django REST Framework's `ModelSerializer` for CRUD resources, `APIView` or `@api_view` for custom endpoints, and viewset/router combinations when the resource maps cleanly to REST semantics. Prefer `Serializer.validated_data` over manual `request.data` access for input handling.

## Templates and Middleware

Prefer template inheritance (`{% extends "base.html" %}`) with `{% block %}` regions over duplication. Use `{% include %}` for reusable fragments. Prefer the `{% url %}` tag over hardcoded paths.

Prefer ordering middleware carefully in `MIDDLEWARE`: security middleware first, session and authentication in the middle, application-specific middleware last. Custom middleware should be a function-based middleware factory (the single-callable pattern) unless the project uses the older class-based `MiddlewareMixin` style.

## Signals

Prefer signals (`post_save`, `pre_delete`, `request_finished`) only for decoupled cross-cutting concerns where the sender should not know about the receiver. For logic tightly coupled to a model, prefer overriding the model's `save()` or `delete()` methods, or calling the logic explicitly in the view. Signals that modify the same model they listen on are a frequent source of infinite recursion and ordering bugs.

## Testing

Prefer `TestCase` (wraps each test in a transaction rollback) for most database tests. Use `TransactionTestCase` only when the test needs to observe real commits, such as testing `on_commit` hooks or database constraints that require committed state. Prefer `Client` for integration-level HTTP tests and `RequestFactory` for unit-testing individual views without middleware. Prefer `factory_boy` factories over fixtures for test data; factories compose, override cleanly per-test, and avoid the brittleness of static JSON/YAML fixtures that break on schema changes. Use `@override_settings()` to modify configuration within a single test rather than patching `django.conf.settings` directly.

## Common Pitfalls

N+1 queries emerge whenever a template or serializer accesses a related object inside a loop without `select_related`/`prefetch_related` on the original queryset. Use `django-debug-toolbar` or `assertNumQueries()` in tests to catch these early.

Migration conflicts occur when two branches add migrations to the same app concurrently. Resolve by running `makemigrations --merge` on the combined branch; do not renumber or delete either migration manually.

Circular imports between apps surface when `models.py` in app A imports from app B and vice versa. Prefer string-based `ForeignKey` references (`"appname.ModelName"`) to break the cycle, and move shared utilities to a separate app or module.

Mutable defaults on model fields (`JSONField(default={})`) share the same dict instance across all unsaved instances. Always use a callable: `JSONField(default=dict)`.

Timezone-naive `datetime.now()` silently produces incorrect timestamps when `USE_TZ = True`. Prefer `django.utils.timezone.now()` everywhere. Store and compare datetimes as UTC; convert to local time only at the display boundary.

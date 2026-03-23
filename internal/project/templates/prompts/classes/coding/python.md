# Python

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer type hints on public API boundaries: function signatures, class attributes, and return types. Internal locals rarely need annotation. On Python 3.14+, annotations are evaluated lazily by default (PEP 649/749), so `from __future__ import annotations` is unnecessary and will eventually be deprecated. On 3.13 and earlier, `from __future__ import annotations` still works for postponed evaluation. Prefer the PEP 695 `type` statement for type aliases (`type Vector = list[float]`) and the new generic syntax (`def first[T](items: list[T]) -> T`) on Python 3.12+.

Prefer `dataclasses.dataclass` for internal structured data with known fields where the inputs are trusted. Use Pydantic `BaseModel` when data crosses a trust boundary (API inputs, config files, user-supplied data) and needs validation or coercion. Use `attrs` when the project already depends on it or when you need composable validators and slotted classes. Reserve plain classes for types with significant behavior or non-trivial initialization.

Prefer context managers (`with` statements) for any resource that needs cleanup: files, locks, database connections, temporary directories. Write custom context managers with `contextlib.contextmanager` when a class-based `__enter__`/`__exit__` pair is overkill.

Prefer comprehensions and generator expressions over `map`/`filter` with lambdas. `[x.name for x in users if x.active]` reads more naturally than `list(filter(lambda x: x.active, map(lambda x: x.name, users)))`. When comprehensions grow beyond a single condition or transformation, extract a helper function.

Prefer `pathlib.Path` over `os.path` for filesystem operations. `Path` objects compose cleanly (`base / "sub" / "file.txt"`), expose a readable API (`path.read_text()`, `path.exists()`), and avoid the string-concatenation traps of `os.path.join`.

Prefer f-strings for string interpolation. They're faster than `.format()` and more readable than `%`-formatting. For log messages that may not be evaluated, use lazy formatting (`logger.debug("found %d items", count)`) to avoid unnecessary string construction.

Prefer `Enum` and `StrEnum` over bare string constants for fixed sets of values. Enums catch typos at attribute access time rather than at runtime comparison.

Prefer raising specific exceptions with context over generic ones. `raise ValueError(f"port must be 1-65535, got {port}")` tells the caller what went wrong. Bare `raise Exception("error")` does not.

## Build and Test

Prefer the project's existing dependency and build tooling. Look for `pyproject.toml` (uv, pip, poetry, hatch, pdm), `setup.py`/`setup.cfg`, or a `Makefile`. When starting fresh, `pyproject.toml` with `uv` is the current standard. `uv` handles package installation, virtual environments, Python version management, and lockfiles in a single fast binary (10-100x faster than pip). Poetry remains a strong choice for library publishing. `setup.py` alone is legacy.

Prefer ruff for both linting and formatting. It replaces flake8, isort, black, and several other tools in a single fast binary. Check for a `[tool.ruff]` section in `pyproject.toml` or a `ruff.toml` file. When the project uses black, flake8, or isort, follow those.

Prefer mypy or pyright for static type checking. Check for a `[tool.mypy]` section in `pyproject.toml` or a `mypy.ini`/`pyrightconfig.json` file. Run the type checker before committing; type errors caught statically are cheaper than runtime `TypeError` surprises.

## Testing

Prefer pytest as the test runner. It discovers tests by file and function name (`test_*.py`, `def test_*()`), provides clear assertion introspection, and supports fixtures, parametrization, and plugins.

Prefer `@pytest.mark.parametrize` for table-driven tests. Name parameters clearly so that `pytest -v` output reads as documentation. When the parameter set is large or computed, use a list of `pytest.param(..., id="case-name")` tuples.

Prefer fixtures over `setUp`/`tearDown`. Fixtures compose naturally (`def test_user(db, factory):`), support scoping (`session`, `module`, `function`), and handle teardown via `yield`. Put shared fixtures in `conftest.py` at the appropriate directory level.

Prefer `tmp_path` (function-scoped) and `tmp_path_factory` (session-scoped) over `tempfile` for temporary files and directories in tests. Pytest cleans them up and provides unique directories per test.

Prefer `unittest.mock.patch` or `monkeypatch` for isolating external boundaries (network, filesystem, environment variables). Mock at the boundary where the dependency is imported, not at its definition site: `patch("mymodule.requests.get")`, not `patch("requests.get")`.

Prefer `pytest-asyncio` for testing async code. Set `asyncio_mode = "auto"` in `pyproject.toml` so async tests run without requiring `@pytest.mark.asyncio` on every function. The default mode is `strict`, which requires explicit markers.

## Common Pitfalls

Mutable default arguments are shared across all calls. `def append(item, lst=[])` reuses the same list object. Use `None` as the default and create the mutable inside the body: `if lst is None: lst = []`.

Late binding in closures captures the variable, not its value. A loop like `[lambda: i for i in range(3)]` produces three functions that all return `2`. Bind the value as a default argument (`lambda i=i: i`) or use `functools.partial`.

Bare `except:` catches `KeyboardInterrupt`, `SystemExit`, and `GeneratorExit` alongside actual errors. Prefer `except Exception:` at minimum, and prefer catching specific exception types when the failure mode is known.

Circular imports surface as `ImportError` or partially-initialized modules. They usually signal that two modules are too tightly coupled. Prefer extracting the shared dependency into a third module, using local imports inside functions, or restructuring the dependency graph.

Forgetting `__init__.py` in package directories causes `ModuleNotFoundError` in explicit namespace packages. While implicit namespace packages exist, most projects expect traditional packages with `__init__.py`. When adding a new sub-package, include one.

`is` compares identity, `==` compares equality. `x is None` is correct; `x is 0` or `x is ""` relies on CPython's small-integer and string interning, which is an implementation detail. Use `==` for value comparison.

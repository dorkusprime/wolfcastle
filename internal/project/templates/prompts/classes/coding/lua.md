# Lua

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer `local` for every variable and function declaration. Lua variables are global by default; an unqualified assignment at any scope creates or mutates a global. Treat any missing `local` as a bug.

Prefer tables as the primary data structure. Tables serve as arrays, dictionaries, objects, modules, and namespaces. Use integer keys for sequences and string keys for records; avoid mixing the two in a single table, since the length operator `#` only counts the array portion.

Prefer metatables and `__index` chaining for object-oriented patterns. A common idiom creates a "class" table, sets `Class.__index = Class`, and uses `setmetatable({}, Class)` as the constructor. Keep inheritance shallow; deep chains slow method lookup and obscure debugging.

Prefer coroutines (`coroutine.create`, `coroutine.resume`, `coroutine.yield`) for cooperative multitasking, generators, and state machines. Coroutines are stackful and first-class, making them more expressive than iterator closures for complex sequences.

Prefer `string.format()` for string building over repeated concatenation with `..`. Lua strings are immutable and interned; chaining `..` in a loop creates intermediate copies. For bulk assembly, collect pieces in a table and call `table.concat()`.

Prefer consistent 2-space or 4-space indentation (match the project). Use `snake_case` for locals, functions, and method names. Use `PascalCase` for module tables that act as classes. Use `UPPER_SNAKE` for constants.

## Build and Test

Prefer LuaRocks as the package manager. `luarocks install <rock>` fetches dependencies; `rockspec` files declare project metadata and dependencies. Pin versions in the rockspec for reproducible builds.

Prefer `busted` as the testing framework. Run `busted` from the project root; it discovers `*_spec.lua` files by convention. Prefer `luacheck` for static analysis: it catches globals, unused variables, and shadowing. When the project has a `.luacheckrc`, respect its settings.

Prefer `LuaFormatter` or `StyLua` for automated formatting when the project uses one. Neither is universal in Lua projects, so follow whatever the project has configured.

## Testing

Prefer `describe`/`it` blocks in busted for test organization. Write descriptions that read as sentences. Group related assertions inside a single `it` block rather than scattering them across blocks with identical setup.

```lua
describe("config parser", function()
  it("returns defaults for missing keys", function()
    local cfg = parse_config({})
    assert.are.equal(30, cfg.timeout)
    assert.is_true(cfg.verbose)
    assert.is_nil(cfg.cache_dir)
  end)
end)
```

Prefer typed assertions from `luassert`: `assert.are.equal()` for value equality, `assert.are.same()` for deep table comparison, `assert.has_error()` for error-throwing functions. Avoid bare `assert()` in tests; it gives no diagnostic output on failure.

Prefer `setup()` and `teardown()` inside `describe` blocks for shared fixture management. Use `before_each()` and `after_each()` when tests need isolated state. Busted supports `mock()` and `stub()` through luassert for replacing functions during tests; restore originals in `after_each` to prevent bleed.

## Common Pitfalls

A missing `local` silently creates a global variable. This is the single most common Lua bug. `luacheck` catches it; run it before every commit. In production code, consider setting a `__newindex` metamethod on `_G` that errors on unexpected global writes.

The length operator `#` is only reliable on sequences (tables with consecutive integer keys starting at 1, no `nil` holes). Inserting `nil` into a sequence breaks `#`, `ipairs`, and `table.sort`. Prefer `table.remove()` over assigning `nil` to an index, or track length explicitly.

Lua uses 1-based indexing. `t[1]` is the first element. Off-by-one errors are common when translating algorithms from 0-based languages. `ipairs` starts at 1 and stops at the first `nil`; `pairs` iterates all keys in undefined order.

Lua 5.1, 5.2, 5.3, 5.4, and LuaJIT differ in significant ways. Integer division (`//`) and bitwise operators exist in 5.3+. `setfenv`/`getfenv` exist in 5.1 and LuaJIT but were removed in 5.2. `goto` was added in 5.2. LuaJIT is 5.1-compatible with extensions (FFI, bit library) but does not support 5.2+ features. Check which runtime the project targets before using version-specific features.

Strings in Lua are immutable and interned. Every distinct string value has exactly one copy in memory, making string equality checks fast (pointer comparison), but string mutation impossible. Building strings incrementally with `..` in a loop is O(n²); collect into a table and `table.concat()`.

Error handling in Lua uses `pcall` and `xpcall` rather than try/catch syntax. Uncaught errors terminate the program. Prefer `xpcall` with a message handler to capture stack traces. Return `nil, err` from functions that can fail conventionally; reserve `error()` for programming mistakes and unrecoverable conditions.

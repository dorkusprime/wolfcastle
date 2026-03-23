# C++

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer RAII for resource management. Acquisition happens in the constructor, release in the destructor. If you're writing `delete`, `free`, `close`, or `release` outside a destructor, reconsider the design.

Prefer `std::unique_ptr` for sole ownership and `std::shared_ptr` for shared ownership. Raw owning pointers are almost never correct in modern C++. Non-owning raw pointers and references are fine when the pointed-to object's lifetime is clearly longer than the pointer's.

Prefer move semantics for transferring ownership of expensive-to-copy objects. Implement move constructors and move assignment operators when a class manages resources. Prefer pass-by-value-and-move for sink parameters that the function will store.

Prefer `constexpr` for values and functions that can be evaluated at compile time. `constexpr` functions serve as both compile-time and runtime functions, reducing duplication.

Prefer `enum class` over unscoped `enum`. Scoped enumerations prevent implicit conversions to integers and keep enumerator names out of the enclosing scope.

Prefer `auto` for local variables when the type is obvious from the right-hand side (iterator declarations, `make_unique` calls, cast results). Spell out the type when `auto` would hide information the reader needs.

Prefer `std::string_view` over `const std::string&` for function parameters that only read the string. It accepts both `std::string` and string literals without allocation.

Prefer the C++ Standard Library algorithms (`std::find`, `std::transform`, `std::sort`) and ranges (C++20/C++23) over hand-written loops when they express the intent more clearly. C++23 adds `std::views::zip`, `std::views::enumerate`, `std::views::chunk`, and other range adaptors that eliminate common manual patterns.

## Build and Test

Prefer CMake as the build system generator. Look for `CMakeLists.txt` at the project root. When the project uses Make directly, use the existing Makefile. When both exist, follow whatever the project's CI uses.

Prefer `clang-format` for code formatting. Check for a `.clang-format` file in the project root. Run it before committing.

Prefer `clang-tidy` for static analysis. Check for a `.clang-tidy` configuration. Its checks catch use-after-move, suspicious pointer arithmetic, modernization opportunities, and more.

Prefer `cppcheck` as a supplementary static analyzer when the project includes it in CI. It catches different classes of bugs than clang-tidy.

Prefer building with warnings enabled (`-Wall -Wextra -Wpedantic` for GCC/Clang). Treat compiler warnings as bugs; fix them, do not suppress them with pragmas unless the warning is provably wrong.

## Testing

Prefer whatever testing framework the project already uses. Google Test and Catch2 are the most common choices. Do not introduce a second framework.

Prefer `TEST` and `TEST_F` (Google Test) or `TEST_CASE` and `SECTION` (Catch2) for organizing tests into logical groups. Name tests for the behavior they verify.

Prefer CTest integration (`enable_testing()` and `add_test()` in CMake) so that `ctest` or `make test` runs the full suite from the build directory.

Prefer test fixtures (`TEST_F` with a fixture class, or Catch2 sections) when multiple tests share setup and teardown logic. Keep fixtures focused; a fixture that sets up the entire world is a sign the code under test has too many dependencies.

## Memory and Lifetime

Prefer stack allocation over heap allocation when the object's lifetime matches the enclosing scope. Stack objects are faster to allocate, automatically cleaned up, and cache-friendly.

Prefer `std::make_unique` and `std::make_shared` over direct `new` expressions. They are exception-safe and express intent.

Prefer references over pointers for parameters that must not be null. A reference communicates "this value is required" while a pointer communicates "this value is optional."

Prefer containers (`std::vector`, `std::array`, `std::string`) over raw arrays and C-style string functions. They manage memory, provide bounds-checking (via `.at()`), and integrate with the standard algorithms.

## Common Pitfalls

Undefined behavior is not "it crashes"; it is "anything can happen, including appearing to work." Signed integer overflow, dereferencing null or dangling pointers, reading uninitialized memory, and out-of-bounds access are all UB. Compilers optimize assuming UB never occurs, which can silently remove your safety checks.

Dangling references arise when a function returns a reference or pointer to a local variable, or when an iterator is used after the container has been modified (insertion, deletion, or reallocation). Prefer returning by value; move semantics make this cheap for most types.

Implicit conversions through single-argument constructors can cause surprising overload resolution. Prefer marking single-argument constructors `explicit` unless implicit conversion is genuinely intended.

The `#include` graph directly affects build times. Prefer forward declarations in headers when a full definition is not needed. Prefer including only what you use; transitive includes from other headers can break when those headers change.

Header-only libraries simplify distribution but increase compile times because the compiler processes the full implementation in every translation unit. For project-internal code, prefer separating declarations (`.h`/`.hpp`) from definitions (`.cpp`) unless the code is templates or trivial inline functions.

Order of initialization for static and global variables across translation units is unspecified. Prefer function-local statics (initialized on first use) or explicit initialization functions to avoid the "static initialization order fiasco."

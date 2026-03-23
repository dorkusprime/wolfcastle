# C

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Standard

C23 (ISO/IEC 9899:2024) is the current standard. GCC 15 made C23 the default dialect and has essentially feature-complete support. Clang 19+ supports most C23 features. Prefer `-std=c23` (or `-std=gnu23` for GNU extensions) when the project's toolchain supports it. Fall back to `-std=c17` for older compilers.

C23 adds `nullptr` and `nullptr_t` (prefer these over `NULL`), `constexpr` for compile-time object definitions, `typeof` and `typeof_unqual` for type inference, `auto` for object type inference, `_BitInt(N)` for bit-precise integers, and `#embed` for including binary resources directly in source. `bool`, `true`, and `false` are keywords, no longer requiring `<stdbool.h>`. Adopt these when your minimum compiler version supports them.

## Style

Prefer explicit memory management with clear ownership semantics. Every `malloc`, `calloc`, or `realloc` should have a single, obvious corresponding `free`. Document which function owns allocated memory and which callers are responsible for freeing it.

Prefer consistent error return patterns. Return an integer status code (0 for success, negative for errors) and pass results through output parameters, or return a pointer that is `NULL` on failure. Pick one convention per module and stick with it.

Prefer defensive initialization. Initialize all variables at declaration. Uninitialized stack variables contain garbage, and reading them is undefined behavior that sanitizers can miss in optimized builds.

Prefer `const` on pointer parameters that the function does not modify. `const char *name` tells the caller their data won't be changed and lets the compiler catch accidental writes.

Prefer `static` for file-scoped functions and variables. Internal linkage prevents symbol collisions across translation units and signals intent to the reader.

Prefer `size_t` for array indices, buffer lengths, and loop counters that interact with memory sizes. Signed/unsigned comparison mismatches are a persistent source of bugs.

Prefer `enum` or `constexpr` constants over magic numbers. In C23, `constexpr` object definitions replace many uses of `#define` for named constants. In pre-C23 code, `enum` or `#define` constants serve the same purpose.

Prefer `typedef struct { ... } FooBar;` or a project-consistent struct naming convention. Whichever the codebase uses, match it.

## Build and Test

Prefer Make or CMake as the build system. Look for a `Makefile` or `CMakeLists.txt` at the project root and use the existing build targets. When both exist, follow whatever CI uses.

Prefer compiling with warnings enabled: `-Wall -Wextra -Wpedantic` for GCC and Clang. Treat warnings as errors in CI (`-Werror`). Fix warnings rather than suppressing them.

Prefer `clang-format` for code formatting. Check for a `.clang-format` file in the project root. Run it before committing.

Prefer AddressSanitizer (`-fsanitize=address`) and UndefinedBehaviorSanitizer (`-fsanitize=undefined`) during development and testing. They catch buffer overflows, use-after-free, signed overflow, and null dereference at runtime with low overhead. ThreadSanitizer (`-fsanitize=thread`) catches data races in multithreaded code. MemorySanitizer (`-fsanitize=memory`, Clang only) catches reads of uninitialized memory. Combine sanitizers with static analysis for the best coverage.

Prefer Valgrind (`valgrind --leak-check=full`) for detecting memory leaks and invalid memory access when sanitizers are not available or when you need a second opinion.

Prefer `clang-tidy` for static analysis. It catches bugs the compiler and sanitizers miss (use-after-free patterns, resource leaks, suspicious logic, modernization opportunities). `cppcheck` is a useful supplement when the project includes it in CI.

## Testing

Prefer whatever testing framework the project already uses. Unity and CMocka are common choices for C projects. Do not introduce a second framework.

Prefer one assertion per logical check. Test functions should be small and named for the behavior they verify, so a failure immediately tells you what broke.

Prefer testing through public function interfaces rather than reaching into static internals. When you must test static functions, do so through a test-only header or a `#include` of the `.c` file in the test file, following the project's convention.

Prefer CTest integration (`enable_testing()` and `add_test()`) when the project uses CMake, so that `ctest` or `make test` runs the full suite.

## Memory and Resource Management

Prefer pairing allocation and deallocation in the same scope or the same module. When a function allocates memory the caller must free, document this in the function's header comment and name the function to suggest ownership transfer (e.g., `create_foo` / `destroy_foo`).

Prefer `calloc` over `malloc` followed by `memset` when zero-initialization is needed. `calloc` is a single call and handles the multiplication overflow check for you.

Prefer freeing resources in the reverse order of acquisition. This avoids use-after-free in cleanup paths where later resources depend on earlier ones.

Prefer a single cleanup label with `goto` for error-path resource release in functions that acquire multiple resources. This is idiomatic C and avoids deeply nested `if` chains or duplicated cleanup code.

## Common Pitfalls

Buffer overflows remain the most exploited class of C bugs. Prefer `snprintf` over `sprintf`, `strncpy` or `strlcpy` over `strcpy`, and always pass explicit buffer sizes. Check return values of functions that write to buffers.

Use-after-free occurs when a pointer is dereferenced after its memory has been freed. Prefer setting pointers to `NULL` after freeing them; a null dereference crashes immediately and visibly, while use-after-free can silently corrupt data.

Null dereference happens when code assumes a pointer is valid without checking. Prefer checking return values from `malloc`, `fopen`, and any function that can return `NULL` before using the result.

Signed integer overflow is undefined behavior in C. The compiler may optimize away overflow checks entirely. Prefer checking for overflow before the arithmetic operation, not after. For unsigned arithmetic, wraparound is defined but still usually a bug.

Missing include guards cause duplicate definition errors and subtle ODR violations. Prefer `#ifndef FOO_H` / `#define FOO_H` / `#endif` or `#pragma once` (widely supported, though not standard) in every header file.

POSIX portability varies across platforms. Prefer POSIX.1-2008 interfaces when possible, check `_POSIX_C_SOURCE` feature test macros, and avoid platform-specific extensions (GNU, BSD) unless the project explicitly targets a single platform.

Implicit function declarations (calling a function without a visible prototype) were removed in C99 and are an error in C23. Prefer compiling with `-std=c23` or `-std=c17` and `-Wpedantic` to catch these. A missing prototype means the compiler assumes the function returns `int` and accepts any arguments, which can silently corrupt the stack.

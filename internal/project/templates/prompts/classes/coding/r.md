# R

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer the tidyverse style guide for naming and formatting. Use `snake_case` for variables and functions, `PascalCase` for R6 classes and S4 classes. Keep line length under 80 characters.

Prefer the native pipe `|>` in R 4.1+ projects. In codebases targeting older R versions or using tidyverse extensively, `%>%` from magrittr is acceptable. Avoid mixing the two pipe styles within a single file.

Prefer tibbles (`tibble()`, `as_tibble()`) over `data.frame()` for interactive and pipeline-oriented work. Tibbles print more sanely, never convert strings to factors by default, and never create row names. In package code that minimizes dependencies, `data.frame()` is fine.

Prefer vectorized operations over explicit loops. `vapply()`, `sapply()`, and `lapply()` outperform `for` loops on large inputs and communicate intent more clearly. For complex transformations, prefer `purrr::map()` and its typed variants (`map_chr`, `map_dbl`, `map_lgl`) over the base `apply` family; the type guarantees prevent silent coercion.

Prefer tidy evaluation and the `{{ }}` (curly-curly) operator when writing functions that accept column names as arguments. Avoid `eval(parse())` patterns; they are fragile and obscure.

Prefer explicit namespace qualification (`dplyr::filter()`, `purrr::map()`) in package code to avoid conflicts with base R and other packages. In scripts and analyses, `library()` calls at the top of the file are conventional.

Prefer `<-` for assignment over `=` at the top level. Reserve `=` for named arguments inside function calls.

## Build and Test

Prefer `devtools` as the development workflow driver for R packages. `devtools::load_all()` reloads the package without reinstalling, `devtools::check()` runs `R CMD check` with sane defaults, and `devtools::document()` regenerates man pages from roxygen2 comments.

Prefer `roxygen2` for documentation. Write `@param`, `@return`, and `@examples` tags directly above each exported function. Run `devtools::document()` to generate the `NAMESPACE` and `.Rd` files; never edit those files by hand.

Prefer `lintr` for static analysis. When the project has a `.lintr` configuration file, respect its settings. Run `lintr::lint_package()` for packages or `lintr::lint("script.R")` for standalone files. Fix lints before committing.

Prefer `styler` for automated formatting. `styler::style_pkg()` reformats a package to tidyverse style; `styler::style_file()` handles individual scripts. Run the formatter before committing to avoid style-only diffs in future changes.

Prefer `renv` for dependency management. `renv::snapshot()` records the lockfile; `renv::restore()` reproduces the environment. Commit `renv.lock` to version control.

## Testing

Prefer `testthat` (edition 3) as the testing framework. Organize tests in `tests/testthat/` with filenames matching `test-*.R`. Each test file should mirror a source file: `R/parse.R` pairs with `tests/testthat/test-parse.R`.

Prefer `test_that()` blocks with descriptive strings that read as sentences. Group related expectations inside a single `test_that()` block rather than scattering them across multiple blocks with near-identical setup.

```r
test_that("parse_config returns defaults for missing keys", {
  result <- parse_config(list())
  expect_equal(result$timeout, 30)
  expect_true(result$verbose)
  expect_null(result$cache_dir)
})
```

Prefer typed `expect_*` assertions over generic `expect_true(x == y)`. Use `expect_equal()` for value comparison (with tolerance for floats), `expect_identical()` when type and attributes must match exactly, `expect_error()` / `expect_warning()` / `expect_message()` for condition testing, and `expect_snapshot()` for complex output that is easier to review visually than to assert field by field.

Prefer `withr::local_*()` and `withr::defer()` for temporary state changes in tests (environment variables, options, working directory). These clean up automatically when the test block exits, preventing cross-test contamination.

Prefer snapshot tests (`expect_snapshot()`) for printed output, error messages, and plots. Snapshot files live in `tests/testthat/_snaps/` and should be committed to version control. Review snapshot changes carefully during code review.

## Common Pitfalls

Non-standard evaluation (NSE) in tidyverse functions means bare column names are not regular variables. Writing `filter(df, x > 5)` inside a function where `x` is a parameter name causes confusion: R finds the column, not the parameter. Use `.data$x` to force column lookup, or `{{ x }}` to inject the caller's variable.

R uses 1-based indexing. `x[1]` is the first element, `x[length(x)]` is the last. Off-by-one errors are common when translating algorithms from 0-based languages. `seq_along(x)` and `seq_len(n)` are safer than `1:length(x)` or `1:n` because they handle zero-length inputs correctly (`1:0` produces `c(1, 0)`, not an empty vector).

`factor` and `character` behave differently in ways that surface late. Factors have fixed levels and integer storage; characters are free-form. `data.frame()` historically converted strings to factors (fixed in R 4.0 with `stringsAsFactors = FALSE` as default). When reading external data, verify column types early with `str()` or `glimpse()`.

R uses copy-on-modify semantics. Assigning `y <- x` does not copy immediately, but modifying `y` triggers a copy. In tight loops or large-object manipulation, unexpected copies can spike memory usage. Prefer in-place-friendly patterns: `data.table` for large dataset mutation, or pre-allocate vectors with `vector("double", n)` rather than growing them inside a loop.

`NA` propagates through most operations: `sum(c(1, NA, 3))` returns `NA`, not `4`. Functions that aggregate data almost always need `na.rm = TRUE`. Logical comparisons with `NA` return `NA`, not `FALSE`, so `if (x == NA)` never branches the way you expect; use `is.na(x)` instead.

Package namespace conflicts are silent and order-dependent. Loading `dplyr` after `stats` masks `stats::filter()` and `stats::lag()` with no error. The `conflicted` package turns these silent masks into hard errors at load time, forcing explicit resolution. In scripts where load order is uncertain, prefer `pkg::fun()` for any function name shared across loaded packages.

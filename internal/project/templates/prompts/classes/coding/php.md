# PHP

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer `declare(strict_types=1)` at the top of every file. Without it, PHP silently coerces scalar arguments to the declared type, turning type declarations into suggestions rather than contracts. Strict mode makes `foo(int $x)` actually reject a string.

Prefer type declarations on all function parameters, return types, and property types. Use union types (`string|int`), intersection types (`Countable&Iterator`), nullable (`?string`), and `mixed` when genuinely polymorphic. Return `void` explicitly on functions that produce no value. Use `never` for functions that always throw or exit.

Prefer readonly properties for value objects and DTOs. `public readonly string $name` communicates immutability without writing a getter. For entire value objects, prefer `readonly class` when every property should be immutable.

Prefer enums over class constants for fixed sets of values. Backed enums (`enum Status: string`) serialize cleanly and can implement interfaces. Avoid stringly-typed status fields when an enum fits.

Prefer `match` expressions over `switch` when returning a value or mapping input to output. `match` uses strict comparison, has no fall-through, and throws `UnhandledMatchError` on missing cases, catching bugs that `switch` silently passes.

Prefer named arguments for functions with boolean or optional parameters. `array_slice($arr, offset: 2, preserve_keys: true)` is self-documenting where positional arguments are not.

Prefer property hooks (PHP 8.4+) for computed or validated properties instead of explicit getter/setter pairs. Prefer asymmetric visibility (`public private(set) string $name`) for properties that should be publicly readable but only privately writable. Prefer the pipe operator `|>` (PHP 8.5+) for functional transformation chains.

Prefer PER Coding Style (the current successor to PSR-12) or the project's adopted standard. PER Coding Style extends PSR-12 with rules for modern PHP features: enums, attributes, match expressions, named arguments, and union/intersection types. Follow one class per file, `PascalCase` for classes, `camelCase` for methods, `SCREAMING_SNAKE_CASE` for constants. Declare `namespace` and `use` statements at the top, grouped and alphabetized.

Prefer early returns over deep nesting. Guard clauses at the top of a method (`if (!$user) { return null; }`) keep the happy path at the base indentation level.

## Build and Test

Prefer Composer for dependency management. Check for `composer.json` and `composer.lock`. Run `composer install` with the lock file; run `composer update` only when intentionally changing versions. Use PSR-4 autoloading; manual `require` chains are a maintenance trap.

Prefer PHPUnit as the test framework. Check for `phpunit.xml` or `phpunit.xml.dist` in the project root. Run `vendor/bin/phpunit` to execute the suite. When the project uses Pest instead, follow that; Pest wraps PHPUnit with a more expressive API.

Prefer PHP-CS-Fixer or PHP_CodeSniffer for automated style enforcement. Check for `.php-cs-fixer.dist.php` or `phpcs.xml`. Run the fixer before committing. When both exist, check the CI config to see which is authoritative.

Prefer PHPStan or Psalm for static analysis. Check for `phpstan.neon` or `psalm.xml`. Run at the project's configured level before committing. Level 0 catches almost nothing; level 6+ catches real bugs. When adding new code, write it to pass at the project's existing level without baseline additions.

## Testing

Prefer `describe`/`test` blocks in Pest or `test*` methods in PHPUnit, organized by class under test. Group related tests into a single test class matching the source class: `UserServiceTest` tests `UserService`.

Prefer data providers for table-driven tests. A `@dataProvider` method returns an array of named cases, keeping the test body focused on a single assertion pattern. Name each case descriptively so PHPUnit output reads as documentation.

Prefer testing behavior over internal wiring. Assert on return values, database state, and HTTP responses rather than on which private methods were called. Use `$this->createMock()` or Mockery/Prophecy at boundaries (HTTP clients, filesystems, external APIs), not for internal collaborators.

Prefer factory patterns or fixture helpers over raw object construction in tests. When the project uses Laravel, prefer model factories (`User::factory()->create()`). When using Doctrine, prefer fixture classes or a builder pattern that keeps test setup readable.

## Common Pitfalls

PHP's loose comparison (`==`) produces surprises: `0 == "foo"` was `true` before PHP 8.0, `"" == null` is `true`, and `"0" == false` is `true`. Prefer strict comparison (`===`) everywhere. The `match` expression uses strict comparison by default, which is one more reason to prefer it over `switch`.

Variable variables (`$$var`) and `extract()` create variables whose names are determined at runtime, making code impossible to trace statically. PHPStan and Psalm cannot follow them. Prefer explicit array access or object properties.

SQL injection in raw queries remains the most common PHP vulnerability. Prefer prepared statements with bound parameters (`$stmt->execute([$id])`) or the project's ORM query builder. Never interpolate user input into SQL strings, even behind an "internal" API.

PHP's standard library has inconsistent parameter ordering: `array_map($callback, $array)` but `array_filter($array, $callback)`. IDE autocompletion and strict types help, but be cautious when writing from memory. Check the signature when in doubt.

`include`/`require` at runtime with user-influenced paths is a remote code execution vector. Prefer Composer autoloading for class loading and explicit, hardcoded paths for configuration. Validate and sanitize any path component derived from input.

Error handling splits between exceptions and PHP's legacy error system. Prefer setting `error_reporting(E_ALL)` and converting errors to exceptions via `set_error_handler` or relying on the framework's error handler. Suppressing errors with `@` hides bugs and slows execution.

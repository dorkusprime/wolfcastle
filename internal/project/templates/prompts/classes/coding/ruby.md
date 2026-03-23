# Ruby

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer `# frozen_string_literal: true` at the top of every file. Without it, string literals are mutable by default, which wastes memory on duplicate allocations and invites accidental mutation bugs.

Prefer duck typing over explicit type checks. Test whether an object responds to a method (`respond_to?(:to_s)`) rather than checking its class (`is_a?(String)`). Ruby's power comes from protocols, not hierarchies.

Prefer blocks for single-use callbacks, procs for stored callable objects, and lambdas when you need arity checking and `return` that exits only the lambda. The differences are subtle: `Proc.new` and `proc {}` have loose arity and `return` exits the enclosing method; `lambda {}` and `->() {}` enforce arity and `return` exits the lambda.

Prefer `Enumerable` methods over manual loops. `select`, `map`, `reject`, `each_with_object`, `flat_map`, `group_by`, and `tally` express intent more clearly than `each` with an accumulator. Chain them when each step is simple; extract named methods when the chain exceeds two or three links.

Prefer convention over configuration. Follow Ruby's naming: `snake_case` for methods and variables, `CamelCase` for classes and modules, `SCREAMING_SNAKE_CASE` for constants. Predicate methods end with `?`, dangerous methods with `!`. Setters end with `=`.

Prefer `Struct` or `Data` (Ruby 3.2+) for simple value objects with named fields. Use full classes when behavior or validation is significant. Avoid `OpenStruct`; it's slow, creates methods via `method_missing`, and makes typos invisible. `Set` is a core class as of Ruby 4.0; no `require 'set'` needed.

Prefer YJIT for production workloads. Enable with `--yjit` or `RUBY_YJIT_ENABLE=1`. YJIT is mature and recommended over the newer ZJIT compiler (Ruby 4.0), which is still experimental. Ractors provide true parallelism but their API is still evolving (Ruby 4.0 introduced `Ractor::Port` for communication). Use Ractors for CPU-bound parallel work; prefer threads or async I/O for I/O-bound concurrency.

Prefer Sorbet or RBS for gradual type checking in larger codebases. Sorbet supports inline RBS comments as of 2025, allowing gradual migration from its proprietary `sig {}` syntax to Ruby's official type annotation format. Steep is a lighter-weight alternative for RBS-only type checking.

Prefer keyword arguments for methods with more than two parameters or any boolean parameters. `create_user(name:, admin: false)` is self-documenting at the call site.

Prefer raising specific exceptions with messages. `raise ArgumentError, "port must be 1-65535, got #{port}"` tells the caller what went wrong. Bare `raise "error"` creates a `RuntimeError` with no semantic meaning.

## Build and Test

Prefer Bundler for dependency management. Check for a `Gemfile` and `Gemfile.lock`. Run `bundle exec` before commands to use the locked gem versions. When adding gems, specify pessimistic version constraints (`~> 3.1`) to allow patch updates while preventing breaking changes.

Prefer Rake for task automation. Check for a `Rakefile` and look at existing tasks (`bundle exec rake -T`). Most projects define `rake test`, `rake spec`, or a `default` task that runs the test suite.

Prefer RuboCop for linting and style enforcement. Check for `.rubocop.yml` in the project root. Run `bundle exec rubocop` before committing. When the project uses Standard Ruby (`standardrb`) instead, follow that; it's RuboCop with a fixed configuration.

Prefer the project's existing test framework. Look for `spec/` directories (RSpec) or `test/` directories (Minitest). When both exist, check the `Gemfile` to see which is primary. When starting fresh, both are reasonable; RSpec is more common in Rails applications, Minitest ships with Ruby's standard library.

## Testing

Prefer `describe`/`context`/`it` blocks in RSpec for hierarchical test organization. `describe` names the unit under test, `context` establishes state ("when the user is an admin"), `it` states the expected behavior. The concatenation should read as a sentence.

Prefer `let` and `let!` for lazy and eager test setup. `let` memoizes per example and keeps setup close to usage. Use `subject` for the object under test when it makes assertions read naturally (`expect(subject).to be_valid`). Avoid `before` blocks that set instance variables when `let` would suffice.

Prefer factories (FactoryBot) over fixtures for test data. Factories compose and override cleanly (`create(:user, admin: true)`), while fixture files grow stale and create hidden coupling between tests. Use `build` or `build_stubbed` instead of `create` when persistence isn't needed, to keep tests fast.

Prefer testing behavior over implementation. Assert on return values, state changes, and side effects rather than on which internal methods were called. Use `have_received` matchers sparingly and only for verifying interactions at boundaries like external APIs.

## Common Pitfalls

Monkey patching core classes (`String`, `Array`, `Hash`) is easy in Ruby and almost always a mistake in application code. Patches create invisible coupling, confuse debugging, and conflict with gems that do the same. Prefer wrapper methods, refinements (Ruby 2.0+), or dedicated utility classes.

`method_missing` is powerful but treacherous. Every `method_missing` override must also override `respond_to_missing?`, or `respond_to?` checks will lie. Prefer explicit method definitions, delegation (`delegate`, `Forwardable`), or metaprogramming with `define_method` when the set of methods is known at load time.

Gem version conflicts surface as `Bundler::VersionConflict` or cryptic `LoadError` at runtime. Pin versions carefully in the `Gemfile`, run `bundle update <gem>` for targeted updates rather than `bundle update` (which updates everything), and read changelogs before major version bumps.

Require order matters. Unlike languages with package-level imports, Ruby loads files sequentially. A `require` that depends on a constant defined in another file only works if that file was required first. Prefer `require_relative` for intra-project files and ensure autoloading (Zeitwerk in Rails, or manual `autoload`) handles cross-cutting dependencies.

Strings are mutable by default (unless `frozen_string_literal` is enabled). Two string literals with identical content are distinct objects that can be mutated independently. This wastes memory in hot paths and enables accidental mutation. Prefer the frozen literal pragma, or call `.freeze` on string constants explicitly.

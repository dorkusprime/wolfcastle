# Sinatra

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Application Style

Prefer modular style (subclassing `Sinatra::Base`) over classic style (`require 'sinatra'` with top-level DSL) for any application beyond a single-file prototype. Classic style mixes route definitions into `Object`'s namespace, which pollutes every object in the process and breaks when multiple Sinatra apps coexist. Modular apps are also mountable as Rack endpoints, composable behind `Rack::URLMap` or inside a `config.ru`.

For new projects that need more structure than Sinatra but less than Rails, consider Roda. Roda uses a routing tree architecture (routes are matched dynamically as requests traverse the tree), offers better performance and lower memory usage than Sinatra, and has a rich plugin system. When the project already uses Sinatra, follow that.

## Routes and Parameters

Prefer explicit route ordering: Sinatra matches routes top-down and stops at the first match, so place specific patterns (`get '/users/search'`) before parameterized ones (`get '/users/:id'`). Use named parameters (`:id`) for path segments and `params[:key]` for query strings. Use `pass` to skip a route and continue matching. Prefer route conditions (`get '/feed', provides: 'rss'`) over manual content-type branching in the handler body.

## Middleware and Rack

Prefer `use MiddlewareName` inside a modular app class or `config.ru` for Rack middleware. Enable sessions with `enable :sessions` or, for production, `use Rack::Session::Cookie, secret: ENV.fetch('SESSION_SECRET')` with an explicit secret. The default session cookie has no secret in classic mode, which makes it trivially forgeable. Prefer `Rack::Protection` (included by default) for CSRF, path traversal, and session hijacking defenses; disable individual protections selectively rather than turning off the whole middleware.

## Templates and Rendering

Prefer `erb :template_name` or `haml :template_name` to render views from `views/`. Use layouts (`erb :index, layout: :application`) for shared page chrome. Prefer passing local variables to templates (`erb :show, locals: { user: user }`) over relying on instance variables, which leak state between helpers and routes. Inline templates (defined with `__END__` and `@@template_name`) are fine for single-file apps but don't scale beyond that.

## Configuration and Environments

Prefer `configure` blocks for environment-specific settings (`configure :production do; set :logging, true; end`). Access settings with `settings.option_name` inside routes and helpers. Use `set :option, value` for custom app-level state. Prefer environment variables for secrets rather than hardcoding values in `configure` blocks. Use `Sinatra::Base.development?`, `.production?`, or `.test?` for conditional logic. The environment defaults to `"development"` unless `APP_ENV` or `RACK_ENV` is set.

## Extensions and Helpers

Prefer `helpers` blocks for methods shared across routes (authentication checks, response formatting). Helpers run in the request scope and have access to `request`, `params`, `session`, and `halt`. Prefer `register` for extensions that add DSL methods, configuration, or lifecycle hooks to the app class. Write extensions as modules with a `self.registered(app)` method. Do not confuse `helpers` (instance methods available in routes) with plain Ruby methods defined outside the app class, which have no access to Sinatra's request context.

## Testing

Prefer `Rack::Test` via `include Rack::Test::Methods` and define `def app; MyApp; end` (returning the modular app class). Use `get '/path'`, `post '/path', params` to simulate requests. Assert on `last_response.status`, `last_response.body`, and `last_response.headers`. For session manipulation, use `env 'rack.session', { user_id: 1 }` in the request block or set session data through a login route. Prefer testing at the HTTP level (request in, response out) rather than calling route handler methods directly, which bypasses middleware, filters, and error handling.

## Common Pitfalls

Route ordering causes silent mismatches. A `get '/:slug'` defined above `get '/about'` swallows the `/about` request because `:slug` matches any single segment. Always order routes from most specific to least specific. Use `pass` if a parameterized route needs to conditionally defer.

Missing session middleware produces nil session values without raising errors. If `session[:key]` always returns nil, verify that sessions are enabled (`enable :sessions` or an explicit `Rack::Session` middleware). In modular apps, session support is not enabled by default.

Classic-style apps that `require 'sinatra'` add route methods to the top-level namespace. If another gem or file in the process also uses Sinatra classic style, routes from both files merge into a single application. Modular style avoids this entirely.

`halt` and `redirect` use throw/catch internally, so wrapping them in a `begin/rescue` block inside a route swallows the control flow and produces unexpected behavior. Keep `halt` and `redirect` calls outside of generic rescue clauses, or rescue specific exception classes rather than `Exception` or `StandardError`.

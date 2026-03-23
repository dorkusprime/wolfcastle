# Symfony

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Application Structure

Prefer Symfony 8.x with Flex recipes for bundle installation and auto-configuration. Organize code by domain under `src/` with subdirectories per bounded context rather than a flat `Controller/`, `Entity/`, `Repository/` layout once the application grows beyond a handful of entities. Prefer autowiring with `services.yaml` defaults (`_defaults: autowire: true, autoconfigure: true`) and let the container resolve dependencies by type. Register services explicitly only when autowiring is ambiguous (multiple implementations of one interface). Use compiler passes for container manipulation that cannot be expressed through configuration; avoid them when a tagged service or `#[AutoconfigureTag]` attribute suffices.

## Routing

Prefer PHP attributes (`#[Route('/users/{id}', methods: ['GET'])]`) on controller methods over YAML or XML routing files. Group related routes with a class-level `#[Route('/api/users')]` prefix. Use `#[MapQueryString]` and `#[MapRequestPayload]` for automatic request deserialization and validation rather than manually reading from the `Request` object. Prefer invokable controllers (`__invoke`) for single-action endpoints.

## Doctrine ORM

Prefer PHP attributes for entity mapping (`#[ORM\Entity]`, `#[ORM\Column]`) over XML or annotation comments. Use repository classes extending `ServiceEntityRepository` and inject them via constructor typing. Prefer DQL or the QueryBuilder for queries beyond simple `find`/`findBy`; use result set mapping or DTOs for read-heavy paths that do not need hydrated entities. Generate migrations with `doctrine:migrations:diff` and review the SQL before applying. Prefer one structural change per migration. Use `doctrine:schema:validate` in CI to catch mapping drift.

## Forms and Validation

Prefer Symfony form types (`AbstractType`) with `configureOptions` setting the `data_class`. Use `buildForm` to declare fields and rely on the Validator component constraints (`#[Assert\NotBlank]`, `#[Assert\Email]`) on the DTO or entity rather than duplicating rules in the form type. Prefer compound forms with `CollectionType` for nested collections. Use `form_themes` in Twig to customize rendering globally rather than inline overrides in each template.

## Events and Messenger

Prefer the EventDispatcher for synchronous, in-process hooks (kernel events, domain events). Use `#[AsEventListener]` attributes on listener methods instead of manual subscriber registration when each listener handles a single event. Prefer Symfony Messenger for asynchronous work: define message classes as simple DTOs, handlers as `#[AsMessageHandler]` services, and route messages to transports (`doctrine`, `amqp`, `redis`) in `messenger.yaml`. Use `stamps` for metadata (delay, priority). Configure retry strategies per transport with `max_retries` and `multiplier`; handle poison messages with the failure transport.

## Twig Templating

Prefer template inheritance (`{% extends 'base.html.twig' %}`) with named blocks over deep include chains. Use `{{ variable|escape }}` (auto-escaping is on by default; prefer leaving it enabled). Prefer Twig components or `{{ component() }}` (Symfony UX) for reusable UI fragments over macros when the fragment carries its own logic. Keep business logic out of templates; compute values in the controller or a Twig extension and pass them as variables.

## Testing

Prefer `KernelTestCase` for service-layer tests that need the container: call `self::getContainer()->get(ServiceName::class)` to retrieve services. Prefer `WebTestCase` with `$client = static::createClient()` for HTTP-level functional tests; use the crawler (`$crawler->filter('.title')->text()`) for HTML assertions and `$client->getResponse()->getStatusCode()` for status checks. Access the profiler and collector for debugging query counts or email sends in tests. Prefer `EntityManagerInterface` from the test container to seed data directly rather than fixtures for focused tests. Use `PHPUnit\Framework\TestCase` (without the kernel) for pure unit tests of domain logic with no framework dependencies. Prefer functional tests over unit tests for controller and form logic; the framework's test client makes integration assertions cheap.

## Common Pitfalls

Service autowiring ambiguity surfaces when two classes implement the same interface and autowiring cannot choose between them. Symfony throws a clear error at container compilation. Resolve with an alias in `services.yaml` (`App\Contract\Mailer: '@App\Infrastructure\SmtpMailer'`) or use `#[AsAlias]` on the preferred implementation. Do not suppress the error by making one service non-autoconfigured.

Doctrine's identity map caches entities for the lifetime of the `EntityManager`. In long-running processes (Messenger workers, daemon commands), stale data accumulates because the map never refreshes. Call `$entityManager->clear()` between message handling cycles, or configure the Messenger `doctrine_clear_entity_manager` middleware (enabled by default).

Form type inheritance complexity grows when extending form types through `getParent()` chains. Each layer can override options, transformers, and events in non-obvious ways. Prefer composition (embedding one form type inside another via `add()`) over deep inheritance. Reserve `getParent()` for genuine specialization of a built-in type.

Event subscriber ordering depends on the `priority` parameter (higher runs first, default is 0). When two listeners on the same event interact (one sets a value the other reads), an implicit ordering dependency exists. Prefer making the dependency explicit through a single listener that orchestrates both operations rather than relying on priority values that future developers may change without context.

YAML indentation in Symfony configuration files (`services.yaml`, `messenger.yaml`, `security.yaml`) is syntactically significant. A single misaligned space changes the structure silently, producing runtime errors far from the misconfigured line. Prefer validating configuration with `debug:config` or `lint:container` after changes. When configuration grows large, prefer splitting into multiple imported YAML files with clear separation of concerns.

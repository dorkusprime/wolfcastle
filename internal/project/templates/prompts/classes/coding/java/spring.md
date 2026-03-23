# Spring

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Application Structure

Prefer Spring Boot 4.x with the `@SpringBootApplication` entry point and auto-configuration. Organize packages by domain feature (`com.example.billing`, `com.example.shipping`) rather than by technical layer (`controllers`, `services`, `repositories`). Each feature package owns its controller, service, repository, and DTOs. Use starter dependencies (`spring-boot-starter-web`, `spring-boot-starter-data-jpa`) to pull consistent, tested dependency sets. Configure behavior through `application.yml` with profile-specific overrides (`application-dev.yml`, `application-prod.yml`). Prefer `@ConfigurationProperties` bound to a record or POJO over scattered `@Value` annotations; it groups related settings, supports validation with `@Validated`, and is testable without loading the full context.

Spring Boot 4 adopts Jakarta EE 11, Jackson 3 (with the `tools.jackson` package prefix replacing `com.fasterxml.jackson`), and JSpecify annotations for null safety. When migrating from Spring Boot 3.x, update Jackson imports project-wide and replace Spring's `@Nullable`/`@NonNull` with JSpecify equivalents. Add `@NullMarked` at the package level via `package-info.java` to declare non-null as the default.

## Virtual Threads

Prefer virtual threads for I/O-heavy services. Enable them with `spring.threads.virtual.enabled=true` (requires Java 21+). With virtual threads enabled, Spring MVC dispatches each request on a virtual thread, the embedded web server uses virtual threads, and task executors default to virtual thread creation. This eliminates the need to tune thread pool sizes for most web applications. Avoid `synchronized` blocks in request-handling code paths; prefer `ReentrantLock` to avoid pinning virtual threads to carrier threads.

## Dependency Injection

Prefer constructor injection over field or setter injection. A class with `final` fields and a single constructor needs no `@Autowired` annotation; Spring injects automatically. Constructor injection makes dependencies explicit, enables plain-object testing, and prevents circular dependency cycles at startup rather than hiding them until runtime. Use `@Component`, `@Service`, and `@Repository` stereotypes to communicate intent: `@Repository` activates exception translation for persistence, `@Service` marks domain logic boundaries. Prefer `@Bean` methods in a `@Configuration` class when the object requires custom setup or comes from a third-party library you cannot annotate.

## Web Layer

Prefer `@RestController` for JSON APIs and `@Controller` with view resolution for server-rendered HTML. Map endpoints with `@GetMapping`, `@PostMapping`, and siblings rather than the generic `@RequestMapping` with a `method` attribute. Return `ResponseEntity<T>` when the status code or headers vary by outcome; return the body type directly when the response is always 200. Prefer `@RequestBody` with a validated DTO (`@Valid`) over reading raw maps or JSON nodes. Use `@RestControllerAdvice` with `@ExceptionHandler` methods for centralized error responses rather than try-catch blocks in each controller.

Spring Framework 7 supports API versioning natively through path, header, query parameter, and media type strategies. When the project needs versioned endpoints, prefer the built-in versioning over manual path prefixes or custom `RequestCondition` implementations.

## Spring Data JPA

Prefer repository interfaces extending `JpaRepository<T, ID>`. Derive queries from method names (`findByEmailAndActiveTrue`) for simple lookups. Use `@Query` with JPQL for anything a method name can't express cleanly. Prefer Spring Data Specifications (`Specification<T>`) for dynamic, composable queries over building JPQL strings at runtime. Use `@EntityGraph` or `JOIN FETCH` in `@Query` to eager-load associations on a per-query basis rather than changing the entity's fetch type globally. Prefer projections (interface-based or record-based DTOs) when the caller needs a subset of columns, because projections avoid pulling and hydrating the full entity graph.

## Transaction Management

Prefer `@Transactional` on service methods that coordinate multiple repository calls, not on repository or controller methods. Use `readOnly = true` for queries to allow the persistence provider to skip dirty checking and flush. Prefer `@Transactional(propagation = REQUIRES_NEW)` sparingly and only when a nested operation must commit independently of the outer transaction (audit logging, for example). Be explicit about rollback: Spring rolls back on unchecked exceptions by default but commits on checked exceptions unless `rollbackFor` is specified.

## Security

Prefer Spring Security's `SecurityFilterChain` bean (configured via `HttpSecurity` in a `@Configuration` class). Use `requestMatchers()` with `permitAll()`, `authenticated()`, or `hasRole()` to declare access rules. Prefer method-level security (`@PreAuthorize("hasRole('ADMIN')")`) for fine-grained authorization on service methods. Use `BCryptPasswordEncoder` or `Argon2PasswordEncoder` for password hashing. When building stateless APIs, disable session creation (`SessionCreationPolicy.STATELESS`) and authenticate via JWT or OAuth2 resource server (`spring-boot-starter-oauth2-resource-server`).

## Testing

Prefer `@SpringBootTest` for full integration tests that load the entire context. Use `@WebMvcTest(SomeController.class)` with `MockMvc` for testing controllers in isolation without starting the server or loading the persistence layer. Use `@DataJpaTest` for repository tests; it auto-configures an embedded database, applies Flyway/Liquibase migrations, and wraps each test in a rollback transaction. Prefer `TestRestTemplate` or `WebTestClient` for tests that need a running server (embedded Tomcat). Prefer sliced test annotations over `@SpringBootTest` when the test scope is narrow; sliced tests start faster and isolate failures more clearly.

Use `@MockitoBean` (from Spring Framework) to replace specific beans in the test context. The older `@MockBean` from spring-boot-test is removed in Spring Boot 4. Note that `@MockitoBean` is not a drop-in replacement: it does not work on `@Configuration` classes or `@Component` classes the way `@MockBean` did. Use `@MockitoSpyBean` instead of the removed `@SpyBean`.

Prefer Testcontainers with the `@ServiceConnection` annotation for integration tests that need real infrastructure (databases, message brokers, caches). `@ServiceConnection` auto-configures the Spring connection properties from the running container, eliminating manual `@DynamicPropertySource` wiring. Annotate the container field with `@ServiceConnection` and Spring Boot maps it to the correct configuration properties automatically.

## Common Pitfalls

`@Transactional` only works on public methods invoked through the Spring proxy. A private `@Transactional` method does nothing. A public `@Transactional` method called from another method within the same class bypasses the proxy and runs without a transaction. Extract the transactional logic into a separate bean if self-invocation is needed.

Circular dependencies from field injection compile and may even start, but they produce fragile, order-dependent initialization. Constructor injection detects cycles immediately at startup. If a cycle surfaces, it signals a design problem: extract shared logic into a third bean or use `@Lazy` on one constructor parameter as a temporary measure.

N+1 queries with lazy-loaded `@OneToMany` or `@ManyToMany` associations appear when iterating over a collection outside the original query. Each access triggers a separate SELECT. Fix with `JOIN FETCH` in the query, `@EntityGraph`, or by switching to a DTO projection that assembles the data in one pass.

Bean scope confusion arises when a singleton bean holds a reference to a request- or session-scoped bean. The singleton captures a single instance at creation time and never sees fresh scoped instances. Prefer injecting a `Provider<T>` or `ObjectFactory<T>` to obtain scoped beans lazily, or use a scoped proxy (`@Scope(proxyMode = ScopedProxyMode.TARGET_CLASS)`).

Profile-dependent configuration drift occurs when `application-prod.yml` enables features or changes behavior that `application-dev.yml` does not. Tests pass locally against the dev profile while production breaks. Prefer keeping profiles limited to infrastructure differences (database URLs, credentials) and defining behavioral configuration in the base `application.yml` shared across all profiles.

Jackson 3 migration pitfalls: the package prefix changed from `com.fasterxml.jackson` to `tools.jackson`, and `ObjectMapper` is superseded by `JsonMapper`. Default serialization behavior changed in Jackson 3 (properties sort alphabetically, dates serialize as ISO-8601 strings instead of timestamps). Verify serialization output in tests after upgrading.

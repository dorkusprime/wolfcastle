// Package config handles loading, merging, and validating the Wolfcastle
// configuration. Configuration is resolved by deep-merging hardcoded
// defaults with the three-tier config files: base/config.json,
// custom/config.json, and local/config.json (ADR-018, ADR-053, ADR-063).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/tierfs"
)

// Defaults returns the hardcoded default configuration.
func Defaults() *Config {
	return &Config{
		Version: CurrentVersion,
		Models: map[string]ModelDef{
			"fast": {
				Command: "claude",
				Args:    []string{"-p", "--model", "claude-haiku-4-5-20251001", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"},
			},
			"mid": {
				Command: "claude",
				Args:    []string{"-p", "--model", "claude-sonnet-4-6", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"},
			},
			"heavy": {
				Command: "claude",
				Args:    []string{"-p", "--model", "claude-opus-4-6", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"},
			},
		},
		Pipeline: PipelineConfig{
			Planning: PlanningConfig{
				Enabled:         true,
				Model:           "heavy",
				MaxChildren:     10,
				MaxTasksPerLeaf: 8,
				MaxReplans:      3,
			},
			StageOrder: []string{"intake", "execute"},
			Stages: map[string]PipelineStage{
				"intake": {
					Model:           "mid",
					PromptFile:      "stages/intake.md",
					AllowedCommands: []string{"project create", "task add", "status"},
				},
				"execute": {
					Model:           "heavy",
					PromptFile:      "stages/execute.md",
					AllowedCommands: []string{"project create", "task add", "task block", "task deliverable", "audit breadcrumb", "audit escalate", "audit gap", "audit fix-gap", "audit scope", "audit summary", "audit resolve-escalation", "status", "adr create", "spec create", "spec link", "spec list"},
				},
			},
		},
		Logs: LogsConfig{
			MaxFiles:   100,
			MaxAgeDays: 30,
			Compress:   true,
		},
		Retries: RetriesConfig{
			InitialDelaySeconds: 30,
			MaxDelaySeconds:     600,
			MaxRetries:          -1,
		},
		Failure: FailureConfig{
			DecompositionThreshold: 10,
			MaxDecompositionDepth:  5,
			HardCap:                50,
		},
		Summary: SummaryConfig{
			Enabled:    true,
			Model:      "fast",
			PromptFile: "summary.md",
		},
		Docs: DocsConfig{
			Directory: "docs",
		},
		Validation: ValidationConfig{
			Commands: []ValidationCommand{},
		},
		Prompts: PromptsConfig{
			Fragments:        []string{},
			ExcludeFragments: []string{},
		},
		Daemon: DaemonConfig{
			PollIntervalSeconds:        5,
			BlockedPollIntervalSeconds: 5,
			InboxPollIntervalSeconds:   5,
			MaxIterations:              -1,
			MaxTurnsPerInvocation:      200,
			InvocationTimeoutSeconds:   3600,
			StallTimeoutSeconds:        120,
			MaxRestarts:                3,
			RestartDelaySeconds:        2,
			LogLevel:                   "info",
			Parallel: ParallelConfig{
				Enabled:    false,
				MaxWorkers: 3,
			},
		},
		Git: GitConfig{
			AutoCommit:            true,
			CommitOnSuccess:       true,
			CommitOnFailure:       true,
			CommitState:           true,
			CommitPrefix:          "wolfcastle",
			CommitMessageFormat:   "wolfcastle: {action} [{node}]",
			VerifyBranch:          true,
			SkipHooksOnAutoCommit: true,
		},
		Doctor: DoctorConfig{
			Model:      "mid",
			PromptFile: "doctor.md",
		},
		OverlapAdvisory: OverlapConfig{
			Enabled:   true,
			Model:     "fast",
			Threshold: 0.3,
		},
		Unblock: UnblockConfig{
			Model:      "heavy",
			PromptFile: "unblock.md",
		},
		Audit: AuditCommandConfig{
			Model:        "heavy",
			PromptFile:   "audits/audit.md",
			RequireTests: "block",
		},
		Knowledge: KnowledgeConfig{
			MaxTokens: 2000,
		},
		Archive: ArchiveConfig{
			AutoArchiveEnabled:    true,
			AutoArchiveDelayHours: 24,
			PollIntervalSeconds:   300,
		},
		TaskClasses: map[string]ClassDef{
			// Language classes
			"coding/python":     {Description: "Type hints, virtual environments, pytest, ruff/black, PEP 8"},
			"coding/javascript": {Description: "ESM vs CJS, Node vs browser, eslint, testing frameworks"},
			"coding/typescript": {Description: "tsconfig strictness, type-only imports, declaration files"},
			"coding/java":       {Description: "Maven/Gradle, JUnit, checked exceptions"},
			"coding/csharp":     {Description: ".NET SDK, NuGet, xUnit/NUnit, nullable reference types"},
			"coding/go":         {Description: "gofmt, go vet, table-driven tests, error wrapping"},
			"coding/rust":       {Description: "cargo clippy, ownership/borrowing guidance, Result/Option patterns"},
			"coding/cpp":        {Description: "CMake, clang-tidy, RAII, smart pointers, UB avoidance"},
			"coding/c":          {Description: "Makefile conventions, valgrind, buffer safety, POSIX portability"},
			"coding/ruby":       {Description: "Bundler, RSpec/minitest, Rubocop"},
			"coding/php":        {Description: "Composer, PHPUnit, PSR standards"},
			"coding/swift":      {Description: "Xcode/SPM, XCTest, optionals, protocol-oriented patterns"},
			"coding/kotlin":     {Description: "Gradle, JUnit/kotest, null safety, coroutine conventions"},
			"coding/scala":      {Description: "sbt, ScalaTest, functional patterns, implicits guidance"},
			"coding/shell":      {Description: "shellcheck, POSIX compatibility, quoting rules, set -euo pipefail"},
			"coding/sql":        {Description: "Dialect awareness (Postgres, MySQL, SQLite), migration patterns, injection prevention"},
			"coding/r":          {Description: "tidyverse conventions, testthat, roxygen2, CRAN packaging"},
			"coding/lua":        {Description: "LuaRocks, busted, metatables, embedding considerations"},
			"coding/elixir":     {Description: "mix, ExUnit, OTP patterns, pattern matching, pipe operator"},
			"coding/haskell":    {Description: "cabal/stack, HSpec, monadic patterns, type-driven development"},
			"coding/dart":       {Description: "pub, flutter test, null safety, widget patterns"},

			// Framework classes
			"coding/typescript/react":   {Description: "Hooks, JSX, React Testing Library, component patterns, state management"},
			"coding/typescript/vue":     {Description: "Composition API, SFCs, Pinia, Vue Test Utils, Vue Router"},
			"coding/typescript/angular": {Description: "Modules/standalone components, RxJS, dependency injection, Jasmine/Karma"},
			"coding/typescript/nextjs":  {Description: "App Router, Server Components, ISR/SSG/SSR, middleware, API routes"},
			"coding/typescript/svelte":  {Description: "Runes, load functions, form actions, server routes"},
			"coding/javascript/react":   {Description: "Same as TS/React but with PropTypes, no type annotations"},
			"coding/javascript/node":    {Description: "Express/Fastify patterns, middleware, async error handling, clustering"},
			"coding/python/django":      {Description: "MTV pattern, ORM, migrations, DRF, management commands, template conventions"},
			"coding/python/fastapi":     {Description: "Pydantic models, dependency injection, async endpoints, OpenAPI"},
			"coding/python/flask":       {Description: "Blueprints, extensions, application factory, Jinja2"},
			"coding/ruby/rails":         {Description: "Convention over configuration, ActiveRecord, concerns, RSpec Rails, generators"},
			"coding/ruby/sinatra":       {Description: "Lightweight routing, modular style, Rack middleware"},
			"coding/java/spring":        {Description: "Auto-configuration, annotations, JPA, Spring Security, integration testing"},
			"coding/csharp/dotnet":      {Description: "Minimal APIs, Entity Framework, middleware pipeline, Razor conventions"},
			"coding/php/laravel":        {Description: "Eloquent, Blade, artisan, service providers, feature tests"},
			"coding/php/symfony":        {Description: "Bundles, Doctrine, Twig, event system, PHPUnit bridge"},
			"coding/kotlin/android":     {Description: "Jetpack Compose, ViewModel, Room, coroutines, instrumented tests"},
			"coding/swift/ios":          {Description: "SwiftUI views, Combine, Core Data, XCUITest, App lifecycle"},
			"coding/dart/flutter":       {Description: "Widget tree, state management (Riverpod/Bloc), platform channels, widget tests"},
			"coding/elixir/phoenix":     {Description: "LiveView, Ecto, PubSub, Channels, ExUnit with Sandbox"},
			"coding/rust/actix":         {Description: "Extractors, middleware, app state, integration tests"},
			"coding/rust/tokio":         {Description: "Spawning, channels, select!, graceful shutdown, tracing"},

			// Non-language classes
			"architecture": {Description: "ADRs, dependency analysis, failure modes, decomposition"},
			"research":     {Description: "Source citation, accuracy over speed, structured output", Model: "fast"},
			"writing":      {Description: "Reader-first, concrete examples, scannable structure"},
			"design":       {Description: "User goals, interaction sequences, edge states"},
			"devops":       {Description: "Dockerfile, GitHub Actions, Terraform, deployment safety"},
			"data":         {Description: "Schemas, pipelines, validation, visualization"},
			"security":     {Description: "OWASP awareness, threat modeling, dependency auditing"},
			"testing":      {Description: "Coverage strategy, fixture design, flaky test prevention"},
			"audit":        {Description: "Read-only review, gap recording, no fixes"},
		},
	}
}

// configTiers returns the three-tier config file paths relative to the
// wolfcastle directory, in resolution order from lowest to highest priority.
// Derived from tierfs.TierNames, the single source of truth (ADR-063).
func configTiers() []string {
	paths := make([]string, len(tierfs.TierNames))
	for i, name := range tierfs.TierNames {
		paths[i] = tierfs.SystemPrefix + "/" + name + "/config.json"
	}
	return paths
}

// Load reads and merges configuration from the .wolfcastle directory.
// Resolution order: hardcoded defaults <- base/config.json <- custom/config.json <- local/config.json
func Load(wolfcastleDir string) (*Config, error) {
	// Start with defaults as raw map
	result, err := structToMap(Defaults())
	if err != nil {
		return nil, fmt.Errorf("marshaling defaults: %w", err)
	}

	var warnings []string

	// Overlay each tier in order, checking for unknown fields per tier
	for i, tier := range configTiers() {
		path := filepath.Join(wolfcastleDir, tier)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", tier, err)
		}

		// Check this tier's raw JSON for unknown fields
		tierLabel := tierfs.TierNames[i] + "/config.json"
		warnings = append(warnings, checkUnknownFields(data, tierLabel)...)

		var overlay map[string]any
		if err := json.Unmarshal(data, &overlay); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", tier, err)
		}
		result = DeepMerge(result, overlay)
	}

	// Apply schema migrations if the merged config is behind CurrentVersion.
	migrated, migrationDescs, migErr := MigrateConfig(result)
	if migErr != nil {
		return nil, fmt.Errorf("config migration: %w", migErr)
	}
	result = migrated
	for _, desc := range migrationDescs {
		warnings = append(warnings, "config migrated: "+desc)
	}

	// Marshal back to Config struct
	merged, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshaling merged config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(merged, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling merged config: %w", err)
	}

	cfg.Warnings = warnings

	// Validate structural integrity (skip identity; handled by resolver)
	if err := ValidateStructure(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// structToMap converts a struct to a map[string]any via JSON round-trip.
func structToMap(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshaling struct: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshaling to map: %w", err)
	}
	return m, nil
}

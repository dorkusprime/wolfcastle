# DevOps

When the project you're working in has established CI/CD pipelines, infrastructure conventions, or deployment practices that differ from what's described here, follow the project.

## Dockerfiles

**Use multi-stage builds to separate build dependencies from runtime.** The build stage installs compilers, build tools, and dev dependencies. The final stage copies only the compiled artifact into a minimal base image (distroless, Alpine, or the language's slim variant). A Go binary does not need gcc in production. A Node app does not need TypeScript in production. Every unnecessary package in the runtime image is attack surface and image bloat.

**Order layers from least to most frequently changing.** Docker caches layers sequentially; a change in one layer invalidates every layer after it. System dependencies change rarely and belong early. Dependency manifests (go.mod, package-lock.json, requirements.txt) change occasionally and go next. Application source code changes constantly and goes last. Copy the dependency manifest and install dependencies before copying source code. This single ordering decision can reduce rebuild times from minutes to seconds.

**Pin base image versions.** `FROM node:latest` is a build that works today and breaks tomorrow. Use a digest or a specific version tag. Document the pinning policy so images get updated deliberately, not accidentally.

## CI/CD Pipelines

**Make every pipeline step independently retriable.** A deployment pipeline that must restart from the beginning on a transient failure wastes time and masks the actual failure. Each step should be idempotent: running it twice produces the same result as running it once. Store intermediate artifacts explicitly rather than relying on implicit state from previous steps.

**Cache aggressively, invalidate precisely.** CI time is developer time multiplied by every push. Cache dependency downloads, build artifacts, and Docker layers. Key caches on the content hash of dependency files, not on branch names or timestamps. A cache that never invalidates wastes storage. A cache that invalidates too eagerly wastes time.

**Keep secrets out of build logs.** Mask environment variables that contain tokens, passwords, or API keys. Never echo secrets for debugging. Never pass secrets as command-line arguments (they appear in process listings). Use your CI platform's secret storage and inject values through environment variables or mounted files.

## Infrastructure as Code

**Treat infrastructure definitions as production code.** Version control, code review, automated testing, and change management apply to Terraform, CloudFormation, and Pulumi with the same rigor as application code. An unreviewed infrastructure change can bring down a system as effectively as a bad deploy.

**Minimize blast radius through resource isolation.** Separate state files by environment and by concern area. A single Terraform state that manages networking, compute, databases, and DNS means a misconfigured DNS record can block a database migration. Smaller state scopes limit the damage from any single apply and allow parallel changes by different teams.

**Plan before apply, always.** Review the diff before executing it. Automated pipelines should gate on plan approval for production changes. A plan that shows 47 resources being destroyed deserves a human looking at it before it runs.

## Deployment Safety

**Deploy progressively.** Canary deployments, blue-green switching, and rolling updates exist to limit exposure. Route a small percentage of traffic to the new version, observe metrics, and expand only when the signals are clean. A deployment strategy that sends 100% of traffic to untested code is not a deployment, it is an experiment on all your users simultaneously.

**Automate rollback and verify it works.** A rollback procedure that has never been tested is a hope, not a plan. Practice rollbacks in staging. Ensure the rollback path handles database migrations, cache invalidation, and feature flag state. If rolling back requires manual steps, document them and keep them current.

## Monitoring and Alerting

**Alert on symptoms, not causes.** Users experience high latency and errors, not high CPU usage. Alert on the conditions that affect user experience: error rate, response time, availability. Investigate causes after the symptom fires. An alert on CPU usage at 80% pages someone who may discover the system is performing perfectly well under legitimate load.

**Every alert must have a runbook.** An alert without a response procedure wakes someone up and leaves them guessing. The runbook describes: what the alert means, how to verify it is real, what to check first, and how to mitigate. Keep runbooks next to the alert definitions, and update them when the system changes.

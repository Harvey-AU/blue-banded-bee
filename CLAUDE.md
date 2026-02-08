# CLAUDE.md

This file is the agent-agnostic operating guide for this repository. It applies
to all agents and models (CLI, IDE, reviewer, CI bots). If an agent cannot
comply due to tooling limits, it must state the limitation and continue within
constraints.

**VERY IMPORTANT:** Use British (Australian) English in all outputs, code
comments, and strings.

**VERY IMPORTANT:** Review this file at the start of each new task, after major
iteration cycles, and before deploying.

Last reviewed: 2026-02-02

## Core Defaults

- Proceed without asking questions unless a decision is truly blocking.
- Ask exactly one targeted question only when ambiguity materially affects the
  result or the change is destructive or irreversible.
- Preserve functionality unless explicitly asked to remove or replace it.
- Prefer minimal, scoped changes over broad refactors.

## Safety and Permissions

- Do not alter production data or credentials.
- Do not deploy or push unless explicitly requested.
- Do not remove working functionality without permission.
- Never log secrets, credentials, JWTs, or end-user content.

## Workflow Reminders

- Use the six-step loop when preparing a PR or release: review docs → test
  locally → commit simply → push → monitor → test production.
- For small tasks, follow a lightweight loop: review relevant docs → change →
  test targeted → summarise.

## Technical Stack Preferences

- Prefer Supabase features over custom implementations whenever possible.
- The dashboard uses vanilla JavaScript without a build step; prefer Web
  Components where relevant and replace non-Web-Component UI when possible.
- Run a `docker build` before merging static asset changes (frontend assets,
  public files, or CSS/JS bundles).
- For any new top-level HTML page/route, update all three surfaces:
  - HTTP route registration in `internal/api/handlers.go`
  - page file on disk (for example `welcome.html`)
  - container packaging in `Dockerfile`
    (`COPY --from=builder /app/<page>.html .`) Missing the Dockerfile copy will
    cause a runtime 404 in Fly deployments.

## Auth Redirect Contract

- Social OAuth redirect targets are centralised in `web/static/js/auth.js`
  (`handleSocialLogin`), with CLI-specific override logic in `initCliAuthPage`.
- Deep-link behaviour: if the current URL contains path/query context (for
  example invite token links), OAuth must return to that exact URL.
- Homepage behaviour: auth started from `/` may route to the default app landing
  page rather than returning to `/`.
- Invite behaviour: after successful invite acceptance, route users to
  `/welcome`.
- Page-specific post-auth redirects are allowed when the page explicitly owns
  that flow.
- Active-organisation source of truth is backend API response
  (`GET /v1/organisations` -> `active_organisation_id`). Frontend state may
  cache this in local storage, but must not override backend truth.

## Git Commit Style

- Keep commit messages to five or six words, no AI attribution, and no footers
  (e.g. `Add user authentication`, `Fix API rate limiting`).

## Code Quality Standards

- **Linting is enforced in CI** - golangci-lint v2 runs on every PR and blocks
  merges if failing.
- **Australian English only** - All code, comments, and strings must use
  Australian spelling (enforced by `misspell` linter with `locale: AU`).
- **Cyclomatic complexity limit: 35** - Functions exceeding this fail CI;
  refactor using Extract + Test + Commit pattern.
- Run `gofmt` and `goimports` on modified Go packages before committing.
- **Linters:** `govet`, `staticcheck`, `errcheck`, `revive`, `ineffassign`,
  `gocyclo`, `misspell`.
- **Formatters:** `gofmt`, `goimports`.
- See `.golangci.yml` for full configuration and exclusion rules;
  `docs/development/DEVELOPMENT.md` for local Docker-based linting.

## Logging Standards

- **General guidance**
  - Use structured logging (`zerolog`) with contextual fields (job ID, domain,
    request ID) rather than string concatenation.
  - Prefer one high-quality log per meaningful event over streaming repetitive
    messages; avoid logging inside tight loops.
  - Never log secrets, Supabase credentials, JWTs, or end-user content.
  - When returning an error to callers, log once at the boundary that handles
    it. Downstream callers should rely on `fmt.Errorf("context: %w", err)`
    rather than double logging.
- **Level selection**
  - `Debug` (development only): temporary diagnostics; keep behind
    `LOG_LEVEL=debug` so production stays quiet.
  - `Info`: expected state changes that help operators follow the happy path.
  - `Warn`: unexpected or degraded behaviour that we recovered from; include the
    next action or retry detail so it is actionable.
  - `Error`: failures we cannot automatically correct; always attach errors via
    `.Err(err)` and capture via Sentry if escalation is required.
- **Sentry**
  - Capture only high-severity or security-relevant issues
    (`sentry.CaptureException(err)` or `CaptureMessage` for suspicious events).
  - Do not spam Sentry with transient warnings already handled by retries.
- **Request tracing**
  - Ensure API handlers log the `request_id` (already injected by middleware).

## Monitoring Utilities

- Use `scripts/monitor_logs.sh` for Fly log sampling and analysis. Default:
  10-second intervals for 4 hours.
- Output folder format: `logs/YYYYMMDD/HHMM_<name>_<interval>s_<duration>h/`.
  - Example: `logs/20251105/0833_heavy-load-test_10s_4h/`.
  - Raw logs: `raw/<timestamp>_iter<N>.log`.
  - JSON summaries: `<timestamp>_iter<N>.json`.
  - Aggregated: `time_series.csv` and `summary.md`.
- Automatic aggregation via `scripts/aggregate_logs.py` runs after each
  iteration.
- Usage:
  ```bash
  ./scripts/monitor_logs.sh                              # Default 4-hour run
  ./scripts/monitor_logs.sh --run-id "custom-name"      # With descriptive name
  ./scripts/monitor_logs.sh --interval 30 --iterations 120  # 30s for 1 hour
  ```

## Testing Approach

- Test locally first (`go test ./...`, targeted unit or integration suites,
  `docker build` when relevant), then rely on GitHub Actions.
- Confirm finished features meet the requirements before handoff.

## Documentation Reading Rules

- **Initial onboarding or large changes**: read the full mandatory list below.
- **Small or focused tasks**: read only the relevant docs and reference others
  as needed.

**Mandatory on first clone or major work:**

1. **CLAUDE.md** - Complete project guidance and workflow
2. **README.md** - Project overview and quick start
3. **CHANGELOG.md** - Recent changes and releases
4. **docs/architecture/ARCHITECTURE.md** - System design and components
5. **docs/architecture/DATABASE.md** - Schema and PostgreSQL features
6. **docs/architecture/API.md** - RESTful API reference
7. **docs/development/BRANCHING.md** - Git workflow and PR process
8. **Roadmap.md** - Upcoming work and priorities
9. **docs/TEST_PLAN.md** - Testing strategy and approach

**Additional references as needed:**

- **docs/development/DEVELOPMENT.md** - Development environment setup
- **SECURITY.md** - Security guidelines and considerations
- **docs/testing/** - Complete testing documentation (setup, CI/CD,
  troubleshooting)

## Project Overview

Blue Banded Bee is a web cache warming service built in Go, focused on Webflow
sites. It uses a PostgreSQL-backed worker pool architecture for efficient URL
crawling and cache warming.

## Refactoring Method (Extract + Test + Commit)

Use this when a function exceeds ~50 lines or complexity limits:

1. Identify distinct responsibilities and extract single-purpose functions.
2. Add table-driven tests for each extracted unit, including edge cases.
3. Keep behaviour identical and verify with targeted tests.
4. Commit each logical step with an atomic, reversible change.

## Database Schema Management

**IMPORTANT: We use Supabase's built-in migration system. Do not duplicate this
functionality in Go code.**

### How to Make Schema Changes

1. **Create a migration file**:

   ```bash
   supabase migration new descriptive_name_here
   # This creates: supabase/migrations/[timestamp]_descriptive_name_here.sql
   ```

2. **Write your SQL changes** in the migration file.
3. **Commit the migration** with your code changes.
4. **Push to your feature branch**.

Migrations apply automatically through the Supabase GitHub integration.

### Important Notes

- Do not edit or rename existing migration files after they are deployed.
- Keep migrations additive (ADD COLUMN, CREATE INDEX), not destructive.
- All data is currently test-only and can be deleted.

## CI, Workflows, and Integrations

- Check platform documentation first when errors occur.
- Prefer configuration changes before code changes.
- Use platform-provided solutions where available (e.g. Supabase pooler URLs for
  IPv4 constraints).
- Document integration requirements and constraints for CI/deployments.

## Communication Guidelines

- Be concise and direct unless a deep dive is requested.
- Provide a brief rationale for decisions, not full chain-of-thought.
- Reference specific files/lines when explaining findings.
- Answer direct questions directly.

## Optional Persona (Use When Appropriate)

These principles are helpful for hands-on coding agents. Skip when the agent or
environment does not support them.

- Own the tools: keyboard-first workflows, fast iteration, minimal context
  switching.
- Prove with data: profile and benchmark before optimising.
- Essential comments only: document invariants or sharp edges.
- Type with intent: strong typing, avoid `any` or stray `clone()`.

## Project Initialisation

**Set up pre-commit hooks on first clone:**

```bash
git config core.hooksPath .githooks
```

This enables automatic code formatting on every commit (gofmt for Go, prettier
for docs/config).

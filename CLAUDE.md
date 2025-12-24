# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

**VERY IMPORTANT:** Please write in British (Australian) English, not American
English.

**VERY IMPORTANT:** Always review [./Claude.md] each time you start new tasks or
after several rounds of iterating on a task, and before deploying.

## Development Preferences

- Treat this document as mandatory reading when starting, wrapping up, or
  publishing any work.
- Use British/Australian English spelling and terminology.
- Keep communication concise and direct unless the user explicitly asks for a
  deep dive.

## Workflow Reminders

- Follow the six-step loop: review the relevant docs → test locally → commit
  simply → push → monitor → test production.

## Technical Stack Preferences

- Prefer Supabase features over custom implementations whenever possible.
- The dashboard uses vanilla JavaScript without a build step; Web Components are
  legacy—touch them only when necessary.
- Run a `docker build` before pushing static asset changes.

## Git Commit Style

- Keep commit messages to five or six words, no AI attribution, and no footers
  (e.g. `Add user authentication`, `Fix API rate limiting`).

## Code Quality Standards

- **Linting is enforced in CI** - golangci-lint v2 runs on every PR and blocks
  merges if failing
- **Australian English only** - All code, comments, and strings must use
  Australian spelling (enforced by `misspell` linter with `locale: AU`)
- **Cyclomatic complexity limit: 35** - Functions exceeding this fail CI;
  refactor using Extract + Test + Commit pattern (see below)
- **Always run `go fmt ./...`** before committing to avoid formatting failures
- **Linters:** `govet`, `staticcheck`, `errcheck`, `revive`, `ineffassign`,
  `gocyclo`, `misspell`
- **Formatters:** `gofmt`, `goimports`
- See `.golangci.yml` for full configuration and exclusion rules;
  `docs/development/DEVELOPMENT.md` for local Docker-based linting

## Logging Standards

- **General guidance**
  - Use structured logging (`zerolog`) with contextual fields (job ID, domain,
    request ID) rather than string concatenation.
  - Prefer one high-quality log per meaningful event over streaming repetitive
    messages; avoid logging inside tight loops.
  - Never log secrets, Supabase credentials, JWTs, or end-user content.
  - When returning an error to callers, log once at the boundary that handles
    it—downstream callers should rely on `fmt.Errorf("context: %w", err)` rather
    than double logging.
- **Level selection**
  - `Debug` (only in development): noisy instrumentation or temporary
    diagnostics; keep behind `LOG_LEVEL=debug` so production stays quiet.
  - `Info`: expected state changes that help operators follow the happy path
    (job created, worker pool started, external webhook received). Include
    enough fields for correlation and avoid chatting every iteration.
  - `Warn`: unexpected or degraded behaviour that we recovered from (retry
    scheduled, third-party rate limiting, fallback sitemap path). Use it
    sparingly and add next action or retry detail so it is actionable.
  - `Error`: failures we cannot automatically correct (request returning 500,
    database write rejected, worker panic). Always attach the error via
    `.Err(err)` and capture via Sentry if escalation is required.
- **Sentry**
  - Capture only high-severity or security relevant issues
    (`sentry.CaptureException(err)` or `CaptureMessage` for suspicious events).
    Do not spam Sentry with transient warnings already handled by retries.
  - High-severity covers infrastructure faults or events that prevent users
    accessing the product (e.g. signup/auth creation failing, database
    unavailable). Skip Sentry for routine validation errors or recoverable
    retries.
- **Request tracing**
  - Ensure API handlers log the `request_id` (already injected by middleware) so
    support can correlate client reports to backend events.

## Monitoring Utilities

- Use `scripts/monitor_logs.sh` for Fly log sampling and analysis. Default:
  10-second intervals for 4 hours.
- Output folder format: `logs/YYYYMMDD/HHMM_<name>_<interval>s_<duration>h/`
  - Example: `logs/20251105/0833_heavy-load-test_10s_4h/`
  - Raw logs: `raw/<timestamp>_iter<N>.log`
  - JSON summaries: `<timestamp>_iter<N>.json`
  - Aggregated: `time_series.csv` and `summary.md`
- Automatic aggregation via `scripts/aggregate_logs.py` runs after each
  iteration
- Usage:
  ```bash
  ./scripts/monitor_logs.sh                              # Default 4-hour run
  ./scripts/monitor_logs.sh --run-id "custom-name"      # With descriptive name
  ./scripts/monitor_logs.sh --interval 30 --iterations 120  # 30s for 1 hour
  ```

## Testing Approach

- Test locally first (`go test ./...`, targeted unit or integration suites,
  `docker build`), then rely on GitHub Actions.
- Confirm finished features meet the requirements before handing off.

## Development Persona

### Primeagen Playbook

- **Own the tools**: Master modal editing (Neovim) with purposeful keymaps,
  custom Telescope/Harpoon workflows, `:Make` integrations, and tmux panes; keep
  workflows keyboard-first and deterministic.
- **Prove with data**: Profile (`perf`, `pprof`, Flamegraph) and benchmark
  (`cargo bench`, `go test -bench`) before optimising or declaring hotspots.
- **Tight feedback loops**: Trigger builds and tests from the editor, surface
  results in quickfix windows, iterate instantly, and avoid context switching.
- **Essential comments only**: Prefer self-explanatory code; when commenting,
  capture invariants, assumptions, or sharp edges—no filler.
- **Type and borrow with intent**: Embrace strong typing (`TypeScript` `never`,
  `satisfies`), respect ownership/borrowing in Rust, and avoid stray `clone()`
  or `any`.
- **Test strategically**: Write tests for critical paths and complex logic
  (`cargo test`, `go test ./...`, `npm test`), vet snapshot updates carefully,
  and map failures to fast reruns (e.g. `<leader>t`).
- **Algorithmic literacy**: Re-derive core patterns (binary search, sliding
  windows, ring buffers) so solutions rest on understanding, not cargo-culting.
- **Transparent debugging**: Log state thoughtfully (`dbg!`, structured logs),
  narrate why fixes work, and never rely on “it just works”.

## SESSION START PROTOCOL

Complete this mandatory checklist before any work:

### 1. Review Context

Check any available context, preferences, or previous work related to this task.

### 2. Understand Current State

Assess what currently exists and what's working in the codebase.

### 3. Critical Session Reminders

- **INVESTIGATE FIRST** - Don't assume things are broken
- **QUALITY OVER SPEED** - Take time to understand code, logic and details
  thoroughly
- **FIND ROOT CAUSES** - Work to figure out true underlying issues, not surface
  symptoms
- **EXPLAIN YOUR REASONING** - Walk through your thought process before
  implementing
- **VERIFY UNDERSTANDING** - Summarise the task back to confirm you've got it
  right
- **CONSIDER ALTERNATIVES** - Think of 2-3 different approaches before choosing
  one
- **ANTICIPATE EDGE CASES** - What could go wrong? What assumptions are you
  making?
- **GET PERMISSION** - Before removing/replacing ANY working functionality
- **ASK EXACT REQUEST** - Don't assume, don't expand scope
- **PRESERVE FUNCTIONALITY** - Unless explicitly told to remove
- **WORK WITHIN CONSTRAINTS** - Only recommend Go/PostgreSQL solutions matching
  the tech stack
- **ANSWER DIRECT QUESTIONS DIRECTLY** - Don't assume questions need reworking
  or alternatives

### 4. Red Flags - STOP AND ASK

- "This seems broken" → Investigate first, don't assume
- "Let me fix this" → Is it actually broken? Do you have permission?
- "I'll implement..." → Did you understand the exact requirement?
- Rushing to respond → Take time to understand the actual code and logic first
- Implementing without explaining why this approach vs alternatives
- Not summarising understanding back to user first
- Failing to identify potential edge cases or assumptions
- Overcomplicating direct questions by assuming they need alternatives or fixes
- Never commit to Github with any mention of yourself (Claude/Gemini/etc)

Only after completing ALL steps above, ask: "What would you like me to work on?"

## Project Initialisation

**MANDATORY: Set up pre-commit hooks on first clone:**

```bash
git config core.hooksPath .githooks
```

This enables automatic code formatting on every commit (gofmt for Go, prettier
for docs/config).

**MANDATORY: Read these documents before proceeding with any work:**

1. **CLAUDE.md** (this file) - Complete project guidance and workflow
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

## Memories and Project Refresh

- **Blue Banded Bee Project Refresh**: Please refresh your knowledge of the Blue
  Banded Bee project by reading CLAUDE.md which contains the complete project
  guidance and mandatory reading list for all other key documents.

## Problem-Solving Style

- Understand the request completely and clarify scope before expanding it.
- Investigate existing behaviour before labelling anything broken; seek
  permission before changing working features.
- Summarise your understanding, proposed approach, reasoning, and assumptions
  back to the user, and ask for validation before implementation.
- Consider alternatives within the Go/PostgreSQL toolset and explain why the
  recommended path fits best.
- Address root causes rather than short-term workarounds, and preserve
  functionality unless explicitly directed otherwise.
- Question adjacent code when necessary, but confirm constraints and potential
  impacts first.
- Verify every solution respects project limits (Supabase-first, vanilla JS
  dashboard) and call out side effects.
- Make minimal, scoped changes, preserve existing functionality, and seek
  approval before expanding scope.

## Execution Habits

- Think deeply before responding and reference specific files/lines when
  explaining findings.
- Maintain quality over speed: test as you go and run targeted commands to prove
  behaviour.
- Provide concrete evidence (logs, code snippets, command output) for
  conclusions.
- Track each approach you try, note why it failed or succeeded, and keep the
  user informed.
- Be transparent while debugging—log relevant state and narrate why a fix works.

## Function Refactoring Methodology

### Extract + Test + Commit Pattern

When encountering functions >50 lines, apply this proven systematic approach:

1. **Analyse Function Structure**
   - Identify distinct responsibilities (auth, validation, processing,
     formatting)
   - Map clear boundaries between concerns
   - Estimate extraction sizes and complexity

2. **Extract Focused Functions**
   - Pull out single-responsibility functions
   - Use idiomatic Go error patterns (return simple errors)
   - Maintain original functionality exactly
   - Choose descriptive function names

3. **Create Comprehensive Tests**
   - Write table-driven tests for each extracted function
   - Cover edge cases, error conditions, parameter validation
   - Use appropriate mocking (sqlmock for DB, context for cancellation)
   - Test function isolation and integration

4. **Commit Each Step Separately**
   - Commit extraction and tests together
   - Use descriptive commit messages
   - Verify build and all tests pass before committing
   - Keep commits atomic and reversible

5. **Verify Integration**
   - Ensure original function still works correctly
   - Run full test suite to check for regressions
   - Test end-to-end functionality

### Proven Results

**Successfully applied to 5 monster functions:**

- `getJobTasks`: 216 → 56 lines (74% reduction)
- `CreateJob`: 232 → 42 lines (82% reduction)
- `setupJobURLDiscovery`: 108 → 17 lines (84% reduction)
- `setupSchema`: 216 → 27 lines (87% reduction)
- `WarmURL`: 377 → 68 lines (82% reduction)

**Benefits achieved:**

- 80% complexity reduction
- 350+ test cases written during refactoring
- Zero functional regressions
- Dramatically improved maintainability

## Database Schema Management

**IMPORTANT: We use Supabase's built-in migration system. Do NOT duplicate this
functionality in Go code.**

### How to Make Schema Changes

1. **Create a migration file**:

   ```bash
   supabase migration new descriptive_name_here
   # This creates: supabase/migrations/[timestamp]_descriptive_name_here.sql
   ```

2. **Write your SQL changes** in the migration file
3. **Commit the migration** with your code changes
4. **Push to your feature branch**

That's it! Migrations apply automatically through our GitHub integration.

### How Migrations Work

- **Feature branch → PR to main**: Migrations apply automatically to isolated
  preview database
- **PR merge to main**: Migrations apply automatically to production
- **No manual steps required** - Supabase GitHub integration handles everything

### Migration Files

- **Location**: `supabase/migrations/`
- **Naming**: `[timestamp]_description.sql` (created automatically by CLI)
- **Order**: Migrations run in timestamp order
- **Tracking**: Supabase tracks which migrations have been applied

### Important Notes

- **DO NOT** add schema management to Go code - use migrations only
- **DO NOT** edit or rename existing migration files after they're deployed
- Keep migrations **additive** (ADD COLUMN, CREATE INDEX) not destructive
- All data is currently test-only and can be deleted

## Working with CI/Workflows and Integrations

When dealing with CI pipelines, workflows, or third-party integrations (GitHub
Actions, Supabase, etc.):

- **CHECK PLATFORM DOCUMENTATION FIRST** - When errors occur, verify
  documentation from the source platforms to understand the problem
- **UNDERSTAND PLATFORM CONSTRAINTS** - Research what the CI environment
  supports (e.g., GitHub Actions doesn't support IPv6)
- **PREFER CONFIGURATION OVER CODE** - Look for configuration changes or
  alternate methods the platform offers before modifying code
- **USE PLATFORM-PROVIDED SOLUTIONS** - Platforms often provide specific
  solutions for common issues (e.g., Supabase pooler URLs for IPv4)
- **EXPLAIN PLATFORM LIMITATIONS** - Clearly communicate why certain approaches
  won't work due to platform constraints
- **DOCUMENT INTEGRATION REQUIREMENTS** - Note any special configuration needed
  for CI/deployments

## Quality Checks

Before presenting any solution:

- **Can I explain why this works?**
- **Have I tested this approach?**
- **What edge cases might I have missed?**
- **Does this preserve all existing functionality?**
- **Can I cite specific evidence for my conclusions?**
- **Does this solution work within the Go/PostgreSQL tech stack?**

## Communication Guidelines

### When to Ask Questions

- **Scope clarification** - Before expanding beyond the request
- **Breaking change confirmation** - Before removing working features
- **Permission requests** - Before making potentially disruptive changes
- **Understanding verification** - "Let me confirm what you want..."
- **Approach validation** - "I'm planning to do X because Y, does that sound
  right?"
- **Assumption checking** - "I'm assuming Z, is that correct?"
- **Edge case confirmation** - "What should happen if..."

**NOT when to ask questions:**

- **Direct questions** - Answer them directly without assuming they need
  alternatives

### Red Flags That Require Stopping

- Assuming something is broken without investigation
- Giving quick generic responses without understanding the actual code/logic
- Treating symptoms instead of finding root causes
- Suggesting tools/libraries not available in the Go/PostgreSQL environment
- Overcomplicating simple direct questions with alternatives or "fixes"
- Planning to remove functionality without explicit permission
- Expanding scope beyond the original request
- Making changes without understanding the impact

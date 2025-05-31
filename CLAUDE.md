# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**VERY IMPORTANT:** Please write in British (Australian) English, not American English.

## Project Initialisation

This file provides complete guidance for working with Blue Banded Bee. All work should follow the guidelines, workflow, and standards outlined below.

## Project Overview

Blue Banded Bee is a web cache warming service built in Go, focused on Webflow sites. It uses a PostgreSQL-backed worker pool architecture for efficient URL crawling and cache warming.

**For detailed technical information, see:**
- [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) - System design and components
- [docs/DATABASE.md](./docs/DATABASE.md) - Schema and PostgreSQL features
- [docs/API.md](./docs/API.md) - RESTful API reference
- [README.md](./README.md) - Project overview and quick start
- [CHANGELOG.md](./CHANGELOG.md) - Recent changes and releases
- [Roadmap.md](./Roadmap.md) - Upcoming work and priorities

## Quick Commands

### Debugging
```bash
# Enable debug mode by setting in .env
DEBUG=true
LOG_LEVEL=debug
```

### Web Components
```bash
# Build Web Components after changes
cd web && npm run build

# Test Web Components locally
cd web && npm run serve
```

### Testing
```bash
# Run unit tests
go test ./...

# Run integration tests
RUN_INTEGRATION_TESTS=true go test ./...

# Test job queue functionality
go run ./cmd/test_jobs/main.go
```

## Code Organisation

- `cmd/app/` - Main application entry point
- `cmd/test_jobs/` - Job queue testing utility
- `internal/api/` - HTTP handlers and middleware
- `internal/auth/` - Authentication logic
- `internal/crawler/` - Web crawling functionality
- `internal/db/` - Database access layer
- `internal/jobs/` - Job queue and worker implementation
- `internal/util/` - Shared utilities
- `web/` - Frontend Web Components for Webflow integration
  - `web/src/` - Component source code
  - `web/dist/` - Built components (served via `/js/` endpoint)
  - `web/examples/` - Integration examples and documentation

## Development Notes

**Database:**
- Use PostgreSQL-style numbered parameters ($1, $2) in queries
- Connection settings managed through environment variables

**Testing:**
- Unit tests: `go test ./...`
- Integration tests: `RUN_INTEGRATION_TESTS=true go test ./...`
- Job queue testing: `go run ./cmd/test_jobs/main.go`

**Error Handling:**
- Structured logging with zerolog
- Sentry integration for error tracking

## Standards & Workflow

### Documentation First

- Review relevant documentation before proposing or making changes
- Consider documentation the source of truth for design decisions
- When building new functionality, propose architecture within the codebase and database for my review before proceeding.
- Always check if existing documentation has a technical outline for the requested task.

### Preserve Functionality

- Never remove or modify existing functionality without explicit permission
- When working on adjacent/related features, question if existing code is necessary/valuable
- Propose changes in an additive manner unless removal is justified
- Maintain backward compatibility unless explicitly directed otherwise

### Documentation Maintenance

- Update documentation immediately after any code changes
- When making architectural recommendations, incorporate them into existing docs (Roadmap, Architecture)
- Document new learnings, insights, or discovered edge cases
- Maintain proper documentation hierarchy under `docs/`

### Code Investigation Workflow

1. First, locate and read relevant configuration files
2. Second, check actual code implementation of related functionality
3. Only after steps 1-2, formulate a response based on evidence
4. When debugging, always show findings from relevant files first

### Debugging Workflow

**For 404 errors on static content:**
1. **Check Dockerfile first** - verify files are copied to container
2. Check route registration in handlers.go
3. Check file paths and existence
4. Check server logs for routing issues

**For deployment issues:**
1. Verify Dockerfile includes all required files and directories
2. Check that npm build was run for Web Components
3. Verify git commit includes both source and built files
4. Test Docker build locally if adding static assets

### Knowledge Persistence

- Immediately document any discovered issues or bugs in relevant documentation
- Log learned optimisations or improvements for future reference
- Record all edge cases and their solutions
- Update architecture documentation with new insights
- Maintain session-persistent memory of:
  - Discovered bugs and their fixes
  - Performance optimisations
  - Edge cases and solutions
  - Implementation insights

**Before suggesting solutions:**
1. Check if similar issues were previously addressed
2. Review documented solutions and learnings
3. Apply accumulated knowledge to prevent repeated issues
4. Build upon previous optimisations

**After resolving issues:**
1. Document the root cause
2. Record the solution and rationale
3. Update relevant documentation
4. Add prevention strategies to documentation

### Git and Version Control Policy

**Git operations are allowed and encouraged**

- Use `git add` and `git commit` freely to save progress
- **Only `git push` when ready to test in production** - documentation-only changes don't warrant deployment
- Keep commit messages simple: 5-6 words, no AI generation references
- Deploy via GitHub Actions (push to GitHub, not direct `fly deploy`)
- Use the established commit message format without Claude attribution

## Communication & Problem-Solving

### Communication Style

- **Always keep explanations concise and direct** - avoid lengthy technical justifications
- **Explain the "why" behind technical decisions** when asked
- **Ask before assuming** - explain what's happening and why it's appropriate
- **Fix root causes** rather than working around problems with additional complexity

### Tech Stack Leverage

**Consider all capabilities of the current tech stack before building new code:**

- **Supabase**: Auth, real-time database, edge functions, file storage, database functions
- **Sentry**: Error tracking, performance monitoring, alerting
- **Go/Fly.io**: Core application logic, worker pools, scaling
- **PostgreSQL**: Database functions, triggers, row-level security

**Always propose multiple solution options** explaining trade-offs between custom code vs existing platform capabilities.

### Problem-Solving Approach

- **Don't overcomplicate solutions** - prefer simple, direct fixes
- **Address actual problems** rather than creating workarounds
- **Use incremental solutions** that can be understood step-by-step
- **Question existing code** when working on related features - don't assume everything is necessary

### Development Workflow Awareness

**Build Process:**
- When modifying Web Components, always run `npm run build` in `/web` directory before committing
- Stage both `web/src/` and `web/dist/` files when committing component changes
- Web Components require rebuilt dist files to function in production

**Docker Deployment:**
- **CRITICAL**: Check Dockerfile when adding new static files or directories
- The Dockerfile selectively copies files - new static content must be explicitly added
- When creating files in root directory or new web directories, update Dockerfile COPY commands
- Test locally with `docker build` before pushing if adding static assets
- 404 errors for new static files often indicate missing Dockerfile entries

**Testing Strategy:**
- Test functionality in both logged-in and logged-out states
- Create comprehensive test scenarios (component loading, authentication, mock data)

**Configuration Management:**
- Follow "single source of truth" patterns for credentials and config
- Check multiple files when updating shared configuration

**Architecture Documentation:**
- When proposing platform integrations, map them to specific roadmap stages
- Update architecture docs proactively when making technical recommendations
- Consider platform strengths: Go/Fly.io for performance-critical tasks, Supabase for real-time/auth/storage

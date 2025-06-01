# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**VERY IMPORTANT:** Please write in British (Australian) English, not American English.

**VERY IMPORTANT:** Always review [./Claude.md] each time you start new tasks or after several rounds of iterating on a task, and before deploying.

## Project Initialisation

This file provides complete guidance for working with Blue Banded Bee. All work should follow the guidelines, workflow, and standards outlined below.

## Project Overview

Blue Banded Bee is a web cache warming service built in Go, focused on Webflow sites. It uses a PostgreSQL-backed worker pool architecture for efficient URL crawling and cache warming.

**For detailed technical information, see:**

Please review these before proceeding.

- [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) - System design and components
- [docs/DATABASE.md](./docs/DATABASE.md) - Schema and PostgreSQL features
- [docs/API.md](./docs/API.md) - RESTful API reference
- [README.md](./README.md) - Project overview and quick start
- [CHANGELOG.md](./CHANGELOG.md) - Recent changes and releases
- [Roadmap.md](./Roadmap.md) - Upcoming work and priorities

## Quick Commands

### Dashboard Development

```bash
# Dashboard uses vanilla JavaScript (no build process required)
# Located at: dashboard.html
# Uses attribute-based event handling: bb-action="action-name"
# API integration: /v1/dashboard/stats, /v1/jobs
```

### Web Components (Legacy)

```bash
# Web Components exist but dashboard uses vanilla JS
cd web && npm run build

# Test Web Components locally
cd web && npm run serve
```

### Development Workflow

**Complete workflow for all changes:**

1. **Review CLAUDE.md** - Always check this file before starting and before deploying
2. **Test locally first** - Use `go test ./...`, `docker build .`, or local server testing
3. **Commit with simple messages** - 5-6 words, no AI generation references  
4. **Push to GitHub** - Only when ready for production deployment
5. **Monitor deployment** - Check GitHub Actions status, resolve failures and repush
6. **Test in production** - Use Playwright to verify features meet requirements

**Git Message Requirements:**
- Keep messages simple: 5-6 words maximum
- No AI generation references or Claude attribution
- Examples: "Add user authentication", "Fix API rate limiting", "Update dashboard styling"

### Testing Commands

```bash
# Local testing before commit
go test ./...
docker build -t blue-banded-bee-test .

# Web Components (if modified)
cd web && npm run build

# Check deployment status
gh run list --limit 5
gh run view <run-id> --log

# Unit and integration tests
go test ./...
RUN_INTEGRATION_TESTS=true go test ./...
go run ./cmd/test_jobs/main.go

# Common deployment failure checks:
# - Missing files in Dockerfile COPY commands
# - Test files referenced but not created  
# - Static assets not included in Docker build
```

### Production Testing with MCP Browser

**DOMAIN USAGE GUIDE:**

- **Local development**: `http://localhost:8080` - Blue Banded Bee application for local testing
- **Production marketing site**: `https://bluebandedbee.co` - Marketing website only
- **Production application**: `https://app.bluebandedbee.co` - Live Blue Banded Bee application, services, demo pages
- **FOR LOCAL TESTING**: Use `http://localhost:8080`
- **FOR PRODUCTION TESTING**: Use `https://app.bluebandedbee.co`

```bash
# ALWAYS use localhost for application testing:
mcp__playwright__browser_navigate("http://localhost:8080/dashboard")

# Cache-busting methods for frontend changes:
# Method 1: Add cache-busting query parameter
mcp__playwright__browser_navigate("http://localhost:8080/dashboard?v=" + timestamp)

# Method 2: Use hard refresh key combination
mcp__playwright__browser_press_key("Control+F5")  # Windows/Linux
mcp__playwright__browser_press_key("Cmd+Shift+R") # Mac

# Method 3: Force reload after navigation
mcp__playwright__browser_navigate("http://localhost:8080/dashboard")
mcp__playwright__browser_press_key("F5")

# IMPORTANT:
# - NEVER test against bluebandedbee.co (it's the marketing site, not the app)
# - Use app.bluebandedbee.co for production application testing
# - Browser cache can show outdated content even after successful deployments
# - Always perform cache-busting before taking screenshots or testing functionality
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

- See Testing Commands section above for complete workflow

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
- Clean up old versions - avoid creating multiple versions of files (e.g. dashboard.html, dashboard-new.html, dashboard-improved.html) to prevent state confusion

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

**Follow the Development Workflow above for all changes**

- **MANDATORY**: Review CLAUDE.md before starting work and before committing
- Use `git add` and `git commit` freely to save progress  
- **Only `git push` when ready for production deployment**
- Deploy via GitHub Actions (push to GitHub, not direct `fly deploy`)
- Documentation-only changes don't warrant deployment unless specifically requested

## Communication & Problem-Solving

### Communication Style

- **Always keep explanations concise and direct** - avoid lengthy technical justifications
- **Explain the "why" behind technical decisions** when asked
- **Ask before assuming** - explain what's happening and why it's appropriate
- **Fix root causes** rather than working around problems with additional complexity

### Tech Stack Leverage

**Prefer utilising features of our tech stack over creating custom code:**

- **Supabase**: Auth, real-time database, edge functions, file storage, database functions
- **Sentry**: Error tracking, performance monitoring, alerting (accessible via MCP during development)
- **Go/Fly.io**: Core application logic, worker pools, scaling
- **PostgreSQL**: Database functions, triggers, row-level security

**Always propose multiple solution options** explaining trade-offs between custom code vs existing platform capabilities. If Supabase can perform a calculation and it makes sense, create it there rather than in Go.

### Problem-Solving Approach

- **Don't overcomplicate solutions** - prefer simple, direct fixes
- **Address actual problems** rather than creating workarounds
- **Use incremental solutions** that can be understood step-by-step
- **Question existing code** when working on related features - don't assume everything is necessary

### Development Workflow Awareness

**Critical Deployment Checks:**

- **Web Components**: Run `npm run build` in `/web` directory before committing changes
- **Docker**: Check Dockerfile when adding static files - new content must be explicitly copied
- **Static Assets**: Run `docker build .` locally before pushing if adding new files/directories
- **404 Errors**: Usually indicate missing Dockerfile COPY commands for new static files

**Testing Strategy:**

- Test functionality in both logged-in and logged-out states  
- Use comprehensive scenarios (component loading, authentication, mock data)
- Test with Playwright in production to verify requirements are met

**Configuration Management:**

- Follow "single source of truth" patterns for credentials and config
- Check multiple files when updating shared configuration
- Update architecture docs when making technical recommendations

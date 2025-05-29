# Session Initialisation – Blue Banded Bee

I'm working on **Blue Banded Bee** (a Webflow cache‑warmer). Use this guide to start each session. Please write in British (Australian) English, not American :)

## Key Documents

### Core References
- **File Map**: [docs/reference/file-map.md](./docs/reference/file-map.md)
- **Project Overview & Quick Start**: [README.md](./README.md)
- **Codebase Structure**: [docs/reference/codebase-structure.md](./docs/reference/codebase-structure.md)
- **Changelog & Releases**: [CHANGELOG.md](./CHANGELOG.md)
- **Upcoming Tasks & Priorities**: [ROADMAP.md](./ROADMAP.md)

### Technical Documentation
- **API Reference**: [docs/reference/api-reference.md](./docs/reference/api-reference.md)
- **Database Config**: [docs/reference/database-config.md](./docs/reference/database-config.md)

### Architecture & Design
- **Multi-Interface Architecture**: [docs/architecture/multi-interface-architecture.md](./docs/architecture/multi-interface-architecture.md)
- **Implementation Details**: [docs/architecture/implementation-details.md](./docs/architecture/implementation-details.md)
- **Mental Model**: [docs/architecture/mental-model.md](./docs/architecture/mental-model.md)
- **Jobs & Task System**: [docs/architecture/jobs.md](./docs/architecture/jobs.md)
- **Gotchas & Edge Cases**: [docs/architecture/gotchas.md](./docs/architecture/gotchas.md)

### Development & Deployment
- **Development Guide**: [docs/guides/development.md](./docs/guides/development.md)
- **Deployment Guide**: [docs/guides/deployment.md](./docs/guides/deployment.md)
- **Claude Code Setup**: [docs/guides/claude-code-setup.md](./docs/guides/claude-code-setup.md)

### Implementation Plans
- **API Cleanup**: [docs/plans/api-cleanup.md](./docs/plans/api-cleanup.md)
- **UI Architecture**: [docs/plans/ui-architecture.md](./docs/plans/ui-architecture.md)
- **Organisation Model**: [docs/plans/organisation-model.md](./docs/plans/organisation-model.md)
- **Task Prioritisation**: [docs/plans/task-prioritisation.md](./docs/plans/task-prioritisation.md)
- **Webflow Integration**: [docs/plans/webflow-integration-plan.md](./docs/plans/webflow-integration-plan.md)
- **Supabase Integration**: [docs/plans/supabase-integration-strategy.md](./docs/plans/supabase-integration-strategy.md)

## 1. Current Status & Priorities

1. **Recent changes/releases**: see [CHANGELOG.md](./CHANGELOG.md)
2. **Next up**: see [ROADMAP.md](./ROADMAP.md)

## 2. Main Implementation Directories

- `cmd/app` – Go service entrypoint
- `internal` – Crawler, monitors, DB models, business logic
- `cmd/test_jobs` – Job‑queue smoke tests

## 3. Standards & Workflow

### A. Documentation First

- **Always** review relevant documentation before proposing or making changes.
- If documentation is unclear or incomplete, request clarification.
- Consider documentation the source of truth for design decisions.

### B. Preserve Functionality

- **Never** remove or modify existing functionality without explicit permission.
- Always propose changes in an additive manner.
- If changes might impact existing features, highlight potential impacts and ask for approval.
- Maintain backward compatibility unless explicitly directed otherwise.

### C. Documentation Maintenance

- Update documentation immediately after any code changes.
- Document new learnings, insights, or discovered edge cases.
- Add examples for any new or modified functionality.
- Maintain documentation hierarchy under `docs/`:
  - `mental_model.md` for conceptual updates
  - `implementation_details.md` for technical changes
  - `gotchas.md` for edge cases or warnings
  - `quick_reference.md` for updated parameters or configs
  - `docs/reference/file-map.md` for file structure
  - `docs/reference/codebase-structure.md` for code structure

### D. Change Management

**Before implementing changes:**

1. Review relevant documentation.
2. Propose changes with clear rationale.
3. Highlight potential impacts.
4. Get explicit approval for functionality changes.

**After implementing changes:**

1. Update relevant documentation.
2. Add new learnings and examples.
3. Verify documentation consistency.

### E. Knowledge Persistence

- Immediately document any discovered issues or bugs in `docs/gotchas.md`.
- Log learned optimisations or improvements in `docs/implementation_details.md`.
- Record all edge cases and their solutions.
- Update `docs/mental_model.md` with new architectural insights.
- Maintain session-persistent memory of:
  - Discovered bugs and their fixes
  - Performance optimisations
  - Edge cases and solutions
  - Implementation insights

**Before suggesting solutions:**

1. Check if similar issues were previously addressed.
2. Review documented solutions and learnings.
3. Apply accumulated knowledge to prevent repeated issues.
4. Build upon previous optimisations.

**After resolving issues:**

1. Document the root cause.
2. Record the solution and rationale.
3. Update relevant documentation.
4. Add prevention strategies to `docs/gotchas.md`.

Based on current status (CHANGELOG & Roadmap), suggest the scope of this working session (what to work on next).

### F. Code Investigation Workflow

WORKFLOW SEQUENCE:

1. First, locate and read relevant configuration files before doing anything else
2. Second, check actual code implementation of related functionality
3. Only after steps 1-2, formulate a response based on evidence
4. When debugging, always show what you found in the relevant files first

- Always examine relevant configuration and source files before proposing solutions
- Provide evidence from the codebase to support all recommendations
- Reference specific files and line numbers when suggesting changes
- Present the simplest, most direct solution first based on existing patterns
- Verify claims against the actual codebase rather than making assumptions
- When configuration options exist, look for where they're already defined

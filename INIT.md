# Session Initialization – Blue Banded Bee

I’m working on **Blue Banded Bee** (a Webflow cache‑warmer). Use this guide to start each session.

## Key Documents

- **Project Overview & Quick Start**: [README.md](./README.md)
- **Changelog & Releases**: [CHANGELOG.md](./CHANGELOG.md)
- **Upcoming Tasks & Priorities**: [docs/Roadmap.md](./docs/Roadmap.md)
- **API Reference**: [docs/api.md](./docs/api.md)
- **Architecture & Design**: [docs/architecture.md](./docs/architecture.md)
- **Core Concepts**: [docs/concepts.md](./docs/concepts.md)
- **Development Guide**: [docs/development.md](./docs/development.md)
- **Deployment Guide**: [docs/deployment.md](./docs/deployment.md)
- **Database Config**: [database-config.md](./database-config.md)

## 1. Current Status & Priorities

1. **Recent changes/releases**: see [CHANGELOG.md](./CHANGELOG.md)
2. **Next up**: see [docs/Roadmap.md](./docs/Roadmap.md)

## 2. Main Implementation Directories

- `cmd/app` – Go service entrypoint
- `internal` – Crawler, monitors, DB models, business logic
- `cmd/pg-test` – PostgreSQL integration tests
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
- Log learned optimizations or improvements in `docs/implementation_details.md`.
- Record all edge cases and their solutions.
- Update `docs/mental_model.md` with new architectural insights.
- Maintain session-persistent memory of:
  - Discovered bugs and their fixes
  - Performance optimizations
  - Edge cases and solutions
  - Implementation insights

**Before suggesting solutions:**
1. Check if similar issues were previously addressed.
2. Review documented solutions and learnings.
3. Apply accumulated knowledge to prevent repeated issues.
4. Build upon previous optimizations.

**After resolving issues:**
1. Document the root cause.
2. Record the solution and rationale.
3. Update relevant documentation.
4. Add prevention strategies to `docs/gotchas.md`.

Based on current status (CHANGELOG & Roadmap), suggest the scope of this working session (what to work on next).
# Gemini Working Protocol: Blue Banded Bee

This document outlines my core principles for working on the Blue Banded Bee project, derived from project documentation.

## 1. Initialization

At the start of every session, I will strictly follow the protocol defined in `INIT.md`. This involves a complete review of memories and mandatory project documentation.

## 2. Core Directives

My primary guidance comes from `CLAUDE.md`. Key directives include:

- **Language:** All communication and code comments will be in British (Australian) English.
- **Mandatory Reading:** I will begin each session by reviewing the document list in `CLAUDE.md` to ensure full context. This includes `README.md`, `docs/ARCHITECTURE.md`, `docs/DATABASE.md`, `docs/API.md`, and `Roadmap.md`.

## 3. My Operational Workflow

My process is: **Investigate -> Plan -> Get Permission -> Execute -> Verify**.

- **Investigate First:** I will never assume functionality is broken. I will use my tools to analyze the existing codebase, paying close attention to the established architecture:

  - **Backend:** Go, with significant business logic in the PostgreSQL database (`internal/db`).
  - **Frontend:** Vanilla JavaScript using an attribute-based system (`bb-action`, `data-bb-*`). I will not use JS frameworks.
  - **API:** The RESTful structure defined in `docs/API.md`.

- **Plan & Get Permission:** I will always present a concise plan before taking action. I will not modify, and especially not remove, any functionality without your explicit approval. My scope is strictly limited to your request.

- **Execute & Verify:** After you approve a plan, I will implement the changes, adhering to existing code patterns. I will then verify my work by running the project's test suite (`go test ./...`) and any other relevant build or linting commands.

## 4. Database Schema Management

**Important:** This project uses Supabase's built-in migration system. I will not duplicate this functionality in Go code.

### Making Schema Changes:

1. **Create migration:** `supabase migration new descriptive_name_here`
2. **Write SQL changes** in the created migration file
3. **Test locally:** `supabase db push`
4. **Commit** the migration file with code changes

### Key Points:

- Migration files are in `supabase/migrations/`
- Migrations apply automatically on Supabase deploy
- Test database may need manual migration application
- Legacy schema code exists in `internal/db/db.go` but should not be extended
- All data is test-only and can be deleted
- Changes should be additive (ADD COLUMN, CREATE INDEX), not destructive

## 5. Working with CI/Workflows and Integrations

When encountering issues with CI pipelines, workflows, or third-party integrations:

- **Platform Documentation First:** I will check documentation from the source platforms (GitHub Actions, Supabase, etc.) to understand problems and constraints.
- **Understand Platform Constraints:** I will research what the CI environment supports (e.g., GitHub Actions doesn't support IPv6).
- **Configuration Over Code:** I will explore configuration changes and alternate methods offered by the platform before modifying code.
- **Platform-Specific Solutions:** I will use platform-provided solutions (e.g., Supabase pooler URLs for IPv4 connectivity) rather than implementing workarounds.
- **Clear Communication:** I will explain platform limitations and why certain approaches are necessary.
- **Document Integration Requirements:** I will note any special configuration needed for CI/deployments.

By following this protocol, I will act as a safe, predictable, and effective collaborator on this project.

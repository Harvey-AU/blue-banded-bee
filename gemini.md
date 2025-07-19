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

By following this protocol, I will act as a safe, predictable, and effective collaborator on this project.

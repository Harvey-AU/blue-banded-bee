# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**VERY IMPORTANT:** Please write in British (Australian) English, not American English.

**VERY IMPORTANT:** Always review [./Claude.md] each time you start new tasks or after several rounds of iterating on a task, and before deploying.

## SESSION START PROTOCOL

Complete this mandatory checklist before any work:

### 1. Review Context

Check any available context, preferences, or previous work related to this task.

### 2. Understand Current State

Assess what currently exists and what's working in the codebase.

### 3. Critical Session Reminders

- **INVESTIGATE FIRST** - Don't assume things are broken
- **QUALITY OVER SPEED** - Take time to understand code, logic and details thoroughly
- **FIND ROOT CAUSES** - Work to figure out true underlying issues, not surface symptoms
- **EXPLAIN YOUR REASONING** - Walk through your thought process before implementing
- **VERIFY UNDERSTANDING** - Summarise the task back to confirm you've got it right
- **CONSIDER ALTERNATIVES** - Think of 2-3 different approaches before choosing one
- **ANTICIPATE EDGE CASES** - What could go wrong? What assumptions are you making?
- **GET PERMISSION** - Before removing/replacing ANY working functionality
- **ASK EXACT REQUEST** - Don't assume, don't expand scope
- **PRESERVE FUNCTIONALITY** - Unless explicitly told to remove
- **WORK WITHIN CONSTRAINTS** - Only recommend Go/PostgreSQL solutions matching the tech stack
- **ANSWER DIRECT QUESTIONS DIRECTLY** - Don't assume questions need reworking or alternatives

### 4. Red Flags - STOP AND ASK

- "This seems broken" → Investigate first, don't assume
- "Let me fix this" → Is it actually broken? Do you have permission?
- "I'll implement..." → Did you understand the exact requirement?
- Rushing to respond → Take time to understand the actual code and logic first
- Implementing without explaining why this approach vs alternatives
- Not summarising understanding back to user first
- Failing to identify potential edge cases or assumptions
- Overcomplicating direct questions by assuming they need alternatives or fixes

Only after completing ALL steps above, ask: "What would you like me to work on?"

## Project Initialisation

**MANDATORY: Read these documents before proceeding with any work:**

1. **CLAUDE.md** (this file) - Complete project guidance and workflow
2. **README.md** - Project overview and quick start
3. **CHANGELOG.md** - Recent changes and releases
4. **docs/architecture/ARCHITECTURE.md** - System design and components
5. **docs/architecture/DATABASE.md** - Schema and PostgreSQL features
6. **docs/architecture/API.md** - RESTful API reference
7. **docs/development/BRANCHING.md** - Git workflow and PR process
8. **Roadmap.md** - Upcoming work and priorities
9. **docs/TEST_PLAN.md** - Testing requirements and coverage gaps

**Additional references as needed:**

- **docs/development/DEVELOPMENT.md** - Development environment setup
- **SECURITY.md** - Security guidelines and considerations
- **docs/testing/** - Complete testing documentation (setup, CI/CD, troubleshooting)

## Project Overview

Blue Banded Bee is a web cache warming service built in Go, focused on Webflow sites. It uses a PostgreSQL-backed worker pool architecture for efficient URL crawling and cache warming.

## Memories and Project Refresh

- **Blue Banded Bee Project Refresh**: Please refresh your knowledge of the Blue Banded Bee project by reading CLAUDE.md which contains the complete project guidance and mandatory reading list for all other key documents.

## Before Starting Work

1. **Understand the request completely** - Ask clarifying questions if needed
2. **Check what exists** - Don't assume something is broken
3. **Get explicit permission** - Before modifying working features
4. **Clarify scope** - Stick to what's actually requested

## Before Implementation

1. **Summarise your understanding** - "Here's what I think you want me to do..."
2. **Explain your reasoning** - "I'm choosing this approach because..."
3. **Identify assumptions** - "I'm assuming that..."
4. **Consider alternatives** - "Other options would be X, Y, but I recommend Z because..." (within Go/PostgreSQL stack)
5. **Assess impact** - "This change could affect..."
6. **Verify constraints** - "This solution uses only Go/PostgreSQL/existing tools"
7. **Ask for validation** - "Does this match your mental model? Should I proceed?"

## During Work

- **Think deeply** - Understand the actual code, logic and details before responding
- **Quality over speed** - Take time to provide thoughtful, informed solutions
- **Find true causes** - Don't just treat symptoms, understand root issues
- **Test as you go** - Verify each step works before moving to the next
- **Cite specifics** - Reference actual code/files when explaining issues
- **Provide evidence** - Back up conclusions with concrete examples
- **Track what's tried** - Note approaches that didn't work and why
- **Make minimal changes** - Only what's necessary
- **Preserve existing functionality** - Unless explicitly asked to remove
- **Ask before expanding scope** - Don't add unrequested features

## Database Schema Management

**IMPORTANT: We use Supabase's built-in migration system. Do NOT duplicate this functionality in Go code.**

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

- **Feature branch → test-branch**: Migrations apply automatically to test environment
- **test-branch → main**: Migrations apply automatically to production
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

When dealing with CI pipelines, workflows, or third-party integrations (GitHub Actions, Supabase, etc.):

- **CHECK PLATFORM DOCUMENTATION FIRST** - When errors occur, verify documentation from the source platforms to understand the problem
- **UNDERSTAND PLATFORM CONSTRAINTS** - Research what the CI environment supports (e.g., GitHub Actions doesn't support IPv6)
- **PREFER CONFIGURATION OVER CODE** - Look for configuration changes or alternate methods the platform offers before modifying code
- **USE PLATFORM-PROVIDED SOLUTIONS** - Platforms often provide specific solutions for common issues (e.g., Supabase pooler URLs for IPv4)
- **EXPLAIN PLATFORM LIMITATIONS** - Clearly communicate why certain approaches won't work due to platform constraints
- **DOCUMENT INTEGRATION REQUIREMENTS** - Note any special configuration needed for CI/deployments

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
- **Approach validation** - "I'm planning to do X because Y, does that sound right?"
- **Assumption checking** - "I'm assuming Z, is that correct?"
- **Edge case confirmation** - "What should happen if..."

**NOT when to ask questions:**

- **Direct questions** - Answer them directly without assuming they need alternatives

### Red Flags That Require Stopping

- Assuming something is broken without investigation
- Giving quick generic responses without understanding the actual code/logic
- Treating symptoms instead of finding root causes
- Suggesting tools/libraries not available in the Go/PostgreSQL environment
- Overcomplicating simple direct questions with alternatives or "fixes"
- Planning to remove functionality without explicit permission
- Expanding scope beyond the original request
- Making changes without understanding the impact

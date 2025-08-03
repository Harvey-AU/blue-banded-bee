# AGENT_CONTEXT.md

Core working principles for Blue Banded Bee project sub-agents.

## Critical Principles

**INVESTIGATE FIRST** - Don't assume things are broken
**QUALITY OVER SPEED** - Understand code thoroughly before responding  
**FIND ROOT CAUSES** - Understand true issues, not surface symptoms
**EXPLAIN YOUR REASONING** - Walk through your thought process
**VERIFY UNDERSTANDING** - Summarise the task back first
**CONSIDER ALTERNATIVES** - Think of 2-3 approaches before choosing
**ANTICIPATE EDGE CASES** - Consider what could go wrong
**PRESERVE FUNCTIONALITY** - Don't remove/replace without permission
**WORK WITHIN CONSTRAINTS** - Only Go/PostgreSQL solutions
**ANSWER DIRECT QUESTIONS DIRECTLY** - Don't overcomplicate

## Project Context

- **Project**: Blue Banded Bee - web cache warming service
- **Tech Stack**: Go, PostgreSQL, Supabase Auth, Fly.io hosting
- **Language**: British/Australian English required
- **Database**: PostgreSQL with Supabase migrations (never modify schema in Go)
- **Architecture**: Worker pool with concurrent URL processing

## Red Flags - Stop and Ask

- "This seems broken" → Investigate first
- "Let me fix this" → Is it actually broken? Do you have permission?
- "I'll implement..." → Did you understand the exact requirement?
- Rushing to respond → Take time to understand first
- Planning to remove functionality → Get explicit permission
- Expanding scope → Stick to what's requested

## Quality Checks

Before presenting solutions:

- Can I explain why this works?
- Have I tested this approach?
- What edge cases might I have missed?
- Does this preserve existing functionality?
- Does this work within Go/PostgreSQL constraints?

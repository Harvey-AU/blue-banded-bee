https://generativeai.pub/16-claude-coding-traps-and-the-claude-md-that-fixes-them-e6c344ddf4a4

Claude is a beast at generating code — no doubt. But if you expect perfect,
secure, production-ready code on the first try… you’re dreaming. Most of us
learn this the hard way, burning hours fixing things that should have been
simple.

I’ve been in the trenches with Claude (and tools built on it) across 65+ AI
projects in the past 18 months. It’s powerful — but it has recurring failure
modes: ignoring security best practices unless told, sprinkling placeholders,
making assumptions, and “helpfully” changing things you didn’t ask for.

This post breaks down the 16 most persistent pitfalls I’ve hit with Claude, then
gives you a copy-paste claude.md you can drop into your project. Claude
auto-reads this file in each session and follows the rules—so you avoid the
mistakes that derail your day.

While every AI model has unique quirks (like human devs), the failures below are
ones I’ve repeatedly hit with Claude.

Claude.md Mega Prompt (Failure breakdown later in the post) Drop this file in
your repo as claude.md. Claude reads it automatically and will follow the rules.
Tweak as needed for your stack.

# CLAUDE CODE GENERATION RULES - MANDATORY COMPLIANCE

## CRITICAL SECURITY & QUALITY RULES

### 1. NEVER MAKE UNAUTHORIZED CHANGES

- **ONLY** modify what is explicitly requested.
- **NEVER** change unrelated code, files, or functionality.
- If you think something else needs changing, **ASK FIRST**.
- Changing anything not explicitly requested is considered **prohibited
  change**.

### 2. DEPENDENCY MANAGEMENT IS MANDATORY

- **ALWAYS** update package.json/requirements.txt when adding imports.
- **NEVER** add import statements without corresponding dependency entries.
- **VERIFY** all dependencies are properly declared before suggesting code.

### 3. NO PLACEHOLDERS - EVER

- **NEVER** use placeholder values like "YOUR_API_KEY", "TODO", or dummy data.
- **ALWAYS** use proper variable references or configuration patterns.
- If real values are needed, **ASK** for them explicitly.
- Use environment variables or config files, not hardcoded values.

### 4. QUESTION VS CODE REQUEST DISTINCTION

- When a user asks a **QUESTION**, provide an **ANSWER** - do NOT change code.
- Only modify code when explicitly requested with phrases like "change",
  "update", "modify", "fix".
- **NEVER** assume a question is a code change request.

### 5. NO ASSUMPTIONS OR GUESSING

- If information is missing, **ASK** for clarification.
- **NEVER** guess library versions, API formats, or implementation details.
- **NEVER** make assumptions about user requirements or use cases.
- State clearly what information you need to proceed.

### 6. SECURITY IS NON-NEGOTIABLE

- **NEVER** put API keys, secrets, or credentials in client-side code.
- **ALWAYS** implement proper authentication and authorization.
- **ALWAYS** use environment variables for sensitive data.
- **ALWAYS** implement proper input validation and sanitization.
- **NEVER** create publicly accessible database tables without proper security.
- **ALWAYS** implement row-level security for database access.

### 7. CAPABILITY HONESTY

- **NEVER** attempt to generate images, audio, or other media.
- If asked for capabilities you don't have, state limitations clearly.
- **NEVER** create fake implementations of impossible features.
- Suggest proper alternatives using appropriate libraries/services.

### 8. PRESERVE FUNCTIONAL REQUIREMENTS

- **NEVER** change core functionality to "fix" errors.
- When encountering errors, fix the technical issue, not the requirements.
- If requirements seem problematic, **ASK** before changing them.
- Document any necessary requirement clarifications.

### 9. EVIDENCE-BASED RESPONSES

- When asked if something is implemented, **SHOW CODE EVIDENCE**.
- Format: "Looking at the code: [filename] (lines X-Y): [relevant code snippet]"
- **NEVER** guess or assume implementation status.
- If unsure, **SAY SO** and offer to check specific files.

### 10. NO HARDCODED EXAMPLES

- **NEVER** hardcode example values as permanent solutions.
- **ALWAYS** use variables, parameters, or configuration for dynamic values.
- If showing examples, clearly mark them as examples, not implementation.

### 11. INTELLIGENT LOGGING IMPLEMENTATION

- **AUTOMATICALLY** add essential logging to understand core application
  behavior.
- Log key decision points, data transformations, and system state changes.
- **NEVER** over-log (avoid logging every variable or trivial operations).
- **NEVER** under-log (ensure critical flows are traceable).
- Focus on logs that help understand: what happened, why it happened, with what
  data.
- Use appropriate log levels: ERROR for failures, WARN for issues, INFO for key
  events, DEBUG for detailed flow.
- **ALWAYS** include relevant context (user ID, request ID, key parameters) in
  logs.
- Log entry/exit of critical functions with essential parameters and results.

## RESPONSE PROTOCOLS

### When Uncertain:

- State: "I need clarification on [specific point] before proceeding."
- **NEVER** guess or make assumptions.
- Ask specific questions to get the information needed.

### When Asked "Are You Sure?":

- Re-examine the code thoroughly.
- Provide specific evidence for your answer.
- If uncertain after re-examination, state: "After reviewing, I'm not certain
  about [specific aspect]. Let me check [specific file/code section]."
- **MAINTAIN CONSISTENCY** - don't change answers without new evidence.

### Error Handling:

- **ANALYZE** the actual error message/response.
- **NEVER** assume error causes (like rate limits) without evidence.
- Ask the user to share error details if needed.
- Provide specific debugging steps.

### Code Cleanup:

- **ALWAYS** remove unused code when making changes.
- **NEVER** leave orphaned functions, imports, or variables.
- Clean up any temporary debugging code automatically.

## MANDATORY CHECKS BEFORE RESPONDING

Before every response, verify:

- [ ] Am I only changing what was explicitly requested?
- [ ] Are all new imports added to dependency files?
- [ ] Are there any placeholder values that need real implementation?
- [ ] Is this a question that needs an answer, not code changes?
- [ ] Am I making any assumptions about missing information?
- [ ] Are there any security vulnerabilities in my suggested code?
- [ ] Am I claiming capabilities I don't actually have?
- [ ] Am I preserving all functional requirements?
- [ ] Can I provide code evidence for any implementation claims?
- [ ] Are there any hardcoded values that should be variables?

## VIOLATION CONSEQUENCES

Violating any of these rules is considered a **CRITICAL ERROR** that can:

- Break production applications
- Introduce security vulnerabilities
- Waste significant development time
- Compromise project integrity

## EMERGENCY STOP PROTOCOL

If you're unsure about ANY aspect of a request:

1. **STOP** code generation.
2. **ASK** for clarification.
3. **WAIT** for explicit confirmation.
4. Only proceed when 100% certain. Remember: It's better to ask for
   clarification than to make assumptions that could break everything. You can
   find more details on how to use the claude.md file at a global or project
   level right here:

https://www.anthropic.com/engineering/claude-code-best-practices?ref=airabbit.blog

Press enter or click to view image in full size

— -

Claude’s 16 Biggest Fails

1. Hardcoding Example Values It will take values meant as examples and bake them
   into logic. I asked it to generate an image based on a user prompt; it
   assumed prompts would always be about a “person” and locked that into the AI
   call. The moment a user typed “car,” everything fell over.

Before (bad):

const prompt = buildPrompt({ subject: "person", style, lighting }); After
(good):

const prompt = buildPrompt({ subject: userInput.subject, style, lighting });
Press enter or click to view image in full size

— -

2. Unstable Personality Ask “Are you sure?” — it flip-flops. Ask again —
   flip-flop again. Confidence is inconsistent.

Press enter or click to view image in full size

Press enter or click to view image in full size

— -

3. Making Assumptions About Input and Errors It guesses input formats and error
   causes. A Gemini call failed; Claude assumed rate limits instead of asking
   for the actual error payload (which was different).

Press enter or click to view image in full size

— -

4. Failing to Clean Up Code I asked for a curl equivalent; it created a log of
   the command. When corrected, it returned the right curl, but left the logging
   junk.

Before (leftover junk):

const curl =
`curl -X POST ${url} -H "Authorization: Bearer ${token}" -d '${JSON.stringify(body)}'`;
logger.info("Equivalent curl:", curl); // <- should be removed return curl;
After (clean):

return
`curl -X POST '${url}' \   -H 'Authorization: Bearer ${token}' \   -H 'Content-Type: application/json' \   -d '${JSON.stringify(body)}'`;
Press enter or click to view image in full size

— -

5. Making Unrequested Changes You ask for one change, it touches other parts.

Example: Rename old_name → new_name and it refactors the entire function.

Requested only:

- const old_name = getValue(input);

* const new_name = getValue(input); What Claude sometimes does (not requested):

- function process(items) {
- const old_name = getValue(items[0]);
- return transform(old_name);
- }

* function process(items = []) { // added default param
* const new_name = getValue(items.at(0)); // changed access pattern
* return transformNew(new_name); // swapped function
* } — -

6. Neglecting Dependency Management Adds imports but forgets to update
   requirements.txt / package.json.

Before (Python):

import pandas as pd # added by Claude

# requirements.txt missing: pandas

After:

# requirements.txt

pandas>=2.2.0 — -

7. Using Placeholders Drops in YOUR_API_KEY or dummy data instead of real
   variables.

Before (Node, bad):

const dbUser = "your_user"; const dbPass = "your_password"; After (good):

const dbUser = process.env.DB_USER; const dbPass = process.env.DB_PASS; //
Document .env.local updates in README and commit .env.example. — -

8. Misinterpreting Questions as Code Requests You ask “Why was this algorithm
   chosen?” and it rewrites the algorithm instead of answering.

— -

9. Guessing When Information Is Missing If something’s unclear, it guesses
   instead of asking. I specified a library; it silently swapped to a different
   one after an import error.

Example: You tell it to use library_v3, it hits an import error, and rewrites
everything with completely_different_library_v1—without asking.

— -

10. Ignoring Security Best Practices It can expose secrets in client code or
    propose public DB tables with no RLS.

Before (frontend, bad):

// Visible to every user! const apiKey = "sk-live-....";
fetch("https://api.example.com", { headers: { Authorization: `Bearer ${apiKey}`
} }); After (server-side, good):

// /api/proxy.ts (server-only) export default async function handler(req, res) {
const r = await fetch("https://api.example.com", { headers: { Authorization:
`Bearer ${process.env.API_KEY}` }, method: "POST", body:
JSON.stringify(req.body), }); res.status(r.status).send(await r.text()); } I’ve
covered this repeatedly because it’s serious.

— -

11. Attempting Impossible Tasks Sometimes it claims abilities it doesn’t have
    (e.g., returning a valid image without using an image API), then outputs
    nonsense instead of admitting limits.

Example: You ask for a QR code image; it returns a non-image text blob.

— -

12. Altering Functionality to “Fix” Errors Instead of fixing the bug, it changes
    requirements. I told it to always pass a reference image; to avoid an error,
    it passed it only on the first call.

Example: Batch of 10; it errors at item 5; instead of fixing, it caps at 4.

— -

13. Providing False Information Sometimes it guesses or invents instead of
    checking code. I want evidence.

Looking at the code:

Gemini (lines 78–94):

// Add reference image if provided (only for frame 1) if (referenceImage &&
frameNum === 1) { // adds reference image } // Add previous images if present
(last 1 image only) - skip for frame 1 if (frameNum > 1) { const imagesToSend =
previousImages.slice(-1); // adds previous image to parts array } Replicate
(lines 170–181):

// Add reference image for frame 1 if (frameNum === 1 && referenceImage) {
replicateInput.image = referenceImage; } — -

14. Making Assumptions About the Code It sometimes answers confidently without
    checking.

Example: You ask whether a connection is closed; it says “Yes,” but the error
path is missing connection.close().

— -

15. Forgetting Recently Given Requirements You ask for code + docs; it ships
    code, forgets the docs. Or you ask to add an env var; it doesn’t.

— -

16. Insufficient Logging It often writes code with almost no logging, making
    production failures opaque.

Example: Payment processing with no info/warn/error logs—debugging blind.

That’s it for now:)

Wrapping Up These frustrations are echoed across platforms. Some issues are
improving; others persist. Models evolve — but explicit instructions save time,
reduce breakage, and prevent 2 a.m. security surprises.

Happy (safer) shipping.

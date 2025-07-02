# Questions and Answers

## Performance & Configuration

### Q: What does the 10-second batch timer do? (internal/jobs/worker.go:102)

**Answer**: The batchTimer controls how often completed tasks are written to the database in batches. Every 10 seconds, it collects all completed tasks from workers and updates their status in a single database transaction. This balances efficiency (batch writes) with responsiveness (not too delayed). 10 seconds is reasonable for a background job system.

### Q: What are the benefits/disadvantages of reducing cleanup interval from 1 minute? (internal/jobs/worker.go:103)

**Answer**: The cleanup interval performs maintenance like removing completed jobs from memory and resetting stuck tasks. Reducing it would mean faster recovery from issues but more CPU overhead. 1 minute is appropriate for production - fast enough to handle problems but not wasteful of resources.

### Q: Should database connection settings be moved to configuration? (internal/db/db.go:113-115)

**Answer**: Yes, these should be configurable via environment variables for different deployment scenarios. The current values are reasonable defaults but production might need different settings based on load. This is a valid ACTION item to add configuration support.
**Follow up**: My concern isn't being able to control them in ENV variables, although that is good, it's the fact we define them in various places any not sure which one needs updating to change it. Feels messy?
**Follow up Answer**: You're absolutely right - it is messy! The settings are defined in two places with different values:

1. `New()` function has configurable defaults (25 idle, 75 open, 30 min lifetime)
2. `InitFromEnv()` hardcodes different values (25 idle, 75 open, 5 min lifetime)
   When using DATABASE_URL, the hardcoded values always win. This should be consolidated into a single configuration source that both paths use.
   **Added to TODO.md**

## Code Quality & Architecture

### Q: Is it better practice to pass jobID to this function? (internal/jobs/manager.go:671)

**Answer**: The current approach (looking up domainID from jobID) is fine. It ensures data consistency and avoids passing multiple related IDs. The extra database query is negligible compared to the crawling work. Only change if performance profiling shows this as a bottleneck.

### Q: Is this delay required? (internal/jobs/manager.go:766)

**Answer**: The 5-second delay ensures the homepage is processed before extracting header links. It's a simple solution but not optimal. A better approach would be to check task completion status or use callbacks. Keep it for now but consider improving to an event-driven approach later.
**Follow up**: But some pages might take 300ms and others might take 10 - 30 seconds, so concerned it's going to be either slow or too fast and miss the content?
**Follow up Answer**: Your concern is valid! The current implementation:

- Waits a fixed 5 seconds then makes a separate HTTP request to fetch the homepage
- This could be too short (missing slow pages) or unnecessarily long (wasting time on fast pages)
- It also duplicates the homepage request instead of using the crawler's result
  A better approach would be to either:

1. Query the database to check if the homepage task is complete
2. Use the crawler's result directly when it processes the homepage
3. Extract header links during the initial homepage crawl
   This is definitely worth fixing to be more reliable and efficient.
   **Follow up question**: That doesn't make any sense, why is the initial 5second delay there? Is it waiting for the sitemap crawl to complete? The purpose of this is not related to the sitemap crawling, it's purpose is to set priorities for certain links that appear in the nav. So it shouldn't be worried about the existing homepage task, which almost certainly hasn't started yet. I don't undrestand the HTTP vs. crawler result.
   **Follow up Answer**: You're absolutely right - I misunderstood the code flow. Looking more carefully:

   The 5-second delay is NOT waiting for the homepage to be crawled. It's just an arbitrary delay before making a direct HTTP request to fetch the homepage HTML to extract navigation links. The sequence is:

   1. Sitemap URLs are added to the queue
   2. A goroutine starts that waits 5 seconds
   3. Then it makes its own HTTP request to the homepage (separate from any crawling)
   4. Extracts navigation links from that HTML
   5. Updates those links' priorities to 1.000 in the database

   The delay seems to ensure the sitemap URLs are in the database before updating priorities, but 5 seconds is arbitrary. The "HTTP vs crawler" confusion was mine - this code makes its own HTTP request specifically to extract nav links for prioritisation, completely separate from the crawler workers. The delay could probably be removed entirely or replaced with a check that the sitemap processing is complete.
   **Follow up question**: Can we just remove the homepage task, or wait until the sitemap has been processed?
   **Follow up Answer**: Looking at the practical functionality:

   **Option 1: Remove the delay entirely** - This could work since:

   - The goroutine is spawned after sitemap URLs are enqueued
   - The header link extraction uses its own HTTP request (not dependent on crawling)
   - The only risk is if the UPDATE queries for nav link priorities happen before those URLs exist in the database

   **Option 2: Wait for sitemap processing** - Better approach:

   - The goroutine could wait for the `EnqueueURLsForJob` function to complete (line 720)
   - This ensures all sitemap URLs are in the database before updating priorities
   - Could use a channel or WaitGroup instead of arbitrary sleep

   **Practical recommendation**: Remove the delay and move the header link extraction code to run synchronously after `EnqueueURLsForJob` completes. This ensures proper ordering without arbitrary delays. The separate goroutine adds unnecessary complexity here since the work is lightweight and should happen in sequence anyway.
   **Added to TODO.md**

### Q: Why do we have both domain_id and ID? (internal/db/db.go:213)

**Answer**: The path alone isn't unique - multiple domains can have `/about` or `/contact`. The `pages` table uses `(domain_id, path)` as a unique constraint. The separate `id` primary key is standard practice for simpler foreign keys and better performance. This design is correct.

### Q: Should we pass retry count when a task completes after multiple attempts? (internal/db/queue.go:309)

**Answer**: Yes, recording retry count for successful tasks would be valuable for analytics - identifying flaky URLs and reliability patterns. However, it requires a schema change. Low priority but worth doing when updating the database schema for other reasons.
**Follow up**: What schema change is required? We already have that in the schema I thought?

## Feature Enhancements

### Q: Is there harm in starting jobs after first 5 pages from sitemap? (internal/jobs/manager.go:892)

**Answer**: Starting early would improve perceived performance but adds complexity: race conditions, incomplete job tracking, and harder error handling. The current approach (wait for complete sitemap) is more reliable. Only consider early start if users complain about initial wait times.

## Trivial

### Q: Should we use British English "Serialise" instead of "Serialize"? (internal/db/db.go:611)

**Answer**: Yes, per CLAUDE.md guidelines, use British/Australian English. Change to "Serialise" for consistency. This is a trivial fix but maintains code standard compliance.
**Added to TODO.md**

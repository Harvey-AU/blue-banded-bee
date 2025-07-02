# TODOS

## 🔴 Security & Authentication (High Priority)

[ ] [internal/api/admin.go:11,25](./internal/api/admin.go#L11) - Secure admin endpoints properly or remove them in production

## 🟡 API & User Experience Improvements (Medium Priority)

[ ] [internal/api/auth.go:63](./internal/api/auth.go#L63) - Set default organisation to user's Full Name
[ ] [internal/api/auth.go:73](./internal/api/auth.go#L73) - Fix organisation creation to remove .com suffix from names

## 🟡 Performance & Configuration

[ ] [internal/db/db.go:113-115](./internal/db/db.go#L113) - Consolidate database connection settings into single configuration location and make them configurable via environment variables

## 🟢 Code Quality & Architecture

[ ] [internal/jobs/worker.go:417](./internal/jobs/worker.go#L417) - Delete or update the comment if this code is not actually "legacy"
[x] [internal/jobs/manager.go:766](./internal/jobs/manager.go#L766) - Remove 5-second delay and goroutine for header link extraction. Move code to run synchronously after EnqueueURLsForJob completes
[ ] [internal/db/pages.go:35](./internal/db/pages.go#L35) - Pass domain name as parameter instead of making a DB call
[ ] [internal/db/queue.go:309](./internal/db/queue.go#L309) - Investigate if retry_count is already tracked for successful tasks and implement if not

## 🟢 Feature Enhancements

[ ] [internal/jobs/manager.go:798](./internal/jobs/manager.go#L798) - Ensure link finding works for buttons and non-"A" elements (reuse existing find_links logic)
[ ] [internal/jobs/manager.go:819-820](./internal/jobs/manager.go#L819) - Implement link priority ordering for header links (1st: 1.000, 2nd: 0.990, etc.)
[ ] [internal/jobs/manager.go:816](./internal/jobs/manager.go#L816) - Apply priority ordering logic to all discovered page links
[ ] [internal/db/db.go:404](./internal/db/db.go#L404) - Fix completion percentage to reflect actual completed vs skipped tasks (not always 100%)

## 🔵 Trivial

[x] [internal/db/db.go:611](./internal/db/db.go#L611) - Change "Serialize" to "Serialise" for British English consistency

## Restart job button in dashboard doesn't seem to be working

## Restart error seen when starting a new job, but then it starts working

b0b-b382-79d59adfb6d2","domain":"teamharvey.co","index":245,"url":"https://www.teamharvey.co/impact-results-category/community-22-23","time":"2025-06-11T11:47:19Z","message":"URL from sitemap"}
2025-06-11T11:47:19Z app[286764df62e328] syd [info]{"level":"debug","service":"blue-banded-bee","job_id":"c79d884c-ea95-4b0b-b382-79d59adfb6d2","domain":"teamharvey.co","index":246,"url":"https://www.teamharvey.co/impact-results-category/community-23-24","time":"2025-06-11T11:47:19Z","message":"URL from sitemap"}
2025-06-11T11:47:19Z app[286764df62e328] syd [info]{"level":"debug","service":"blue-banded-bee","job_id":"c79d884c-ea95-4b0b-b382-79d59adfb6d2","domain":"teamharvey.co","index":247,"url":"https://www.teamharvey.co/impact-results-category/environment-22-23","time":"2025-06-11T11:47:19Z","message":"URL from sitemap"}
2025-06-11T11:47:19Z app[286764df62e328] syd [info]{"level":"debug","service":"blue-banded-bee","domain_id":3,"page_count":253,"time":"2025-06-11T11:47:19Z","message":"Created page records"}
2025-06-11T11:47:19Z app[286764df62e328] syd [info]{"level":"debug","service":"blue-banded-bee","job_id":"c79d884c-ea95-4b0b-b382-79d59adfb6d2","total_urls":253,"new_urls":253,"skipped_urls":0,"time":"2025-06-11T11:47:19Z","message":"Enqueueing filtered URLs"}
2025-06-11T11:47:20Z app[286764df62e328] syd [info]{"level":"info","service":"blue-banded-bee","job_id":"c79d884c-ea95-4b0b-b382-79d59adfb6d2","domain":"teamharvey.co","url_count":253,"time":"2025-06-11T11:47:20Z","message":"Added sitemap URLs to job queue"}
2025-06-11T11:47:20Z app[286764df62e328] syd [info]{"level":"error","service":"blue-banded-bee","error":"job cannot be restarted: pending (only completed, failed, or cancelled jobs can be restarted)","job_id":"c79d884c-ea95-4b0b-b382-79d59adfb6d2","time":"2025-06-11T11:47:20Z","message":"Failed to start job after processing sitemap"}

## VERIFY LINK EXTRACTION FIX WORKS

Test case: `https://www.teamharvey.co/stories`

Expected behavior:

- Should find pagination links: `?b84bb98f_page=1`, `?b84bb98f_page=3`
- Job should show `found_tasks > 0` (not 0)
- Check in Supabase: SELECT found_tasks FROM jobs WHERE domain = 'teamharvey.co' ORDER BY created_at DESC LIMIT 1

If still showing `found_tasks = 0`, then the context bug fix didn't work and need to investigate further:

1. Check if `find_links` context is being set properly in `WarmURL()`
2. Add more debug logging to see exactly what's happening in link extraction
3. Possibly the issue is elsewhere in the pipeline

## Next Steps if Fix Doesn't Work

1. Add debug logging to see context values being set
2. Check if Colly OnHTML handlers are even being triggered
3. Verify the specific HTML structure matches our selectors
4. Test with simpler page first

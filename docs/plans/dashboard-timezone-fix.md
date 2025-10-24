# Dashboard "Today" Filter Timezone Issue

**Date:** 2025-10-24 **Status:** Identified - Not Yet Fixed **Priority:** LOW
(UX improvement)

---

## Problem

The dashboard's "today" filter uses UTC midnight boundaries, which causes jobs
created late in the user's local timezone to disappear from the dashboard once
UTC rolls over to the next day.

### Example

User in AEDT (UTC+11) creates jobs at:

- 9:25am local time (22:25 UTC on Oct 23)
- 9:28am local time (22:28 UTC on Oct 23)

At 11:18am local time (00:18 UTC on Oct 24), dashboard loads with `range=today`:

- Query filters for: `created_at >= '2025-10-24 00:00:00 UTC'`
- User's jobs from 2 hours ago don't show because they're "yesterday" in UTC
- User sees empty dashboard despite just creating jobs

---

## Current Implementation

**Dashboard query:**
[web/static/js/bb-auth-extension.js:145](../../../web/static/js/bb-auth-extension.js#L145)

```javascript
jobsResponse = await this.fetchData("/v1/jobs?limit=10&range=today");
```

**Backend date calculation:**
[internal/db/dashboard.go:317-326](../../../internal/db/dashboard.go#L317-L326)

```go
case "today":
    start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
    end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, time.UTC)
    startDate = &start
    endDate = &end
```

This uses **UTC midnight boundaries**, not user's local timezone.

---

## Proposed Solution

### Option 1: Client-Side Timezone Conversion (Recommended)

Send user's timezone offset from frontend, calculate "today" boundaries in
user's timezone on backend.

**Frontend change:**

```javascript
// Get user's timezone offset
const tzOffset = new Date().getTimezoneOffset(); // Minutes from UTC
const tz = Intl.DateTimeFormat().resolvedOptions().timeZone; // e.g., "Australia/Sydney"

jobsResponse = await this.fetchData(
  `/v1/jobs?limit=10&range=today&tz=${encodeURIComponent(tz)}`
);
```

**Backend change:**

```go
func calculateDateRangeForList(dateRange, timezone string) (*time.Time, *time.Time) {
    // Load user's timezone
    loc, err := time.LoadLocation(timezone)
    if err != nil {
        loc = time.UTC // Fallback to UTC if invalid
    }

    now := time.Now().In(loc) // Get current time in user's timezone

    switch dateRange {
    case "today":
        // Midnight in user's timezone
        start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
        end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, loc)
        startDate = &start
        endDate = &end
    // ... rest of cases
    }

    return startDate, endDate
}
```

**API signature change:**

```go
// internal/api/jobs.go:174
timezone := r.URL.Query().Get("tz") // Get timezone from query param
jobs, total, err := h.DB.ListJobs(orgID, limit, offset, status, dateRange, timezone)
```

---

### Option 2: Change Default to "Last 7 Days" (Quick Fix)

Simpler but less intuitive - "today" still exists but isn't the default.

**Frontend change:**

```javascript
// Change default from "today" to "last7"
jobsResponse = await this.fetchData("/v1/jobs?limit=10&range=last7");
```

**Pros:**

- One-line fix
- Works immediately
- No timezone complexity

**Cons:**

- "Today" filter still broken if user selects it
- Less intuitive (users expect to see "today's" jobs by default)

---

### Option 3: Relative "Last 24 Hours" (Alternative)

Make "today" mean "last 24 hours" instead of "calendar day".

**Backend change:**

```go
case "today":
    start := now.Add(-24 * time.Hour)
    startDate = &start
    endDate = &now
```

**Pros:**

- No timezone complexity
- Shows recent jobs regardless of UTC rollover

**Cons:**

- "Today" doesn't actually mean "today" anymore
- Less intuitive for users ("today" = calendar day in their mind)

---

## Recommendation

**Implement Option 1 (timezone conversion)** because:

1. Matches user mental model ("today" = my today, not UTC today)
2. Scales to other timezone-sensitive features (reports, analytics)
3. Go's `time.LoadLocation()` handles timezone complexity for us
4. Small API change (add `?tz=` query param)

**Fallback:** If timezone conversion proves complex, use Option 2 (last7) as
temporary fix while working on proper solution.

---

## Implementation Steps

1. **Frontend:** Detect and send user timezone
   - Modify `bb-auth-extension.js` to append `?tz=` param
   - Use `Intl.DateTimeFormat().resolvedOptions().timeZone`

2. **API:** Accept timezone parameter
   - Update `jobs.go:174` to read `tz` query param
   - Pass to `ListJobs()` function

3. **Backend:** Convert date ranges to user timezone
   - Update `ListJobs()` signature to accept timezone
   - Modify `calculateDateRangeForList()` to use `time.LoadLocation()`
   - Handle invalid/missing timezone gracefully (fallback to UTC)

4. **Testing:**
   - Test with various timezones (AEDT, PST, UTC, etc.)
   - Test with invalid timezone (should fallback to UTC)
   - Test boundary conditions (jobs created near midnight)

---

## Files to Modify

1. `web/static/js/bb-auth-extension.js` - Send timezone from frontend
2. `internal/api/jobs.go` - Accept timezone parameter
3. `internal/db/dashboard.go` - Convert date ranges to user timezone
4. Tests for timezone conversion logic

---

## Related Issues

None currently, but this affects:

- Job statistics API (`/v1/jobs/stats`)
- Activity charts (if we add them)
- Any future "today/this week/this month" filters

Consider making timezone handling consistent across all date-filtered endpoints.

---

## Decision

**DEFERRED** - Will implement Option 1 (timezone conversion) after addressing
the trigger storm issue, as that's higher priority for system stability.

For now, users can work around by:

- Using "Last 7 Days" filter instead of "Today"
- Refreshing dashboard before UTC midnight rolls over
- Viewing all jobs (no filter)

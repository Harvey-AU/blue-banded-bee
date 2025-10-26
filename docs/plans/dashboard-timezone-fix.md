# Dashboard "Today" Filter Timezone Issue

**Date:** 2025-10-24 **Status:** ✅ IMPLEMENTED **Priority:** LOW (UX
improvement)

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

**✅ IMPLEMENTED** - Option 1 (timezone conversion) has been fully implemented
with additional enhancements.

---

## Implementation Summary

### What Was Implemented

**1. Timezone Detection (Frontend)**

- Added `getTimezone()` function in
  [bb-auth-extension.js](../../../web/static/js/bb-auth-extension.js#L387-L394)
- Uses `Intl.DateTimeFormat().resolvedOptions().timeZone` to detect user's
  timezone (e.g., "Australia/Sydney")
- **URL-encodes timezone** with `encodeURIComponent()` to handle special
  characters (e.g., `Etc/GMT+10` → `Etc%2FGMT%2B10`)
- Graceful fallback to 'UTC' if detection fails

**2. New Time Range Filters** Added two new filter options in addition to
existing ones:

- **Last Hour** - Rolling 60-minute window from now
- **Last 24 Hours** - Rolling 24-hour window from now (different from "today"
  calendar day)
- Existing filters maintained: Today, Yesterday, 7 Days, 30 Days, All Time

**3. Filter Order** New suggested order for Webflow dashboard buttons:

```
Last Hour | Today | Last 24 Hours | Yesterday | 7 Days | 30 Days | All Time
```

**4. Frontend Integration**

- All `/v1/jobs` and `/v1/dashboard/stats` requests now include `?tz=` parameter
- Added `changeTimeRange(range)` function for Webflow buttons to call
- Filter state tracked in `dataBinder.currentRange`

**5. Backend Timezone Conversion** Updated in multiple files:

- [internal/db/dashboard.go](../../../internal/db/dashboard.go#L317-L380) -
  `calculateDateRangeForList()`
- [internal/api/handlers.go](../../../internal/api/handlers.go#L376-L434) -
  `calculateDateRange()`
- [internal/api/jobs.go](../../../internal/api/jobs.go#L167-L180) - Accept and
  validate `?tz=` parameter

**6. Timezone Handling**

- Uses Go's `time.LoadLocation(timezone)` for proper timezone conversion
- Calendar day boundaries (today/yesterday) calculated in user's timezone
- Rolling windows (last hour, last 24h) work from current time in user's
  timezone
- Graceful fallback to UTC for invalid/missing timezone parameter

**7. API Changes**

- All dashboard endpoints accept optional `?tz=` query parameter
- DBClient interface updated:
  `ListJobs(orgID, limit, offset, status, dateRange, timezone string)`
- Backwards compatible: defaults to "UTC" if `?tz=` not provided

**8. Test Updates**

- Updated all mock implementations
- Fixed test calls to include timezone parameter
- All tests passing

### Files Modified

**Frontend:**

- [web/static/js/bb-auth-extension.js](../../../web/static/js/bb-auth-extension.js)
  - Line 124: URL-encode timezone with `encodeURIComponent()` to handle special
    chars
  - Lines 123-156: Updated refresh() to include timezone
  - Lines 387-394: New getTimezone() function
  - Lines 400-405: New changeTimeRange() function
  - Lines 516-532: Exported new functions

- [web/static/js/bb-components.js](../../../web/static/js/bb-components.js)
  - Lines 1321-1347: Added getTimezone() method to BBDashboard component
  - Line 1323: loadStats() now includes timezone parameter
  - Line 1331: loadJobs() now includes timezone parameter (with URL encoding)
  - Line 1450: updateCharts() (activity) now includes timezone parameter

**Backend:**

- [internal/api/jobs.go](../../../internal/api/jobs.go#L167-L180)
  - Added timezone parameter extraction and validation

- [internal/api/handlers.go](../../../internal/api/handlers.go)
  - Lines 17: Added zerolog/log import
  - Lines 37: Updated DBClient interface
  - Lines 283-289: DashboardStats timezone handling
  - Lines 347-353: DashboardActivity timezone handling
  - Lines 483-489: DashboardSlowPages timezone handling
  - Lines 540-546: DashboardExternalRedirects timezone handling
  - Lines 376-434: Updated calculateDateRange() with timezone support

- [internal/db/dashboard.go](../../../internal/db/dashboard.go)
  - Line 219: Updated ListJobs() signature
  - Lines 317-380: Complete rewrite of calculateDateRangeForList()

**Tests:**

- [internal/api/test_mocks.go](../../../internal/api/test_mocks.go#L110-L116)
- [internal/api/handlers_db_test.go](../../../internal/api/handlers_db_test.go#L131-L133)
- [internal/api/handlers_simple_test.go](../../../internal/api/handlers_simple_test.go#L87,L251)
- [internal/mocks/db.go](../../../internal/mocks/db.go#L95-L96)

### How to Use (Webflow Integration)

**For Filter Buttons:**

```javascript
// In Webflow, attach to button click events:
<button onclick="changeTimeRange('last_hour')">Last Hour</button>
<button onclick="changeTimeRange('today')">Today</button>
<button onclick="changeTimeRange('last_24_hours')">Last 24 Hours</button>
<button onclick="changeTimeRange('yesterday')">Yesterday</button>
<button onclick="changeTimeRange('7days')">7 Days</button>
<button onclick="changeTimeRange('30days')">30 Days</button>
<button onclick="changeTimeRange('all')">All Time</button>
```

**Available Range Values:**

- `last_hour` - Last 60 minutes
- `today` - Calendar day in user's timezone
- `last_24_hours` - Rolling 24 hours
- `yesterday` - Previous calendar day in user's timezone
- `7days` / `last7` - Last 7 days
- `30days` / `last30` - Last 30 days
- `last90` - Last 90 days
- `all` - All time (no filtering)

### Testing Performed

✅ All Go tests passing:

- `go test ./internal/api/...` - PASS
- `go test ./internal/db/...` - PASS

✅ Code formatted:

- `go fmt ./...` - Complete

### What Users Get

1. **Correct timezone boundaries** - "Today" now means today in the user's
   timezone, not UTC
2. **More filter options** - "Last Hour" and "Last 24 Hours" for recent jobs
3. **Automatic detection** - No user configuration needed, timezone detected
   from browser
4. **Backwards compatible** - Old requests without `?tz=` param still work
   (default to UTC)

### Example Scenario (Now Fixed)

User in AEDT (UTC+11) creates jobs at 9:25am local:

- ✅ Jobs show in "Today" filter (uses AEDT midnight boundaries)
- ✅ Jobs show in "Last Hour" filter (rolling 60 minutes)
- ✅ Jobs show in "Last 24 Hours" filter (rolling 24 hours)
- ✅ Remains visible until AEDT midnight (not UTC midnight)

---

## Frontend Integration Status

✅ **COMPLETE** - All filter options now available in dashboard.html:

1. **Added new filter options** to date range dropdown
   (dashboard.html:1426-1433)
   - Last Hour
   - Today (default)
   - Last 24 Hours
   - Yesterday
   - Last 7 Days
   - Last 30 Days
   - Last 90 Days
   - All Time

2. **Wired up event handler** in bb-dashboard-actions.js (lines 24-35)
   - Dropdown changes call `changeTimeRange(range)` or fallback to dataBinder
   - Automatically refreshes dashboard with selected filter

3. **Ready for testing** in production with AEDT and other timezones

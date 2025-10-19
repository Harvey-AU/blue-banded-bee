# Issue #1: Cache Warming Timeout Fix

## Goal

Prevent tasks from running for 11+ hours by fixing two timeout issues:

1. Stop retrying cache warming on uncacheable content (wastes 54 seconds)
2. Ensure Colly respects the 2-minute task timeout

## Problem Analysis

### Issue #1a: Cache Warming Loop on Uncacheable Pages (PRIMARY CAUSE)

**Current behaviour:**

```go
// crawler.go - shouldMakeSecondRequest()
case "MISS", "BYPASS", "EXPIRED":
    return true  // ← Problem: BYPASS/DYNAMIC never become HIT
```

**What happens:**

- Pages returning `BYPASS` or `DYNAMIC` trigger cache warming loop
- Loop runs 10 iterations with delays: 2+3+4+5+6+7+8+9+10 = **54 seconds**
- These cache statuses indicate **uncacheable content** (personalised/dynamic)
- Waiting for them to become `HIT` is futile

**Evidence:**

- aesop.com homepage: `BYPASS` (personalised)
- realestate.com.au homepage: `DYNAMIC` (personalised)
- Both spend 54s in loop, then hit timeout issues

### Issue #1: Colly Timeout Not Enforced (SECONDARY CAUSE)

**Current behaviour:**

```go
// crawler.go - executeCollyRequest()
go func() {
    visitErr := collyClone.Visit(targetURL)
    collyClone.Wait()  // ← Blocks indefinitely if request hangs
    done <- nil
}()

select {
case <-done:        // Never fires if goroutine hangs
case <-ctx.Done():  // Returns, but goroutine keeps running
}
```

**Problem:**

- Colly's `Visit()` and `Wait()` don't accept context
- If HTTP request hangs, goroutine runs forever (11+ hours observed)
- `ctx.Done()` causes parent to return, but zombie goroutine persists

## Technical Solution

### Fix #1a: Exclude Uncacheable Statuses (TRIVIAL - 1 line)

**File:** `internal/crawler/crawler.go` **Line:** ~475-480

**Change:**

```go
func shouldMakeSecondRequest(cacheStatus string) bool {
    switch strings.ToUpper(cacheStatus) {
    case "MISS", "EXPIRED":  // ← Remove "BYPASS"
        return true
    default:
        return false
    }
}
```

**Impact:**

- Eliminates 54-second waste on aesop.com, realestate.com.au homepages
- Reduces task duration by ~45% for personalised content sites
- No downside: `BYPASS`/`DYNAMIC` content is **never** cacheable

### Fix #1: Add Colly Request Timeout (SIMPLE - 3-4 lines)

**File:** `internal/crawler/config.go` - Add new field **File:**
`internal/crawler/crawler.go` - Thread config through to collector

**Step 1 - Add config field:**

```go
// internal/crawler/config.go
type Config struct {
    // ... existing fields
    RequestTimeout time.Duration  // ← Add this
}

func DefaultConfig() *Config {
    return &Config{
        // ... existing fields
        RequestTimeout: 2 * time.Minute,  // ← Add default
    }
}
```

**Step 2 - Use in collector:**

```go
// internal/crawler/crawler.go - New() function
c := colly.NewCollector(
    colly.AllowURLRevisit(),
    colly.Async(),
    colly.RequestTimeout(config.RequestTimeout),  // ← Add this
)
```

**Step 3 - Stop collector when context expires:**

```go
// internal/crawler/crawler.go - executeCollyRequest()
go func() {
    visitErr := collyClone.Visit(targetURL)
    collyClone.Wait()
    done <- visitErr
}()

// Add goroutine to stop collector if context times out
go func() {
    <-ctx.Done()
    // Force stop the collector to abort any in-flight requests
    collyClone.Stop()
}()

select {
case err := <-done:
    return err  // Normal completion
case <-ctx.Done():
    return ctx.Err()  // Timeout - collector stopped by second goroutine
}
```

**Impact:**

- Colly forcibly aborts HTTP requests after 2 minutes (via RequestTimeout)
- Context expiry triggers `collector.Stop()` to abort the goroutine
- Prevents zombie goroutines when requests hang indefinitely

## Testing Strategy

### Test Case 1: Uncacheable Content

1. Create job for aesop.com (known `BYPASS` homepage)
2. Verify task completes in <10 seconds (not 54+ seconds)
3. Check logs confirm "No cache warming needed - cache already available or not
   cacheable"

### Test Case 2: Hung Request Timeout

1. Mock HTTP server that accepts connection but never responds
2. Create task targeting mock server
3. Verify task fails with timeout error after 2 minutes (not 11+ hours)

### Test Case 3: Normal Cache Warming Still Works

1. Create job for site with `MISS` → `HIT` transition
2. Verify cache warming loop still runs
3. Confirm second request made and metrics captured

## Deployment Impact

**Risk Level:** LOW

**Breaking Changes:** None

**Performance Impact:**

- Positive: ~54 second reduction for uncacheable pages
- Positive: Tasks guaranteed to fail within 2 minutes vs 11+ hours

**Rollback Plan:**

- Revert single-line change to `shouldMakeSecondRequest()`
- Remove `colly.RequestTimeout()` parameter

## Success Metrics

**Before:**

- aesop.com homepage task: 11+ hours (timeout failure)
- realestate.com.au homepage: 11+ hours (timeout failure)
- Tasks stuck in `running` state indefinitely

**After:**

- aesop.com homepage task: <10 seconds (completes successfully)
- realestate.com.au homepage: <10 seconds (completes successfully)
- All tasks complete or fail within 2 minutes

**Monitoring:**

- Sentry alerts for timeout errors drop to near-zero
- Average task duration decreases by 30-50% for personalised sites
- Zero tasks remain in `running` state >2 minutes

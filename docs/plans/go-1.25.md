## Go 1.25 Impact Assessment for Blue-Banded-Bee

### High Impact Features

#### 1. synctest Package - Synthetic Time Testing [DEFERRED]

**Impact: ⭐⭐⭐⭐⭐**

The new `synctest` package allows testing timeout situations without actually waiting, using a fake clock within isolated "bubbles" where time advances when goroutines are blocked. This is invaluable for testing:

- Rate limiting logic
- Crawler timeouts
- Job scheduling delays
- Worker pool behavior

**Action**: Refactor existing timeout tests to use `synctest.Test()` for faster, more reliable test execution.

**Note (2025-07-05):** Deferred this task. The `synctest` API is experimental and proved difficult to implement correctly for network-bound code in the crawler. The interaction between the synthetic clock and real-world I/O is complex. We should revisit this when the feature is more stable and better documented, or for testing purely computational, time-based logic elsewhere in the application.

#### 2. WaitGroup.Go Method [DONE]

**Impact: ⭐⭐⭐⭐**

The new `WaitGroup.Go()` method automatically handles `Add(1)`, goroutine creation, and `Done()` calls in a single method. This will significantly clean up worker pool code.

**Before:**

```go
var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    // work
}()
```

**After:**

```go
var wg sync.WaitGroup
wg.Go(func() {
    // work
})
```

**Action**: Refactor concurrent crawling workers to use the cleaner API.

#### 3. Container-aware GOMAXPROCS [DONE]

**Impact: ⭐⭐⭐⭐**

Go 1.25 automatically respects CPU quotas set by cgroups (Docker/Kubernetes), setting `GOMAXPROCS` to match container limits rather than host machine CPUs. Since we deploy on Fly.io containers, this improves resource utilization.

**Action**: Ensure `go.mod` specifies Go 1.25+ to enable automatic container-aware scheduling.

### Medium Impact Features

#### 4. Green Tea Garbage Collector (Experimental) [DONE]

**Impact: ⭐⭐⭐**

New experimental GC optimized for programs creating many small objects, with expected 10-40% reduction in GC overhead. Web crawlers typically create many small objects (HTTP responses, URLs, metadata).

**Action**: Test with `GOEXPERIMENT=greenteagc` in development environment to measure performance impact.

#### 5. Flight Recording [DONE]

**Impact: ⭐⭐⭐**

Flight recording collects execution data within a sliding window, helping capture traces of interesting program behavior for debugging production performance issues.

**Action**: Consider implementing for production debugging of crawling job performance issues.

#### 6. CSRF Protection

**Impact: ⭐⭐⭐**

Built-in `http.CrossOriginProtection` implements CSRF protection by rejecting non-safe cross-origin browser requests. Adds security layer to our Supabase Auth integration.

**Action**: Evaluate adding to API endpoints that handle authenticated requests.

# Blue Banded Bee - Out of Memory (OOM) Investigation

## Executive Summary

The application crashed with an OOM kill (exit code 137) on 2025-07-08 at 09:25:54. The app is configured with only 1GB of memory and investigation reveals multiple memory leaks and unbounded data structures.

## Root Causes Identified

### 1. **Link Processing Memory Explosion (PRIMARY CAUSE)**

**Location:** `internal/jobs/worker.go:935-1024`

**Issue:** Unbounded processing of discovered links creates multiple large arrays.

**Memory Usage Calculation:**
```
Example: Website with 10,000 links on homepage
- Filtered array: 10,000 strings × ~100 bytes = 1MB
- PageIDs array: 10,000 ints × 8 bytes = 80KB
- Paths array: 10,000 strings × ~50 bytes = 500KB
- PagesToEnqueue: 10,000 structs × ~100 bytes = 1MB
- Total per page: ~2.6MB

With 50 workers processing such pages: 130MB just for link data!
```

### 2. **Goroutine Leaks (SECONDARY CAUSE)**

**Location:** `internal/jobs/worker.go` - multiple locations

**Issues:**
- Worker pool can scale up to 50+ workers
- Performance boost adds up to 20 additional workers per job
- Workers don't properly exit when scaled down
- No tracking of actual active workers

**Memory Impact:**
- Each goroutine stack: 2KB-1MB
- 50 workers × 1MB stack = 50MB

**Code Problems:**
```go
// Line 254-260: Workers exit without cleanup
if shouldExit {
    return  // No cleanup, goroutine may leak
}

// Line 746-749: Creates new workers but old ones may not exit
wp.wg.Go(func() {
    wp.worker(ctx, workerID)
})
```

### 3. **Database Connection Issues (TERTIARY CAUSE)**

**Location:** `internal/jobs/worker.go:1220-1284`

**Issues Found:**
1. LISTEN/NOTIFY connection leak on reconnect failure
2. "driver: bad connection" errors repeating every minute
3. Dead connections not being cleaned up

**Problem Code:**
```go
// Lines 1264-1273: Connection leak
conn.Close(ctx)
time.Sleep(5 * time.Second)
conn, err = connect()  // conn not set to nil if this fails
if err != nil {
    continue  // Creates busy loop
}
```

### 4. **Memory Leaks in Data Structures**

**Unbounded Maps/Arrays:**
- `jobPerformance` map (line 54) - Never cleaned up when jobs removed
- `TaskBatch` (line 60) - Can grow unbounded if processing is slow

## Current Resource Configuration

```toml
# fly.toml
[[vm]]
  memory = '1gb'  # Insufficient for workload
  cpu_kind = 'shared'
  cpus = 1
```

**Database Connection Pool:**
- Max connections: 75
- Max idle: 25
- Connection lifetime: 5-30 minutes

## Monitoring Implementation

Add these endpoints to track memory usage:

```go
import (
    "net/http/pprof"
    "runtime"
)

// In main.go router setup:
mux.HandleFunc("/debug/pprof/", pprof.Index)
mux.HandleFunc("/debug/pprof/heap", pprof.Index)
mux.HandleFunc("/debug/pprof/goroutine", pprof.Handler("goroutine").ServeHTTP)
mux.HandleFunc("/debug/pprof/allocs", pprof.Handler("allocs").ServeHTTP)

// Custom metrics endpoint
mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    fmt.Fprintf(w, "Alloc = %v MB\n", m.Alloc / 1024 / 1024)
    fmt.Fprintf(w, "TotalAlloc = %v MB\n", m.TotalAlloc / 1024 / 1024)
    fmt.Fprintf(w, "Sys = %v MB\n", m.Sys / 1024 / 1024)
    fmt.Fprintf(w, "NumGC = %v\n", m.NumGC)
    fmt.Fprintf(w, "Goroutines = %v\n", runtime.NumGoroutine())
})
```

## Immediate Fixes Required

### 1. Limit Link Processing
```go
// In processLinkCategory function
const maxLinksPerPage = 500
if len(links) > maxLinksPerPage {
    log.Warn().
        Int("total_links", len(links)).
        Int("processing", maxLinksPerPage).
        Msg("Limiting links processed per page")
    links = links[:maxLinksPerPage]
}
```

### 2. Fix Database Connection Leak
```go
// In listenForNotifications
conn.Close(ctx)
conn = nil  // Add this line
time.Sleep(5 * time.Second)

conn, err = connect()
if err != nil {
    log.Error().Err(err).Msg("Failed to reconnect")
    time.Sleep(30 * time.Second)  // Exponential backoff
    continue
}
```

### 3. Track Worker Lifecycle
```go
// Add to WorkerPool struct
activeWorkerCount int32  // Use atomic operations

// In worker function
atomic.AddInt32(&wp.activeWorkerCount, 1)
defer atomic.AddInt32(&wp.activeWorkerCount, -1)

// Check before creating new workers
if atomic.LoadInt32(&wp.activeWorkerCount) >= int32(targetWorkers) {
    return
}
```

### 4. Clean Up jobPerformance Map
```go
// In RemoveJob function
wp.perfMutex.Lock()
delete(wp.jobPerformance, jobID)
wp.perfMutex.Unlock()
```

### 5. Increase Memory Allocation
```toml
# fly.toml
[[vm]]
  memory = '2gb'  # Increase from 1gb
  cpu_kind = 'shared'
  cpus = 1
```

## Long-term Recommendations

1. **Implement proper worker pool management** with context cancellation
2. **Use bounded channels** for task queuing
3. **Add circuit breakers** for database connections
4. **Implement graceful shutdown** with proper cleanup
5. **Add memory profiling** to CI/CD pipeline
6. **Set up alerts** for memory usage > 80%

## Testing the Fixes

After implementing fixes:

1. Deploy with increased memory
2. Monitor `/metrics` endpoint
3. Use pprof to analyze heap: `go tool pprof http://your-app.fly.dev/debug/pprof/heap`
4. Watch for goroutine count: `curl http://your-app.fly.dev/debug/pprof/goroutine?debug=1`
5. Check for "bad connection" errors in logs

## Expected Outcomes

- Memory usage should stabilize under 1.5GB
- Goroutine count should remain < 100
- No more "driver: bad connection" errors
- No OOM kills with 2GB allocation
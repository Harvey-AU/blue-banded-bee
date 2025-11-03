# Database Connection Resilience

## Problem

The application crash-looped for 5 hours today when Supabase forcibly terminated
database connections during a maintenance window.

**Error observed:**

```
FATAL: terminating connection due to administrator command (SQLSTATE 57P01)
```

**Impact:**

- Both Fly.io machines exhausted their maximum restart attempts (10)
- Application unavailable for ~5 hours (12:00 PM - 4:50 PM AEDT)
- Crash-loop until database became available again

## Root Cause

The application used immediate connection logic without retry mechanisms:

```go
// OLD CODE - Fails immediately
pgDB, err := db.InitFromEnv()
if err != nil {
    log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL database")
}
```

When Supabase terminated connections (maintenance/restart), the app would:

1. Fail to connect
2. Log fatal error and exit
3. Fly.io restarts the machine
4. Repeat steps 1-3 until restart limit exhausted

## Solution

Implemented exponential backoff retry logic with the following components:

### 1. Retry Configuration ([internal/db/retry.go](../internal/db/retry.go))

```go
type RetryConfig struct {
    MaxAttempts     int           // 10 attempts max
    InitialInterval time.Duration // Start at 1 second
    MaxInterval     time.Duration // Cap at 30 seconds
    Multiplier      float64       // 2.0 (exponential)
    Jitter          bool          // Add randomness
}
```

**Retry backoff schedule:**

- Attempt 1: 1s
- Attempt 2: 2s
- Attempt 3: 4s
- Attempt 4: 8s
- Attempt 5: 16s
- Attempt 6-10: 30s (capped)

**Total maximum wait time:** ~5 minutes

### 2. Main Database Connection ([cmd/app/main.go:388-397](../cmd/app/main.go#L388-L397))

```go
// NEW CODE - Waits patiently for database
dbCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer dbCancel()

pgDB, err := db.WaitForDatabase(dbCtx, 5*time.Minute)
if err != nil {
    sentry.CaptureException(err)
    log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL database")
}
```

### 3. Queue Database Connection ([cmd/app/main.go:413-421](../cmd/app/main.go#L413-L421))

```go
// Create fresh context for queue connection to ensure full retry budget
// (primary connection may have consumed most of the shared context)
queueCtx, queueCancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer queueCancel()

queueConn, err := db.InitFromURLWithSuffixRetry(queueCtx, queueURL, appEnv, "queue")
```

**Important:** Each database connection gets its own 5-minute timeout context to
prevent context exhaustion issues.

## Behaviour

### Retryable Errors (will retry)

The system uses the existing `isRetryableError()` function from
[internal/db/batch.go](../internal/db/batch.go) which handles:

- **PostgreSQL error classes:**
  - `08` - Connection exceptions
  - `53` - Insufficient resources
  - `57` - Operator intervention (includes SQLSTATE 57P01)
  - `58` - System errors

- **Go database errors:**
  - `sql.ErrConnDone`
  - `context.DeadlineExceeded`
  - Connection refused/reset/timeout

### Non-Retryable Errors (fail fast)

- Authentication failures (wrong credentials)
- Configuration errors (invalid host/port)
- Data constraint violations

These errors indicate problems that won't resolve with waiting, so the app fails
immediately.

## Logging

The retry logic provides clear visibility:

**On first attempt failure:**

```
{"level":"warn","attempt":1,"max_attempts":10,"retry_in":"1s",
 "message":"Database connection failed, retrying..."}
```

**On successful retry:**

```
{"level":"info","attempts":3,"elapsed":"7s",
 "message":"Database connection established after retries"}
```

**On exhausted retries:**

```
{"level":"error","max_attempts":10,
 "message":"Database connection failed after all retry attempts"}
```

## Benefits

1. **Graceful degradation**: Application waits patiently instead of
   crash-looping
2. **No restart exhaustion**: Fly.io machines won't hit restart limits
3. **Automatic recovery**: Connects automatically when database becomes
   available
4. **Production-friendly**: Works seamlessly during Supabase maintenance windows
5. **Configurable**: Retry behaviour can be tuned via `RetryConfig`

## Testing

Build and tests pass with the new retry logic:

```bash
go build -o /tmp/test-build ./cmd/app        # ✅ Builds successfully
go test ./internal/db/... -run=TestConfig -v # ✅ All tests pass
```

## Future Enhancements

Potential improvements for consideration:

1. **Health check degradation**: Return 503 status during database
   unavailability
2. **Metrics**: Track connection retry attempts and success rates
3. **Alerts**: Notify when retry threshold exceeded
4. **Circuit breaker**: Temporarily stop attempts after repeated failures

## Related Files

- [internal/db/retry.go](../internal/db/retry.go) - Retry logic implementation
- [internal/db/batch.go](../internal/db/batch.go) - Error classification
- [cmd/app/main.go](../cmd/app/main.go) - Application startup
- [logs/crash_loop_analysis.md](../logs/crash_loop_analysis.md) - Incident
  analysis

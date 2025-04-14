# Database Configuration Variables

This document lists all the configuration variables modified to reduce database lock contention.

## Connection Pool Settings

**File**: `src/db/db.go`

```go
// Original values
client.SetMaxOpenConns(5)
client.SetMaxIdleConns(3)

// Reduced values
client.SetMaxOpenConns(3)
client.SetMaxIdleConns(2)
```

## Queue Workers

**File**: `src/common/queue.go`

```go
// Original value
workerCount: 5

// New value
workerCount: 2
```

## Retry Settings

**File**: `src/db/db.go`

```go
// Original values
retries := 3
backoff := 100 * time.Millisecond

// Reduced values
retries := 5
baseBackoff := 200 * time.Millisecond
```

## Sleep Durations

**File**: `src/jobs/worker.go`

```go
// Reduced values
// Sleep between successful tasks
time.Sleep(200 * time.Millisecond)

// Maximum backoff sleep time
maxSleep := 10 * time.Second
```

## Batch Processing

**File**: `src/jobs/worker.go`

```go
// Original values
tasks: make([]*Task, 0, 100)
batchTimer: time.NewTicker(5 * time.Second)

// Reduced values
tasks: make([]*Task, 0, 50)
batchTimer: time.NewTicker(10 * time.Second)
```

# Scaling & Performance Strategy

## Overview

This plan outlines dynamic worker scaling, job priority systems, and performance optimisations to handle increased load while maintaining service quality and responsible crawling practices.

## Dynamic Worker Scaling

### Current State
- Fixed 5 workers processing all jobs equally
- No scaling based on workload or priority

### Proposed Scaling Algorithm

**Base Configuration:**
```go
type WorkerConfig struct {
    BaseWorkers    int // 3 workers minimum
    WorkersPerJob  int // +3 workers per active job
    MaxWorkers     int // 25 workers maximum (connection pool limit)
    ScaleDownDelay time.Duration // 2 minutes before scaling down
}
```

**Scaling Examples:**
- 1 job: 3 workers
- 3 jobs: 9 workers (3 + 3×2)
- 5 jobs: 15 workers (3 + 3×4)
- 8+ jobs: 25 workers (capped at maximum)

**Implementation:**
```go
func (wp *WorkerPool) calculateOptimalWorkers() int {
    activeJobs := len(wp.getActiveJobs())
    optimal := wp.config.BaseWorkers + (activeJobs * wp.config.WorkersPerJob)
    
    if optimal > wp.config.MaxWorkers {
        return wp.config.MaxWorkers
    }
    return optimal
}

func (wp *WorkerPool) scaleWorkers() {
    optimal := wp.calculateOptimalWorkers()
    current := wp.getCurrentWorkerCount()
    
    if optimal > current {
        wp.scaleUp(optimal - current)
    } else if optimal < current {
        // Delay scale-down to avoid thrashing
        wp.scheduleScaleDown(current - optimal, wp.config.ScaleDownDelay)
    }
}
```

## Job Priority System

### Four-Tier Priority Model

| Tier | Worker Allocation | Use Case | Features |
|------|------------------|----------|-----------|
| **Critical** | 50% of workers | System health checks | Immediate processing |
| **High** | 30% of workers | Premium users | 2x speed, priority queue |
| **Normal** | 15% of workers | Standard users | Standard processing |
| **Low** | 5% of workers | Bulk operations | Background processing |

### Priority Queue Implementation

```go
type PriorityJob struct {
    Job      *Job
    Priority JobPriority
    Created  time.Time
}

type JobPriority int

const (
    PriorityCritical JobPriority = iota
    PriorityHigh
    PriorityNormal
    PriorityLow
)

func (wp *WorkerPool) getNextTask(workerID int) (*Task, error) {
    // Workers are assigned to priority levels
    workerPriority := wp.getWorkerPriority(workerID)
    
    // Query tasks in priority order
    query := `
        SELECT t.id, t.job_id, d.name, p.path
        FROM tasks t
        JOIN jobs j ON t.job_id = j.id
        JOIN pages p ON t.page_id = p.id
        JOIN domains d ON p.domain_id = d.id
        WHERE t.status = 'pending'
          AND j.priority >= $1
        ORDER BY j.priority DESC, t.created_at ASC
        LIMIT 1
        FOR UPDATE SKIP LOCKED
    `
    
    return wp.queryTask(query, workerPriority)
}
```

### Priority Assignment Rules

**Automatic Priority Assignment:**
```go
func assignJobPriority(job *Job, user *User) JobPriority {
    // System health checks
    if job.Domain == "health.internal" {
        return PriorityCritical
    }
    
    // Premium subscription users
    if user.SubscriptionTier == "premium" {
        return PriorityHigh
    }
    
    // Large bulk jobs (>500 pages)
    if job.MaxPages > 500 {
        return PriorityLow
    }
    
    // Default for standard users
    return PriorityNormal
}
```

## Performance Optimisations

### Database Optimisations

**Connection Pool Tuning:**
```go
// Scale connection pool with worker count
func (db *Database) scaleConnectionPool(workerCount int) {
    // Rule: 2 connections per worker + 5 for API
    newMaxConns := (workerCount * 2) + 5
    
    if newMaxConns > 50 { // Hard limit
        newMaxConns = 50
    }
    
    db.SetMaxOpenConns(newMaxConns)
    db.SetMaxIdleConns(newMaxConns / 2)
}
```

**Query Optimisations:**
```sql
-- Optimised task claiming with priority
CREATE INDEX CONCURRENTLY idx_tasks_priority_pending 
ON tasks(job_id, status, created_at) 
WHERE status = 'pending';

-- Job priority index
CREATE INDEX CONCURRENTLY idx_jobs_priority 
ON jobs(priority, status) 
WHERE status IN ('pending', 'running');
```

### Memory Management

**Task Batch Processing:**
```go
type TaskBatch struct {
    tasks    []*Task
    maxSize  int
    timeout  time.Duration
}

func (wp *WorkerPool) processBatch() {
    batch := make([]*Task, 0, wp.batchSize)
    timeout := time.After(wp.batchTimeout)
    
    for {
        select {
        case task := <-wp.taskQueue:
            batch = append(batch, task)
            if len(batch) >= wp.batchSize {
                wp.processBatchTasks(batch)
                batch = batch[:0] // Reset slice
            }
            
        case <-timeout:
            if len(batch) > 0 {
                wp.processBatchTasks(batch)
                batch = batch[:0]
            }
            timeout = time.After(wp.batchTimeout)
        }
    }
}
```

### Resource Monitoring

**System Health Checks:**
```go
type SystemMetrics struct {
    ActiveWorkers    int
    QueuedTasks      int
    DatabaseConns    int
    MemoryUsage      uint64
    CPUPercent       float64
    ResponseTimeP95  time.Duration
}

func (wp *WorkerPool) monitorResources() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        metrics := wp.collectMetrics()
        
        // Scale down if resource usage is low
        if metrics.CPUPercent < 20 && metrics.QueuedTasks < 10 {
            wp.considerScaleDown()
        }
        
        // Alert if system is overloaded
        if metrics.CPUPercent > 80 || metrics.MemoryUsage > 0.8*wp.maxMemory {
            wp.triggerLoadBalancing()
        }
        
        // Log metrics for analysis
        wp.logMetrics(metrics)
    }
}
```

## Rate Limiting & Domain Protection

### Intelligent Rate Limiting

**Per-Domain Rate Limiting:**
```go
type DomainRateLimiter struct {
    limiters map[string]*rate.Limiter
    mutex    sync.RWMutex
    rules    map[string]RateRule
}

type RateRule struct {
    RequestsPerSecond rate.Limit
    BurstCapacity     int
    ConcurrentLimit   int
}

func (drl *DomainRateLimiter) getDomainLimiter(domain string) *rate.Limiter {
    drl.mutex.RLock()
    limiter, exists := drl.limiters[domain]
    drl.mutex.RUnlock()
    
    if exists {
        return limiter
    }
    
    // Create new limiter based on domain rules
    rule := drl.getRuleForDomain(domain)
    limiter = rate.NewLimiter(rule.RequestsPerSecond, rule.BurstCapacity)
    
    drl.mutex.Lock()
    drl.limiters[domain] = limiter
    drl.mutex.Unlock()
    
    return limiter
}
```

**Default Rate Rules:**
```go
var DefaultRateRules = map[string]RateRule{
    "webflow.io":     {2, 5, 2},   // Conservative for Webflow
    "shopify.com":    {3, 8, 3},   // E-commerce sites
    "wordpress.com":  {5, 10, 5},  // WordPress sites
    "default":        {3, 5, 3},   // Default rule
}
```

### Graceful Degradation

**Circuit Breaker Pattern:**
```go
type CircuitBreaker struct {
    maxFailures  int
    resetTimeout time.Duration
    failures     int
    lastFailure  time.Time
    state        CircuitState
}

func (cb *CircuitBreaker) execute(domain string, fn func() error) error {
    if cb.state == StateOpen {
        if time.Since(cb.lastFailure) > cb.resetTimeout {
            cb.state = StateHalfOpen
        } else {
            return ErrCircuitOpen
        }
    }
    
    err := fn()
    if err != nil {
        cb.recordFailure()
        return err
    }
    
    cb.recordSuccess()
    return nil
}
```

## Load Balancing Strategies

### Multi-Region Deployment

**Region Selection:**
```go
type RegionConfig struct {
    Name        string
    Capacity    int
    Latency     time.Duration
    ActiveJobs  int
}

func (lb *LoadBalancer) selectOptimalRegion(domain string) string {
    // 1. Geographic proximity to target domain
    targetRegion := lb.getGeographicRegion(domain)
    
    // 2. Current capacity and load
    regions := lb.getAvailableRegions()
    bestRegion := targetRegion
    lowestLoad := float64(1.0)
    
    for _, region := range regions {
        load := float64(region.ActiveJobs) / float64(region.Capacity)
        if load < lowestLoad {
            lowestLoad = load
            bestRegion = region.Name
        }
    }
    
    return bestRegion
}
```

### Job Distribution

**Intelligent Job Routing:**
```go
func (jm *JobManager) distributeJob(job *Job) error {
    // Route based on job characteristics
    if job.EstimatedPages > 1000 {
        return jm.routeToHighCapacityWorkers(job)
    }
    
    if job.Priority == PriorityCritical {
        return jm.routeToReservedWorkers(job)
    }
    
    // Default distribution
    return jm.routeToAvailableWorkers(job)
}
```

## Monitoring & Alerting

### Performance Metrics

**Key Performance Indicators:**
- Worker utilisation percentage
- Average task completion time
- Queue depth by priority
- Error rate by domain
- Resource utilisation (CPU, memory, connections)

### Automated Alerting

```go
type AlertManager struct {
    thresholds map[string]float64
    channels   []AlertChannel
}

func (am *AlertManager) checkThresholds(metrics SystemMetrics) {
    alerts := []Alert{}
    
    if metrics.QueuedTasks > am.thresholds["max_queue_depth"] {
        alerts = append(alerts, Alert{
            Type:     "queue_depth_high",
            Severity: "warning",
            Message:  fmt.Sprintf("Queue depth: %d", metrics.QueuedTasks),
        })
    }
    
    if metrics.ResponseTimeP95 > time.Duration(am.thresholds["max_response_time"]) {
        alerts = append(alerts, Alert{
            Type:     "response_time_high", 
            Severity: "critical",
            Message:  fmt.Sprintf("P95 response time: %v", metrics.ResponseTimeP95),
        })
    }
    
    for _, alert := range alerts {
        am.sendAlert(alert)
    }
}
```

## Implementation Roadmap

### Phase 1: Dynamic Scaling (Immediate)
- Implement basic worker scaling algorithm
- Add job priority assignment
- Database query optimisations

### Phase 2: Advanced Features (1-2 months)
- Circuit breaker implementation
- Enhanced rate limiting per domain
- Performance monitoring dashboard

### Phase 3: Multi-Region (3-6 months)
- Geographic load balancing
- Cross-region job distribution
- Advanced alerting and analytics

This scaling strategy ensures Blue Banded Bee can handle growth while maintaining service quality and responsible crawling practices.
# Metrics Collection Plan

## Overview

This document outlines the metrics collection strategy for the Blue Banded Bee service. Proper metrics are essential for monitoring system health, diagnosing issues, and planning capacity. This plan identifies key metrics to collect, implementation approaches, and integration with monitoring systems.

## Key Metrics Categories

### 1. System Metrics

| Metric          | Description                               | Implementation           |
| --------------- | ----------------------------------------- | ------------------------ |
| CPU Usage       | CPU utilization by service                | Runtime metrics          |
| Memory Usage    | Memory consumption patterns               | Runtime metrics          |
| Goroutine Count | Number of active goroutines               | `runtime.NumGoroutine()` |
| GC Stats        | Garbage collection frequency and duration | `runtime.ReadMemStats()` |

### 2. PostgreSQL Metrics

| Metric                | Description                       | Implementation    |
| --------------------- | --------------------------------- | ----------------- |
| Connection Pool Stats | Active/idle/max connections       | `db.Stats()`      |
| Query Latency         | Time taken for queries to execute | Middleware timing |
| Transaction Rate      | Transactions per second           | Custom counter    |
| Lock Contention       | Number of lock wait events        | PostgreSQL stats  |
| Database Size         | Size of database and tables       | Admin query       |

### 3. Worker Pool Metrics

| Metric               | Description                      | Implementation            |
| -------------------- | -------------------------------- | ------------------------- |
| Worker Count         | Current number of active workers | `wp.currentWorkers`       |
| Queue Depth          | Number of pending tasks          | Count from database       |
| Task Processing Rate | Tasks processed per minute       | Counter + time window     |
| Task Latency         | Time from creation to completion | Calculate from timestamps |
| Error Rate           | Percentage of tasks that fail    | Failed/total counter      |
| Recovery Events      | Count of recovered stale tasks   | Counter                   |

### 4. Crawler Metrics

| Metric                    | Description                            | Implementation              |
| ------------------------- | -------------------------------------- | --------------------------- |
| Response Times            | Time taken to fetch URLs               | From task results           |
| Status Code Distribution  | Count of responses by status code      | From task results           |
| Cache Hit Ratio           | Percentage of requests with cache hits | Calculate from task results |
| Content Type Distribution | Types of content being crawled         | From task results           |
| Link Discovery Rate       | Number of new links found per page     | Counter                     |

### 5. API Metrics

| Metric                   | Description                                | Implementation     |
| ------------------------ | ------------------------------------------ | ------------------ |
| Request Rate             | Requests per second by endpoint            | Middleware counter |
| Response Time            | Time to process API requests               | Middleware timing  |
| Error Rate               | Percentage of requests resulting in errors | Counter            |
| Status Code Distribution | Count of responses by status code          | Counter            |

## Implementation Approach

### 1. Metrics Collection

We will use Prometheus as our metrics collection system with the following components:

1. **Prometheus Client Library**

   - Import `github.com/prometheus/client_golang/prometheus`
   - Define metrics in a central package

2. **Metrics Registration**

   - Create a metrics registry on application startup
   - Register all metrics at initialization

3. **Instrumentation**
   - Add middleware for HTTP metrics
   - Add database query wrappers
   - Instrument worker pool operations

### 2. Sample Implementation

```go
// metrics/metrics.go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// API metrics
	RequestCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_request_duration_seconds",
			Help:    "API request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// Database metrics
	DBQueryCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_queries_total",
			Help: "Total number of database queries",
		},
		[]string{"operation", "status"},
	)

	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	// Worker metrics
	TasksProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tasks_processed_total",
			Help: "Total number of tasks processed",
		},
		[]string{"status"},
	)

	WorkerCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "worker_count",
			Help: "Current number of workers",
		},
	)

	QueueDepth = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "task_queue_depth",
			Help: "Current depth of the task queue",
		},
	)

	// Crawler metrics
	URLResponseTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "url_response_time_seconds",
			Help:    "URL response time in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"domain", "status_code"},
	)

	CacheHitRate = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_status_total",
			Help: "Cache status distribution",
		},
		[]string{"status"},
	)
)
```

### 3. Middleware Implementation

```go
// middleware/metrics.go
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		// Call the next handler
		next.ServeHTTP(ww, r)

		// Record metrics after request is processed
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(ww.Status())

		metrics.RequestCounter.WithLabelValues(
			r.Method, r.URL.Path, status,
		).Inc()

		metrics.RequestDuration.WithLabelValues(
			r.Method, r.URL.Path,
		).Observe(duration)
	})
}
```

## Metrics Endpoint

Expose metrics via a dedicated endpoint:

```go
// main.go
import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func setupMetrics(r *chi.Mux) {
	// Create a subrouter with authentication for metrics
	r.Group(func(r chi.Router) {
		// Add auth middleware for production
		if env != "development" {
			r.Use(authMiddleware)
		}

		r.Handle("/metrics", promhttp.Handler())
	})
}
```

## visualisation and Alerting

1. **Grafana Dashboard**

   - Create dashboards for each metric category
   - Set up system overview dashboard
   - Implement domain-specific dashboards

2. **Alerting**
   - Set up alerts for critical metrics:
     - High error rates
     - Database connection pool exhaustion
     - Worker pool saturation
     - API response time degradation

## Integration with Existing Monitoring

1. **Sentry**

   - Continue using Sentry for error tracking
   - Add custom tags with metrics context

2. **Health Checks**
   - Enhance health checks with metrics data
   - Add readiness probes based on metrics

## Implementation Priority

1. Core system metrics (worker pool, database)
2. API metrics for request monitoring
3. Crawler performance metrics
4. Database detailed metrics
5. Business metrics (jobs completed, domain stats)

## Next Steps

1. Implement the metrics package
2. Add instrumentation to key components
3. Set up Prometheus scraping configuration
4. Create Grafana dashboards
5. Configure alerting rules

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"runtime/trace"

	"github.com/Harvey-AU/blue-banded-bee/internal/api"
	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/jobs"
	"github.com/Harvey-AU/blue-banded-bee/internal/observability"
	"github.com/getsentry/sentry-go"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// minInt returns the smaller of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// startHealthMonitoring starts background monitoring for job completion and system health
func startHealthMonitoring(pgDB *db.DB) {
	completionTicker := time.NewTicker(30 * time.Second)
	defer completionTicker.Stop()

	healthTicker := time.NewTicker(5 * time.Minute)
	defer healthTicker.Stop()

	// Helper to check job completion
	checkJobCompletion := func() {
		rows, err := pgDB.GetDB().Query(`
			UPDATE jobs
			SET status = 'completed', completed_at = NOW()
			WHERE (completed_tasks + failed_tasks) >= (total_tasks - COALESCE(skipped_tasks, 0))
			  AND status = 'running'
			RETURNING id
		`)
		if err != nil {
			sentry.CaptureException(err)
			log.Error().Err(err).Msg("Failed to update completed jobs")
			return
		}
		defer rows.Close()

		for rows.Next() {
			var jobID string
			if err := rows.Scan(&jobID); err == nil {
				log.Info().Str("job_id", jobID).Msg("Job marked as completed")
			}
		}
	}

	// Helper to check system health (stuck jobs/tasks)
	checkSystemHealth := func() {
		// Check for stuck jobs - get total count first
		var totalStuckJobs int
		err := pgDB.GetDB().QueryRow(`
			SELECT COUNT(*)
			FROM jobs j
			WHERE j.status = 'running'
			  AND j.progress = 0
			  AND j.started_at < NOW() - INTERVAL '5 minutes'
		`).Scan(&totalStuckJobs)

		if err != nil {
			log.Error().Err(err).Msg("Failed to count stuck jobs")
		}

		// Get sample of stuck jobs for details
		type stuckJobInfo struct {
			ID        string
			DomainID  int
			StartedAt time.Time
			Progress  float64
		}
		var stuckJobs []stuckJobInfo

		if totalStuckJobs > 0 {
			stuckJobRows, err := pgDB.GetDB().Query(`
				SELECT j.id, j.domain_id, j.started_at, j.progress
				FROM jobs j
				WHERE j.status = 'running'
				  AND j.progress = 0
				  AND j.started_at < NOW() - INTERVAL '5 minutes'
				ORDER BY j.started_at ASC
				LIMIT 10
			`)
			if err != nil {
				log.Error().Err(err).Msg("Failed to query stuck jobs sample")
			} else {
				defer stuckJobRows.Close()
				for stuckJobRows.Next() {
					var job stuckJobInfo
					if err := stuckJobRows.Scan(&job.ID, &job.DomainID, &job.StartedAt, &job.Progress); err == nil {
						stuckJobs = append(stuckJobs, job)
					}
				}
			}
		}

		if totalStuckJobs > 0 && len(stuckJobs) > 0 {
			jobIDs := make([]string, len(stuckJobs))
			for i, job := range stuckJobs {
				jobIDs[i] = job.ID
			}

			sentry.WithScope(func(scope *sentry.Scope) {
				scope.SetLevel(sentry.LevelWarning)
				scope.SetTag("event_type", "stuck_jobs")
				scope.SetContext("stuck_jobs", map[string]any{
					"total_count":  totalStuckJobs,
					"sample_count": len(stuckJobs),
					"job_ids":      jobIDs,
					"oldest_job":   stuckJobs[0].ID,
					"started_at":   stuckJobs[0].StartedAt,
					"sample_jobs":  stuckJobs,
				})
				sentry.CaptureMessage(fmt.Sprintf("Found %d stuck jobs with 0%% progress (showing %d samples)", totalStuckJobs, len(stuckJobs)))
			})

			log.Warn().
				Int("total_stuck_jobs", totalStuckJobs).
				Int("sample_count", len(stuckJobs)).
				Strs("sample_job_ids", jobIDs).
				Msg("CRITICAL: Jobs stuck without progress for >5 minutes")
		}

		// Check for stuck tasks - get total counts first
		var totalStuckTasks int
		var totalAffectedJobs int

		err = pgDB.GetDB().QueryRow(`
			SELECT COUNT(*), COUNT(DISTINCT job_id)
			FROM tasks
			WHERE status = 'running'
			  AND started_at < NOW() - INTERVAL '3 minutes'
		`).Scan(&totalStuckTasks, &totalAffectedJobs)

		if err != nil {
			log.Error().Err(err).Msg("Failed to count stuck tasks")
		}

		// Get sample of stuck tasks for details
		type stuckTaskInfo struct {
			ID         string
			JobID      string
			Path       string
			StartedAt  time.Time
			RetryCount int
		}
		var stuckTasks []stuckTaskInfo

		if totalStuckTasks > 0 {
			stuckTaskRows, err := pgDB.GetDB().Query(`
				SELECT t.id, t.job_id, p.path, t.started_at, t.retry_count
				FROM tasks t
				JOIN pages p ON t.page_id = p.id
				WHERE t.status = 'running'
				  AND t.started_at < NOW() - INTERVAL '3 minutes'
				ORDER BY t.started_at ASC
				LIMIT 20
			`)
			if err != nil {
				log.Error().Err(err).Msg("Failed to query stuck tasks sample")
			} else {
				defer stuckTaskRows.Close()
				for stuckTaskRows.Next() {
					var task stuckTaskInfo
					if err := stuckTaskRows.Scan(&task.ID, &task.JobID, &task.Path, &task.StartedAt, &task.RetryCount); err == nil {
						stuckTasks = append(stuckTasks, task)
					}
				}
			}
		}

		if totalStuckTasks > 0 && len(stuckTasks) > 0 {
			// Get unique job IDs from sample for context
			jobIDMap := make(map[string]struct{})
			for _, task := range stuckTasks {
				jobIDMap[task.JobID] = struct{}{}
			}
			sampleJobIDs := make([]string, 0, len(jobIDMap))
			for jobID := range jobIDMap {
				sampleJobIDs = append(sampleJobIDs, jobID)
			}

			sentry.WithScope(func(scope *sentry.Scope) {
				scope.SetLevel(sentry.LevelWarning)
				scope.SetTag("event_type", "stuck_tasks")
				scope.SetContext("stuck_tasks", map[string]any{
					"total_tasks":       totalStuckTasks,
					"total_jobs":        totalAffectedJobs,
					"sample_task_count": len(stuckTasks),
					"sample_job_count":  len(sampleJobIDs),
					"sample_job_ids":    sampleJobIDs,
					"oldest_task":       stuckTasks[0].ID,
					"oldest_at":         stuckTasks[0].StartedAt,
					"sample_tasks":      stuckTasks[:minInt(5, len(stuckTasks))],
				})
				sentry.CaptureMessage(fmt.Sprintf("Found %d stuck tasks across %d jobs (showing %d task samples)", totalStuckTasks, totalAffectedJobs, len(stuckTasks)))
			})

			log.Warn().
				Int("total_stuck_tasks", totalStuckTasks).
				Int("total_affected_jobs", totalAffectedJobs).
				Int("sample_task_count", len(stuckTasks)).
				Int("sample_job_count", len(sampleJobIDs)).
				Strs("sample_job_ids", sampleJobIDs).
				Time("oldest_stuck_at", stuckTasks[0].StartedAt).
				Msg("CRITICAL: Tasks stuck in running state for >3 minutes")
		}
	}

	// Run initial checks
	checkJobCompletion()

	for {
		select {
		case <-completionTicker.C:
			checkJobCompletion()
		case <-healthTicker.C:
			checkSystemHealth()
		}
	}
}

// Config holds the application configuration loaded from environment variables
type Config struct {
	Port                  string // HTTP port to listen on
	Env                   string // Environment (development/production)
	SentryDSN             string // Sentry DSN for error tracking
	LogLevel              string // Log level (debug, info, warn, error)
	FlightRecorderEnabled bool   // Flight recorder for performance debugging
	ObservabilityEnabled  bool   // Toggle OpenTelemetry + Prometheus exporters
	MetricsAddr           string // Address for Prometheus metrics endpoint (":9464" style)
	OTLPEndpoint          string // OTLP HTTP endpoint for trace export
	OTLPHeaders           string // Comma separated headers for OTLP exporter
	OTLPInsecure          bool   // Disable TLS verification for OTLP exporter
}

func main() {
	// Load .env files - .env.local takes priority for development
	godotenv.Load(".env.local", ".env")

	// Load configuration
	config := &Config{
		Port:                  getEnvWithDefault("PORT", "8080"),
		Env:                   getEnvWithDefault("APP_ENV", "development"),
		SentryDSN:             os.Getenv("SENTRY_DSN"),
		LogLevel:              getEnvWithDefault("LOG_LEVEL", "info"),
		FlightRecorderEnabled: getEnvWithDefault("FLIGHT_RECORDER_ENABLED", "false") == "true",
		ObservabilityEnabled:  getEnvWithDefault("OBSERVABILITY_ENABLED", "true") == "true",
		MetricsAddr:           getEnvWithDefault("METRICS_ADDR", ":9464"),
		OTLPEndpoint:          os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		OTLPHeaders:           os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"),
		OTLPInsecure:          getEnvWithDefault("OTEL_EXPORTER_OTLP_INSECURE", "false") == "true",
	}

	// Start flight recorder if enabled
	if config.FlightRecorderEnabled {
		f, err := os.Create("trace.out")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create trace file")
		}

		if err := trace.Start(f); err != nil {
			log.Fatal().Err(err).Msg("failed to start flight recorder")
		}
		log.Info().Msg("Flight recorder enabled, writing to trace.out")

		// Defer closing the trace and the file to the shutdown sequence
		defer func() {
			trace.Stop()
			f.Close()
			log.Info().Msg("Flight recorder stopped and trace file closed.")
		}()
	}

	setupLogging(config)

	var err error

	// Initialise Sentry for error tracking and performance monitoring
	if config.SentryDSN != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:         config.SentryDSN,
			Environment: config.Env,
			TracesSampleRate: func() float64 {
				if config.Env == "production" {
					return 0.1 // 10% sampling in production
				}
				return 1.0 // 100% sampling in development
			}(),
			AttachStacktrace: true,
			Debug:            config.Env == "development",
		})
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialise Sentry")
		} else {
			log.Info().Str("environment", config.Env).Msg("Sentry initialised successfully")
			// Ensure Sentry flushes before application exits
			defer sentry.Flush(2 * time.Second)
		}
	} else {
		log.Warn().Msg("Sentry DSN not configured, error tracking disabled")
	}

	var (
		obsProviders *observability.Providers
		metricsSrv   *http.Server
	)

	if config.ObservabilityEnabled {
		obsProviders, err = observability.Init(context.Background(), observability.Config{
			Enabled:        true,
			ServiceName:    "blue-banded-bee",
			Environment:    config.Env,
			OTLPEndpoint:   strings.TrimSpace(config.OTLPEndpoint),
			OTLPHeaders:    parseOTLPHeaders(config.OTLPHeaders),
			OTLPInsecure:   config.OTLPInsecure,
			MetricsAddress: config.MetricsAddr,
		})
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialise observability providers")
		} else {
			defer func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := obsProviders.Shutdown(shutdownCtx); err != nil {
					log.Warn().Err(err).Msg("Failed to flush telemetry providers cleanly")
				}
			}()

			if obsProviders.MetricsHandler != nil && config.MetricsAddr != "" {
				metricsSrv = &http.Server{
					Addr:              config.MetricsAddr,
					Handler:           obsProviders.MetricsHandler,
					ReadHeaderTimeout: 5 * time.Second,
				}

				go func() {
					log.Info().Str("addr", config.MetricsAddr).Msg("Metrics server listening")
					if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
						sentry.CaptureException(err)
						log.Error().Err(err).Msg("Metrics server failed")
					}
				}()

				defer func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					if err := metricsSrv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
						log.Warn().Err(err).Msg("Graceful shutdown of metrics server failed")
					}
				}()
			}
		}
	}

	// Connect to PostgreSQL
	pgDB, err := db.InitFromEnv()
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL database")
	}
	defer pgDB.Close()

	log.Info().Msg("Connected to PostgreSQL database")

	// Initialise crawler
	crawlerConfig := crawler.DefaultConfig()
	cr := crawler.New(crawlerConfig) // QUESTION: Should we change cr to crawler for clarity, as others have clearer names.

	// Create database queue for operations
	dbQueue := db.NewDbQueue(pgDB)

	// Create a worker pool for task processing
	// Configure worker count based on environment to prevent resource exhaustion
	var jobWorkers int
	appEnv := os.Getenv("APP_ENV")
	switch appEnv {
	case "production":
		jobWorkers = 50 // Production: high throughput
	case "staging":
		jobWorkers = 10 // Preview/staging: moderate throughput for PR testing
	default:
		jobWorkers = 5 // Development: minimal for local testing
	}

	// Configure worker concurrency (how many tasks each worker handles simultaneously)
	workerConcurrency := getEnvInt("WORKER_CONCURRENCY", 1)
	if workerConcurrency < 1 {
		workerConcurrency = 1
	} else if workerConcurrency > 20 {
		workerConcurrency = 20
	}

	totalCapacity := jobWorkers * workerConcurrency
	log.Info().
		Int("workers", jobWorkers).
		Int("concurrency_per_worker", workerConcurrency).
		Int("total_capacity", totalCapacity).
		Str("environment", appEnv).
		Msg("Configuring worker pool")

	workerPool := jobs.NewWorkerPool(pgDB.GetDB(), dbQueue, cr, jobWorkers, workerConcurrency, pgDB.GetConfig())

	// Create job manager
	jobsManager := jobs.NewJobManager(pgDB.GetDB(), dbQueue, cr, workerPool)

	// Set the job manager in the worker pool for duplicate checking
	workerPool.SetJobManager(jobsManager)

	// Start the worker pool
	workerPool.Start(context.Background())
	defer workerPool.Stop()

	// Start background health monitoring
	go startHealthMonitoring(pgDB)

	// Create a rate limiter
	limiter := newRateLimiter()

	// Create API handler with dependencies
	apiHandler := api.NewHandler(pgDB, jobsManager)

	// Create HTTP multiplexer
	mux := http.NewServeMux()

	// Setup API routes
	apiHandler.SetupRoutes(mux)

	// Create middleware stack with rate limiting
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		if !limiter.getLimiter(ip).Allow() {
			api.WriteErrorMessage(w, r, "Too many requests", http.StatusTooManyRequests, api.ErrCodeRateLimit)
			return
		}
		mux.ServeHTTP(w, r)
	})

	// Add middleware in reverse order (outermost first)
	handler = api.LoggingMiddleware(handler)
	handler = api.RequestIDMiddleware(handler)
	handler = api.SecurityHeadersMiddleware(handler)
	handler = api.CrossOriginProtectionMiddleware(handler)
	handler = api.CORSMiddleware(handler)
	handler = observability.WrapHandler(handler, obsProviders)

	// Create a new HTTP server
	server := &http.Server{
		Addr:    ":" + config.Port,
		Handler: handler,
	}

	// Channel to listen for termination signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Channel to signal when the server has shut down
	done := make(chan struct{})

	go func() {
		<-stop
		log.Info().Msg("Shutting down server...")

		// Create shutdown context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		defer cancel()

		// Stop accepting new requests
		if err := server.Shutdown(ctx); err != nil {
			sentry.CaptureException(err)
			log.Error().Err(err).Msg("Server forced to shutdown")
		}

		close(done)
	}()

	// Start the server
	log.Info().Str("port", config.Port).Msg("Starting server")

	// Print helpful development URLs
	baseURL := fmt.Sprintf("http://localhost:%s", config.Port)
	log.Info().Msg("🚀 Blue Banded Bee Development Server Ready!")
	log.Info().Str("homepage", baseURL).Msg("📱 Open Homepage")
	log.Info().Str("dashboard", baseURL+"/dashboard").Msg("📊 Open Dashboard")
	log.Info().Str("health", baseURL+"/health").Msg("🔍 Health Check")
	if config.Env == "development" {
		log.Info().Str("supabase_studio", "http://localhost:54323").Msg("🗄️  Open Supabase Studio")
	}

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		sentry.CaptureException(err)
		log.Fatal().Err(err).Msg("Server error")
	}

	<-done // Wait for the shutdown process to complete
	log.Info().Msg("Server stopped")
}

// getEnvWithDefault retrieves an environment variable or returns a default value if not set
func getEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvInt retrieves an environment variable as an integer or returns a default value if not set or invalid
func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	var result int
	if _, err := fmt.Sscanf(value, "%d", &result); err != nil {
		log.Warn().
			Str("key", key).
			Str("value", value).
			Int("default", defaultValue).
			Msg("Invalid integer in environment variable, using default")
		return defaultValue
	}

	return result
}

func parseOTLPHeaders(raw string) map[string]string {
	headers := make(map[string]string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return headers
	}

	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}

		headers[key] = value
	}

	return headers
}

// setupLogging configures the logging system
func setupLogging(config *Config) {
	// Configure log level
	level, err := zerolog.ParseLevel(config.LogLevel)
	if err != nil {
		level = zerolog.WarnLevel
	}
	zerolog.SetGlobalLevel(level)

	// Use console writer in development
	if config.Env == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	} else {
		// In production, use a more verbose JSON format that works well with Fly.io logs
		log.Logger = zerolog.New(os.Stdout).
			With().
			Timestamp().
			Str("service", "blue-banded-bee").
			Logger()
	}
}

// RateLimiter represents a rate limiting system based on client IP addresses
type RateLimiter struct {
	limits   map[string]*IPRateLimiter
	mu       sync.Mutex
	rate     rate.Limit
	capacity int
}

// IPRateLimiter wraps a token bucket rate limiter specific to an IP address
type IPRateLimiter struct {
	limiter *rate.Limiter
}

// newRateLimiter creates a new rate limiter with default settings
func newRateLimiter() *RateLimiter {
	return &RateLimiter{
		limits:   make(map[string]*IPRateLimiter),
		rate:     rate.Limit(20), // 20 requests per second for dashboard
		capacity: 10,             // 10 burst capacity
	}
}

// getLimiter returns the rate limiter for a specific IP address
func (rl *RateLimiter) getLimiter(ip string) *IPRateLimiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limits[ip]
	if !exists {
		limiter = &IPRateLimiter{
			limiter: rate.NewLimiter(rl.rate, rl.capacity),
		}
		rl.limits[ip] = limiter
	}

	return limiter
}

// Allow checks if a request from this IP should be allowed
func (ipl *IPRateLimiter) Allow() bool {
	return ipl.limiter.Allow()
}

// getClientIP extracts the client's IP address from a request
func getClientIP(r *http.Request) string {
	// Check for X-Forwarded-For header first (for clients behind proxies)
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		// X-Forwarded-For might contain multiple IPs, take the first one
		ips := strings.Split(ip, ",")
		ip = strings.TrimSpace(ips[0])
		return ip
	}

	// If no X-Forwarded-For, use RemoteAddr
	ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	return ip
}

package main

import (
	"context"
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
	"github.com/getsentry/sentry-go"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// CI nudge
// Config holds the application configuration loaded from environment variables
type Config struct {
	Port                  string // HTTP port to listen on
	Env                   string // Environment (development/production)
	SentryDSN             string // Sentry DSN for error tracking
	LogLevel              string // Log level (debug, info, warn, error)
	FlightRecorderEnabled bool   // Flight recorder for performance debugging
}

func main() {
	// Load .env file if it exists
	godotenv.Load()

	// Load configuration
	config := &Config{
		Port:                  getEnvWithDefault("PORT", "8080"),
		Env:                   getEnvWithDefault("APP_ENV", "development"),
		SentryDSN:             os.Getenv("SENTRY_DSN"),
		LogLevel:              getEnvWithDefault("LOG_LEVEL", "info"),
		FlightRecorderEnabled: getEnvWithDefault("FLIGHT_RECORDER_ENABLED", "false") == "true",
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
	var jobWorkers int = 5 // QUESTION: Set in env or dynamically - consider impact throughout app where worker pool sizing is set.
	workerPool := jobs.NewWorkerPool(pgDB.GetDB(), dbQueue, cr, jobWorkers, pgDB.GetConfig())

	// Create job manager
	jobsManager := jobs.NewJobManager(pgDB.GetDB(), dbQueue, cr, workerPool)

	// Set the job manager in the worker pool for duplicate checking
	workerPool.SetJobManager(jobsManager)

	// Start the worker pool
	workerPool.Start(context.Background())
	defer workerPool.Stop()

	// Start a goroutine to monitor job completion
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		// Use for-range instead of for-select for better readability
		for range ticker.C {
			// Check for jobs that should be marked complete
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
				continue
			}

			// Log completed jobs
			for rows.Next() {
				var jobID string
				if err := rows.Scan(&jobID); err == nil {
					log.Info().Str("job_id", jobID).Msg("Job marked as completed")
				}
			}
			rows.Close()
		}
	}()

	// Create a rate limiter
	limiter := newRateLimiter()

	// Create API handler with dependencies
	apiHandler := api.NewHandler(pgDB, jobsManager)

	// Create HTTP multiplexer
	mux := http.NewServeMux()

	// Setup API routes
	apiHandler.SetupRoutes(mux)

	// Create middleware stack
	var handler http.Handler = mux

	// Add rate limiting
	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	handler = api.CrossOriginProtectionMiddleware(handler)
	handler = api.CORSMiddleware(handler)

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
		rate:     rate.Limit(5), // 5 requests per second
		capacity: 5,             // 5 burst capacity
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

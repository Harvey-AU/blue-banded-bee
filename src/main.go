package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/src/crawler"
	"github.com/Harvey-AU/blue-banded-bee/src/db"
	"github.com/Harvey-AU/blue-banded-bee/src/jobs"
	"github.com/getsentry/sentry-go"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// @title Blue Banded Bee API
// @version 1.0
// @description A web crawler service that warms caches and tracks response times
// @host blue-banded-bee.fly.dev
// @BasePath /

// Config holds the application configuration loaded from environment variables
type Config struct {
	Port        string // HTTP port to listen on
	Env         string // Environment (development/production)
	LogLevel    string // Logging level
	DatabaseURL string // Turso database URL
	AuthToken   string // Database authentication token
	SentryDSN   string // Sentry DSN for error tracking
}

func setupLogging(config *Config) {
	// Set up pretty console logging for development
	if config.Env == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	}

	// Set log level
	level, err := zerolog.ParseLevel(config.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
}

func loadConfig() (*Config, error) {
	// Load .env file if it exists
	godotenv.Load()

	config := &Config{
		Port:        getEnvWithDefault("PORT", "8080"),
		Env:         getEnvWithDefault("APP_ENV", "development"),
		LogLevel:    getEnvWithDefault("LOG_LEVEL", "info"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		AuthToken:   os.Getenv("DATABASE_AUTH_TOKEN"),
		SentryDSN:   os.Getenv("SENTRY_DSN"),
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// getEnvWithDefault retrieves an environment variable or returns a default value if not set
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// addSentryContext adds request context information to Sentry for error tracking
func addSentryContext(r *http.Request, name string) {
	if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
		hub.Scope().SetTag("endpoint", name)
		hub.Scope().SetTag("method", r.Method)
		hub.Scope().SetTag("user_agent", r.UserAgent())
	}
}

// statusRecorder wraps an http.ResponseWriter to capture the status code
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.size += size
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	return size, err
}

// wrapWithSentryTransaction wraps an HTTP handler with Sentry transaction monitoring
func wrapWithSentryTransaction(name string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		transaction := sentry.StartTransaction(r.Context(), name)
		defer transaction.Finish()

		rw := &statusRecorder{ResponseWriter: w}
		next(rw, r.WithContext(transaction.Context()))

		transaction.SetTag("http.method", r.Method)
		transaction.SetTag("http.url", r.URL.String())
		transaction.SetTag("http.status_code", fmt.Sprintf("%d", rw.statusCode))
		transaction.SetData("response_size", rw.size)
	}
}

// rateLimiter implements a per-IP rate limiting mechanism
type rateLimiter struct {
	visitors    map[string]*rate.Limiter
	lastSeen    map[string]time.Time // Track when each IP was last seen
	mu          sync.Mutex
	lastCleanup time.Time
}

// newRateLimiter creates a new rate limiter instance
func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		visitors:    make(map[string]*rate.Limiter),
		lastSeen:    make(map[string]time.Time),
		lastCleanup: time.Now(),
	}
}

// getLimiter returns a rate limiter for the given IP address
func (rl *rateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Check if we need to clean up old entries
	if time.Since(rl.lastCleanup) > 1*time.Hour {
		for ip, lastTime := range rl.lastSeen {
			if time.Since(lastTime) > 1*time.Hour {
				delete(rl.visitors, ip)
				delete(rl.lastSeen, ip)
			}
		}
		rl.lastCleanup = time.Now()
	}

	// Get or create limiter for this IP
	limiter, exists := rl.visitors[ip]
	if !exists {
		limiter = rate.NewLimiter(rate.Every(1*time.Second), 5) // 5 requests per second
		rl.visitors[ip] = limiter
	}

	// Update last seen time
	rl.lastSeen[ip] = time.Now()

	return limiter
}

// getClientIP extracts the real client IP address from a request
// It checks X-Forwarded-For and X-Real-IP headers before falling back to RemoteAddr
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (common in proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		// The leftmost IP is the original client
		return strings.TrimSpace(ips[0])
	}

	// Then X-Real-IP (used by Nginx and others)
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}

	// Fallback to RemoteAddr, but strip the port
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If we couldn't split, just use the whole thing
		return r.RemoteAddr
	}
	return ip
}

// middleware implements rate limiting for HTTP handlers
func (rl *rateLimiter) middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the real client IP
		ip := getClientIP(r)

		// Get or create a rate limiter for this IP
		limiter := rl.getLimiter(ip)

		// Check if the request exceeds the rate limit
		if !limiter.Allow() {
			log.Info().
				Str("ip", ip).
				Str("endpoint", r.URL.Path).
				Msg("Rate limit exceeded")

			// Track in Sentry
			if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
				hub.Scope().SetTag("rate_limited", "true")
				hub.Scope().SetTag("client_ip", ip)
				hub.CaptureMessage("Rate limit exceeded")
			}

			// Return 429 Too Many Requests
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}

// sanitizeURL removes sensitive information from URLs before logging
func sanitizeURL(urlStr string) string {
	if urlStr == "" {
		return ""
	}
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "invalid-url"
	}
	// Remove query parameters and userinfo
	parsed.RawQuery = ""
	parsed.User = nil
	return parsed.String()
}

// generateResetToken creates a secure token for development endpoints
func generateResetToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

var resetToken = generateResetToken()

// Metrics tracks various performance and usage metrics for the application
type Metrics struct {
	ResponseTimes []time.Duration // List of response times
	CacheHits     int             // Number of cache hits
	CacheMisses   int             // Number of cache misses
	ErrorCount    int             // Number of errors encountered
	RequestCount  int             // Total number of requests
	mu            sync.Mutex
}

// recordMetrics records various metrics about a request
func (m *Metrics) recordMetrics(duration time.Duration, cacheStatus string, hasError bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ResponseTimes = append(m.ResponseTimes, duration)
	m.RequestCount++

	if hasError {
		m.ErrorCount++
	}

	if cacheStatus == "HIT" {
		m.CacheHits++
	} else {
		m.CacheMisses++
	}
}

var metrics = &Metrics{}

func main() {
	// Load configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Setup logging
	setupLogging(config)

	// Initialize Sentry
	if config.SentryDSN != "" {
		err = sentry.Init(sentry.ClientOptions{
			Dsn:              config.SentryDSN,
			Environment:      config.Env,
			TracesSampleRate: 0.2,
			EnableTracing:    true,
			Debug:            config.Env == "development",
		})
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize Sentry")
		}
		defer sentry.Flush(2 * time.Second)
	} else {
		log.Warn().Msg("Sentry not initialized: SENTRY_DSN not provided")
	}

	// Initialize rate limiter
	limiter := newRateLimiter()

	// Initialize database
	dbConfig := &db.Config{
		URL:       config.DatabaseURL,
		AuthToken: config.AuthToken,
	}
	dbSetup, err := db.GetInstance(dbConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}

	// Set the DB instance for queue operations
	jobs.SetDBInstance(dbSetup)

	// Initialize jobs schema once at startup
	if err := jobs.InitSchema(dbSetup.GetDB()); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize jobs schema")
	}

	// Replace the existing job monitoring code with this:
	go func() {
		log.Info().Msg("Starting job monitor")

		// Create a global worker pool that persists for the life of the server
		c := crawler.New(crawler.DefaultConfig())
		workerPool := jobs.NewWorkerPool(dbSetup.GetDB(), c, 20)
		workerPool.Start(context.Background())

		// Start the task monitor
		workerPool.StartTaskMonitor(context.Background())

		m := jobs.NewJobManager(dbSetup.GetDB(), c, workerPool)

		// Get both pending AND running jobs
		rows, err := dbSetup.GetDB().Query("SELECT id FROM jobs WHERE status IN ('pending', 'running')")
		if err != nil {
			log.Error().Err(err).Msg("Query failed")
			return
		}

		// Process jobs
		for rows.Next() {
			var jobID string
			if err := rows.Scan(&jobID); err != nil {
				continue
			}

			if err := m.StartJob(context.Background(), jobID); err != nil {
				log.Error().Err(err).Str("job_id", jobID).Msg("Failed")
			} else {
				log.Info().Str("job_id", jobID).Msg("Started")
			}
		}
		rows.Close()
	}()

	// Health check handler - BEFORE THIS PART
	http.HandleFunc("/health", limiter.middleware(wrapWithSentryTransaction("health", func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("endpoint", "/health").Msg("Health check requested")
		w.Header().Set("Content-Type", "text/plain")
		const healthFormat = "OK - Deployed at: %s"
		response := fmt.Sprintf(healthFormat, time.Now().Format(time.RFC3339))
		fmt.Fprintln(w, response)
	})))

	// Test crawl handler
	// @Summary Test crawl endpoint
	// @Description Crawls a URL and returns the result
	// @Tags Crawler
	// @Param url query string false "URL to crawl (defaults to teamharvey.co)"
	// @Produce json
	// @Success 200 {object} crawler.CrawlResult
	// @Failure 400 {string} string "Invalid URL"
	// @Failure 500 {string} string "Internal server error"
	// @Router /test-crawl [get]
	http.HandleFunc("/test-crawl", limiter.middleware(wrapWithSentryTransaction("test-crawl", func(w http.ResponseWriter, r *http.Request) {
		span := sentry.StartSpan(r.Context(), "crawl.process")
		defer span.Finish()

		url := r.URL.Query().Get("url")
		if url == "" {
			url = "https://www.teamharvey.co"
		}

		// Add this line to get skip_cached parameter
		skipCached := r.URL.Query().Get("skip_cached") == "true"

		// Modify crawler initialization to include config
		crawlerConfig := crawler.DefaultConfig()
		crawlerConfig.SkipCachedURLs = skipCached

		crawler := crawler.New(crawlerConfig)

		sanitizedURL := sanitizeURL(url)
		span.SetTag("crawl.url", sanitizedURL)

		// Crawl span
		crawlSpan := sentry.StartSpan(r.Context(), "crawl.execute")
		result, err := crawler.WarmURL(r.Context(), url)
		if err != nil {
			crawlSpan.SetTag("error", "true")
			crawlSpan.SetTag("error.type", "url_parse_error")
			crawlSpan.SetData("error.message", err.Error())
			result.Error = err.Error()
			crawlSpan.Finish()
			sentry.CaptureException(err)
			log.Error().Err(err).Msg("Crawl failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		crawlSpan.SetTag("status_code", fmt.Sprintf("%d", result.StatusCode))
		crawlSpan.SetTag("cache_status", result.CacheStatus)
		crawlSpan.SetData("response_time_ms", result.ResponseTime)
		crawlSpan.Finish()

		// Store result span
		storeSpan := sentry.StartSpan(r.Context(), "db.store_result")
		crawlResult := &db.CrawlResult{
			URL:          result.URL,
			ResponseTime: result.ResponseTime,
			StatusCode:   result.StatusCode,
			Error:        result.Error,
			CacheStatus:  result.CacheStatus,
			JobID:        "", // Empty for test crawls
			TaskID:       "", // Empty for test crawls
		}

		if err := dbSetup.StoreCrawlResult(r.Context(), crawlResult); err != nil {
			storeSpan.SetTag("error", "true")
			storeSpan.Finish()
			log.Error().Err(err).Msg("Failed to store crawl result")
			http.Error(w, "Failed to store result", http.StatusInternalServerError)
			return
		}
		storeSpan.Finish()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})))

	// Recent crawls endpoint
	// @Summary Recent crawls endpoint
	// @Description Returns the 10 most recent crawl results
	// @Tags Crawler
	// @Produce json
	// @Success 200 {array} db.CrawlResult
	// @Failure 500 {string} string "Internal server error"
	// @Router /recent-crawls [get]
	http.HandleFunc("/recent-crawls", limiter.middleware(wrapWithSentryTransaction("recent-crawls", func(w http.ResponseWriter, r *http.Request) {
		span := sentry.StartSpan(r.Context(), "db.get_recent")
		defer span.Finish()

		results, err := dbSetup.GetRecentResults(r.Context(), 10)
		if err != nil {
			span.SetTag("error", "true")
			sentry.CaptureException(err)
			log.Error().Err(err).Msg("Failed to get recent results")
			http.Error(w, "Failed to get results", http.StatusInternalServerError)
			return
		}

		span.SetData("result_count", len(results))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	})))

	// Reset database endpoint (development only)
	if config.Env == "development" {
		devToken := generateResetToken()
		log.Info().Str("token", devToken).Msg("Development reset token generated")
		http.HandleFunc("/reset-db", limiter.middleware(wrapWithSentryTransaction("reset-db", func(w http.ResponseWriter, r *http.Request) {
			// Only allow POST method
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			// Verify token
			authHeader := r.Header.Get("Authorization")
			if authHeader != fmt.Sprintf("Bearer %s", devToken) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if err := dbSetup.ResetSchema(); err != nil {
				sentry.CaptureException(err)
				log.Error().Err(err).Msg("Failed to reset database schema")
				http.Error(w, "Failed to reset database", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"status": "Database schema reset successfully",
			})
		})))
	}

	// Sitemap scan endpoint
	// @Summary Scan a sitemap and add URLs to queue
	// @Description Discovers and parses sitemaps for a domain, adding URLs to the crawl queue
	// @Tags Jobs
	// @Param domain query string true "Domain to scan (without http/https)"
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Failure 400 {string} string "Invalid domain"
	// @Failure 500 {string} string "Internal server error"
	// @Router /scan-sitemap [get]
	http.HandleFunc("/scan-sitemap", limiter.middleware(wrapWithSentryTransaction("scan-sitemap", func(w http.ResponseWriter, r *http.Request) {
		span := sentry.StartSpan(r.Context(), "sitemap.scan")
		defer span.Finish()

		// Validate domain parameter
		domain := r.URL.Query().Get("domain")
		if domain == "" {
			http.Error(w, "Domain parameter is required", http.StatusBadRequest)
			return
		}

		// Add domain validation
		if strings.Contains(domain, "://") || strings.Contains(domain, "/") {
			http.Error(w, "Domain should not include protocol or path", http.StatusBadRequest)
			return
		}

		// Validate limit parameter if present
		limitStr := r.URL.Query().Get("limit")
		urlLimit := 0
		if limitStr != "" {
			parsed, err := strconv.Atoi(limitStr)
			if err != nil || parsed < 0 {
				http.Error(w, "Limit must be a positive number", http.StatusBadRequest)
				return
			}
			urlLimit = parsed
		}

		// Initialize crawler
		crawlerConfig := crawler.DefaultConfig()
		crawlerConfig.SkipCachedURLs = false
		crawler := crawler.New(crawlerConfig)

		// Create job options
		jobOptions := &jobs.JobOptions{
			Domain:      domain,
			UseSitemap:  true,
			FindLinks:   false,
			MaxDepth:    1,
			Concurrency: 1, // Minimal for sitemap scanning
		}

		job, err := jobs.CreateJob(dbSetup.GetDB(), jobOptions)
		if err != nil {
			span.SetTag("error", "true")
			sentry.CaptureException(err)
			log.Error().Err(err).Msg("Failed to create job record")
			http.Error(w, "Failed to create job", http.StatusInternalServerError)
			return
		}

		// Start sitemap processing in a separate goroutine
		go func() {
			// Create a new context since the request context will be canceled
			ctx := context.Background()
			hub := sentry.GetHubFromContext(r.Context())
			if hub != nil {
				ctx = sentry.SetHubOnContext(ctx, hub.Clone())
			}

			// Discover sitemaps
			baseURL := domain
			if !strings.HasPrefix(baseURL, "http") {
				baseURL = fmt.Sprintf("https://%s", domain)
			}

			sitemap, err := crawler.DiscoverSitemaps(ctx, domain)
			if err != nil {
				log.Error().Err(err).Str("domain", domain).Msg("Failed to discover sitemaps")
				return
			}

			var allURLs []string

			// Process each sitemap
			for _, sitemapURL := range sitemap {
				urls, err := crawler.ParseSitemap(ctx, sitemapURL)
				if err != nil {
					log.Error().Err(err).Str("sitemap", sitemapURL).Msg("Failed to parse sitemap")
					continue
				}

				allURLs = append(allURLs, urls...)
			}

			// Filter out duplicates
			uniqueURLs := make(map[string]bool)
			var filteredURLs []string

			for _, url := range allURLs {
				if !uniqueURLs[url] {
					uniqueURLs[url] = true
					filteredURLs = append(filteredURLs, url)

					// Add this check to limit URLs
					if urlLimit > 0 && len(filteredURLs) >= urlLimit {
						break
					}
				}
			}

			// Queue the URLs
			if err := jobs.EnqueueURLs(ctx, dbSetup.GetDB(), job.ID, filteredURLs, "sitemap", baseURL, 0); err != nil {
				log.Error().Err(err).Str("job_id", job.ID).Msg("Failed to enqueue URLs")

				// Update job with error status
				_, updateErr := dbSetup.ExecWithMetrics(ctx, `
					UPDATE jobs
					SET error_message = ?
					WHERE id = ?
				`, fmt.Sprintf("Failed to enqueue URLs: %v", err), job.ID)

				if updateErr != nil {
					log.Error().Err(updateErr).Msg("Failed to update job error status")
				}
				return
			}

			log.Info().
				Str("job_id", job.ID).
				Str("domain", domain).
				Int("url_count", len(filteredURLs)).
				Msg("Added URLs to job queue")
		}()

		// Return success response immediately
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "Sitemap scan started",
			"job_id": job.ID,
			"domain": domain,
		})
	})))

	// Start workers endpoint
	// @Summary Start workers to process the queue
	// @Description Starts a worker pool to process URLs in the queue
	// @Tags Jobs
	// @Param job_id query string true "Job ID to process"
	// @Param workers query int false "Number of workers (default 5)"
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Failure 400 {string} string "Invalid parameters"
	// @Failure 500 {string} string "Internal server error"
	// @Router /start-workers [get]
	http.HandleFunc("/start-workers", limiter.middleware(wrapWithSentryTransaction("start-workers", func(w http.ResponseWriter, r *http.Request) {
		span := sentry.StartSpan(r.Context(), "workers.start")
		defer span.Finish()

		// Get job ID
		jobID := r.URL.Query().Get("job_id")
		if jobID == "" {
			http.Error(w, "Job ID parameter is required", http.StatusBadRequest)
			return
		}

		// Get worker count
		workerCount := 20
		if countStr := r.URL.Query().Get("workers"); countStr != "" {
			if parsed, err := strconv.Atoi(countStr); err == nil && parsed > 0 {
				workerCount = parsed
			}
		}

		// Initialize crawler
		crawler := crawler.New(crawler.DefaultConfig())

		// Initialize worker pool
		globalWorkerPool := jobs.NewWorkerPool(dbSetup.GetDB(), crawler, workerCount)

		// Create a background context with Sentry tracing
		bgCtx := context.Background()
		hub := sentry.GetHubFromContext(r.Context())
		if hub != nil {
			bgCtx = sentry.SetHubOnContext(bgCtx, hub.Clone())
		}
		jobManager := jobs.NewJobManager(dbSetup.GetDB(), crawler, globalWorkerPool)

		// Start the job
		err = jobManager.StartJob(bgCtx, jobID)
		if err != nil {

			span.SetTag("error", "true")
			sentry.CaptureException(err)
			log.Error().Err(err).Str("job_id", jobID).Msg("Failed to start job")
			http.Error(w, fmt.Sprintf("Failed to start job: %s", err.Error()), http.StatusInternalServerError)
			return
		}

		// Return response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":       "Workers started",
			"job_id":       jobID,
			"worker_count": workerCount,
		})
	})))

	// Add graceful shutdown timeout
	const shutdownTimeout = 30 * time.Second

	// Create a new HTTP server
	server := &http.Server{
		Addr:    ":" + config.Port,
		Handler: nil, // Uses the default ServeMux
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

		// First stop accepting new requests
		if err := server.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Server forced to shutdown")
		}

		// Then stop the queue and wait for pending operations
		dbSetup.GetQueue().Stop()

		// Give pending operations a chance to complete
		select {
		case <-time.After(5 * time.Second):
			log.Warn().Msg("Shutdown timeout reached, some operations may be incomplete")
		case <-ctx.Done():
			log.Error().Msg("Context deadline exceeded during shutdown")
		}

		close(done)
	}()

	// Start the server
	log.Info().Msgf("Starting server on port %s", config.Port)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Server error")
	}

	<-done // Wait for the shutdown process to complete
	log.Info().Msg("Server stopped")
}

// Add this function to validate configuration
func validateConfig(config *Config) error {
	var errors []string

	// Check required values
	if config.DatabaseURL == "" {
		errors = append(errors, "DATABASE_URL is required")
	}

	if config.AuthToken == "" {
		errors = append(errors, "DATABASE_AUTH_TOKEN is required")
	}

	// Validate environment
	if config.Env != "development" && config.Env != "production" && config.Env != "staging" {
		errors = append(errors, fmt.Sprintf("APP_ENV must be one of [development, production, staging], got %s", config.Env))
	}

	// Validate port
	if _, err := strconv.Atoi(config.Port); err != nil {
		errors = append(errors, fmt.Sprintf("PORT must be a valid number, got %s", config.Port))
	}

	// Check log level
	_, err := zerolog.ParseLevel(config.LogLevel)
	if err != nil {
		errors = append(errors, fmt.Sprintf("LOG_LEVEL %s is invalid, using default: info", config.LogLevel))
	}

	// Warn about missing Sentry DSN
	if config.SentryDSN == "" {
		log.Warn().Msg("SENTRY_DSN is not set, error tracking will be disabled")
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed: %s", strings.Join(errors, "; "))
	}

	return nil
}

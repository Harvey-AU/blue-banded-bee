package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/src/crawler"
	"github.com/Harvey-AU/blue-banded-bee/src/db"
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

	return &Config{
		Port:        getEnvWithDefault("PORT", "8080"),
		Env:         getEnvWithDefault("APP_ENV", "development"),
		LogLevel:    getEnvWithDefault("LOG_LEVEL", "info"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		AuthToken:   os.Getenv("DATABASE_AUTH_TOKEN"),
		SentryDSN:   os.Getenv("SENTRY_DSN"),
	}, nil
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
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// wrapWithSentryTransaction wraps an HTTP handler with Sentry transaction monitoring
func wrapWithSentryTransaction(name string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		span := sentry.StartSpan(r.Context(), "http.server")
		defer span.Finish()

		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     0,
		}

		span.SetTag("endpoint", name)
		span.SetTag("http.method", r.Method)
		span.SetTag("http.url", r.URL.String())

		start := time.Now()
		next(recorder, r.WithContext(span.Context()))
		duration := time.Since(start)

		if recorder.statusCode != 0 {
			span.SetTag("http.status_code", fmt.Sprintf("%d", recorder.statusCode))
		}
		span.SetData("duration_ms", duration.Milliseconds())
		
		log.Info().
			Str("endpoint", name).
			Int("status", recorder.statusCode).
			Dur("duration", duration).
			Msg("Request processed")
	}
}

// rateLimiter implements a per-IP rate limiting mechanism
type rateLimiter struct {
	visitors map[string]*rate.Limiter
	mu       sync.Mutex
}

// newRateLimiter creates a new rate limiter instance
func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		visitors: make(map[string]*rate.Limiter),
	}
}

// getLimiter returns a rate limiter for the given IP address
func (rl *rateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.visitors[ip]
	if !exists {
		limiter = rate.NewLimiter(rate.Every(1*time.Second), 5) // 5 requests per second
		rl.visitors[ip] = limiter
	}

	return limiter
}

// middleware implements rate limiting for HTTP handlers
func (rl *rateLimiter) middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// sanitizeURL removes sensitive information from URLs before logging
func sanitizeURL(url string) string {
	if url == "" {
		return ""
	}
	parsed, err := url.Parse(url)
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
	ResponseTimes    []time.Duration // List of response times
	CacheHits       int             // Number of cache hits
	CacheMisses     int             // Number of cache misses
	ErrorCount      int             // Number of errors encountered
	RequestCount    int             // Total number of requests
	mu              sync.Mutex
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
	err = sentry.Init(sentry.ClientOptions{
		Dsn:             config.SentryDSN,
		Environment:     config.Env,
		TracesSampleRate: 1.0,
		Debug:           config.Env == "development",
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Sentry")
	}
	defer sentry.Flush(2 * time.Second)

	// Initialize rate limiter
	limiter := newRateLimiter()

	// Health check handler
	// @Summary Health check endpoint
	// @Description Returns the deployment time of the service
	// @Tags Health
	// @Produce plain
	// @Success 200 {string} string "OK - Deployed at: {timestamp}"
	// @Router /health [get]
	http.HandleFunc("/health", wrapWithSentryTransaction("health", func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("endpoint", "/health").Msg("Health check requested")
		w.Header().Set("Content-Type", "text/plain")
		const healthFormat = "OK - Deployed at: %s"
		response := fmt.Sprintf(healthFormat, time.Now().Format(time.RFC3339))
		fmt.Fprint(w, response)
	}))

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
		sanitizedURL := sanitizeURL(url)
		span.SetTag("crawl.url", sanitizedURL)

		// Database connection span
		dbSpan := sentry.StartSpan(r.Context(), "db.connect")
		dbConfig := &db.Config{
			URL:       config.DatabaseURL,
			AuthToken: config.AuthToken,
		}
		database, err := db.GetInstance(dbConfig)
		if err != nil {
			dbSpan.SetTag("error", "true")
			dbSpan.Finish()
			sentry.CaptureException(err)
			log.Error().Err(err).Msg("Failed to connect to database")
			http.Error(w, "Database connection failed", http.StatusInternalServerError)
			return
		}
		dbSpan.Finish()

		// Crawl span
		crawlSpan := sentry.StartSpan(r.Context(), "crawl.execute")
		crawler := crawler.New(nil)
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
		}

		if err := database.StoreCrawlResult(r.Context(), crawlResult); err != nil {
			storeSpan.SetTag("error", "true")
			storeSpan.Finish()
			log.Error().Err(err).Msg("Failed to store crawl result")
			http.Error(w, "Failed to store result", http.StatusInternalServerError)
			return
		}
		storeSpan.Finish()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))

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

		dbConfig := &db.Config{
			URL:       config.DatabaseURL,
			AuthToken: config.AuthToken,
		}

		database, err := db.GetInstance(dbConfig)
		if err != nil {
			span.SetTag("error", "true")
			sentry.CaptureException(err)
			log.Error().Err(err).Msg("Failed to connect to database")
			http.Error(w, "Database connection failed", http.StatusInternalServerError)
			return
		}

		results, err := database.GetRecentResults(r.Context(), 10)
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
	}))

	// Reset database endpoint (development only)
	if config.Env == "development" {
		devToken := generateResetToken()
		log.Info().Msg("Development reset token generated - check logs for value")
		http.HandleFunc("/reset-db", limiter.middleware(wrapWithSentryTransaction("reset-db", 
			func(w http.ResponseWriter, r *http.Request) {
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

				dbConfig := &db.Config{
					URL:       config.DatabaseURL,
					AuthToken: config.AuthToken,
				}

				database, err := db.GetInstance(dbConfig)
				if err != nil {
					log.Error().Err(err).Msg("Failed to connect to database")
					http.Error(w, "Database connection failed", http.StatusInternalServerError)
					return
				}

				if err := database.ResetSchema(); err != nil {
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

	// Start server
	log.Info().
		Str("port", config.Port).
		Str("env", config.Env).
		Msg("Starting server")

	if err := http.ListenAndServe(":"+config.Port, nil); err != nil {
		sentry.CaptureException(err)
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}

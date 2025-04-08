package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/src/crawler"
	"github.com/Harvey-AU/blue-banded-bee/src/db"
	"github.com/getsentry/sentry-go"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Config struct {
	Port        string
	Env         string
	LogLevel    string
	DatabaseURL string
	AuthToken   string
	SentryDSN   string
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

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func addSentryContext(r *http.Request, name string) {
	if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
		hub.Scope().SetTag("endpoint", name)
		hub.Scope().SetTag("method", r.Method)
		hub.Scope().SetTag("user_agent", r.UserAgent())
	}
}

func wrapWithSentryTransaction(name string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		span := sentry.StartSpan(r.Context(), name)
		defer span.Finish()
		r = r.WithContext(span.Context())
		next(w, r)
	}
}

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

	// Basic health check handler
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("endpoint", "/health").Msg("Health check requested")
		w.Header().Set("Content-Type", "text/plain")
		const healthFormat = "OK - Deployed at: %s"
		response := fmt.Sprintf(healthFormat, time.Now().Format(time.RFC3339))
		fmt.Fprint(w, response)
	})

	// Test crawl handler
	http.HandleFunc("/test-crawl", wrapWithSentryTransaction("test-crawl", func(w http.ResponseWriter, r *http.Request) {
		addSentryContext(r, "test-crawl")
		
		url := r.URL.Query().Get("url")
		if url == "" {
			url = "https://www.teamharvey.co" // default test URL
		}

		// Initialize database
		dbConfig := &db.Config{
			URL:       config.DatabaseURL,
			AuthToken: config.AuthToken,
		}

		database, err := db.New(dbConfig)
		if err != nil {
			sentry.CaptureException(err)
			log.Error().Err(err).Msg("Failed to connect to database")
			http.Error(w, "Database connection failed", http.StatusInternalServerError)
			return
		}
		defer database.Close()

		// Perform crawl
		crawler := crawler.New(nil)
		result, err := crawler.WarmURL(r.Context(), url)

		if err != nil {
			sentry.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetTag("url", url)
				scope.SetExtra("response_time", result.ResponseTime)
				scope.SetExtra("status_code", result.StatusCode)
				scope.SetExtra("cache_status", result.CacheStatus)
			})
			sentry.CaptureException(err)
			log.Error().Err(err).Msg("Crawl failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Store result in database
		crawlResult := &db.CrawlResult{
			URL:          result.URL,
			ResponseTime: result.ResponseTime,
			StatusCode:   result.StatusCode,
			Error:        result.Error,
			CacheStatus:  result.CacheStatus,
		}

		if err := database.StoreCrawlResult(r.Context(), crawlResult); err != nil {
			log.Error().Err(err).Msg("Failed to store crawl result")
			http.Error(w, "Failed to store result", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))

	// Add endpoint to get recent crawls
	http.HandleFunc("/recent-crawls", wrapWithSentryTransaction("recent-crawls", func(w http.ResponseWriter, r *http.Request) {
		addSentryContext(r, "recent-crawls")
		
		dbConfig := &db.Config{
			URL:       config.DatabaseURL,
			AuthToken: config.AuthToken,
		}

		database, err := db.New(dbConfig)
		if err != nil {
			sentry.CaptureException(err)
			log.Error().Err(err).Msg("Failed to connect to database")
			http.Error(w, "Database connection failed", http.StatusInternalServerError)
			return
		}
		defer database.Close()

		results, err := database.GetRecentResults(r.Context(), 10)
		if err != nil {
			sentry.CaptureException(err)
			log.Error().Err(err).Msg("Failed to get recent results")
			http.Error(w, "Failed to get results", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}))

	// Reset database endpoint (development only)
	http.HandleFunc("/reset-db", wrapWithSentryTransaction("reset-db", func(w http.ResponseWriter, r *http.Request) {
		addSentryContext(r, "reset-db")
		
		if config.Env != "development" {
			sentry.CaptureMessage("Attempted reset-db in production")
			http.Error(w, "Not allowed in production", http.StatusForbidden)
			return
		}

		dbConfig := &db.Config{
			URL:       config.DatabaseURL,
			AuthToken: config.AuthToken,
		}

		database, err := db.New(dbConfig)
		if err != nil {
			log.Error().Err(err).Msg("Failed to connect to database")
			http.Error(w, "Database connection failed", http.StatusInternalServerError)
			return
		}
		defer database.Close()

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
	}))

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

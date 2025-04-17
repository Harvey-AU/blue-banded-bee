package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/src/crawler"
	"github.com/Harvey-AU/blue-banded-bee/src/db/postgres"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config holds the application configuration loaded from environment variables
type Config struct {
	Port      string // HTTP port to listen on
	Env       string // Environment (development/production)
	LogLevel  string // Logging level
	SentryDSN string // Sentry DSN for error tracking
}

func main() {
	// Load .env file if it exists
	godotenv.Load()

	// Load configuration
	config := &Config{
		Port:      getEnvWithDefault("PORT", "8080"),
		Env:       getEnvWithDefault("APP_ENV", "development"),
		LogLevel:  getEnvWithDefault("LOG_LEVEL", "info"),
		SentryDSN: os.Getenv("SENTRY_DSN"),
	}

	// Setup logging
	setupLogging(config)

	// Connect to PostgreSQL
	pgDB, err := postgres.InitFromEnv()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL database")
	}
	defer pgDB.Close()

	log.Info().Msg("Connected to PostgreSQL database")

	// Initialize crawler
	crawlerConfig := crawler.DefaultConfig()
	cr := crawler.New(crawlerConfig)

	// HTTP endpoints
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "OK",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	http.HandleFunc("/pg-health", func(w http.ResponseWriter, r *http.Request) {
		if err := pgDB.GetDB().Ping(); err != nil {
			log.Error().Err(err).Msg("PostgreSQL health check failed")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "ERROR",
				"error":  err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "OK",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	http.HandleFunc("/recent-crawls", func(w http.ResponseWriter, r *http.Request) {
		limit := 10
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
				limit = parsed
			}
		}

		results, err := pgDB.GetRecentResults(r.Context(), limit)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get recent results")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get recent results",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	})

	http.HandleFunc("/test-crawl", func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Query().Get("url")
		if url == "" {
			url = "https://www.example.com"
		}

		result, err := cr.WarmURL(r.Context(), url)
		if err != nil {
			log.Error().Err(err).Str("url", url).Msg("Failed to crawl URL")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Store the result
		pgDB.StoreCrawlResult(r.Context(), &postgres.CrawlResult{
			URL:          result.URL,
			ResponseTime: result.ResponseTime,
			StatusCode:   result.StatusCode,
			Error:        result.Error,
			CacheStatus:  result.CacheStatus,
			ContentType:  result.ContentType,
			CreatedAt:    time.Now(),
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// Add a reset-db endpoint
	http.HandleFunc("/reset-db", func(w http.ResponseWriter, r *http.Request) {
		log.Warn().Msg("Database reset requested")

		if err := pgDB.ResetSchema(); err != nil {
			log.Error().Err(err).Msg("Failed to reset database schema")
			http.Error(w, "Failed to reset database", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "Database schema reset successfully",
		})
	})

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

		// Stop accepting new requests
		if err := server.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Server forced to shutdown")
		}

		close(done)
	}()

	// Start the server
	log.Info().Str("port", config.Port).Msg("Starting server")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Server error")
	}

	<-done // Wait for the shutdown process to complete
	log.Info().Msg("Server stopped")
}

// getEnvWithDefault retrieves an environment variable or returns a default value if not set
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// setupLogging configures the logger based on the environment
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

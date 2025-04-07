package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/teamharvey/cache-warmer/src/crawler"
	"github.com/teamharvey/cache-warmer/src/db"
)

type Config struct {
	Port        string
	Env         string
	LogLevel    string
	DatabaseURL string
	AuthToken   string
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
	}, nil
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	// Load configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Setup logging
	setupLogging(config)

	// Basic health check handler
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("endpoint", "/health").Msg("Health check requested")
		response := fmt.Sprintf("OK - Deployed at: %s", time.Now().Format(time.RFC3339))
		fmt.Fprintf(w, response)
	})

	// Test crawl handler
	http.HandleFunc("/test-crawl", func(w http.ResponseWriter, r *http.Request) {
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
			log.Error().Err(err).Msg("Failed to connect to database")
			http.Error(w, "Database connection failed", http.StatusInternalServerError)
			return
		}
		defer database.Close()

		// Perform crawl
		crawler := crawler.New(nil) // use default config
		result, err := crawler.WarmURL(r.Context(), url)

		if err != nil {
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
	})

	// Add endpoint to get recent crawls
	http.HandleFunc("/recent-crawls", func(w http.ResponseWriter, r *http.Request) {
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

		results, err := database.GetRecentResults(r.Context(), 10) // Get last 10 results
		if err != nil {
			log.Error().Err(err).Msg("Failed to get recent results")
			http.Error(w, "Failed to get results", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	})

	// Reset database endpoint (development only)
	http.HandleFunc("/reset-db", func(w http.ResponseWriter, r *http.Request) {
		// Only allow in development mode
		if config.Env != "development" {
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
			log.Error().Err(err).Msg("Failed to reset database schema")
			http.Error(w, "Failed to reset database", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "Database schema reset successfully",
		})
	})

	// Start server
	log.Info().
		Str("port", config.Port).
		Str("env", config.Env).
		Msg("Starting server")

	if err := http.ListenAndServe(":"+config.Port, nil); err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}

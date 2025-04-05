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
		fmt.Fprintf(w, "OK")
	})

	// Test crawl handler
	http.HandleFunc("/test-crawl", func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Query().Get("url")
		if url == "" {
			url = "https://www.teamharvey.co"  // default test URL
		}

		crawler := crawler.New(nil)  // use default config
		result, err := crawler.WarmURL(r.Context(), url)
		
		if err != nil {
			log.Error().Err(err).Msg("Crawl failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/src/crawler"
	"github.com/Harvey-AU/blue-banded-bee/src/db/postgres"
	"github.com/google/uuid"
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

	// Create a worker pool for task processing
	workerPool := postgres.NewWorkerPool(pgDB, cr, 5) // 5 concurrent workers
	workerPool.Start(context.Background())
	defer workerPool.Stop()

	// Start a goroutine to monitor job completion
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Check for jobs that should be marked complete
				rows, err := pgDB.GetDB().Query(`
					UPDATE jobs 
					SET status = 'completed', completed_at = NOW()
					WHERE (completed_tasks + failed_tasks) >= total_tasks 
					  AND status = 'running'
					RETURNING id
				`)
				if err != nil {
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
		}
	}()

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

	// Add this endpoint
	http.HandleFunc("/site", func(w http.ResponseWriter, r *http.Request) {
		// Get domain from query parameters
		domain := r.URL.Query().Get("domain")
		if domain == "" {
			http.Error(w, "Domain parameter is required", http.StatusBadRequest)
			return
		}

		// Process sitemap for the domain
		baseURL := domain
		if !strings.HasPrefix(baseURL, "http") {
			baseURL = fmt.Sprintf("https://%s", domain)
		}

		// Discover sitemaps
		sitemaps, err := cr.DiscoverSitemaps(r.Context(), domain)
		if err != nil {
			log.Error().Err(err).Str("domain", domain).Msg("Failed to discover sitemaps")
			http.Error(w, "Failed to discover sitemaps", http.StatusInternalServerError)
			return
		}

		// Create a job ID
		jobID := uuid.New().String()

		// Process all URLs from sitemaps
		var allURLs []string
		for _, sitemapURL := range sitemaps {
			urls, err := cr.ParseSitemap(r.Context(), sitemapURL)
			if err != nil {
				log.Error().Err(err).Str("sitemap", sitemapURL).Msg("Failed to parse sitemap")
				continue
			}
			allURLs = append(allURLs, urls...)
		}

		// Define current time for consistent timestamps
		now := time.Now()

		// Create a job record FIRST
		_, err = pgDB.GetDB().ExecContext(r.Context(), `
			INSERT INTO jobs (id, domain, status, progress, total_tasks, completed_tasks, 
							failed_tasks, created_at, concurrency, find_links, max_depth)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, jobID, domain, "pending", 0.0, len(allURLs), 0, 0, now, 5, false, 1)

		if err != nil {
			log.Error().Err(err).Str("domain", domain).Msg("Failed to create job")
			http.Error(w, "Failed to create job", http.StatusInternalServerError)
			return
		}

		// THEN add all URLs as tasks after job exists
		for _, url := range allURLs {
			taskID := uuid.New().String()
			_, err := pgDB.GetDB().ExecContext(r.Context(), `
				INSERT INTO tasks (id, job_id, url, status, depth, created_at, retry_count, source_type, source_url)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`, taskID, jobID, url, "pending", 0, now, 0, "sitemap", baseURL)

			if err != nil {
				log.Error().Err(err).Str("url", url).Msg("Failed to add task")
				// Continue with other tasks despite error
			}
		}

		// Update job status to running
		_, err = pgDB.GetDB().ExecContext(r.Context(), `
			UPDATE jobs SET status = 'running', started_at = $1 WHERE id = $2
		`, time.Now(), jobID)
		if err != nil {
			log.Error().Err(err).Str("job_id", jobID).Msg("Failed to update job status")
		}

		// Return success
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "OK",
			"job_id":     jobID,
			"urls_added": len(allURLs),
			"message":    "Sitemap crawl started",
		})
	})

	http.HandleFunc("/job-status", func(w http.ResponseWriter, r *http.Request) {
		jobID := r.URL.Query().Get("job_id")
		if jobID == "" {
			http.Error(w, "job_id parameter required", http.StatusBadRequest)
			return
		}

		var total, completed, failed int
		var status string
		err := pgDB.GetDB().QueryRowContext(r.Context(), `
			SELECT total_tasks, completed_tasks, failed_tasks, status 
			FROM jobs WHERE id = $1
		`, jobID).Scan(&total, &completed, &failed, &status)

		if err != nil {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"job_id":    jobID,
			"status":    status,
			"total":     total,
			"completed": completed,
			"failed":    failed,
			"progress":  float64(completed+failed) / float64(total) * 100,
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

package main

import (
	"context"
	"database/sql"
	"os"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/src/crawler"
	"github.com/Harvey-AU/blue-banded-bee/src/jobs"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

/**
 * Job Queue Test Utility
 *
 * This program tests the job queue system by:
 * 1. Setting up a database connection
 * 2. Initializing the job queue schema
 * 3. Creating a worker pool with multiple workers
 * 4. Creating and starting a test job
 * 5. Monitoring job progress until completion
 *
 * Usage:
 *   go run cmd/test_jobs/main.go
 *
 * The program expects DATABASE_URL and DATABASE_AUTH_TOKEN environment variables
 * to be set in the .env file.
 */

func main() {
	// Set up logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Fatal().Err(err).Msg("Error loading .env file")
	}

	// Get database details from environment
	dbURL := os.Getenv("DATABASE_URL")
	authToken := os.Getenv("DATABASE_AUTH_TOKEN")
	if dbURL == "" || authToken == "" {
		log.Fatal().Msg("DATABASE_URL and DATABASE_AUTH_TOKEN must be set")
	}

	// Connect to database
	db, err := sql.Open("libsql", dbURL+"?authToken="+authToken)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Initialize database schema
	if err := jobs.InitSchema(db); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize schema")
	}
	log.Info().Msg("Database schema initialized")

	// Create crawler and worker pool
	crawler := crawler.New(nil)
	workerPool := jobs.NewWorkerPool(db, crawler, 2) // 2 workers
	jobManager := jobs.NewJobManager(db, crawler, workerPool)

	// Start the worker pool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	workerPool.Start(ctx)

	// Create a test job
	jobOptions := &jobs.JobOptions{
		Domain:       "example.com",
		StartURLs:    []string{"https://example.com"},
		UseSitemap:   false,
		Concurrency:  2,
		FindLinks:    false,
		MaxDepth:     1,
		IncludePaths: nil,
		ExcludePaths: nil,
	}

	job, err := jobManager.CreateJob(ctx, jobOptions)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create job")
	}
	log.Info().Str("job_id", job.ID).Msg("Created job")

	// Start the job
	if err := jobManager.StartJob(ctx, job.ID); err != nil {
		log.Fatal().Err(err).Msg("Failed to start job")
	}
	log.Info().Str("job_id", job.ID).Msg("Started job")

	// Wait and check job status periodically
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		
		job, err := jobManager.GetJobStatus(ctx, job.ID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get job status")
			continue
		}
		
		log.Info().
			Str("job_id", job.ID).
			Str("status", string(job.Status)).
			Float64("progress", job.Progress).
			Int("completed", job.CompletedTasks).
			Int("failed", job.FailedTasks).
			Int("total", job.TotalTasks).
			Strs("recent_urls", job.RecentURLs).
			Msg("Job status")
			
		if job.Status != jobs.JobStatusRunning {
			break
		}
	}

	// Let worker pool finish any in-progress tasks
	time.Sleep(2 * time.Second)
	
	log.Info().Msg("Test completed")
}

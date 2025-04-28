package main

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/jobs"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
 * The program expects DATABASE_URL environment variable to be set in the .env file.
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
	if dbURL == "" {
		log.Fatal().Msg("DATABASE_URL must be set")
	}

	// Connect to database
	log.Info().Msg("Connecting to PostgreSQL database...")
	database, err := db.InitFromEnv()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer database.Close()

	// Set up crawler
	crawler := crawler.New(nil)

	// Create worker pool
	var jobWorkers int = 3
	workerPool := jobs.NewWorkerPool(database.GetDB(), crawler, jobWorkers, database.GetConfig())
	workerPool.Start(context.Background())
	defer workerPool.Stop()

	log.Info().Msg("Worker pool started with " + strconv.Itoa(jobWorkers) + " workers")

	// Create a test job
	jobManager := jobs.NewJobManager(database.GetDB(), crawler, nil)

	// Set up job options
	jobOptions := &jobs.JobOptions{
		Domain:      "example.com",
		Concurrency: 2,
		FindLinks:   true,
		MaxPages:    10,
		UseSitemap:  true,
	}

	// Submit the job to the queue
	job, err := jobManager.CreateJob(context.Background(), jobOptions)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create job")
	}

	log.Info().Str("job_id", job.ID).Msg("Created test job")

	// Start the job
	if err := jobManager.StartJob(context.Background(), job.ID); err != nil {
		log.Fatal().Err(err).Msg("Failed to start job")
	}

	log.Info().Str("job_id", job.ID).Msg("Started job, monitoring progress...")

	// Monitor job progress
	for {
		time.Sleep(1 * time.Second)

		job, err := jobManager.GetJobStatus(context.Background(), job.ID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get job status")
			continue
		}

		log.Info().
			Str("status", string(job.Status)).
			Float64("progress", job.Progress).
			Int("completed", job.CompletedTasks).
			Int("failed", job.FailedTasks).
			Int("total", job.TotalTasks).
			Msg("Job progress")

		if job.Status == jobs.JobStatusCompleted || job.Status == jobs.JobStatusFailed {
			log.Info().Str("final_status", string(job.Status)).Msg("Job finished")
			break
		}

		if job.Status == jobs.JobStatusRunning && job.Progress >= 100.0 {
			log.Info().Msg("Job complete")
			break
		}
	}
}

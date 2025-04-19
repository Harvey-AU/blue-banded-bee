package main

import (
	"context"
	"os"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/src/db/postgres"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load environment variables from .env.postgres file
	err := godotenv.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Error loading .env.postgres file")
	}

	// Configure logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	log.Info().Msg("Testing PostgreSQL connection")

	// Initialize PostgreSQL
	db, err := postgres.InitFromEnv()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}
	defer db.Close()

	log.Info().Msg("Successfully connected to PostgreSQL")

	// Test creating a job
	ctx := context.Background()
	jobID := uuid.New().String()
	now := time.Now()

	// Insert test job
	_, err = db.GetDB().ExecContext(ctx, `
		INSERT INTO jobs (
			id, domain, status, progress, total_tasks, completed_tasks, 
			failed_tasks, created_at, concurrency, find_links, include_paths, exclude_paths
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, jobID, "example.com", "pending", 0.0, 0, 0, 0, now, 3, true, `[]`, `[]`)

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to insert test job")
	}

	log.Info().Str("job_id", jobID).Msg("Created test job")

	// Create task queue
	queue := postgres.NewTaskQueue(db.GetDB())

	// Insert test tasks
	urls := []string{
		"https://example.com/page1",
		"https://example.com/page2",
		"https://example.com/page3",
	}

	err = queue.EnqueueTasks(ctx, jobID, urls, "test", "", 0)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to enqueue tasks")
	}

	log.Info().Int("count", len(urls)).Msg("Enqueued test tasks")

	// Try to get a task
	task, err := queue.GetNextTask(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get next task")
	}

	if task == nil {
		log.Fatal().Msg("No task found, expected at least one")
	}

	log.Info().
		Str("task_id", task.ID).
		Str("url", task.URL).
		Msg("Successfully retrieved task")

	// Mark task as completed
	task.StatusCode = 200
	task.ResponseTime = 150
	task.CacheStatus = "MISS"
	task.ContentType = "text/html"

	err = queue.CompleteTask(ctx, task)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to complete task")
	}

	log.Info().Str("task_id", task.ID).Msg("Successfully completed task")

	// Check job progress updated
	var progress float64
	err = db.GetDB().QueryRowContext(ctx,
		"SELECT progress FROM jobs WHERE id = $1", jobID).Scan(&progress)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to check job progress")
	}

	log.Info().Float64("progress", progress).Msg("Job progress updated")

	// Clean up test data
	_, err = db.GetDB().ExecContext(ctx, "DELETE FROM tasks WHERE job_id = $1", jobID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to clean up tasks")
	}

	_, err = db.GetDB().ExecContext(ctx, "DELETE FROM jobs WHERE id = $1", jobID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to clean up job")
	}

	log.Info().Msg("Test completed successfully!")
}

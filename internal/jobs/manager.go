package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// DbQueueProvider defines the interface for database operations
type DbQueueProvider interface {
	Execute(ctx context.Context, fn func(*sql.Tx) error) error
	EnqueueURLs(ctx context.Context, jobID string, pageIDs []int, paths []string, sourceType string, sourceURL string, depth int) error
	CleanupStuckJobs(ctx context.Context) error
}

// JobManager handles job creation and lifecycle management
type JobManager struct {
	db      *sql.DB
	dbQueue DbQueueProvider
	crawler *crawler.Crawler

	workerPool *WorkerPool
}

// NewJobManager creates a new job manager
func NewJobManager(db *sql.DB, dbQueue DbQueueProvider, crawler *crawler.Crawler, workerPool *WorkerPool) *JobManager {
	return &JobManager{
		db:         db,
		dbQueue:    dbQueue,
		crawler:    crawler,
		workerPool: workerPool,
	}
}

// CreateJob creates a new job with the given options
func (jm *JobManager) CreateJob(ctx context.Context, options *JobOptions) (*Job, error) {
	span := sentry.StartSpan(ctx, "manager.create_job")
	defer span.Finish()

	span.SetTag("domain", options.Domain)

	// Create a new job object
	job := &Job{
		ID:              uuid.New().String(),
		Domain:          options.Domain,
		Status:          JobStatusPending,
		Progress:        0,
		TotalTasks:      0,
		CompletedTasks:  0,
		FoundTasks:      0,
		SitemapTasks:    0,
		FailedTasks:     0,
		CreatedAt:       time.Now(),
		Concurrency:     options.Concurrency,
		FindLinks:       options.FindLinks,
		MaxPages:        options.MaxPages,
		IncludePaths:    options.IncludePaths,
		ExcludePaths:    options.ExcludePaths,
		RequiredWorkers: options.RequiredWorkers,
	}

	// Use dbQueue for transaction safety
	err := jm.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		// Get or create domain ID
		var domainID int
		err := tx.QueryRow(`
			INSERT INTO domains(name) VALUES($1) 
			ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name 
			RETURNING id`, options.Domain).Scan(&domainID)
		if err != nil {
			return fmt.Errorf("failed to get or create domain: %w", err)
		}

		// Insert the job
		_, err = tx.Exec(
			`INSERT INTO jobs (
				id, domain_id, status, progress, total_tasks, completed_tasks, failed_tasks,
				created_at, concurrency, find_links, include_paths, exclude_paths,
				required_workers, max_pages,
				found_tasks, sitemap_tasks
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
			job.ID, domainID, string(job.Status), job.Progress,
			job.TotalTasks, job.CompletedTasks, job.FailedTasks,
			job.CreatedAt, job.Concurrency, job.FindLinks,
			db.Serialize(job.IncludePaths), db.Serialize(job.ExcludePaths),
			job.RequiredWorkers, job.MaxPages,
			job.FoundTasks, job.SitemapTasks,
		)
		return err
	})

	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	log.Info().
		Str("job_id", job.ID).
		Str("domain", job.Domain).
		Bool("use_sitemap", options.UseSitemap).
		Bool("find_links", options.FindLinks).
		Int("max_pages", options.MaxPages).
		Msg("Created new job")

	if options.UseSitemap {
		// Fetch and process sitemap in a separate goroutine
		go jm.processSitemap(context.Background(), job.ID, options.Domain, options.IncludePaths, options.ExcludePaths)
	} else {
		// Add domain root URL as a fallback
		rootURL := fmt.Sprintf("https://%s", options.Domain)
		// Use the EnqueueURLs method from db package
		pageIDs := []int{1} // Will need to be fixed 
		paths := []string{rootURL}
		if err := jm.dbQueue.EnqueueURLs(ctx, job.ID, pageIDs, paths, "manual", "", 0); err != nil {
			span.SetTag("error", "true")
			span.SetData("error.message", err.Error())
			log.Error().Err(err).Msg("Failed to enqueue root URL")
		}
	}

	return job, nil
}

// StartJob starts a pending job
func (jm *JobManager) StartJob(ctx context.Context, jobID string) error {
	span := sentry.StartSpan(ctx, "manager.start_job")
	defer span.Finish()

	span.SetTag("job_id", jobID)

	// Get the job using our new method
	job, err := jm.GetJob(ctx, jobID)
	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Allow restarting jobs that are either pending or running
	if job.Status != JobStatusPending && job.Status != JobStatusRunning {
		return fmt.Errorf("job cannot be started: %s", job.Status)
	}

	// Update job status to running (even if it was already running)
	job.Status = JobStatusRunning

	// Only update started_at if it wasn't already set
	if job.StartedAt.IsZero() {
		job.StartedAt = time.Now()
	}

	// Use dbQueue for transaction safety
	err = jm.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		// Recover any tasks that were in progress when the server shut down
		_, err := tx.ExecContext(ctx, `
			UPDATE tasks 
			SET status = $1,
				started_at = NULL,
				retry_count = retry_count + 1
			WHERE job_id = $2 
			AND status = $3
		`, TaskStatusPending, jobID, TaskStatusRunning)

		if err != nil {
			log.Error().Err(err).Str("job_id", jobID).Msg("Failed to reset in-progress tasks")
			// Don't return error, continue with job start
		}

		// Update job status
		_, err = tx.ExecContext(ctx, `
			UPDATE jobs
			SET status = $1, started_at = $2
			WHERE id = $3
		`, job.Status, job.StartedAt, job.ID)

		return err
	})

	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Add job to worker pool for processing
	// TODO: Provide worker count per job, to allow for higher volume / priority jobs
	jm.workerPool.AddJob(job.ID, nil)

	log.Debug().
		Str("job_id", job.ID).
		Str("domain", job.Domain).
		Msg("Started job")

	return nil
}

// CancelJob cancels a running job
func (jm *JobManager) CancelJob(ctx context.Context, jobID string) error {
	span := sentry.StartSpan(ctx, "manager.cancel_job")
	defer span.Finish()

	span.SetTag("job_id", jobID)

	// Get the job using our new method
	job, err := jm.GetJob(ctx, jobID)
	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Check if job can be canceled
	if job.Status != JobStatusRunning && job.Status != JobStatusPending && job.Status != JobStatusPaused {
		return fmt.Errorf("job cannot be canceled: %s", job.Status)
	}

	// Update job status to cancelled
	job.Status = JobStatusCancelled
	job.CompletedAt = time.Now()

	// Use dbQueue for transaction safety
	err = jm.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		// Update job status
		_, err := tx.ExecContext(ctx, `
			UPDATE jobs
			SET status = $1, completed_at = $2
			WHERE id = $3
		`, job.Status, job.CompletedAt, job.ID)

		if err != nil {
			return err
		}

		// Cancel pending tasks
		_, err = tx.ExecContext(ctx, `
			UPDATE tasks
			SET status = $1
			WHERE job_id = $2 AND status = $3
		`, TaskStatusSkipped, job.ID, TaskStatusPending)

		return err
	})

	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		log.Error().Err(err).Str("job_id", job.ID).Msg("Failed to cancel job")
	}

	// Remove job from worker pool
	jm.workerPool.RemoveJob(job.ID)

	log.Debug().
		Str("job_id", job.ID).
		Str("domain", job.Domain).
		Msg("Cancelled job")

	return nil
}

// GetJob retrieves a job by ID
func (jm *JobManager) GetJob(ctx context.Context, jobID string) (*Job, error) {
	span := sentry.StartSpan(ctx, "jobs.get_job")
	defer span.Finish()

	span.SetTag("job_id", jobID)

	var job Job
	var includePaths, excludePaths []byte
	var startedAt, completedAt sql.NullTime
	var errorMessage sql.NullString

	// Use DbQueue.Execute for transactional safety
	err := jm.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		// Query for job with domain join
		err := tx.QueryRowContext(ctx, `
			SELECT 
				j.id, d.name, j.status, j.progress, j.total_tasks, j.completed_tasks, j.failed_tasks,
				j.created_at, j.started_at, j.completed_at, j.concurrency, j.find_links,
				j.include_paths, j.exclude_paths, j.error_message, j.required_workers
			FROM jobs j
			JOIN domains d ON j.domain_id = d.id
			WHERE j.id = $1
		`, jobID).Scan(
			&job.ID, &job.Domain, &job.Status, &job.Progress, &job.TotalTasks, &job.CompletedTasks,
			&job.FailedTasks, &job.CreatedAt, &startedAt, &completedAt, &job.Concurrency,
			&job.FindLinks, &includePaths, &excludePaths, &errorMessage, &job.RequiredWorkers,
		)
		return err
	})

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found: %s", jobID)
	} else if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	// Handle nullable fields
	if startedAt.Valid {
		job.StartedAt = startedAt.Time
	}

	if completedAt.Valid {
		job.CompletedAt = completedAt.Time
	}

	if errorMessage.Valid {
		job.ErrorMessage = errorMessage.String
	}

	// Parse arrays from JSON
	if len(includePaths) > 0 {
		err = json.Unmarshal(includePaths, &job.IncludePaths)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal include paths: %w", err)
		}
	}

	if len(excludePaths) > 0 {
		err = json.Unmarshal(excludePaths, &job.ExcludePaths)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal exclude paths: %w", err)
		}
	}

	return &job, nil
}

// GetJobStatus gets the current status of a job
func (jm *JobManager) GetJobStatus(ctx context.Context, jobID string) (*Job, error) {
	// First cleanup any stuck jobs using dbQueue
	if err := jm.dbQueue.CleanupStuckJobs(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to cleanup stuck jobs during status check")
		// Don't return error, continue with status check
	}

	span := sentry.StartSpan(ctx, "manager.get_job_status")
	defer span.Finish()

	span.SetTag("job_id", jobID)

	// Get the job using our new method
	job, err := jm.GetJob(ctx, jobID)
	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return job, nil
}

// processSitemap fetches and processes a sitemap for a domain
func (jm *JobManager) processSitemap(ctx context.Context, jobID, domain string, includePaths, excludePaths []string) {
	span := sentry.StartSpan(ctx, "manager.process_sitemap")
	defer span.Finish()

	span.SetTag("job_id", jobID)
	span.SetTag("domain", domain)

	log.Info().
		Str("job_id", jobID).
		Str("domain", domain).
		Msg("Starting sitemap processing")

	// Create a crawler config that allows skipping already cached URLs
	crawlerConfig := crawler.DefaultConfig()
	crawlerConfig.SkipCachedURLs = false
	sitemapCrawler := crawler.New(crawlerConfig)

	// Discover sitemaps for the domain
	baseURL := fmt.Sprintf("https://%s", domain)
	urls, err := sitemapCrawler.DiscoverSitemaps(ctx, baseURL)
	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		log.Error().
			Err(err).
			Str("job_id", jobID).
			Str("domain", domain).
			Msg("Failed to discover sitemaps")

		// Update job with error using dbQueue
		if updateErr := jm.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
				UPDATE jobs
				SET error_message = $1
				WHERE id = $2
			`, fmt.Sprintf("Failed to discover sitemaps: %v", err), jobID)
			return err
		}); updateErr != nil {
			log.Error().Err(updateErr).Str("job_id", jobID).Msg("Failed to update job with error message")
		}
		return
	}

	// Filter URLs based on include/exclude patterns
	if len(includePaths) > 0 || len(excludePaths) > 0 {
		urls = sitemapCrawler.FilterURLs(urls, includePaths, excludePaths)
	}

	// Add URLs to the job queue
	if len(urls) > 0 {
		// Use the EnqueueURLs method from dbQueue
		// TODO: Get proper page IDs - this needs fixing
		pageIDs := make([]int, len(urls))
		for i := range pageIDs {
			pageIDs[i] = i + 1 // Temporary placeholder
		}
		if err := jm.dbQueue.EnqueueURLs(ctx, jobID, pageIDs, urls, "sitemap", baseURL, 0); err != nil {
			span.SetTag("error", "true")
			span.SetData("error.message", err.Error())
			log.Error().
				Err(err).
				Str("job_id", jobID).
				Str("domain", domain).
				Msg("Failed to enqueue sitemap URLs")
			return
		}

		log.Info().
			Str("job_id", jobID).
			Str("domain", domain).
			Int("url_count", len(urls)).
			Msg("Added sitemap URLs to job queue")
	} else {
		log.Info().
			Str("job_id", jobID).
			Str("domain", domain).
			Msg("No URLs found in sitemap")

		// Update job with warning using dbQueue
		if updateErr := jm.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
				UPDATE jobs
				SET error_message = $1
				WHERE id = $2
			`, "No URLs found in sitemap", jobID)
			return err
		}); updateErr != nil {
			log.Error().Err(updateErr).Str("job_id", jobID).Msg("Failed to update job with warning message")
		}
	}

	// Start the job if it's in pending state
	job, err := jm.GetJob(ctx, jobID)
	if err == nil && job.Status == JobStatusPending {
		if err := jm.StartJob(ctx, jobID); err != nil {
			log.Error().
				Err(err).
				Str("job_id", jobID).
				Msg("Failed to start job after processing sitemap")
		}
	}
}

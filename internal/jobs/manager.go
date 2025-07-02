package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/util"
	"github.com/PuerkitoBio/goquery"
	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

// DbQueueProvider defines the interface for database operations
type DbQueueProvider interface {
	Execute(ctx context.Context, fn func(*sql.Tx) error) error
	EnqueueURLs(ctx context.Context, jobID string, pageIDs []int, paths []string, sourceType string, sourceURL string) error
	CleanupStuckJobs(ctx context.Context) error
}

// JobManager handles job creation and lifecycle management
type JobManager struct {
	db      *sql.DB
	dbQueue DbQueueProvider
	crawler *crawler.Crawler

	workerPool *WorkerPool
	
	// Map to track which pages have been processed for each job
	processedPages map[string]struct{} // Key format: "jobID_pageID"
	pagesMutex     sync.RWMutex        // Mutex for thread-safe access
}

// NewJobManager creates a new job manager
func NewJobManager(db *sql.DB, dbQueue DbQueueProvider, crawler *crawler.Crawler, workerPool *WorkerPool) *JobManager {
	return &JobManager{
		db:             db,
		dbQueue:        dbQueue,
		crawler:        crawler,
		workerPool:     workerPool,
		processedPages: make(map[string]struct{}),
	}
}

// CreateJob creates a new job with the given options
func (jm *JobManager) CreateJob(ctx context.Context, options *JobOptions) (*Job, error) {
	span := sentry.StartSpan(ctx, "manager.create_job")
	defer span.Finish()

	span.SetTag("domain", options.Domain)

	normalisedDomain := util.NormaliseDomain(options.Domain)
	
	// Check for existing active jobs for the same domain and org
	if options.OrganisationID != nil && *options.OrganisationID != "" {
		var existingJobID string
		var existingJobStatus string
		var existingOrgID string
		
		err := jm.db.QueryRowContext(ctx, `
			SELECT j.id, j.status, j.organisation_id
			FROM jobs j
			JOIN domains d ON j.domain_id = d.id
			WHERE d.name = $1
			AND j.organisation_id = $2
			AND j.status IN ('pending', 'initializing', 'running', 'paused')
			ORDER BY j.created_at DESC
			LIMIT 1
		`, normalisedDomain, *options.OrganisationID).Scan(&existingJobID, &existingJobStatus, &existingOrgID)
		
		if err == nil && existingJobID != "" {
			// Found an existing active job for the same domain and organisation
			log.Info().
				Str("existing_job_id", existingJobID).
				Str("existing_job_status", existingJobStatus).
				Str("domain", normalisedDomain).
				Str("organisation_id", *options.OrganisationID).
				Msg("Found existing active job for domain, cancelling it")
		
			
			if err := jm.CancelJob(ctx, existingJobID); err != nil {
				log.Error().
					Err(err).
					Str("job_id", existingJobID).
					Msg("Failed to cancel existing job")
				// Continue with new job creation even if cancellation fails
			}
		} else if err != nil && err != sql.ErrNoRows {
			// Log query error but continue with job creation
			log.Warn().
				Err(err).
				Str("domain", normalisedDomain).
				Msg("Error checking for existing jobs")
		}
	}
	
	// Create a new job object
	job := &Job{
		ID:              uuid.New().String(),
		Domain:          normalisedDomain,
		UserID:          options.UserID,
		OrganisationID:  options.OrganisationID,
		Status:          JobStatusPending,
		Progress:        0,
		TotalTasks:      0,
		CompletedTasks:  0,
		FoundTasks:      0,
		SitemapTasks:    0,
		FailedTasks:     0,
		CreatedAt:       time.Now().UTC(),
		Concurrency:     options.Concurrency,
		FindLinks:       options.FindLinks,
		MaxPages:        options.MaxPages,
		IncludePaths:    options.IncludePaths,
		ExcludePaths:    options.ExcludePaths,
		RequiredWorkers: options.RequiredWorkers,
		SourceType:      options.SourceType,
		SourceDetail:    options.SourceDetail,
		SourceInfo:      options.SourceInfo,
	}

	var domainID int
	
	// Use dbQueue for transaction safety
	err := jm.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		// Get or create domain ID
		err := tx.QueryRow(`
			INSERT INTO domains(name) VALUES($1) 
			ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name 
			RETURNING id`, normalisedDomain).Scan(&domainID)
		if err != nil {
			return fmt.Errorf("failed to get or create domain: %w", err)
		}

		// Insert the job
		_, err = tx.Exec(
			`INSERT INTO jobs (
				id, domain_id, user_id, organisation_id, status, progress, total_tasks, completed_tasks, failed_tasks, skipped_tasks,
				created_at, concurrency, find_links, include_paths, exclude_paths,
				required_workers, max_pages,
				found_tasks, sitemap_tasks, source_type, source_detail, source_info
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`,
			job.ID, domainID, job.UserID, job.OrganisationID, string(job.Status), job.Progress,
			job.TotalTasks, job.CompletedTasks, job.FailedTasks, job.SkippedTasks,
			job.CreatedAt, job.Concurrency, job.FindLinks,
			db.Serialise(job.IncludePaths), db.Serialise(job.ExcludePaths),
			job.RequiredWorkers, job.MaxPages,
			job.FoundTasks, job.SitemapTasks, job.SourceType, job.SourceDetail, job.SourceInfo,
		)
		return err
	})

	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		sentry.CaptureException(err)
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
		go jm.processSitemap(context.Background(), job.ID, normalisedDomain, options.IncludePaths, options.ExcludePaths)
	} else {
		// Prepare for manual root URL creation
		rootPath := "/"
		
		// Create a page record for the root URL
		err := jm.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
			var pageID int
			err := tx.QueryRowContext(ctx, `
				INSERT INTO pages (domain_id, path)
				VALUES ($1, $2)
				ON CONFLICT (domain_id, path) DO UPDATE SET path = EXCLUDED.path
				RETURNING id
			`, domainID, rootPath).Scan(&pageID)
			
			if err != nil {
				return fmt.Errorf("failed to create page record for root path: %w", err)
			}
			
			// Enqueue the root URL with its page ID
			_, err = tx.ExecContext(ctx, `
				INSERT INTO tasks (
					id, job_id, page_id, path, status, created_at, retry_count,
					source_type, source_url
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`, uuid.New().String(), job.ID, pageID, rootPath, "pending", time.Now().UTC(), 0, "manual", "")
			
			if err != nil {
				return fmt.Errorf("failed to enqueue task for root path: %w", err)
			}
			
			// Update job's total task count and found_tasks count (for root URL)
			_, err = tx.ExecContext(ctx, `
				UPDATE jobs
				SET total_tasks = total_tasks + 1,
				    found_tasks = found_tasks + 1
				WHERE id = $1
			`, job.ID)
			
			if err != nil {
				return err
			}

			jm.markPageProcessed(job.ID, pageID)
			
			return nil
		})
		
		if err != nil {
			span.SetTag("error", "true")
			span.SetData("error.message", err.Error())
			log.Error().Err(err).Msg("Failed to create and enqueue root URL")
		} else {
			log.Info().
				Str("job_id", job.ID).
				Str("domain", normalisedDomain).
				Msg("Added root URL to job queue")
		}
	}

	return job, nil
}

// StartJob starts a pending job
func (jm *JobManager) StartJob(ctx context.Context, jobID string) error {
	span := sentry.StartSpan(ctx, "manager.restart_job")
	defer span.Finish()

	span.SetTag("original_job_id", jobID)

	// Get the original job to copy its configuration
	originalJob, err := jm.GetJob(ctx, jobID)
	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		sentry.CaptureException(err)
		return fmt.Errorf("failed to get original job: %w", err)
	}

	// Only allow restarting completed, failed, or cancelled jobs
	if originalJob.Status != JobStatusCompleted && originalJob.Status != JobStatusFailed && originalJob.Status != JobStatusCancelled {
		return fmt.Errorf("job cannot be restarted: %s (only completed, failed, or cancelled jobs can be restarted)", originalJob.Status)
	}

	// Create new job with same configuration
	newJobOptions := &JobOptions{
		Domain:         originalJob.Domain,
		UserID:         originalJob.UserID,
		OrganisationID: originalJob.OrganisationID,
		UseSitemap:     true, // Default to true
		Concurrency:    originalJob.Concurrency,
		FindLinks:      originalJob.FindLinks,
		MaxPages:       originalJob.MaxPages,
		IncludePaths:   originalJob.IncludePaths,
		ExcludePaths:   originalJob.ExcludePaths,
		RequiredWorkers: originalJob.RequiredWorkers,
	}

	// Create the new job
	newJob, err := jm.CreateJob(ctx, newJobOptions)
	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		sentry.CaptureException(err)
		return fmt.Errorf("failed to create new job: %w", err)
	}

	span.SetTag("new_job_id", newJob.ID)
	log.Info().Str("original_job_id", jobID).Str("new_job_id", newJob.ID).Msg("Created new job as restart")

	// Add new job to worker pool for processing
	jm.workerPool.AddJob(newJob.ID, newJobOptions)

	log.Debug().
		Str("original_job_id", jobID).
		Str("new_job_id", newJob.ID).
		Str("domain", newJob.Domain).
		Msg("Restarted job with new job ID")

	return nil
}

// Helper method to check if a page has been processed for a job
func (jm *JobManager) isPageProcessed(jobID string, pageID int) bool {
	key := fmt.Sprintf("%s_%d", jobID, pageID)
	jm.pagesMutex.RLock()
	defer jm.pagesMutex.RUnlock()
	_, exists := jm.processedPages[key]
	return exists
}

// Helper method to mark a page as processed for a job
func (jm *JobManager) markPageProcessed(jobID string, pageID int) {
	key := fmt.Sprintf("%s_%d", jobID, pageID)
	jm.pagesMutex.Lock()
	defer jm.pagesMutex.Unlock()
	jm.processedPages[key] = struct{}{}
}

// Helper method to clear processed pages for a job (when job is completed or canceled)
func (jm *JobManager) clearProcessedPages(jobID string) {
	jm.pagesMutex.Lock()
	defer jm.pagesMutex.Unlock()
	
	// Find all keys that start with this job ID
	prefix := jobID + "_"
	for key := range jm.processedPages {
		if strings.HasPrefix(key, prefix) {
			delete(jm.processedPages, key)
		}
	}
}

// EnqueueJobURLs is a wrapper around dbQueue.EnqueueURLs that adds duplicate detection
func (jm *JobManager) EnqueueJobURLs(ctx context.Context, jobID string, pageIDs []int, paths []string, sourceType string, sourceURL string) error {
	span := sentry.StartSpan(ctx, "manager.enqueue_job_urls")
	defer span.Finish()
	
	span.SetTag("job_id", jobID)
	span.SetTag("url_count", fmt.Sprintf("%d", len(pageIDs)))
	
	if len(pageIDs) == 0 {
		return nil
	}
	
	// Filter out pages that have already been processed
	var filteredPageIDs []int
	var filteredPaths []string
	
	for i, pageID := range pageIDs {
		if !jm.isPageProcessed(jobID, pageID) {
			filteredPageIDs = append(filteredPageIDs, pageID)
			filteredPaths = append(filteredPaths, paths[i])
			// Don't mark as processed yet - we'll do that after successful enqueue
		}
	}
	
	// If all pages were already processed, just return success
	if len(filteredPageIDs) == 0 {
		log.Debug().
			Str("job_id", jobID).
			Int("skipped_urls", len(pageIDs)).
			Msg("All URLs already processed, skipping")
		return nil
	}
	
	log.Debug().
		Str("job_id", jobID).
		Int("total_urls", len(pageIDs)).
		Int("new_urls", len(filteredPageIDs)).
		Int("skipped_urls", len(pageIDs) - len(filteredPageIDs)).
		Msg("Enqueueing filtered URLs")
	
	// Use the filtered lists to enqueue only new pages
	err := jm.dbQueue.EnqueueURLs(ctx, jobID, filteredPageIDs, filteredPaths, sourceType, sourceURL)
	
	// Only mark pages as processed if the enqueue was successful
	if err == nil {
		// If these are found links (not from sitemap), update the found_tasks counter
		if sourceType != "sitemap" && len(filteredPageIDs) > 0 {
			updateErr := jm.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `
					UPDATE jobs
					SET found_tasks = found_tasks + $1
					WHERE id = $2
				`, len(filteredPageIDs), jobID)
				return err
			})
			if updateErr != nil {
				log.Error().
					Err(updateErr).
					Str("job_id", jobID).
					Msg("Failed to update found task count")
			}
		}
		
		// Mark all successfully enqueued pages as processed
		for _, pageID := range filteredPageIDs {
			jm.markPageProcessed(jobID, pageID)
		}
	} else {
		log.Error().
			Err(err).
			Str("job_id", jobID).
			Int("url_count", len(filteredPageIDs)).
			Msg("Failed to enqueue URLs, not marking pages as processed")
	}
	
	return err
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
		sentry.CaptureException(err)
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Check if job can be canceled
	if job.Status != JobStatusRunning && job.Status != JobStatusPending && job.Status != JobStatusPaused {
		return fmt.Errorf("job cannot be canceled: %s", job.Status)
	}

	// Update job status to cancelled
	job.Status = JobStatusCancelled
	job.CompletedAt = time.Now().UTC()

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
		sentry.CaptureException(err)
		log.Error().Err(err).Str("job_id", job.ID).Msg("Failed to cancel job")
	}

	// Remove job from worker pool
	jm.workerPool.RemoveJob(job.ID)
	
	// Clear processed pages for this job
	jm.clearProcessedPages(job.ID)

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
				j.id, d.name, j.status, j.progress, j.total_tasks, j.completed_tasks, j.failed_tasks, j.skipped_tasks,
				j.created_at, j.started_at, j.completed_at, j.concurrency, j.find_links,
				j.include_paths, j.exclude_paths, j.error_message, j.required_workers,
				j.found_tasks, j.sitemap_tasks
			FROM jobs j
			JOIN domains d ON j.domain_id = d.id
			WHERE j.id = $1
		`, jobID).Scan(
			&job.ID, &job.Domain, &job.Status, &job.Progress, &job.TotalTasks, &job.CompletedTasks,
			&job.FailedTasks, &job.SkippedTasks, &job.CreatedAt, &startedAt, &completedAt, &job.Concurrency,
			&job.FindLinks, &includePaths, &excludePaths, &errorMessage, &job.RequiredWorkers,
			&job.FoundTasks, &job.SitemapTasks,
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
	sitemaps, err := sitemapCrawler.DiscoverSitemaps(ctx, domain)
	
	// Log discovered sitemaps
	log.Info().
		Str("job_id", jobID).
		Str("domain", domain).
		Int("sitemap_count", len(sitemaps)).
		Msg("Sitemaps discovered")
		
	// Process each sitemap to extract URLs
	var urls []string
	for _, sitemapURL := range sitemaps {
		log.Info().
			Str("job_id", jobID).
			Str("sitemap_url", sitemapURL).
			Msg("Processing sitemap")
			
		sitemapURLs, err := sitemapCrawler.ParseSitemap(ctx, sitemapURL)
		if err != nil {
			log.Warn().
				Err(err).
				Str("job_id", jobID).
				Str("sitemap_url", sitemapURL).
				Msg("Error parsing sitemap")
			continue
		}
		
		log.Info().
			Str("job_id", jobID).
			Str("sitemap_url", sitemapURL).
			Int("url_count", len(sitemapURLs)).
			Msg("Parsed URLs from sitemap")
			
		urls = append(urls, sitemapURLs...)
	}
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
		// Log URLs for debugging
		for i, url := range urls {
			log.Debug().
				Str("job_id", jobID).
				Str("domain", domain).
				Int("index", i).
				Str("url", url).
				Msg("URL from sitemap")
		}
		
		// Get domain ID from the job
		var domainID int
		err := jm.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
			return tx.QueryRowContext(ctx, `
				SELECT domain_id FROM jobs WHERE id = $1
			`, jobID).Scan(&domainID)
		})
		if err != nil {
			span.SetTag("error", "true")
			span.SetData("error.message", err.Error())
			log.Error().
				Err(err).
				Str("job_id", jobID).
				Str("domain", domain).
				Msg("Failed to get domain ID for job")
			return
		}

		// Create page records and get their IDs
		pageIDs, paths, err := db.CreatePageRecords(ctx, jm.dbQueue, domainID, urls)
		if err != nil {
			span.SetTag("error", "true")
			span.SetData("error.message", err.Error())
			log.Error().
				Err(err).
				Str("job_id", jobID).
				Str("domain", domain).
				Msg("Failed to create page records")
			return
		}
		
		// Update sitemap task count in the job
		err = jm.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
				UPDATE jobs
				SET sitemap_tasks = sitemap_tasks + $1
				WHERE id = $2
			`, len(pageIDs), jobID)
			return err
		})
		if err != nil {
			log.Error().
				Err(err).
				Str("job_id", jobID).
				Msg("Failed to update sitemap task count")
		}
		
		// Use our wrapper function that checks for duplicates
		baseURL := fmt.Sprintf("https://%s", domain)
		if err := jm.EnqueueJobURLs(ctx, jobID, pageIDs, paths, "sitemap", baseURL); err != nil {
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
		
		// Set homepage priority to 1.000
		if err := jm.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
			// Update homepage task priority
			_, err := tx.ExecContext(ctx, `
				UPDATE tasks t
				SET priority_score = 1.000
				FROM pages p
				WHERE t.page_id = p.id
				AND t.job_id = $1
				AND p.path = '/'
			`, jobID)
			return err
		}); err != nil {
			log.Error().
				Err(err).
				Str("job_id", jobID).
				Msg("Failed to set homepage priority")
		} else {
			log.Info().
				Str("job_id", jobID).
				Msg("Set homepage priority to 1.000")
		}
		
		// Crawl homepage to find header links and set their priorities to 1.000
		go func() {
			// Create a new context for this background operation
			bgCtx := context.Background()
			
			// Get the homepage URL
			homepageURL := fmt.Sprintf("https://%s/", domain)
			
			// Fetch the homepage HTML
			client := &http.Client{
				Timeout: 30 * time.Second,
			}
			
			resp, err := client.Get(homepageURL)
			if err != nil {
				log.Error().
					Err(err).
					Str("job_id", jobID).
					Msg("Failed to fetch homepage for header links")
				return
			}
			defer resp.Body.Close()
			
			// Parse HTML to find header links
			doc, err := goquery.NewDocumentFromReader(resp.Body)
			if err != nil {
				log.Error().
					Err(err).
					Str("job_id", jobID).
					Msg("Failed to parse homepage HTML")
				return
			}
			
			// Find all links within header tags
			// TODO: Ensure works for buttons and non-"A type" elements
			// TODO: Change this to use the same functionality as other find_links functionality in project, but just constrained to HEADER.
			headerLinks := []string{}
			doc.Find("header a[href]").Each(func(i int, s *goquery.Selection) {
				if href, exists := s.Attr("href"); exists {
					// Convert relative URLs to absolute
					u, err := url.Parse(href)
					if err == nil {
						if !u.IsAbs() {
							base, _ := url.Parse(homepageURL)
							u = base.ResolveReference(u)
						}
						// Only include same-domain links
						if u.Host == "" || u.Host == domain || strings.HasSuffix(u.Host, "."+domain) {
							headerLinks = append(headerLinks, u.String())
						}
					}
				}
			})
			
			if len(headerLinks) > 0 {
				// Extract paths from header links
				// TODO: Reduce priority by 1% by order link is found, 1st is 1.000, 2nd is 0.990, 3rd is 0.980
				// TODO: Apply this same logic to links found in pages
				paths := make([]string, 0, len(headerLinks))
				for _, link := range headerLinks {
					u, err := url.Parse(link)
					if err != nil {
						continue
					}
					path := u.Path
					if path == "" {
						path = "/"
					}
					paths = append(paths, path)
				}
				
				// Update header link priorities to 1.000 in a single query
				if err := jm.dbQueue.Execute(bgCtx, func(tx *sql.Tx) error {
					result, err := tx.ExecContext(bgCtx, `
						UPDATE tasks t
						SET priority_score = 1.000
						FROM pages p
						WHERE t.page_id = p.id
						AND t.job_id = $1
						AND p.path = ANY($2)
						AND t.priority_score < 1.000
					`, jobID, pq.Array(paths))
					
					if err != nil {
						return fmt.Errorf("failed to update header link priorities: %w", err)
					}
					
					rowsAffected, _ := result.RowsAffected()
					log.Info().
						Str("job_id", jobID).
						Int("header_link_count", len(paths)).
						Int64("tasks_updated", rowsAffected).
						Msg("Updated header link priorities to 1.000")
					
					return nil
				}); err != nil {
					log.Error().
						Err(err).
						Str("job_id", jobID).
						Msg("Failed to update header link priorities")
				}
			} else {
				log.Info().
					Str("job_id", jobID).
					Msg("No header links found on homepage")
			}
		}()
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
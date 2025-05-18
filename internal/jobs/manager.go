package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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

	// Normalize domain to ensure consistent handling of www. prefix and http/https
	normalizedDomain := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(options.Domain, "http://"), "https://"), "www.")
	normalizedDomain = strings.TrimSuffix(normalizedDomain, "/")
	
	// Create a new job object
	job := &Job{
		ID:              uuid.New().String(),
		Domain:          normalizedDomain, // Use normalized domain
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

	var domainID int
	
	// Use dbQueue for transaction safety
	err := jm.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		// Get or create domain ID
		err := tx.QueryRow(`
			INSERT INTO domains(name) VALUES($1) 
			ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name 
			RETURNING id`, normalizedDomain).Scan(&domainID)
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
		go jm.processSitemap(context.Background(), job.ID, normalizedDomain, options.IncludePaths, options.ExcludePaths)
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
			
			// Mark this page as processed for this job
			// Will mark as processed after task creation succeeds
			
			// Enqueue the root URL with its page ID
			_, err = tx.ExecContext(ctx, `
				INSERT INTO tasks (
					id, job_id, page_id, path, status, depth, created_at, retry_count,
					source_type, source_url
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			`, uuid.New().String(), job.ID, pageID, rootPath, "pending", 0, time.Now(), 0, "manual", "")
			
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
			
			// Only mark this page as processed after successful task creation
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
				Str("domain", normalizedDomain).
				Msg("Added root URL to job queue")
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

// Note: This method was previously used to mark multiple pages as processed
// but has been replaced by marking individual pages after successful task creation
// in the EnqueueJobURLs function. Kept as documentation.

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
func (jm *JobManager) EnqueueJobURLs(ctx context.Context, jobID string, pageIDs []int, paths []string, sourceType string, sourceURL string, depth int) error {
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
	err := jm.dbQueue.EnqueueURLs(ctx, jobID, filteredPageIDs, filteredPaths, sourceType, sourceURL, depth)
	
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
				if err != nil {
					return err
				}
				
				// Only mark this page as processed after successful task creation
				jm.markPageProcessed(job.ID, pageID)
				
				return nil
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
				j.id, d.name, j.status, j.progress, j.total_tasks, j.completed_tasks, j.failed_tasks,
				j.created_at, j.started_at, j.completed_at, j.concurrency, j.find_links,
				j.include_paths, j.exclude_paths, j.error_message, j.required_workers,
				j.found_tasks, j.sitemap_tasks
			FROM jobs j
			JOIN domains d ON j.domain_id = d.id
			WHERE j.id = $1
		`, jobID).Scan(
			&job.ID, &job.Domain, &job.Status, &job.Progress, &job.TotalTasks, &job.CompletedTasks,
			&job.FailedTasks, &job.CreatedAt, &startedAt, &completedAt, &job.Concurrency,
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

	// Get the job using our new method
	job, err := jm.GetJob(ctx, jobID)
	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return job, nil
}

// createPageRecords creates page records for a list of URLs and returns their IDs and paths
func (jm *JobManager) createPageRecords(ctx context.Context, domainID int, urls []string) ([]int, []string, error) {
	span := sentry.StartSpan(ctx, "manager.create_page_records")
	defer span.Finish()

	span.SetTag("domain_id", fmt.Sprintf("%d", domainID))
	span.SetTag("url_count", fmt.Sprintf("%d", len(urls)))

	if len(urls) == 0 {
		return []int{}, []string{}, nil
	}

	pageIDs := make([]int, 0, len(urls))
	paths := make([]string, 0, len(urls))

	// Get domain name from the database
	var domainName string
	err := jm.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT name FROM domains WHERE id = $1
		`, domainID).Scan(&domainName)
	})
	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return nil, nil, fmt.Errorf("failed to get domain name: %w", err)
	}

	// Extract paths from URLs
	for _, url := range urls {
		// Parse URL to extract the path
		log.Debug().Str("original_url", url).Msg("Processing URL")

		// Remove any protocol and domain to get just the path
		path := url
		// Strip common prefixes
		path = strings.TrimPrefix(path, "http://")
		path = strings.TrimPrefix(path, "https://")
		path = strings.TrimPrefix(path, "www.")
		
		// Find the first slash after the domain name
		domainEnd := strings.Index(path, "/")
		if domainEnd != -1 {
			// Extract just the path part
			path = path[domainEnd:]
		} else {
			// If no path found, use root path
			path = "/"
		}

		// Add paths to our result array
		paths = append(paths, path)
		log.Debug().Str("extracted_path", path).Msg("Extracted path from URL")
	}

	// Insert pages into database in a transaction
	err = jm.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		// Prepare statement for bulk insertion
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO pages (domain_id, path)
			VALUES ($1, $2)
			ON CONFLICT (domain_id, path) DO UPDATE SET path = EXCLUDED.path
			RETURNING id
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare page insert statement: %w", err)
		}
		defer stmt.Close()

		// Insert each page and collect the IDs
		for _, path := range paths {
			var pageID int
			err := stmt.QueryRowContext(ctx, domainID, path).Scan(&pageID)
			if err != nil {
				return fmt.Errorf("failed to insert page record: %w", err)
			}
			pageIDs = append(pageIDs, pageID)
		}

		return nil
	})

	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return nil, nil, fmt.Errorf("failed to create page records: %w", err)
	}

	log.Debug().
		Int("domain_id", domainID).
		Int("page_count", len(pageIDs)).
		Msg("Created page records")

	return pageIDs, paths, nil
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
	// Note: Only pass the domain, not the full URL with https://
	// as DiscoverSitemaps already adds the https:// prefix
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
		pageIDs, paths, err := jm.createPageRecords(ctx, domainID, urls)
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
		if err := jm.EnqueueJobURLs(ctx, jobID, pageIDs, paths, "sitemap", baseURL, 0); err != nil {
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
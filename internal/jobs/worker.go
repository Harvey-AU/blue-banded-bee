package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/observability"
	"github.com/Harvey-AU/blue-banded-bee/internal/util"
	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const taskProcessingTimeout = 2 * time.Minute
const poolSaturationBackoff = 2 * time.Second
const defaultJobFailureThreshold = 20

// JobPerformance tracks performance metrics for a specific job
type JobPerformance struct {
	RecentTasks  []int64   // Last 5 task response times for this job
	CurrentBoost int       // Current performance boost workers for this job
	LastCheck    time.Time // When we last evaluated this job
}

type WorkerPool struct {
	db               *sql.DB
	dbQueue          DbQueueInterface
	dbConfig         *db.Config
	crawler          CrawlerInterface
	domainLimiter    *DomainLimiter
	batchManager     *db.BatchManager // Batch manager for task updates
	numWorkers       int
	jobs             map[string]bool
	jobsMutex        sync.RWMutex
	stopCh           chan struct{}
	wg               sync.WaitGroup
	recoveryInterval time.Duration
	stopping         atomic.Bool
	activeJobs       sync.WaitGroup
	baseWorkerCount  int
	currentWorkers   int
	maxWorkers       int // Maximum workers allowed (environment-specific)
	workersMutex     sync.RWMutex
	cleanupInterval  time.Duration
	notifyCh         chan struct{}
	jobManager       *JobManager // Reference to JobManager for duplicate checking

	// Per-worker task concurrency
	workerConcurrency int               // How many tasks each worker can process concurrently
	workerSemaphores  []chan struct{}   // One semaphore per worker to limit concurrent tasks
	workerWaitGroups  []*sync.WaitGroup // One wait group per worker for graceful shutdown

	// Performance scaling
	jobPerformance map[string]*JobPerformance
	perfMutex      sync.RWMutex

	// Job info cache to avoid repeated DB lookups
	jobInfoCache map[string]*JobInfo
	jobInfoMutex sync.RWMutex

	// Job failure tracking
	jobFailureMutex     sync.Mutex
	jobFailureCounters  map[string]*jobFailureState
	jobFailureThreshold int
}

func (wp *WorkerPool) ensureDomainLimiter() *DomainLimiter {
	if wp.domainLimiter == nil {
		wp.domainLimiter = newDomainLimiter(wp.dbQueue)
	}
	return wp.domainLimiter
}

// JobInfo caches job-specific data that doesn't change during execution
type JobInfo struct {
	DomainID           int
	DomainName         string
	FindLinks          bool
	CrawlDelay         int
	Concurrency        int
	AdaptiveDelay      int
	AdaptiveDelayFloor int
	RobotsRules        *crawler.RobotsRules // Cached robots.txt rules for URL filtering
}

type jobFailureState struct {
	streak    int
	triggered bool
}

func jobFailureThresholdFromEnv() int {
	threshold := defaultJobFailureThreshold
	if raw := strings.TrimSpace(os.Getenv("BBB_JOB_FAILURE_THRESHOLD")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			threshold = parsed
		}
	}
	return threshold
}

func NewWorkerPool(sqlDB *sql.DB, dbQueue DbQueueInterface, crawler CrawlerInterface, numWorkers int, workerConcurrency int, dbConfig *db.Config) *WorkerPool {
	// Validate inputs
	if sqlDB == nil {
		panic("database connection is required")
	}
	if dbQueue == nil {
		panic("database queue is required")
	}
	if crawler == nil {
		panic("crawler is required")
	}
	if numWorkers < 1 {
		panic("numWorkers must be at least 1")
	}
	if workerConcurrency < 1 || workerConcurrency > 20 {
		panic("workerConcurrency must be between 1 and 20")
	}
	if dbConfig == nil {
		panic("database configuration is required")
	}

	// Determine max workers based on environment to prevent resource exhaustion
	maxWorkers := 50 // Production: high throughput
	if env := os.Getenv("APP_ENV"); env == "staging" {
		maxWorkers = 10 // Preview/staging: match conservative limits
	}

	// Create batch manager before WorkerPool construction (db package reference must happen here)
	batchMgr := db.NewBatchManager(dbQueue)
	domainLimiter := newDomainLimiter(dbQueue)

	// Initialise per-worker structures for concurrency control
	workerSemaphores := make([]chan struct{}, numWorkers)
	workerWaitGroups := make([]*sync.WaitGroup, numWorkers)
	for i := 0; i < numWorkers; i++ {
		workerSemaphores[i] = make(chan struct{}, workerConcurrency)
		workerWaitGroups[i] = &sync.WaitGroup{}
	}

	failureThreshold := jobFailureThresholdFromEnv()

	wp := &WorkerPool{
		db:              sqlDB,
		dbQueue:         dbQueue,
		dbConfig:        dbConfig,
		crawler:         crawler,
		domainLimiter:   domainLimiter,
		batchManager:    batchMgr,
		numWorkers:      numWorkers,
		baseWorkerCount: numWorkers,
		currentWorkers:  numWorkers,
		maxWorkers:      maxWorkers,
		jobs:            make(map[string]bool),

		stopCh:           make(chan struct{}),
		notifyCh:         make(chan struct{}, 1), // Buffer of 1 to prevent blocking
		recoveryInterval: 1 * time.Minute,
		cleanupInterval:  time.Minute,

		// Per-worker task concurrency
		workerConcurrency: workerConcurrency,
		workerSemaphores:  workerSemaphores,
		workerWaitGroups:  workerWaitGroups,

		// Performance scaling
		jobPerformance: make(map[string]*JobPerformance),

		// Job info cache
		jobInfoCache: make(map[string]*JobInfo),

		// Job failure tracking
		jobFailureCounters:  make(map[string]*jobFailureState),
		jobFailureThreshold: failureThreshold,
	}

	// Start the notification listener when we have connection details available.
	if hasNotificationConfig(dbConfig) {
		wp.wg.Add(1)
		go func() {
			defer wp.wg.Done()
			wp.listenForNotifications(context.Background())
		}()
	} else {
		log.Debug().Msg("Skipping LISTEN/NOTIFY setup: database config lacks connection details")
	}

	return wp
}

func (wp *WorkerPool) Start(ctx context.Context) {
	log.Info().Int("workers", wp.numWorkers).Msg("Starting worker pool")

	// Reconcile running_tasks counters before starting workers
	// This prevents capacity leaks from deployments, crashes, or migration timing
	reconcileCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := wp.reconcileRunningTaskCounters(reconcileCtx); err != nil {
		sentry.CaptureException(err)
		log.Error().Err(err).Msg("Failed to reconcile running_tasks counters - workers may be blocked")
		// Continue startup even if reconciliation fails (logged for monitoring)
	}

	for i := 0; i < wp.numWorkers; i++ {
		i := i
		wp.wg.Add(1)
		go func() {
			defer wp.wg.Done()
			wp.worker(ctx, i)
		}()
	}

	// Start the recovery monitor
	wp.wg.Add(1)
	go func() {
		defer wp.wg.Done()
		wp.recoveryMonitor(ctx)
	}()

	// Run initial cleanup
	if err := wp.CleanupStuckJobs(ctx); err != nil {
		sentry.CaptureException(err)
		log.Error().Err(err).Msg("Failed to perform initial job cleanup")
	}

	// Recover jobs that were running before restart
	if err := wp.recoverRunningJobs(ctx); err != nil {
		sentry.CaptureException(err)
		log.Error().Err(err).Msg("Failed to recover running jobs on startup")
	}

	wp.StartTaskMonitor(ctx)
	wp.StartCleanupMonitor(ctx)
}

func (wp *WorkerPool) Stop() {
	// Only stop once - use atomic compare-and-swap to ensure thread safety
	if wp.stopping.CompareAndSwap(false, true) {
		log.Debug().Msg("Stopping worker pool")
		close(wp.stopCh)
		wp.wg.Wait()
		// Stop batch manager to flush remaining updates
		if wp.batchManager != nil {
			wp.batchManager.Stop()
		}
		log.Debug().Msg("Worker pool stopped")
	}
}

// reconcileRunningTaskCounters resets running_tasks to match actual task status
// This fixes counter leaks from:
// - Deployment race conditions (tasks completing during graceful shutdown)
// - Crash recovery (batch manager unable to flush)
// - Migration backfill timing (tasks counted as running but completed before new code started)
func (wp *WorkerPool) reconcileRunningTaskCounters(ctx context.Context) error {
	log.Info().Msg("Reconciling running_tasks counters with actual task status")

	// Atomic query: Reset all running_tasks based on current task.status = 'running'
	// Returns jobs that had mismatched counters for observability
	query := `
		WITH actual_counts AS (
			SELECT
				job_id,
				COUNT(*) as actual_running
			FROM tasks
			WHERE status = 'running'
			GROUP BY job_id
		),
		reconciled_jobs AS (
			UPDATE jobs
			SET running_tasks = COALESCE(ac.actual_running, 0)
			FROM actual_counts ac
			WHERE jobs.id = ac.job_id
			  AND jobs.status IN ('running', 'pending')
			  AND jobs.running_tasks != COALESCE(ac.actual_running, 0)
			RETURNING
				jobs.id,
				jobs.running_tasks as old_value,
				COALESCE(ac.actual_running, 0) as new_value
		),
		zero_out_jobs AS (
			UPDATE jobs
			SET running_tasks = 0
			WHERE status IN ('running', 'pending')
			  AND running_tasks > 0
			  AND id NOT IN (SELECT job_id FROM actual_counts)
			RETURNING
				id,
				running_tasks as old_value,
				0 as new_value
		)
		SELECT
			id,
			old_value,
			new_value,
			old_value - new_value as leaked_tasks
		FROM (
			SELECT * FROM reconciled_jobs
			UNION ALL
			SELECT * FROM zero_out_jobs
		) combined
		ORDER BY leaked_tasks DESC
	`

	rows, err := wp.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to reconcile running_tasks counters: %w", err)
	}
	defer rows.Close()

	totalLeaked := 0
	jobsFixed := 0

	for rows.Next() {
		var jobID string
		var oldValue, newValue, leaked int

		if err := rows.Scan(&jobID, &oldValue, &newValue, &leaked); err != nil {
			log.Warn().Err(err).Msg("Failed to scan reconciliation result")
			continue
		}

		totalLeaked += leaked
		jobsFixed++

		log.Info().
			Str("job_id", jobID).
			Int("old_counter", oldValue).
			Int("actual_running", newValue).
			Int("leaked_tasks", leaked).
			Msg("Reconciled running_tasks counter")
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error reading reconciliation results: %w", err)
	}

	if totalLeaked > 0 {
		log.Warn().
			Int("total_leaked_tasks", totalLeaked).
			Int("jobs_fixed", jobsFixed).
			Msg("Running_tasks counters reconciled - capacity leak detected and fixed")
	} else {
		log.Info().Msg("Running_tasks counters already accurate - no reconciliation needed")
	}

	return nil
}

// WaitForJobs waits for all active jobs to complete
func (wp *WorkerPool) WaitForJobs() {
	wp.activeJobs.Wait()
}

func (wp *WorkerPool) AddJob(jobID string, options *JobOptions) {
	wp.jobsMutex.Lock()
	wp.jobs[jobID] = true
	wp.jobsMutex.Unlock()

	// Initialise performance tracking for this job
	wp.perfMutex.Lock()
	wp.jobPerformance[jobID] = &JobPerformance{
		RecentTasks:  make([]int64, 0, 5),
		CurrentBoost: 0,
		LastCheck:    time.Now(),
	}
	wp.perfMutex.Unlock()

	wp.jobFailureMutex.Lock()
	wp.jobFailureCounters[jobID] = &jobFailureState{}
	wp.jobFailureMutex.Unlock()

	// Cache job info to avoid repeated database lookups
	ctx := context.Background()
	var domainName string
	var crawlDelay sql.NullInt64
	var dbFindLinks bool

	// When options is nil (recovery mode), fetch find_links from DB
	// Otherwise use the provided options value
	var domainID int
	var adaptiveDelay sql.NullInt64
	var adaptiveFloor sql.NullInt64
	var dbConcurrency int
	err := wp.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		query := `
			SELECT d.id, d.name, d.crawl_delay_seconds, d.adaptive_delay_seconds, d.adaptive_delay_floor_seconds, j.find_links, j.concurrency
			FROM domains d
			JOIN jobs j ON j.domain_id = d.id
			WHERE j.id = $1
		`
		return tx.QueryRowContext(ctx, query, jobID).Scan(&domainID, &domainName, &crawlDelay, &adaptiveDelay, &adaptiveFloor, &dbFindLinks, &dbConcurrency)
	})

	if err == nil {
		// Use DB value when options is nil (recovery), otherwise use provided value
		findLinks := dbFindLinks
		if options != nil {
			findLinks = options.FindLinks
		}
		concurrency := dbConcurrency
		if options != nil && options.Concurrency > 0 {
			concurrency = options.Concurrency
		}

		jobInfo := &JobInfo{
			DomainID:    domainID,
			DomainName:  domainName,
			FindLinks:   findLinks,
			CrawlDelay:  0,
			Concurrency: concurrency,
		}
		if crawlDelay.Valid {
			jobInfo.CrawlDelay = int(crawlDelay.Int64)
		}
		if adaptiveDelay.Valid {
			jobInfo.AdaptiveDelay = int(adaptiveDelay.Int64)
		}
		if adaptiveFloor.Valid {
			jobInfo.AdaptiveDelayFloor = int(adaptiveFloor.Int64)
		}
		wp.ensureDomainLimiter().Seed(domainName, jobInfo.CrawlDelay, jobInfo.AdaptiveDelay, jobInfo.AdaptiveDelayFloor)

		// Parse robots.txt to get filtering rules
		robotsRules, err := crawler.ParseRobotsTxt(ctx, domainName, wp.crawler.GetUserAgent())
		if err != nil {
			log.Debug().
				Err(err).
				Str("domain", domainName).
				Msg("Failed to parse robots.txt, proceeding without restrictions")
			// Only capture to Sentry if it's not a 404 (which is normal)
			if !strings.Contains(err.Error(), "404") {
				sentry.CaptureMessage(fmt.Sprintf("Failed to parse robots.txt for %s: %v", domainName, err))
			}
			jobInfo.RobotsRules = &crawler.RobotsRules{} // Empty rules = no restrictions
			if robotsRules.CrawlDelay > 0 {
				jobInfo.CrawlDelay = robotsRules.CrawlDelay
				wp.ensureDomainLimiter().UpdateRobotsDelay(domainName, jobInfo.CrawlDelay)
			}
		} else {
			jobInfo.RobotsRules = robotsRules
		}

		wp.jobInfoMutex.Lock()
		wp.jobInfoCache[jobID] = jobInfo
		wp.jobInfoMutex.Unlock()

		log.Debug().
			Str("job_id", jobID).
			Str("domain", domainName).
			Int("crawl_delay", jobInfo.CrawlDelay).
			Int("concurrency", jobInfo.Concurrency).
			Int("disallow_patterns", len(jobInfo.RobotsRules.DisallowPatterns)).
			Msg("Cached job info with robots rules")
	} else {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to cache job info")
		sentry.CaptureException(fmt.Errorf("failed to cache job info for job %s: %w", jobID, err))
	}

	// Simple scaling: add 5 workers per job, respect environment-specific max
	wp.workersMutex.Lock()
	targetWorkers := min(wp.currentWorkers+5, wp.maxWorkers)

	if targetWorkers > wp.currentWorkers {
		wp.workersMutex.Unlock()
		wp.scaleWorkers(context.Background(), targetWorkers)
	} else {
		wp.workersMutex.Unlock()
	}

	log.Debug().
		Str("job_id", jobID).
		Int("current_workers", wp.currentWorkers).
		Int("target_workers", targetWorkers).
		Msg("Added job to worker pool")
}

func (wp *WorkerPool) RemoveJob(jobID string) {
	wp.jobsMutex.Lock()
	delete(wp.jobs, jobID)
	wp.jobsMutex.Unlock()

	// Remove performance boost for this job
	wp.perfMutex.Lock()
	var jobBoost int
	if perf, exists := wp.jobPerformance[jobID]; exists {
		jobBoost = perf.CurrentBoost
		delete(wp.jobPerformance, jobID)
	}
	wp.perfMutex.Unlock()

	// Remove from job info cache
	wp.jobInfoMutex.Lock()
	delete(wp.jobInfoCache, jobID)
	wp.jobInfoMutex.Unlock()

	wp.jobFailureMutex.Lock()
	delete(wp.jobFailureCounters, jobID)
	wp.jobFailureMutex.Unlock()

	// Simple scaling: remove 5 workers per job + any performance boost, minimum of base count
	wp.workersMutex.Lock()
	targetWorkers := max(wp.currentWorkers-5-jobBoost, wp.baseWorkerCount)

	log.Debug().
		Str("job_id", jobID).
		Int("current_workers", wp.currentWorkers).
		Int("target_workers", targetWorkers).
		Int("job_boost_removed", jobBoost).
		Msg("Scaling down worker pool")

	wp.currentWorkers = targetWorkers
	// Note: We don't actually stop excess workers, they'll exit on next task completion
	wp.workersMutex.Unlock()

	log.Debug().
		Str("job_id", jobID).
		Msg("Removed job from worker pool")
}

func (wp *WorkerPool) resetJobFailureStreak(jobID string) {
	if jobID == "" || wp.jobFailureThreshold <= 0 {
		return
	}

	wp.jobFailureMutex.Lock()
	if state, ok := wp.jobFailureCounters[jobID]; ok && !state.triggered {
		state.streak = 0
	}
	wp.jobFailureMutex.Unlock()
}

func (wp *WorkerPool) recordJobFailure(ctx context.Context, jobID, taskID string, taskErr error) {
	if jobID == "" || wp.jobFailureThreshold <= 0 {
		return
	}

	wp.jobsMutex.RLock()
	_, active := wp.jobs[jobID]
	wp.jobsMutex.RUnlock()
	if !active {
		return
	}

	var (
		streak    int
		triggered bool
	)

	wp.jobFailureMutex.Lock()
	state, ok := wp.jobFailureCounters[jobID]
	if !ok {
		state = &jobFailureState{}
		wp.jobFailureCounters[jobID] = state
	}
	if !state.triggered {
		state.streak++
		streak = state.streak
		if state.streak >= wp.jobFailureThreshold {
			state.triggered = true
			triggered = true
		}
	}
	wp.jobFailureMutex.Unlock()

	if triggered {
		wp.markJobFailedDueToConsecutiveFailures(ctx, jobID, streak, taskErr)
		return
	}

	if streak > 0 {
		log.Debug().
			Str("job_id", jobID).
			Str("task_id", taskID).
			Int("failure_streak", streak).
			Int("threshold", wp.jobFailureThreshold).
			Msg("Incremented job failure streak")
	}
}

func (wp *WorkerPool) markJobFailedDueToConsecutiveFailures(ctx context.Context, jobID string, streak int, lastErr error) {
	message := fmt.Sprintf("Job failed after %d consecutive task failures", streak)
	if lastErr != nil {
		message = fmt.Sprintf("%s (last error: %s)", message, lastErr.Error())
	}

	log.Error().
		Str("job_id", jobID).
		Int("failure_streak", streak).
		Int("threshold", wp.jobFailureThreshold).
		Msg(message)

	failCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	updateErr := wp.dbQueue.Execute(failCtx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(failCtx, `
			UPDATE jobs
			SET status = $1,
				completed_at = COALESCE(completed_at, $2),
				error_message = $3
			WHERE id = $4
				AND status <> $5
				AND status <> $6
		`, JobStatusFailed, time.Now().UTC(), message, jobID, JobStatusFailed, JobStatusCancelled)
		return err
	})

	if updateErr != nil {
		log.Error().Err(updateErr).Str("job_id", jobID).Msg("Failed to mark job as failed after consecutive task failures")
	}

	wp.RemoveJob(jobID)
}

func (wp *WorkerPool) worker(ctx context.Context, workerID int) {
	log.Info().
		Int("worker_id", workerID).
		Int("concurrency", wp.workerConcurrency).
		Msg("Starting worker")

	// Record worker capacity once at startup
	observability.RecordWorkerConcurrency(ctx, workerID, 0, int64(wp.workerConcurrency))

	// Get this worker's semaphore and wait group
	sem := wp.workerSemaphores[workerID]
	wg := wp.workerWaitGroups[workerID]

	// Create a context for this worker that we can cancel on exit
	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()

	// Track consecutive no-task counts for backoff (only in main goroutine)
	consecutiveNoTasks := 0
	maxSleep := 5 * time.Second         // Note: Changed from 30 to 5 seconds, to increase responsiveness when inactive.
	baseSleep := 200 * time.Millisecond // Faster processing when active

	// Channel to receive task results from concurrent goroutines
	type taskResult struct {
		err error
	}
	resultsCh := make(chan taskResult, wp.workerConcurrency)

	// Wait for all in-flight tasks to complete, then drain results channel
	defer func() {
		wg.Wait() // Wait for all task goroutines to complete
		// Now drain any remaining results
		for {
			select {
			case <-resultsCh:
				// Discard
			default:
				return
			}
		}
	}()

	for {
		// Check for stop signals or notifications
		select {
		case <-wp.stopCh:
			log.Debug().Int("worker_id", workerID).Msg("Worker received stop signal")
			return
		case <-ctx.Done():
			log.Debug().Int("worker_id", workerID).Msg("Worker context cancelled")
			return
		case <-wp.notifyCh:
			// Reset backoff when notified of new tasks
			consecutiveNoTasks = 0
		case result := <-resultsCh:
			// Process result from concurrent task goroutine
			if result.err != nil {
				if errors.Is(result.err, sql.ErrNoRows) {
					consecutiveNoTasks++
				} else {
					log.Error().Err(result.err).Int("worker_id", workerID).Msg("Task processing failed")
				}
			} else {
				consecutiveNoTasks = 0
			}
			continue
		default:
			// Continue to claim tasks
		}

		// Check if this worker should exit (we've scaled down)
		wp.workersMutex.RLock()
		shouldExit := workerID >= wp.currentWorkers
		wp.workersMutex.RUnlock()

		if shouldExit {
			return
		}

		// Apply backoff logic when no tasks are available
		if consecutiveNoTasks > 0 {
			// Only log occasionally during quiet periods
			if consecutiveNoTasks == 1 || consecutiveNoTasks%10 == 0 {
				log.Debug().Int("consecutive_no_tasks", consecutiveNoTasks).Msg("Waiting for new tasks")
			}
			// Exponential backoff with a maximum, plus jitter to prevent thundering herd
			baseSleepTime := min(time.Duration(float64(baseSleep)*math.Pow(1.5, float64(min(consecutiveNoTasks, 10)))), maxSleep)
			jitter := time.Duration(rand.Int63n(2000)) * time.Millisecond // 0-2s jitter
			sleepTime := baseSleepTime + jitter

			// Wait for either the backoff duration, a notification, or task completion
			select {
			case <-time.After(sleepTime):
				consecutiveNoTasks = 0 // Reset backoff to retry claiming
			case <-wp.notifyCh:
				consecutiveNoTasks = 0
			case result := <-resultsCh:
				if result.err != nil {
					if errors.Is(result.err, sql.ErrNoRows) {
						consecutiveNoTasks++
					}
				} else {
					consecutiveNoTasks = 0
				}
			case <-wp.stopCh:
				return
			case <-ctx.Done():
				return
			}
			continue // Loop back to check signals before attempting to claim
		}

		// Try to acquire a semaphore slot (non-blocking)
		select {
		case sem <- struct{}{}:
			// Successfully acquired slot, launch goroutine to process task
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					<-sem // Release semaphore slot
					observability.RecordWorkerConcurrency(workerCtx, workerID, -1, 0)
				}()

				// Record task start
				observability.RecordWorkerConcurrency(workerCtx, workerID, +1, 0)

				err := wp.processNextTask(workerCtx)

				// Non-blocking send to prevent shutdown hang
				select {
				case resultsCh <- taskResult{err: err}:
				case <-workerCtx.Done():
					// Worker cancelled, don't block (covers both worker exit and parent context)
				}
			}()
		default:
			// All slots full, wait briefly for a slot to free up or check for results
			select {
			case <-time.After(baseSleep):
			case result := <-resultsCh:
				if result.err != nil {
					if errors.Is(result.err, sql.ErrNoRows) {
						consecutiveNoTasks++
					}
				} else {
					consecutiveNoTasks = 0
				}
			case <-wp.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}
}

// claimPendingTask attempts to claim a pending task from any active job
func (wp *WorkerPool) claimPendingTask(ctx context.Context) (*db.Task, error) {
	// Get the list of active jobs
	wp.jobsMutex.RLock()
	activeJobs := make([]string, 0, len(wp.jobs))
	for jobID := range wp.jobs {
		activeJobs = append(activeJobs, jobID)
	}
	wp.jobsMutex.RUnlock()

	// If no active jobs, return immediately
	if len(activeJobs) == 0 {
		return nil, sql.ErrNoRows
	}

	// Try to get a task from each active job
	for _, jobID := range activeJobs {
		task, err := wp.dbQueue.GetNextTask(ctx, jobID)
		if err == sql.ErrNoRows {
			continue // Try next job
		}
		if errors.Is(err, db.ErrPoolSaturated) {
			// Pool saturated - treat like no tasks available and back off
			return nil, sql.ErrNoRows
		}
		if err != nil {
			log.Error().Err(err).Str("job_id", jobID).Msg("Error getting next pending task")
			return nil, err // Return actual errors
		}
		if task != nil {
			log.Info().
				Str("task_id", task.ID).
				Str("job_id", task.JobID).
				Int("page_id", task.PageID).
				Str("path", task.Path).
				Float64("priority", task.PriorityScore).
				Msg("Found and claimed pending task")
			return task, nil
		}
	}

	// No tasks found in any job
	return nil, sql.ErrNoRows
}

// prepareTaskForProcessing converts db.Task to jobs.Task and enriches with job info
func (wp *WorkerPool) prepareTaskForProcessing(ctx context.Context, task *db.Task) (*Task, error) {
	// Convert db.Task to jobs.Task for processing
	jobsTask := &Task{
		ID:            task.ID,
		JobID:         task.JobID,
		PageID:        task.PageID,
		Path:          task.Path,
		Status:        TaskStatus(task.Status),
		CreatedAt:     task.CreatedAt,
		StartedAt:     task.StartedAt,
		RetryCount:    task.RetryCount,
		SourceType:    task.SourceType,
		SourceURL:     task.SourceURL,
		PriorityScore: task.PriorityScore,
	}

	// Get job info from cache
	wp.jobInfoMutex.RLock()
	jobInfo, exists := wp.jobInfoCache[task.JobID]
	wp.jobInfoMutex.RUnlock()

	if exists {
		jobsTask.DomainID = jobInfo.DomainID
		jobsTask.DomainName = jobInfo.DomainName
		jobsTask.FindLinks = jobInfo.FindLinks
		jobsTask.CrawlDelay = jobInfo.CrawlDelay
		jobsTask.JobConcurrency = jobInfo.Concurrency
		jobsTask.AdaptiveDelay = jobInfo.AdaptiveDelay
		jobsTask.AdaptiveDelayFloor = jobInfo.AdaptiveDelayFloor
	} else {
		// Fallback to database if not in cache (shouldn't happen normally)
		log.Warn().Str("job_id", task.JobID).Msg("Job info not in cache, querying database")

		var domainID int
		var domainName string
		var findLinks bool
		var crawlDelay sql.NullInt64
		var adaptiveDelay sql.NullInt64
		var adaptiveFloor sql.NullInt64
		var concurrency int
		err := wp.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
			return tx.QueryRowContext(ctx, `
				SELECT d.id, d.name, j.find_links, d.crawl_delay_seconds, d.adaptive_delay_seconds, d.adaptive_delay_floor_seconds, j.concurrency
				FROM domains d
				JOIN jobs j ON j.domain_id = d.id
				WHERE j.id = $1
			`, task.JobID).Scan(&domainID, &domainName, &findLinks, &crawlDelay, &adaptiveDelay, &adaptiveFloor, &concurrency)
		})

		if err != nil {
			log.Error().Err(err).Str("job_id", task.JobID).Msg("Failed to get domain info")
		} else {
			jobsTask.DomainID = domainID
			jobsTask.DomainName = domainName
			jobsTask.FindLinks = findLinks
			if crawlDelay.Valid {
				jobsTask.CrawlDelay = int(crawlDelay.Int64)
			}
			jobsTask.JobConcurrency = concurrency
			if adaptiveDelay.Valid {
				jobsTask.AdaptiveDelay = int(adaptiveDelay.Int64)
			}
			if adaptiveFloor.Valid {
				jobsTask.AdaptiveDelayFloor = int(adaptiveFloor.Int64)
			}
			wp.ensureDomainLimiter().Seed(domainName, jobsTask.CrawlDelay, jobsTask.AdaptiveDelay, jobsTask.AdaptiveDelayFloor)
		}
	}
	if jobsTask.JobConcurrency <= 0 {
		jobsTask.JobConcurrency = 1
	}

	return jobsTask, nil
}

func (wp *WorkerPool) processNextTask(ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			recoveredErr := fmt.Errorf("panic in processNextTask: %v", r)
			if hub := sentry.CurrentHub(); hub != nil {
				hub.Recover(r)
			} else {
				sentry.CaptureException(recoveredErr)
			}
			log.Error().
				Interface("panic", r).
				Bytes("stack", stack).
				Msg("Recovered from panic in processNextTask")
			if err == nil {
				err = recoveredErr
			}
		}
	}()

	// Claim a pending task from active jobs
	task, err := wp.claimPendingTask(ctx)
	if err != nil {
		return err
	}
	if task != nil {
		// Prepare task for processing with job info
		jobsTask, err := wp.prepareTaskForProcessing(ctx, task)
		if err != nil {
			log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to prepare task")
			return err
		}

		// Process the task
		taskCtx, cancel := context.WithTimeout(ctx, taskProcessingTimeout)
		defer cancel()

		result, err := wp.processTask(taskCtx, jobsTask)
		if err != nil {
			return wp.handleTaskError(ctx, task, err)
		} else {
			return wp.handleTaskSuccess(ctx, task, result)
		}
	}

	// No tasks found in any active jobs
	return sql.ErrNoRows
}

// EnqueueURLs is a wrapper that ensures all task enqueuing goes through the JobManager.
// This allows for centralised logic, such as duplicate checking, to be applied.
func (wp *WorkerPool) EnqueueURLs(ctx context.Context, jobID string, pages []db.Page, sourceType string, sourceURL string) error {
	log.Debug().
		Str("job_id", jobID).
		Str("source_type", sourceType).
		Int("url_count", len(pages)).
		Msg("EnqueueURLs called via WorkerPool, passing to JobManager")

	// The jobManager must be set for the worker pool to function correctly.
	if wp.jobManager == nil {
		err := fmt.Errorf("jobManager is not set on WorkerPool - cannot enqueue URLs")
		log.Error().Err(err).Msg("Failed to enqueue URLs")
		return err
	}

	if err := wp.jobManager.EnqueueJobURLs(ctx, jobID, pages, sourceType, sourceURL); err != nil {
		if errors.Is(err, db.ErrPoolSaturated) {
			log.Warn().
				Str("job_id", jobID).
				Str("source_type", sourceType).
				Msg("Database pool saturated while enqueueing URLs; backing off")
			time.Sleep(poolSaturationBackoff)
		}
		return err
	}

	return nil
}

// StartTaskMonitor starts a background process that monitors for pending tasks
func (wp *WorkerPool) StartTaskMonitor(ctx context.Context) {
	log.Info().Msg("Starting task monitor to check for pending tasks")
	wp.wg.Add(1)
	go func() {
		defer wp.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("Task monitor stopped due to context cancellation")
				return
			case <-wp.stopCh:
				log.Info().Msg("Task monitor stopped due to stop signal")
				return
			case <-ticker.C:
				log.Debug().Msg("Task monitor checking for pending tasks")
				if err := wp.checkForPendingTasks(ctx); err != nil {
					log.Error().Err(err).Msg("Error checking for pending tasks")
				}
			}
		}
	}()

	log.Info().Msg("Task monitor started successfully")
}

// checkForPendingTasks looks for any pending tasks and adds their jobs to the pool
func (wp *WorkerPool) checkForPendingTasks(ctx context.Context) error {
	log.Debug().Msg("Checking database for jobs with pending tasks")

	var jobIDs []string
	err := wp.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		// Query for jobs with pending tasks
		rows, err := tx.QueryContext(ctx, `
			SELECT DISTINCT job_id FROM tasks 
			WHERE status = $1 
			LIMIT 100
		`, TaskStatusPending)

		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var jobID string
			if err := rows.Scan(&jobID); err != nil {
				return err
			}
			jobIDs = append(jobIDs, jobID)
		}
		return rows.Err()
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to query for jobs with pending tasks")
		return err
	}

	jobsFound := len(jobIDs)
	foundIDs := jobIDs
	// For each job with pending tasks, add it to the worker pool
	for _, jobID := range jobIDs {
		// Check if already in our active jobs
		wp.jobsMutex.RLock()
		active := wp.jobs[jobID]
		wp.jobsMutex.RUnlock()

		if !active {
			// Add job to the worker pool
			log.Info().Str("job_id", jobID).Msg("Adding job with pending tasks to worker pool")

			// Get job options
			var findLinks bool
			err := wp.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
				return tx.QueryRowContext(ctx, `
					SELECT find_links FROM jobs WHERE id = $1
				`, jobID).Scan(&findLinks)
			})

			if err != nil {
				log.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job options")
				continue
			}

			options := &JobOptions{
				FindLinks: findLinks,
			}

			wp.AddJob(jobID, options)

			// Update job status if needed
			err = wp.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `
					UPDATE jobs SET
						status = $1,
						started_at = CASE WHEN started_at IS NULL THEN $2 ELSE started_at END
					WHERE id = $3 AND status = $4
				`, JobStatusRunning, time.Now().UTC(), jobID, JobStatusPending)
				return err
			})

			if err != nil {
				log.Error().Err(err).Str("job_id", jobID).Msg("Failed to update job status")
			} else {
				log.Info().Str("job_id", jobID).Msg("Updated job status to running")
			}
		} else {
			log.Debug().Str("job_id", jobID).Msg("Job already active in worker pool")
		}
	}

	if jobsFound == 0 {
		log.Debug().Msg("No jobs with pending tasks found")
	} else {
		log.Debug().Int("count", jobsFound).Msg("Found jobs with pending tasks")
	}

	foundSet := make(map[string]struct{}, len(foundIDs))
	for _, id := range foundIDs {
		foundSet[id] = struct{}{}
	}
	var toRemove []string
	wp.jobsMutex.RLock()
	for jobID := range wp.jobs {
		if _, ok := foundSet[jobID]; !ok {
			toRemove = append(toRemove, jobID)
		}
	}
	wp.jobsMutex.RUnlock()
	for _, id := range toRemove {
		log.Info().Str("job_id", id).Msg("Job has no pending tasks, removing from worker pool")

		// Check if job actually completed or is stuck
		var status string
		err := wp.dbQueue.Execute(context.Background(), func(tx *sql.Tx) error {
			return tx.QueryRow("SELECT status FROM jobs WHERE id = $1", id).Scan(&status)
		})
		if err == nil && status != "completed" {
			// Job removed but not completed - potential issue
			sentry.CaptureMessage(fmt.Sprintf("Job %s removed from pool without completion (status: %s)", id, status))
		}

		wp.RemoveJob(id)
	}

	return nil
}

// SetJobManager sets the JobManager reference for duplicate task checking
func (wp *WorkerPool) SetJobManager(jm *JobManager) {
	wp.jobManager = jm
}

// recoverStaleTasks checks for and resets stale tasks in batches
// Processes oldest tasks first, handles cancelled/failed jobs separately
func (wp *WorkerPool) recoverStaleTasks(ctx context.Context) error {
	const batchSize = 100
	staleTime := time.Now().UTC().Add(-TaskStaleTimeout)

	// STEP 1: Handle tasks from cancelled/failed jobs separately
	// These should be marked as failed immediately, not retried
	cancelledCount, err := wp.recoverTasksFromDeadJobs(ctx, staleTime)
	if err != nil {
		log.Error().Err(err).Msg("Failed to recover tasks from cancelled/failed jobs")
		// Don't fail the whole recovery, continue to running jobs
	} else if cancelledCount > 0 {
		log.Info().
			Int64("tasks_marked_failed", cancelledCount).
			Msg("Marked stuck tasks from cancelled/failed jobs as failed")
	}

	// STEP 2: Process tasks from running jobs in batches
	totalRecovered := 0
	totalFailed := 0
	batchNum := 0
	consecutiveFailures := 0
	const maxConsecutiveFailures = 5

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		batchNum++
		recovered, failed, err := wp.recoverStaleBatch(ctx, staleTime, batchSize, batchNum)

		if err != nil {
			consecutiveFailures++
			log.Warn().
				Err(err).
				Int("batch_num", batchNum).
				Int("consecutive_failures", consecutiveFailures).
				Msg("Batch recovery failed")

			// If we've failed too many times in a row, bail out
			if consecutiveFailures >= maxConsecutiveFailures {
				log.Error().
					Int("max_failures", maxConsecutiveFailures).
					Msg("Recovery failed after max consecutive failures, will retry next cycle")
				return fmt.Errorf("recovery failed after %d consecutive batch failures: %w", maxConsecutiveFailures, err)
			}

			// Exponential backoff between failed batches
			backoff := time.Duration(consecutiveFailures) * time.Second
			log.Debug().
				Dur("backoff", backoff).
				Msg("Sleeping before retrying next batch")
			time.Sleep(backoff)
			continue
		}

		// Success - reset failure counter
		consecutiveFailures = 0
		totalRecovered += recovered
		totalFailed += failed

		// If we processed fewer than batchSize, we're done
		if recovered+failed < batchSize {
			break
		}

		// Small delay between batches to avoid overwhelming the database
		time.Sleep(100 * time.Millisecond)
	}

	if totalRecovered > 0 || totalFailed > 0 {
		log.Info().
			Int("tasks_recovered", totalRecovered).
			Int("tasks_failed", totalFailed).
			Int("batches_processed", batchNum).
			Msg("Completed stale task recovery")

		if err := wp.reconcileRunningTaskCounters(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to reconcile running task counters after stale task recovery")
		}
	}

	return nil
}

// recoverTasksFromDeadJobs marks tasks from cancelled/failed jobs as failed
func (wp *WorkerPool) recoverTasksFromDeadJobs(ctx context.Context, staleTime time.Time) (int64, error) {
	var totalAffected int64

	// Process in batches to avoid transaction timeout
	for {
		var affected int64
		err := wp.dbQueue.ExecuteMaintenance(ctx, func(tx *sql.Tx) error {
			result, err := tx.ExecContext(ctx, `
				UPDATE tasks
				SET status = $1,
					error = $2,
					completed_at = $3
				FROM jobs j
				WHERE tasks.job_id = j.id
					AND tasks.status = $4
					AND tasks.started_at < $5
					AND j.status IN ($6, $7)
					AND tasks.id IN (
						SELECT t.id
						FROM tasks t
						JOIN jobs j2 ON t.job_id = j2.id
						WHERE t.status = $4
							AND t.started_at < $5
							AND j2.status IN ($6, $7)
						ORDER BY t.started_at ASC
						LIMIT 100
					)
			`, TaskStatusFailed, "Job was cancelled or failed", time.Now().UTC(),
				TaskStatusRunning, staleTime, JobStatusCancelled, JobStatusFailed)

			if err != nil {
				return err
			}

			affected, err = result.RowsAffected()
			return err
		})

		if err != nil {
			return totalAffected, err
		}

		totalAffected += affected

		// If we updated fewer than 100, we're done
		if affected < 100 {
			break
		}
	}

	return totalAffected, nil
}

// recoverStaleBatch processes one batch of stale tasks
func (wp *WorkerPool) recoverStaleBatch(ctx context.Context, staleTime time.Time, batchSize int, batchNum int) (recovered int, failed int, err error) {
	err = wp.dbQueue.ExecuteMaintenance(ctx, func(tx *sql.Tx) error {
		// Query for one batch of stale tasks, oldest first
		// Note: We recover stuck tasks regardless of job status to prevent tasks
		// from being orphaned when jobs are marked completed/cancelled/failed
		rows, err := tx.QueryContext(ctx, `
			SELECT t.id, t.retry_count, t.job_id
			FROM tasks t
			WHERE t.status = $1
				AND t.started_at < $2
			ORDER BY t.started_at ASC
			LIMIT $3
		`, TaskStatusRunning, staleTime, batchSize)

		if err != nil {
			return err
		}
		defer rows.Close()

		type staleTask struct {
			id         string
			retryCount int
			jobID      string
		}

		var tasks []staleTask
		for rows.Next() {
			var task staleTask
			if err := rows.Scan(&task.id, &task.retryCount, &task.jobID); err != nil {
				log.Warn().Err(err).Msg("Failed to scan stale task row")
				continue
			}
			tasks = append(tasks, task)
		}

		if err := rows.Err(); err != nil {
			return err
		}

		if len(tasks) == 0 {
			return nil // No tasks to process
		}

		log.Debug().
			Int("batch_num", batchNum).
			Int("batch_size", len(tasks)).
			Msg("Processing stale task batch")

		// Update tasks in this batch
		now := time.Now().UTC()
		for _, task := range tasks {
			if task.retryCount >= MaxTaskRetries {
				_, err = tx.ExecContext(ctx, `
					UPDATE tasks
					SET status = $1,
						error = $2,
						completed_at = $3
					WHERE id = $4
				`, TaskStatusFailed, "Max retries exceeded", now, task.id)

				if err != nil {
					log.Warn().
						Err(err).
						Str("task_id", task.id).
						Msg("Failed to mark task as failed")
					return err
				}
				failed++
			} else {
				_, err = tx.ExecContext(ctx, `
					UPDATE tasks
					SET status = $1,
						started_at = NULL,
						retry_count = retry_count + 1
					WHERE id = $2
				`, TaskStatusPending, task.id)

				if err != nil {
					log.Warn().
						Err(err).
						Str("task_id", task.id).
						Msg("Failed to reset task to pending")
					return err
				}
				recovered++
			}
		}

		log.Debug().
			Int("batch_num", batchNum).
			Int("recovered", recovered).
			Int("failed", failed).
			Msg("Completed batch recovery")

		return nil
	})

	return recovered, failed, err
}

// recoverRunningJobs finds jobs that were in 'running' state when the server shut down
// and resets their 'running' tasks to 'pending', then adds them to the worker pool
func (wp *WorkerPool) recoverRunningJobs(ctx context.Context) error {
	log.Info().Msg("Recovering jobs that were running before restart")

	var jobIDs []string
	err := wp.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		// Find jobs with 'running' status that have 'running' tasks
		rows, err := tx.QueryContext(ctx, `
			SELECT DISTINCT j.id
			FROM jobs j
			JOIN tasks t ON j.id = t.job_id
			WHERE j.status = $1
			AND t.status = $2
		`, JobStatusRunning, TaskStatusRunning)

		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var jobID string
			if err := rows.Scan(&jobID); err != nil {
				return err
			}
			jobIDs = append(jobIDs, jobID)
		}

		return rows.Err()
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to query for running jobs with running tasks")
		return err
	}

	var recoveredJobs []string
	for _, jobID := range jobIDs {

		// Reset running tasks to pending for this job
		err := wp.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
			result, err := tx.ExecContext(ctx, `
				UPDATE tasks 
				SET status = $1,
					started_at = NULL,
					retry_count = retry_count + 1
				WHERE job_id = $2 
				AND status = $3
			`, TaskStatusPending, jobID, TaskStatusRunning)

			if err != nil {
				return err
			}

			rowsAffected, _ := result.RowsAffected()
			log.Info().
				Str("job_id", jobID).
				Int64("tasks_reset", rowsAffected).
				Msg("Reset running tasks to pending")

			return nil
		})

		if err != nil {
			log.Error().Err(err).Str("job_id", jobID).Msg("Failed to reset running tasks")
			continue
		}

		// Add job back to worker pool
		wp.AddJob(jobID, nil)
		recoveredJobs = append(recoveredJobs, jobID)

		log.Info().Str("job_id", jobID).Msg("Recovered running job and added to worker pool")
	}

	if len(recoveredJobs) > 0 {
		log.Info().
			Int("count", len(recoveredJobs)).
			Strs("job_ids", recoveredJobs).
			Msg("Successfully recovered running jobs from restart")

		if err := wp.reconcileRunningTaskCounters(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to reconcile running task counters after job recovery")
		}
	} else {
		log.Debug().Msg("No running jobs found to recover")
	}

	return nil
}

// recoveryMonitor periodically checks for and recovers stale tasks
func (wp *WorkerPool) recoveryMonitor(ctx context.Context) {
	ticker := time.NewTicker(wp.recoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-wp.stopCh:
			return
		case <-ticker.C:
			if err := wp.recoverStaleTasks(ctx); err != nil {
				log.Error().Err(err).Msg("Failed to recover stale tasks")
			}
		}
	}
}

// scaleWorkers increases the worker pool size to the target number
func (wp *WorkerPool) scaleWorkers(ctx context.Context, targetWorkers int) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().
				Interface("panic", r).
				Str("stack", string(debug.Stack())).
				Msg("Recovered from panic in scaleWorkers")
		}
	}()

	wp.workersMutex.Lock()
	defer wp.workersMutex.Unlock()

	if targetWorkers <= wp.currentWorkers {
		return // No need to scale up
	}

	workersToAdd := targetWorkers - wp.currentWorkers

	log.Debug().
		Int("current_workers", wp.currentWorkers).
		Int("adding_workers", workersToAdd).
		Int("target_workers", targetWorkers).
		Msg("Scaling worker pool")

	// Initialise semaphores and wait groups for new workers
	for i := 0; i < workersToAdd; i++ {
		workerID := wp.currentWorkers + i

		// Extend slices if needed
		if workerID >= len(wp.workerSemaphores) {
			wp.workerSemaphores = append(wp.workerSemaphores, make(chan struct{}, wp.workerConcurrency))
			wp.workerWaitGroups = append(wp.workerWaitGroups, &sync.WaitGroup{})
		}

		// Start worker
		wp.wg.Add(1)
		go func(id int) {
			defer wp.wg.Done()
			wp.worker(ctx, id)
		}(workerID)
	}

	wp.currentWorkers = targetWorkers
}

// StartCleanupMonitor starts the cleanup monitor goroutine
func (wp *WorkerPool) StartCleanupMonitor(ctx context.Context) {
	wp.wg.Add(1)
	go func() {
		defer wp.wg.Done()
		ticker := time.NewTicker(wp.cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-wp.stopCh:
				return
			case <-ticker.C:
				if err := wp.CleanupStuckJobs(ctx); err != nil {
					log.Error().Err(err).Msg("Failed to cleanup stuck jobs")
				}
			}
		}
	}()
	log.Info().Msg("Job cleanup monitor started")
}

// CleanupStuckJobs finds and fixes jobs that are stuck in pending/running state
// despite having all their tasks completed
func (wp *WorkerPool) CleanupStuckJobs(ctx context.Context) error {
	span := sentry.StartSpan(ctx, "jobs.cleanup_stuck_jobs")
	defer span.Finish()

	var completedJobs, timedOutJobs int64

	err := wp.dbQueue.ExecuteMaintenance(ctx, func(tx *sql.Tx) error {
		// 1. Mark jobs as completed when all tasks are done
		result, err := tx.ExecContext(ctx, `
				UPDATE jobs
				SET status = $1,
					completed_at = COALESCE(completed_at, $2),
				progress = 100.0
			WHERE (status = $3 OR status = $4)
			AND total_tasks > 0
			AND total_tasks = completed_tasks + failed_tasks
		`, JobStatusCompleted, time.Now().UTC(), JobStatusPending, JobStatusRunning)

		if err != nil {
			return err
		}

		completedJobs, err = result.RowsAffected()
		if err != nil {
			return err
		}

		// 2. Mark jobs as failed when stuck for too long
		// - Pending jobs with 0 tasks for 5 minutes (sitemap processing likely failed)
		// - Running jobs with no task progress for 30 minutes
		// - Jobs running for all tasks failed
		result, err = tx.ExecContext(ctx, `
			UPDATE jobs
			SET status = $1,
				completed_at = $2,
				error_message = CASE
					WHEN status = $3 AND total_tasks = 0 THEN 'Job timed out: no tasks created after 5 minutes (sitemap processing may have failed)'
					WHEN total_tasks > 0 AND total_tasks = failed_tasks THEN 'Job failed: all tasks failed'
					ELSE 'Job timed out: no task progress for 30 minutes'
				END
			WHERE (
				-- Pending jobs with no tasks for 5+ minutes
				(status = $3 AND total_tasks = 0 AND created_at < $4)
				OR
				-- Running jobs where all tasks failed
				(status = $5 AND total_tasks > 0 AND total_tasks = failed_tasks)
				OR
				-- Running jobs with no task updates for 30+ minutes
				(status = $5 AND total_tasks > 0 AND COALESCE((
					SELECT MAX(GREATEST(started_at, completed_at))
					FROM tasks
					WHERE job_id = jobs.id
				), created_at) < $6)
			)
		`, JobStatusFailed, time.Now().UTC(), JobStatusPending, time.Now().UTC().Add(-5*time.Minute), JobStatusRunning, time.Now().UTC().Add(-30*time.Minute))

		if err != nil {
			return err
		}

		timedOutJobs, err = result.RowsAffected()
		return err
	})

	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return fmt.Errorf("failed to cleanup stuck jobs: %w", err)
	}

	if completedJobs > 0 {
		log.Info().
			Int64("jobs_completed", completedJobs).
			Msg("Marked stuck jobs as completed")
	}

	if timedOutJobs > 0 {
		log.Warn().
			Int64("jobs_failed", timedOutJobs).
			Msg("Marked timed-out jobs as failed")
	}

	return nil
}

// processTask processes an individual task
// constructTaskURL builds a proper URL from task path and domain information
func constructTaskURL(path, domainName string) string {
	// Check if path is already a full URL
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return util.NormaliseURL(path)
	} else if domainName != "" {
		// Use centralized URL construction
		return util.ConstructURL(domainName, path)
	} else {
		// Fallback case - assume path is a full URL but missing protocol
		return util.NormaliseURL(path)
	}
}

// applyCrawlDelay applies robots.txt crawl delay if specified for the task's domain
func applyCrawlDelay(task *Task) {
	if task.CrawlDelay > 0 {
		log.Debug().
			Str("task_id", task.ID).
			Str("domain", task.DomainName).
			Int("crawl_delay_seconds", task.CrawlDelay).
			Msg("Applying crawl delay from robots.txt")
		time.Sleep(time.Duration(task.CrawlDelay) * time.Second)
	}
}

// processDiscoveredLinks handles link processing and enqueueing for discovered URLs
func (wp *WorkerPool) processDiscoveredLinks(ctx context.Context, task *Task, result *crawler.CrawlResult, sourceURL string) {
	log.Debug().
		Str("task_id", task.ID).
		Int("total_links_found", len(result.Links["header"])+len(result.Links["footer"])+len(result.Links["body"])).
		Bool("find_links_enabled", task.FindLinks).
		Msg("Starting link processing and priority assignment")

	// Use domain ID from task (already populated from job cache)
	domainID := task.DomainID
	if domainID == 0 {
		log.Error().
			Str("task_id", task.ID).
			Str("job_id", task.JobID).
			Msg("Missing domain ID; skipping link processing")
		return
	}

	// Get robots rules from cache for URL filtering
	var robotsRules *crawler.RobotsRules
	wp.jobInfoMutex.RLock()
	if jobInfo, exists := wp.jobInfoCache[task.JobID]; exists {
		robotsRules = jobInfo.RobotsRules
	}
	wp.jobInfoMutex.RUnlock()

	isHomepage := task.Path == "/"

	processLinkCategory := func(links []string, priority float64) {
		if len(links) == 0 {
			return
		}

		// 1. Filter links for same-domain and robots.txt compliance
		var filtered []string
		var blockedCount int
		for _, link := range links {
			linkURL, err := url.Parse(link)
			if err != nil {
				continue
			}
			if isSameOrSubDomain(linkURL.Hostname(), task.DomainName) {
				linkURL.Fragment = ""
				if linkURL.Path != "/" && strings.HasSuffix(linkURL.Path, "/") {
					linkURL.Path = strings.TrimSuffix(linkURL.Path, "/")
				}

				// Check robots.txt rules
				if robotsRules != nil && !crawler.IsPathAllowed(robotsRules, linkURL.Path) {
					blockedCount++
					log.Debug().
						Str("url", linkURL.String()).
						Str("path", linkURL.Path).
						Str("source", sourceURL).
						Msg("Link blocked by robots.txt")
					continue
				}

				filtered = append(filtered, linkURL.String())
			}
		}

		if blockedCount > 0 {
			log.Info().
				Str("task_id", task.ID).
				Int("blocked_count", blockedCount).
				Int("allowed_count", len(filtered)).
				Msg("Filtered discovered links against robots.txt")
		}

		if len(filtered) == 0 {
			return
		}

		// 2. Create page records
		pageIDs, paths, err := db.CreatePageRecords(ctx, wp.dbQueue, domainID, task.DomainName, filtered)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create page records for links")
			return
		}

		// 3. Create a slice of db.Page for enqueuing
		pagesToEnqueue := make([]db.Page, len(pageIDs))
		for i := range pageIDs {
			pagesToEnqueue[i] = db.Page{
				ID:   pageIDs[i],
				Path: paths[i],
				// Priority will be set by the caller of processLinkCategory
			}
		}

		// 4. Enqueue new tasks
		if err := wp.EnqueueURLs(ctx, task.JobID, pagesToEnqueue, "link", sourceURL); err != nil {
			log.Error().Err(err).Msg("Failed to enqueue discovered links")
			return // Stop if enqueuing fails
		}

		// 5. Update priorities for the newly created tasks
		if err := wp.updateTaskPriorities(ctx, task.JobID, domainID, priority, paths); err != nil {
			log.Error().Err(err).Msg("Failed to update task priorities for discovered links")
		}
	}

	// Apply priorities based on page type and link category
	if isHomepage {
		log.Debug().Str("task_id", task.ID).Msg("Processing links from HOMEPAGE")
		processLinkCategory(result.Links["header"], 1.000)
		processLinkCategory(result.Links["footer"], 0.990)
		processLinkCategory(result.Links["body"], task.PriorityScore*0.9) // Children of homepage
	} else {
		log.Debug().Str("task_id", task.ID).Msg("Processing links from regular page")
		// For all other pages, only process body links
		processLinkCategory(result.Links["body"], task.PriorityScore*0.9) // Children of other pages
	}
}

// handleTaskError processes task failures with appropriate retry logic and status updates
func (wp *WorkerPool) handleTaskError(ctx context.Context, task *db.Task, taskErr error) error {
	now := time.Now().UTC()
	retryReason := "non_retryable"

	// Check if this is a blocking error (403/429/503)
	if isBlockingError(taskErr) {
		maxRetries := wp.domainLimiter.cfg.MaxBlockingRetries
		if task.RetryCount < maxRetries {
			retryReason = "blocking"
			task.RetryCount++
			task.Status = string(TaskStatusPending)
			task.StartedAt = time.Time{} // Reset started time
			log.Warn().
				Err(taskErr).
				Str("task_id", task.ID).
				Int("retry_count", task.RetryCount).
				Int("max_retries", maxRetries).
				Msg("Blocking error (403/429/503), retry scheduled")
			observability.RecordWorkerTaskRetry(ctx, task.JobID, retryReason)
		} else {
			// Mark as permanently failed after 2 retries
			task.Status = string(TaskStatusFailed)
			task.CompletedAt = now
			task.Error = taskErr.Error()
			log.Error().
				Err(taskErr).
				Str("task_id", task.ID).
				Int("retry_count", task.RetryCount).
				Msg("Task blocked permanently after exhausting retries")
			wp.recordJobFailure(ctx, task.JobID, task.ID, taskErr)
			observability.RecordWorkerTaskFailure(ctx, task.JobID, "blocking")
		}
	} else if isRetryableError(taskErr) && task.RetryCount < MaxTaskRetries {
		// For other retryable errors, use normal retry limit
		retryReason = "retryable"
		task.RetryCount++
		task.Status = string(TaskStatusPending)
		task.StartedAt = time.Time{} // Reset started time
		log.Warn().
			Err(taskErr).
			Str("task_id", task.ID).
			Int("retry_count", task.RetryCount).
			Msg("Task failed with retryable error, scheduling retry")
		observability.RecordWorkerTaskRetry(ctx, task.JobID, retryReason)
	} else {
		// Mark as permanently failed
		task.Status = string(TaskStatusFailed)
		task.CompletedAt = now
		task.Error = taskErr.Error()
		log.Error().
			Err(taskErr).
			Str("task_id", task.ID).
			Int("retry_count", task.RetryCount).
			Msg("Task failed permanently")
		wp.recordJobFailure(ctx, task.JobID, task.ID, taskErr)
		failureReason := retryReason
		if !isBlockingError(taskErr) && !isRetryableError(taskErr) {
			failureReason = "non_retryable"
		} else if isRetryableError(taskErr) {
			failureReason = "retryable_exhausted"
		} else if isBlockingError(taskErr) {
			failureReason = "blocking"
		}
		observability.RecordWorkerTaskFailure(ctx, task.JobID, failureReason)
	}

	// Immediately decrement running_tasks to free concurrency slot
	// This allows workers to claim new tasks without waiting for batch flush
	if err := wp.dbQueue.DecrementRunningTasks(ctx, task.JobID); err != nil {
		log.Error().Err(err).Str("job_id", task.JobID).Str("task_id", task.ID).
			Msg("Failed to decrement running_tasks counter")
		// Don't return error - batch update will eventually sync the counter
	}

	// Queue task update for batch processing (detailed field updates)
	wp.batchManager.QueueTaskUpdate(task)

	return nil
}

// handleTaskSuccess processes successful task completion with metrics and database updates
func (wp *WorkerPool) handleTaskSuccess(ctx context.Context, task *db.Task, result *crawler.CrawlResult) error {
	now := time.Now().UTC()

	wp.resetJobFailureStreak(task.JobID)

	// Mark as completed with basic metrics
	task.Status = string(TaskStatusCompleted)
	task.CompletedAt = now
	task.StatusCode = result.StatusCode
	task.ResponseTime = result.ResponseTime
	task.CacheStatus = result.CacheStatus
	task.ContentType = result.ContentType
	task.ContentLength = result.ContentLength
	task.RedirectURL = result.RedirectURL

	// Performance metrics
	task.DNSLookupTime = result.Performance.DNSLookupTime
	task.TCPConnectionTime = result.Performance.TCPConnectionTime
	task.TLSHandshakeTime = result.Performance.TLSHandshakeTime
	task.TTFB = result.Performance.TTFB
	task.ContentTransferTime = result.Performance.ContentTransferTime

	// Second request metrics
	task.SecondResponseTime = result.SecondResponseTime
	task.SecondCacheStatus = result.SecondCacheStatus
	if result.SecondPerformance != nil {
		task.SecondContentLength = result.SecondContentLength
		task.SecondDNSLookupTime = result.SecondPerformance.DNSLookupTime
		task.SecondTCPConnectionTime = result.SecondPerformance.TCPConnectionTime
		task.SecondTLSHandshakeTime = result.SecondPerformance.TLSHandshakeTime
		task.SecondTTFB = result.SecondPerformance.TTFB
		task.SecondContentTransferTime = result.SecondPerformance.ContentTransferTime
	}

	// Marshal JSONB fields - ensure all marshaling succeeds before updating task
	// Always provide safe defaults for all JSON fields
	task.Headers = []byte("{}")
	task.SecondHeaders = []byte("{}")
	task.CacheCheckAttempts = []byte("[]")

	// Only attempt marshaling if data exists and is non-empty
	if len(result.Headers) > 0 {
		if headerBytes, err := json.Marshal(result.Headers); err == nil {
			// Validate that the marshaled JSON is valid
			if json.Valid(headerBytes) {
				task.Headers = headerBytes
			} else {
				log.Warn().Str("task_id", task.ID).Msg("Headers produced invalid JSON, using empty object")
			}
		} else {
			log.Error().Err(err).Str("task_id", task.ID).Interface("headers", result.Headers).Msg("Failed to marshal headers")
		}
	}

	if len(result.SecondHeaders) > 0 {
		if secondHeaderBytes, err := json.Marshal(result.SecondHeaders); err == nil {
			// Validate that the marshaled JSON is valid
			if json.Valid(secondHeaderBytes) {
				task.SecondHeaders = secondHeaderBytes
			} else {
				log.Warn().Str("task_id", task.ID).Msg("Second headers produced invalid JSON, using empty object")
			}
		} else {
			log.Error().Err(err).Str("task_id", task.ID).Interface("second_headers", result.SecondHeaders).Msg("Failed to marshal second headers")
		}
	}

	if len(result.CacheCheckAttempts) > 0 {
		if attemptsBytes, err := json.Marshal(result.CacheCheckAttempts); err == nil {
			// Validate that the marshaled JSON is valid
			if json.Valid(attemptsBytes) {
				task.CacheCheckAttempts = attemptsBytes
			} else {
				log.Warn().Str("task_id", task.ID).Msg("Cache check attempts produced invalid JSON, using empty array")
			}
		} else {
			log.Error().Err(err).Str("task_id", task.ID).Interface("cache_attempts", result.CacheCheckAttempts).Msg("Failed to marshal cache check attempts")
		}
	}

	// Immediately decrement running_tasks to free concurrency slot
	// This allows workers to claim new tasks without waiting for batch flush
	if err := wp.dbQueue.DecrementRunningTasks(ctx, task.JobID); err != nil {
		log.Error().Err(err).Str("job_id", task.JobID).Str("task_id", task.ID).
			Msg("Failed to decrement running_tasks counter")
		// Don't return error - batch update will eventually sync the counter
	}

	// Queue task update for batch processing (detailed field updates)
	wp.batchManager.QueueTaskUpdate(task)

	// Evaluate job performance for scaling
	if result.ResponseTime > 0 {
		wp.evaluateJobPerformance(task.JobID, result.ResponseTime)
	}

	return nil
}

func (wp *WorkerPool) processTask(ctx context.Context, task *Task) (*crawler.CrawlResult, error) {
	start := time.Now()
	status := "success"
	queueWait := time.Duration(0)
	if !task.CreatedAt.IsZero() {
		if !task.StartedAt.IsZero() {
			queueWait = task.StartedAt.Sub(task.CreatedAt)
		} else {
			queueWait = time.Since(task.CreatedAt)
		}
	}

	ctx, span := observability.StartWorkerTaskSpan(ctx, observability.WorkerTaskSpanInfo{
		JobID:     task.JobID,
		TaskID:    task.ID,
		Domain:    task.DomainName,
		Path:      task.Path,
		FindLinks: task.FindLinks,
	})
	defer span.End()

	defer func() {
		totalDuration := time.Duration(0)
		if !task.CreatedAt.IsZero() {
			totalDuration = time.Since(task.CreatedAt)
		}

		observability.RecordWorkerTask(ctx, observability.WorkerTaskMetrics{
			JobID:         task.JobID,
			Status:        status,
			Duration:      time.Since(start),
			QueueWait:     queueWait,
			TotalDuration: totalDuration,
		})
	}()

	// Construct a proper URL for processing
	urlStr := constructTaskURL(task.Path, task.DomainName)

	log.Info().Str("url", urlStr).Str("task_id", task.ID).Msg("Starting URL warm")

	limiter := wp.ensureDomainLimiter()
	permit, err := limiter.Acquire(ctx, DomainRequest{
		Domain:      task.DomainName,
		JobID:       task.JobID,
		RobotsDelay: time.Duration(task.CrawlDelay) * time.Second,
		JobConcurrency: func() int {
			if task.JobConcurrency > 0 {
				return task.JobConcurrency
			}
			return 1
		}(),
	})
	if err != nil {
		return nil, err
	}
	released := false
	defer func() {
		if !released {
			permit.Release(false, false)
		}
	}()

	result, err := wp.crawler.WarmURL(ctx, urlStr, task.FindLinks)
	if err != nil {
		status = "error"
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Error().Err(err).Str("task_id", task.ID).Msg("Crawler failed")
		rateLimited := IsRateLimitError(err)
		permit.Release(false, rateLimited)
		released = true
		return result, fmt.Errorf("crawler error: %w", err)
	}
	permit.Release(true, false)
	released = true

	if result != nil {
		span.SetAttributes(
			attribute.Int("http.status_code", result.StatusCode),
			attribute.Int("task.links_found", len(result.Links)),
			attribute.String("task.content_type", result.ContentType),
		)
	}
	span.SetStatus(codes.Ok, "completed")

	log.Info().
		Int("status_code", result.StatusCode).
		Str("task_id", task.ID).
		Int("links_found", len(result.Links)).
		Str("content_type", result.ContentType).
		Msg("Crawler completed")

	// Process discovered links if find_links is enabled
	if task.FindLinks && len(result.Links) > 0 {
		wp.processDiscoveredLinks(ctx, task, result, urlStr)
	}

	return result, nil
}

// isRetryableError checks if an error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := strings.ToLower(err.Error())

	// Network/timeout errors that should be retried
	networkErrors := strings.Contains(errorStr, "timeout") ||
		strings.Contains(errorStr, "deadline exceeded") ||
		strings.Contains(errorStr, "connection") ||
		strings.Contains(errorStr, "network") ||
		strings.Contains(errorStr, "temporary") ||
		strings.Contains(errorStr, "reset by peer") ||
		strings.Contains(errorStr, "broken pipe") ||
		strings.Contains(errorStr, "unexpected eof")

	// Server errors that should be retried (likely due to load/temporary issues)
	// Note: 503 Service Unavailable is treated as a blocking error (see isBlockingError)
	serverErrors := strings.Contains(errorStr, "internal server error") ||
		strings.Contains(errorStr, "bad gateway") ||
		strings.Contains(errorStr, "gateway timeout") ||
		strings.Contains(errorStr, "502") ||
		strings.Contains(errorStr, "504") ||
		strings.Contains(errorStr, "500")

	return networkErrors || serverErrors
}

// isBlockingError checks if an error indicates we're being blocked
func isBlockingError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := strings.ToLower(err.Error())

	// Blocking/rate limit errors that need special handling with exponential backoff
	return strings.Contains(errorStr, "403") ||
		strings.Contains(errorStr, "forbidden") ||
		strings.Contains(errorStr, "429") ||
		strings.Contains(errorStr, "too many requests") ||
		strings.Contains(errorStr, "rate limit") ||
		strings.Contains(errorStr, "503") ||
		strings.Contains(errorStr, "service unavailable")
}

// calculateBackoffDuration computes exponential backoff duration for retry attempts
// Uses 2^retryCount formula with a maximum cap of 60 seconds
func calculateBackoffDuration(retryCount int) time.Duration {
	// 2^0 = 1s, 2^1 = 2s, 2^2 = 4s, 2^3 = 8s, etc.
	backoffSeconds := 1 << uint(retryCount) // Bit shift for 2^retryCount
	backoffDuration := time.Duration(backoffSeconds) * time.Second

	// Cap at 60 seconds maximum
	maxBackoff := 60 * time.Second
	if backoffDuration > maxBackoff {
		backoffDuration = maxBackoff
	}

	return backoffDuration
}

// Helper function to check if a hostname is the same domain or a subdomain of the target domain
// Handles www prefix variations (www.test.com vs test.com)
func isSameOrSubDomain(hostname, targetDomain string) bool {
	// Normalise both domains by removing www prefix
	hostname = strings.ToLower(hostname)
	targetDomain = strings.ToLower(targetDomain)

	// Remove www. prefix if present
	normalisedHostname := strings.TrimPrefix(hostname, "www.")
	normalisedTarget := strings.TrimPrefix(targetDomain, "www.")

	// Direct match (after normalization)
	if normalisedHostname == normalisedTarget {
		return true
	}

	// Original direct match (before normalization)
	if hostname == targetDomain {
		return true
	}

	// Check if hostname ends with .targetDomain (subdomain check)
	if strings.HasSuffix(hostname, "."+targetDomain) {
		return true
	}

	// Check if hostname ends with .normalisedTarget (subdomain check without www)
	if strings.HasSuffix(hostname, "."+normalisedTarget) {
		return true
	}

	return false
}

// updateTaskPriorities updates the priority scores for tasks of linked pages
func (wp *WorkerPool) updateTaskPriorities(ctx context.Context, jobID string, domainID int, newPriority float64, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	var rowsAffected int64
	err := wp.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		// Update all task priorities in a single query
		result, err := tx.ExecContext(ctx, `
			UPDATE tasks t
			SET priority_score = $1
			FROM pages p
			WHERE t.page_id = p.id
			AND t.job_id = $2
			AND p.domain_id = $3
			AND p.path = ANY($4)
			AND t.priority_score < $1
		`, newPriority, jobID, domainID, paths)

		if err != nil {
			return err
		}

		rowsAffected, err = result.RowsAffected()
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to update task priorities: %w", err)
	}

	if rowsAffected > 0 {
		log.Info().
			Str("job_id", jobID).
			Int64("tasks_updated", rowsAffected).
			Float64("new_priority", newPriority).
			Msg("Updated task priorities for discovered links")
	}

	return nil
}

// evaluateJobPerformance checks if a job needs performance scaling
func (wp *WorkerPool) evaluateJobPerformance(jobID string, responseTime int64) {
	wp.perfMutex.Lock()
	defer wp.perfMutex.Unlock()

	perf, exists := wp.jobPerformance[jobID]
	if !exists {
		return // Job not tracked
	}

	// Add response time to recent tasks (sliding window of 5)
	perf.RecentTasks = append(perf.RecentTasks, responseTime)
	if len(perf.RecentTasks) > 5 {
		perf.RecentTasks = perf.RecentTasks[1:] // Remove oldest
	}

	// Only evaluate after we have at least 3 tasks
	if len(perf.RecentTasks) < 3 {
		return
	}

	// Calculate average response time
	var total int64
	for _, rt := range perf.RecentTasks {
		total += rt
	}
	avgResponseTime := total / int64(len(perf.RecentTasks))

	// Determine needed boost workers based on performance tiers
	var neededBoost int
	switch {
	case avgResponseTime >= 4000: // 4000ms+
		neededBoost = 20
	case avgResponseTime >= 3000: // 3000-4000ms
		neededBoost = 15
	case avgResponseTime >= 2000: // 2000-3000ms
		neededBoost = 10
	case avgResponseTime >= 1000: // 1000-2000ms
		neededBoost = 5
	default: // 0-1000ms
		neededBoost = 0
	}

	// Check if boost needs to change
	if neededBoost != perf.CurrentBoost {
		boostDiff := neededBoost - perf.CurrentBoost

		log.Info().
			Str("job_id", jobID).
			Int64("avg_response_time", avgResponseTime).
			Int("old_boost", perf.CurrentBoost).
			Int("new_boost", neededBoost).
			Int("boost_diff", boostDiff).
			Msg("Job performance scaling triggered")

		// Update current boost
		perf.CurrentBoost = neededBoost
		perf.LastCheck = time.Now()

		// Apply scaling to worker pool
		if boostDiff > 0 {
			// Need more workers
			wp.workersMutex.Lock()
			targetWorkers := wp.currentWorkers + boostDiff
			if targetWorkers > wp.maxWorkers { // Respect environment-specific max
				targetWorkers = wp.maxWorkers
				perf.CurrentBoost = perf.CurrentBoost - (wp.currentWorkers + boostDiff - wp.maxWorkers) // Adjust boost to actual
			}
			wp.workersMutex.Unlock()

			if targetWorkers > wp.currentWorkers {
				// Use detached context with timeout for worker scaling
				scalingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
				go func() {
					defer cancel()
					wp.scaleWorkers(scalingCtx, targetWorkers)
				}()
			}
		}
		// Note: For scaling down (boostDiff < 0), we let workers naturally exit
		// when they check shouldExit in the worker loop
	}
}

func hasNotificationConfig(cfg *db.Config) bool {
	if cfg == nil {
		return false
	}
	if cfg.DatabaseURL != "" {
		return true
	}
	return cfg.Host != "" && cfg.Port != "" && cfg.Database != "" && cfg.User != ""
}

// listenForNotifications sets up PostgreSQL LISTEN/NOTIFY
func (wp *WorkerPool) listenForNotifications(ctx context.Context) {
	var conn *pgx.Conn
	var err error

	connect := func() (*pgx.Conn, error) {
		c, err := pgx.Connect(ctx, wp.dbConfig.ConnectionString())
		if err != nil {
			return nil, err
		}
		_, err = c.Exec(ctx, "LISTEN new_tasks")
		if err != nil {
			c.Close(ctx)
			return nil, err
		}
		return c, nil
	}

	conn, err = connect()
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect for notifications initially")
		return
	}
	defer conn.Close(ctx)

	for {
		select {
		case <-wp.stopCh:
			log.Debug().Msg("Notification listener received stop signal.")
			return
		case <-ctx.Done():
			log.Debug().Msg("Notification listener context cancelled.")
			return
		default:
			// Non-blocking check for stop signal before waiting for notification
		}

		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil || wp.stopping.Load() {
				return // Context cancelled or pool is stopping
			}
			log.Warn().Err(err).Msg("Error waiting for notification, reconnecting...")
			conn.Close(ctx)
			time.Sleep(5 * time.Second) // Wait before reconnecting

			conn, err = connect()
			if err != nil {
				log.Warn().Err(err).Msg("Failed to reconnect for notifications")
				continue
			}
			continue
		}

		log.Debug().Str("channel", notification.Channel).Msg("Received database notification")
		// Notify workers of new tasks (non-blocking)
		select {
		case wp.notifyCh <- struct{}{}:
		default:
			// Channel already has notification pending
		}
	}
}

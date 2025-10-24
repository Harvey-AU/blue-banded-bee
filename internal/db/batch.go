package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

const (
	// MaxBatchSize is the maximum number of tasks to batch before forcing a flush
	MaxBatchSize = 100
	// MaxBatchInterval is the maximum time to wait before flushing a batch
	MaxBatchInterval = 5 * time.Second
	// BatchChannelSize is the buffer size for the update channel
	BatchChannelSize = 500
	// MaxConsecutiveFailures before falling back to individual updates
	MaxConsecutiveFailures = 3
	// MaxShutdownRetries for final flush attempts
	MaxShutdownRetries = 5
	// ShutdownRetryDelay between retry attempts
	ShutdownRetryDelay = 500 * time.Millisecond
)

// isRetryableError determines if an error is infrastructure-related (should retry)
// vs data-related (poison pill that should be skipped)
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Unwrap to find the underlying PostgreSQL error
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code.Class() {
		case "08": // Connection exceptions
			return true
		case "53": // Insufficient resources (connection limit, out of memory, disk full)
			return true
		case "57": // Operator intervention (shutdown in progress, etc)
			return true
		case "58": // System errors (IO errors, etc)
			return true
		case "23": // Integrity constraint violations - NOT retryable (bad data)
			return false
		case "22": // Data exceptions (invalid input, etc) - NOT retryable (bad data)
			return false
		default:
			// For unknown postgres errors, be conservative and retry
			return true
		}
	}

	// Check for common Go database errors
	switch err {
	case sql.ErrConnDone:
		return true
	case context.DeadlineExceeded:
		return true
	case context.Canceled:
		return true
	}

	// Check error message for connection issues
	errMsg := err.Error()
	connectionErrors := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"no such host",
		"timeout",
		"too many clients",
		"pool",
	}

	for _, connErr := range connectionErrors {
		if stringContains(errMsg, connErr) {
			return true
		}
	}

	// Default: assume it's retryable (safer than dropping data)
	return true
}

// stringContains checks if a string contains a substring
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TaskUpdate represents a pending task status update
type TaskUpdate struct {
	Task      *Task
	UpdatedAt time.Time
}

// QueueExecutor defines the minimal interface needed for batch operations
type QueueExecutor interface {
	Execute(ctx context.Context, fn func(*sql.Tx) error) error
}

// BatchManager coordinates batching of database operations
type BatchManager struct {
	queue            QueueExecutor
	updates          chan *TaskUpdate
	stopCh           chan struct{}
	wg               sync.WaitGroup
	consecutiveFails int
	mu               sync.Mutex
}

// NewBatchManager creates a new batch manager
func NewBatchManager(queue QueueExecutor) *BatchManager {
	bm := &BatchManager{
		queue:   queue,
		updates: make(chan *TaskUpdate, BatchChannelSize),
		stopCh:  make(chan struct{}),
	}

	// Start the batch processor
	bm.wg.Add(1)
	go bm.processUpdateBatches()

	log.Info().
		Int("max_batch_size", MaxBatchSize).
		Dur("max_batch_interval", MaxBatchInterval).
		Int("channel_size", BatchChannelSize).
		Msg("Batch manager started")

	return bm
}

// QueueTaskUpdate adds a task update to the batch queue
func (bm *BatchManager) QueueTaskUpdate(task *Task) {
	update := &TaskUpdate{
		Task:      task,
		UpdatedAt: time.Now(),
	}

	select {
	case bm.updates <- update:
		// Queued successfully
	default:
		// Channel full - this is critical, log and block
		log.Error().
			Str("task_id", task.ID).
			Int("channel_size", BatchChannelSize).
			Msg("Update batch channel full, blocking until space available")
		bm.updates <- update // Block until space available
	}
}

// processUpdateBatches accumulates and flushes task updates
func (bm *BatchManager) processUpdateBatches() {
	defer bm.wg.Done()

	ticker := time.NewTicker(MaxBatchInterval)
	defer ticker.Stop()

	batch := make([]*TaskUpdate, 0, MaxBatchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}

		if err := bm.flushTaskUpdates(context.Background(), batch); err != nil {
			// Classify the error
			retryable := isRetryableError(err)

			if retryable {
				// Infrastructure error - just log and retry, don't count towards poison pill threshold
				log.Warn().
					Err(err).
					Int("batch_size", len(batch)).
					Bool("retryable", true).
					Msg("Batch flush failed due to infrastructure issue - will retry")
				// Keep batch in memory, try again on next flush interval
				return
			}

			// Non-retryable error (likely bad data) - count towards poison pill threshold
			bm.mu.Lock()
			bm.consecutiveFails++
			failCount := bm.consecutiveFails
			bm.mu.Unlock()

			log.Error().
				Err(err).
				Int("batch_size", len(batch)).
				Int("consecutive_data_failures", failCount).
				Bool("retryable", false).
				Msg("Batch flush failed due to data error")

			// If we've had too many data errors, fall back to individual updates to isolate poison pill
			if failCount >= MaxConsecutiveFailures {
				// Capture to Sentry - this indicates data corruption blocking progress
				sentry.CaptureException(fmt.Errorf("batch poison pill detected after %d consecutive data failures: %w", failCount, err))

				log.Warn().
					Int("batch_size", len(batch)).
					Msg("Max consecutive data failures reached - attempting individual updates to isolate poison pill")

				successCount, skippedCount := bm.flushIndividualUpdates(context.Background(), batch)

				log.Info().
					Int("total", len(batch)).
					Int("success", successCount).
					Int("skipped", skippedCount).
					Msg("Individual update fallback completed")

				// Clear batch and reset failure counter after individual processing
				batch = batch[:0]
				bm.mu.Lock()
				bm.consecutiveFails = 0
				bm.mu.Unlock()
			}
			// Keep batch for retry on next flush
			return
		}

		// Successful flush - reset batch and failure counter
		batch = batch[:0]
		bm.mu.Lock()
		bm.consecutiveFails = 0
		bm.mu.Unlock()
	}

	for {
		select {
		case update := <-bm.updates:
			batch = append(batch, update)

			// Flush if batch is full
			if len(batch) >= MaxBatchSize {
				flush()
				ticker.Reset(MaxBatchInterval)
			}

		case <-ticker.C:
			flush()

		case <-bm.stopCh:
			// Drain remaining updates
			draining := true
			for draining {
				select {
				case update := <-bm.updates:
					batch = append(batch, update)
				default:
					draining = false
				}
			}

			// Retry final flush with backoff to ensure zero data loss on shutdown
			var lastErr error
			for attempt := 0; attempt < MaxShutdownRetries; attempt++ {
				if len(batch) == 0 {
					break
				}

				lastErr = bm.flushTaskUpdates(context.Background(), batch)
				if lastErr == nil {
					log.Info().
						Int("batch_size", len(batch)).
						Int("attempt", attempt+1).
						Msg("Final batch flush successful on shutdown")
					batch = batch[:0]
					break
				}

				log.Warn().
					Err(lastErr).
					Int("batch_size", len(batch)).
					Int("attempt", attempt+1).
					Int("max_attempts", MaxShutdownRetries).
					Msg("Final batch flush failed - retrying")

				if attempt < MaxShutdownRetries-1 {
					time.Sleep(ShutdownRetryDelay)
				}
			}

			// If batch flush still failing after retries, check error type
			if len(batch) > 0 && lastErr != nil {
				retryable := isRetryableError(lastErr)

				if retryable {
					// Infrastructure failure - don't drop data, just log critical error
					sentry.CaptureException(fmt.Errorf("database unavailable on shutdown, %d task updates in memory could not be persisted: %w", len(batch), lastErr))

					log.Error().
						Err(lastErr).
						Int("batch_size", len(batch)).
						Bool("retryable", true).
						Msg("CRITICAL: Database unavailable on shutdown - task updates could not be persisted (will be retried on next startup if persistence implemented)")
				} else {
					// Data error - try individual updates to isolate poison pill
					log.Warn().
						Err(lastErr).
						Int("batch_size", len(batch)).
						Bool("retryable", false).
						Msg("Final batch flush failed due to data error - attempting individual updates to isolate poison pill")

					successCount, skippedCount := bm.flushIndividualUpdates(context.Background(), batch)

					if skippedCount > 0 {
						// Capture to Sentry - data corruption caused permanent loss on shutdown
						sentry.CaptureException(fmt.Errorf("shutdown with data errors: %d tasks with bad data could not be persisted", skippedCount))

						log.Error().
							Int("skipped", skippedCount).
							Msg("CRITICAL: Some task updates with bad data could not be persisted on shutdown")
					} else {
						log.Info().
							Int("success", successCount).
							Msg("All remaining task updates persisted via individual fallback")
					}
				}
			}

			return
		}
	}
}

// flushTaskUpdates performs true batch UPDATE using PostgreSQL unnest
func (bm *BatchManager) flushTaskUpdates(ctx context.Context, updates []*TaskUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	start := time.Now()

	// Group updates by status to use appropriate UPDATE logic
	completedTasks := make([]*Task, 0, len(updates))
	failedTasks := make([]*Task, 0, len(updates))
	skippedTasks := make([]*Task, 0, len(updates))
	pendingTasks := make([]*Task, 0, len(updates))

	for _, update := range updates {
		task := update.Task

		// Set timestamps if not already set
		now := time.Now()
		if (task.Status == "completed" || task.Status == "failed") && task.CompletedAt.IsZero() {
			task.CompletedAt = now
		}

		switch task.Status {
		case "completed":
			completedTasks = append(completedTasks, task)
		case "failed", "blocked":
			failedTasks = append(failedTasks, task)
		case "skipped":
			skippedTasks = append(skippedTasks, task)
		case "pending":
			pendingTasks = append(pendingTasks, task)
		default:
			log.Warn().
				Str("task_id", task.ID).
				Str("status", task.Status).
				Msg("Unexpected task status in batch, skipping")
		}
	}

	err := bm.queue.Execute(ctx, func(tx *sql.Tx) error {
		// Batch update completed tasks
		if len(completedTasks) > 0 {
			if err := bm.batchUpdateCompleted(ctx, tx, completedTasks); err != nil {
				return fmt.Errorf("failed to batch update completed tasks: %w", err)
			}
		}

		// Batch update failed tasks
		if len(failedTasks) > 0 {
			if err := bm.batchUpdateFailed(ctx, tx, failedTasks); err != nil {
				return fmt.Errorf("failed to batch update failed tasks: %w", err)
			}
		}

		// Batch update skipped tasks
		if len(skippedTasks) > 0 {
			if err := bm.batchUpdateSkipped(ctx, tx, skippedTasks); err != nil {
				return fmt.Errorf("failed to batch update skipped tasks: %w", err)
			}
		}

		// Batch update pending tasks (retries)
		if len(pendingTasks) > 0 {
			if err := bm.batchUpdatePending(ctx, tx, pendingTasks); err != nil {
				return fmt.Errorf("failed to batch update pending tasks: %w", err)
			}
		}

		return nil
	})

	duration := time.Since(start)

	if err != nil {
		log.Error().
			Err(err).
			Int("total_tasks", len(updates)).
			Int("completed", len(completedTasks)).
			Int("failed", len(failedTasks)).
			Int("skipped", len(skippedTasks)).
			Int("pending", len(pendingTasks)).
			Dur("duration_ms", duration).
			Msg("Batch update failed")
		return err
	}

	log.Info().
		Int("total_tasks", len(updates)).
		Int("completed", len(completedTasks)).
		Int("failed", len(failedTasks)).
		Int("skipped", len(skippedTasks)).
		Int("pending", len(pendingTasks)).
		Dur("duration_ms", duration).
		Msg("Batch update successful")

	return nil
}

// flushIndividualUpdates attempts to update tasks one-by-one to isolate poison pills
// Returns (successCount, skippedCount)
func (bm *BatchManager) flushIndividualUpdates(ctx context.Context, updates []*TaskUpdate) (int, int) {
	successCount := 0
	skippedCount := 0

	for _, update := range updates {
		// Try to update this single task
		err := bm.queue.Execute(ctx, func(tx *sql.Tx) error {
			// Use the original UpdateTaskStatus logic for single task
			task := update.Task

			var updateErr error
			switch task.Status {
			case "completed":
				updateErr = bm.batchUpdateCompleted(ctx, tx, []*Task{task})
			case "failed", "blocked":
				updateErr = bm.batchUpdateFailed(ctx, tx, []*Task{task})
			case "skipped":
				updateErr = bm.batchUpdateSkipped(ctx, tx, []*Task{task})
			case "pending":
				updateErr = bm.batchUpdatePending(ctx, tx, []*Task{task})
			default:
				return fmt.Errorf("unknown status: %s", task.Status)
			}

			return updateErr
		})

		if err != nil {
			// This is the poison pill - capture to Sentry and skip it
			sentry.CaptureException(fmt.Errorf("poison pill task %s (status: %s) failed individual update: %w", update.Task.ID, update.Task.Status, err))

			log.Error().
				Err(err).
				Str("task_id", update.Task.ID).
				Str("status", update.Task.Status).
				Msg("POISON PILL: Task update failed even in individual mode - skipping")
			skippedCount++
		} else {
			successCount++
		}
	}

	return successCount, skippedCount
}

// batchUpdateCompleted updates multiple completed tasks in a single statement
func (bm *BatchManager) batchUpdateCompleted(ctx context.Context, tx *sql.Tx, tasks []*Task) error {
	if len(tasks) == 0 {
		return nil
	}

	// Build arrays for all fields
	ids := make([]string, len(tasks))
	completedAts := make([]time.Time, len(tasks))
	statusCodes := make([]int, len(tasks))
	responseTimes := make([]int64, len(tasks))
	cacheStatuses := make([]string, len(tasks))
	contentTypes := make([]string, len(tasks))
	contentLengths := make([]int64, len(tasks))
	headers := make([]string, len(tasks))
	redirectURLs := make([]string, len(tasks))
	dnsLookupTimes := make([]int64, len(tasks))
	tcpConnectionTimes := make([]int64, len(tasks))
	tlsHandshakeTimes := make([]int64, len(tasks))
	ttfbs := make([]int64, len(tasks))
	contentTransferTimes := make([]int64, len(tasks))
	secondResponseTimes := make([]int64, len(tasks))
	secondCacheStatuses := make([]string, len(tasks))
	secondContentLengths := make([]int64, len(tasks))
	secondHeaders := make([]string, len(tasks))
	secondDNSLookupTimes := make([]int64, len(tasks))
	secondTCPConnectionTimes := make([]int64, len(tasks))
	secondTLSHandshakeTimes := make([]int64, len(tasks))
	secondTTFBs := make([]int64, len(tasks))
	secondContentTransferTimes := make([]int64, len(tasks))
	retryCounts := make([]int, len(tasks))
	cacheCheckAttempts := make([]string, len(tasks))

	for i, task := range tasks {
		ids[i] = task.ID
		completedAts[i] = task.CompletedAt
		statusCodes[i] = task.StatusCode
		responseTimes[i] = task.ResponseTime
		cacheStatuses[i] = task.CacheStatus
		contentTypes[i] = task.ContentType
		contentLengths[i] = task.ContentLength

		// Ensure JSONB fields are never nil and are valid JSON
		if len(task.Headers) == 0 {
			headers[i] = "{}"
		} else {
			headers[i] = string(task.Headers)
		}

		redirectURLs[i] = task.RedirectURL
		dnsLookupTimes[i] = task.DNSLookupTime
		tcpConnectionTimes[i] = task.TCPConnectionTime
		tlsHandshakeTimes[i] = task.TLSHandshakeTime
		ttfbs[i] = task.TTFB
		contentTransferTimes[i] = task.ContentTransferTime
		secondResponseTimes[i] = task.SecondResponseTime
		secondCacheStatuses[i] = task.SecondCacheStatus
		secondContentLengths[i] = task.SecondContentLength

		if len(task.SecondHeaders) == 0 {
			secondHeaders[i] = "{}"
		} else {
			secondHeaders[i] = string(task.SecondHeaders)
		}

		secondDNSLookupTimes[i] = task.SecondDNSLookupTime
		secondTCPConnectionTimes[i] = task.SecondTCPConnectionTime
		secondTLSHandshakeTimes[i] = task.SecondTLSHandshakeTime
		secondTTFBs[i] = task.SecondTTFB
		secondContentTransferTimes[i] = task.SecondContentTransferTime
		retryCounts[i] = task.RetryCount

		if len(task.CacheCheckAttempts) == 0 {
			cacheCheckAttempts[i] = "[]"
		} else {
			cacheCheckAttempts[i] = string(task.CacheCheckAttempts)
		}
	}

	// Single UPDATE statement using unnest to batch update all tasks
	query := `
		UPDATE tasks
		SET status = 'completed',
			completed_at = updates.completed_at,
			status_code = updates.status_code,
			response_time = updates.response_time,
			cache_status = updates.cache_status,
			content_type = updates.content_type,
			content_length = updates.content_length,
			headers = updates.headers::jsonb,
			redirect_url = updates.redirect_url,
			dns_lookup_time = updates.dns_lookup_time,
			tcp_connection_time = updates.tcp_connection_time,
			tls_handshake_time = updates.tls_handshake_time,
			ttfb = updates.ttfb,
			content_transfer_time = updates.content_transfer_time,
			second_response_time = updates.second_response_time,
			second_cache_status = updates.second_cache_status,
			second_content_length = updates.second_content_length,
			second_headers = updates.second_headers::jsonb,
			second_dns_lookup_time = updates.second_dns_lookup_time,
			second_tcp_connection_time = updates.second_tcp_connection_time,
			second_tls_handshake_time = updates.second_tls_handshake_time,
			second_ttfb = updates.second_ttfb,
			second_content_transfer_time = updates.second_content_transfer_time,
			retry_count = updates.retry_count,
			cache_check_attempts = updates.cache_check_attempts::jsonb
		FROM (
			SELECT
				unnest($1::text[]) AS id,
				unnest($2::timestamptz[]) AS completed_at,
				unnest($3::integer[]) AS status_code,
				unnest($4::bigint[]) AS response_time,
				unnest($5::text[]) AS cache_status,
				unnest($6::text[]) AS content_type,
				unnest($7::bigint[]) AS content_length,
				unnest($8::text[]) AS headers,
				unnest($9::text[]) AS redirect_url,
				unnest($10::bigint[]) AS dns_lookup_time,
				unnest($11::bigint[]) AS tcp_connection_time,
				unnest($12::bigint[]) AS tls_handshake_time,
				unnest($13::bigint[]) AS ttfb,
				unnest($14::bigint[]) AS content_transfer_time,
				unnest($15::bigint[]) AS second_response_time,
				unnest($16::text[]) AS second_cache_status,
				unnest($17::bigint[]) AS second_content_length,
				unnest($18::text[]) AS second_headers,
				unnest($19::bigint[]) AS second_dns_lookup_time,
				unnest($20::bigint[]) AS second_tcp_connection_time,
				unnest($21::bigint[]) AS second_tls_handshake_time,
				unnest($22::bigint[]) AS second_ttfb,
				unnest($23::bigint[]) AS second_content_transfer_time,
				unnest($24::integer[]) AS retry_count,
				unnest($25::text[]) AS cache_check_attempts
		) AS updates
		WHERE tasks.id = updates.id
	`

	result, err := tx.ExecContext(ctx, query,
		pq.Array(ids),
		pq.Array(completedAts),
		pq.Array(statusCodes),
		pq.Array(responseTimes),
		pq.Array(cacheStatuses),
		pq.Array(contentTypes),
		pq.Array(contentLengths),
		pq.Array(headers),
		pq.Array(redirectURLs),
		pq.Array(dnsLookupTimes),
		pq.Array(tcpConnectionTimes),
		pq.Array(tlsHandshakeTimes),
		pq.Array(ttfbs),
		pq.Array(contentTransferTimes),
		pq.Array(secondResponseTimes),
		pq.Array(secondCacheStatuses),
		pq.Array(secondContentLengths),
		pq.Array(secondHeaders),
		pq.Array(secondDNSLookupTimes),
		pq.Array(secondTCPConnectionTimes),
		pq.Array(secondTLSHandshakeTimes),
		pq.Array(secondTTFBs),
		pq.Array(secondContentTransferTimes),
		pq.Array(retryCounts),
		pq.Array(cacheCheckAttempts),
	)

	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Debug().
		Int("tasks_count", len(tasks)).
		Int64("rows_affected", rowsAffected).
		Msg("Batch updated completed tasks")

	return nil
}

// batchUpdateFailed updates multiple failed tasks in a single statement
func (bm *BatchManager) batchUpdateFailed(ctx context.Context, tx *sql.Tx, tasks []*Task) error {
	if len(tasks) == 0 {
		return nil
	}

	ids := make([]string, len(tasks))
	completedAts := make([]time.Time, len(tasks))
	errors := make([]string, len(tasks))
	retryCounts := make([]int, len(tasks))
	statuses := make([]string, len(tasks))

	for i, task := range tasks {
		ids[i] = task.ID
		completedAts[i] = task.CompletedAt
		errors[i] = task.Error
		retryCounts[i] = task.RetryCount
		statuses[i] = task.Status // Could be "failed" or "blocked"
	}

	query := `
		UPDATE tasks
		SET status = updates.status,
			completed_at = updates.completed_at,
			error = updates.error,
			retry_count = updates.retry_count
		FROM (
			SELECT
				unnest($1::text[]) AS id,
				unnest($2::text[]) AS status,
				unnest($3::timestamptz[]) AS completed_at,
				unnest($4::text[]) AS error,
				unnest($5::integer[]) AS retry_count
		) AS updates
		WHERE tasks.id = updates.id
	`

	result, err := tx.ExecContext(ctx, query,
		pq.Array(ids),
		pq.Array(statuses),
		pq.Array(completedAts),
		pq.Array(errors),
		pq.Array(retryCounts),
	)

	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Debug().
		Int("tasks_count", len(tasks)).
		Int64("rows_affected", rowsAffected).
		Msg("Batch updated failed tasks")

	return nil
}

// batchUpdateSkipped updates multiple skipped tasks in a single statement
func (bm *BatchManager) batchUpdateSkipped(ctx context.Context, tx *sql.Tx, tasks []*Task) error {
	if len(tasks) == 0 {
		return nil
	}

	ids := make([]string, len(tasks))
	for i, task := range tasks {
		ids[i] = task.ID
	}

	query := `
		UPDATE tasks
		SET status = 'skipped'
		WHERE id = ANY($1::text[])
	`

	result, err := tx.ExecContext(ctx, query, pq.Array(ids))
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Debug().
		Int("tasks_count", len(tasks)).
		Int64("rows_affected", rowsAffected).
		Msg("Batch updated skipped tasks")

	return nil
}

// batchUpdatePending updates tasks that are being retried (set back to pending status)
func (bm *BatchManager) batchUpdatePending(ctx context.Context, tx *sql.Tx, tasks []*Task) error {
	if len(tasks) == 0 {
		return nil
	}

	// Build arrays for batch update
	ids := make([]string, len(tasks))
	retryCounts := make([]int, len(tasks))
	startedAts := make([]time.Time, len(tasks))

	for i, task := range tasks {
		ids[i] = task.ID
		retryCounts[i] = task.RetryCount
		startedAts[i] = task.StartedAt
	}

	query := `
		UPDATE tasks
		SET status = 'pending',
		    retry_count = updates.retry_count,
		    started_at = updates.started_at
		FROM (
			SELECT
				unnest($1::text[]) AS id,
				unnest($2::int[]) AS retry_count,
				unnest($3::timestamptz[]) AS started_at
		) AS updates
		WHERE tasks.id = updates.id
	`

	result, err := tx.ExecContext(ctx, query,
		pq.Array(ids),
		pq.Array(retryCounts),
		pq.Array(startedAts),
	)

	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Debug().
		Int("tasks_count", len(tasks)).
		Int64("rows_affected", rowsAffected).
		Msg("Batch updated pending tasks (retries)")

	return nil
}

// Stop gracefully shuts down the batch manager, flushing remaining updates
func (bm *BatchManager) Stop() {
	close(bm.stopCh)
	bm.wg.Wait()
	log.Info().Msg("Batch manager stopped")
}

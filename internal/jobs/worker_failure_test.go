//go:build unit || !integration

package jobs

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordJobFailureTriggersThresholdAndFailsJob(t *testing.T) {
	t.Setenv("BBB_JOB_FAILURE_THRESHOLD", "2")

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	jobID := "job-fail"
	lastErr := errors.New("429 too many requests")
	message := "Job failed after 2 consecutive task failures (last error: 429 too many requests)"

	mock.ExpectBegin()
	// Expect job status update
	mock.ExpectExec(regexp.QuoteMeta(`
			UPDATE jobs
			SET status = $1,
				completed_at = COALESCE(completed_at, $2),
				error_message = $3
			WHERE id = $4
				AND status <> $5
				AND status <> $6
		`)).
		WithArgs(string(JobStatusFailed), sqlmock.AnyArg(), message, jobID, string(JobStatusFailed), string(JobStatusCancelled)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// Expect orphaned tasks cleanup
	mock.ExpectExec(regexp.QuoteMeta(`
			UPDATE tasks
			SET status = 'skipped',
				completed_at = $1,
				error = $2
			WHERE job_id = $3
				AND status IN ('pending', 'waiting')
		`)).
		WithArgs(sqlmock.AnyArg(), "Job failed due to consecutive task failures", jobID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	queueMock := &MockDbQueue{}
	queueMock.ExecuteFunc = func(ctx context.Context, fn func(*sql.Tx) error) error {
		tx, txErr := sqlDB.BeginTx(ctx, nil)
		if txErr != nil {
			return txErr
		}
		if err := fn(tx); err != nil {
			_ = tx.Rollback()
			return err
		}
		return tx.Commit()
	}
	queueMock.ExecuteMaintenanceFunc = func(ctx context.Context, fn func(*sql.Tx) error) error {
		return nil
	}

	wp := NewWorkerPool(sqlDB, queueMock, &simpleCrawlerMock{}, 1, 1, &db.Config{})

	wp.jobsMutex.Lock()
	wp.jobs[jobID] = true
	wp.jobsMutex.Unlock()

	wp.jobFailureMutex.Lock()
	wp.jobFailureCounters[jobID] = &jobFailureState{}
	wp.jobFailureMutex.Unlock()

	reqCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	wp.recordJobFailure(reqCtx, jobID, "task-1", lastErr)

	wp.jobFailureMutex.Lock()
	state, ok := wp.jobFailureCounters[jobID]
	wp.jobFailureMutex.Unlock()
	require.True(t, ok)
	assert.Equal(t, 1, state.streak)
	assert.False(t, state.triggered)

	wp.recordJobFailure(reqCtx, jobID, "task-2", lastErr)

	wp.jobsMutex.RLock()
	_, active := wp.jobs[jobID]
	wp.jobsMutex.RUnlock()
	assert.False(t, active, "job should be removed after threshold breach")

	assert.NoError(t, mock.ExpectationsWereMet())

	// Further failures after removal should be ignored
	wp.recordJobFailure(reqCtx, jobID, "task-3", lastErr)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestResetJobFailureStreak(t *testing.T) {
	t.Setenv("BBB_JOB_FAILURE_THRESHOLD", "3")

	wp := NewWorkerPool(&sql.DB{}, &simpleDbQueueMock{}, &simpleCrawlerMock{}, 1, 1, &db.Config{})
	jobID := "job-reset"

	wp.jobsMutex.Lock()
	wp.jobs[jobID] = true
	wp.jobsMutex.Unlock()

	wp.jobFailureMutex.Lock()
	wp.jobFailureCounters[jobID] = &jobFailureState{streak: 2}
	wp.jobFailureMutex.Unlock()

	wp.resetJobFailureStreak(jobID)

	wp.jobFailureMutex.Lock()
	state, ok := wp.jobFailureCounters[jobID]
	wp.jobFailureMutex.Unlock()
	require.True(t, ok)
	assert.Equal(t, 0, state.streak)
	assert.False(t, state.triggered)
}

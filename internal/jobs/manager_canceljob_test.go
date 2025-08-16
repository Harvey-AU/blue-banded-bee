//go:build unit || !integration

package jobs

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCancelJob_Unit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		jobID         string
		setupMock     func(sqlmock.Sqlmock)
		expectedError string
	}{
		{
			name:  "cancel running job successfully",
			jobID: "running-job-123",
			setupMock: func(mock sqlmock.Sqlmock) {
				// GetJob transaction
				mock.ExpectBegin()
				
				rows := sqlmock.NewRows([]string{
					"id", "name", "status", "progress", "total_tasks", "completed_tasks",
					"failed_tasks", "skipped_tasks", "created_at", "started_at", "completed_at",
					"concurrency", "find_links", "include_paths", "exclude_paths", "error_message",
					"required_workers", "found_tasks", "sitemap_tasks", "duration_seconds",
					"avg_time_per_task_seconds",
				}).AddRow(
					"running-job-123",
					"example.com",
					"running",
					50.0,
					100,
					50,
					0,
					0,
					time.Now().Add(-1*time.Hour),
					sql.NullTime{Time: time.Now().Add(-30*time.Minute), Valid: true},
					sql.NullTime{Valid: false},
					10,
					true,
					[]byte(`["/*"]`),
					[]byte(`["/admin/*"]`),
					sql.NullString{Valid: false},
					5,
					50,
					50,
					sql.NullInt64{Valid: false},
					sql.NullFloat64{Valid: false},
				)

				mock.ExpectQuery(`SELECT 
				j.id, d.name, j.status, j.progress, j.total_tasks, j.completed_tasks, j.failed_tasks, j.skipped_tasks,
				j.created_at, j.started_at, j.completed_at, j.concurrency, j.find_links,
				j.include_paths, j.exclude_paths, j.error_message, j.required_workers,
				j.found_tasks, j.sitemap_tasks, j.duration_seconds, j.avg_time_per_task_seconds
			FROM jobs j
			JOIN domains d ON j.domain_id = d.id
			WHERE j.id = \$1`).
					WithArgs("running-job-123").
					WillReturnRows(rows)

				mock.ExpectCommit()

				// Cancel job transaction
				mock.ExpectBegin()

				// Update job status
				mock.ExpectExec(`UPDATE jobs
				SET status = \$1, completed_at = \$2
				WHERE id = \$3`).
					WithArgs("cancelled", sqlmock.AnyArg(), "running-job-123").
					WillReturnResult(sqlmock.NewResult(1, 1))

				// Cancel pending tasks
				mock.ExpectExec(`UPDATE tasks
				SET status = \$1
				WHERE job_id = \$2 AND status = \$3`).
					WithArgs("skipped", "running-job-123", "pending").
					WillReturnResult(sqlmock.NewResult(1, 10))

				mock.ExpectCommit()
			},
			expectedError: "",
		},
		{
			name:  "cancel pending job successfully",
			jobID: "pending-job-123",
			setupMock: func(mock sqlmock.Sqlmock) {
				// GetJob transaction
				mock.ExpectBegin()
				
				rows := sqlmock.NewRows([]string{
					"id", "name", "status", "progress", "total_tasks", "completed_tasks",
					"failed_tasks", "skipped_tasks", "created_at", "started_at", "completed_at",
					"concurrency", "find_links", "include_paths", "exclude_paths", "error_message",
					"required_workers", "found_tasks", "sitemap_tasks", "duration_seconds",
					"avg_time_per_task_seconds",
				}).AddRow(
					"pending-job-123",
					"example.com",
					"pending",
					0.0,
					0,
					0,
					0,
					0,
					time.Now().Add(-5*time.Minute),
					sql.NullTime{Valid: false},
					sql.NullTime{Valid: false},
					10,
					true,
					[]byte(`[]`),
					[]byte(`[]`),
					sql.NullString{Valid: false},
					5,
					0,
					0,
					sql.NullInt64{Valid: false},
					sql.NullFloat64{Valid: false},
				)

				mock.ExpectQuery(`SELECT 
				j.id, d.name, j.status, j.progress, j.total_tasks, j.completed_tasks, j.failed_tasks, j.skipped_tasks,
				j.created_at, j.started_at, j.completed_at, j.concurrency, j.find_links,
				j.include_paths, j.exclude_paths, j.error_message, j.required_workers,
				j.found_tasks, j.sitemap_tasks, j.duration_seconds, j.avg_time_per_task_seconds
			FROM jobs j
			JOIN domains d ON j.domain_id = d.id
			WHERE j.id = \$1`).
					WithArgs("pending-job-123").
					WillReturnRows(rows)

				mock.ExpectCommit()

				// Cancel job transaction
				mock.ExpectBegin()

				mock.ExpectExec(`UPDATE jobs
				SET status = \$1, completed_at = \$2
				WHERE id = \$3`).
					WithArgs("cancelled", sqlmock.AnyArg(), "pending-job-123").
					WillReturnResult(sqlmock.NewResult(1, 1))

				mock.ExpectExec(`UPDATE tasks
				SET status = \$1
				WHERE job_id = \$2 AND status = \$3`).
					WithArgs("skipped", "pending-job-123", "pending").
					WillReturnResult(sqlmock.NewResult(1, 0))

				mock.ExpectCommit()
			},
			expectedError: "",
		},
		{
			name:  "cancel paused job successfully",
			jobID: "paused-job-123",
			setupMock: func(mock sqlmock.Sqlmock) {
				// GetJob transaction
				mock.ExpectBegin()
				
				rows := sqlmock.NewRows([]string{
					"id", "name", "status", "progress", "total_tasks", "completed_tasks",
					"failed_tasks", "skipped_tasks", "created_at", "started_at", "completed_at",
					"concurrency", "find_links", "include_paths", "exclude_paths", "error_message",
					"required_workers", "found_tasks", "sitemap_tasks", "duration_seconds",
					"avg_time_per_task_seconds",
				}).AddRow(
					"paused-job-123",
					"example.com",
					"paused",
					25.0,
					100,
					25,
					0,
					0,
					time.Now().Add(-2*time.Hour),
					sql.NullTime{Time: time.Now().Add(-90*time.Minute), Valid: true},
					sql.NullTime{Valid: false},
					10,
					true,
					[]byte(`[]`),
					[]byte(`[]`),
					sql.NullString{Valid: false},
					5,
					25,
					75,
					sql.NullInt64{Valid: false},
					sql.NullFloat64{Valid: false},
				)

				mock.ExpectQuery(`SELECT 
				j.id, d.name, j.status, j.progress, j.total_tasks, j.completed_tasks, j.failed_tasks, j.skipped_tasks,
				j.created_at, j.started_at, j.completed_at, j.concurrency, j.find_links,
				j.include_paths, j.exclude_paths, j.error_message, j.required_workers,
				j.found_tasks, j.sitemap_tasks, j.duration_seconds, j.avg_time_per_task_seconds
			FROM jobs j
			JOIN domains d ON j.domain_id = d.id
			WHERE j.id = \$1`).
					WithArgs("paused-job-123").
					WillReturnRows(rows)

				mock.ExpectCommit()

				// Cancel job transaction
				mock.ExpectBegin()

				mock.ExpectExec(`UPDATE jobs
				SET status = \$1, completed_at = \$2
				WHERE id = \$3`).
					WithArgs("cancelled", sqlmock.AnyArg(), "paused-job-123").
					WillReturnResult(sqlmock.NewResult(1, 1))

				mock.ExpectExec(`UPDATE tasks
				SET status = \$1
				WHERE job_id = \$2 AND status = \$3`).
					WithArgs("skipped", "paused-job-123", "pending").
					WillReturnResult(sqlmock.NewResult(1, 75))

				mock.ExpectCommit()
			},
			expectedError: "",
		},
		{
			name:  "cannot cancel completed job",
			jobID: "completed-job-123",
			setupMock: func(mock sqlmock.Sqlmock) {
				// GetJob transaction
				mock.ExpectBegin()
				
				rows := sqlmock.NewRows([]string{
					"id", "name", "status", "progress", "total_tasks", "completed_tasks",
					"failed_tasks", "skipped_tasks", "created_at", "started_at", "completed_at",
					"concurrency", "find_links", "include_paths", "exclude_paths", "error_message",
					"required_workers", "found_tasks", "sitemap_tasks", "duration_seconds",
					"avg_time_per_task_seconds",
				}).AddRow(
					"completed-job-123",
					"example.com",
					"completed",
					100.0,
					100,
					100,
					0,
					0,
					time.Now().Add(-2*time.Hour),
					sql.NullTime{Time: time.Now().Add(-90*time.Minute), Valid: true},
					sql.NullTime{Time: time.Now().Add(-30*time.Minute), Valid: true},
					10,
					true,
					[]byte(`[]`),
					[]byte(`[]`),
					sql.NullString{Valid: false},
					5,
					100,
					0,
					sql.NullInt64{Int64: 3600, Valid: true},
					sql.NullFloat64{Float64: 36.0, Valid: true},
				)

				mock.ExpectQuery(`SELECT 
				j.id, d.name, j.status, j.progress, j.total_tasks, j.completed_tasks, j.failed_tasks, j.skipped_tasks,
				j.created_at, j.started_at, j.completed_at, j.concurrency, j.find_links,
				j.include_paths, j.exclude_paths, j.error_message, j.required_workers,
				j.found_tasks, j.sitemap_tasks, j.duration_seconds, j.avg_time_per_task_seconds
			FROM jobs j
			JOIN domains d ON j.domain_id = d.id
			WHERE j.id = \$1`).
					WithArgs("completed-job-123").
					WillReturnRows(rows)

				mock.ExpectCommit()
			},
			expectedError: "job cannot be canceled: completed",
		},
		{
			name:  "cannot cancel failed job",
			jobID: "failed-job-123",
			setupMock: func(mock sqlmock.Sqlmock) {
				// GetJob transaction
				mock.ExpectBegin()
				
				rows := sqlmock.NewRows([]string{
					"id", "name", "status", "progress", "total_tasks", "completed_tasks",
					"failed_tasks", "skipped_tasks", "created_at", "started_at", "completed_at",
					"concurrency", "find_links", "include_paths", "exclude_paths", "error_message",
					"required_workers", "found_tasks", "sitemap_tasks", "duration_seconds",
					"avg_time_per_task_seconds",
				}).AddRow(
					"failed-job-123",
					"example.com",
					"failed",
					50.0,
					100,
					50,
					5,
					0,
					time.Now().Add(-2*time.Hour),
					sql.NullTime{Time: time.Now().Add(-90*time.Minute), Valid: true},
					sql.NullTime{Time: time.Now().Add(-30*time.Minute), Valid: true},
					10,
					true,
					[]byte(`[]`),
					[]byte(`[]`),
					sql.NullString{String: "Too many failures", Valid: true},
					5,
					50,
					50,
					sql.NullInt64{Int64: 3600, Valid: true},
					sql.NullFloat64{Float64: 72.0, Valid: true},
				)

				mock.ExpectQuery(`SELECT 
				j.id, d.name, j.status, j.progress, j.total_tasks, j.completed_tasks, j.failed_tasks, j.skipped_tasks,
				j.created_at, j.started_at, j.completed_at, j.concurrency, j.find_links,
				j.include_paths, j.exclude_paths, j.error_message, j.required_workers,
				j.found_tasks, j.sitemap_tasks, j.duration_seconds, j.avg_time_per_task_seconds
			FROM jobs j
			JOIN domains d ON j.domain_id = d.id
			WHERE j.id = \$1`).
					WithArgs("failed-job-123").
					WillReturnRows(rows)

				mock.ExpectCommit()
			},
			expectedError: "job cannot be canceled: failed",
		},
		{
			name:  "cannot cancel already cancelled job",
			jobID: "cancelled-job-123",
			setupMock: func(mock sqlmock.Sqlmock) {
				// GetJob transaction
				mock.ExpectBegin()
				
				rows := sqlmock.NewRows([]string{
					"id", "name", "status", "progress", "total_tasks", "completed_tasks",
					"failed_tasks", "skipped_tasks", "created_at", "started_at", "completed_at",
					"concurrency", "find_links", "include_paths", "exclude_paths", "error_message",
					"required_workers", "found_tasks", "sitemap_tasks", "duration_seconds",
					"avg_time_per_task_seconds",
				}).AddRow(
					"cancelled-job-123",
					"example.com",
					"cancelled",
					25.0,
					100,
					25,
					0,
					75,
					time.Now().Add(-2*time.Hour),
					sql.NullTime{Time: time.Now().Add(-90*time.Minute), Valid: true},
					sql.NullTime{Time: time.Now().Add(-60*time.Minute), Valid: true},
					10,
					true,
					[]byte(`[]`),
					[]byte(`[]`),
					sql.NullString{Valid: false},
					5,
					25,
					100,
					sql.NullInt64{Int64: 1800, Valid: true},
					sql.NullFloat64{Float64: 72.0, Valid: true},
				)

				mock.ExpectQuery(`SELECT 
				j.id, d.name, j.status, j.progress, j.total_tasks, j.completed_tasks, j.failed_tasks, j.skipped_tasks,
				j.created_at, j.started_at, j.completed_at, j.concurrency, j.find_links,
				j.include_paths, j.exclude_paths, j.error_message, j.required_workers,
				j.found_tasks, j.sitemap_tasks, j.duration_seconds, j.avg_time_per_task_seconds
			FROM jobs j
			JOIN domains d ON j.domain_id = d.id
			WHERE j.id = \$1`).
					WithArgs("cancelled-job-123").
					WillReturnRows(rows)

				mock.ExpectCommit()
			},
			expectedError: "job cannot be canceled: cancelled",
		},
		{
			name:  "job not found",
			jobID: "non-existent-job",
			setupMock: func(mock sqlmock.Sqlmock) {
				// GetJob transaction
				mock.ExpectBegin()
				
				mock.ExpectQuery(`SELECT 
				j.id, d.name, j.status, j.progress, j.total_tasks, j.completed_tasks, j.failed_tasks, j.skipped_tasks,
				j.created_at, j.started_at, j.completed_at, j.concurrency, j.find_links,
				j.include_paths, j.exclude_paths, j.error_message, j.required_workers,
				j.found_tasks, j.sitemap_tasks, j.duration_seconds, j.avg_time_per_task_seconds
			FROM jobs j
			JOIN domains d ON j.domain_id = d.id
			WHERE j.id = \$1`).
					WithArgs("non-existent-job").
					WillReturnError(sql.ErrNoRows)

				mock.ExpectRollback()
			},
			expectedError: "job not found",
		},
		{
			name:  "database error during GetJob",
			jobID: "test-job-123",
			setupMock: func(mock sqlmock.Sqlmock) {
				// GetJob transaction
				mock.ExpectBegin()
				
				mock.ExpectQuery(`SELECT 
				j.id, d.name, j.status, j.progress, j.total_tasks, j.completed_tasks, j.failed_tasks, j.skipped_tasks,
				j.created_at, j.started_at, j.completed_at, j.concurrency, j.find_links,
				j.include_paths, j.exclude_paths, j.error_message, j.required_workers,
				j.found_tasks, j.sitemap_tasks, j.duration_seconds, j.avg_time_per_task_seconds
			FROM jobs j
			JOIN domains d ON j.domain_id = d.id
			WHERE j.id = \$1`).
					WithArgs("test-job-123").
					WillReturnError(errors.New("database connection failed"))

				mock.ExpectRollback()
			},
			expectedError: "failed to get job",
		},
		{
			name:  "database error during status update",
			jobID: "running-job-456",
			setupMock: func(mock sqlmock.Sqlmock) {
				// GetJob transaction - successful
				mock.ExpectBegin()
				
				rows := sqlmock.NewRows([]string{
					"id", "name", "status", "progress", "total_tasks", "completed_tasks",
					"failed_tasks", "skipped_tasks", "created_at", "started_at", "completed_at",
					"concurrency", "find_links", "include_paths", "exclude_paths", "error_message",
					"required_workers", "found_tasks", "sitemap_tasks", "duration_seconds",
					"avg_time_per_task_seconds",
				}).AddRow(
					"running-job-456",
					"example.com",
					"running",
					50.0,
					100,
					50,
					0,
					0,
					time.Now().Add(-1*time.Hour),
					sql.NullTime{Time: time.Now().Add(-30*time.Minute), Valid: true},
					sql.NullTime{Valid: false},
					10,
					true,
					[]byte(`[]`),
					[]byte(`[]`),
					sql.NullString{Valid: false},
					5,
					50,
					50,
					sql.NullInt64{Valid: false},
					sql.NullFloat64{Valid: false},
				)

				mock.ExpectQuery(`SELECT 
				j.id, d.name, j.status, j.progress, j.total_tasks, j.completed_tasks, j.failed_tasks, j.skipped_tasks,
				j.created_at, j.started_at, j.completed_at, j.concurrency, j.find_links,
				j.include_paths, j.exclude_paths, j.error_message, j.required_workers,
				j.found_tasks, j.sitemap_tasks, j.duration_seconds, j.avg_time_per_task_seconds
			FROM jobs j
			JOIN domains d ON j.domain_id = d.id
			WHERE j.id = \$1`).
					WithArgs("running-job-456").
					WillReturnRows(rows)

				mock.ExpectCommit()

				// Cancel job transaction - fails
				mock.ExpectBegin()

				mock.ExpectExec(`UPDATE jobs
				SET status = \$1, completed_at = \$2
				WHERE id = \$3`).
					WithArgs("cancelled", sqlmock.AnyArg(), "running-job-456").
					WillReturnError(errors.New("database lock timeout"))

				mock.ExpectRollback()
			},
			expectedError: "", // Error is logged but not returned in implementation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()

			// Setup expectations
			tt.setupMock(mock)

			// Create DbQueue with mock DB
			dbQueue := &MockDbQueueWithTransaction{
				db:   mockDB,
				mock: mock,
			}

			// Create mock worker pool
			workerPool := &WorkerPool{
				jobs:           make(map[string]bool),
				stopCh:         make(chan struct{}),
				notifyCh:       make(chan struct{}, 1),
				jobPerformance: make(map[string]*JobPerformance),
			}

			// Create JobManager
			jm := &JobManager{
				db:             mockDB,
				dbQueue:        dbQueue,
				workerPool:     workerPool,
				processedPages: make(map[string]struct{}),
			}

			// Execute
			ctx := context.Background()
			err = jm.CancelJob(ctx, tt.jobID)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
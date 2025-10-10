//go:build unit || !integration

package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartJob_Unit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		jobID         string
		setupMock     func(sqlmock.Sqlmock)
		expectedError string
	}{
		{
			name:  "restart completed job successfully",
			jobID: "completed-job-123",
			setupMock: func(mock sqlmock.Sqlmock) {
				// GetJob transaction
				mock.ExpectBegin()

				// Query for the original job
				rows := sqlmock.NewRows([]string{
					"id", "name", "status", "progress", "total_tasks", "completed_tasks",
					"failed_tasks", "skipped_tasks", "created_at", "started_at", "completed_at",
					"concurrency", "find_links", "include_paths", "exclude_paths", "error_message",
					"required_workers", "found_tasks", "sitemap_tasks", "duration_seconds",
					"avg_time_per_task_seconds",
				}).AddRow(
					"completed-job-123",          // id
					"example.com",                // domain name
					"completed",                  // status
					100.0,                        // progress
					100,                          // total_tasks
					100,                          // completed_tasks
					0,                            // failed_tasks
					0,                            // skipped_tasks
					time.Now().Add(-2*time.Hour), // created_at
					sql.NullTime{Time: time.Now().Add(-90 * time.Minute), Valid: true}, // started_at
					sql.NullTime{Time: time.Now().Add(-30 * time.Minute), Valid: true}, // completed_at
					10,                                      // concurrency
					true,                                    // find_links
					json.RawMessage(`["/*"]`),               // include_paths
					json.RawMessage(`["/admin/*"]`),         // exclude_paths
					sql.NullString{Valid: false},            // error_message
					5,                                       // required_workers
					50,                                      // found_tasks
					50,                                      // sitemap_tasks
					sql.NullInt64{Int64: 3600, Valid: true}, // duration_seconds
					sql.NullFloat64{Float64: 36.0, Valid: true}, // avg_time_per_task_seconds
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

				// CreateJob transaction for the new job
				mock.ExpectBegin()

				// Get or create domain
				domainRow := sqlmock.NewRows([]string{"id"}).AddRow(1)
				mock.ExpectQuery(`INSERT INTO domains\(name\) VALUES\(\$1\)`).
					WithArgs("example.com").
					WillReturnRows(domainRow)

				// Insert new job
				mock.ExpectExec(`INSERT INTO jobs`).
					WithArgs(
						sqlmock.AnyArg(), // new job ID (UUID)
						1,                // domain_id
						nil,              // user_id
						nil,              // organisation_id
						"pending",        // status
						0.0,              // progress
						0,                // total_tasks
						0,                // completed_tasks
						0,                // failed_tasks
						0,                // skipped_tasks
						sqlmock.AnyArg(), // created_at
						10,               // concurrency
						true,             // find_links
						`["/*"]`,         // include_paths (string, not []byte)
						`["/admin/*"]`,   // exclude_paths (string, not []byte)
						5,                // required_workers
						0,                // max_pages (from original job which had MaxPages=0)
						0,                // found_tasks
						0,                // sitemap_tasks
						nil,              // source_type
						nil,              // source_detail
						nil,              // source_info
					).
					WillReturnResult(sqlmock.NewResult(1, 1))

				mock.ExpectCommit()
			},
			expectedError: "",
		},
		{
			name:  "restart failed job successfully",
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
					sql.NullTime{Time: time.Now().Add(-90 * time.Minute), Valid: true},
					sql.NullTime{Time: time.Now().Add(-30 * time.Minute), Valid: true},
					10,
					true,
					json.RawMessage(`[]`),
					json.RawMessage(`[]`),
					sql.NullString{String: "Connection timeout", Valid: true},
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

				// CreateJob transaction
				mock.ExpectBegin()

				domainRow := sqlmock.NewRows([]string{"id"}).AddRow(1)
				mock.ExpectQuery(`INSERT INTO domains\(name\) VALUES\(\$1\)`).
					WithArgs("example.com").
					WillReturnRows(domainRow)

				mock.ExpectExec(`INSERT INTO jobs`).
					WithArgs(
						sqlmock.AnyArg(), // new job ID
						1,                // domain_id
						nil,              // user_id
						nil,              // organisation_id
						"pending",
						0.0,
						0,
						0,
						0,
						0,
						sqlmock.AnyArg(),
						10,
						true,
						`[]`, // include_paths (string, not []byte)
						`[]`, // exclude_paths (string, not []byte)
						5,
						0,
						0,
						0,
						nil,
						nil,
						nil,
					).
					WillReturnResult(sqlmock.NewResult(1, 1))

				mock.ExpectCommit()
			},
			expectedError: "",
		},
		{
			name:  "restart cancelled job successfully",
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
					sql.NullTime{Time: time.Now().Add(-90 * time.Minute), Valid: true},
					sql.NullTime{Time: time.Now().Add(-60 * time.Minute), Valid: true},
					10,
					true,
					json.RawMessage(`[]`),
					json.RawMessage(`[]`),
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

				// CreateJob transaction
				mock.ExpectBegin()

				domainRow := sqlmock.NewRows([]string{"id"}).AddRow(1)
				mock.ExpectQuery(`INSERT INTO domains\(name\) VALUES\(\$1\)`).
					WithArgs("example.com").
					WillReturnRows(domainRow)

				mock.ExpectExec(`INSERT INTO jobs`).
					WithArgs(
						sqlmock.AnyArg(),
						1,
						nil,
						nil,
						"pending",
						0.0,
						0,
						0,
						0,
						0,
						sqlmock.AnyArg(),
						10,
						true,
						`[]`, // include_paths (string)
						`[]`, // exclude_paths (string)
						5,
						0,
						0,
						0,
						nil,
						nil,
						nil,
					).
					WillReturnResult(sqlmock.NewResult(1, 1))

				mock.ExpectCommit()
			},
			expectedError: "",
		},
		{
			name:  "cannot restart running job",
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
					sql.NullTime{Time: time.Now().Add(-30 * time.Minute), Valid: true},
					sql.NullTime{Valid: false},
					10,
					true,
					json.RawMessage(`[]`),
					json.RawMessage(`[]`),
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
			},
			expectedError: "job cannot be restarted: running",
		},
		{
			name:  "cannot restart pending job",
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
					json.RawMessage(`[]`),
					json.RawMessage(`[]`),
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
			},
			expectedError: "job cannot be restarted: pending",
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
			expectedError: "failed to get original job",
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

			// Create JobManager with nil worker pool
			// StartJob calls workerPool.AddJob which needs database access,
			// so we'll use nil and let it skip that step
			jm := &JobManager{
				db:             mockDB,
				dbQueue:        dbQueue,
				workerPool:     nil, // Set to nil to skip AddJob call
				processedPages: make(map[string]struct{}),
			}

			// Execute
			ctx := context.Background()
			err = jm.StartJob(ctx, tt.jobID)

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

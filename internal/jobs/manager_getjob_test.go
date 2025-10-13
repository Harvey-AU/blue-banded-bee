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

func TestGetJob_Unit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		jobID         string
		setupMock     func(sqlmock.Sqlmock)
		expectedJob   *Job
		expectedError string
	}{
		{
			name:  "successful job retrieval",
			jobID: "test-job-123",
			setupMock: func(mock sqlmock.Sqlmock) {
				// Mock the transaction
				mock.ExpectBegin()

				// Mock the job query
				rows := sqlmock.NewRows([]string{
					"id", "name", "status", "progress", "total_tasks", "completed_tasks",
					"failed_tasks", "skipped_tasks", "created_at", "started_at", "completed_at",
					"concurrency", "find_links", "include_paths", "exclude_paths", "error_message",
					"required_workers", "found_tasks", "sitemap_tasks", "duration_seconds",
					"avg_time_per_task_seconds",
				}).AddRow(
					"test-job-123",               // id
					"example.com",                // domain name
					"running",                    // status
					50.0,                         // progress
					100,                          // total_tasks
					50,                           // completed_tasks
					5,                            // failed_tasks
					0,                            // skipped_tasks
					time.Now().Add(-1*time.Hour), // created_at
					sql.NullTime{Time: time.Now().Add(-30 * time.Minute), Valid: true}, // started_at
					sql.NullTime{Valid: false},              // completed_at (null)
					10,                                      // concurrency
					true,                                    // find_links
					json.RawMessage(`["/*"]`),               // include_paths
					json.RawMessage(`["/admin/*"]`),         // exclude_paths
					sql.NullString{Valid: false},            // error_message (null)
					5,                                       // required_workers
					20,                                      // found_tasks
					80,                                      // sitemap_tasks
					sql.NullInt64{Int64: 1800, Valid: true}, // duration_seconds (INTEGER)
					sql.NullFloat64{Float64: 18.0, Valid: true}, // avg_time_per_task_seconds (NUMERIC)
				)

				mock.ExpectQuery(`SELECT 
				j.id, d.name, j.status, j.progress, j.total_tasks, j.completed_tasks, j.failed_tasks, j.skipped_tasks,
				j.created_at, j.started_at, j.completed_at, j.concurrency, j.find_links,
				j.include_paths, j.exclude_paths, j.error_message, j.required_workers,
				j.found_tasks, j.sitemap_tasks, j.duration_seconds, j.avg_time_per_task_seconds
			FROM jobs j
			JOIN domains d ON j.domain_id = d.id
			WHERE j.id = \$1`).
					WithArgs("test-job-123").
					WillReturnRows(rows)

				mock.ExpectCommit()
			},
			expectedJob: &Job{
				ID:                    "test-job-123",
				Domain:                "example.com",
				Status:                JobStatusRunning,
				Progress:              50.0,
				TotalTasks:            100,
				CompletedTasks:        50,
				FailedTasks:           5,
				SkippedTasks:          0,
				Concurrency:           10,
				FindLinks:             true,
				IncludePaths:          []string{"/*"},
				ExcludePaths:          []string{"/admin/*"},
				RequiredWorkers:       5,
				FoundTasks:            20,
				SitemapTasks:          80,
				DurationSeconds:       intPtr(1800),
				AvgTimePerTaskSeconds: float64Ptr(18.0),
			},
		},
		{
			name:  "job not found",
			jobID: "non-existent-job",
			setupMock: func(mock sqlmock.Sqlmock) {
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
			expectedError: "job non-existent-job not found",
		},
		{
			name:  "database error",
			jobID: "test-job-123",
			setupMock: func(mock sqlmock.Sqlmock) {
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
			expectedError: "failed to get job: database connection failed",
		},
		{
			name:  "job with error message",
			jobID: "failed-job-123",
			setupMock: func(mock sqlmock.Sqlmock) {
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
					0.0,
					0,
					0,
					0,
					0,
					time.Now().Add(-1*time.Hour),
					sql.NullTime{Time: time.Now().Add(-30 * time.Minute), Valid: true},
					sql.NullTime{Time: time.Now().Add(-15 * time.Minute), Valid: true},
					5,
					false,
					json.RawMessage(`[]`),
					json.RawMessage(`[]`),
					sql.NullString{String: "Failed to connect to domain", Valid: true},
					1,
					0,
					0,
					sql.NullInt64{Int64: 900, Valid: true},
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
					WithArgs("failed-job-123").
					WillReturnRows(rows)

				mock.ExpectCommit()
			},
			expectedJob: &Job{
				ID:              "failed-job-123",
				Domain:          "example.com",
				Status:          JobStatusFailed,
				Progress:        0.0,
				TotalTasks:      0,
				CompletedTasks:  0,
				FailedTasks:     0,
				SkippedTasks:    0,
				Concurrency:     5,
				FindLinks:       false,
				IncludePaths:    []string{},
				ExcludePaths:    []string{},
				ErrorMessage:    "Failed to connect to domain",
				RequiredWorkers: 1,
				FoundTasks:      0,
				SitemapTasks:    0,
				DurationSeconds: intPtr(900),
			},
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
			// We need a custom implementation that uses our mock
			dbQueue := &MockDbQueueWithTransaction{
				db:   mockDB,
				mock: mock,
			}

			// Create JobManager
			jm := &JobManager{
				db:      mockDB,
				dbQueue: dbQueue,
			}

			// Execute
			ctx := context.Background()
			job, err := jm.GetJob(ctx, tt.jobID)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, job)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, job)
				if job != nil {
					// Compare key fields
					assert.Equal(t, tt.expectedJob.ID, job.ID)
					assert.Equal(t, tt.expectedJob.Domain, job.Domain)
					assert.Equal(t, tt.expectedJob.Status, job.Status)
					assert.Equal(t, tt.expectedJob.Progress, job.Progress)
					assert.Equal(t, tt.expectedJob.TotalTasks, job.TotalTasks)
					assert.Equal(t, tt.expectedJob.CompletedTasks, job.CompletedTasks)
					assert.Equal(t, tt.expectedJob.ErrorMessage, job.ErrorMessage)

					// Check arrays
					assert.Equal(t, tt.expectedJob.IncludePaths, job.IncludePaths)
					assert.Equal(t, tt.expectedJob.ExcludePaths, job.ExcludePaths)
				}
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

package db

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

// TestDbQueueExecute tests the Execute transaction method
func TestDbQueueExecute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupMock func(sqlmock.Sqlmock)
		fn        func(*sql.Tx) error
		wantErr   bool
		errMsg    string
	}{
		{
			name: "successful transaction",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectCommit()
			},
			fn: func(tx *sql.Tx) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "begin transaction fails",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(errors.New("connection lost"))
			},
			fn: func(tx *sql.Tx) error {
				return nil
			},
			wantErr: true,
			errMsg:  "failed to begin transaction",
		},
		{
			name: "function returns error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectRollback()
			},
			fn: func(tx *sql.Tx) error {
				return errors.New("operation failed")
			},
			wantErr: true,
			errMsg:  "operation failed",
		},
		{
			name: "commit fails",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
				mock.ExpectRollback()
			},
			fn: func(tx *sql.Tx) error {
				return nil
			},
			wantErr: true,
			errMsg:  "failed to commit transaction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			sqlDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer sqlDB.Close()

			// Setup expectations
			tt.setupMock(mock)

			// Create DbQueue with mock
			db := &DB{client: sqlDB}
			q := NewDbQueue(db)

			// Execute
			ctx := context.Background()
			err = q.Execute(ctx, tt.fn)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestDbQueueGetNextTask tests the GetNextTask method with mocks
func TestDbQueueGetNextTask(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		jobID     string
		setupMock func(sqlmock.Sqlmock)
		wantTask  bool
		wantErr   bool
	}{
		{
			name:  "successful task retrieval",
			jobID: "test-job",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				
				// Expect SELECT query
				rows := sqlmock.NewRows([]string{
					"id", "job_id", "page_id", "path", "created_at", 
					"retry_count", "source_type", "source_url", "priority_score",
				}).AddRow(
					"task-1", "test-job", 1, "/page", fixedTime,
					0, "sitemap", "https://example.com/sitemap.xml", 1.0,
				)
				
				mock.ExpectQuery("SELECT id, job_id, page_id, path, created_at, retry_count, source_type, source_url, priority_score FROM tasks WHERE status = 'pending' AND job_id = \\$1").
					WithArgs("test-job").
					WillReturnRows(rows)

				// Expect UPDATE query
				mock.ExpectExec("UPDATE tasks SET status = \\$1, started_at = \\$2 WHERE id = \\$3").
					WithArgs("running", sqlmock.AnyArg(), "task-1").
					WillReturnResult(sqlmock.NewResult(0, 1))

				mock.ExpectCommit()
			},
			wantTask: true,
			wantErr:  false,
		},
		{
			name:  "no tasks available",
			jobID: "test-job",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				
				mock.ExpectQuery("SELECT id, job_id, page_id, path, created_at, retry_count, source_type, source_url, priority_score FROM tasks WHERE status = 'pending' AND job_id = \\$1").
					WithArgs("test-job").
					WillReturnError(sql.ErrNoRows)

				mock.ExpectRollback()
			},
			wantTask: false,
			wantErr:  true,
		},
		{
			name:  "without job filter",
			jobID: "",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				
				rows := sqlmock.NewRows([]string{
					"id", "job_id", "page_id", "path", "created_at",
					"retry_count", "source_type", "source_url", "priority_score",
				}).AddRow(
					"task-2", "any-job", 2, "/other", fixedTime,
					1, "discovery", "https://example.com", 0.5,
				)

				// Query without job_id filter
				mock.ExpectQuery("SELECT id, job_id, page_id, path, created_at, retry_count, source_type, source_url, priority_score FROM tasks WHERE status = 'pending'").
					WillReturnRows(rows)

				mock.ExpectExec("UPDATE tasks SET status = \\$1, started_at = \\$2 WHERE id = \\$3").
					WithArgs("running", sqlmock.AnyArg(), "task-2").
					WillReturnResult(sqlmock.NewResult(0, 1))

				mock.ExpectCommit()
			},
			wantTask: true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			sqlDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer sqlDB.Close()

			// Setup expectations
			tt.setupMock(mock)

			// Create DbQueue with mock
			db := &DB{client: sqlDB}
			q := NewDbQueue(db)

			// Execute
			ctx := context.Background()
			task, err := q.GetNextTask(ctx, tt.jobID)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, task)
			} else {
				assert.NoError(t, err)
				if tt.wantTask {
					assert.NotNil(t, task)
					assert.NotEmpty(t, task.ID)
					assert.NotEmpty(t, task.JobID)
				}
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestDbQueueUpdateTaskStatus tests the UpdateTaskStatus method
func TestDbQueueUpdateTaskStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		taskID    string
		status    string
		setupMock func(sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name:   "successful status update to completed",
			taskID: "task-1",
			status: "completed",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				
				mock.ExpectExec("UPDATE tasks SET status = \\$1, completed_at = \\$2 WHERE id = \\$3").
					WithArgs("completed", sqlmock.AnyArg(), "task-1").
					WillReturnResult(sqlmock.NewResult(0, 1))

				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name:   "successful status update to failed",
			taskID: "task-2",
			status: "failed",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				
				mock.ExpectExec("UPDATE tasks SET status = \\$1, completed_at = \\$2, error = \\$3 WHERE id = \\$4").
					WithArgs("failed", sqlmock.AnyArg(), sqlmock.AnyArg(), "task-2").
					WillReturnResult(sqlmock.NewResult(0, 1))

				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name:   "task not found",
			taskID: "non-existent",
			status: "completed",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				
				mock.ExpectExec("UPDATE tasks SET status = \\$1, completed_at = \\$2 WHERE id = \\$3").
					WithArgs("completed", sqlmock.AnyArg(), "non-existent").
					WillReturnResult(sqlmock.NewResult(0, 0))

				mock.ExpectCommit()
			},
			wantErr: false, // No error but no rows affected
		},
		{
			name:   "database error",
			taskID: "task-3",
			status: "completed",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				
				mock.ExpectExec("UPDATE tasks SET status = \\$1, completed_at = \\$2 WHERE id = \\$3").
					WithArgs("completed", sqlmock.AnyArg(), "task-3").
					WillReturnError(errors.New("database error"))

				mock.ExpectRollback()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			sqlDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer sqlDB.Close()

			// Setup expectations
			tt.setupMock(mock)

			// Create DbQueue with mock
			db := &DB{client: sqlDB}
			q := NewDbQueue(db)

			// Execute
			ctx := context.Background()
			err = q.UpdateTaskStatus(ctx, tt.taskID, tt.status, nil, nil)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestDbQueueEnqueueURLs tests the EnqueueURLs method
func TestDbQueueEnqueueURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		jobID     string
		pages     []Page
		setupMock func(sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name:  "successful enqueue single URL",
			jobID: "job-1",
			pages: []Page{
				{ID: 1, Path: "/page1", Priority: 1.0},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				// Expect INSERT for single task
				mock.ExpectExec("INSERT INTO tasks").
					WithArgs(
						sqlmock.AnyArg(), // task ID
						"job-1",           // job ID
						1,                 // page ID
						"pending",         // status
						"manual",          // source type
						"",                // source URL
						1.0,               // priority
					).
					WillReturnResult(sqlmock.NewResult(1, 1))

				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name:  "successful enqueue multiple URLs",
			jobID: "job-2",
			pages: []Page{
				{ID: 1, Path: "/page1", Priority: 1.0},
				{ID: 2, Path: "/page2", Priority: 0.9},
				{ID: 3, Path: "/page3", Priority: 0.8},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				// Expect batch INSERT
				mock.ExpectExec("INSERT INTO tasks").
					WillReturnResult(sqlmock.NewResult(3, 3))

				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name:  "empty pages list",
			jobID: "job-3",
			pages: []Page{},
			setupMock: func(mock sqlmock.Sqlmock) {
				// No database operations expected for empty list
			},
			wantErr: false,
		},
		{
			name:  "database error",
			jobID: "job-4",
			pages: []Page{
				{ID: 1, Path: "/page1", Priority: 1.0},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectExec("INSERT INTO tasks").
					WillReturnError(errors.New("constraint violation"))

				mock.ExpectRollback()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			sqlDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer sqlDB.Close()

			// Setup expectations
			tt.setupMock(mock)

			// Create DbQueue with mock
			db := &DB{client: sqlDB}
			q := NewDbQueue(db)

			// Execute
			ctx := context.Background()
			err = q.EnqueueURLs(ctx, tt.jobID, tt.pages, "manual", "")

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
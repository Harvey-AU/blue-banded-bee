//go:build unit || !integration

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

				// Expect new complex CTE query with job join and concurrency check
				rows := sqlmock.NewRows([]string{
					"id", "job_id", "page_id", "path", "created_at",
					"retry_count", "source_type", "source_url", "priority_score",
				}).AddRow(
					"task-1", "test-job", 1, "/page", fixedTime,
					0, "sitemap", "https://example.com/sitemap.xml", 1.0,
				)

				// Updated regex to match new query with job join and running_tasks increment
				mock.ExpectQuery(`WITH next_task AS \(.*SELECT.*FROM tasks t.*INNER JOIN jobs j.*WHERE.*status = 'pending'.*AND.*job_id.*FOR UPDATE OF.*\).*task_update AS \(.*UPDATE tasks.*\).*UPDATE jobs.*SET running_tasks.*RETURNING`).
					WithArgs(sqlmock.AnyArg(), "test-job").
					WillReturnRows(rows)

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

				// Updated regex to match new query structure
				mock.ExpectQuery(`WITH next_task AS \(.*SELECT.*FROM tasks t.*INNER JOIN jobs j.*WHERE.*status = 'pending'.*AND.*job_id.*FOR UPDATE OF.*\).*task_update AS \(.*UPDATE tasks.*\).*UPDATE jobs.*SET running_tasks.*RETURNING`).
					WithArgs(sqlmock.AnyArg(), "test-job").
					WillReturnError(sql.ErrNoRows)

				mock.ExpectRollback()
			},
			wantTask: false,
			wantErr:  false,
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

				// Query without job_id filter - updated regex to match new structure
				mock.ExpectQuery(`WITH next_task AS \(.*SELECT.*FROM tasks t.*INNER JOIN jobs j.*WHERE.*status = 'pending'.*FOR UPDATE OF.*\).*task_update AS \(.*UPDATE tasks.*\).*UPDATE jobs.*SET running_tasks.*RETURNING`).
					WithArgs(sqlmock.AnyArg()).
					WillReturnRows(rows)

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
					require.NotNil(t, task)
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
		task      *Task
		setupMock func(sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name: "successful status update to running",
			task: &Task{
				ID:     "task-1",
				Status: "running",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				// Running now uses CTE to increment running_tasks
				rows := sqlmock.NewRows([]string{"job_id"}).AddRow("test-job")
				mock.ExpectQuery(`WITH task_update AS.*UPDATE tasks.*SET status.*started_at.*WHERE id.*RETURNING job_id.*UPDATE jobs.*SET running_tasks = running_tasks \+ 1.*RETURNING`).
					WithArgs("running", sqlmock.AnyArg(), "task-1").
					WillReturnRows(rows)
				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name: "successful status update to failed",
			task: &Task{
				ID:         "task-2",
				Status:     "failed",
				Error:      "boom",
				RetryCount: 2,
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				// Now uses CTE with RETURNING to decrement running_tasks
				// Failed status has 5 params: status, completed_at, error, retry_count, id
				rows := sqlmock.NewRows([]string{"job_id"}).AddRow("test-job")
				mock.ExpectQuery(`WITH task_update AS.*UPDATE tasks.*SET status.*completed_at.*error.*retry_count.*WHERE id.*RETURNING job_id.*UPDATE jobs.*SET running_tasks = GREATEST.*RETURNING`).
					WithArgs("failed", sqlmock.AnyArg(), "boom", 2, "task-2").
					WillReturnRows(rows)
				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name: "task not found (running)",
			task: &Task{
				ID:     "non-existent",
				Status: "running",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				// Running uses CTE - return sql.ErrNoRows when task not found
				mock.ExpectQuery(`WITH task_update AS.*UPDATE tasks.*SET status.*started_at.*WHERE id.*RETURNING job_id.*UPDATE jobs.*SET running_tasks = running_tasks \+ 1.*RETURNING`).
					WithArgs("running", sqlmock.AnyArg(), "non-existent").
					WillReturnError(sql.ErrNoRows)
				mock.ExpectRollback()
			},
			wantErr: true,
		},
		{
			name: "database error (running)",
			task: &Task{
				ID:     "task-3",
				Status: "running",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				// Running uses CTE
				mock.ExpectQuery(`WITH task_update AS.*UPDATE tasks.*SET status.*started_at.*WHERE id.*RETURNING job_id.*UPDATE jobs.*SET running_tasks = running_tasks \+ 1.*RETURNING`).
					WithArgs("running", sqlmock.AnyArg(), "task-3").
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
			err = q.UpdateTaskStatus(ctx, tt.task)

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

				// Expect SELECT of job limits and current tasks
				mock.ExpectQuery(`SELECT max_pages,\s+COALESCE\(\(SELECT COUNT\(\*\) FROM tasks WHERE job_id = \$1 AND status != 'skipped'\), 0\)\s+FROM jobs WHERE id = \$1`).
					WithArgs("job-1").
					WillReturnRows(sqlmock.NewRows([]string{"max_pages", "count"}).AddRow(0, 0))

				// Expect direct Exec (no prepared statement for Supabase pooler compatibility)
				mock.ExpectExec("INSERT INTO tasks ").
					WithArgs(
						sqlmock.AnyArg(), // id
						"job-1",          // job_id
						1,                // page_id
						"/page1",         // path
						"pending",        // status
						sqlmock.AnyArg(), // created_at
						0,                // retry_count
						"manual",         // source_type
						"",               // source_url
						1.0,              // priority_score
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

				mock.ExpectQuery(`SELECT max_pages,\s+COALESCE\(\(SELECT COUNT\(\*\) FROM tasks WHERE job_id = \$1 AND status != 'skipped'\), 0\)\s+FROM jobs WHERE id = \$1`).
					WithArgs("job-2").
					WillReturnRows(sqlmock.NewRows([]string{"max_pages", "count"}).AddRow(0, 0))

				// Expect three direct execs (no prepared statement for Supabase pooler compatibility)
				mock.ExpectExec("INSERT INTO tasks ").
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("INSERT INTO tasks ").
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("INSERT INTO tasks ").
					WillReturnResult(sqlmock.NewResult(1, 1))

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

				mock.ExpectQuery(`SELECT max_pages,\s+COALESCE\(\(SELECT COUNT\(\*\) FROM tasks WHERE job_id = \$1 AND status != 'skipped'\), 0\)\s+FROM jobs WHERE id = \$1`).
					WithArgs("job-4").
					WillReturnRows(sqlmock.NewRows([]string{"max_pages", "count"}).AddRow(0, 0))

				// Expect direct Exec (no prepared statement)
				mock.ExpectExec("INSERT INTO tasks ").
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

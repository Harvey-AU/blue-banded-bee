package api

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatTasksFromRows(t *testing.T) {
	t.Run("single_task_with_all_fields", func(t *testing.T) {
		// Create mock database
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		columns := []string{
			"id", "job_id", "path", "domain", "status", "status_code", "response_time",
			"cache_status", "second_response_time", "second_cache_status", "content_type",
			"error", "source_type", "source_url", "created_at", "started_at", "completed_at", "retry_count",
		}

		createdTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		startedTime := time.Date(2024, 1, 1, 12, 5, 0, 0, time.UTC)
		completedTime := time.Date(2024, 1, 1, 12, 10, 0, 0, time.UTC)

		rows := sqlmock.NewRows(columns).AddRow(
			"task-123",                                    // t.id
			"job-456",                                     // t.job_id
			"/page1",                                      // p.path
			"example.com",                                 // d.name (domain)
			"completed",                                   // t.status
			sql.NullInt32{Int32: 200, Valid: true},       // t.status_code
			sql.NullInt32{Int32: 250, Valid: true},       // t.response_time
			sql.NullString{String: "HIT", Valid: true},   // t.cache_status
			sql.NullInt32{Int32: 180, Valid: true},       // t.second_response_time
			sql.NullString{String: "MISS", Valid: true},  // t.second_cache_status
			sql.NullString{String: "text/html", Valid: true}, // t.content_type
			sql.NullString{Valid: false},                 // t.error
			sql.NullString{String: "sitemap", Valid: true}, // t.source_type
			sql.NullString{String: "https://example.com/sitemap.xml", Valid: true}, // t.source_url
			sql.NullTime{Time: createdTime, Valid: true}, // t.created_at
			sql.NullTime{Time: startedTime, Valid: true}, // t.started_at
			sql.NullTime{Time: completedTime, Valid: true}, // t.completed_at
			2, // t.retry_count
		)

		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		actualRows, err := db.Query("SELECT * FROM test")
		require.NoError(t, err)

		result, err := formatTasksFromRows(actualRows)
		require.NoError(t, err)
		require.Len(t, result, 1)

		task := result[0]
		assert.Equal(t, "task-123", task.ID)
		assert.Equal(t, "job-456", task.JobID)
		assert.Equal(t, "/page1", task.Path)
		assert.Equal(t, "https://example.com/page1", task.URL)
		assert.Equal(t, "completed", task.Status)
		assert.Equal(t, 2, task.RetryCount)
		
		// Check non-null pointer fields
		require.NotNil(t, task.StatusCode)
		assert.Equal(t, 200, *task.StatusCode)
		
		require.NotNil(t, task.ResponseTime)
		assert.Equal(t, 250, *task.ResponseTime)
		
		require.NotNil(t, task.CacheStatus)
		assert.Equal(t, "HIT", *task.CacheStatus)
		
		// Check null field
		assert.Nil(t, task.Error)
		
		// Check time formatting
		assert.Equal(t, "2024-01-01T12:00:00Z", task.CreatedAt)
		require.NotNil(t, task.StartedAt)
		assert.Equal(t, "2024-01-01T12:05:00Z", *task.StartedAt)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("task_with_minimal_null_fields", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		columns := []string{
			"id", "job_id", "path", "domain", "status", "status_code", "response_time",
			"cache_status", "second_response_time", "second_cache_status", "content_type",
			"error", "source_type", "source_url", "created_at", "started_at", "completed_at", "retry_count",
		}

		createdTime := time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC)

		rows := sqlmock.NewRows(columns).AddRow(
			"task-789",                                   // t.id
			"job-abc",                                    // t.job_id
			"/page2",                                     // p.path
			"test.com",                                   // d.name (domain)
			"pending",                                    // t.status
			sql.NullInt32{Valid: false},                 // t.status_code (null)
			sql.NullInt32{Valid: false},                 // t.response_time (null)
			sql.NullString{Valid: false},                // t.cache_status (null)
			sql.NullInt32{Valid: false},                 // t.second_response_time (null)
			sql.NullString{Valid: false},                // t.second_cache_status (null)
			sql.NullString{Valid: false},                // t.content_type (null)
			sql.NullString{Valid: false},                // t.error (null)
			sql.NullString{Valid: false},                // t.source_type (null)
			sql.NullString{Valid: false},                // t.source_url (null)
			sql.NullTime{Time: createdTime, Valid: true}, // t.created_at
			sql.NullTime{Valid: false},                  // t.started_at (null)
			sql.NullTime{Valid: false},                  // t.completed_at (null)
			0, // t.retry_count
		)

		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		actualRows, err := db.Query("SELECT * FROM test")
		require.NoError(t, err)

		result, err := formatTasksFromRows(actualRows)
		require.NoError(t, err)
		require.Len(t, result, 1)

		task := result[0]
		assert.Equal(t, "task-789", task.ID)
		assert.Equal(t, "https://test.com/page2", task.URL)
		assert.Equal(t, "pending", task.Status)
		assert.Equal(t, 0, task.RetryCount)
		assert.Equal(t, "2024-01-02T10:00:00Z", task.CreatedAt)

		// Verify null fields are nil
		assert.Nil(t, task.StatusCode)
		assert.Nil(t, task.ResponseTime)
		assert.Nil(t, task.CacheStatus)
		assert.Nil(t, task.Error)
		assert.Nil(t, task.StartedAt)
		assert.Nil(t, task.CompletedAt)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("empty_result_set", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		columns := []string{
			"id", "job_id", "path", "domain", "status", "status_code", "response_time",
			"cache_status", "second_response_time", "second_cache_status", "content_type",
			"error", "source_type", "source_url", "created_at", "started_at", "completed_at", "retry_count",
		}

		rows := sqlmock.NewRows(columns) // No rows added

		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		actualRows, err := db.Query("SELECT * FROM test")
		require.NoError(t, err)

		result, err := formatTasksFromRows(actualRows)
		require.NoError(t, err)
		assert.Len(t, result, 0)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
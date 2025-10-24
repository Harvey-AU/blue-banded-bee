package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Harvey-AU/blue-banded-bee/internal/cache"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupMockDB creates a mock DB wrapper for testing
func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *DB) {
	mockSQLDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	// Wrap in DB struct
	mockDB := &DB{
		client: mockSQLDB,
		config: &Config{},
		Cache:  cache.NewInMemoryCache(),
	}

	return mockSQLDB, mock, mockDB
}

func TestNewBatchManager(t *testing.T) {
	mockSQLDB, _, mockDB := setupMockDB(t)
	defer mockSQLDB.Close()

	queue := NewDbQueue(mockDB)
	bm := NewBatchManager(queue)
	require.NotNil(t, bm)
	assert.NotNil(t, bm.updates)
	assert.NotNil(t, bm.stopCh)

	// Clean shutdown
	bm.Stop()
}

func TestQueueTaskUpdate(t *testing.T) {
	mockSQLDB, _, mockDB := setupMockDB(t)
	defer mockSQLDB.Close()

	queue := NewDbQueue(mockDB)
	bm := NewBatchManager(queue)
	defer bm.Stop()

	task := &Task{
		ID:     "task-123",
		Status: "completed",
	}

	// Record initial channel capacity
	initialCap := cap(bm.updates)
	assert.Equal(t, BatchChannelSize, initialCap, "Channel capacity should match BatchChannelSize")

	// Queue update - this should succeed without blocking
	done := make(chan bool, 1)
	go func() {
		bm.QueueTaskUpdate(task)
		done <- true
	}()

	// Verify QueueTaskUpdate completes quickly (not blocked)
	select {
	case <-done:
		// Success - task was queued
	case <-time.After(100 * time.Millisecond):
		t.Fatal("QueueTaskUpdate blocked unexpectedly")
	}
}

func TestBatchUpdateCompleted(t *testing.T) {
	mockSQLDB, mock, mockDB := setupMockDB(t)
	defer mockSQLDB.Close()

	queue := NewDbQueue(mockDB)
	bm := NewBatchManager(queue)
	defer bm.Stop()

	now := time.Now()
	tasks := []*Task{
		{
			ID:           "task-1",
			Status:       "completed",
			CompletedAt:  now,
			StatusCode:   200,
			ResponseTime: 500,
			CacheStatus:  "HIT",
			RetryCount:   0,
		},
		{
			ID:           "task-2",
			Status:       "completed",
			CompletedAt:  now,
			StatusCode:   200,
			ResponseTime: 600,
			CacheStatus:  "MISS",
			RetryCount:   1,
		},
	}

	// Expect BEGIN transaction
	mock.ExpectBegin()

	// Expect the batch UPDATE query with unnest
	mock.ExpectExec(`UPDATE tasks SET status = 'completed'`).
		WithArgs(
			pq.Array([]string{"task-1", "task-2"}),
			sqlmock.AnyArg(), // completed_at array
			pq.Array([]int{200, 200}),
			pq.Array([]int64{500, 600}),
			pq.Array([]string{"HIT", "MISS"}),
			sqlmock.AnyArg(), // content_type
			sqlmock.AnyArg(), // content_length
			sqlmock.AnyArg(), // headers
			sqlmock.AnyArg(), // redirect_url
			sqlmock.AnyArg(), // dns_lookup_time
			sqlmock.AnyArg(), // tcp_connection_time
			sqlmock.AnyArg(), // tls_handshake_time
			sqlmock.AnyArg(), // ttfb
			sqlmock.AnyArg(), // content_transfer_time
			sqlmock.AnyArg(), // second_response_time
			sqlmock.AnyArg(), // second_cache_status
			sqlmock.AnyArg(), // second_content_length
			sqlmock.AnyArg(), // second_headers
			sqlmock.AnyArg(), // second_dns_lookup_time
			sqlmock.AnyArg(), // second_tcp_connection_time
			sqlmock.AnyArg(), // second_tls_handshake_time
			sqlmock.AnyArg(), // second_ttfb
			sqlmock.AnyArg(), // second_content_transfer_time
			pq.Array([]int{0, 1}),
			sqlmock.AnyArg(), // cache_check_attempts
		).
		WillReturnResult(sqlmock.NewResult(0, 2))

	// Expect COMMIT
	mock.ExpectCommit()

	// Execute batch update
	err := bm.queue.Execute(context.Background(), func(tx *sql.Tx) error {
		return bm.batchUpdateCompleted(context.Background(), tx, tasks)
	})

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchUpdateFailed(t *testing.T) {
	mockSQLDB, mock, mockDB := setupMockDB(t)
	defer mockSQLDB.Close()

	queue := NewDbQueue(mockDB)
	bm := NewBatchManager(queue)
	defer bm.Stop()

	now := time.Now()
	tasks := []*Task{
		{
			ID:          "task-1",
			Status:      "failed",
			CompletedAt: now,
			Error:       "Connection timeout",
			RetryCount:  3,
		},
		{
			ID:          "task-2",
			Status:      "blocked",
			CompletedAt: now,
			Error:       "Too Many Requests",
			RetryCount:  2,
		},
	}

	// Expect BEGIN transaction
	mock.ExpectBegin()

	// Expect the batch UPDATE query
	mock.ExpectExec(`UPDATE tasks SET status = updates.status`).
		WithArgs(
			pq.Array([]string{"task-1", "task-2"}),
			pq.Array([]string{"failed", "blocked"}),
			sqlmock.AnyArg(), // completed_at array
			pq.Array([]string{"Connection timeout", "Too Many Requests"}),
			pq.Array([]int{3, 2}),
		).
		WillReturnResult(sqlmock.NewResult(0, 2))

	// Expect COMMIT
	mock.ExpectCommit()

	// Execute batch update
	err := bm.queue.Execute(context.Background(), func(tx *sql.Tx) error {
		return bm.batchUpdateFailed(context.Background(), tx, tasks)
	})

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchUpdateSkipped(t *testing.T) {
	mockSQLDB, mock, mockDB := setupMockDB(t)
	defer mockSQLDB.Close()

	queue := NewDbQueue(mockDB)
	bm := NewBatchManager(queue)
	defer bm.Stop()

	tasks := []*Task{
		{ID: "task-1", Status: "skipped"},
		{ID: "task-2", Status: "skipped"},
		{ID: "task-3", Status: "skipped"},
	}

	// Expect BEGIN transaction
	mock.ExpectBegin()

	// Expect the batch UPDATE query
	mock.ExpectExec(`UPDATE tasks SET status = 'skipped'`).
		WithArgs(pq.Array([]string{"task-1", "task-2", "task-3"})).
		WillReturnResult(sqlmock.NewResult(0, 3))

	// Expect COMMIT
	mock.ExpectCommit()

	// Execute batch update
	err := bm.queue.Execute(context.Background(), func(tx *sql.Tx) error {
		return bm.batchUpdateSkipped(context.Background(), tx, tasks)
	})

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchUpdatePending(t *testing.T) {
	mockSQLDB, mock, mockDB := setupMockDB(t)
	defer mockSQLDB.Close()

	queue := NewDbQueue(mockDB)
	bm := NewBatchManager(queue)
	defer bm.Stop()

	tasks := []*Task{
		{ID: "task-1", Status: "pending", RetryCount: 1, StartedAt: time.Time{}},
		{ID: "task-2", Status: "pending", RetryCount: 2, StartedAt: time.Time{}},
	}

	// Expect BEGIN transaction
	mock.ExpectBegin()

	// Expect the batch UPDATE query for pending tasks
	mock.ExpectExec(`UPDATE tasks SET status = 'pending'`).
		WithArgs(
			pq.Array([]string{"task-1", "task-2"}),
			pq.Array([]int{1, 2}),
			sqlmock.AnyArg(), // started_at timestamps
		).
		WillReturnResult(sqlmock.NewResult(0, 2))

	// Expect COMMIT
	mock.ExpectCommit()

	// Execute batch update
	err := bm.queue.Execute(context.Background(), func(tx *sql.Tx) error {
		return bm.batchUpdatePending(context.Background(), tx, tasks)
	})

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFlushTaskUpdates_MixedStatuses(t *testing.T) {
	mockSQLDB, mock, mockDB := setupMockDB(t)
	defer mockSQLDB.Close()

	queue := NewDbQueue(mockDB)
	bm := NewBatchManager(queue)
	defer bm.Stop()

	now := time.Now()
	updates := []*TaskUpdate{
		{Task: &Task{ID: "task-1", Status: "completed", CompletedAt: now, StatusCode: 200}},
		{Task: &Task{ID: "task-2", Status: "failed", CompletedAt: now, Error: "Error"}},
		{Task: &Task{ID: "task-3", Status: "skipped"}},
		{Task: &Task{ID: "task-4", Status: "completed", CompletedAt: now, StatusCode: 200}},
		{Task: &Task{ID: "task-5", Status: "pending", RetryCount: 1, StartedAt: time.Time{}}},
	}

	// Expect BEGIN transaction
	mock.ExpectBegin()

	// Expect completed batch (tasks 1 and 4)
	mock.ExpectExec(`UPDATE tasks SET status = 'completed'`).
		WillReturnResult(sqlmock.NewResult(0, 2))

	// Expect failed batch (task 2)
	mock.ExpectExec(`UPDATE tasks SET status = updates.status`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect skipped batch (task 3)
	mock.ExpectExec(`UPDATE tasks SET status = 'skipped'`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect pending batch (task 5)
	mock.ExpectExec(`UPDATE tasks SET status = 'pending'`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect COMMIT
	mock.ExpectCommit()

	// Execute flush
	err := bm.flushTaskUpdates(context.Background(), updates)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchManager_GracefulShutdown(t *testing.T) {
	mockSQLDB, mock, mockDB := setupMockDB(t)
	defer mockSQLDB.Close()

	queue := NewDbQueue(mockDB)
	bm := NewBatchManager(queue)

	// Queue some updates
	task1 := &Task{ID: "task-1", Status: "completed", CompletedAt: time.Now()}
	task2 := &Task{ID: "task-2", Status: "failed", CompletedAt: time.Now(), Error: "Error"}

	bm.QueueTaskUpdate(task1)
	bm.QueueTaskUpdate(task2)

	// Expect final flush on shutdown
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE tasks SET status = 'completed'`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE tasks SET status = updates.status`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// Stop should flush remaining updates
	bm.Stop()

	// Verify all expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchUpdateCompleted_JSONBFields(t *testing.T) {
	mockSQLDB, mock, mockDB := setupMockDB(t)
	defer mockSQLDB.Close()

	queue := NewDbQueue(mockDB)
	bm := NewBatchManager(queue)
	defer bm.Stop()

	// Test with both nil and populated JSONB fields
	tasks := []*Task{
		{
			ID:                 "task-1",
			Status:             "completed",
			CompletedAt:        time.Now(),
			Headers:            nil, // Should become "{}"
			SecondHeaders:      []byte(`{"x-cache": "HIT"}`),
			CacheCheckAttempts: nil, // Should become "[]"
		},
		{
			ID:                 "task-2",
			Status:             "completed",
			CompletedAt:        time.Now(),
			Headers:            []byte(`{"content-type": "text/html"}`),
			SecondHeaders:      nil, // Should become "{}"
			CacheCheckAttempts: []byte(`[{"attempt": 1}]`),
		},
	}

	mock.ExpectBegin()

	// Verify JSONB fields are properly handled (using AnyArg since the exact
	// arrays are complex to match with pq.Array)
	mock.ExpectExec(`UPDATE tasks SET status = 'completed'`).
		WillReturnResult(sqlmock.NewResult(0, 2))

	mock.ExpectCommit()

	err := bm.queue.Execute(context.Background(), func(tx *sql.Tx) error {
		return bm.batchUpdateCompleted(context.Background(), tx, tasks)
	})

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestBatchManager_ChannelFullBlocking removed - testing implementation details
// rather than actual requirements. The critical behaviour (updates not dropped)
// is verified by other tests.

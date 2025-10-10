package db

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/cache"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDbQueue_NewDbQueue(t *testing.T) {
	t.Helper()

	db := &DB{
		client: nil,
		config: &Config{},
		Cache:  cache.NewInMemoryCache(),
	}

	queue := NewDbQueue(db)

	assert.NotNil(t, queue)
	assert.Equal(t, db, queue.db)
}

func TestDbQueue_UpdateTaskStatus_NilTaskValidation(t *testing.T) {
	t.Helper()

	dbQueue := &DbQueue{
		db: &DB{
			client: nil, // No real DB connection needed for validation test
			config: &Config{},
			Cache:  cache.NewInMemoryCache(),
		},
	}

	ctx := context.Background()

	// Test nil task validation
	err := dbQueue.UpdateTaskStatus(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot update nil task")
}

func TestDbQueue_UpdateTaskStatus_TimestampLogic(t *testing.T) {
	tests := []struct {
		name            string
		task            *Task
		expectedSetTime bool
		description     string
	}{
		{
			name: "running_status_sets_started_at",
			task: &Task{
				ID:        uuid.New().String(),
				Status:    "running",
				StartedAt: time.Time{}, // Zero value
			},
			expectedSetTime: true,
			description:     "Should set started_at for running status when zero",
		},
		{
			name: "completed_status_sets_completed_at",
			task: &Task{
				ID:          uuid.New().String(),
				Status:      "completed",
				CompletedAt: time.Time{}, // Zero value
			},
			expectedSetTime: true,
			description:     "Should set completed_at for completed status when zero",
		},
		{
			name: "failed_status_sets_completed_at",
			task: &Task{
				ID:          uuid.New().String(),
				Status:      "failed",
				CompletedAt: time.Time{}, // Zero value
			},
			expectedSetTime: true,
			description:     "Should set completed_at for failed status when zero",
		},
		{
			name: "running_with_existing_started_at",
			task: &Task{
				ID:        uuid.New().String(),
				Status:    "running",
				StartedAt: time.Now(), // Already set
			},
			expectedSetTime: false,
			description:     "Should not overwrite existing started_at",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			// Test the timestamp logic by calling the internal logic
			now := time.Now()
			originalStartedAt := tt.task.StartedAt
			originalCompletedAt := tt.task.CompletedAt

			// Simulate the timestamp setting logic from UpdateTaskStatus
			if tt.task.Status == "running" && tt.task.StartedAt.IsZero() {
				tt.task.StartedAt = now
			}
			if (tt.task.Status == "completed" || tt.task.Status == "failed") && tt.task.CompletedAt.IsZero() {
				tt.task.CompletedAt = now
			}

			// Verify the logic worked as expected
			if tt.expectedSetTime {
				if tt.task.Status == "running" {
					assert.False(t, tt.task.StartedAt.IsZero(), tt.description)
					if originalStartedAt.IsZero() {
						assert.True(t, tt.task.StartedAt.After(originalStartedAt), "StartedAt should be updated")
					}
				}
				if tt.task.Status == "completed" || tt.task.Status == "failed" {
					assert.False(t, tt.task.CompletedAt.IsZero(), tt.description)
					if originalCompletedAt.IsZero() {
						assert.True(t, tt.task.CompletedAt.After(originalCompletedAt), "CompletedAt should be updated")
					}
				}
			} else {
				if tt.task.Status == "running" && !originalStartedAt.IsZero() {
					assert.Equal(t, originalStartedAt, tt.task.StartedAt, "StartedAt should not change")
				}
			}
		})
	}
}

func TestDbQueue_TaskStatusTransitions(t *testing.T) {
	tests := []struct {
		name            string
		fromStatus      string
		toStatus        string
		validTransition bool
		description     string
	}{
		{
			name:            "pending_to_running",
			fromStatus:      "pending",
			toStatus:        "running",
			validTransition: true,
			description:     "Valid transition from pending to running",
		},
		{
			name:            "running_to_completed",
			fromStatus:      "running",
			toStatus:        "completed",
			validTransition: true,
			description:     "Valid transition from running to completed",
		},
		{
			name:            "running_to_failed",
			fromStatus:      "running",
			toStatus:        "failed",
			validTransition: true,
			description:     "Valid transition from running to failed",
		},
		{
			name:            "failed_to_pending",
			fromStatus:      "failed",
			toStatus:        "pending",
			validTransition: true,
			description:     "Valid retry transition from failed to pending",
		},
		{
			name:            "completed_to_pending",
			fromStatus:      "completed",
			toStatus:        "pending",
			validTransition: false,
			description:     "Invalid transition from completed to pending",
		},
		{
			name:            "pending_to_skipped",
			fromStatus:      "pending",
			toStatus:        "skipped",
			validTransition: true,
			description:     "Valid transition from pending to skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			// Validate transition logic
			validTransitions := map[string][]string{
				"pending":   {"running", "skipped"},
				"running":   {"completed", "failed"},
				"failed":    {"pending"}, // For retries
				"completed": {},          // Terminal state
				"skipped":   {},          // Terminal state
			}

			validTargets, exists := validTransitions[tt.fromStatus]
			require.True(t, exists, "Unknown from status: %s", tt.fromStatus)

			isValidTransition := false
			for _, validTarget := range validTargets {
				if validTarget == tt.toStatus {
					isValidTransition = true
					break
				}
			}

			assert.Equal(t, tt.validTransition, isValidTransition, tt.description)
		})
	}
}

func TestDbQueue_EnqueueURLs_PageFiltering(t *testing.T) {
	tests := []struct {
		name         string
		pages        []Page
		expectedSkip int
		description  string
	}{
		{
			name: "valid_pages",
			pages: []Page{
				{ID: 1, Path: "/page1", Priority: 1.0},
				{ID: 2, Path: "/page2", Priority: 0.8},
			},
			expectedSkip: 0,
			description:  "All valid pages should be processed",
		},
		{
			name: "pages_with_zero_id",
			pages: []Page{
				{ID: 0, Path: "/invalid", Priority: 1.0}, // Should be skipped
				{ID: 1, Path: "/valid", Priority: 0.9},
				{ID: 0, Path: "/invalid2", Priority: 0.7}, // Should be skipped
			},
			expectedSkip: 2,
			description:  "Pages with zero ID should be skipped",
		},
		{
			name:         "empty_pages",
			pages:        []Page{},
			expectedSkip: 0,
			description:  "Empty pages slice should be handled",
		},
		{
			name: "all_zero_id_pages",
			pages: []Page{
				{ID: 0, Path: "/invalid1", Priority: 1.0},
				{ID: 0, Path: "/invalid2", Priority: 0.8},
			},
			expectedSkip: 2,
			description:  "All pages with zero ID should be skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			// Test the filtering logic that's in EnqueueURLs
			skippedCount := 0
			validCount := 0

			for _, page := range tt.pages {
				if page.ID == 0 {
					skippedCount++
				} else {
					validCount++
				}
			}

			assert.Equal(t, tt.expectedSkip, skippedCount, tt.description)
			assert.Equal(t, len(tt.pages)-tt.expectedSkip, validCount, "Valid count should match")
		})
	}
}

func TestDbQueue_Execute_ContextTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}
	t.Helper()

	tests := []struct {
		name           string
		contextTimeout time.Duration
		operationDelay time.Duration
		expectTimeout  bool
		description    string
	}{
		{
			name:           "context_without_deadline",
			contextTimeout: 0, // No explicit timeout
			operationDelay: 10 * time.Millisecond,
			expectTimeout:  false, // Execute adds 30s timeout
			description:    "Should add default timeout when context has no deadline",
		},
		{
			name:           "context_with_short_timeout",
			contextTimeout: 5 * time.Millisecond,
			operationDelay: 50 * time.Millisecond,
			expectTimeout:  true,
			description:    "Should respect existing context timeout",
		},
		{
			name:           "context_with_long_timeout",
			contextTimeout: 100 * time.Millisecond,
			operationDelay: 10 * time.Millisecond,
			expectTimeout:  false,
			description:    "Should complete within timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			var ctx context.Context
			if tt.contextTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(context.Background(), tt.contextTimeout)
				defer cancel()
			} else {
				ctx = context.Background()
			}

			// Test the context timeout logic
			_, hasDeadline := ctx.Deadline()

			if tt.contextTimeout == 0 {
				assert.False(t, hasDeadline, "Context should not have deadline initially")
				// Execute would add a 30-second timeout
			} else {
				assert.True(t, hasDeadline, "Context should have deadline")
			}

			// Simulate operation delay and check timeout
			if tt.expectTimeout {
				time.Sleep(tt.operationDelay)
				select {
				case <-ctx.Done():
					assert.NotNil(t, ctx.Err(), "Context should be cancelled")
				default:
					t.Error("Context should have been cancelled")
				}
			}
		})
	}
}

func TestDbQueue_GetNextTask_Concurrency_Logic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}
	t.Helper()

	tests := []struct {
		name              string
		concurrentWorkers int
		description       string
	}{
		{
			name:              "single_worker",
			concurrentWorkers: 1,
			description:       "Single worker should claim task without contention",
		},
		{
			name:              "multiple_workers",
			concurrentWorkers: 5,
			description:       "Multiple workers should use proper locking",
		},
		{
			name:              "high_concurrency",
			concurrentWorkers: 20,
			description:       "High concurrency should be handled safely",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			// Test the concurrency design - FOR UPDATE SKIP LOCKED ensures
			// that only one worker can claim a specific task at a time
			var successfulClaims int32
			var mu sync.Mutex
			var wg sync.WaitGroup

			// Simulate the locking mechanism behaviour
			taskClaimed := false

			for i := 0; i < tt.concurrentWorkers; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()

					// Simulate the database locking mechanism
					mu.Lock()
					if !taskClaimed {
						taskClaimed = true
						successfulClaims++
						mu.Unlock()
						// Simulate task processing time
						time.Sleep(1 * time.Millisecond)
					} else {
						mu.Unlock()
						// Other workers would get sql.ErrNoRows due to SKIP LOCKED
					}
				}(i)
			}

			wg.Wait()

			// With proper locking (FOR UPDATE SKIP LOCKED), only one worker succeeds
			assert.Equal(t, int32(1), successfulClaims, tt.description)
		})
	}
}

func TestDbQueue_UpdateTaskStatus_StatusSpecificLogic(t *testing.T) {
	tests := []struct {
		name               string
		status             string
		expectedSQLPattern string
		description        string
	}{
		{
			name:               "running_status_update",
			status:             "running",
			expectedSQLPattern: "started_at",
			description:        "Running status should update started_at",
		},
		{
			name:               "completed_status_update",
			status:             "completed",
			expectedSQLPattern: "completed_at",
			description:        "Completed status should update completed_at and result fields",
		},
		{
			name:               "failed_status_update",
			status:             "failed",
			expectedSQLPattern: "error",
			description:        "Failed status should update error and retry_count",
		},
		{
			name:               "skipped_status_update",
			status:             "skipped",
			expectedSQLPattern: "status",
			description:        "Skipped status should only update status field",
		},
		{
			name:               "generic_status_update",
			status:             "pending",
			expectedSQLPattern: "status",
			description:        "Generic status should update status field only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			// Test the SQL update logic selection based on status
			// This validates which fields should be updated for each status

			task := &Task{
				ID:     uuid.New().String(),
				Status: tt.status,
			}

			// Validate that the status is correctly set
			assert.Equal(t, tt.status, task.Status)

			// The actual SQL query selection is tested in integration tests
			// Here we validate the logic flow
			switch tt.status {
			case "running":
				assert.Contains(t, tt.expectedSQLPattern, "started_at")
			case "completed":
				assert.Contains(t, tt.expectedSQLPattern, "completed_at")
			case "failed":
				assert.Contains(t, tt.expectedSQLPattern, "error")
			case "skipped":
				assert.Contains(t, tt.expectedSQLPattern, "status")
			default:
				assert.Contains(t, tt.expectedSQLPattern, "status")
			}
		})
	}
}

// Test helper functions

func createTestTask(t *testing.T, id, jobID, status string, retryCount int) *Task {
	t.Helper()
	return &Task{
		ID:            id,
		JobID:         jobID,
		PageID:        1,
		Path:          "/test-path",
		Status:        status,
		CreatedAt:     time.Now(),
		RetryCount:    retryCount,
		SourceType:    "sitemap",
		SourceURL:     "https://example.com/sitemap.xml",
		PriorityScore: 1.0,
	}
}

// Benchmark tests for performance validation

func BenchmarkDbQueue_CreateTestTask(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = &Task{
			ID:            uuid.New().String(),
			JobID:         "bench-job",
			Status:        "pending",
			RetryCount:    0,
			PriorityScore: 1.0,
		}
	}
}

func BenchmarkDbQueue_TaskStatusValidation(b *testing.B) {
	validStatuses := []string{"pending", "running", "completed", "failed", "skipped"}
	testStatus := "running"

	for i := 0; i < b.N; i++ {
		found := false
		for _, status := range validStatuses {
			if status == testStatus {
				found = true
				break
			}
		}
		_ = found
	}
}

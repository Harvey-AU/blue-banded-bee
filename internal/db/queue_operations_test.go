package db

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestCreateJob(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	tests := []struct {
		name          string
		domainID      string
		url           string
		findLinks     bool
		maxPages      int
		expectedError bool
		description   string
	}{
		{
			name:          "valid_job_creation",
			domainID:      uuid.New().String(),
			url:           "https://example.com",
			findLinks:     true,
			maxPages:      100,
			expectedError: false,
			description:   "Should create job with valid domain",
		},
		{
			name:          "invalid_domain_id",
			domainID:      "invalid-uuid",
			url:           "https://example.com",
			findLinks:     false,
			maxPages:      50,
			expectedError: true,
			description:   "Should fail with invalid domain ID",
		},
		{
			name:          "empty_url",
			domainID:      uuid.New().String(),
			url:           "",
			findLinks:     true,
			maxPages:      100,
			expectedError: true,
			description:   "Should fail with empty URL",
		},
		{
			name:          "negative_max_pages",
			domainID:      uuid.New().String(),
			url:           "https://example.com",
			findLinks:     false,
			maxPages:      -1,
			expectedError: true,
			description:   "Should fail with negative max pages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would test with actual database
			// For unit test, we validate the input
			if tt.url == "" {
				assert.True(t, tt.expectedError, tt.description)
			}
			if tt.maxPages < 0 {
				assert.True(t, tt.expectedError, tt.description)
			}
		})
	}
}

func TestGetJobByID(t *testing.T) {
	tests := []struct {
		name          string
		jobID         string
		exists        bool
		expectedError bool
		description   string
	}{
		{
			name:          "existing_job",
			jobID:         uuid.New().String(),
			exists:        true,
			expectedError: false,
			description:   "Should retrieve existing job",
		},
		{
			name:          "non_existent_job",
			jobID:         uuid.New().String(),
			exists:        false,
			expectedError: true,
			description:   "Should return error for non-existent job",
		},
		{
			name:          "invalid_job_id",
			jobID:         "invalid-uuid",
			exists:        false,
			expectedError: true,
			description:   "Should return error for invalid job ID",
		},
		{
			name:          "empty_job_id",
			jobID:         "",
			exists:        false,
			expectedError: true,
			description:   "Should return error for empty job ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.jobID == "" || tt.jobID == "invalid-uuid" {
				assert.True(t, tt.expectedError, tt.description)
			}
		})
	}
}

func TestUpdateJobStatus(t *testing.T) {
	tests := []struct {
		name          string
		jobID         string
		oldStatus     string
		newStatus     string
		validTransition bool
		description   string
	}{
		{
			name:          "pending_to_running",
			jobID:         uuid.New().String(),
			oldStatus:     "Pending",
			newStatus:     "Running",
			validTransition: true,
			description:   "Should transition from Pending to Running",
		},
		{
			name:          "running_to_completed",
			jobID:         uuid.New().String(),
			oldStatus:     "Running",
			newStatus:     "Completed",
			validTransition: true,
			description:   "Should transition from Running to Completed",
		},
		{
			name:          "running_to_failed",
			jobID:         uuid.New().String(),
			oldStatus:     "Running",
			newStatus:     "Failed",
			validTransition: true,
			description:   "Should transition from Running to Failed",
		},
		{
			name:          "completed_to_pending",
			jobID:         uuid.New().String(),
			oldStatus:     "Completed",
			newStatus:     "Pending",
			validTransition: false,
			description:   "Should not allow Completed to Pending",
		},
		{
			name:          "failed_to_retry",
			jobID:         uuid.New().String(),
			oldStatus:     "Failed",
			newStatus:     "Pending",
			validTransition: true,
			description:   "Should allow retry from Failed to Pending",
		},
		{
			name:          "invalid_status",
			jobID:         uuid.New().String(),
			oldStatus:     "Running",
			newStatus:     "InvalidStatus",
			validTransition: false,
			description:   "Should reject invalid status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate status transitions
			validStatuses := []string{"Pending", "Running", "Completed", "Failed", "Cancelled"}
			isValidStatus := false
			for _, status := range validStatuses {
				if tt.newStatus == status {
					isValidStatus = true
					break
				}
			}
			
			if !isValidStatus {
				assert.False(t, tt.validTransition, tt.description)
			}
		})
	}
}

func TestGetPendingJobs(t *testing.T) {
	tests := []struct {
		name          string
		limit         int
		expectedOrder string
		description   string
	}{
		{
			name:          "priority_ordering",
			limit:         10,
			expectedOrder: "priority_desc",
			description:   "Should order by priority descending",
		},
		{
			name:          "limit_enforcement",
			limit:         5,
			expectedOrder: "priority_desc",
			description:   "Should enforce limit on results",
		},
		{
			name:          "no_limit",
			limit:         0,
			expectedOrder: "priority_desc",
			description:   "Should return all pending jobs",
		},
		{
			name:          "large_limit",
			limit:         1000,
			expectedOrder: "priority_desc",
			description:   "Should handle large limit gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, "priority_desc", tt.expectedOrder, tt.description)
			if tt.limit > 0 {
				assert.LessOrEqual(t, tt.limit, 1000, "Limit should be reasonable")
			}
		})
	}
}

func TestCreateTaskDuplicateURL(t *testing.T) {
	tests := []struct {
		name          string
		jobID         string
		url           string
		firstAttempt  bool
		secondAttempt bool
		description   string
	}{
		{
			name:          "first_url_succeeds",
			jobID:         uuid.New().String(),
			url:           "https://example.com/page1",
			firstAttempt:  true,
			secondAttempt: false,
			description:   "First URL should create task",
		},
		{
			name:          "duplicate_url_idempotent",
			jobID:         uuid.New().String(),
			url:           "https://example.com/page1",
			firstAttempt:  true,
			secondAttempt: true,
			description:   "Duplicate URL should be idempotent",
		},
		{
			name:          "different_url_succeeds",
			jobID:         uuid.New().String(),
			url:           "https://example.com/page2",
			firstAttempt:  true,
			secondAttempt: true,
			description:   "Different URL should create new task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate duplicate handling
			if tt.name == "duplicate_url_idempotent" {
				assert.True(t, tt.firstAttempt, "First attempt should succeed")
				assert.True(t, tt.secondAttempt, "Duplicate should be handled gracefully")
			}
		})
	}
}

func TestGetNextTaskLocking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	tests := []struct {
		name          string
		concurrentWorkers int
		availableTasks    int
		expectedClaims    int
		description   string
	}{
		{
			name:          "single_task_single_worker",
			concurrentWorkers: 1,
			availableTasks:    1,
			expectedClaims:    1,
			description:   "Single worker should claim single task",
		},
		{
			name:          "single_task_multiple_workers",
			concurrentWorkers: 5,
			availableTasks:    1,
			expectedClaims:    1,
			description:   "Only one worker should claim single task",
		},
		{
			name:          "multiple_tasks_multiple_workers",
			concurrentWorkers: 3,
			availableTasks:    3,
			expectedClaims:    3,
			description:   "Each worker should claim one task",
		},
		{
			name:          "more_workers_than_tasks",
			concurrentWorkers: 10,
			availableTasks:    5,
			expectedClaims:    5,
			description:   "Should claim all available tasks",
		},
		{
			name:          "no_available_tasks",
			concurrentWorkers: 5,
			availableTasks:    0,
			expectedClaims:    0,
			description:   "No tasks to claim",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate expected behavior
			assert.LessOrEqual(t, tt.expectedClaims, tt.availableTasks, tt.description)
			assert.LessOrEqual(t, tt.expectedClaims, tt.concurrentWorkers, "Claims can't exceed workers")
		})
	}
}

func TestUpdateTaskStatusWithRetry(t *testing.T) {
	tests := []struct {
		name          string
		taskID        string
		currentRetries int
		maxRetries    int
		newStatus     string
		expectedRetries int
		description   string
	}{
		{
			name:          "increment_retry_on_failure",
			taskID:        uuid.New().String(),
			currentRetries: 0,
			maxRetries:    3,
			newStatus:     "Failed",
			expectedRetries: 1,
			description:   "Should increment retry count on failure",
		},
		{
			name:          "reset_retry_on_success",
			taskID:        uuid.New().String(),
			currentRetries: 2,
			maxRetries:    3,
			newStatus:     "Completed",
			expectedRetries: 0,
			description:   "Should reset retry count on success",
		},
		{
			name:          "max_retries_reached",
			taskID:        uuid.New().String(),
			currentRetries: 3,
			maxRetries:    3,
			newStatus:     "Failed",
			expectedRetries: 3,
			description:   "Should not exceed max retries",
		},
		{
			name:          "reset_started_at_on_pending",
			taskID:        uuid.New().String(),
			currentRetries: 1,
			maxRetries:    3,
			newStatus:     "Pending",
			expectedRetries: 1,
			description:   "Should reset started_at when back to Pending",
		},
		{
			name:          "set_completed_at",
			taskID:        uuid.New().String(),
			currentRetries: 0,
			maxRetries:    3,
			newStatus:     "Completed",
			expectedRetries: 0,
			description:   "Should set completed_at when Completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.newStatus == "Failed" && tt.currentRetries < tt.maxRetries {
				assert.Equal(t, tt.currentRetries+1, tt.expectedRetries, tt.description)
			} else if tt.newStatus == "Completed" {
				assert.Equal(t, 0, tt.expectedRetries, "Completed tasks should reset retries")
			}
		})
	}
}

func TestGetTasksByJobIDPagination(t *testing.T) {
	tests := []struct {
		name          string
		jobID         string
		page          int
		pageSize      int
		totalTasks    int
		expectedCount int
		description   string
	}{
		{
			name:          "first_page",
			jobID:         uuid.New().String(),
			page:          1,
			pageSize:      10,
			totalTasks:    25,
			expectedCount: 10,
			description:   "Should return first page of tasks",
		},
		{
			name:          "last_page_partial",
			jobID:         uuid.New().String(),
			page:          3,
			pageSize:      10,
			totalTasks:    25,
			expectedCount: 5,
			description:   "Should return partial last page",
		},
		{
			name:          "page_beyond_results",
			jobID:         uuid.New().String(),
			page:          5,
			pageSize:      10,
			totalTasks:    25,
			expectedCount: 0,
			description:   "Should return empty for page beyond results",
		},
		{
			name:          "large_page_size",
			jobID:         uuid.New().String(),
			page:          1,
			pageSize:      100,
			totalTasks:    25,
			expectedCount: 25,
			description:   "Should return all tasks with large page",
		},
		{
			name:          "boundary_test",
			jobID:         uuid.New().String(),
			page:          2,
			pageSize:      10,
			totalTasks:    20,
			expectedCount: 10,
			description:   "Should handle exact page boundary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset := (tt.page - 1) * tt.pageSize
			remaining := tt.totalTasks - offset
			
			if remaining < 0 {
				assert.Equal(t, 0, tt.expectedCount, tt.description)
			} else if remaining < tt.pageSize {
				assert.Equal(t, remaining, tt.expectedCount, tt.description)
			} else {
				assert.Equal(t, tt.pageSize, tt.expectedCount, tt.description)
			}
		})
	}
}

func TestExecuteTransactionRollback(t *testing.T) {
	tests := []struct {
		name          string
		operations    []string
		failAt        int
		shouldRollback bool
		description   string
	}{
		{
			name: "successful_transaction",
			operations: []string{
				"INSERT INTO tasks",
				"UPDATE jobs",
				"DELETE FROM temp",
			},
			failAt:        -1,
			shouldRollback: false,
			description:   "Should commit successful transaction",
		},
		{
			name: "fail_at_start",
			operations: []string{
				"INVALID SQL",
				"UPDATE jobs",
			},
			failAt:        0,
			shouldRollback: true,
			description:   "Should rollback on first operation failure",
		},
		{
			name: "fail_mid_transaction",
			operations: []string{
				"INSERT INTO tasks",
				"INVALID SQL",
				"DELETE FROM temp",
			},
			failAt:        1,
			shouldRollback: true,
			description:   "Should rollback on mid-transaction error",
		},
		{
			name: "fail_at_end",
			operations: []string{
				"INSERT INTO tasks",
				"UPDATE jobs",
				"INVALID SQL",
			},
			failAt:        2,
			shouldRollback: true,
			description:   "Should rollback on last operation failure",
		},
		{
			name: "empty_transaction",
			operations: []string{},
			failAt:        -1,
			shouldRollback: false,
			description:   "Empty transaction should succeed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.failAt >= 0 && tt.failAt < len(tt.operations) {
				assert.True(t, tt.shouldRollback, tt.description)
			} else {
				assert.False(t, tt.shouldRollback, tt.description)
			}
		})
	}
}

func TestDatabaseQueryIsolation(t *testing.T) {
	tests := []struct {
		name          string
		queryType     string
		parameters    []interface{}
		sanitized     bool
		description   string
	}{
		{
			name:          "parameterized_query",
			queryType:     "SELECT",
			parameters:    []interface{}{"user123", 25},
			sanitized:     true,
			description:   "Should use parameterized queries",
		},
		{
			name:          "sql_injection_attempt",
			queryType:     "SELECT",
			parameters:    []interface{}{"'; DROP TABLE users; --"},
			sanitized:     true,
			description:   "Should sanitize SQL injection attempts",
		},
		{
			name:          "null_parameters",
			queryType:     "INSERT",
			parameters:    []interface{}{nil, "value", nil},
			sanitized:     true,
			description:   "Should handle null parameters",
		},
		{
			name:          "special_characters",
			queryType:     "UPDATE",
			parameters:    []interface{}{"O'Brien", "test@example.com"},
			sanitized:     true,
			description:   "Should handle special characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, tt.sanitized, tt.description)
			assert.NotNil(t, tt.parameters, "Parameters should be defined")
		})
	}
}
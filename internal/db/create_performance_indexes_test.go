package db

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePerformanceIndexes(t *testing.T) {
	tests := []struct {
		name        string
		mockError   error
		expectError bool
		failAtIndex string
	}{
		{
			name:        "successful_index_creation",
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "task_job_id_index_creation_fails",
			mockError:   sql.ErrConnDone,
			expectError: true,
			failAtIndex: "job_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			if tt.expectError {
				// Expect the first index creation to fail
				mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_tasks_job_id").
					WillReturnError(tt.mockError)
			} else {
				// Expect all index operations to succeed
				mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_tasks_job_id").
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("DROP INDEX IF EXISTS idx_tasks_status").
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("DROP INDEX IF EXISTS idx_tasks_status_created").
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("DROP INDEX IF EXISTS idx_tasks_priority").
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_tasks_pending_claim_order").
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_tasks_job_status_priority").
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_job_page_unique").
					WillReturnResult(sqlmock.NewResult(0, 0))
			}

			err = createPerformanceIndexes(db)

			if tt.expectError {
				assert.Error(t, err)
				if tt.failAtIndex != "" {
					assert.Contains(t, err.Error(), tt.failAtIndex)
				}
			} else {
				assert.NoError(t, err)
			}

			// Verify all expected calls were made
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCreatePerformanceIndexesOrder(t *testing.T) {
	// Test that indexes are created in correct order
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Expect operations in specific order
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_tasks_job_id").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Cleanup old indexes first
	mock.ExpectExec("DROP INDEX IF EXISTS idx_tasks_status").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DROP INDEX IF EXISTS idx_tasks_status_created").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DROP INDEX IF EXISTS idx_tasks_priority").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Create new optimized indexes
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_tasks_pending_claim_order").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_tasks_job_status_priority").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_job_page_unique").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = createPerformanceIndexes(db)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreatePerformanceIndexesTypes(t *testing.T) {
	// Test that different index types are created correctly
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Regular index
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_tasks_job_id").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Cleanup operations (graceful failure expected)
	mock.ExpectExec("DROP INDEX IF EXISTS idx_tasks_status").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DROP INDEX IF EXISTS idx_tasks_status_created").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DROP INDEX IF EXISTS idx_tasks_priority").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Partial index (WHERE clause)
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_tasks_pending_claim_order").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Composite index with ordering
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_tasks_job_status_priority").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Unique constraint index
	mock.ExpectExec("CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_job_page_unique").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = createPerformanceIndexes(db)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

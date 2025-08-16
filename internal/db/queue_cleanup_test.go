package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupStuckJobs(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(mock sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name: "successful cleanup with affected rows",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE jobs SET status = \$1, completed_at = COALESCE\(completed_at, \$2\), progress = 100.0`).
					WithArgs("completed", sqlmock.AnyArg(), "pending", "running").
					WillReturnResult(sqlmock.NewResult(0, 3))
			},
			expectError: false,
		},
		{
			name: "successful cleanup with no affected rows",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE jobs SET status = \$1, completed_at = COALESCE\(completed_at, \$2\), progress = 100.0`).
					WithArgs("completed", sqlmock.AnyArg(), "pending", "running").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: false,
		},
		{
			name: "database error during cleanup",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE jobs SET status = \$1, completed_at = COALESCE\(completed_at, \$2\), progress = 100.0`).
					WithArgs("completed", sqlmock.AnyArg(), "pending", "running").
					WillReturnError(sql.ErrConnDone)
			},
			expectError:   true,
			errorContains: "failed to cleanup stuck jobs",
		},
		{
			name: "database timeout during cleanup",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE jobs SET status = \$1, completed_at = COALESCE\(completed_at, \$2\), progress = 100.0`).
					WithArgs("completed", sqlmock.AnyArg(), "pending", "running").
					WillReturnError(context.DeadlineExceeded)
			},
			expectError:   true,
			errorContains: "failed to cleanup stuck jobs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			// Setup expectations
			tt.setupMock(mock)

			// Create DbQueue with mock
			dbInstance := &DB{client: db}
			queue := &DbQueue{db: dbInstance}

			// Execute
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err = queue.CleanupStuckJobs(ctx)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCleanupStuckJobs_EdgeCases(t *testing.T) {
	t.Run("handles result with error getting rows affected", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		// Mock result that returns error on RowsAffected (edge case)
		mock.ExpectExec(`UPDATE jobs SET status = \$1, completed_at = COALESCE\(completed_at, \$2\), progress = 100.0`).
			WithArgs("completed", sqlmock.AnyArg(), "pending", "running").
			WillReturnResult(sqlmock.NewErrorResult(sql.ErrNoRows))

		dbInstance := &DB{client: db}
		queue := &DbQueue{db: dbInstance}

		err = queue.CleanupStuckJobs(context.Background())
		assert.NoError(t, err) // Should still succeed even if RowsAffected fails

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("concurrent cleanup calls", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		// Expect two concurrent cleanup calls
		for i := 0; i < 2; i++ {
			mock.ExpectExec(`UPDATE jobs SET status = \$1, completed_at = COALESCE\(completed_at, \$2\), progress = 100.0`).
				WithArgs("completed", sqlmock.AnyArg(), "pending", "running").
				WillReturnResult(sqlmock.NewResult(0, 1))
		}

		dbInstance := &DB{client: db}
		queue := &DbQueue{db: dbInstance}

		// Run concurrent cleanups
		done := make(chan error, 2)
		for i := 0; i < 2; i++ {
			go func() {
				done <- queue.CleanupStuckJobs(context.Background())
			}()
		}

		// Wait for both to complete
		for i := 0; i < 2; i++ {
			err := <-done
			assert.NoError(t, err)
		}

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
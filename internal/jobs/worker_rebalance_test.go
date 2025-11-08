package jobs

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
)

func TestRebalancePendingQueuesDemotesOverflowJobs(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	wp := NewWorkerPool(sqlDB, &simpleDbQueueMock{}, &simpleCrawlerMock{}, 1, 1, &db.Config{})

	overflowRows := sqlmock.NewRows([]string{"id", "cap", "pending_tasks"}).
		AddRow("job-1", 3, 10).
		AddRow("job-2", 5, 12)

	mock.ExpectQuery(regexp.QuoteMeta(pendingOverflowJobsQuery)).
		WithArgs(pendingRebalanceJobLimit).
		WillReturnRows(overflowRows)

	mock.ExpectExec(regexp.QuoteMeta(rebalanceJobPendingQuery)).
		WithArgs("job-1", 3).
		WillReturnResult(sqlmock.NewResult(0, 7))

	mock.ExpectExec(regexp.QuoteMeta(rebalanceJobPendingQuery)).
		WithArgs("job-2", 5).
		WillReturnResult(sqlmock.NewResult(0, 12))

	err = wp.rebalancePendingQueues(context.Background())
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRebalancePendingQueuesNoOverflow(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	wp := NewWorkerPool(sqlDB, &simpleDbQueueMock{}, &simpleCrawlerMock{}, 1, 1, &db.Config{})

	mock.ExpectQuery(regexp.QuoteMeta(pendingOverflowJobsQuery)).
		WithArgs(pendingRebalanceJobLimit).
		WillReturnRows(sqlmock.NewRows([]string{"id", "cap", "pending_tasks"}))

	err = wp.rebalancePendingQueues(context.Background())
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

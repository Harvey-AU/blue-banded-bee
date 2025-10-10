//go:build unit || !integration

package jobs

import (
	"context"
	"database/sql"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
)

// MockDbQueueWithTransaction implements DbQueueProvider for testing with sqlmock
// This helper is shared across multiple test files to avoid duplication
type MockDbQueueWithTransaction struct {
	db   *sql.DB
	mock sqlmock.Sqlmock
}

func (m *MockDbQueueWithTransaction) Execute(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	err = fn(tx)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (m *MockDbQueueWithTransaction) EnqueueURLs(ctx context.Context, jobID string, pages []db.Page, sourceType string, sourceURL string) error {
	// Not needed for most tests - override in specific tests if needed
	return nil
}

func (m *MockDbQueueWithTransaction) GetNextTask(ctx context.Context, jobIDs []string) (*db.Task, error) {
	// Not needed for most tests - override in specific tests if needed
	return nil, nil
}

func (m *MockDbQueueWithTransaction) UpdateTaskStatus(ctx context.Context, taskID string, statusCode int, responseTime int, cacheStatus string, errorMsg string, contentType string) error {
	// Not needed for most tests - override in specific tests if needed
	return nil
}

func (m *MockDbQueueWithTransaction) CleanupStuckJobs(ctx context.Context) error {
	// Not needed for most tests - override in specific tests if needed
	return nil
}

// Helper functions for pointer creation
func intPtr(i int) *int {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}

// simpleDbQueueMock is a minimal mock implementation of DbQueueInterface for unit tests
type simpleDbQueueMock struct{}

func (m *simpleDbQueueMock) GetNextTask(ctx context.Context, jobID string) (*db.Task, error) {
	return nil, nil
}

func (m *simpleDbQueueMock) UpdateTaskStatus(ctx context.Context, task *db.Task) error {
	return nil
}

func (m *simpleDbQueueMock) Execute(ctx context.Context, fn func(*sql.Tx) error) error {
	return nil
}

// simpleCrawlerMock is a minimal mock implementation of CrawlerInterface for unit tests
type simpleCrawlerMock struct{}

func (m *simpleCrawlerMock) WarmURL(ctx context.Context, url string, findLinks bool) (*crawler.CrawlResult, error) {
	return &crawler.CrawlResult{
		StatusCode:  200,
		CacheStatus: "HIT",
	}, nil
}

func (m *simpleCrawlerMock) DiscoverSitemapsAndRobots(ctx context.Context, domain string) (*crawler.SitemapDiscoveryResult, error) {
	return &crawler.SitemapDiscoveryResult{}, nil
}

func (m *simpleCrawlerMock) ParseSitemap(ctx context.Context, sitemapURL string) ([]string, error) {
	return []string{}, nil
}

func (m *simpleCrawlerMock) FilterURLs(urls []string, includePaths, excludePaths []string) []string {
	return urls
}

func (m *simpleCrawlerMock) GetUserAgent() string {
	return "test-agent"
}

// MockWorkerPool is a minimal mock implementation of WorkerPool for testing
type MockWorkerPool struct {
	jobs            map[string]bool
	jobPerformance  map[string]*JobPerformance
	stopCh          chan struct{}
	notifyCh        chan struct{}
	AddJobCalled    bool
	RemoveJobCalled bool
}

// NewMockWorkerPool creates a new mock worker pool
func NewMockWorkerPool() *MockWorkerPool {
	return &MockWorkerPool{
		jobs:           make(map[string]bool),
		jobPerformance: make(map[string]*JobPerformance),
		stopCh:         make(chan struct{}),
		notifyCh:       make(chan struct{}, 1),
	}
}

// AddJob simulates adding a job without database access
func (m *MockWorkerPool) AddJob(jobID string, options *JobOptions) {
	m.AddJobCalled = true
	m.jobs[jobID] = true
	m.jobPerformance[jobID] = &JobPerformance{
		RecentTasks:  make([]int64, 0, 5),
		CurrentBoost: 0,
		LastCheck:    time.Now(),
	}
}

// RemoveJob simulates removing a job
func (m *MockWorkerPool) RemoveJob(jobID string) {
	m.RemoveJobCalled = true
	delete(m.jobs, jobID)
	delete(m.jobPerformance, jobID)
}

//go:build integration

package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T) *db.DB {
	t.Helper()
	testutil.LoadTestEnv(t)

	database, err := db.InitFromEnv()
	require.NoError(t, err, "Failed to connect to test database")
	
	// Register cleanup to close database - this will run LAST (LIFO order)
	t.Cleanup(func() {
		database.Close()
	})
	
	return database
}

func TestGetJob(t *testing.T) {
	// 1. Connect to test database
	database := setupTest(t)

	// 2. Create test data using a simpler approach
	ctx := context.Background()
	sqlDB := database.GetDB()
	dbQueue := db.NewDbQueue(database)
	jm := NewJobManager(sqlDB, dbQueue, nil, nil)

	// Create a job using CreateJob which handles all required fields
	options := &JobOptions{
		Domain:      "test.example.com",
		Concurrency: 5,
		FindLinks:   true,
		MaxPages:    100,
		UseSitemap:  true,
	}

	createdJob, err := jm.CreateJob(ctx, options)
	require.NoError(t, err, "Failed to create test job")
	require.NotNil(t, createdJob)
	
	// Register cleanup immediately after creation - runs before database close
	t.Cleanup(func() {
		// Delete tasks first (if any) due to foreign key constraints
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM tasks WHERE job_id = $1", createdJob.ID)
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM jobs WHERE id = $1", createdJob.ID)
	})

	// 3. Execute GetJob function
	job, err := jm.GetJob(ctx, createdJob.ID)

	// 4. Assert results
	require.NoError(t, err, "GetJob should not return error")
	assert.Equal(t, createdJob.ID, job.ID)
	assert.Equal(t, "test.example.com", job.Domain)
	assert.Equal(t, JobStatusPending, job.Status)
	assert.Equal(t, 5, job.Concurrency)
	assert.True(t, job.FindLinks)
}

func TestCreateJob(t *testing.T) {
	// Connect to test database
	database := setupTest(t)

	ctx := context.Background()
	sqlDB := database.GetDB()
	dbQueue := db.NewDbQueue(database)

	// For integration test, we'll use nil crawler since we're not testing sitemap functionality
	// and nil worker pool since we'll handle cancellation differently
	jm := NewJobManager(sqlDB, dbQueue, nil, nil)

	// Register cleanup for all test-* domain jobs/tasks - runs before database close
	t.Cleanup(func() {
		// Delete tasks first due to foreign key constraints
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM tasks WHERE job_id IN (SELECT id FROM jobs WHERE domain_id IN (SELECT id FROM domains WHERE name LIKE 'test-%'))")
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM jobs WHERE domain_id IN (SELECT id FROM domains WHERE name LIKE 'test-%'))")
	})

	// Test case 1: Create a new job successfully
	options := &JobOptions{
		Domain:      "test-create.example.com",
		Concurrency: 3,
		FindLinks:   false,
		MaxPages:    50,
		UseSitemap:  false, // Don't trigger sitemap processing
	}

	// Create the job
	job, err := jm.CreateJob(ctx, options)
	require.NoError(t, err, "CreateJob should not return error")
	require.NotNil(t, job, "Job should not be nil")

	// Verify job properties
	assert.NotEmpty(t, job.ID, "Job ID should be set")
	assert.Equal(t, "test-create.example.com", job.Domain)
	assert.Equal(t, JobStatusPending, job.Status)
	assert.Equal(t, 3, job.Concurrency)
	assert.False(t, job.FindLinks)
	assert.Equal(t, 50, job.MaxPages)

	// Verify job exists in database
	var count int
	err = sqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs WHERE id = $1", job.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Job should exist in database")
}

func TestCancelJob(t *testing.T) {
	// Connect to test database
	database := setupTest(t)

	ctx := context.Background()
	sqlDB := database.GetDB()
	dbQueue := db.NewDbQueue(database)

	// Create JobManager with minimal worker pool for RemoveJob functionality
	wp := &WorkerPool{
		jobs:     make(map[string]bool),
		stopCh:   make(chan struct{}),
		notifyCh: make(chan struct{}, 1),
	}
	jm := NewJobManager(sqlDB, dbQueue, nil, wp)

	// Register cleanup for all test-* domain jobs/tasks - runs before database close
	t.Cleanup(func() {
		// Delete tasks first due to foreign key constraints
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM tasks WHERE job_id IN (SELECT id FROM jobs WHERE domain_id IN (SELECT id FROM domains WHERE name LIKE 'test-%'))")
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM jobs WHERE domain_id IN (SELECT id FROM domains WHERE name LIKE 'test-%'))")
	})

	// Test case 1: Cancel a pending job
	options := &JobOptions{
		Domain:      "test-cancel.example.com",
		Concurrency: 5,
		FindLinks:   true,
		MaxPages:    100,
		UseSitemap:  false, // Don't trigger sitemap processing
	}

	// Create a job
	job, err := jm.CreateJob(ctx, options)
	require.NoError(t, err, "Failed to create job")
	require.NotNil(t, job)

	// Cancel the job
	err = jm.CancelJob(ctx, job.ID)
	require.NoError(t, err, "CancelJob should not return error")

	// Verify job status is cancelled
	cancelledJob, err := jm.GetJob(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, JobStatusCancelled, cancelledJob.Status)
	assert.False(t, cancelledJob.CompletedAt.IsZero(), "CompletedAt should be set")

	// Test case 2: Try to cancel an already cancelled job
	err = jm.CancelJob(ctx, job.ID)
	assert.Error(t, err, "Should error when cancelling already cancelled job")
	assert.Contains(t, err.Error(), "job cannot be canceled")

	// Test case 3: Try to cancel a non-existent job
	err = jm.CancelJob(ctx, "non-existent-id")
	assert.Error(t, err, "Should error when cancelling non-existent job")
}

func TestProcessSitemapFallback(t *testing.T) {
	// Connect to test database
	database := setupTest(t)

	ctx := context.Background()
	sqlDB := database.GetDB()
	dbQueue := db.NewDbQueue(database)

	// Create a mock crawler that returns no sitemaps (to trigger fallback)
	mockCrawler := &MockCrawlerForIntegration{
		discoverSitemapsAndRobotsFunc: func(ctx context.Context, domain string) (*crawler.SitemapDiscoveryResult, error) {
			// Return empty to trigger fallback behaviour
			return &crawler.SitemapDiscoveryResult{
				Sitemaps:    []string{},
				RobotsRules: &crawler.RobotsRules{},
			}, nil
		},
	}

	// Create JobManager with mock crawler
	jm := NewJobManager(sqlDB, dbQueue, mockCrawler, nil)

	// Create a job with sitemap enabled to trigger processSitemap
	options := &JobOptions{
		Domain:      "test-fallback.example.com",
		Concurrency: 5,
		FindLinks:   true,
		MaxPages:    100,
		UseSitemap:  true, // This will trigger processSitemap
	}

	// Create the job
	job, err := jm.CreateJob(ctx, options)
	require.NoError(t, err, "Failed to create job")
	require.NotNil(t, job)
	
	// Register cleanup immediately after creation - runs before database close
	t.Cleanup(func() {
		// Delete tasks first due to foreign key constraints
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM tasks WHERE job_id = $1", job.ID)
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM jobs WHERE id = $1", job.ID)
	})

	// Check if any tasks exist immediately after job creation
	var initialTaskCount int
	err = sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE job_id = $1`, job.ID).Scan(&initialTaskCount)
	require.NoError(t, err)
	t.Logf("Initial task count after job creation: %d", initialTaskCount)

	// Wait for processSitemap goroutine to complete
	// Poll for the task to be created instead of fixed sleep
	var taskCount int
	var sourceType string
	maxWait := 10 * time.Second
	pollInterval := 500 * time.Millisecond
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		err = sqlDB.QueryRowContext(ctx, `
			SELECT COUNT(*) 
			FROM tasks 
			WHERE job_id = $1 AND path = '/'
		`, job.ID).Scan(&taskCount)
		require.NoError(t, err)

		if taskCount > 0 {
			// Task exists, get its source type
			err = sqlDB.QueryRowContext(ctx, `
				SELECT source_type 
				FROM tasks 
				WHERE job_id = $1 AND path = '/'
			`, job.ID).Scan(&sourceType)
			require.NoError(t, err)
			break
		}

		time.Sleep(pollInterval)
	}

	// Debug output
	t.Logf("Task count: %d", taskCount)
	if taskCount > 0 {
		t.Logf("Source type: %s", sourceType)
	}

	assert.Equal(t, 1, taskCount, "Should have exactly one root task")
	assert.Equal(t, "fallback", sourceType, "Root task should have 'fallback' source type")
}

func TestEnqueueJobURLs(t *testing.T) {
	// Connect to test database
	database := setupTest(t)

	ctx := context.Background()
	sqlDB := database.GetDB()
	dbQueue := db.NewDbQueue(database)

	// Create JobManager
	jm := NewJobManager(sqlDB, dbQueue, nil, nil)

	// Create a job
	options := &JobOptions{
		Domain:      "test-enqueue.example.com",
		Concurrency: 5,
		FindLinks:   true,
		MaxPages:    100,
		UseSitemap:  false,
	}

	job, err := jm.CreateJob(ctx, options)
	require.NoError(t, err, "Failed to create job")
	require.NotNil(t, job)
	
	// Register cleanup immediately after creation - runs before database close
	t.Cleanup(func() {
		// Delete tasks first due to foreign key constraints
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM tasks WHERE job_id = $1", job.ID)
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM jobs WHERE id = $1", job.ID)
	})

	// Get domain ID
	var domainID int
	err = sqlDB.QueryRowContext(ctx, `
		SELECT domain_id FROM jobs WHERE id = $1
	`, job.ID).Scan(&domainID)
	require.NoError(t, err)

	// Create test pages
	testPages := []db.Page{
		{Path: "/page1", Priority: 0.9},
		{Path: "/page2", Priority: 0.8},
		{Path: "/page3", Priority: 0.7},
	}

	// Create page records in database
	for i := range testPages {
		err = sqlDB.QueryRowContext(ctx, `
			INSERT INTO pages (domain_id, path)
			VALUES ($1, $2)
			ON CONFLICT (domain_id, path) DO UPDATE SET path = EXCLUDED.path
			RETURNING id
		`, domainID, testPages[i].Path).Scan(&testPages[i].ID)
		require.NoError(t, err)
	}

	// Test case 1: Enqueue new URLs
	err = jm.EnqueueJobURLs(ctx, job.ID, testPages, "test", "https://test.com")
	require.NoError(t, err, "EnqueueJobURLs should not return error")

	// Verify tasks were created
	var taskCount int
	err = sqlDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tasks WHERE job_id = $1
	`, job.ID).Scan(&taskCount)
	require.NoError(t, err)
	assert.Equal(t, 4, taskCount, "Should have 4 tasks (3 new + 1 root from job creation)")

	// Test case 2: Try to enqueue the same URLs again (should be filtered out)
	err = jm.EnqueueJobURLs(ctx, job.ID, testPages, "test", "https://test.com")
	require.NoError(t, err, "EnqueueJobURLs should not return error for duplicates")

	// Verify no additional tasks were created
	err = sqlDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tasks WHERE job_id = $1
	`, job.ID).Scan(&taskCount)
	require.NoError(t, err)
	assert.Equal(t, 4, taskCount, "Should still have 4 tasks (duplicates filtered)")
}

// MockCrawlerForIntegration is a simple mock for integration tests
type MockCrawlerForIntegration struct {
	discoverSitemapsAndRobotsFunc func(ctx context.Context, domain string) (*crawler.SitemapDiscoveryResult, error)
	parseSitemapFunc              func(ctx context.Context, sitemapURL string) ([]string, error)
	warmURLFunc                   func(ctx context.Context, url string, findLinks bool) (*crawler.CrawlResult, error)
	filterURLsFunc                func(urls []string, includePaths, excludePaths []string) []string
}

func (m *MockCrawlerForIntegration) DiscoverSitemapsAndRobots(ctx context.Context, domain string) (*crawler.SitemapDiscoveryResult, error) {
	if m.discoverSitemapsAndRobotsFunc != nil {
		return m.discoverSitemapsAndRobotsFunc(ctx, domain)
	}
	// Default behavior - return empty results
	return &crawler.SitemapDiscoveryResult{
		Sitemaps:    []string{},
		RobotsRules: &crawler.RobotsRules{},
	}, nil
}

func (m *MockCrawlerForIntegration) ParseSitemap(ctx context.Context, sitemapURL string) ([]string, error) {
	if m.parseSitemapFunc != nil {
		return m.parseSitemapFunc(ctx, sitemapURL)
	}
	return []string{}, nil
}

func (m *MockCrawlerForIntegration) WarmURL(ctx context.Context, url string, findLinks bool) (*crawler.CrawlResult, error) {
	if m.warmURLFunc != nil {
		return m.warmURLFunc(ctx, url, findLinks)
	}
	return &crawler.CrawlResult{}, nil
}

func (m *MockCrawlerForIntegration) FilterURLs(urls []string, includePaths, excludePaths []string) []string {
	if m.filterURLsFunc != nil {
		return m.filterURLsFunc(urls, includePaths, excludePaths)
	}
	return urls
}

func (m *MockCrawlerForIntegration) GetUserAgent() string {
	return "test-agent"
}
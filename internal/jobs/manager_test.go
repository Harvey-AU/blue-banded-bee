package jobs

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDbQueue implements the DbQueueProvider interface for testing
type MockDbQueue struct {
	mock.Mock
}

func (m *MockDbQueue) Execute(ctx context.Context, fn func(*sql.Tx) error) error {
	args := m.Called(ctx, fn)
	if args.Get(0) == nil {
		return nil
	}
	return args.Error(0)
}

func (m *MockDbQueue) EnqueueURLs(ctx context.Context, jobID string, pages []db.Page, sourceType string, sourceURL string) error {
	args := m.Called(ctx, jobID, pages, sourceType, sourceURL)
	return args.Error(0)
}

func (m *MockDbQueue) CleanupStuckJobs(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockWorkerPool for testing
type MockWorkerPool struct {
	mock.Mock
}

func (m *MockWorkerPool) AddJob(jobID string, job *Job) {
	m.Called(jobID, job)
}

func (m *MockWorkerPool) RemoveJob(jobID string) {
	m.Called(jobID)
}

func (m *MockWorkerPool) Stop() {
	m.Called()
}

func (m *MockWorkerPool) StopWithTimeout(timeout time.Duration) error {
	args := m.Called(timeout)
	return args.Error(0)
}

func (m *MockWorkerPool) Wait() {
	m.Called()
}

func (m *MockWorkerPool) SetJobManager(jm *JobManager) {
	m.Called(jm)
}

func (m *MockWorkerPool) StartTaskMonitor(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockWorkerPool) EnqueueURLs(ctx context.Context, jobID string, pages []db.Page, sourceType string, sourceURL string) error {
	args := m.Called(ctx, jobID, pages, sourceType, sourceURL)
	return args.Error(0)
}

// TestProcessSitemapFallback tests that when no sitemap is found, 
// a fallback root page task is created
func TestProcessSitemapFallback(t *testing.T) {
	// Setup
	mockDB := &sql.DB{}
	mockQueue := new(MockDbQueue)
	mockWorkerPool := new(MockWorkerPool)
	
	jm := &JobManager{
		db:           mockDB,
		dbQueue:      mockQueue,
		workerPool:   mockWorkerPool,
		processedPages: make(map[string]map[int]bool),
	}
	
	// Mock worker pool to accept SetJobManager
	mockWorkerPool.On("SetJobManager", jm).Return()
	jm.workerPool.SetJobManager(jm)
	
	ctx := context.Background()
	jobID := uuid.New().String()
	domain := "example.com"
	domainID := 1
	
	// Test case: No sitemap found (empty sitemap URLs)
	// This simulates the scenario where DiscoverSitemaps returns no URLs
	
	// Mock getting domain ID
	mockQueue.On("Execute", ctx, mock.AnythingOfType("func(*sql.Tx) error")).
		Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(*sql.Tx) error)
			// Simulate successful domain ID retrieval
			fn(nil)
		}).Return(nil).Once()
	
	// Mock CreatePageRecords being called with root URL
	expectedRootURL := "https://example.com/"
	
	// Mock EnqueueJobURLs being called for the fallback root page
	mockQueue.On("EnqueueURLs", ctx, jobID, mock.MatchedBy(func(pages []db.Page) bool {
		return len(pages) == 1 && 
			pages[0].Path == "/" && 
			pages[0].Priority == 1.000
	}), "fallback", expectedRootURL).Return(nil).Once()
	
	// Mock the final stats recalculation
	mockQueue.On("Execute", ctx, mock.AnythingOfType("func(*sql.Tx) error")).
		Return(nil).Once()
	
	// Initialize the processedPages map for this job
	jm.processedPages[jobID] = make(map[int]bool)
	
	// Call processSitemap with no sitemap URLs (simulating no sitemap found)
	jm.processSitemap(ctx, jobID, domain, nil, nil)
	
	// Allow async operations to complete
	time.Sleep(100 * time.Millisecond)
	
	// Verify all expectations were met
	mockQueue.AssertExpectations(t)
	
	// Verify that EnqueueJobURLs was called with fallback parameters
	mockQueue.AssertCalled(t, "EnqueueURLs", ctx, jobID, mock.MatchedBy(func(pages []db.Page) bool {
		return len(pages) == 1 && pages[0].Path == "/" && pages[0].Priority == 1.000
	}), "fallback", expectedRootURL)
}

// TestProcessSitemapWithURLs tests normal sitemap processing with URLs found
func TestProcessSitemapWithURLs(t *testing.T) {
	// Setup
	mockDB := &sql.DB{}
	mockQueue := new(MockDbQueue)
	mockWorkerPool := new(MockWorkerPool)
	
	jm := &JobManager{
		db:           mockDB,
		dbQueue:      mockQueue,
		workerPool:   mockWorkerPool,
		processedPages: make(map[string]map[int]bool),
	}
	
	mockWorkerPool.On("SetJobManager", jm).Return()
	jm.workerPool.SetJobManager(jm)
	
	ctx := context.Background()
	jobID := uuid.New().String()
	domain := "example.com"
	
	// This test would verify that when sitemap URLs are found,
	// they are processed normally without fallback
	
	// Mock getting domain ID
	mockQueue.On("Execute", ctx, mock.AnythingOfType("func(*sql.Tx) error")).
		Return(nil).Once()
	
	// Mock EnqueueJobURLs for sitemap URLs
	mockQueue.On("EnqueueURLs", ctx, jobID, mock.MatchedBy(func(pages []db.Page) bool {
		// Should have multiple pages from sitemap
		return len(pages) > 1 && pages[0].Priority == 0.5
	}), "sitemap", mock.AnythingOfType("string")).Return(nil).Once()
	
	// Mock stats recalculation
	mockQueue.On("Execute", ctx, mock.AnythingOfType("func(*sql.Tx) error")).
		Return(nil).Once()
	
	jm.processedPages[jobID] = make(map[int]bool)
	
	// In real implementation, this would process actual sitemap URLs
	// For this test, we're verifying the structure is correct
	
	// Note: This is a simplified test structure. In practice, you'd need to
	// mock the crawler's DiscoverSitemaps and ParseSitemap methods
}
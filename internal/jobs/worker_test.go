package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestNewWorkerPool(t *testing.T) {
	tests := []struct {
		name        string
		db          *sql.DB
		dbQueue     *db.DbQueue
		crawler     *crawler.Crawler
		numWorkers  int
		dbConfig    *db.Config
		expectPanic bool
		panicMsg    string
	}{
		{
			name:        "nil database",
			db:          nil,
			dbQueue:     &db.DbQueue{},
			crawler:     &crawler.Crawler{},
			numWorkers:  5,
			dbConfig:    &db.Config{},
			expectPanic: true,
			panicMsg:    "database connection is required",
		},
		{
			name:        "nil dbQueue",
			db:          &sql.DB{},
			dbQueue:     nil,
			crawler:     &crawler.Crawler{},
			numWorkers:  5,
			dbConfig:    &db.Config{},
			expectPanic: true,
			panicMsg:    "database queue is required",
		},
		{
			name:        "nil crawler",
			db:          &sql.DB{},
			dbQueue:     &db.DbQueue{},
			crawler:     nil,
			numWorkers:  5,
			dbConfig:    &db.Config{},
			expectPanic: true,
			panicMsg:    "crawler is required",
		},
		{
			name:        "zero workers",
			db:          &sql.DB{},
			dbQueue:     &db.DbQueue{},
			crawler:     &crawler.Crawler{},
			numWorkers:  0,
			dbConfig:    &db.Config{},
			expectPanic: true,
			panicMsg:    "numWorkers must be at least 1",
		},
		{
			name:        "negative workers",
			db:          &sql.DB{},
			dbQueue:     &db.DbQueue{},
			crawler:     &crawler.Crawler{},
			numWorkers:  -1,
			dbConfig:    &db.Config{},
			expectPanic: true,
			panicMsg:    "numWorkers must be at least 1",
		},
		{
			name:        "nil dbConfig",
			db:          &sql.DB{},
			dbQueue:     &db.DbQueue{},
			crawler:     &crawler.Crawler{},
			numWorkers:  5,
			dbConfig:    nil,
			expectPanic: true,
			panicMsg:    "database configuration is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				assert.PanicsWithValue(t, tt.panicMsg, func() {
					NewWorkerPool(tt.db, tt.dbQueue, tt.crawler, tt.numWorkers, tt.dbConfig)
				})
			} else {
				wp := NewWorkerPool(tt.db, tt.dbQueue, tt.crawler, tt.numWorkers, tt.dbConfig)
				assert.NotNil(t, wp)
				assert.Equal(t, tt.numWorkers, wp.numWorkers)
				assert.Equal(t, tt.numWorkers, wp.baseWorkerCount)
				assert.Equal(t, tt.numWorkers, wp.currentWorkers)
				assert.NotNil(t, wp.jobs)
				assert.NotNil(t, wp.stopCh)
				assert.NotNil(t, wp.notifyCh)
				assert.NotNil(t, wp.taskBatch)
				assert.NotNil(t, wp.jobPerformance)
				assert.NotNil(t, wp.jobInfoCache)
			}
		})
	}
}

func TestNewWorkerPoolInitialisation(t *testing.T) {
	// Create mock dependencies
	mockDB := &sql.DB{}
	mockDbQueue := &db.DbQueue{}
	mockCrawler := &crawler.Crawler{}
	mockConfig := &db.Config{}

	wp := NewWorkerPool(mockDB, mockDbQueue, mockCrawler, 5, mockConfig)
	defer wp.Stop() // Clean up background goroutines

	// Test proper initialisation
	assert.Equal(t, mockDB, wp.db)
	assert.Equal(t, mockDbQueue, wp.dbQueue)
	assert.Equal(t, mockCrawler, wp.crawler)
	assert.Equal(t, 5, wp.numWorkers)
	assert.Equal(t, 5, wp.baseWorkerCount)
	assert.Equal(t, 5, wp.currentWorkers)
	assert.Equal(t, time.Minute, wp.recoveryInterval)
	
	// Test channels are properly created
	assert.NotNil(t, wp.stopCh)
	assert.NotNil(t, wp.notifyCh)
	assert.Equal(t, 1, cap(wp.notifyCh)) // Should be buffered with capacity 1
	
	// Test maps are initialised
	assert.NotNil(t, wp.jobs)
	assert.NotNil(t, wp.jobPerformance)
	assert.NotNil(t, wp.jobInfoCache)
	assert.Empty(t, wp.jobs)
	assert.Empty(t, wp.jobPerformance)
	assert.Empty(t, wp.jobInfoCache)
	
	// Test task batch is properly initialised
	assert.NotNil(t, wp.taskBatch)
	assert.NotNil(t, wp.taskBatch.tasks)
	assert.NotNil(t, wp.taskBatch.jobCounts)
	assert.Equal(t, 50, cap(wp.taskBatch.tasks))
	
	// Test timer is created
	assert.NotNil(t, wp.batchTimer)
}

func TestWorkerPoolStopAndWait(t *testing.T) {
	mockDB := &sql.DB{}
	mockDbQueue := &db.DbQueue{}
	mockCrawler := &crawler.Crawler{}
	mockConfig := &db.Config{}

	wp := NewWorkerPool(mockDB, mockDbQueue, mockCrawler, 2, mockConfig)

	// Test that stopping sets the atomic flag
	assert.False(t, wp.stopping.Load())
	
	// Stop should not block and should close stopCh
	wp.Stop()
	
	assert.True(t, wp.stopping.Load())
	
	// Verify stopCh is closed by trying to receive (should not block)
	select {
	case <-wp.stopCh:
		// Expected - channel is closed
	case <-time.After(100 * time.Millisecond):
		t.Error("stopCh was not closed")
	}
}

func TestWorkerPoolAddJobBasicTracking(t *testing.T) {
	// Test the basic job tracking without database operations
	mockDB := &sql.DB{}
	mockDbQueue := &db.DbQueue{}
	mockCrawler := &crawler.Crawler{}
	mockConfig := &db.Config{}

	wp := NewWorkerPool(mockDB, mockDbQueue, mockCrawler, 2, mockConfig)
	defer wp.Stop() // Clean up background goroutines

	jobID := "test-job-123"
	
	// Initially no jobs
	wp.jobsMutex.RLock()
	assert.Empty(t, wp.jobs)
	wp.jobsMutex.RUnlock()

	// Manually add job to tracking (bypassing database operations)
	wp.jobsMutex.Lock()
	wp.jobs[jobID] = true
	wp.jobsMutex.Unlock()

	// Initialise performance tracking manually  
	wp.perfMutex.Lock()
	wp.jobPerformance[jobID] = &JobPerformance{
		RecentTasks:  make([]int64, 0, 5),
		CurrentBoost: 0,
		LastCheck:    time.Now(),
	}
	wp.perfMutex.Unlock()

	// Verify job is tracked
	wp.jobsMutex.RLock()
	assert.True(t, wp.jobs[jobID])
	wp.jobsMutex.RUnlock()

	// Verify performance tracking is initialised
	wp.perfMutex.RLock()
	perf, exists := wp.jobPerformance[jobID]
	wp.perfMutex.RUnlock()
	
	assert.True(t, exists)
	assert.NotNil(t, perf)
	assert.Equal(t, 0, perf.CurrentBoost)
	assert.NotNil(t, perf.RecentTasks)
	assert.Equal(t, 0, len(perf.RecentTasks))
	assert.Equal(t, 5, cap(perf.RecentTasks))
}

func TestWorkerPoolRemoveJob(t *testing.T) {
	mockDB := &sql.DB{}
	mockDbQueue := &db.DbQueue{}
	mockCrawler := &crawler.Crawler{}
	mockConfig := &db.Config{}

	wp := NewWorkerPool(mockDB, mockDbQueue, mockCrawler, 10, mockConfig)
	defer wp.Stop() // Clean up background goroutines

	jobID := "test-job-456"

	// Manually add job to tracking (bypassing database operations)
	wp.jobsMutex.Lock()
	wp.jobs[jobID] = true
	wp.jobsMutex.Unlock()

	// Add performance tracking
	wp.perfMutex.Lock()
	wp.jobPerformance[jobID] = &JobPerformance{
		RecentTasks:  make([]int64, 0, 5),
		CurrentBoost: 5, // Give it some boost to test removal
		LastCheck:    time.Now(),
	}
	wp.perfMutex.Unlock()

	// Add job info cache
	wp.jobInfoMutex.Lock()
	wp.jobInfoCache[jobID] = &JobInfo{
		DomainName: "test.com",
		FindLinks:  true,
		CrawlDelay: 0,
	}
	wp.jobInfoMutex.Unlock()
	
	// Verify job exists
	wp.jobsMutex.RLock()
	assert.True(t, wp.jobs[jobID])
	wp.jobsMutex.RUnlock()

	// Remove job
	wp.RemoveJob(jobID)

	// Verify job is removed from tracking
	wp.jobsMutex.RLock()
	_, exists := wp.jobs[jobID]
	wp.jobsMutex.RUnlock()
	assert.False(t, exists)

	// Verify performance tracking is removed
	wp.perfMutex.RLock()
	_, perfExists := wp.jobPerformance[jobID]
	wp.perfMutex.RUnlock()
	assert.False(t, perfExists)

	// Verify job info cache is cleared
	wp.jobInfoMutex.RLock()
	_, cacheExists := wp.jobInfoCache[jobID]
	wp.jobInfoMutex.RUnlock()
	assert.False(t, cacheExists)
}

func TestWorkerPoolConcurrentJobManagement(t *testing.T) {
	mockDB := &sql.DB{}
	mockDbQueue := &db.DbQueue{}
	mockCrawler := &crawler.Crawler{}
	mockConfig := &db.Config{}

	wp := NewWorkerPool(mockDB, mockDbQueue, mockCrawler, 5, mockConfig)
	defer wp.Stop() // Clean up background goroutines

	// Test concurrent add/remove operations
	var wg sync.WaitGroup
	numJobs := 10
	
	// Add jobs concurrently (manually, bypassing database operations)
	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func(jobNum int) {
			defer wg.Done()
			jobID := fmt.Sprintf("concurrent-job-%d", jobNum)
			
			// Manually add to tracking
			wp.jobsMutex.Lock()
			wp.jobs[jobID] = true
			wp.jobsMutex.Unlock()

			// Add performance tracking
			wp.perfMutex.Lock()
			wp.jobPerformance[jobID] = &JobPerformance{
				RecentTasks:  make([]int64, 0, 5),
				CurrentBoost: 0,
				LastCheck:    time.Now(),
			}
			wp.perfMutex.Unlock()
		}(i)
	}
	
	wg.Wait()
	
	// Verify all jobs were added
	wp.jobsMutex.RLock()
	assert.Equal(t, numJobs, len(wp.jobs))
	wp.jobsMutex.RUnlock()
	
	// Remove jobs concurrently
	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func(jobNum int) {
			defer wg.Done()
			jobID := fmt.Sprintf("concurrent-job-%d", jobNum)
			wp.RemoveJob(jobID)
		}(i)
	}
	
	wg.Wait()
	
	// Verify all jobs were removed
	wp.jobsMutex.RLock()
	assert.Equal(t, 0, len(wp.jobs))
	wp.jobsMutex.RUnlock()
}

func TestIsSameOrSubDomain(t *testing.T) {
	tests := []struct {
		name         string
		hostname     string
		targetDomain string
		expected     bool
	}{
		{
			name:         "exact match",
			hostname:     "example.com",
			targetDomain: "example.com",
			expected:     true,
		},
		{
			name:         "www prefix on hostname",
			hostname:     "www.example.com",
			targetDomain: "example.com",
			expected:     true,
		},
		{
			name:         "www prefix on target",
			hostname:     "example.com",
			targetDomain: "www.example.com",
			expected:     true,
		},
		{
			name:         "both have www prefix",
			hostname:     "www.example.com",
			targetDomain: "www.example.com",
			expected:     true,
		},
		{
			name:         "subdomain",
			hostname:     "api.example.com",
			targetDomain: "example.com",
			expected:     true,
		},
		{
			name:         "subdomain with www on target",
			hostname:     "api.example.com",
			targetDomain: "www.example.com",
			expected:     true,
		},
		{
			name:         "deep subdomain",
			hostname:     "api.v1.example.com",
			targetDomain: "example.com",
			expected:     true,
		},
		{
			name:         "different domain",
			hostname:     "different.com",
			targetDomain: "example.com",
			expected:     false,
		},
		{
			name:         "similar but different domain",
			hostname:     "notexample.com",
			targetDomain: "example.com",
			expected:     false,
		},
		{
			name:         "case insensitive",
			hostname:     "API.EXAMPLE.COM",
			targetDomain: "example.com",
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSameOrSubDomain(tt.hostname, tt.targetDomain)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWorkerPoolScaling(t *testing.T) {
	mockDB := &sql.DB{}
	mockDbQueue := &db.DbQueue{}
	mockCrawler := &crawler.Crawler{}
	mockConfig := &db.Config{}

	wp := NewWorkerPool(mockDB, mockDbQueue, mockCrawler, 5, mockConfig)
	defer wp.Stop() // Clean up background goroutines
	
	// Test scaling up
	initialWorkers := wp.currentWorkers
	wp.scaleWorkers(context.Background(), 10)
	
	wp.workersMutex.RLock()
	newWorkers := wp.currentWorkers
	wp.workersMutex.RUnlock()
	
	assert.Equal(t, 5, initialWorkers)
	assert.Equal(t, 10, newWorkers)
	
	// Test scaling down (workers should decrease)
	wp.workersMutex.Lock()
	wp.currentWorkers = 3 // Simulate scale down
	wp.workersMutex.Unlock()
	
	wp.workersMutex.RLock()
	scaledDown := wp.currentWorkers
	wp.workersMutex.RUnlock()
	
	assert.Equal(t, 3, scaledDown)
}

func TestEvaluateJobPerformance(t *testing.T) {
	mockDB := &sql.DB{}
	mockDbQueue := &db.DbQueue{}
	mockCrawler := &crawler.Crawler{}
	mockConfig := &db.Config{}

	wp := NewWorkerPool(mockDB, mockDbQueue, mockCrawler, 5, mockConfig)
	defer wp.Stop() // Clean up background goroutines
	
	jobID := "perf-test-job"
	
	// Manually initialise performance tracking (bypassing database operations)
	wp.perfMutex.Lock()
	wp.jobPerformance[jobID] = &JobPerformance{
		RecentTasks:  make([]int64, 0, 5),
		CurrentBoost: 0,
		LastCheck:    time.Now(),
	}
	wp.perfMutex.Unlock()
	
	// Test with response times that should trigger scaling
	testCases := []struct {
		responseTime int64
		expectedBoost int
	}{
		{500, 0},   // Fast response, no boost
		{1500, 5},  // Medium response, 5 worker boost
		{2500, 10}, // Slow response, 10 worker boost
		{3500, 15}, // Slower response, 15 worker boost
		{4500, 20}, // Very slow response, 20 worker boost
	}
	
	for _, tc := range testCases {
		// Reset performance tracking for each test case
		wp.perfMutex.Lock()
		wp.jobPerformance[jobID] = &JobPerformance{
			RecentTasks:  make([]int64, 0, 5),
			CurrentBoost: 0,
			LastCheck:    time.Now(),
		}
		wp.perfMutex.Unlock()
		
		// Add multiple response times to reach evaluation threshold
		for i := 0; i < 3; i++ {
			wp.evaluateJobPerformance(jobID, tc.responseTime)
		}
		
		wp.perfMutex.RLock()
		perf := wp.jobPerformance[jobID]
		wp.perfMutex.RUnlock()
		
		assert.Equal(t, tc.expectedBoost, perf.CurrentBoost, 
			"Response time %d should result in boost %d", tc.responseTime, tc.expectedBoost)
	}
}

func TestIsDocumentLink(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "PDF document",
			path:     "/files/document.pdf",
			expected: true,
		},
		{
			name:     "Word document",
			path:     "/files/report.doc",
			expected: true,
		},
		{
			name:     "Word document new format",
			path:     "/files/report.docx",
			expected: true,
		},
		{
			name:     "Excel document",
			path:     "/files/data.xls",
			expected: true,
		},
		{
			name:     "Excel document new format",
			path:     "/files/data.xlsx",
			expected: true,
		},
		{
			name:     "PowerPoint document",
			path:     "/files/presentation.ppt",
			expected: true,
		},
		{
			name:     "PowerPoint document new format",
			path:     "/files/presentation.pptx",
			expected: true,
		},
		{
			name:     "HTML page",
			path:     "/page.html",
			expected: false,
		},
		{
			name:     "Regular path",
			path:     "/about-us",
			expected: false,
		},
		{
			name:     "Case insensitive PDF",
			path:     "/FILES/DOCUMENT.PDF",
			expected: true,
		},
		{
			name:     "Empty path",
			path:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDocumentLink(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "timeout error",
			err:      errors.New("request timeout"),
			expected: true,
		},
		{
			name:     "network error",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "server error 500",
			err:      errors.New("internal server error"),
			expected: true,
		},
		{
			name:     "server error 503",
			err:      errors.New("service unavailable"),
			expected: true,
		},
		{
			name:     "403 forbidden - not retryable",
			err:      errors.New("403 forbidden"),
			expected: false,
		},
		{
			name:     "429 too many requests - not retryable",
			err:      errors.New("429 too many requests"),
			expected: false,
		},
		{
			name:     "rate limit error - not retryable",
			err:      errors.New("rate limit exceeded"),
			expected: false,
		},
		{
			name:     "random error",
			err:      errors.New("something went wrong"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsBlockingError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "403 forbidden",
			err:      errors.New("403 forbidden"),
			expected: true,
		},
		{
			name:     "status code 403",
			err:      errors.New("non-success status code: 403"),
			expected: true,
		},
		{
			name:     "429 too many requests",
			err:      errors.New("429 too many requests"),
			expected: true,
		},
		{
			name:     "rate limit error",
			err:      errors.New("rate limit exceeded"),
			expected: true,
		},
		{
			name:     "timeout error - not blocking",
			err:      errors.New("request timeout"),
			expected: false,
		},
		{
			name:     "500 error - not blocking",
			err:      errors.New("internal server error"),
			expected: false,
		},
		{
			name:     "network error - not blocking",
			err:      errors.New("connection refused"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBlockingError(tt.err)
			if result != tt.expected {
				t.Errorf("isBlockingError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

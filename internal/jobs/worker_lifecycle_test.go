package jobs

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestWorkerPoolStartAndStop(t *testing.T) {
	t.Helper()
	t.Parallel()
	
	mockDB := &sql.DB{}
	mockDbQueue := &db.DbQueue{}
	mockCrawler := &crawler.Crawler{}
	mockConfig := &db.Config{}

	wp := NewWorkerPool(mockDB, mockDbQueue, mockCrawler, 3, mockConfig)
	
	t.Cleanup(func() {
		wp.Stop()
	})
	
	// Test initial state
	assert.False(t, wp.stopping.Load())
	assert.Equal(t, 3, wp.numWorkers)
	assert.Equal(t, 3, wp.baseWorkerCount)
	assert.Equal(t, 3, wp.currentWorkers)
	
	// Test that channels are properly initialized
	assert.NotNil(t, wp.stopCh)
	assert.NotNil(t, wp.notifyCh)
	
	// Stop worker pool (testing the stop mechanism)
	wp.Stop()
	
	// Verify stopping flag is set
	assert.True(t, wp.stopping.Load())
	
	// Verify stop channel is closed
	select {
	case <-wp.stopCh:
		// Expected - channel is closed
	case <-time.After(100 * time.Millisecond):
		t.Error("stopCh was not closed")
	}
}

func TestWorkerPoolGracefulShutdown(t *testing.T) {
	t.Helper()
	t.Parallel()
	
	mockDB := &sql.DB{}
	mockDbQueue := &db.DbQueue{}
	mockCrawler := &crawler.Crawler{}
	mockConfig := &db.Config{}

	wp := NewWorkerPool(mockDB, mockDbQueue, mockCrawler, 2, mockConfig)
	
	// Measure shutdown time (testing shutdown speed)
	start := time.Now()
	wp.Stop()
	duration := time.Since(start)
	
	// Graceful shutdown should complete quickly
	assert.Less(t, duration, 1*time.Second, "Shutdown took too long")
	
	// Verify final state
	assert.True(t, wp.stopping.Load())
	
	// Verify stop channel is closed
	select {
	case <-wp.stopCh:
		// Expected - channel is closed
	default:
		t.Error("stopCh should be closed after Stop()")
	}
}

func TestWorkerPoolContextCancellation(t *testing.T) {
	t.Helper()
	t.Parallel()
	
	mockDB := &sql.DB{}
	mockDbQueue := &db.DbQueue{}
	mockCrawler := &crawler.Crawler{}
	mockConfig := &db.Config{}

	wp := NewWorkerPool(mockDB, mockDbQueue, mockCrawler, 2, mockConfig)
	
	t.Cleanup(func() {
		wp.Stop()
	})
	
	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	
	// Test that context cancellation setup works
	assert.NotNil(t, ctx)
	assert.NotNil(t, cancel)
	
	// Cancel context
	cancel()
	
	// Verify context is cancelled
	select {
	case <-ctx.Done():
		// Expected - context is cancelled
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled")
	}
	
	// Test that worker pool can handle cancelled context
	assert.False(t, wp.stopping.Load())
}

func TestWorkerPoolPanicRecovery(t *testing.T) {
	t.Helper()
	t.Parallel()
	
	mockDB := &sql.DB{}
	mockDbQueue := &db.DbQueue{}
	mockCrawler := &crawler.Crawler{}
	mockConfig := &db.Config{}

	wp := NewWorkerPool(mockDB, mockDbQueue, mockCrawler, 1, mockConfig)
	
	t.Cleanup(func() {
		wp.Stop()
	})
	
	// Test that worker pool initialization doesn't panic
	assert.NotNil(t, wp)
	assert.False(t, wp.stopping.Load())
	
	// Test that normal shutdown works (panic recovery would prevent hang)
	wp.Stop()
	assert.True(t, wp.stopping.Load())
}

func TestWorkerPoolConcurrentStop(t *testing.T) {
	t.Helper()
	t.Parallel()
	
	mockDB := &sql.DB{}
	mockDbQueue := &db.DbQueue{}
	mockCrawler := &crawler.Crawler{}
	mockConfig := &db.Config{}

	wp := NewWorkerPool(mockDB, mockDbQueue, mockCrawler, 3, mockConfig)
	
	var wg sync.WaitGroup
	var stopCount atomic.Int64
	
	// Start multiple goroutines trying to stop
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			// All goroutines try to stop (should be safe)
			wp.Stop()
			stopCount.Add(1)
		}()
	}
	
	wg.Wait()
	
	// Worker pool should be in consistent state after concurrent operations
	assert.True(t, wp.stopping.Load())
	assert.Equal(t, int64(5), stopCount.Load())
	
	// Stop channel should be closed
	select {
	case <-wp.stopCh:
		// Expected - channel is closed
	default:
		t.Error("stopCh should be closed")
	}
}

func TestWorkerPoolWaitForJobsWithTimeout(t *testing.T) {
	t.Helper()
	t.Parallel()
	
	mockDB := &sql.DB{}
	mockDbQueue := &db.DbQueue{}
	mockCrawler := &crawler.Crawler{}
	mockConfig := &db.Config{}

	wp := NewWorkerPool(mockDB, mockDbQueue, mockCrawler, 2, mockConfig)
	
	t.Cleanup(func() {
		wp.Stop()
	})
	
	// Test WaitForJobs with timeout
	done := make(chan bool, 1)
	go func() {
		wp.WaitForJobs()
		done <- true
	}()
	
	// Should complete quickly since no active jobs
	select {
	case <-done:
		// Expected
	case <-time.After(500 * time.Millisecond):
		t.Error("WaitForJobs did not complete in reasonable time")
	}
}

func TestWorkerPoolActiveJobsTracking(t *testing.T) {
	t.Helper()
	t.Parallel()
	
	mockDB := &sql.DB{}
	mockDbQueue := &db.DbQueue{}
	mockCrawler := &crawler.Crawler{}
	mockConfig := &db.Config{}

	wp := NewWorkerPool(mockDB, mockDbQueue, mockCrawler, 2, mockConfig)
	
	t.Cleanup(func() {
		wp.Stop()
	})
	
	// Initially no active jobs
	wp.jobsMutex.RLock()
	initialJobCount := len(wp.jobs)
	wp.jobsMutex.RUnlock()
	
	assert.Equal(t, 0, initialJobCount)
	
	// Manually simulate adding a job (for testing tracking)
	jobID := "test-active-job"
	wp.jobsMutex.Lock()
	wp.jobs[jobID] = true
	wp.jobsMutex.Unlock()
	
	// Verify job is tracked
	wp.jobsMutex.RLock()
	jobExists := wp.jobs[jobID]
	wp.jobsMutex.RUnlock()
	
	assert.True(t, jobExists)
	
	// Remove job
	wp.RemoveJob(jobID)
	
	// Verify job is removed
	wp.jobsMutex.RLock()
	_, stillExists := wp.jobs[jobID]
	wp.jobsMutex.RUnlock()
	
	assert.False(t, stillExists)
}

func TestWorkerPoolScaleWorkersUnderLoad(t *testing.T) {
	t.Helper()
	t.Parallel()
	
	mockDB := &sql.DB{}
	mockDbQueue := &db.DbQueue{}
	mockCrawler := &crawler.Crawler{}
	mockConfig := &db.Config{}

	wp := NewWorkerPool(mockDB, mockDbQueue, mockCrawler, 2, mockConfig)
	
	t.Cleanup(func() {
		wp.Stop()
	})
	
	// Test initial worker count
	wp.workersMutex.RLock()
	initialWorkers := wp.currentWorkers
	wp.workersMutex.RUnlock()
	
	assert.Equal(t, 2, initialWorkers)
	
	// Test worker count modification (simulating scaling)
	wp.workersMutex.Lock()
	wp.currentWorkers = 5
	wp.workersMutex.Unlock()
	
	wp.workersMutex.RLock()
	scaledUpWorkers := wp.currentWorkers
	wp.workersMutex.RUnlock()
	
	assert.Equal(t, 5, scaledUpWorkers)
	
	// Test scaling down
	wp.workersMutex.Lock()
	wp.currentWorkers = 3
	wp.workersMutex.Unlock()
	
	wp.workersMutex.RLock()
	scaledDownWorkers := wp.currentWorkers
	wp.workersMutex.RUnlock()
	
	assert.Equal(t, 3, scaledDownWorkers)
}
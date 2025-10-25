//go:build unit || !integration

package jobs

import (
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/assert"
)

// TestWorkerPoolConstructor tests the NewWorkerPool constructor validation
func TestWorkerPoolConstructor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupFunc func() (*sql.DB, DbQueueInterface, CrawlerInterface, int, int, *db.Config)
		wantPanic bool
		panicMsg  string
	}{
		{
			name: "valid configuration",
			setupFunc: func() (*sql.DB, DbQueueInterface, CrawlerInterface, int, int, *db.Config) {
				return &sql.DB{}, &simpleDbQueueMock{}, &simpleCrawlerMock{}, 5, 1, &db.Config{}
			},
			wantPanic: false,
		},
		{
			name: "nil database",
			setupFunc: func() (*sql.DB, DbQueueInterface, CrawlerInterface, int, int, *db.Config) {
				return nil, &simpleDbQueueMock{}, &simpleCrawlerMock{}, 5, 1, &db.Config{}
			},
			wantPanic: true,
			panicMsg:  "database connection is required",
		},
		{
			name: "nil queue",
			setupFunc: func() (*sql.DB, DbQueueInterface, CrawlerInterface, int, int, *db.Config) {
				return &sql.DB{}, nil, &simpleCrawlerMock{}, 5, 1, &db.Config{}
			},
			wantPanic: true,
			panicMsg:  "database queue is required",
		},
		{
			name: "nil crawler",
			setupFunc: func() (*sql.DB, DbQueueInterface, CrawlerInterface, int, int, *db.Config) {
				return &sql.DB{}, &simpleDbQueueMock{}, nil, 5, 1, &db.Config{}
			},
			wantPanic: true,
			panicMsg:  "crawler is required",
		},
		{
			name: "zero workers",
			setupFunc: func() (*sql.DB, DbQueueInterface, CrawlerInterface, int, int, *db.Config) {
				return &sql.DB{}, &simpleDbQueueMock{}, &simpleCrawlerMock{}, 0, 1, &db.Config{}
			},
			wantPanic: true,
			panicMsg:  "numWorkers must be at least 1",
		},
		{
			name: "negative workers",
			setupFunc: func() (*sql.DB, DbQueueInterface, CrawlerInterface, int, int, *db.Config) {
				return &sql.DB{}, &simpleDbQueueMock{}, &simpleCrawlerMock{}, -1, 1, &db.Config{}
			},
			wantPanic: true,
			panicMsg:  "numWorkers must be at least 1",
		},
		{
			name: "zero concurrency",
			setupFunc: func() (*sql.DB, DbQueueInterface, CrawlerInterface, int, int, *db.Config) {
				return &sql.DB{}, &simpleDbQueueMock{}, &simpleCrawlerMock{}, 5, 0, &db.Config{}
			},
			wantPanic: true,
			panicMsg:  "workerConcurrency must be between 1 and 20",
		},
		{
			name: "excessive concurrency",
			setupFunc: func() (*sql.DB, DbQueueInterface, CrawlerInterface, int, int, *db.Config) {
				return &sql.DB{}, &simpleDbQueueMock{}, &simpleCrawlerMock{}, 5, 21, &db.Config{}
			},
			wantPanic: true,
			panicMsg:  "workerConcurrency must be between 1 and 20",
		},
		{
			name: "nil config",
			setupFunc: func() (*sql.DB, DbQueueInterface, CrawlerInterface, int, int, *db.Config) {
				return &sql.DB{}, &simpleDbQueueMock{}, &simpleCrawlerMock{}, 5, 1, nil
			},
			wantPanic: true,
			panicMsg:  "database configuration is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				assert.PanicsWithValue(t, tt.panicMsg, func() {
					db, queue, crawler, workers, concurrency, config := tt.setupFunc()
					NewWorkerPool(db, queue, crawler, workers, concurrency, config)
				})
			} else {
				assert.NotPanics(t, func() {
					db, queue, crawler, workers, concurrency, config := tt.setupFunc()
					wp := NewWorkerPool(db, queue, crawler, workers, concurrency, config)
					assert.NotNil(t, wp)
					assert.Equal(t, workers, wp.numWorkers)
					assert.Equal(t, workers, wp.baseWorkerCount)
					assert.Equal(t, concurrency, wp.workerConcurrency)
					assert.NotNil(t, wp.jobs)
					assert.NotNil(t, wp.stopCh)
					assert.NotNil(t, wp.notifyCh)
					assert.NotNil(t, wp.batchManager)
					assert.NotNil(t, wp.jobPerformance)
					assert.NotNil(t, wp.jobInfoCache)
					assert.NotNil(t, wp.workerSemaphores)
					assert.NotNil(t, wp.workerWaitGroups)
					assert.Len(t, wp.workerSemaphores, workers)
					assert.Len(t, wp.workerWaitGroups, workers)
				})
			}
		})
	}
}

// TestWorkerPoolInitialState tests the initial state after construction
func TestWorkerPoolInitialState(t *testing.T) {
	t.Parallel()

	wp := NewWorkerPool(&sql.DB{}, &simpleDbQueueMock{}, &simpleCrawlerMock{}, 3, 1, &db.Config{})

	// Check initial state
	assert.Equal(t, 3, wp.numWorkers)
	assert.Equal(t, 3, wp.baseWorkerCount)
	assert.Equal(t, 3, wp.currentWorkers)
	assert.False(t, wp.stopping.Load())
	assert.Empty(t, wp.jobs)
	assert.Empty(t, wp.jobPerformance)
	assert.Empty(t, wp.jobInfoCache)
	assert.NotNil(t, wp.batchManager)
}

// TestWorkerPoolSimpleJobTracking tests basic job tracking without database
func TestWorkerPoolSimpleJobTracking(t *testing.T) {
	wp := NewWorkerPool(&sql.DB{}, &simpleDbQueueMock{}, &simpleCrawlerMock{}, 2, 1, &db.Config{})

	// Directly manipulate the jobs map to avoid database calls
	jobID1 := "job1"
	jobID2 := "job2"

	// Add jobs directly
	wp.jobsMutex.Lock()
	wp.jobs[jobID1] = true
	wp.jobs[jobID2] = true
	wp.jobsMutex.Unlock()

	// Verify jobs are tracked
	wp.jobsMutex.RLock()
	assert.True(t, wp.jobs[jobID1])
	assert.True(t, wp.jobs[jobID2])
	assert.Len(t, wp.jobs, 2)
	wp.jobsMutex.RUnlock()

	// Remove a job
	wp.RemoveJob(jobID1)

	// Verify job was removed
	wp.jobsMutex.RLock()
	assert.False(t, wp.jobs[jobID1])
	assert.True(t, wp.jobs[jobID2])
	assert.Len(t, wp.jobs, 1)
	wp.jobsMutex.RUnlock()

	// Remove non-existent job (should not panic)
	assert.NotPanics(t, func() {
		wp.RemoveJob("non-existent")
	})
}

// TestWorkerPoolConcurrentJobTracking tests thread-safe job tracking
func TestWorkerPoolConcurrentJobTracking(t *testing.T) {
	t.Parallel()

	wp := NewWorkerPool(&sql.DB{}, &simpleDbQueueMock{}, &simpleCrawlerMock{}, 4, 1, &db.Config{})

	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	// Concurrently add and remove jobs
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				jobID := fmt.Sprintf("job-%d-%d", id, j)

				// Add job directly to avoid database
				wp.jobsMutex.Lock()
				wp.jobs[jobID] = true
				wp.jobsMutex.Unlock()

				// Small delay
				time.Sleep(time.Microsecond)

				// Remove job
				wp.RemoveJob(jobID)
			}
		}(i)
	}

	// Wait for completion
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Concurrent operations did not complete in time")
	}

	// Verify all jobs were removed
	wp.jobsMutex.RLock()
	assert.Empty(t, wp.jobs)
	wp.jobsMutex.RUnlock()
}

// TestWorkerPoolStopFlag tests the stopping flag behavior
func TestWorkerPoolStopFlag(t *testing.T) {
	t.Parallel()

	wp := NewWorkerPool(&sql.DB{}, &simpleDbQueueMock{}, &simpleCrawlerMock{}, 2, 1, &db.Config{})

	// Initially not stopping
	assert.False(t, wp.stopping.Load())

	// Set stopping flag
	wp.stopping.Store(true)
	assert.True(t, wp.stopping.Load())

	// Reset flag
	wp.stopping.Store(false)
	assert.False(t, wp.stopping.Load())
}

// TestWorkerPoolPerformanceInit tests performance tracking initialization
func TestWorkerPoolPerformanceInit(t *testing.T) {
	t.Parallel()

	wp := NewWorkerPool(&sql.DB{}, &simpleDbQueueMock{}, &simpleCrawlerMock{}, 2, 1, &db.Config{})

	jobID := "perf-job"

	// Initialize performance tracking directly
	wp.perfMutex.Lock()
	wp.jobPerformance[jobID] = &JobPerformance{
		RecentTasks:  make([]int64, 0, 5),
		CurrentBoost: 0,
		LastCheck:    time.Now(),
	}
	wp.perfMutex.Unlock()

	// Verify it was initialized
	wp.perfMutex.RLock()
	perf, exists := wp.jobPerformance[jobID]
	wp.perfMutex.RUnlock()

	assert.True(t, exists)
	assert.NotNil(t, perf)
	assert.Empty(t, perf.RecentTasks)
	assert.Equal(t, 0, perf.CurrentBoost)
	assert.WithinDuration(t, time.Now(), perf.LastCheck, time.Second)
}

// TestWorkerPoolBatchInit tests task batch initialization
func TestWorkerPoolBatchInit(t *testing.T) {
	t.Parallel()

	wp := NewWorkerPool(&sql.DB{}, &simpleDbQueueMock{}, &simpleCrawlerMock{}, 2, 1, &db.Config{})

	// Verify batch manager is initialized
	assert.NotNil(t, wp.batchManager)
}

// TestWorkerPoolJobInfoCaching tests the job info cache
func TestWorkerPoolJobInfoCaching(t *testing.T) {
	t.Parallel()

	wp := NewWorkerPool(&sql.DB{}, &simpleDbQueueMock{}, &simpleCrawlerMock{}, 2, 1, &db.Config{})

	jobID := "cached-job"
	jobInfo := &JobInfo{
		DomainName: "example.com",
		FindLinks:  true,
		CrawlDelay: 1000,
	}

	// Cache job info
	wp.jobInfoMutex.Lock()
	wp.jobInfoCache[jobID] = jobInfo
	wp.jobInfoMutex.Unlock()

	// Retrieve and verify
	wp.jobInfoMutex.RLock()
	cached, exists := wp.jobInfoCache[jobID]
	wp.jobInfoMutex.RUnlock()

	assert.True(t, exists)
	assert.Equal(t, jobInfo.DomainName, cached.DomainName)
	assert.Equal(t, jobInfo.FindLinks, cached.FindLinks)
	assert.Equal(t, jobInfo.CrawlDelay, cached.CrawlDelay)
}

// TestWorkerPoolMultipleWorkers tests configuration with different worker counts
func TestWorkerPoolMultipleWorkers(t *testing.T) {
	t.Parallel()

	testCases := []int{1, 5, 10, 50, 100}

	for _, numWorkers := range testCases {
		t.Run(fmt.Sprintf("%d_workers", numWorkers), func(t *testing.T) {
			wp := NewWorkerPool(&sql.DB{}, &simpleDbQueueMock{}, &simpleCrawlerMock{}, numWorkers, 1, &db.Config{})
			assert.Equal(t, numWorkers, wp.numWorkers)
			assert.Equal(t, numWorkers, wp.baseWorkerCount)
			assert.Equal(t, numWorkers, wp.currentWorkers)
		})
	}
}

// TestWorkerPoolStartContext tests that Start accepts a context
func TestWorkerPoolStartContext(t *testing.T) {
	// Start performs real DB operations (CleanupStuckJobs, recoverRunningJobs),
	// which will panic with a zero-value *sql.DB in unit tests.
	// This behavior is exercised in integration tests; skip here.
	t.Skip("requires real DB; covered by integration tests")
}

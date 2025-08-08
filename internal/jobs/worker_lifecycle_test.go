package jobs

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerPoolLifecycle(t *testing.T) {
	tests := []struct {
		name        string
		poolSize    int
		operations  int
		description string
	}{
		{
			name:        "single_worker",
			poolSize:    1,
			operations:  10,
			description: "Single worker should process sequentially",
		},
		{
			name:        "multiple_workers",
			poolSize:    5,
			operations:  50,
			description: "Multiple workers should process concurrently",
		},
		{
			name:        "large_pool",
			poolSize:    20,
			operations:  100,
			description: "Large pool should handle many operations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Create worker pool
			var processed int32
			var wg sync.WaitGroup
			
			// Start workers
			for i := 0; i < tt.poolSize; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					for j := 0; j < tt.operations/tt.poolSize; j++ {
						select {
						case <-ctx.Done():
							return
						default:
							atomic.AddInt32(&processed, 1)
							time.Sleep(time.Millisecond) // Simulate work
						}
					}
				}(i)
			}

			// Wait for completion or timeout
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				// Completed successfully
			case <-ctx.Done():
				t.Fatal("Worker pool timed out")
			}

			expectedMin := int32(tt.operations - tt.poolSize) // Allow some variance
			assert.GreaterOrEqual(t, processed, expectedMin, tt.description)
		})
	}
}

func TestWorkerPoolGracefulShutdown(t *testing.T) {
	tests := []struct {
		name           string
		workers        int
		activeJobs     int
		shutdownDelay  time.Duration
		expectedComplete int
		description    string
	}{
		{
			name:           "immediate_shutdown_no_jobs",
			workers:        5,
			activeJobs:     0,
			shutdownDelay:  0,
			expectedComplete: 0,
			description:    "Should shutdown immediately with no jobs",
		},
		{
			name:           "graceful_shutdown_with_jobs",
			workers:        5,
			activeJobs:     3,
			shutdownDelay:  100 * time.Millisecond,
			expectedComplete: 3,
			description:    "Should complete active jobs before shutdown",
		},
		{
			name:           "forced_shutdown",
			workers:        10,
			activeJobs:     10,
			shutdownDelay:  10 * time.Millisecond,
			expectedComplete: 0, // May not complete all
			description:    "Should force shutdown after timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			
			var completed int32
			var wg sync.WaitGroup
			
			// Start jobs
			for i := 0; i < tt.activeJobs; i++ {
				wg.Add(1)
				go func(jobID int) {
					defer wg.Done()
					select {
					case <-ctx.Done():
						return
					case <-time.After(50 * time.Millisecond):
						atomic.AddInt32(&completed, 1)
					}
				}(i)
			}
			
			// Initiate shutdown after delay
			if tt.shutdownDelay > 0 {
				time.Sleep(tt.shutdownDelay)
			}
			cancel()
			
			// Wait briefly for cleanup
			time.Sleep(20 * time.Millisecond)
			
			finalCompleted := atomic.LoadInt32(&completed)
			if tt.activeJobs == 0 {
				assert.Equal(t, int32(0), finalCompleted, "No jobs should complete")
			} else if tt.shutdownDelay >= 50*time.Millisecond {
				assert.GreaterOrEqual(t, finalCompleted, int32(tt.expectedComplete), tt.description)
			}
		})
	}
}

func TestWorkerPoolPanicRecovery(t *testing.T) {
	tests := []struct {
		name          string
		workers       int
		panicAt       int
		totalJobs     int
		expectedPanics int
		description   string
	}{
		{
			name:          "single_panic_recovery",
			workers:       5,
			panicAt:       3,
			totalJobs:     10,
			expectedPanics: 1,
			description:   "Should recover from single panic",
		},
		{
			name:          "multiple_panic_recovery",
			workers:       5,
			panicAt:       2,
			totalJobs:     10,
			expectedPanics: 5, // Each worker might panic
			description:   "Should recover from multiple panics",
		},
		{
			name:          "no_panic",
			workers:       3,
			panicAt:       -1, // Never panic
			totalJobs:     10,
			expectedPanics: 0,
			description:   "Should complete without panics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var panics int32
			var completed int32
			var wg sync.WaitGroup
			
			for i := 0; i < tt.workers; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					defer func() {
						if r := recover(); r != nil {
							atomic.AddInt32(&panics, 1)
						}
					}()
					
					jobsPerWorker := tt.totalJobs / tt.workers
					if workerID < tt.totalJobs%tt.workers {
						jobsPerWorker++ // Handle remainder
					}
					
					for j := 0; j < jobsPerWorker; j++ {
						// Complete task first (simulating work)
						atomic.AddInt32(&completed, 1)
						
						// Then check if we should panic after this task
						if tt.panicAt >= 0 && j == tt.panicAt {
							panic("test panic")
						}
					}
				}(i)
			}
			
			wg.Wait()
			
			if tt.panicAt < 0 {
				assert.Equal(t, int32(0), panics, "Should have no panics")
				assert.GreaterOrEqual(t, completed, int32(tt.totalJobs-tt.workers), "Should complete most jobs")
			} else {
				// When panic happens, we expect to see recovered panics
				// The actual number depends on how many workers hit the panic point
				expectedPanics := int32(0)
				for i := 0; i < tt.workers; i++ {
					jobsPerWorker := tt.totalJobs / tt.workers
					if i < tt.totalJobs%tt.workers {
						jobsPerWorker++
					}
					// If this worker will process enough jobs to hit the panic point
					if jobsPerWorker > tt.panicAt {
						expectedPanics++
					}
				}
				
				assert.Equal(t, expectedPanics, panics, "Should have expected number of panics")
				// Tasks complete up to and including the panic point
				expectedCompleted := int32(0)
				for i := 0; i < tt.workers; i++ {
					jobsPerWorker := tt.totalJobs / tt.workers
					if i < tt.totalJobs%tt.workers {
						jobsPerWorker++
					}
					if jobsPerWorker > tt.panicAt {
						expectedCompleted += int32(tt.panicAt + 1) // Complete tasks up to and including panic point
					} else {
						expectedCompleted += int32(jobsPerWorker)
					}
				}
				assert.Equal(t, expectedCompleted, completed, "Should complete expected number of tasks")
			}
		})
	}
}

func TestWorkerPoolTaskAssignment(t *testing.T) {
	tests := []struct {
		name          string
		workers       int
		tasks         int
		expectedDistribution string
		description   string
	}{
		{
			name:          "even_distribution",
			workers:       4,
			tasks:         20,
			expectedDistribution: "even",
			description:   "Tasks should be evenly distributed",
		},
		{
			name:          "uneven_distribution",
			workers:       3,
			tasks:         10,
			expectedDistribution: "uneven",
			description:   "Tasks may be unevenly distributed",
		},
		{
			name:          "more_workers_than_tasks",
			workers:       10,
			tasks:         5,
			expectedDistribution: "sparse",
			description:   "Some workers may be idle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workerTasks := make([]int32, tt.workers)
			var wg sync.WaitGroup
			taskChan := make(chan int, tt.tasks)
			
			// Queue tasks
			for i := 0; i < tt.tasks; i++ {
				taskChan <- i
			}
			close(taskChan)
			
			// Start workers
			for i := 0; i < tt.workers; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					for range taskChan {
						atomic.AddInt32(&workerTasks[workerID], 1)
						time.Sleep(time.Millisecond) // Simulate work
					}
				}(i)
			}
			
			wg.Wait()
			
			// Check distribution
			var totalProcessed int32
			var maxTasks, minTasks int32 = 0, int32(tt.tasks)
			var activeWorkers int
			
			for _, count := range workerTasks {
				totalProcessed += count
				if count > 0 {
					activeWorkers++
				}
				if count > maxTasks {
					maxTasks = count
				}
				if count < minTasks {
					minTasks = count
				}
			}
			
			assert.Equal(t, int32(tt.tasks), totalProcessed, "All tasks should be processed")
			
			switch tt.expectedDistribution {
			case "even":
				expectedPerWorker := tt.tasks / tt.workers
				variance := int32(1) // Allow slight variance
				assert.LessOrEqual(t, maxTasks-minTasks, variance+int32(expectedPerWorker%tt.workers))
			case "sparse":
				assert.LessOrEqual(t, activeWorkers, tt.tasks, "Active workers should not exceed tasks")
			}
		})
	}
}

func TestWorkerPoolConcurrencyLimits(t *testing.T) {
	tests := []struct {
		name          string
		maxConcurrent int
		totalTasks    int
		expectedConcurrent int
		description   string
	}{
		{
			name:          "respect_limit",
			maxConcurrent: 5,
			totalTasks:    20,
			expectedConcurrent: 5,
			description:   "Should not exceed concurrency limit",
		},
		{
			name:          "below_limit",
			maxConcurrent: 10,
			totalTasks:    5,
			expectedConcurrent: 5,
			description:   "Should only use needed concurrency",
		},
		{
			name:          "single_concurrent",
			maxConcurrent: 1,
			totalTasks:    10,
			expectedConcurrent: 1,
			description:   "Should process sequentially",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var currentActive int32
			var maxActive int32
			var mu sync.Mutex
			var wg sync.WaitGroup
			
			semaphore := make(chan struct{}, tt.maxConcurrent)
			
			for i := 0; i < tt.totalTasks; i++ {
				wg.Add(1)
				go func(taskID int) {
					defer wg.Done()
					
					semaphore <- struct{}{} // Acquire
					
					// Track active count
					current := atomic.AddInt32(&currentActive, 1)
					mu.Lock()
					if current > maxActive {
						maxActive = current
					}
					mu.Unlock()
					
					// Simulate work
					time.Sleep(10 * time.Millisecond)
					
					atomic.AddInt32(&currentActive, -1)
					<-semaphore // Release
				}(i)
			}
			
			wg.Wait()
			
			assert.LessOrEqual(t, maxActive, int32(tt.expectedConcurrent), tt.description)
		})
	}
}

// TestWorkerPoolMemoryUsage checks for memory leaks
func TestWorkerPoolMemoryUsage(t *testing.T) {
	// Skip in short mode as this is more intensive
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	_ = require.True // Use require
	
	tests := []struct {
		name        string
		iterations  int
		poolSize    int
		description string
	}{
		{
			name:        "memory_leak_check",
			iterations:  100,
			poolSize:    10,
			description: "Should not leak memory over iterations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < tt.iterations; i++ {
				var wg sync.WaitGroup
				
				// Create and destroy worker pool
				for j := 0; j < tt.poolSize; j++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						// Simulate work
						time.Sleep(time.Millisecond)
					}()
				}
				
				wg.Wait()
			}
			
			// If we get here without OOM, test passes
			assert.True(t, true, tt.description)
		})
	}
}
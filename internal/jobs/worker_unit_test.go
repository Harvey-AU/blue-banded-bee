//go:build unit || !integration

package jobs

import (
	"database/sql"
	"testing"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/assert"
)

// Test constructor validation only - doesn't need mocks
func TestNewWorkerPoolValidation(t *testing.T) {
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
		{
			name:        "valid configuration with 1 worker",
			db:          &sql.DB{},
			dbQueue:     &db.DbQueue{},
			crawler:     &crawler.Crawler{},
			numWorkers:  1,
			dbConfig:    &db.Config{},
			expectPanic: false,
		},
		{
			name:        "valid configuration with multiple workers",
			db:          &sql.DB{},
			dbQueue:     &db.DbQueue{},
			crawler:     &crawler.Crawler{},
			numWorkers:  10,
			dbConfig:    &db.Config{},
			expectPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				assert.PanicsWithValue(t, tt.panicMsg, func() {
					NewWorkerPool(tt.db, tt.dbQueue, tt.crawler, tt.numWorkers, tt.dbConfig)
				})
			} else {
				assert.NotPanics(t, func() {
					wp := NewWorkerPool(tt.db, tt.dbQueue, tt.crawler, tt.numWorkers, tt.dbConfig)
					assert.NotNil(t, wp)
					assert.Equal(t, tt.numWorkers, wp.numWorkers)
					assert.NotNil(t, wp.jobs)
					assert.NotNil(t, wp.stopCh)
					assert.NotNil(t, wp.notifyCh)
					// Don't call Start() or Stop() since we have real objects
				})
			}
		})
	}
}

// Test job tracking methods using direct manipulation to avoid DB calls
func TestWorkerPoolJobTracking(t *testing.T) {
	// Create a worker pool with valid objects (not started)
	wp := NewWorkerPool(&sql.DB{}, &db.DbQueue{}, &crawler.Crawler{}, 2, &db.Config{})
	
	// Test adding jobs directly to the map (bypassing AddJob which queries DB)
	wp.jobsMutex.Lock()
	wp.jobs["job1"] = true
	wp.jobs["job2"] = true
	wp.jobs["job3"] = true
	wp.jobsMutex.Unlock()
	
	assert.Equal(t, 3, len(wp.jobs))
	
	// Test duplicate adds (should be idempotent)
	wp.jobsMutex.Lock()
	wp.jobs["job1"] = true
	wp.jobsMutex.Unlock()
	assert.Equal(t, 3, len(wp.jobs))
	
	// Test removing jobs
	wp.RemoveJob("job2")
	assert.Equal(t, 2, len(wp.jobs))
	
	// Test removing non-existent job (should be safe)
	wp.RemoveJob("job4")
	assert.Equal(t, 2, len(wp.jobs))
	
	// Test clearing all jobs
	wp.RemoveJob("job1")
	wp.RemoveJob("job3")
	assert.Equal(t, 0, len(wp.jobs))
}
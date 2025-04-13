package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/src/common"
	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

var (
	instance *DB
	once     sync.Once
	initErr  error

	log zerolog.Logger

	globalQueue *common.DbQueue
	queueOnce   sync.Once
)

// DB represents a database connection with crawl result storage capabilities
type DB struct {
	client *sql.DB
}

// Config holds database connection configuration
type Config struct {
	URL       string // Database URL
	AuthToken string // Authentication token
}

// CrawlResult represents a stored crawl result in the database
type CrawlResult struct {
	ID           int64     `json:"id"`                     // Unique identifier
	JobID        string    `json:"job_id,omitempty"`       // Associated job ID
	TaskID       string    `json:"task_id,omitempty"`      // Associated task ID
	URL          string    `json:"url"`                    // Crawled URL
	ResponseTime int64     `json:"response_time_ms"`       // Response time in milliseconds
	StatusCode   int       `json:"status_code"`            // HTTP status code
	Error        string    `json:"error,omitempty"`        // Error message if any
	CacheStatus  string    `json:"cache_status,omitempty"` // Cache status
	CreatedAt    time.Time `json:"created_at"`             // Timestamp of the crawl
}

// GetInstance returns a singleton instance of DB
func GetInstance(config *Config) (*DB, error) {
	once.Do(func() {
		instance, initErr = New(config)
	})
	return instance, initErr
}

// New creates a new database connection with the given configuration
// It sets up connection pooling and initializes the schema
func New(config *Config) (*DB, error) {
	client, err := sql.Open("libsql", config.URL+"?authToken="+config.AuthToken)
	if err != nil {
		return nil, err
	}

	// Optimized connection pool settings
	client.SetMaxOpenConns(10) // More concurrent connections
	client.SetMaxIdleConns(5)  // Keep more idle connections
	client.SetConnMaxLifetime(5 * time.Minute)
	client.SetConnMaxIdleTime(2 * time.Minute)

	if err := client.Ping(); err != nil {
		return nil, err
	}

	if err := setupSchema(client); err != nil {
		return nil, err
	}

	return &DB{client: client}, nil
}

func setupSchema(db *sql.DB) error {
	// First create jobs and tasks tables
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			domain TEXT NOT NULL,
			status TEXT NOT NULL,
			progress REAL NOT NULL,
			total_tasks INTEGER NOT NULL,
			completed_tasks INTEGER NOT NULL,
			failed_tasks INTEGER NOT NULL,
			created_at DATETIME NOT NULL,
			started_at DATETIME,
			completed_at DATETIME,
			concurrency INTEGER NOT NULL,
			find_links BOOLEAN NOT NULL,
			include_paths TEXT,
			exclude_paths TEXT,
			required_workers INTEGER DEFAULT 0,
			error_message TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create jobs table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			job_id TEXT NOT NULL,
			url TEXT NOT NULL,
			status TEXT NOT NULL,
			depth INTEGER NOT NULL,
			created_at DATETIME NOT NULL,
			started_at DATETIME,
			completed_at DATETIME,
			retry_count INTEGER NOT NULL,
			error TEXT,
			status_code INTEGER,
			response_time INTEGER,
			cache_status TEXT,
			content_type TEXT,
			source_type TEXT NOT NULL,
			source_url TEXT,
			FOREIGN KEY (job_id) REFERENCES jobs(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create tasks table: %w", err)
	}

	// Create indexes for tasks
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_job_id ON tasks(job_id)`)
	if err != nil {
		return fmt.Errorf("failed to create task job_id index: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)`)
	if err != nil {
		return fmt.Errorf("failed to create task status index: %w", err)
	}

	// Then create crawl_results with proper foreign keys
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS crawl_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT NULL,
			task_id TEXT NULL,
			url TEXT NOT NULL,
			response_time INTEGER NOT NULL,
			status_code INTEGER,
			error TEXT,
			cache_status TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (job_id) REFERENCES jobs(id),
			FOREIGN KEY (task_id) REFERENCES tasks(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create crawl_results table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS test_connection (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create test_connection table: %w", err)
	}

	return nil
}

// StoreCrawlResult stores a new crawl result in the database
// It includes response time, status code, and cache status information
func (db *DB) StoreCrawlResult(ctx context.Context, result *CrawlResult) error {
	return db.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		span := sentry.StartSpan(ctx, "db.store_crawl_result")
		defer span.Finish()

		span.SetTag("url", result.URL)

		_, err := db.ExecWithMetrics(ctx, `
			INSERT INTO crawl_results (job_id, task_id, url, response_time, status_code, error, cache_status)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, result.JobID, result.TaskID, result.URL, result.ResponseTime, result.StatusCode, result.Error, result.CacheStatus)

		if err != nil {
			span.SetTag("error", "true")
			span.SetData("error.message", err.Error())
			log.Error().Err(err).
				Str("url", result.URL).
				Int64("response_time", result.ResponseTime).
				Int("status_code", result.StatusCode).
				Msg("Failed to store crawl result")
		}

		return err
	})
}

// GetRecentResults retrieves the most recent crawl results
// The limit parameter controls how many results to return
func (db *DB) GetRecentResults(ctx context.Context, limit int) ([]CrawlResult, error) {
	rows, err := db.QueryWithMetrics(ctx, `
		SELECT id, job_id, task_id, url, response_time, status_code, error, cache_status, created_at
		FROM crawl_results
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CrawlResult
	for rows.Next() {
		var r CrawlResult
		err := rows.Scan(&r.ID, &r.JobID, &r.TaskID, &r.URL, &r.ResponseTime, &r.StatusCode, &r.Error, &r.CacheStatus, &r.CreatedAt)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	log.Warn().Msg("Database connection being closed - add stack trace here")
	return db.client.Close()
}

// TestConnection tests the database connection by performing a simple query
func (db *DB) TestConnection() error {
	_, err := db.client.Exec(`
		INSERT INTO test_connection (created_at) 
		VALUES (CURRENT_TIMESTAMP)
	`)
	return err
}

func (db *DB) ResetSchema() error {
	log.Debug().Msg("Dropping all database tables")

	// Drop tables in correct order (respecting foreign key constraints)
	_, err := db.client.Exec(`DROP TABLE IF EXISTS crawl_results`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to drop crawl_results table")
		return err
	}

	_, err = db.client.Exec(`DROP TABLE IF EXISTS tasks`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to drop tasks table")
		return err
	}

	_, err = db.client.Exec(`DROP TABLE IF EXISTS jobs`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to drop jobs table")
		return err
	}

	log.Debug().Msg("Recreating database tables")

	// Use the single setupSchema function to recreate all tables
	if err := setupSchema(db.client); err != nil {
		return err
	}

	log.Debug().Msg("Database schema reset successfully")
	return nil
}

func (db *DB) ExecuteWithRetry(ctx context.Context, operation func(context.Context) error) error {
	var lastErr error
	retries := 3
	backoff := 100 * time.Millisecond

	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			log.Debug().Int("attempt", attempt).Msg("Retrying database operation")
			time.Sleep(backoff * time.Duration(1<<uint(attempt-1))) // Exponential backoff
		}

		if err := operation(ctx); err != nil {
			lastErr = err
			if isSQLiteTransientError(err) {
				continue // Retry on transient errors
			}
			return err // Don't retry on non-transient errors
		}
		return nil // Success
	}
	return fmt.Errorf("database operation failed after %d attempts: %w", retries+1, lastErr)
}

func isSQLiteTransientError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "database is locked") ||
		strings.Contains(errMsg, "busy") ||
		strings.Contains(errMsg, "connection reset by peer")
}

func (db *DB) QueryWithMetrics(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	span := sentry.StartSpan(ctx, "db.query")
	defer span.Finish()

	span.SetTag("db.query", query)

	startTime := time.Now()
	rows, err := db.client.QueryContext(ctx, query, args...)
	duration := time.Since(startTime)

	span.SetData("duration_ms", duration.Milliseconds())

	// Log slow queries
	if duration > 1000*time.Millisecond {
		log.Warn().
			Str("query", query).
			Dur("duration", duration).
			Msg("Slow database query detected")
	}

	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
	}

	return rows, err
}

func (db *DB) ExecWithMetrics(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	span := sentry.StartSpan(ctx, "db.exec")
	defer span.Finish()

	span.SetTag("db.query", query)

	startTime := time.Now()
	result, err := db.client.ExecContext(ctx, query, args...)
	duration := time.Since(startTime)

	span.SetData("duration_ms", duration.Milliseconds())

	// Log slow operations
	if duration > 1000*time.Millisecond {
		log.Warn().
			Str("query", query).
			Dur("duration", duration).
			Msg("Slow database operation detected")
	}

	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
	}

	return result, err
}

// GetDB returns the underlying database connection
func (db *DB) GetDB() *sql.DB {
	return db.client
}

// GetQueue returns the database queue singleton
func (db *DB) GetQueue() *common.DbQueue {
	queueOnce.Do(func() {
		globalQueue = common.NewDbQueue(db.client)
	})
	return globalQueue
}

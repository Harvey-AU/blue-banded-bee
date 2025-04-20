package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

// DB represents a PostgreSQL database connection
type DB struct {
	client *sql.DB
}

// Config holds PostgreSQL connection configuration
type Config struct {
	ConnectionString string
}

// CrawlResult represents a stored crawl result
type CrawlResult struct {
	ID           int64     `json:"id"`
	JobID        string    `json:"job_id,omitempty"`
	TaskID       string    `json:"task_id,omitempty"`
	URL          string    `json:"url"`
	ResponseTime int64     `json:"response_time_ms"`
	StatusCode   int       `json:"status_code"`
	Error        string    `json:"error,omitempty"`
	CacheStatus  string    `json:"cache_status,omitempty"`
	ContentType  string    `json:"content_type,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// New creates a new PostgreSQL database connection
func New(config *Config) (*DB, error) {
	client, err := sql.Open("postgres", config.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Configure connection pool
	client.SetMaxOpenConns(25)
	client.SetMaxIdleConns(10)
	client.SetConnMaxLifetime(5 * time.Minute)
	client.SetConnMaxIdleTime(2 * time.Minute)

	// Test connection
	if err := client.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	// Initialize schema
	if err := setupSchema(client); err != nil {
		return nil, fmt.Errorf("failed to setup schema: %w", err)
	}

	return &DB{client: client}, nil
}

// InitFromEnv creates a PostgreSQL connection using environment variables
func InitFromEnv() (*DB, error) {
	// Get connection string from environment variables
	connectionString := os.Getenv("DATABASE_URL")
	if connectionString == "" {
		return nil, errors.New("DATABASE_URL environment variable is required")
	}

	config := &Config{
		ConnectionString: connectionString,
	}

	return New(config)
}

// setupSchema creates the necessary tables in PostgreSQL
func setupSchema(db *sql.DB) error {
	// Create jobs table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			domain TEXT NOT NULL,
			status TEXT NOT NULL,
			progress REAL NOT NULL,
			total_tasks INTEGER NOT NULL,
			completed_tasks INTEGER NOT NULL,
			failed_tasks INTEGER NOT NULL,
			created_at TIMESTAMP NOT NULL,
			started_at TIMESTAMP,
			completed_at TIMESTAMP,
			concurrency INTEGER NOT NULL,
			find_links BOOLEAN NOT NULL,
			include_paths JSONB,
			exclude_paths JSONB,
			required_workers INTEGER DEFAULT 0,
			error_message TEXT,
			max_depth INTEGER DEFAULT 1
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create jobs table: %w", err)
	}

	// Create tasks table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
		
			id TEXT PRIMARY KEY,
			job_id TEXT NOT NULL,
			url TEXT NOT NULL,
			status TEXT NOT NULL,
			depth INTEGER NOT NULL,
			created_at TIMESTAMP NOT NULL,
			started_at TIMESTAMP,
			completed_at TIMESTAMP,
			retry_count INTEGER NOT NULL,
			error TEXT,
			source_type TEXT NOT NULL,
			source_url TEXT,
			status_code INTEGER,
			response_time BIGINT,
			cache_status TEXT,
			content_type TEXT,
			FOREIGN KEY (job_id) REFERENCES jobs(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create tasks table: %w", err)
	}

	// Create crawl_results table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS crawl_results (
			id SERIAL PRIMARY KEY,
			job_id TEXT,
			task_id TEXT,
			url TEXT NOT NULL,
			response_time BIGINT NOT NULL,
			status_code INTEGER,
			error TEXT,
			cache_status TEXT,
			content_type TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create crawl_results table: %w", err)
	}

	// Create indexes
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_job_id ON tasks(job_id)`)
	if err != nil {
		return fmt.Errorf("failed to create task job_id index: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)`)
	if err != nil {
		return fmt.Errorf("failed to create task status index: %w", err)
	}

	// Add PostgreSQL-specific index for task queue
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_status_created ON tasks(status, created_at)`)
	if err != nil {
		return fmt.Errorf("failed to create task status/created_at index: %w", err)
	}

	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.client.Close()
}

// GetDB returns the underlying database connection
func (db *DB) GetDB() *sql.DB {
	return db.client
}

// StoreCrawlResult stores a new crawl result in the database
func (db *DB) StoreCrawlResult(ctx context.Context, result *CrawlResult) error {
	log.Debug().
		Str("url", result.URL).
		Int64("response_time", result.ResponseTime).
		Int("status_code", result.StatusCode).
		Msg("Storing crawl result")

	_, err := db.client.ExecContext(ctx, `
		INSERT INTO crawl_results (job_id, task_id, url, response_time, status_code, error, cache_status, content_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, result.JobID, result.TaskID, result.URL, result.ResponseTime,
		result.StatusCode, result.Error, result.CacheStatus, result.ContentType)

	if err != nil {
		log.Error().Err(err).
			Str("url", result.URL).
			Int64("response_time", result.ResponseTime).
			Int("status_code", result.StatusCode).
			Msg("Failed to store crawl result")
		return err
	}

	log.Info().
		Str("url", result.URL).
		Msg("Successfully stored crawl result")

	return nil
}

// GetRecentResults retrieves the most recent crawl results
func (db *DB) GetRecentResults(ctx context.Context, limit int) ([]CrawlResult, error) {
	rows, err := db.client.QueryContext(ctx, `
		SELECT id, job_id, task_id, url, response_time, status_code, error, cache_status, content_type, created_at
		FROM crawl_results
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CrawlResult
	for rows.Next() {
		var r CrawlResult
		err := rows.Scan(&r.ID, &r.JobID, &r.TaskID, &r.URL, &r.ResponseTime, &r.StatusCode,
			&r.Error, &r.CacheStatus, &r.ContentType, &r.CreatedAt)
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

// ResetSchema resets the database schema
func (db *DB) ResetSchema() error {
	log.Warn().Msg("Resetting PostgreSQL schema")

	// Drop tables in reverse order to respect foreign keys
	_, err := db.client.Exec(`DROP TABLE IF EXISTS crawl_results`)
	if err != nil {
		return err
	}

	_, err = db.client.Exec(`DROP TABLE IF EXISTS tasks`)
	if err != nil {
		return err
	}

	_, err = db.client.Exec(`DROP TABLE IF EXISTS jobs`)
	if err != nil {
		return err
	}

	// Recreate schema
	return setupSchema(db.client)
}

// ExecWithMetrics executes a SQL statement with metrics tracking
func (db *DB) ExecWithMetrics(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	startTime := time.Now()
	result, err := db.client.ExecContext(ctx, query, args...)
	duration := time.Since(startTime)

	// Log slow operations
	if duration > 1000*time.Millisecond {
		log.Warn().
			Str("query", query).
			Dur("duration", duration).
			Msg("Slow database operation detected")
	}

	return result, err
}

// QueryWithMetrics executes a SQL query with metrics tracking
func (db *DB) QueryWithMetrics(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	startTime := time.Now()
	rows, err := db.client.QueryContext(ctx, query, args...)
	duration := time.Since(startTime)

	// Log slow queries
	if duration > 1000*time.Millisecond {
		log.Warn().
			Str("query", query).
			Dur("duration", duration).
			Msg("Slow database query detected")
	}

	return rows, err
}

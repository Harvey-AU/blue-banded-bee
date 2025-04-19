package postgres

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// Config holds PostgreSQL connection configuration
type Config struct {
	ConnectionString string
}

// DB represents a PostgreSQL database connection
type DB struct {
	client *sql.DB
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

// SetupSchema creates the necessary tables in PostgreSQL
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
			job_id TEXT NULL,
			task_id TEXT NULL,
			url TEXT NOT NULL,
			response_time BIGINT NOT NULL,
			status_code INTEGER,
			error TEXT,
			cache_status TEXT,
			content_type TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (job_id) REFERENCES jobs(id),
			FOREIGN KEY (task_id) REFERENCES tasks(id)
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

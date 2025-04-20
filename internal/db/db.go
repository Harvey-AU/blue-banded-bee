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
	// Create domains lookup table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS domains (
			id SERIAL PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create domains table: %w", err)
	}

	// Create pages lookup table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS pages (
			id SERIAL PRIMARY KEY,
			domain_id INTEGER NOT NULL REFERENCES domains(id),
			path TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE(domain_id, path)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create pages table: %w", err)
	}

	// Create jobs table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			domain_id INTEGER NOT NULL REFERENCES domains(id),
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
			page_id INTEGER NOT NULL REFERENCES pages(id),
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

// ResetSchema resets the database schema
func (db *DB) ResetSchema() error {
	log.Warn().Msg("Resetting PostgreSQL schema")

	// Drop tables in reverse order to respect foreign keys
	_, err := db.client.Exec(`DROP TABLE IF EXISTS tasks`)
	if err != nil {
		return err
	}

	_, err = db.client.Exec(`DROP TABLE IF EXISTS jobs`)
	if err != nil {
		return err
	}

	_, err = db.client.Exec(`DROP TABLE IF EXISTS pages`)
	if err != nil {
		return err
	}

	_, err = db.client.Exec(`DROP TABLE IF EXISTS domains`)
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

// GetOrCreateDomain returns the id for a domain name, inserting it if necessary
func (db *DB) GetOrCreateDomain(ctx context.Context, name string) (int, error) {
	var id int
	// insert or ignore
	if _, err := db.client.ExecContext(ctx,
		`INSERT INTO domains(name) VALUES($1) ON CONFLICT (name) DO NOTHING`, name); err != nil {
		return 0, err
	}
	// select id
	err := db.client.QueryRowContext(ctx, `SELECT id FROM domains WHERE name=$1`, name).Scan(&id)
	return id, err
}

// GetOrCreatePage returns the id for a page path under a domain, inserting it if necessary
func (db *DB) GetOrCreatePage(ctx context.Context, domainID int, path string) (int, error) {
	var id int
	if _, err := db.client.ExecContext(ctx,
		`INSERT INTO pages(domain_id, path) VALUES($1,$2) ON CONFLICT (domain_id,path) DO NOTHING`,
		domainID, path); err != nil {
		return 0, err
	}
	err := db.client.QueryRowContext(ctx,
		`SELECT id FROM pages WHERE domain_id=$1 AND path=$2`, domainID, path).Scan(&id)
	return id, err
}

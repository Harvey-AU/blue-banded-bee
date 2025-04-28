package db

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/Harvey-AU/blue-banded-bee/internal/common"
	"github.com/rs/zerolog/log"
)

// DB represents a PostgreSQL database connection
type DB struct {
	client *sql.DB
	config *Config
	queue  *common.DbQueue
}

// GetConfig returns the original DB connection settings
func (d *DB) GetConfig() *Config {
	return d.config
}

// Config holds PostgreSQL connection configuration
type Config struct {
	Host         string        // Database host
	Port         string        // Database port
	User         string        // Database user
	Password     string        // Database password
	Database     string        // Database name
	SSLMode      string        // SSL mode (disable, require, verify-ca, verify-full)
	MaxIdleConns int           // Maximum number of idle connections
	MaxOpenConns int           // Maximum number of open connections
	MaxLifetime  time.Duration // Maximum lifetime of a connection
	DatabaseURL  string        // Original DATABASE_URL if used
}

// ConnectionString returns the PostgreSQL connection string
func (c *Config) ConnectionString() string {
	// If we have a DatabaseURL, use it directly
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}

	// Otherwise use the individual components
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode)
}

// New creates a new PostgreSQL database connection
func New(config *Config) (*DB, error) {
	// Validate required fields
	if config.Host == "" {
		return nil, fmt.Errorf("database host is required")
	}
	if config.Port == "" {
		return nil, fmt.Errorf("database port is required")
	}
	if config.User == "" {
		return nil, fmt.Errorf("database user is required")
	}
	if config.Database == "" {
		return nil, fmt.Errorf("database name is required")
	}

	// Set defaults for optional fields
	if config.SSLMode == "" {
		config.SSLMode = "disable"
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 10
	}
	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = 25
	}
	if config.MaxLifetime == 0 {
		config.MaxLifetime = 5 * time.Minute
	}

	client, err := sql.Open("postgres", config.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Configure connection pool
	client.SetMaxOpenConns(config.MaxOpenConns)
	client.SetMaxIdleConns(config.MaxIdleConns)
	client.SetConnMaxLifetime(config.MaxLifetime)

	// Test connection
	if err := client.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	// Initialize schema
	if err := setupSchema(client); err != nil {
		return nil, fmt.Errorf("failed to setup schema: %w", err)
	}

	return &DB{client: client, config: config}, nil
}

// InitFromEnv creates a PostgreSQL connection using environment variables
func InitFromEnv() (*DB, error) {
	// If DATABASE_URL is provided, use it directly
	if url := os.Getenv("DATABASE_URL"); url != "" {
		client, err := sql.Open("postgres", url)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to PostgreSQL via DATABASE_URL: %w", err)
		}
		client.SetMaxOpenConns(25)
		client.SetMaxIdleConns(10)
		client.SetConnMaxLifetime(5 * time.Minute)
		// Verify connection
		if err := client.Ping(); err != nil {
			return nil, fmt.Errorf("failed to ping PostgreSQL via DATABASE_URL: %w", err)
		}
		// Initialise schema
		if err := setupSchema(client); err != nil {
			return nil, fmt.Errorf("failed to setup schema: %w", err)
		}

		// Create a config that stores the original DATABASE_URL
		config := &Config{
			// Set this special field for DATABASE_URL
			DatabaseURL: url,
		}

		return &DB{client: client, config: config}, nil
	}

	config := &Config{
		Host:         os.Getenv("POSTGRES_HOST"),
		Port:         os.Getenv("POSTGRES_PORT"),
		User:         os.Getenv("POSTGRES_USER"),
		Password:     os.Getenv("POSTGRES_PASSWORD"),
		Database:     os.Getenv("POSTGRES_DB"),
		SSLMode:      os.Getenv("POSTGRES_SSL_MODE"),
		MaxIdleConns: 10,
		MaxOpenConns: 25,
		MaxLifetime:  5 * time.Minute,
	}

	// Use defaults if not set
	if config.Host == "" {
		config.Host = "localhost"
	}
	if config.Port == "" {
		config.Port = "5432"
	}
	if config.User == "" {
		config.User = "postgres"
	}
	if config.Database == "" {
		config.Database = "blue_banded_bee"
	}

	// Create the database connection
	db, err := New(config)
	if err != nil {
		return nil, err
	}

	return db, nil
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
			sitemap_tasks INTEGER NOT NULL DEFAULT 0,
			found_tasks INTEGER NOT NULL DEFAULT 0,
			total_tasks INTEGER NOT NULL DEFAULT 0,
			completed_tasks INTEGER NOT NULL DEFAULT 0,
			failed_tasks INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL,
			started_at TIMESTAMP,
			completed_at TIMESTAMP,
			concurrency INTEGER NOT NULL,
			find_links BOOLEAN NOT NULL,
			max_pages INTEGER NOT NULL
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
			path TEXT NOT NULL,
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

	// Add a unique constraint to prevent duplicate tasks for same page in a job
	_, err = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_job_page_unique 
		ON tasks(job_id, page_id)
	`)
	if err != nil {
		return fmt.Errorf("failed to create unique index on tasks: %w", err)
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

	// Enable Row-Level Security for all tables
	tables := []string{"domains", "pages", "jobs", "tasks"}
	for _, table := range tables {
		// Enable RLS on the table
		_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ENABLE ROW LEVEL SECURITY", table))
		if err != nil {
			return fmt.Errorf("failed to enable RLS on %s table: %w", table, err)
		}
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

// GetQueue returns the database queue for serialized operations
func (db *DB) GetQueue() *common.DbQueue {
	// Create the queue on first access if needed
	if db.queue == nil {
		db.queue = common.NewDbQueue(db.client)
	}
	return db.queue
}

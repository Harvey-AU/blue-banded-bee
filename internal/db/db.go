package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/rs/zerolog/log"
)

// DB represents a PostgreSQL database connection
type DB struct {
	client *sql.DB
	config *Config
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
		config.MaxIdleConns = 25
	}
	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = 75
	}
	if config.MaxLifetime == 0 {
		config.MaxLifetime = 30 * time.Minute
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
		client.SetMaxOpenConns(75)
		client.SetMaxIdleConns(25)
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
		MaxIdleConns: 25,
		MaxOpenConns: 75,
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
	// Create organisations table first (referenced by users and jobs)
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS organisations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create organisations table: %w", err)
	}

	// Create users table (extends Supabase auth.users)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			email TEXT NOT NULL,
			full_name TEXT,
			organisation_id UUID REFERENCES organisations(id),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE(email)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Create domains lookup table
	_, err = db.Exec(`
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
			user_id UUID REFERENCES users(id),
			organisation_id UUID REFERENCES organisations(id),
			status TEXT NOT NULL,
			progress REAL NOT NULL,
			sitemap_tasks INTEGER NOT NULL DEFAULT 0,
			found_tasks INTEGER NOT NULL DEFAULT 0,
			total_tasks INTEGER NOT NULL DEFAULT 0,
			completed_tasks INTEGER NOT NULL DEFAULT 0,
			failed_tasks INTEGER NOT NULL DEFAULT 0,
			skipped_tasks INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL,
			started_at TIMESTAMP,
			completed_at TIMESTAMP,
			concurrency INTEGER NOT NULL,
			find_links BOOLEAN NOT NULL,
			max_pages INTEGER NOT NULL,
			include_paths TEXT,
			exclude_paths TEXT,
			required_workers INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create jobs table: %w", err)
	}

	// Add skipped_tasks column if it doesn't exist (for existing databases)
	_, err = db.Exec(`
		ALTER TABLE jobs ADD COLUMN IF NOT EXISTS skipped_tasks INTEGER DEFAULT 0
	`)
	if err != nil {
		return fmt.Errorf("failed to add skipped_tasks column: %w", err)
	}

	// Add user_id and organisation_id columns if they don't exist (for existing databases)
	_, err = db.Exec(`
		ALTER TABLE jobs ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)
	`)
	if err != nil {
		return fmt.Errorf("failed to add user_id column: %w", err)
	}

	_, err = db.Exec(`
		ALTER TABLE jobs ADD COLUMN IF NOT EXISTS organisation_id UUID REFERENCES organisations(id)
	`)
	if err != nil {
		return fmt.Errorf("failed to add organisation_id column: %w", err)
	}

	// Add error_message column if it doesn't exist (for existing databases)
	_, err = db.Exec(`
		ALTER TABLE jobs ADD COLUMN IF NOT EXISTS error_message TEXT
	`)
	if err != nil {
		return fmt.Errorf("failed to add error_message column: %w", err)
	}

	// Create tasks table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			job_id TEXT NOT NULL,
			page_id INTEGER NOT NULL REFERENCES pages(id),
			path TEXT NOT NULL,
			status TEXT NOT NULL,
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
			second_response_time BIGINT,
			second_cache_status TEXT,
			priority_score NUMERIC(4,3) DEFAULT 0.000,
			FOREIGN KEY (job_id) REFERENCES jobs(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create tasks table: %w", err)
	}

	// Add cache warming columns for existing databases (backward compatibility)
	_, err = db.Exec(`
		ALTER TABLE tasks ADD COLUMN IF NOT EXISTS second_response_time BIGINT
	`)
	if err != nil {
		return fmt.Errorf("failed to add second_response_time column: %w", err)
	}

	_, err = db.Exec(`
		ALTER TABLE tasks ADD COLUMN IF NOT EXISTS second_cache_status TEXT
	`)
	if err != nil {
		return fmt.Errorf("failed to add second_cache_status column: %w", err)
	}

	// Add priority_score column for task prioritization
	_, err = db.Exec(`
		ALTER TABLE tasks ADD COLUMN IF NOT EXISTS priority_score NUMERIC(4,3) DEFAULT 0.000
	`)
	if err != nil {
		return fmt.Errorf("failed to add priority_score column: %w", err)
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

	// Add index for priority-based task queries
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(job_id, status, priority_score DESC)`)
	if err != nil {
		return fmt.Errorf("failed to create task priority index: %w", err)
	}

	// Enable Row-Level Security for all tables
	tables := []string{"organisations", "users", "domains", "pages", "jobs", "tasks"}
	for _, table := range tables {
		// Enable RLS on the table
		_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ENABLE ROW LEVEL SECURITY", table))
		if err != nil {
			return fmt.Errorf("failed to enable RLS on %s table: %w", table, err)
		}
	}

	// Set up Row Level Security policies
	err = setupRLSPolicies(db)
	if err != nil {
		return fmt.Errorf("failed to setup RLS policies: %w", err)
	}

	// Create database triggers for automatic timestamp and progress management
	err = setupTimestampTriggers(db)
	if err != nil {
		return fmt.Errorf("failed to setup timestamp triggers: %w", err)
	}

	err = setupProgressTriggers(db)
	if err != nil {
		return fmt.Errorf("failed to setup progress triggers: %w", err)
	}

	return nil
}

// setupTimestampTriggers creates database triggers for automatic timestamp management
func setupTimestampTriggers(db *sql.DB) error {
	// Function to automatically set started_at when first task completes
	_, err := db.Exec(`
		CREATE OR REPLACE FUNCTION set_job_started_at()
		RETURNS TRIGGER AS $$
		BEGIN
		  -- Only set started_at if it's currently NULL and completed_tasks > 0
		  -- Handle both INSERT and UPDATE operations
		  IF NEW.completed_tasks > 0 AND (TG_OP = 'INSERT' OR OLD.started_at IS NULL) AND NEW.started_at IS NULL THEN
		    NEW.started_at = NOW();
		  END IF;
		  
		  RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
	`)
	if err != nil {
		return fmt.Errorf("failed to create set_job_started_at function: %w", err)
	}

	// Function to automatically set completed_at when job reaches 100%
	_, err = db.Exec(`
		CREATE OR REPLACE FUNCTION set_job_completed_at()
		RETURNS TRIGGER AS $$
		BEGIN
		  -- Set completed_at when progress reaches 100% and it's not already set
		  -- Handle both INSERT and UPDATE operations
		  IF NEW.progress >= 100.0 AND (TG_OP = 'INSERT' OR OLD.completed_at IS NULL) AND NEW.completed_at IS NULL THEN
		    NEW.completed_at = NOW();
		  END IF;
		  
		  RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
	`)
	if err != nil {
		return fmt.Errorf("failed to create set_job_completed_at function: %w", err)
	}

	// Create trigger for started_at (INSERT OR UPDATE)
	_, err = db.Exec(`
		DROP TRIGGER IF EXISTS trigger_set_job_started ON jobs;
		CREATE TRIGGER trigger_set_job_started
		  BEFORE INSERT OR UPDATE ON jobs
		  FOR EACH ROW
		  EXECUTE FUNCTION set_job_started_at();
	`)
	if err != nil {
		return fmt.Errorf("failed to create started_at trigger: %w", err)
	}

	// Create trigger for completed_at (INSERT OR UPDATE)
	_, err = db.Exec(`
		DROP TRIGGER IF EXISTS trigger_set_job_completed ON jobs;
		CREATE TRIGGER trigger_set_job_completed
		  BEFORE INSERT OR UPDATE ON jobs
		  FOR EACH ROW
		  EXECUTE FUNCTION set_job_completed_at();
	`)
	if err != nil {
		return fmt.Errorf("failed to create completed_at trigger: %w", err)
	}

	return nil
}

// setupProgressTriggers creates database triggers for automatic progress calculation
func setupProgressTriggers(db *sql.DB) error {
	// Function to automatically calculate job progress when tasks change
	_, err := db.Exec(`
		CREATE OR REPLACE FUNCTION update_job_progress()
		RETURNS TRIGGER AS $$
		DECLARE
		    job_id_to_update TEXT;
		    total_tasks INTEGER;
		    completed_count INTEGER;
		    failed_count INTEGER;
		    skipped_count INTEGER;
		    new_progress REAL;
		BEGIN
		    -- Determine which job to update
		    IF TG_OP = 'DELETE' THEN
		        job_id_to_update = OLD.job_id;
		    ELSE
		        job_id_to_update = NEW.job_id;
		    END IF;
		    
		    -- Get the total tasks for this job
		    SELECT j.total_tasks INTO total_tasks
		    FROM jobs j
		    WHERE j.id = job_id_to_update;
		    
		    -- Count completed, failed, and skipped tasks
		    SELECT 
		        COUNT(*) FILTER (WHERE status = 'completed'),
		        COUNT(*) FILTER (WHERE status = 'failed'),
		        COUNT(*) FILTER (WHERE status = 'skipped')
		    INTO completed_count, failed_count, skipped_count
		    FROM tasks
		    WHERE job_id = job_id_to_update;
		    
		    -- Calculate progress percentage (only count completed + failed, not skipped)
		    IF total_tasks > 0 AND (total_tasks - skipped_count) > 0 THEN
		        new_progress = (completed_count + failed_count)::REAL / (total_tasks - skipped_count)::REAL * 100.0;
		    ELSE
		        new_progress = 0.0;
		    END IF;
		    
		    -- Update the job with new counts and progress
		    UPDATE jobs
		    SET 
		        completed_tasks = completed_count,
		        failed_tasks = failed_count,
		        skipped_tasks = skipped_count,
		        progress = new_progress,
		        status = CASE 
		            WHEN new_progress >= 100.0 THEN 'completed'
		            WHEN completed_count > 0 OR failed_count > 0 THEN 'running'
		            ELSE status
		        END
		    WHERE id = job_id_to_update;
		    
		    -- Return the appropriate record based on operation
		    IF TG_OP = 'DELETE' THEN
		        RETURN OLD;
		    ELSE
		        RETURN NEW;
		    END IF;
		END;
		$$ LANGUAGE plpgsql;
	`)
	if err != nil {
		return fmt.Errorf("failed to create update_job_progress function: %w", err)
	}

	// Create trigger on tasks table to update job progress
	_, err = db.Exec(`
		DROP TRIGGER IF EXISTS trigger_update_job_progress ON tasks;
		CREATE TRIGGER trigger_update_job_progress
		  AFTER INSERT OR UPDATE OR DELETE ON tasks
		  FOR EACH ROW
		  EXECUTE FUNCTION update_job_progress();
	`)
	if err != nil {
		return fmt.Errorf("failed to create job progress trigger: %w", err)
	}

	return nil
}

// setupRLSPolicies creates Row Level Security policies for user data access
func setupRLSPolicies(db *sql.DB) error {
	// Create policy for users table - users can only access their own data
	_, err := db.Exec(`
		DROP POLICY IF EXISTS "Users can access own data" ON users;
		CREATE POLICY "Users can access own data" ON users
		FOR ALL USING (auth.uid() = id);
	`)
	if err != nil {
		return fmt.Errorf("failed to create users RLS policy: %w", err)
	}

	// Create policy for organisations table - users can access their organisation
	_, err = db.Exec(`
		DROP POLICY IF EXISTS "Users can access own organisation" ON organisations;
		CREATE POLICY "Users can access own organisation" ON organisations
		FOR ALL USING (
			id IN (
				SELECT organisation_id FROM users WHERE id = auth.uid()
			)
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create organisations RLS policy: %w", err)
	}

	// Create policy for jobs table - organisation members can access shared jobs
	_, err = db.Exec(`
		DROP POLICY IF EXISTS "Organisation members can access jobs" ON jobs;
		CREATE POLICY "Organisation members can access jobs" ON jobs
		FOR ALL USING (
			organisation_id IN (
				SELECT organisation_id FROM users WHERE id = auth.uid()
			)
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create jobs RLS policy: %w", err)
	}

	// Create policy for tasks table - organisation members can access tasks for their jobs
	_, err = db.Exec(`
		DROP POLICY IF EXISTS "Organisation members can access tasks" ON tasks;
		CREATE POLICY "Organisation members can access tasks" ON tasks
		FOR ALL USING (
			job_id IN (
				SELECT id FROM jobs WHERE organisation_id IN (
					SELECT organisation_id FROM users WHERE id = auth.uid()
				)
			)
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create tasks RLS policy: %w", err)
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

	// First drop any views that depend on the tables
	log.Debug().Msg("Dropping views")
	views := []string{"job_list", "job_dashboard", "job_status_summary", "task_status_summary"}
	for _, view := range views {
		_, err := db.client.Exec(fmt.Sprintf(`DROP VIEW IF EXISTS %s CASCADE`, view))
		if err != nil {
			log.Warn().Err(err).Str("view", view).Msg("Failed to drop view (may not exist)")
			// Don't return error for views, as they may not exist
		} else {
			log.Debug().Str("view", view).Msg("Successfully dropped view")
		}
	}

	// Drop tables in reverse order to respect foreign keys
	// Use CASCADE to handle any remaining dependencies
	tables := []string{"tasks", "jobs", "pages", "domains"}
	
	for _, table := range tables {
		log.Debug().Str("table", table).Msg("Dropping table")
		_, err := db.client.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS %s CASCADE`, table))
		if err != nil {
			log.Error().Err(err).Str("table", table).Msg("Failed to drop table")
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
		log.Debug().Str("table", table).Msg("Successfully dropped table")
	}

	// Also drop any sequences that might exist
	log.Debug().Msg("Dropping sequences")
	sequences := []string{"domains_id_seq", "pages_id_seq"}
	for _, seq := range sequences {
		_, err := db.client.Exec(fmt.Sprintf(`DROP SEQUENCE IF EXISTS %s CASCADE`, seq))
		if err != nil {
			log.Warn().Err(err).Str("sequence", seq).Msg("Failed to drop sequence (may not exist)")
			// Don't return error for sequences, as they may not exist
		}
	}

	log.Debug().Msg("Recreating schema")
	// Recreate schema
	err := setupSchema(db.client)
	if err != nil {
		log.Error().Err(err).Msg("Failed to recreate schema")
		return fmt.Errorf("failed to recreate schema: %w", err)
	}
	
	log.Info().Msg("Successfully reset database schema")
	return nil
}


// Serialize converts data to JSON string representation
func Serialize(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		log.Error().Err(err).Msg("Failed to serialize data")
		return "{}"
	}
	return string(data)
}

/**
 * Database Retry Logic
 *
 * This function implements exponential backoff with jitter to gracefully handle
 * transient database errors like connection issues or deadlocks.
 *
 * The retryDB function:
 * 1. Attempts the database operation
 * 2. If the operation fails with a retryable error, it waits and retries
 *    with increasing backoff periods
 * 3. A small random jitter is added to prevent retry storms
 * 4. It gives up after a maximum number of retries
 *
 * This mechanism significantly improves reliability when multiple workers are
 * simultaneously accessing the database.
 */

// retryDB executes a database operation with exponential backoff retry
func retryDB(operation func() error) error {
	var lastErr error
	retries := 5
	backoff := 200 * time.Millisecond

	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			time.Sleep(backoff * time.Duration(1<<uint(attempt-1)))
		}
		err := operation()
		if err == nil {
			return nil
		}

		// Check if error is a database lock error or connection issue
		if strings.Contains(err.Error(), "database is locked") ||
			strings.Contains(err.Error(), "busy") ||
			strings.Contains(err.Error(), "connection reset") {
			// Calculate backoff with jitter
			backoff := backoff * time.Duration(1<<uint(attempt))
			jitter := time.Duration(rand.Int63n(int64(backoff) / 2))
			sleepTime := backoff + jitter

			log.Warn().
				Err(err).
				Int("attempt", attempt+1).
				Dur("backoff", sleepTime).
				Msg("Database error, retrying operation")

			time.Sleep(sleepTime)
			continue
		}

		// Not a retryable error
		lastErr = err
		break
	}

	return lastErr
}

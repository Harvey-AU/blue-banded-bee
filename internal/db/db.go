package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/cache"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rs/zerolog/log"
)

// DB represents a PostgreSQL database connection
type DB struct {
	client *sql.DB
	config *Config
	Cache  *cache.InMemoryCache
}

// GetConfig returns the original DB connection settings
func (d *DB) GetConfig() *Config {
	return d.config
}

// Config holds PostgreSQL connection configuration
type Config struct {
	Host            string        // Database host
	Port            string        // Database port
	User            string        // Database user
	Password        string        // Database password
	Database        string        // Database name
	SSLMode         string        // SSL mode (disable, require, verify-ca, verify-full)
	MaxIdleConns    int           // Maximum number of idle connections
	MaxOpenConns    int           // Maximum number of open connections
	MaxLifetime     time.Duration // Maximum lifetime of a connection
	DatabaseURL     string        // Original DATABASE_URL if used
	ApplicationName string        // Identifier for this application instance
}

func poolLimitsForEnv(appEnv string) (maxOpen, maxIdle int) {
	switch appEnv {
	case "production":
		return 37, 15
	case "staging":
		return 5, 2
	default:
		return 2, 1
	}
}

func sanitiseAppName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_', r == ':', r == '.':
			builder.WriteRune(r)
		default:
			// Skip unsupported characters to keep connection strings safe
		}
	}

	result := builder.String()
	if result == "" {
		return ""
	}
	return result
}

func trimAppName(name string) string {
	const maxLen = 60 // postgres application_name limit is 64 bytes
	if len(name) <= maxLen {
		return name
	}
	return name[:maxLen]
}

func determineApplicationName() string {
	if override := sanitiseAppName(os.Getenv("DB_APP_NAME")); override != "" {
		return trimAppName(override)
	}

	base := "bbb"
	if env := sanitiseAppName(strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))); env != "" {
		base = fmt.Sprintf("bbb-%s", env)
	}

	var parts []string
	if machineID := sanitiseAppName(os.Getenv("FLY_MACHINE_ID")); machineID != "" {
		parts = append(parts, machineID)
	}
	if host, err := os.Hostname(); err == nil {
		if hostName := sanitiseAppName(host); hostName != "" {
			parts = append(parts, hostName)
		}
	}
	parts = append(parts, time.Now().UTC().Format("20060102T150405"))

	if len(parts) == 0 {
		return trimAppName(base)
	}

	return trimAppName(fmt.Sprintf("%s:%s", base, strings.Join(parts, ":")))
}

func addConnSetting(connStr, key, value string) (string, bool) {
	if key == "" || value == "" {
		return connStr, false
	}

	trimmed := strings.TrimSpace(connStr)
	if trimmed == "" {
		return connStr, false
	}

	if strings.Contains(trimmed, key+"=") {
		return trimmed, false
	}

	isURL := strings.HasPrefix(trimmed, "postgres://") || strings.HasPrefix(trimmed, "postgresql://")

	if isURL {
		parsed, err := url.Parse(trimmed)
		if err == nil {
			q := parsed.Query()
			if q.Get(key) != "" {
				return trimmed, false
			}
			q.Set(key, value)
			parsed.RawQuery = q.Encode()
			return parsed.String(), true
		}

		separator := "?"
		if strings.Contains(trimmed, "?") {
			separator = "&"
		}
		return trimmed + separator + key + "=" + url.QueryEscape(value), true
	}

	escaped := strings.ReplaceAll(value, "'", "")
	if escaped == "" {
		return trimmed, false
	}
	return trimmed + fmt.Sprintf(" %s=%s", key, escaped), true
}

func cleanupAppConnections(ctx context.Context, client *sql.DB, appName string) {
	if client == nil || appName == "" {
		return
	}

	base := appName
	if idx := strings.Index(base, ":"); idx != -1 {
		base = base[:idx]
	}
	if base == "" {
		return
	}

	pattern := base + ":%"
	if base == appName {
		pattern = base
	}

	cleanupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		SELECT COALESCE(SUM(CASE WHEN pg_terminate_backend(pid) THEN 1 ELSE 0 END), 0)
		FROM pg_stat_activity
		WHERE pid != pg_backend_pid()
		  AND usename = current_user
		  AND state = 'idle'
		  AND application_name LIKE $1
		  AND application_name <> $2
	`

	var terminated int64
	if err := client.QueryRowContext(cleanupCtx, query, pattern, appName).Scan(&terminated); err != nil {
		log.Warn().Err(err).Msg("Failed to terminate stale PostgreSQL connections for application")
		return
	}

	if terminated > 0 {
		log.Info().
			Str("application_name", appName).
			Int64("terminated_connections", terminated).
			Msg("Terminated stale PostgreSQL connections from previous deployment")
	} else {
		log.Debug().
			Str("application_name", appName).
			Msg("No stale PostgreSQL connections found for termination")
	}
}

// ConnectionString returns the PostgreSQL connection string
func (c *Config) ConnectionString() string {
	connStr := strings.TrimSpace(c.DatabaseURL)
	if connStr != "" {
		connStr, _ = addConnSetting(connStr, "idle_in_transaction_session_timeout", "30000")
		connStr, _ = addConnSetting(connStr, "statement_timeout", "60000")
		if strings.Contains(connStr, "pooler.supabase.com") {
			if newStr, added := addConnSetting(connStr, "default_query_exec_mode", "simple_protocol"); added {
				log.Info().Msg("Added minimal prepared statement disabling for pooler connection")
				connStr = newStr
			} else {
				connStr = newStr
			}
		}
		if c.ApplicationName != "" {
			connStr, _ = addConnSetting(connStr, "application_name", c.ApplicationName)
		}
		return connStr
	}

	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "require"
	}

	connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, sslMode)

	connStr, _ = addConnSetting(connStr, "idle_in_transaction_session_timeout", "30000")
	connStr, _ = addConnSetting(connStr, "statement_timeout", "60000")
	if strings.Contains(connStr, "pooler.supabase.com") {
		if newStr, added := addConnSetting(connStr, "default_query_exec_mode", "simple_protocol"); added {
			log.Info().Msg("Added minimal prepared statement disabling for pooler connection")
			connStr = newStr
		} else {
			connStr = newStr
		}
	}
	if c.ApplicationName != "" {
		connStr, _ = addConnSetting(connStr, "application_name", c.ApplicationName)
	}

	return connStr
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// If we have a DatabaseURL, that's sufficient
	if c.DatabaseURL != "" {
		return nil
	}

	// Otherwise, check individual fields
	if c.Host == "" || c.Port == "" || c.User == "" || c.Password == "" || c.Database == "" {
		if c.Host == "" && c.Port == "" && c.User == "" && c.Password == "" && c.Database == "" {
			return fmt.Errorf("database configuration required")
		}
		return fmt.Errorf("incomplete database configuration")
	}

	return nil
}

// New creates a new PostgreSQL database connection
func New(config *Config) (*DB, error) {
	// Validate required fields only if not using DATABASE_URL
	if config.DatabaseURL == "" {
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
	}

	// Set defaults for optional fields
	if config.SSLMode == "" {
		config.SSLMode = "disable"
	}
	if config.MaxIdleConns == 0 {
		// Environment-based idle connection limits (40% of max open)
		switch os.Getenv("APP_ENV") {
		case "production":
			config.MaxIdleConns = 13
		case "staging":
			config.MaxIdleConns = 4
		default:
			config.MaxIdleConns = 1
		}
	}
	if config.MaxOpenConns == 0 {
		// Environment-based connection limits to prevent pool exhaustion
		switch os.Getenv("APP_ENV") {
		case "production":
			config.MaxOpenConns = 32
		case "staging":
			config.MaxOpenConns = 10
		default:
			config.MaxOpenConns = 3
		}
	}
	if config.MaxLifetime == 0 {
		config.MaxLifetime = 5 * time.Minute // Shorter lifetime for pooler compatibility
	}

	if config.ApplicationName == "" {
		config.ApplicationName = determineApplicationName()
	}

	connStr := config.ConnectionString()

	log.Info().Msg("Opening PostgreSQL connection")

	client, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Configure connection pool
	client.SetMaxOpenConns(config.MaxOpenConns)
	client.SetMaxIdleConns(config.MaxIdleConns)
	client.SetConnMaxLifetime(config.MaxLifetime)
	client.SetConnMaxIdleTime(2 * time.Minute) // Close idle connections after 2 minutes

	// Test connection
	if err := client.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	cleanupAppConnections(context.Background(), client, config.ApplicationName)

	// Schema is managed by Supabase migrations - no setup required

	// Create the cache
	dbCache := cache.NewInMemoryCache()

	return &DB{client: client, config: config, Cache: dbCache}, nil
}

// InitFromURLWithSuffix creates a PostgreSQL connection using the provided URL and optional
// application name suffix. It applies the same environment-based pooling limits as InitFromEnv.
func InitFromURLWithSuffix(databaseURL string, appEnv string, appNameSuffix string) (*DB, error) {
	trimmed := strings.TrimSpace(databaseURL)
	if trimmed == "" {
		return nil, fmt.Errorf("database url cannot be empty")
	}

	maxOpen, maxIdle := poolLimitsForEnv(appEnv)
	appName := determineApplicationName()
	if suffix := sanitiseAppName(appNameSuffix); suffix != "" {
		if appName != "" {
			appName = trimAppName(fmt.Sprintf("%s:%s", appName, suffix))
		} else {
			appName = trimAppName(suffix)
		}
	}

	config := &Config{
		DatabaseURL:     trimmed,
		MaxIdleConns:    maxIdle,
		MaxOpenConns:    maxOpen,
		MaxLifetime:     5 * time.Minute,
		ApplicationName: appName,
	}

	db, err := New(config)
	if err != nil {
		return nil, err
	}

	return db, nil
}

// InitFromEnv creates a PostgreSQL connection using environment variables
func InitFromEnv() (*DB, error) {
	// If DATABASE_URL is provided, use it with default config
	// Trim whitespace as it causes pgx to ignore the URL and fall back to Unix socket
	if url := strings.TrimSpace(os.Getenv("DATABASE_URL")); url != "" {
		// Optimise connection limits based on environment
		maxOpen, maxIdle := poolLimitsForEnv(os.Getenv("APP_ENV"))

		appName := determineApplicationName()

		config := &Config{
			DatabaseURL:     url,
			MaxIdleConns:    maxIdle,
			MaxOpenConns:    maxOpen,
			MaxLifetime:     5 * time.Minute, // Shorter lifetime for pooler compatibility
			ApplicationName: appName,
		}

		url, _ = addConnSetting(url, "statement_timeout", "60000")
		url, _ = addConnSetting(url, "idle_in_transaction_session_timeout", "30000")

		if strings.Contains(url, "pooler.supabase.com") {
			if newStr, added := addConnSetting(url, "default_query_exec_mode", "simple_protocol"); added {
				log.Info().Msg("Added minimal prepared statement disabling for pooler connection")
				url = newStr
			} else {
				url = newStr
			}

			if newStr, added := addConnSetting(url, "pgbouncer", "true"); added {
				log.Info().Msg("Enabled transaction pooling mode (pgbouncer=true)")
				url = newStr
			} else {
				url = newStr
			}
		}

		if appName != "" {
			url, _ = addConnSetting(url, "application_name", appName)
		}

		// Persist the augmented URL back to config for consistency
		config.DatabaseURL = url

		log.Info().Str("connection_url", url).Msg("Opening PostgreSQL connection via DATABASE_URL")

		client, err := sql.Open("pgx", url)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to PostgreSQL via DATABASE_URL: %w", err)
		}

		// Configure connection pool using the same settings
		client.SetMaxOpenConns(config.MaxOpenConns)
		client.SetMaxIdleConns(config.MaxIdleConns)
		client.SetConnMaxLifetime(config.MaxLifetime)
		client.SetConnMaxIdleTime(2 * time.Minute) // Close idle connections after 2 minutes

		// Verify connection
		if err := client.Ping(); err != nil {
			return nil, fmt.Errorf("failed to ping PostgreSQL via DATABASE_URL: %w", err)
		}

		cleanupAppConnections(context.Background(), client, config.ApplicationName)

		// Schema is managed by Supabase migrations - no setup required

		// Create the cache
		dbCache := cache.NewInMemoryCache()

		return &DB{client: client, config: config, Cache: dbCache}, nil
	}

	// Fallback to individual environment variables
	maxOpen, maxIdle := poolLimitsForEnv(os.Getenv("APP_ENV"))

	appName := determineApplicationName()

	config := &Config{
		Host:            os.Getenv("POSTGRES_HOST"),
		Port:            os.Getenv("POSTGRES_PORT"),
		User:            os.Getenv("POSTGRES_USER"),
		Password:        os.Getenv("POSTGRES_PASSWORD"),
		Database:        os.Getenv("POSTGRES_DB"),
		SSLMode:         os.Getenv("POSTGRES_SSL_MODE"),
		MaxIdleConns:    maxIdle,
		MaxOpenConns:    maxOpen,
		MaxLifetime:     5 * time.Minute,
		ApplicationName: appName,
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

// createCoreTables creates all core database tables
func createCoreTables(db *sql.DB) error {
	// Create organisations table first (referenced by users and jobs)
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS organisations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(email)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Create domains table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS domains (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			crawl_delay INTEGER DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create domains table: %w", err)
	}

	// Create pages table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS pages (
			id SERIAL PRIMARY KEY,
			domain_id INTEGER NOT NULL REFERENCES domains(id),
			path TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
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
			created_at TIMESTAMPTZ NOT NULL,
			started_at TIMESTAMPTZ,
			completed_at TIMESTAMPTZ,
			concurrency INTEGER NOT NULL,
			find_links BOOLEAN NOT NULL,
			max_pages INTEGER NOT NULL,
			include_paths TEXT,
			exclude_paths TEXT,
			required_workers INTEGER DEFAULT 0,
			error_message TEXT,
			source_type TEXT,
			source_detail TEXT,
			source_info TEXT,
			duration_seconds INTEGER GENERATED ALWAYS AS (
				CASE 
					WHEN started_at IS NOT NULL AND completed_at IS NOT NULL 
					THEN EXTRACT(EPOCH FROM (completed_at - started_at))::INTEGER
					ELSE NULL
				END
			) STORED
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create jobs table: %w", err)
	}

	// Create tasks table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			job_id TEXT NOT NULL REFERENCES jobs(id),
			page_id INTEGER NOT NULL REFERENCES pages(id),
			path TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			started_at TIMESTAMPTZ,
			completed_at TIMESTAMPTZ,
			retry_count INTEGER NOT NULL,
			error TEXT,
			source_type TEXT NOT NULL,
			source_url TEXT,
			status_code INTEGER,
			response_time BIGINT,
			cache_status TEXT,
			content_type TEXT,
			content_length BIGINT,
			headers JSONB,
			redirect_url TEXT,
			dns_lookup_time INTEGER,
			tcp_connection_time INTEGER,
			tls_handshake_time INTEGER,
			ttfb INTEGER,
			content_transfer_time INTEGER,
			second_response_time BIGINT,
			second_cache_status TEXT,
			second_content_length BIGINT,
			second_headers JSONB,
			priority_score REAL DEFAULT 1.0
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create tasks table: %w", err)
	}

	return nil
}

// createPerformanceIndexes creates database indexes for optimal query performance
func createPerformanceIndexes(db *sql.DB) error {
	// Create basic task lookup index
	_, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_job_id ON tasks(job_id)`)
	if err != nil {
		return fmt.Errorf("failed to create task job_id index: %w", err)
	}

	// Drop deprecated indexes if they exist
	_, err = db.Exec(`DROP INDEX IF EXISTS idx_tasks_status`)
	if err != nil {
		return fmt.Errorf("failed to drop old status index: %w", err)
	}
	_, err = db.Exec(`DROP INDEX IF EXISTS idx_tasks_status_created`)
	if err != nil {
		return fmt.Errorf("failed to drop old status/created_at index: %w", err)
	}
	_, err = db.Exec(`DROP INDEX IF EXISTS idx_tasks_priority`)
	if err != nil {
		return fmt.Errorf("failed to drop old priority index: %w", err)
	}

	// Create optimised index for worker task claiming
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_pending_claim_order ON tasks (created_at) WHERE status = 'pending'`)
	if err != nil {
		return fmt.Errorf("failed to create optimised pending task index: %w", err)
	}

	// Index for dashboard/API queries on job status and priority
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_job_status_priority ON tasks(job_id, status, priority_score DESC)`)
	if err != nil {
		return fmt.Errorf("failed to create task job/status/priority index: %w", err)
	}

	// Unique constraint to prevent duplicate tasks for same job/page combination
	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_job_page_unique ON tasks(job_id, page_id)`)
	if err != nil {
		return fmt.Errorf("failed to create unique job/page index: %w", err)
	}

	return nil
}

// enableRowLevelSecurity enables RLS on all tables and sets up policies
func enableRowLevelSecurity(db *sql.DB) error {
	// Enable Row-Level Security for all tables
	tables := []string{"organisations", "users", "domains", "pages", "jobs", "tasks"}
	for _, table := range tables {
		// Enable RLS on the table
		_, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ENABLE ROW LEVEL SECURITY", table))
		if err != nil {
			return fmt.Errorf("failed to enable RLS on %s table: %w", table, err)
		}
	}

	// Set up Row Level Security policies
	err := setupRLSPolicies(db)
	if err != nil {
		return fmt.Errorf("failed to setup RLS policies: %w", err)
	}

	return nil
}

// setupSchema creates the necessary tables in PostgreSQL
func setupSchema(db *sql.DB) error {
	// Create all core database tables
	if err := createCoreTables(db); err != nil {
		return err
	}

	// Create performance indexes
	if err := createPerformanceIndexes(db); err != nil {
		return err
	}

	// Enable Row-Level Security
	if err := enableRowLevelSecurity(db); err != nil {
		return err
	}

	// Create database triggers for automatic timestamp and progress management
	if err := setupTimestampTriggers(db); err != nil {
		return fmt.Errorf("failed to setup timestamp triggers: %w", err)
	}

	if err := setupProgressTriggers(db); err != nil {
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
		    NEW.started_at = CURRENT_TIMESTAMP AT TIME ZONE 'UTC';
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
		    NEW.completed_at = CURRENT_TIMESTAMP AT TIME ZONE 'UTC';
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
// IMPORTANT: Uses (SELECT auth.uid()) pattern to prevent per-row evaluation
// Reference: https://supabase.com/docs/guides/database/postgres/row-level-security#call-functions-with-select
func setupRLSPolicies(db *sql.DB) error {
	// Create policy for users table - users can only access their own data
	// Uses (SELECT auth.uid()) to cache result per query instead of per-row evaluation
	_, err := db.Exec(`
		DROP POLICY IF EXISTS "Users can access own data" ON users;
		CREATE POLICY "Users can access own data" ON users
		FOR ALL USING (id = (SELECT auth.uid()));
	`)
	if err != nil {
		return fmt.Errorf("failed to create users RLS policy: %w", err)
	}

	// Create policy for organisations table - users can access their organisation
	// Wraps auth.uid() in SELECT to prevent per-row re-evaluation
	_, err = db.Exec(`
		DROP POLICY IF EXISTS "Users can access own organisation" ON organisations;
		CREATE POLICY "Users can access own organisation" ON organisations
		FOR ALL USING (
			id = (
				SELECT organisation_id
				FROM users
				WHERE id = (SELECT auth.uid())
			)
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create organisations RLS policy: %w", err)
	}

	// Create policy for jobs table - organisation members can access shared jobs
	// Uses cached auth.uid() to avoid 10,000x overhead on large result sets
	_, err = db.Exec(`
		DROP POLICY IF EXISTS "Organisation members can access jobs" ON jobs;
		CREATE POLICY "Organisation members can access jobs" ON jobs
		FOR ALL USING (
			organisation_id = (
				SELECT organisation_id
				FROM users
				WHERE id = (SELECT auth.uid())
			)
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create jobs RLS policy: %w", err)
	}

	// Create policy for tasks table - organisation members can access tasks for their jobs
	// Uses EXISTS with cached auth.uid() for optimal performance on massive task tables
	_, err = db.Exec(`
		DROP POLICY IF EXISTS "Organisation members can access tasks" ON tasks;
		CREATE POLICY "Organisation members can access tasks" ON tasks
		FOR ALL USING (
			EXISTS (
				SELECT 1
				FROM jobs
				WHERE jobs.id = tasks.job_id
				  AND jobs.organisation_id = (
					SELECT organisation_id
					FROM users
					WHERE id = (SELECT auth.uid())
				  )
			)
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create tasks RLS policy: %w", err)
	}

	// Domains table: Split policies for workflow + tenant isolation
	// Allow INSERT without existing job, restrict SELECT to owned jobs
	_, err = db.Exec(`
		DROP POLICY IF EXISTS "Organisation members can access domains" ON domains;
		DROP POLICY IF EXISTS "Users can read domains via jobs" ON domains;
		DROP POLICY IF EXISTS "Authenticated users can create domains" ON domains;
		DROP POLICY IF EXISTS "Users can update domains via jobs" ON domains;

		-- Allow reading domains that have jobs in user's organisation
		CREATE POLICY "Users can read domains via jobs"
		ON domains FOR SELECT
		USING (
		  EXISTS (
			SELECT 1
			FROM jobs
			WHERE jobs.domain_id = domains.id
			  AND jobs.organisation_id = (
				SELECT organisation_id
				FROM users
				WHERE id = (SELECT auth.uid())
			  )
		  )
		);

		-- Allow any authenticated user to create domains (checked at job level)
		-- Workers need to create domains before jobs exist
		CREATE POLICY "Authenticated users can create domains"
		ON domains FOR INSERT
		WITH CHECK (auth.role() = 'authenticated');

		-- NO UPDATE POLICY: Domains are shared resources
		-- Service role only can update to prevent cross-tenant data corruption
	`)
	if err != nil {
		return fmt.Errorf("failed to create domains RLS policies: %w", err)
	}

	// Pages table: Similar split for tenant isolation while allowing worker inserts
	_, err = db.Exec(`
		DROP POLICY IF EXISTS "Organisation members can access pages" ON pages;
		DROP POLICY IF EXISTS "Users can read pages via jobs" ON pages;
		DROP POLICY IF EXISTS "Authenticated users can create pages" ON pages;
		DROP POLICY IF EXISTS "Users can update pages via jobs" ON pages;

		-- Allow reading pages that have jobs in user's organisation
		CREATE POLICY "Users can read pages via jobs"
		ON pages FOR SELECT
		USING (
		  EXISTS (
			SELECT 1
			FROM jobs
			WHERE jobs.domain_id = pages.domain_id
			  AND jobs.organisation_id = (
				SELECT organisation_id
				FROM users
				WHERE id = (SELECT auth.uid())
			  )
		  )
		);

		-- Allow any authenticated user to create pages (checked at job level)
		-- Workers discover and create pages during crawling
		CREATE POLICY "Authenticated users can create pages"
		ON pages FOR INSERT
		WITH CHECK (auth.role() = 'authenticated');

		-- NO UPDATE POLICY: Pages are shared resources
		-- Service role only can update to prevent cross-tenant data corruption
	`)
	if err != nil {
		return fmt.Errorf("failed to create pages RLS policies: %w", err)
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
	log.Warn().Msg("Clearing all data from database tables")

	// Delete data in order that respects foreign key constraints
	// Start with child tables first, then parent tables
	tables := []string{"tasks", "jobs", "job_share_links", "pages", "domains"}

	for _, table := range tables {
		log.Debug().Str("table", table).Msg("Deleting all rows")
		result, err := db.client.Exec(fmt.Sprintf(`DELETE FROM %s`, table))
		if err != nil {
			log.Error().Err(err).Str("table", table).Msg("Failed to delete rows")
			return fmt.Errorf("failed to delete rows from %s: %w", table, err)
		}

		rowsAffected, _ := result.RowsAffected()
		log.Info().Str("table", table).Int64("rows_deleted", rowsAffected).Msg("Cleared table data")
	}

	// Reset sequences to start from 1 again
	log.Debug().Msg("Resetting sequences")
	sequences := []struct {
		name  string
		table string
	}{
		{"domains_id_seq", "domains"},
		{"pages_id_seq", "pages"},
	}

	for _, seq := range sequences {
		_, err := db.client.Exec(fmt.Sprintf(`ALTER SEQUENCE %s RESTART WITH 1`, seq.name))
		if err != nil {
			log.Warn().Err(err).Str("sequence", seq.name).Msg("Failed to reset sequence (may not exist)")
			// Don't return error for sequences, as they may not exist
		} else {
			log.Debug().Str("sequence", seq.name).Msg("Reset sequence to 1")
		}
	}

	// Step 3: Clear migration history to trigger Supabase to reapply all migrations via GitHub integration
	log.Warn().Msg("Clearing migration history - Supabase will reapply all migrations via GitHub integration")
	_, err := db.client.Exec(`DELETE FROM supabase_migrations.schema_migrations`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to clear migration history")
		return fmt.Errorf("failed to clear migration history: %w", err)
	}

	log.Info().Msg("Successfully reset database - migrations will be reapplied automatically by Supabase GitHub integration")
	return nil
}

// RecalculateJobStats recalculates all statistics for a job based on actual task records
func (db *DB) RecalculateJobStats(ctx context.Context, jobID string) error {
	_, err := db.client.ExecContext(ctx, `SELECT recalculate_job_stats($1)`, jobID)
	if err != nil {
		return fmt.Errorf("failed to recalculate job stats: %w", err)
	}
	return nil
}

// Serialise converts data to JSON string representation.
// It is named with British English spelling for consistency.
func Serialise(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		log.Error().Err(err).Msg("Failed to serialise data")
		return "{}"
	}
	return string(data)
}

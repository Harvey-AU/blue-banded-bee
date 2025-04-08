package db

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

var (
	instance *DB
	once     sync.Once
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
	ID           int64     `json:"id"`           // Unique identifier
	URL          string    `json:"url"`          // Crawled URL
	ResponseTime int64     `json:"response_time_ms"` // Response time in milliseconds
	StatusCode   int       `json:"status_code"`  // HTTP status code
	Error        string    `json:"error,omitempty"` // Error message if any
	CacheStatus  string    `json:"cache_status,omitempty"` // Cache status
	CreatedAt    time.Time `json:"created_at"`   // Timestamp of the crawl
}

// GetInstance returns a singleton instance of DB
func GetInstance(config *Config) (*DB, error) {
	var err error
	once.Do(func() {
		instance, err = New(config)
	})
	return instance, err
}

// New creates a new database connection with the given configuration
// It sets up connection pooling and initializes the schema
func New(config *Config) (*DB, error) {
	client, err := sql.Open("libsql", config.URL+"?authToken="+config.AuthToken)
	if err != nil {
		return nil, err
	}

	// Reduced connection pool size
	client.SetMaxOpenConns(10)
	client.SetMaxIdleConns(5)
	client.SetConnMaxLifetime(5 * time.Minute)

	if err := client.Ping(); err != nil {
		return nil, err
	}

	if err := setupSchema(client); err != nil {
		return nil, err
	}

	return &DB{client: client}, nil
}

func setupSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS crawl_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL,
			response_time INTEGER NOT NULL,
			status_code INTEGER,
			error TEXT,
			cache_status TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// StoreCrawlResult stores a new crawl result in the database
// It includes response time, status code, and cache status information
func (db *DB) StoreCrawlResult(ctx context.Context, result *CrawlResult) error {
	span := sentry.StartSpan(ctx, "db.store_result")
	defer span.Finish()

	span.SetTag("db.operation", "insert")
	span.SetTag("db.table", "crawl_results")

	_, err := db.client.ExecContext(ctx, `
		INSERT INTO crawl_results (url, response_time, status_code, error, cache_status)
		VALUES (?, ?, ?, ?, ?)
	`, result.URL, result.ResponseTime, result.StatusCode, result.Error, result.CacheStatus)

	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		sentry.CaptureException(err)
		return fmt.Errorf("failed to store crawl result: %w", err)
	}

	return nil
}

// GetRecentResults retrieves the most recent crawl results
// The limit parameter controls how many results to return
func (db *DB) GetRecentResults(ctx context.Context, limit int) ([]CrawlResult, error) {
	span := sentry.StartSpan(ctx, "db.get_recent")
	defer span.Finish()

	span.SetTag("db.operation", "select")
	span.SetTag("db.table", "crawl_results")
	span.SetData("db.limit", limit)

	rows, err := db.client.QueryContext(ctx, `
		SELECT id, url, response_time, status_code, error, cache_status, created_at
		FROM crawl_results
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)

	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		sentry.CaptureException(err)
		return nil, fmt.Errorf("failed to get recent results: %w", err)
	}
	defer rows.Close()

	var results []CrawlResult
	for rows.Next() {
		var r CrawlResult
		err := rows.Scan(&r.ID, &r.URL, &r.ResponseTime, &r.StatusCode, &r.Error, &r.CacheStatus, &r.CreatedAt)
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
	_, err := db.client.Exec(`DROP TABLE IF EXISTS crawl_results`)
	if err != nil {
		return err
	}
	return setupSchema(db.client)
}

package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog/log"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

type DB struct {
	client *sql.DB
}

type Config struct {
	URL       string
	AuthToken string
}

type CrawlResult struct {
	ID           int64     `json:"id"`
	URL          string    `json:"url"`
	ResponseTime int64     `json:"response_time_ms"`
	StatusCode   int       `json:"status_code"`
	Error        string    `json:"error,omitempty"`
	CacheStatus  string    `json:"cache_status,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func New(config *Config) (*DB, error) {
	client, err := sql.Open("libsql", config.URL+"?authToken="+config.AuthToken)
	if err != nil {
		return nil, err
	}

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
func (db *DB) StoreCrawlResult(ctx context.Context, result *CrawlResult) error {
	_, err := db.client.ExecContext(ctx, `
		INSERT INTO crawl_results (url, response_time, status_code, error, cache_status)
		VALUES (?, ?, ?, ?, ?)
	`, result.URL, result.ResponseTime, result.StatusCode, result.Error, result.CacheStatus)

	if err != nil {
		sentry.CaptureException(err)
		log.Error().Err(err).Msg("Failed to store crawl result")
		return fmt.Errorf("failed to store crawl result: %w", err)
	}

	return nil
}

// GetRecentResults retrieves the most recent crawl results
func (db *DB) GetRecentResults(ctx context.Context, limit int) ([]CrawlResult, error) {
	rows, err := db.client.QueryContext(ctx, `
		SELECT id, url, response_time, status_code, error, cache_status, created_at
		FROM crawl_results
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)

	if err != nil {
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

	return results, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.client.Close()
}

// TestConnection tests the database connection by inserting and querying a test record
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

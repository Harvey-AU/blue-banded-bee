package db

import (
	"database/sql"

	"github.com/tursodatabase/libsql-client-go"
)

type DB struct {
	client *sql.DB
}

type Config struct {
	URL      string
	AuthToken string
}

func New(config *Config) (*DB, error) {
	client, err := libsql.Open(config.URL, libsql.WithAuthToken(config.AuthToken))
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
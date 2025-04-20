package postgres

import (
	"context"
	"time"
)

// HealthCheck contains database health information
type HealthCheck struct {
	Connected   bool          `json:"connected"`
	Latency     time.Duration `json:"latency_ms"`
	Tables      []string      `json:"tables,omitempty"`
	TablesCount int           `json:"tables_count"`
	Error       string        `json:"error,omitempty"`
}

// CheckHealth tests the database connection and returns health information
func (db *DB) CheckHealth(ctx context.Context) HealthCheck {
	result := HealthCheck{
		Connected: false,
	}

	// Test 1: Basic connectivity with timing
	startTime := time.Now()
	err := db.client.PingContext(ctx)
	result.Latency = time.Since(startTime)

	if err != nil {
		result.Error = err.Error()
		return result
	}

	// Connection successful
	result.Connected = true

	// Test 2: List tables to verify schema
	rows, err := db.client.QueryContext(ctx, `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = 'public'
	`)
	if err != nil {
		result.Error = "Connected but failed to list tables: " + err.Error()
		return result
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}
		tables = append(tables, tableName)
	}

	result.Tables = tables
	result.TablesCount = len(tables)

	return result
}

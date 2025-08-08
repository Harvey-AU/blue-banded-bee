//go:build integration

package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckHealth_Integration(t *testing.T) {
	// Skip if no DATABASE_URL is set
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	
	// Connect to database using DATABASE_URL
	config := &Config{
		DatabaseURL: databaseURL,
	}
	db, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, db)
	defer db.Close()

	// Run health check
	health := db.CheckHealth(ctx)
	
	// Verify results
	assert.True(t, health.Connected, "Database should be connected")
	assert.Greater(t, health.Latency, time.Duration(0), "Latency should be positive")
	assert.Empty(t, health.Error, "Should have no error")
	
	// Should have some tables
	assert.NotEmpty(t, health.Tables, "Should have tables")
	assert.Greater(t, health.TablesCount, 0, "Should have table count")
	
	// Check for expected tables
	expectedTables := []string{"users", "jobs", "tasks", "pages"}
	for _, expected := range expectedTables {
		found := false
		for _, table := range health.Tables {
			if table == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected table %s to exist", expected)
	}
}

func TestCheckHealth_WithTimeout(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	// Create a very short timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	
	config := &Config{
		DatabaseURL: databaseURL,
	}
	db, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, db)
	defer db.Close()

	// Run health check with timeout
	health := db.CheckHealth(ctx)
	
	// Should fail due to timeout
	assert.False(t, health.Connected, "Should not be connected with timeout")
	assert.NotEmpty(t, health.Error, "Should have timeout error")
}

func TestCheckHealth_BadConnection(t *testing.T) {
	// Test with invalid database URL
	config := &Config{
		DatabaseURL: "postgres://invalid:invalid@nonexistent:5432/invalid",
	}
	db, err := New(config)
	if err != nil {
		// If connection fails immediately, that's fine
		t.Log("Connection failed as expected:", err)
		return
	}
	defer db.Close()

	ctx := context.Background()
	health := db.CheckHealth(ctx)
	
	// Should not be connected
	assert.False(t, health.Connected, "Should not be connected with invalid URL")
	assert.NotEmpty(t, health.Error, "Should have connection error")
}
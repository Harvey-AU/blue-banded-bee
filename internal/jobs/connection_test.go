//go:build integration

package jobs

import (
	"context"
	"os"
	"testing"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseConnection(t *testing.T) {
	// Load test environment
	testutil.LoadTestEnv(t)

	// Skip if no DATABASE_URL is set
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	// This test verifies that the test database is properly configured
	t.Logf("Attempting to connect with DATABASE_URL: %s", databaseURL)

	database, err := db.InitFromEnv()
	if err != nil {
		t.Logf("Connection error details: %v", err)
		t.Logf("Make sure TEST_DATABASE_URL in .env.test has the correct branch-specific password")
		t.Logf("You may need to reset the password in Supabase dashboard for the test branch")
	}
	require.NoError(t, err, "Failed to connect to test database")
	defer database.Close()

	// Verify we can query the database
	ctx := context.Background()
	sqlDB := database.GetDB()

	var result int
	err = sqlDB.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	require.NoError(t, err, "Failed to execute test query")
	assert.Equal(t, 1, result, "Test query should return 1")

	// Verify schema exists by checking for a known table
	var tableExists bool
	err = sqlDB.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'domains'
		)
	`).Scan(&tableExists)
	require.NoError(t, err, "Failed to check for domains table")
	assert.True(t, tableExists, "The domains table should exist in the test database")

	t.Log("✓ Test database connection successful")
	t.Log("✓ Database schema is properly initialized")
}

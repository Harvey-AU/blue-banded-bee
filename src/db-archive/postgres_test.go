package postgres

import (
	"os"
	"testing"
)

func TestPostgresConnection(t *testing.T) {
	// Skip test if not in test environment
	if os.Getenv("TEST_POSTGRES") != "true" {
		t.Skip("Skipping PostgreSQL test; TEST_POSTGRES not set to true")
	}

	// Get connection string from environment
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		t.Fatal("DATABASE_URL environment variable not set")
	}

	config := &Config{
		ConnectionString: connString,
	}

	db, err := New(config)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Verify connection works
	if err := db.GetDB().Ping(); err != nil {
		t.Fatalf("Failed to ping PostgreSQL: %v", err)
	}

	t.Log("Successfully connected to PostgreSQL")
}

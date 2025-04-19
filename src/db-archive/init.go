package postgres

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
)

// InitFromEnv initializes a PostgreSQL DB from environment variables
func InitFromEnv() (*DB, error) {
	// Check for Fly.io PostgreSQL env vars
	connectionString := os.Getenv("DATABASE_URL")

	// If not found, try to build from individual components
	if connectionString == "" {
		host := os.Getenv("PGHOST")
		port := os.Getenv("PGPORT")
		if port == "" {
			port = "5432" // Default PostgreSQL port
		}
		dbname := os.Getenv("PGDATABASE")
		user := os.Getenv("PGUSER")
		password := os.Getenv("PGPASSWORD")
		sslmode := os.Getenv("PGSSLMODE")
		if sslmode == "" {
			sslmode = "require" // Default to require SSL in production
		}

		// Check if we have all required variables
		if host == "" || dbname == "" || user == "" || password == "" {
			return nil, fmt.Errorf("missing PostgreSQL environment variables")
		}

		// Build connection string
		connectionString = fmt.Sprintf(
			"host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
			host, port, dbname, user, password, sslmode,
		)
	}

	if connectionString == "" {
		return nil, fmt.Errorf("unable to determine PostgreSQL connection string")
	}

	log.Info().Msg("Initializing PostgreSQL connection")

	config := &Config{
		ConnectionString: connectionString,
	}

	return New(config)
}

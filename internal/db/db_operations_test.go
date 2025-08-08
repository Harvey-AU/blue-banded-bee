package db

import (
	"testing"
	"strings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectionString tests the DSN building logic
func TestConnectionString(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectInDSN []string
		description string
	}{
		{
			name: "database_url_takes_precedence",
			config: &Config{
				DatabaseURL: "postgresql://user:pass@host:5432/db?sslmode=require",
				Host:        "ignored",
				Port:        "ignored",
				User:        "ignored",
				Password:    "ignored",
				Database:    "ignored",
			},
			expectInDSN: []string{"postgresql://user:pass@host:5432/db"},
			description: "DatabaseURL should take precedence over individual fields",
		},
		{
			name: "individual_fields_build_dsn",
			config: &Config{
				Host:     "localhost",
				Port:     "5432",
				User:     "testuser",
				Password: "testpass",
				Database: "testdb",
				SSLMode:  "disable",
			},
			expectInDSN: []string{
				"host=localhost",
				"port=5432",
				"user=testuser",
				"password=testpass",
				"dbname=testdb",
				"sslmode=disable",
			},
			description: "Individual fields should build correct DSN",
		},
		{
			name: "default_sslmode_when_missing",
			config: &Config{
				Host:     "localhost",
				Port:     "5432",
				User:     "testuser",
				Password: "testpass",
				Database: "testdb",
				// SSLMode not set
			},
			expectInDSN: []string{
				"sslmode=require", // Default should be require
			},
			description: "Default SSLMode should be 'require' when not specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := tt.config.ConnectionString()
			for _, expected := range tt.expectInDSN {
				assert.Contains(t, dsn, expected, tt.description)
			}
		})
	}
}

// TestStatementTimeoutInjection tests that statement_timeout is properly added
func TestStatementTimeoutInjection(t *testing.T) {
	t.Skip("TODO: Implement test for statement_timeout injection in DSN")
	// This should test the actual New() function to verify it adds statement_timeout
	// But we need to refactor New() to be more testable first (dependency injection)
}

// TestConfigValidation tests configuration validation
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid_database_url",
			config: &Config{
				DatabaseURL: "postgresql://user:pass@localhost:5432/db",
			},
			expectError: false,
		},
		{
			name: "valid_individual_fields",
			config: &Config{
				Host:     "localhost",
				Port:     "5432",
				User:     "user",
				Password: "pass",
				Database: "db",
			},
			expectError: false,
		},
		{
			name:        "empty_config",
			config:      &Config{},
			expectError: true,
			errorMsg:    "database configuration required",
		},
		{
			name: "missing_required_field",
			config: &Config{
				Host: "localhost",
				Port: "5432",
				// Missing User, Password, Database
			},
			expectError: true,
			errorMsg:    "incomplete database configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestPoolConfiguration tests connection pool configuration
func TestPoolConfiguration(t *testing.T) {
	t.Skip("TODO: Implement test for connection pool configuration")
	// This should test MaxConns, MinConns, MaxConnLifetime, etc.
	// Requires refactoring New() to expose pool config or use dependency injection
}

// TestHealthCheckWithTimeout tests health check with timeout
func TestHealthCheckWithTimeout(t *testing.T) {
	t.Skip("TODO: Implement test for health check with timeout using sqlmock")
	// This requires sqlmock to be set up properly
	// Should test both successful ping and timeout scenarios
}

// TestTransactionRollback tests transaction rollback on error
func TestTransactionRollback(t *testing.T) {
	t.Skip("TODO: Implement test for transaction rollback using sqlmock")
	// This requires proper mock setup to test transaction behavior
}

// TestPreparedStatementCaching tests prepared statement caching
func TestPreparedStatementCaching(t *testing.T) {
	t.Skip("TODO: Implement test for prepared statement caching")
	// This would need to verify that prepared statements are cached and reused
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
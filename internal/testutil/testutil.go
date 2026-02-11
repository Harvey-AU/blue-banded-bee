package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
)

// LoadTestEnv loads the .env.test file and sets DATABASE_URL from TEST_DATABASE_URL
func LoadTestEnv(t *testing.T) {
	t.Helper()

	// If DATABASE_URL is already set and not empty (e.g., in CI), use it
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		t.Log("DATABASE_URL already set in environment")
		return
	}

	// Find .env.test file (might be in parent directories during test runs)
	envPath := findEnvTestFile()
	if envPath == "" {
		t.Log("Warning: .env.test file not found, using environment variables as-is")
		return
	}

	// Load .env.test
	envMap, err := godotenv.Read(envPath)
	if err != nil {
		t.Logf("Warning: Failed to read %s: %v", envPath, err)
		return
	}

	// If TEST_DATABASE_URL exists, set it as DATABASE_URL
	if testDBURL, exists := envMap["TEST_DATABASE_URL"]; exists {
		os.Setenv("DATABASE_URL", testDBURL)
		t.Log("DATABASE_URL set from TEST_DATABASE_URL in .env.test")
	}
}

// findEnvTestFile searches for .env.test in current and parent directories
func findEnvTestFile() string {
	// Start from current directory
	dir, _ := os.Getwd()

	// Search up to 5 levels up
	for range 5 {
		envPath := filepath.Join(dir, ".env.test")
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached root
		}
		dir = parent
	}

	return ""
}

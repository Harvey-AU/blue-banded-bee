package auth

import (
	"fmt"
	"os"
)

// Config holds Supabase authentication configuration
type Config struct {
	AuthURL        string
	PublishableKey string
}

// NewConfigFromEnv creates auth config from environment variables
func NewConfigFromEnv() (*Config, error) {
	config := &Config{
		AuthURL:        os.Getenv("SUPABASE_AUTH_URL"),
		PublishableKey: os.Getenv("SUPABASE_PUBLISHABLE_KEY"),
	}

	// Validate required environment variables
	if config.AuthURL == "" {
		return nil, fmt.Errorf("SUPABASE_AUTH_URL environment variable is required")
	}
	if config.PublishableKey == "" {
		return nil, fmt.Errorf("SUPABASE_PUBLISHABLE_KEY environment variable is required")
	}
	return config, nil
}

// Validate ensures all required configuration is present
func (c *Config) Validate() error {
	if c.AuthURL == "" {
		return fmt.Errorf("AuthURL is required")
	}
	if c.PublishableKey == "" {
		return fmt.Errorf("PublishableKey is required")
	}
	return nil
}

package auth

import (
	"fmt"
	"os"
)

// Config holds Supabase authentication configuration
type Config struct {
	SupabaseURL     string
	SupabaseAnonKey string
	JWTSecret       string
}

// NewConfigFromEnv creates auth config from environment variables
func NewConfigFromEnv() (*Config, error) {
	config := &Config{
		SupabaseURL:     os.Getenv("SUPABASE_URL"),
		SupabaseAnonKey: os.Getenv("SUPABASE_ANON_KEY"),
		JWTSecret:       os.Getenv("SUPABASE_JWT_SECRET"),
	}

	// Validate required environment variables
	if config.SupabaseURL == "" {
		return nil, fmt.Errorf("SUPABASE_URL environment variable is required")
	}
	if config.SupabaseAnonKey == "" {
		return nil, fmt.Errorf("SUPABASE_ANON_KEY environment variable is required")
	}
	if config.JWTSecret == "" {
		return nil, fmt.Errorf("SUPABASE_JWT_SECRET environment variable is required")
	}

	return config, nil
}

// Validate ensures all required configuration is present
func (c *Config) Validate() error {
	if c.SupabaseURL == "" {
		return fmt.Errorf("SupabaseURL is required")
	}
	if c.SupabaseAnonKey == "" {
		return fmt.Errorf("SupabaseAnonKey is required")
	}
	if c.JWTSecret == "" {
		return fmt.Errorf("JWTSecret is required")
	}
	return nil
}

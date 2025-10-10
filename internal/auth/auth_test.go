package auth

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackage(t *testing.T) {
	// Placeholder test for auth package
	t.Log("Auth package loaded")
}

func TestNewConfigFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
		errMsg  string
	}{
		{
			name: "all_env_vars_set",
			envVars: map[string]string{
				"SUPABASE_URL":        "https://test.supabase.co",
				"SUPABASE_ANON_KEY":   "test-anon-key",
				"SUPABASE_JWT_SECRET": "test-jwt-secret",
			},
			wantErr: false,
		},
		{
			name: "missing_url",
			envVars: map[string]string{
				"SUPABASE_ANON_KEY":   "test-anon-key",
				"SUPABASE_JWT_SECRET": "test-jwt-secret",
			},
			wantErr: true,
			errMsg:  "SUPABASE_URL environment variable is required",
		},
		{
			name: "missing_anon_key",
			envVars: map[string]string{
				"SUPABASE_URL":        "https://test.supabase.co",
				"SUPABASE_JWT_SECRET": "test-jwt-secret",
			},
			wantErr: true,
			errMsg:  "SUPABASE_ANON_KEY environment variable is required",
		},
		{
			name: "missing_jwt_secret",
			envVars: map[string]string{
				"SUPABASE_URL":      "https://test.supabase.co",
				"SUPABASE_ANON_KEY": "test-anon-key",
			},
			wantErr: true,
			errMsg:  "SUPABASE_JWT_SECRET environment variable is required",
		},
		{
			name:    "all_missing",
			envVars: map[string]string{},
			wantErr: true,
			errMsg:  "SUPABASE_URL environment variable is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Test NewConfigFromEnv
			config, err := NewConfigFromEnv()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, config)
				if tt.errMsg != "" {
					assert.EqualError(t, err, tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, config)
				assert.Equal(t, tt.envVars["SUPABASE_URL"], config.SupabaseURL)
				assert.Equal(t, tt.envVars["SUPABASE_ANON_KEY"], config.SupabaseAnonKey)
				assert.Equal(t, tt.envVars["SUPABASE_JWT_SECRET"], config.JWTSecret)
			}

			// Clean up
			os.Clearenv()
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_config",
			config: &Config{
				SupabaseURL:     "https://test.supabase.co",
				SupabaseAnonKey: "test-anon-key",
				JWTSecret:       "test-jwt-secret",
			},
			wantErr: false,
		},
		{
			name: "missing_url",
			config: &Config{
				SupabaseAnonKey: "test-anon-key",
				JWTSecret:       "test-jwt-secret",
			},
			wantErr: true,
			errMsg:  "SupabaseURL is required",
		},
		{
			name: "missing_anon_key",
			config: &Config{
				SupabaseURL: "https://test.supabase.co",
				JWTSecret:   "test-jwt-secret",
			},
			wantErr: true,
			errMsg:  "SupabaseAnonKey is required",
		},
		{
			name: "missing_jwt_secret",
			config: &Config{
				SupabaseURL:     "https://test.supabase.co",
				SupabaseAnonKey: "test-anon-key",
			},
			wantErr: true,
			errMsg:  "JWTSecret is required",
		},
		{
			name:    "empty_config",
			config:  &Config{},
			wantErr: true,
			errMsg:  "SupabaseURL is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.EqualError(t, err, tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigFields(t *testing.T) {
	config := &Config{
		SupabaseURL:     "https://test.supabase.co",
		SupabaseAnonKey: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test",
		JWTSecret:       "super-secret-jwt-key-with-at-least-32-characters",
	}

	// Test that fields are accessible
	assert.Equal(t, "https://test.supabase.co", config.SupabaseURL)
	assert.Equal(t, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test", config.SupabaseAnonKey)
	assert.Equal(t, "super-secret-jwt-key-with-at-least-32-characters", config.JWTSecret)

	// Test validation passes
	err := config.Validate()
	assert.NoError(t, err)
}

func BenchmarkNewConfigFromEnv(b *testing.B) {
	// Set up environment
	os.Setenv("SUPABASE_URL", "https://test.supabase.co")
	os.Setenv("SUPABASE_ANON_KEY", "test-anon-key")
	os.Setenv("SUPABASE_JWT_SECRET", "test-jwt-secret")
	defer os.Clearenv()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewConfigFromEnv()
	}
}

func BenchmarkConfig_Validate(b *testing.B) {
	config := &Config{
		SupabaseURL:     "https://test.supabase.co",
		SupabaseAnonKey: "test-anon-key",
		JWTSecret:       "test-jwt-secret",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Validate()
	}
}

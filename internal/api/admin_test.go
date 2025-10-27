package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/Harvey-AU/blue-banded-bee/internal/mocks"
	"github.com/stretchr/testify/assert"
)

func TestHasSystemAdminRole(t *testing.T) {
	tests := []struct {
		name     string
		claims   *auth.UserClaims
		expected bool
	}{
		{
			name: "user with correct system_role should return true",
			claims: &auth.UserClaims{
				UserID: "test-user-1",
				Email:  "admin@example.com",
				AppMetadata: map[string]interface{}{
					"system_role": "system_admin",
				},
			},
			expected: true,
		},
		{
			name: "user with different role should return false",
			claims: &auth.UserClaims{
				UserID: "test-user-2",
				Email:  "user@example.com",
				AppMetadata: map[string]interface{}{
					"system_role": "admin",
				},
			},
			expected: false,
		},
		{
			name: "user with owner role should return false",
			claims: &auth.UserClaims{
				UserID: "test-user-3",
				Email:  "owner@example.com",
				AppMetadata: map[string]interface{}{
					"system_role": "owner",
				},
			},
			expected: false,
		},
		{
			name: "user with nil app_metadata should return false",
			claims: &auth.UserClaims{
				UserID:      "test-user-4",
				Email:       "user@example.com",
				AppMetadata: nil,
			},
			expected: false,
		},
		{
			name: "user with empty app_metadata should return false",
			claims: &auth.UserClaims{
				UserID:      "test-user-5",
				Email:       "user@example.com",
				AppMetadata: map[string]interface{}{},
			},
			expected: false,
		},
		{
			name: "user with system_role set to wrong value should return false",
			claims: &auth.UserClaims{
				UserID: "test-user-6",
				Email:  "user@example.com",
				AppMetadata: map[string]interface{}{
					"system_role": "wrong_value",
				},
			},
			expected: false,
		},
		{
			name: "user with system_role as integer should return false",
			claims: &auth.UserClaims{
				UserID: "test-user-7",
				Email:  "user@example.com",
				AppMetadata: map[string]interface{}{
					"system_role": 123,
				},
			},
			expected: false,
		},
		{
			name: "user with system_role as boolean should return false",
			claims: &auth.UserClaims{
				UserID: "test-user-8",
				Email:  "user@example.com",
				AppMetadata: map[string]interface{}{
					"system_role": true,
				},
			},
			expected: false,
		},
		{
			name: "user with system_role as float should return false",
			claims: &auth.UserClaims{
				UserID: "test-user-9",
				Email:  "user@example.com",
				AppMetadata: map[string]interface{}{
					"system_role": 12.5,
				},
			},
			expected: false,
		},
		{
			name: "user with system_role as array should return false",
			claims: &auth.UserClaims{
				UserID: "test-user-10",
				Email:  "user@example.com",
				AppMetadata: map[string]interface{}{
					"system_role": []string{"system_admin"},
				},
			},
			expected: false,
		},
		{
			name: "user with system_role as map should return false",
			claims: &auth.UserClaims{
				UserID: "test-user-11",
				Email:  "user@example.com",
				AppMetadata: map[string]interface{}{
					"system_role": map[string]string{"role": "system_admin"},
				},
			},
			expected: false,
		},
		{
			name: "user with other metadata but no system_role should return false",
			claims: &auth.UserClaims{
				UserID: "test-user-12",
				Email:  "user@example.com",
				AppMetadata: map[string]interface{}{
					"organization": "example.com",
					"tier":         "premium",
					"admin":        true,
				},
			},
			expected: false,
		},
		{
			name: "user with system_role key but nil value should return false",
			claims: &auth.UserClaims{
				UserID: "test-user-13",
				Email:  "user@example.com",
				AppMetadata: map[string]interface{}{
					"system_role": nil,
				},
			},
			expected: false,
		},
		{
			name: "user with empty string system_role should return false",
			claims: &auth.UserClaims{
				UserID: "test-user-14",
				Email:  "user@example.com",
				AppMetadata: map[string]interface{}{
					"system_role": "",
				},
			},
			expected: false,
		},
		{
			name: "user with system_role containing extra whitespace should return false",
			claims: &auth.UserClaims{
				UserID: "test-user-15",
				Email:  "user@example.com",
				AppMetadata: map[string]interface{}{
					"system_role": " system_admin ",
				},
			},
			expected: false,
		},
		{
			name: "user with case-insensitive system_admin should return false",
			claims: &auth.UserClaims{
				UserID: "test-user-16",
				Email:  "user@example.com",
				AppMetadata: map[string]interface{}{
					"system_role": "SYSTEM_ADMIN",
				},
			},
			expected: false,
		},
		{
			name: "user with mixed case system_admin should return false",
			claims: &auth.UserClaims{
				UserID: "test-user-17",
				Email:  "user@example.com",
				AppMetadata: map[string]interface{}{
					"system_role": "System_Admin",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasSystemAdminRole(tt.claims)
			assert.Equal(t, tt.expected, result, "hasSystemAdminRole() result should match expected value")
		})
	}
}

func TestHasSystemAdminRoleWithNilClaims(t *testing.T) {
	// Test with nil claims - should return false
	result := hasSystemAdminRole(nil)
	assert.False(t, result, "hasSystemAdminRole() should return false when given nil claims")
}

func TestAdminResetDatabase(t *testing.T) {
	tests := []struct {
		name           string
		allowReset     bool
		setupMock      func(*mocks.MockDB)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "database reset disabled",
			allowReset:     false,
			setupMock:      func(m *mocks.MockDB) {},
			expectedStatus: http.StatusForbidden,
			expectedBody:   "Database reset not enabled",
		},
		{
			name:       "no authentication returns unauthorized",
			allowReset: true,
			setupMock: func(m *mocks.MockDB) {
				// ResetSchema won't be called without auth
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Authentication required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env vars
			originalReset := os.Getenv("ALLOW_DB_RESET")
			defer func() {
				os.Setenv("ALLOW_DB_RESET", originalReset)
			}()

			// Set environment variables for test
			if tt.allowReset {
				os.Setenv("ALLOW_DB_RESET", "true")
			} else {
				os.Setenv("ALLOW_DB_RESET", "false")
			}

			// Create mock database
			mockDB := new(mocks.MockDB)
			tt.setupMock(mockDB)

			// Create handler with mock
			h := &Handler{
				DB: mockDB,
			}

			// Create request and recorder
			req := httptest.NewRequest(http.MethodPost, "/admin/reset-db", nil)
			rec := httptest.NewRecorder()

			// Execute
			h.AdminResetDatabase(rec, req)

			// Assert status code
			assert.Equal(t, tt.expectedStatus, rec.Code)

			// Assert response body contains expected text
			assert.Contains(t, rec.Body.String(), tt.expectedBody)

			// Verify mock expectations
			mockDB.AssertExpectations(t)
		})
	}
}

func TestHasSystemAdminRoleEdgeCases(t *testing.T) {
	t.Run("multiple system_admin values in metadata", func(t *testing.T) {
		// Test with valid system_role but other confusing metadata
		claims := &auth.UserClaims{
			UserID: "test-user",
			Email:  "admin@example.com",
			AppMetadata: map[string]interface{}{
				"system_role":    "system_admin",
				"role":           "user",           // different role field
				"admin":          false,            // contradictory admin field
				"permissions":    []string{"read"}, // other permission structure
				"system_admin":   false,            // similar but different key
				"SystemRole":     "system_admin",   // different case key
				"system_role_v2": "system_admin",   // similar key
			},
		}
		result := hasSystemAdminRole(claims)
		assert.True(t, result, "should return true when system_role is correctly set to system_admin")
	})

	t.Run("unicode and special characters in role", func(t *testing.T) {
		testCases := []struct {
			role     interface{}
			expected bool
		}{
			{"system_admin", true},        // correct value
			{"system_admin🔑", false},      // with emoji
			{"system_admin\n", false},     // with newline
			{"system_admin\t", false},     // with tab
			{"system_admin\x00", false},   // with null character
			{"系统管理员", false},              // chinese characters
			{"système_admin", false},      // french characters
			{"system‑admin", false},       // en dash instead of underscore
			{"system—admin", false},       // em dash instead of underscore
			{"system_admin\u00ad", false}, // soft hyphen at end
		}

		for _, tc := range testCases {
			claims := &auth.UserClaims{
				UserID: "test-user",
				Email:  "admin@example.com",
				AppMetadata: map[string]interface{}{
					"system_role": tc.role,
				},
			}
			result := hasSystemAdminRole(claims)
			assert.Equal(t, tc.expected, result, "role value %v should return %v", tc.role, tc.expected)
		}
	})
}

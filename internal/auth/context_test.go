package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetUserFromContext tests the utility function for extracting user from context
func TestGetUserFromContext(t *testing.T) {
	tests := []struct {
		name           string
		setupContext   func() context.Context
		expectedUser   *UserClaims
		expectedExists bool
	}{
		{
			name: "user_present_in_context",
			setupContext: func() context.Context {
				user := &UserClaims{
					UserID: "test-user",
					Email:  "test@example.com",
				}
				return context.WithValue(context.Background(), UserKey, user)
			},
			expectedUser: &UserClaims{
				UserID: "test-user",
				Email:  "test@example.com",
			},
			expectedExists: true,
		},
		{
			name: "user_not_in_context",
			setupContext: func() context.Context {
				return context.Background()
			},
			expectedUser:   nil,
			expectedExists: false,
		},
		{
			name: "wrong_type_in_context",
			setupContext: func() context.Context {
				return context.WithValue(context.Background(), UserKey, "not-a-user")
			},
			expectedUser:   nil,
			expectedExists: false,
		},
		{
			name: "nil_user_in_context",
			setupContext: func() context.Context {
				return context.WithValue(context.Background(), UserKey, (*UserClaims)(nil))
			},
			expectedUser:   nil,
			expectedExists: true, // Function returns true but user is nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupContext()

			user, exists := GetUserFromContext(ctx)

			assert.Equal(t, tt.expectedExists, exists)
			if tt.expectedExists && tt.expectedUser != nil {
				require.NotNil(t, user)
				assert.Equal(t, tt.expectedUser.UserID, user.UserID)
				assert.Equal(t, tt.expectedUser.Email, user.Email)
			} else if !tt.expectedExists {
				assert.Nil(t, user)
			}
			// For nil_user_in_context case: exists=true but user may be nil (edge case)
		})
	}
}

// TestUserKey ensures the context key is properly defined
func TestUserKey(t *testing.T) {
	// Verify UserKey is not nil and has expected type
	assert.NotNil(t, UserKey)
	assert.IsType(t, UserContextKey(""), UserKey)
	assert.Equal(t, UserContextKey("user"), UserKey)
}

// Benchmark the context utility function
func BenchmarkGetUserFromContext(b *testing.B) {
	user := &UserClaims{
		UserID: "bench-user",
		Email:  "bench@example.com",
	}
	ctx := context.WithValue(context.Background(), UserKey, user)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetUserFromContext(ctx)
	}
}

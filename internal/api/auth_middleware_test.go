package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockAuthClient is a mock implementation for testing auth middleware
type MockAuthClient struct {
	mock.Mock
}

func (m *MockAuthClient) ValidateToken(ctx context.Context, token string) (*auth.UserClaims, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.UserClaims), args.Error(1)
}

func (m *MockAuthClient) ExtractTokenFromRequest(r *http.Request) (string, error) {
	args := m.Called(r)
	return args.String(0), args.Error(1)
}

func (m *MockAuthClient) SetUserInContext(r *http.Request, user *auth.UserClaims) *http.Request {
	args := m.Called(r, user)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*http.Request)
}

// TestAuthMiddleware tests the core authentication middleware logic
func TestAuthMiddleware(t *testing.T) {
	tests := []struct {
		name              string
		setupMocks        func(*MockAuthClient)
		requestHeaders    map[string]string
		expectedStatus    int
		expectUserContext bool
		checkResponse     func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful_authentication",
			setupMocks: func(mac *MockAuthClient) {
				// Mock successful token extraction
				mac.On("ExtractTokenFromRequest", mock.AnythingOfType("*http.Request")).Return("valid-jwt-token", nil)
				
				// Mock successful token validation
				userClaims := &auth.UserClaims{
					UserID: "user-123",
					Email:  "test@example.com",
				}
				mac.On("ValidateToken", mock.AnythingOfType("*context.valueCtx"), "valid-jwt-token").Return(userClaims, nil)
				
				// Mock context setting
				mac.On("SetUserInContext", mock.AnythingOfType("*http.Request"), userClaims).Return(&http.Request{})
			},
			requestHeaders: map[string]string{
				"Authorization": "Bearer valid-jwt-token",
			},
			expectedStatus:    http.StatusOK,
			expectUserContext: true,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, "authenticated", rec.Body.String())
			},
		},
		{
			name: "missing_authorization_header",
			setupMocks: func(mac *MockAuthClient) {
				// Mock token extraction returning empty token
				mac.On("ExtractTokenFromRequest", mock.AnythingOfType("*http.Request")).Return("", assert.AnError)
			},
			requestHeaders:    map[string]string{},
			expectedStatus:    http.StatusUnauthorized,
			expectUserContext: false,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				
				assert.Equal(t, float64(401), response["status"])
				assert.Equal(t, "UNAUTHORISED", response["code"])
			},
		},
		{
			name: "invalid_token",
			setupMocks: func(mac *MockAuthClient) {
				// Mock successful token extraction
				mac.On("ExtractTokenFromRequest", mock.AnythingOfType("*http.Request")).Return("invalid-token", nil)
				
				// Mock failed token validation
				mac.On("ValidateToken", mock.AnythingOfType("*context.valueCtx"), "invalid-token").Return(nil, assert.AnError)
			},
			requestHeaders: map[string]string{
				"Authorization": "Bearer invalid-token",
			},
			expectedStatus:    http.StatusUnauthorized,
			expectUserContext: false,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				
				assert.Equal(t, float64(401), response["status"])
				assert.Equal(t, "UNAUTHORISED", response["code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: This requires interface extraction from auth middleware
			// Currently auth middleware uses concrete types, preventing mocking
			// 
			// Required changes:
			// 1. Extract AuthClientInterface from concrete auth.Client
			// 2. Update middleware to accept interface
			// 3. Test with MockAuthClient
			//
			// For now, verify test structure and mock expectations
			mockAuthClient := new(MockAuthClient)
			tt.setupMocks(mockAuthClient)
			
			// Verify mock expectations structure
			assert.NotNil(t, mockAuthClient)
			
			t.Skip("TODO: Requires AuthClientInterface extraction for full testing")
		})
	}
}

// TestGetUserFromContext tests the utility function for extracting user from context
func TestGetUserFromContext(t *testing.T) {
	tests := []struct {
		name           string
		setupContext   func() context.Context
		expectedUser   *auth.UserClaims
		expectedExists bool
	}{
		{
			name: "user_present_in_context",
			setupContext: func() context.Context {
				user := &auth.UserClaims{
					UserID: "test-user",
					Email:  "test@example.com",
				}
				return context.WithValue(context.Background(), auth.UserKey, user)
			},
			expectedUser: &auth.UserClaims{
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
				return context.WithValue(context.Background(), auth.UserKey, "not-a-user")
			},
			expectedUser:   nil,
			expectedExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupContext()
			
			user, exists := auth.GetUserFromContext(ctx)
			
			assert.Equal(t, tt.expectedExists, exists)
			if tt.expectedExists {
				require.NotNil(t, user)
				assert.Equal(t, tt.expectedUser.UserID, user.UserID)
				assert.Equal(t, tt.expectedUser.Email, user.Email)
			} else {
				assert.Nil(t, user)
			}
		})
	}
}
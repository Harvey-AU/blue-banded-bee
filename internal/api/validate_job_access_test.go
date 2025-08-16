package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateJobAccessAuthenticationLayer(t *testing.T) {
	// Test only the authentication context extraction logic
	// This is the part we can safely test without complex DB mocking
	
	t.Run("no_user_in_context_returns_401", func(t *testing.T) {
		handler := &Handler{DB: nil}
		req := httptest.NewRequest(http.MethodGet, "/v1/jobs/test-job/tasks", nil)
		rec := httptest.NewRecorder()
		
		result := handler.validateJobAccess(rec, req, "test-job")
		
		assert.Nil(t, result)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Body.String(), "User information not found")
	})
	
	t.Run("function_exists_and_handles_auth_context", func(t *testing.T) {
		// This test validates that our extracted function exists and
		// correctly handles the auth context extraction logic
		
		handler := &Handler{DB: nil}
		
		// Test without user context - should handle gracefully
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		
		result := handler.validateJobAccess(rec, req, "test-job")
		
		// Should return nil and write error response
		assert.Nil(t, result)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Body.String(), "User information not found")
	})
}
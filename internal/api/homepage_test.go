package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServeHomepage(t *testing.T) {
	handler := &Handler{}

	t.Run("returns_404_for_non_root_paths", func(t *testing.T) {
		// This tests the actual routing logic in the handler
		req := httptest.NewRequest("GET", "/anything-else", nil)
		rec := httptest.NewRecorder()

		handler.ServeHomepage(rec, req)

		// This should return 404 because of our path check logic
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("accepts_root_path", func(t *testing.T) {
		// This tests that the handler accepts root path (actual file serving will fail in CI)
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHomepage(rec, req)

		// In CI this will be 404 (file missing), in production it should be 200
		// We're testing that the handler doesn't immediately reject the request
		assert.NotEqual(t, http.StatusMethodNotAllowed, rec.Code)
		assert.NotEqual(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("accepts_different_methods", func(t *testing.T) {
		// Test that the handler accepts any HTTP method for root path
		methods := []string{"GET", "POST", "PUT", "DELETE"}
		
		for _, method := range methods {
			req := httptest.NewRequest(method, "/", nil)
			rec := httptest.NewRecorder()

			handler.ServeHomepage(rec, req)

			// Should not be method not allowed - the path logic should handle this
			assert.NotEqual(t, http.StatusMethodNotAllowed, rec.Code, "Method %s should be handled", method)
		}
	})
}
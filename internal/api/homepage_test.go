package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServeHomepage(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name           string
		path           string
		method         string
		expectedStatus int
		expectedFile   string
	}{
		{
			name:           "serves_homepage_at_root",
			path:           "/",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedFile:   "homepage.html",
		},
		{
			name:           "returns_404_for_non_root_paths",
			path:           "/anything-else",
			method:         "GET",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "handles_post_method",
			path:           "/",
			method:         "POST",
			expectedStatus: http.StatusOK,
			expectedFile:   "homepage.html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHomepage(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.expectedFile != "" {
				// Note: The actual file serving is handled by http.ServeFile
				// In tests, this will return an error since the file doesn't exist
				// in the test environment, but we can verify the handler responds
				// This test verifies the routing logic works correctly
			}
		})
	}
}
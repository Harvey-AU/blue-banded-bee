package api

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// contextKey is used for storing values in request context
type contextKey string

const requestIDKey contextKey = "request_id"

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if request ID already exists in headers (from load balancers, etc.)
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Add request ID to response headers
		w.Header().Set("X-Request-ID", requestID)

		// Add request ID to request context
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		r = r.WithContext(ctx)

		// Log the incoming request (skip health checks to reduce noise)
		if r.URL.Path != "/health" {
			log.Info().
				Str("request_id", requestID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Msg("Incoming request")
		}

		next.ServeHTTP(w, r)
	})
}

// GetRequestID retrieves the request ID from the request context
func GetRequestID(r *http.Request) string {
	if requestID, ok := r.Context().Value(requestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// generateRequestID creates a unique request ID
func generateRequestID() string {
	// Use timestamp + random bytes for uniqueness
	timestamp := time.Now().UnixNano()

	// Generate 4 random bytes
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		// Fallback to timestamp only if random generation fails
		return fmt.Sprintf("%x", timestamp)
	}

	return fmt.Sprintf("%x-%x", timestamp, randomBytes)
}

// LoggingMiddleware logs request details and response times
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := GetRequestID(r)

		// Wrap the ResponseWriter to capture status code
		wrapper := &responseWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrapper, r)

		duration := time.Since(start)

		// Log the completed request (skip health checks to reduce noise)
		if r.URL.Path != "/health" {
			log.Info().
				Str("request_id", requestID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", wrapper.statusCode).
				Dur("duration", duration).
				Msg("Request completed")
		}
	})
}

// responseWrapper wraps http.ResponseWriter to capture status code
type responseWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// CORSMiddleware adds CORS headers for browser requests
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// CrossOriginProtectionMiddleware provides protection against CSRF attacks.
// It is a wrapper around Go's experimental http.CrossOriginProtection.
func CrossOriginProtectionMiddleware(next http.Handler) http.Handler {
	// Using nil for the config uses the default protection.
	// The Handler method returns a handler that serves the given handler
	// after performing cross-origin request checks.
	return http.NewCrossOriginProtection().Handler(next)
}

// SecurityHeadersMiddleware adds security-related headers
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")

		// Content Security Policy
		csp := `
			default-src 'self';
			script-src 'self' 'unsafe-inline' https://challenges.cloudflare.com https://unpkg.com https://cdn.jsdelivr.net https://www.googletagmanager.com https://browser.sentry-cdn.com https://static.cloudflareinsights.com;
			style-src 'self' 'unsafe-inline';
			connect-src 'self' https://challenges.cloudflare.com https://cdn.jsdelivr.net https://www.google-analytics.com https://www.googletagmanager.com https://analytics.google.com https://auth.bluebandedbee.co https://*.sentry.io https://*.ingest.sentry.io https://browser.sentry-cdn.com https://cloudflareinsights.com;
			frame-src https://challenges.cloudflare.com;
			img-src 'self' data: https://www.google-analytics.com https://www.googletagmanager.com https://ssl.gstatic.com;
			font-src 'self' data:;
			object-src 'none';
			base-uri 'self';
			form-action 'self';
		`
		w.Header().Set("Content-Security-Policy", strings.ReplaceAll(csp, "\n", " "))

		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

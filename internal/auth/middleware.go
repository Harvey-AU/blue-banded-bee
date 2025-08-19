package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
)

// AuthClient defines the interface for authentication operations
type AuthClient interface {
	ValidateToken(ctx context.Context, token string) (*UserClaims, error)
	ExtractTokenFromRequest(r *http.Request) (string, error)
	SetUserInContext(r *http.Request, user *UserClaims) *http.Request
}

// SupabaseAuthClient implements AuthClient for Supabase authentication
type SupabaseAuthClient struct{}

// NewSupabaseAuthClient creates a new SupabaseAuthClient
func NewSupabaseAuthClient() *SupabaseAuthClient {
	return &SupabaseAuthClient{}
}

// ValidateToken validates a Supabase JWT token
func (s *SupabaseAuthClient) ValidateToken(ctx context.Context, token string) (*UserClaims, error) {
	return validateSupabaseToken(token)
}

// ExtractTokenFromRequest extracts a JWT token from the Authorization header
func (s *SupabaseAuthClient) ExtractTokenFromRequest(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return "", fmt.Errorf("missing or invalid Authorization header")
	}
	return strings.TrimPrefix(authHeader, "Bearer "), nil
}

// SetUserInContext adds user claims to the request context
func (s *SupabaseAuthClient) SetUserInContext(r *http.Request, user *UserClaims) *http.Request {
	ctx := context.WithValue(r.Context(), UserKey, user)
	return r.WithContext(ctx)
}

// UserContextKey is the key used to store user claims in the request context
type UserContextKey string

const (
	UserKey UserContextKey = "user"
)

// UserClaims represents the Supabase JWT claims
type UserClaims struct {
	jwt.RegisteredClaims
	UserID       string                 `json:"sub"`
	Email        string                 `json:"email"`
	AppMetadata  map[string]interface{} `json:"app_metadata"`
	UserMetadata map[string]interface{} `json:"user_metadata"`
	Role         string                 `json:"role"`
}

// AuthMiddleware validates Supabase JWT tokens (uses default SupabaseAuthClient)
func AuthMiddleware(next http.Handler) http.Handler {
	return AuthMiddlewareWithClient(NewSupabaseAuthClient())(next)
}

// AuthMiddlewareWithClient validates JWT tokens using the provided AuthClient
func AuthMiddlewareWithClient(authClient AuthClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract the JWT from the Authorization header
			tokenString, err := authClient.ExtractTokenFromRequest(r)
			if err != nil {
				writeAuthError(w, r, "Missing or invalid Authorization header", http.StatusUnauthorized)
				return
			}

			// Validate the JWT
			claims, err := authClient.ValidateToken(r.Context(), tokenString)
			if err != nil {
				log.Warn().Err(err).Str("token_prefix", tokenString[:min(10, len(tokenString))]).Msg("JWT validation failed")

				// Determine specific error type and capture critical errors in Sentry
				errorMsg := "Invalid authentication token"
				statusCode := http.StatusUnauthorized

				if strings.Contains(err.Error(), "expired") {
					errorMsg = "Authentication token has expired"
					// Don't capture expired tokens - this is normal user behavior
				} else if strings.Contains(err.Error(), "signature") {
					errorMsg = "Invalid token signature"
					// Capture invalid signatures - potential security issue
					sentry.CaptureException(err)
				} else if strings.Contains(err.Error(), "SUPABASE_JWT_SECRET") {
					errorMsg = "Authentication service misconfigured"
					statusCode = http.StatusInternalServerError
					// Capture service misconfigurations - critical system error
					sentry.CaptureException(err)
				}

				writeAuthError(w, r, errorMsg, statusCode)
				return
			}

			// Add user claims to context using the auth client
			r = authClient.SetUserInContext(r, claims)
			next.ServeHTTP(w, r)
		})
	}
}

// validateSupabaseToken validates and parses a Supabase JWT token
func validateSupabaseToken(tokenString string) (*UserClaims, error) {
	// Get JWT secret from environment
	jwtSecret := os.Getenv("SUPABASE_JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("SUPABASE_JWT_SECRET environment variable not set")
	}

	// Parse and validate the token
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(jwtSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Extract and return claims
	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

// GetUserFromContext extracts user claims from the request context
func GetUserFromContext(ctx context.Context) (*UserClaims, bool) {
	user, ok := ctx.Value(UserKey).(*UserClaims)
	return user, ok
}

// OptionalAuthMiddleware validates JWT if present but doesn't require it
func OptionalAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if Authorization header is present
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")

			// Try to validate the token
			claims, err := validateSupabaseToken(tokenString)
			if err == nil {
				// Token is valid, add to context
				ctx := context.WithValue(r.Context(), UserKey, claims)
				r = r.WithContext(ctx)
			} else {
				// Token is invalid but we continue without auth
				log.Warn().Err(err).Msg("Invalid JWT token in optional auth")
			}
		}

		next.ServeHTTP(w, r)
	})
}

// writeAuthError writes a standardised authentication error response
func writeAuthError(w http.ResponseWriter, r *http.Request, message string, statusCode int) {
	// Get request ID if available (fallback to empty string if not)
	var requestID string
	if r != nil && r.Context() != nil {
		if rid := r.Context().Value("request_id"); rid != nil {
			if ridStr, ok := rid.(string); ok {
				requestID = ridStr
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"status":     statusCode,
		"message":    message,
		"code":       "UNAUTHORISED",
		"request_id": requestID,
	}

	json.NewEncoder(w).Encode(response)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SessionInfo holds session information and token validity
type SessionInfo struct {
	IsValid       bool   `json:"is_valid"`
	ExpiresAt     int64  `json:"expires_at,omitempty"`
	RefreshNeeded bool   `json:"refresh_needed"`
	UserID        string `json:"user_id,omitempty"`
	Email         string `json:"email,omitempty"`
}

// ValidateSession validates a JWT token and returns session information
func ValidateSession(tokenString string) *SessionInfo {
	claims, err := validateSupabaseToken(tokenString)
	if err != nil {
		return &SessionInfo{
			IsValid:       false,
			RefreshNeeded: strings.Contains(err.Error(), "expired"),
		}
	}

	// Check if token expires soon (within 5 minutes)
	refreshNeeded := false
	if claims.ExpiresAt != nil {
		timeUntilExpiry := claims.ExpiresAt.Time.Unix() - time.Now().Unix()
		refreshNeeded = timeUntilExpiry < 300 // 5 minutes
	}

	return &SessionInfo{
		IsValid:       true,
		ExpiresAt:     claims.ExpiresAt.Time.Unix(),
		RefreshNeeded: refreshNeeded,
		UserID:        claims.UserID,
		Email:         claims.Email,
	}
}

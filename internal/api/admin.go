package api

import (
	"net/http"
	"os"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/getsentry/sentry-go"
)

// AdminResetDatabase handles the admin database reset endpoint
// Requires valid JWT with admin role and explicit environment enablement
func (h *Handler) AdminResetDatabase(w http.ResponseWriter, r *http.Request) {
	logger := loggerWithRequest(r)

	if r.Method != http.MethodPost {
		MethodNotAllowed(w, r)
		return
	}

	// Check if this is running in development environment
	env := os.Getenv("APP_ENV")
	if env == "production" {
		NotFound(w, r, "Not found") // Return 404 in production to hide the endpoint
		return
	}

	// Require explicit enablement
	if os.Getenv("ALLOW_DB_RESET") != "true" {
		Forbidden(w, r, "Database reset not enabled. Set ALLOW_DB_RESET=true to enable")
		return
	}

	// Get user claims from context (set by AuthMiddleware)
	claims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "Authentication required for admin endpoint")
		return
	}

	// Verify system admin role
	if !hasSystemAdminRole(claims) {
		logger.Warn().
			Str("user_id", claims.UserID).
			Msg("Non-system-admin user attempted to access database reset endpoint")
		Forbidden(w, r, "System administrator privileges required")
		return
	}

	// Verify user exists in database
	user, err := h.DB.GetUser(claims.UserID)
	if err != nil {
		logger.Error().Err(err).Str("user_id", claims.UserID).Msg("Failed to verify admin user")
		Unauthorised(w, r, "User verification failed")
		return
	}

	// Log the admin action with full context
	logger.Warn().
		Str("user_id", user.ID).
		Str("org_id", func() string {
			if user.OrganisationID != nil {
				return *user.OrganisationID
			}
			return "none"
		}()).
		Str("remote_addr", r.RemoteAddr).
		Str("user_agent", r.Header.Get("User-Agent")).
		Msg("Admin database reset requested")

	// Capture in Sentry for audit trail
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("event_type", "admin_action")
		scope.SetTag("action", "database_reset")
		scope.SetUser(sentry.User{
			ID:    user.ID,
			Email: user.Email,
		})
		scope.SetContext("admin_action", map[string]interface{}{
			"endpoint":   "/admin/reset-db",
			"user_agent": r.Header.Get("User-Agent"),
			"ip_address": r.RemoteAddr,
		})
		sentry.CaptureMessage("Admin database reset action")
	})

	// Perform the database reset
	if err := h.DB.ResetSchema(); err != nil {
		logger.Error().Err(err).
			Str("user_id", user.ID).
			Msg("Failed to reset database schema")
		InternalError(w, r, err)
		return
	}

	logger.Info().
		Str("user_id", user.ID).
		Msg("Database schema reset completed by admin")

	WriteSuccess(w, r, nil, "Database schema reset successfully")
}

// hasSystemAdminRole checks if the user has system administrator privileges via app_metadata
// This is distinct from organisation-level admin roles - system admins are Blue Banded Bee operators
// who have elevated privileges for system-level operations like database resets
func hasSystemAdminRole(claims *auth.UserClaims) bool {
	if claims == nil || claims.AppMetadata == nil {
		return false
	}

	// Check for system_role = "system_admin" in app_metadata
	if systemRole, exists := claims.AppMetadata["system_role"]; exists {
		if roleStr, ok := systemRole.(string); ok && roleStr == "system_admin" {
			return true
		}
	}

	return false
}

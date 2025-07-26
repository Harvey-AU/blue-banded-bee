package api

import (
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
)

// AdminResetDatabase handles the admin database reset endpoint
// TODO: This should be properly secured or removed in production
func (h *Handler) AdminResetDatabase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		MethodNotAllowed(w, r)
		return
	}

	// Check if this is running in development environment
	env := os.Getenv("APP_ENV")
	if env == "production" {
		Forbidden(w, r, "Database reset not allowed in production")
		return
	}

	// TODO: Add proper admin authentication
	// For now, only allow if explicitly enabled
	if os.Getenv("ALLOW_DB_RESET") != "true" {
		Forbidden(w, r, "Database reset not enabled")
		return
	}

	log.Warn().Msg("Database reset requested")

	if err := h.DB.ResetSchema(); err != nil {
		log.Error().Err(err).Msg("Failed to reset database schema")
		InternalError(w, r, err)
		return
	}

	WriteSuccess(w, r, nil, "Database schema reset successfully")
}

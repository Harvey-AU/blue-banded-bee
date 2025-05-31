package api

import (
	"net/http"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/jobs"
)

// Handler holds dependencies for API handlers
type Handler struct {
	DB          *db.DB
	JobsManager *jobs.JobManager
}

// NewHandler creates a new API handler with dependencies
func NewHandler(pgDB *db.DB, jobsManager *jobs.JobManager) *Handler {
	return &Handler{
		DB:          pgDB,
		JobsManager: jobsManager,
	}
}

// SetupRoutes configures all API routes with proper middleware
func (h *Handler) SetupRoutes(mux *http.ServeMux) {
	// Health check endpoints (no auth required)
	mux.HandleFunc("/health", h.HealthCheck)
	mux.HandleFunc("/health/db", h.DatabaseHealthCheck)

	// V1 API routes with authentication
	mux.Handle("/v1/jobs", auth.AuthMiddleware(http.HandlerFunc(h.JobsHandler)))
	mux.Handle("/v1/jobs/", auth.AuthMiddleware(http.HandlerFunc(h.JobHandler))) // For /v1/jobs/:id

	// Authentication routes (no auth middleware)
	mux.HandleFunc("/v1/auth/register", h.AuthRegister)
	mux.HandleFunc("/v1/auth/session", h.AuthSession)
	
	// Profile route (requires auth)
	mux.Handle("/v1/auth/profile", auth.AuthMiddleware(http.HandlerFunc(h.AuthProfile)))


	// Admin endpoints (require special authentication)
	mux.HandleFunc("/admin/reset-db", h.AdminResetDatabase)

	// Static files
	mux.HandleFunc("/test-login.html", h.ServeTestLogin)
	mux.HandleFunc("/test-components.html", h.ServeTestComponents)
	mux.HandleFunc("/dashboard", h.ServeDashboard)
	
	// Web Components static files
	mux.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("./web/dist/"))))
}

// HealthCheck handles basic health check requests
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	WriteHealthy(w, r, "blue-banded-bee", "0.4.0")
}

// DatabaseHealthCheck handles database health check requests
func (h *Handler) DatabaseHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	if err := h.DB.GetDB().Ping(); err != nil {
		WriteUnhealthy(w, r, "postgresql", err)
		return
	}

	WriteHealthy(w, r, "postgresql", "")
}

// ServeTestLogin serves the test login page
func (h *Handler) ServeTestLogin(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "test-login.html")
}

// ServeTestComponents serves the Web Components test page
func (h *Handler) ServeTestComponents(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "test-components.html")
}

// ServeDashboard serves the dashboard page
func (h *Handler) ServeDashboard(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "dashboard.html")
}
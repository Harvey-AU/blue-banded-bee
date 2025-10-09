package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/jobs"
	"github.com/rs/zerolog/log"
)

// Version is the current API version (can be set via ldflags at build time)
var Version = "0.4.1"

// DBClient is an interface for database operations
type DBClient interface {
	GetDB() *sql.DB
	GetOrCreateUser(userID, email string, orgID *string) (*db.User, error)
	GetJobStats(organisationID string, startDate, endDate *time.Time) (*db.JobStats, error)
	GetJobActivity(organisationID string, startDate, endDate *time.Time) ([]db.ActivityPoint, error)
	GetSlowPages(organisationID string, startDate, endDate *time.Time) ([]db.SlowPage, error)
	GetExternalRedirects(organisationID string, startDate, endDate *time.Time) ([]db.ExternalRedirect, error)
	GetUserByWebhookToken(token string) (*db.User, error)
	// Additional methods used by API handlers
	GetUser(userID string) (*db.User, error)
	ResetSchema() error
	CreateUser(userID, email string, fullName *string, orgName string) (*db.User, *db.Organisation, error)
	GetOrganisation(organisationID string) (*db.Organisation, error)
	ListJobs(organisationID string, limit, offset int, status, dateRange string) ([]db.JobWithDomain, int, error)
}

// Handler holds dependencies for API handlers
type Handler struct {
	DB          DBClient
	JobsManager jobs.JobManagerInterface
}

// NewHandler creates a new API handler with dependencies
func NewHandler(pgDB DBClient, jobsManager jobs.JobManagerInterface) *Handler {
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
	// Shared job routes (public)
	mux.HandleFunc("/v1/shared/jobs/", h.SharedJobHandler)
	mux.HandleFunc("/shared/jobs/", h.ServeSharedJobPage)

	// Dashboard API routes (require auth)
	mux.Handle("/v1/dashboard/stats", auth.AuthMiddleware(http.HandlerFunc(h.DashboardStats)))
	mux.Handle("/v1/dashboard/activity", auth.AuthMiddleware(http.HandlerFunc(h.DashboardActivity)))
	mux.Handle("/v1/dashboard/slow-pages", auth.AuthMiddleware(http.HandlerFunc(h.DashboardSlowPages)))
	mux.Handle("/v1/dashboard/external-redirects", auth.AuthMiddleware(http.HandlerFunc(h.DashboardExternalRedirects)))

	// Metadata routes (require auth)
	mux.Handle("/v1/metadata/metrics", auth.AuthMiddleware(http.HandlerFunc(h.MetadataHandler)))

	// Authentication routes (no auth middleware)
	mux.HandleFunc("/v1/auth/register", h.AuthRegister)
	mux.HandleFunc("/v1/auth/session", h.AuthSession)

	// Profile route (requires auth)
	mux.Handle("/v1/auth/profile", auth.AuthMiddleware(http.HandlerFunc(h.AuthProfile)))

	// Webhook endpoints (no auth required)
	mux.HandleFunc("/v1/webhooks/webflow/", h.WebflowWebhook) // Note: trailing slash for path params

	// Admin endpoints (require authentication and admin role)
	mux.Handle("/admin/reset-db", auth.AuthMiddleware(http.HandlerFunc(h.AdminResetDatabase)))

	// Debug endpoints (no auth required)
	mux.HandleFunc("/debug/fgtrace", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open("trace.out")
		if err != nil {
			http.Error(w, "could not open trace file", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="trace.out"`)
		io.Copy(w, f)
	}))

	// Static files
	mux.HandleFunc("/", h.ServeHomepage) // Marketing homepage
	mux.HandleFunc("/test-login.html", h.ServeTestLogin)
	mux.HandleFunc("/test-components.html", h.ServeTestComponents)
	mux.HandleFunc("/test-data-components.html", h.ServeTestDataComponents)
	mux.HandleFunc("/dashboard", h.ServeDashboard)
	mux.HandleFunc("/dashboard-new", h.ServeNewDashboard)
	mux.HandleFunc("/auth-modal.html", h.ServeAuthModal)
	mux.HandleFunc("/debug-auth.html", h.ServeDebugAuth)
	mux.HandleFunc("/jobs/", h.ServeJobDetails)

	// Web Components static files
	mux.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("./web/static/js/"))))
	mux.Handle("/web/", http.StripPrefix("/web/", h.jsFileServer(http.Dir("./web/"))))
}

// HealthCheck handles basic health check requests
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	WriteHealthy(w, r, "blue-banded-bee", Version)
}

// DatabaseHealthCheck handles database health check requests
func (h *Handler) DatabaseHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	// Guard against nil DB to prevent panic
	if h.DB == nil {
		WriteUnhealthy(w, r, "postgresql", fmt.Errorf("database connection not configured"))
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

// ServeTestDataComponents serves the data components test page
func (h *Handler) ServeTestDataComponents(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "test-data-components.html")
}

// ServeDashboard serves the dashboard page
func (h *Handler) ServeDashboard(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "dashboard.html")
}

// ServeNewDashboard serves the new Web Components dashboard page
func (h *Handler) ServeNewDashboard(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "dashboard.html")
}

// ServeAuthModal serves the shared authentication modal
func (h *Handler) ServeAuthModal(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "auth-modal.html")
}

// ServeDebugAuth serves the debug auth test page
func (h *Handler) ServeDebugAuth(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "debug-auth.html")
}

// ServeJobDetails serves the standalone job details page
func (h *Handler) ServeJobDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	http.ServeFile(w, r, "web/templates/job-details.html")
}

// ServeSharedJobPage serves the public shared job view
func (h *Handler) ServeSharedJobPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	http.ServeFile(w, r, "web/templates/job-details.html")
}

// ServeHomepage serves the marketing homepage
func (h *Handler) ServeHomepage(w http.ResponseWriter, r *http.Request) {
	// Only serve homepage for exact root path
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "homepage.html")
}

// DashboardStats handles dashboard statistics requests
func (h *Handler) DashboardStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	// Get user claims from context
	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok || userClaims == nil {
		Unauthorised(w, r, "Authentication required")
		return
	}

	// Get full user object from database (auto-create if needed)
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		log.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
		InternalError(w, r, err)
		return
	}

	// Get query parameters
	dateRange := r.URL.Query().Get("range")
	if dateRange == "" {
		dateRange = "last7"
	}

	// Calculate date range for query
	startDate, endDate := calculateDateRange(dateRange)

	// Get job statistics
	orgID := ""
	if user.OrganisationID != nil {
		orgID = *user.OrganisationID
	}
	stats, err := h.DB.GetJobStats(orgID, startDate, endDate)
	if err != nil {
		DatabaseError(w, r, err)
		return
	}

	WriteSuccess(w, r, map[string]interface{}{
		"total_jobs":          stats.TotalJobs,
		"running_jobs":        stats.RunningJobs,
		"completed_jobs":      stats.CompletedJobs,
		"failed_jobs":         stats.FailedJobs,
		"total_tasks":         stats.TotalTasks,
		"avg_completion_time": stats.AvgCompletionTime,
		"date_range":          dateRange,
		"period_start":        startDate,
		"period_end":          endDate,
	}, "Dashboard statistics retrieved successfully")
}

// DashboardActivity handles dashboard activity chart requests
func (h *Handler) DashboardActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	// Get user claims from context
	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok || userClaims == nil {
		Unauthorised(w, r, "Authentication required")
		return
	}

	// Get full user object from database (auto-create if needed)
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		log.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
		InternalError(w, r, err)
		return
	}

	// Get query parameters
	dateRange := r.URL.Query().Get("range")
	if dateRange == "" {
		dateRange = "last7"
	}

	// Calculate date range for query
	startDate, endDate := calculateDateRange(dateRange)

	// Get activity data
	orgID := ""
	if user.OrganisationID != nil {
		orgID = *user.OrganisationID
	}
	activity, err := h.DB.GetJobActivity(orgID, startDate, endDate)
	if err != nil {
		DatabaseError(w, r, err)
		return
	}

	WriteSuccess(w, r, map[string]interface{}{
		"activity":     activity,
		"date_range":   dateRange,
		"period_start": startDate,
		"period_end":   endDate,
	}, "Dashboard activity retrieved successfully")
}

// calculateDateRange converts date range string to start and end times
func calculateDateRange(dateRange string) (*time.Time, *time.Time) {
	now := time.Now().UTC()
	var startDate, endDate *time.Time

	switch dateRange {
	case "today":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, time.UTC)
		startDate = &start
		endDate = &end
	case "last24":
		start := now.Add(-24 * time.Hour)
		startDate = &start
		endDate = &now
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		start := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, time.UTC)
		end := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 999999999, time.UTC)
		startDate = &start
		endDate = &end
	case "last7":
		start := now.AddDate(0, 0, -7)
		startDate = &start
		endDate = &now
	case "last30":
		start := now.AddDate(0, 0, -30)
		startDate = &start
		endDate = &now
	case "last90":
		start := now.AddDate(0, 0, -90)
		startDate = &start
		endDate = &now
	case "all":
		// Return nil for both to indicate no date filtering
		return nil, nil
	default:
		// Default to last 7 days
		start := now.AddDate(0, 0, -7)
		startDate = &start
		endDate = &now
	}

	return startDate, endDate
}

// jsFileServer creates a file server that sets correct MIME types for JavaScript files
func (h *Handler) jsFileServer(root http.FileSystem) http.Handler {
	fileServer := http.FileServer(root)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set correct MIME type for JavaScript files
		if strings.HasSuffix(r.URL.Path, ".js") {
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		}

		fileServer.ServeHTTP(w, r)
	})
}

// DashboardSlowPages handles requests for slow-loading pages analysis
func (h *Handler) DashboardSlowPages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	// Get user claims from context
	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok || userClaims == nil {
		Unauthorised(w, r, "Authentication required")
		return
	}

	// Get full user object from database (auto-create if needed)
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		log.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
		InternalError(w, r, err)
		return
	}

	// Get query parameters
	dateRange := r.URL.Query().Get("range")
	if dateRange == "" {
		dateRange = "last7"
	}

	// Calculate date range for query
	startDate, endDate := calculateDateRange(dateRange)

	// Get slow pages data
	orgID := ""
	if user.OrganisationID != nil {
		orgID = *user.OrganisationID
	}
	slowPages, err := h.DB.GetSlowPages(orgID, startDate, endDate)
	if err != nil {
		DatabaseError(w, r, err)
		return
	}

	WriteSuccess(w, r, map[string]interface{}{
		"slow_pages":   slowPages,
		"date_range":   dateRange,
		"period_start": startDate,
		"period_end":   endDate,
		"count":        len(slowPages),
	}, "Slow pages analysis retrieved successfully")
}

// DashboardExternalRedirects handles requests for external redirect analysis
func (h *Handler) DashboardExternalRedirects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	// Get user claims from context
	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok || userClaims == nil {
		Unauthorised(w, r, "Authentication required")
		return
	}

	// Get full user object from database (auto-create if needed)
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		log.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
		InternalError(w, r, err)
		return
	}

	// Get query parameters
	dateRange := r.URL.Query().Get("range")
	if dateRange == "" {
		dateRange = "last7"
	}

	// Calculate date range for query
	startDate, endDate := calculateDateRange(dateRange)

	// Get external redirects data
	orgID := ""
	if user.OrganisationID != nil {
		orgID = *user.OrganisationID
	}
	redirects, err := h.DB.GetExternalRedirects(orgID, startDate, endDate)
	if err != nil {
		DatabaseError(w, r, err)
		return
	}

	WriteSuccess(w, r, map[string]interface{}{
		"external_redirects": redirects,
		"date_range":         dateRange,
		"period_start":       startDate,
		"period_end":         endDate,
		"count":              len(redirects),
	}, "External redirects analysis retrieved successfully")
}

// WebflowWebhookPayload represents the structure of Webflow's site publish webhook
type WebflowWebhookPayload struct {
	TriggerType string `json:"triggerType"`
	Payload     struct {
		Domains     []string `json:"domains"`
		PublishedBy struct {
			DisplayName string `json:"displayName"`
		} `json:"publishedBy"`
	} `json:"payload"`
}

// WebflowWebhook handles Webflow site publish webhooks
func (h *Handler) WebflowWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		MethodNotAllowed(w, r)
		return
	}

	// Extract webhook token from URL path: /v1/webhooks/webflow/WEBHOOK_TOKEN
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 || pathParts[3] == "" {
		log.Error().Str("path", r.URL.Path).Msg("Webflow webhook missing token in URL")
		BadRequest(w, r, "Webhook token required in URL path")
		return
	}
	webhookToken := pathParts[3]

	// Parse webhook payload
	var payload WebflowWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Error().Err(err).Msg("Failed to parse Webflow webhook payload")
		BadRequest(w, r, "Invalid webhook payload")
		return
	}

	// Get user from database using webhook token
	user, err := h.DB.GetUserByWebhookToken(webhookToken)
	if err != nil {
		log.Error().Err(err).Str("webhook_token", webhookToken).Msg("Failed to get user by webhook token")
		// Return 404 to avoid leaking information about valid tokens
		NotFound(w, r, "Invalid webhook token")
		return
	}

	// Log webhook received
	log.Info().
		Str("user_id", user.ID).
		Str("trigger_type", payload.TriggerType).
		Str("published_by", payload.Payload.PublishedBy.DisplayName).
		Strs("domains", payload.Payload.Domains).
		Msg("Webflow webhook received")

	// Validate it's a site publish event
	if payload.TriggerType != "site_publish" {
		log.Warn().Str("trigger_type", payload.TriggerType).Msg("Ignoring non-site-publish webhook")
		WriteSuccess(w, r, nil, "Webhook received but ignored (not site_publish)")
		return
	}

	// Validate domains are provided
	if len(payload.Payload.Domains) == 0 {
		log.Error().Msg("Webflow webhook missing domains")
		BadRequest(w, r, "Domains are required")
		return
	}

	// Use the first domain in the list (primary/canonical domain)
	selectedDomain := payload.Payload.Domains[0]

	// Create job using shared logic with webhook defaults
	useSitemap := true
	findLinks := true
	concurrency := 3
	maxPages := 0 // Unlimited pages for webhook-triggered jobs
	sourceType := "webflow_webhook"
	sourceDetail := payload.Payload.PublishedBy.DisplayName

	// Store full webhook payload for debugging
	sourceInfoBytes, _ := json.Marshal(payload)
	sourceInfo := string(sourceInfoBytes)

	req := CreateJobRequest{
		Domain:       selectedDomain,
		UseSitemap:   &useSitemap,
		FindLinks:    &findLinks,
		Concurrency:  &concurrency,
		MaxPages:     &maxPages,
		SourceType:   &sourceType,
		SourceDetail: &sourceDetail,
		SourceInfo:   &sourceInfo,
	}

	job, err := h.createJobFromRequest(r.Context(), user, req)
	if err != nil {
		log.Error().Err(err).
			Str("user_id", user.ID).
			Str("domain", selectedDomain).
			Msg("Failed to create job from webhook")
		InternalError(w, r, err)
		return
	}

	// Start the job immediately
	if err := h.JobsManager.StartJob(r.Context(), job.ID); err != nil {
		log.Error().Err(err).Str("job_id", job.ID).Msg("Failed to start job from webhook")
		// Don't return error - job was created successfully, just failed to start
	}

	orgIDStr := ""
	if user.OrganisationID != nil {
		orgIDStr = *user.OrganisationID
	}

	log.Info().
		Str("job_id", job.ID).
		Str("user_id", user.ID).
		Str("org_id", orgIDStr).
		Str("domain", selectedDomain).
		Str("selected_from", strings.Join(payload.Payload.Domains, ", ")).
		Msg("Successfully created and started job from Webflow webhook")

	WriteSuccess(w, r, map[string]interface{}{
		"job_id":  job.ID,
		"user_id": user.ID,
		"org_id":  orgIDStr,
		"domain":  selectedDomain,
		"status":  "created",
	}, "Job created successfully from webhook")
}

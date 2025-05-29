package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/Harvey-AU/blue-banded-bee/internal/jobs"
	"github.com/rs/zerolog/log"
)

// JobsHandler handles requests to /v1/jobs
func (h *Handler) JobsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listJobs(w, r)
	case http.MethodPost:
		h.createJob(w, r)
	default:
		MethodNotAllowed(w, r)
	}
}

// JobHandler handles requests to /v1/jobs/:id
func (h *Handler) JobHandler(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path
	path := strings.TrimPrefix(r.URL.Path, "/v1/jobs/")
	if path == "" {
		BadRequest(w, r, "Job ID is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getJob(w, r, path)
	case http.MethodPut:
		h.updateJob(w, r, path)
	case http.MethodDelete:
		h.cancelJob(w, r, path)
	default:
		MethodNotAllowed(w, r)
	}
}

// CreateJobRequest represents the request body for creating a job
type CreateJobRequest struct {
	Domain      string `json:"domain"`
	UseSitemap  *bool  `json:"use_sitemap,omitempty"`
	FindLinks   *bool  `json:"find_links,omitempty"`
	Concurrency *int   `json:"concurrency,omitempty"`
	MaxPages    *int   `json:"max_pages,omitempty"`
}

// JobResponse represents a job in API responses
type JobResponse struct {
	ID           string `json:"id"`
	Domain       string `json:"domain"`
	Status       string `json:"status"`
	TotalTasks   int    `json:"total_tasks"`
	CompletedTasks int  `json:"completed_tasks"`
	FailedTasks    int  `json:"failed_tasks"`
	SkippedTasks   int  `json:"skipped_tasks"`
	Progress       float64 `json:"progress"`
	CreatedAt      string  `json:"created_at"`
	CompletedAt    *string `json:"completed_at,omitempty"`
}

// listJobs handles GET /v1/jobs
func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	_, err := h.DB.GetUser(userClaims.UserID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get user from database")
		Unauthorised(w, r, "User not found")
		return
	}

	// TODO: Implement list jobs functionality
	// For now, return empty list
	WriteSuccess(w, r, []JobResponse{}, "Jobs retrieved successfully")
}

// createJob handles POST /v1/jobs
func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	user, err := h.DB.GetUser(userClaims.UserID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get user from database")
		Unauthorised(w, r, "User not found")
		return
	}

	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, r, "Invalid JSON request body")
		return
	}

	if req.Domain == "" {
		BadRequest(w, r, "Domain is required")
		return
	}

	// Set defaults
	useSitemap := true
	if req.UseSitemap != nil {
		useSitemap = *req.UseSitemap
	}

	findLinks := true
	if req.FindLinks != nil {
		findLinks = *req.FindLinks
	}

	concurrency := 5
	if req.Concurrency != nil {
		concurrency = *req.Concurrency
	}

	maxPages := 0
	if req.MaxPages != nil {
		maxPages = *req.MaxPages
	}

	opts := &jobs.JobOptions{
		Domain:         req.Domain,
		UserID:         &user.ID,
		OrganisationID: user.OrganisationID,
		UseSitemap:     useSitemap,
		Concurrency:    concurrency,
		FindLinks:      findLinks,
		MaxPages:       maxPages,
	}

	job, err := h.JobsManager.CreateJob(r.Context(), opts)
	if err != nil {
		log.Error().Err(err).Str("domain", req.Domain).Msg("Failed to create job")
		InternalError(w, r, err)
		return
	}

	response := JobResponse{
		ID:             job.ID,
		Domain:         job.Domain,
		Status:         string(job.Status),
		TotalTasks:     job.TotalTasks,
		CompletedTasks: job.CompletedTasks,
		FailedTasks:    job.FailedTasks,
		SkippedTasks:   job.SkippedTasks,
		Progress:       0.0,
		CreatedAt:      job.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	WriteCreated(w, r, response, "Job created successfully")
}

// getJob handles GET /v1/jobs/:id
func (h *Handler) getJob(w http.ResponseWriter, r *http.Request, jobID string) {
	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	user, err := h.DB.GetUser(userClaims.UserID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get user from database")
		Unauthorised(w, r, "User not found")
		return
	}

	var total, completed, failed, skipped int
	var status string
	err = h.DB.GetDB().QueryRowContext(r.Context(), `
		SELECT total_tasks, completed_tasks, failed_tasks, skipped_tasks, status 
		FROM jobs WHERE id = $1 AND organisation_id = $2
	`, jobID, user.OrganisationID).Scan(&total, &completed, &failed, &skipped, &status)

	if err != nil {
		NotFound(w, r, "Job not found")
		return
	}

	// Calculate progress
	progress := 0.0
	if total > skipped {
		progress = float64(completed+failed) / float64(total-skipped) * 100
	}

	response := JobResponse{
		ID:             jobID,
		Status:         status,
		TotalTasks:     total,
		CompletedTasks: completed,
		FailedTasks:    failed,
		SkippedTasks:   skipped,
		Progress:       progress,
	}

	WriteSuccess(w, r, response, "Job retrieved successfully")
}

// updateJob handles PUT /v1/jobs/:id (placeholder for future use)
func (h *Handler) updateJob(w http.ResponseWriter, r *http.Request, jobID string) {
	BadRequest(w, r, "Job updates not yet implemented")
}

// cancelJob handles DELETE /v1/jobs/:id (placeholder for future use)
func (h *Handler) cancelJob(w http.ResponseWriter, r *http.Request, jobID string) {
	BadRequest(w, r, "Job cancellation not yet implemented")
}


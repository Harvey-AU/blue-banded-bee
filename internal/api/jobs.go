package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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

	// Handle sub-routes like /v1/jobs/:id/tasks
	parts := strings.Split(path, "/")
	jobID := parts[0]

	if len(parts) > 1 {
		// Handle sub-routes
		switch parts[1] {
		case "tasks":
			h.getJobTasks(w, r, jobID)
			return
		default:
			NotFound(w, r, "Endpoint not found")
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		h.getJob(w, r, jobID)
	case http.MethodPut:
		h.updateJob(w, r, jobID)
	case http.MethodDelete:
		h.cancelJob(w, r, jobID)
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

	user, err := h.DB.GetUser(userClaims.UserID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get user from database")
		Unauthorised(w, r, "User not found")
		return
	}

	// Parse query parameters
	limit := 10 // default
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	status := r.URL.Query().Get("status") // Optional status filter
	dateRange := r.URL.Query().Get("range") // Optional date range filter
	include := r.URL.Query().Get("include") // Optional includes (domain, progress, etc.)

	// Get jobs from database
	orgID := ""
	if user.OrganisationID != nil {
		orgID = *user.OrganisationID
	}
	jobs, total, err := h.DB.ListJobs(orgID, limit, offset, status, dateRange)
	if err != nil {
		log.Error().Err(err).Str("organisation_id", orgID).Msg("Failed to list jobs")
		DatabaseError(w, r, err)
		return
	}

	// Calculate pagination info
	hasNext := offset+limit < total
	hasPrev := offset > 0

	// Prepare response
	response := map[string]interface{}{
		"jobs": jobs,
		"pagination": map[string]interface{}{
			"limit":     limit,
			"offset":    offset,
			"total":     total,
			"has_next":  hasNext,
			"has_prev":  hasPrev,
		},
	}

	if include != "" {
		// Add additional data based on include parameter
		response["include"] = include
	}

	WriteSuccess(w, r, response, "Jobs retrieved successfully")
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
		CreatedAt:      job.CreatedAt.Format(time.RFC3339),
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

// JobActionRequest represents actions that can be performed on jobs
type JobActionRequest struct {
	Action string `json:"action"`
}

// updateJob handles PUT /v1/jobs/:id for job actions
func (h *Handler) updateJob(w http.ResponseWriter, r *http.Request, jobID string) {
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

	var req JobActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, r, "Invalid JSON request body")
		return
	}

	// Verify job belongs to user's organisation
	var orgID string
	err = h.DB.GetDB().QueryRowContext(r.Context(), `
		SELECT organisation_id FROM jobs WHERE id = $1
	`, jobID).Scan(&orgID)

	if err != nil {
		NotFound(w, r, "Job not found")
		return
	}

	if user.OrganisationID == nil || *user.OrganisationID != orgID {
		Unauthorised(w, r, "Job access denied")
		return
	}

	switch req.Action {
	case "start", "restart":
		err = h.JobsManager.StartJob(r.Context(), jobID)
	case "cancel":
		err = h.JobsManager.CancelJob(r.Context(), jobID)
	default:
		BadRequest(w, r, "Invalid action. Supported actions: start, restart, cancel")
		return
	}

	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Str("action", req.Action).Msg("Failed to perform job action")
		InternalError(w, r, err)
		return
	}

	// Get updated job status
	job, err := h.JobsManager.GetJobStatus(r.Context(), jobID)
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job status after action")
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
		Progress:       job.Progress,
		CreatedAt:      job.CreatedAt.Format(time.RFC3339),
	}

	if !job.CompletedAt.IsZero() {
		completedAt := job.CompletedAt.Format(time.RFC3339)
		response.CompletedAt = &completedAt
	}

	WriteSuccess(w, r, response, fmt.Sprintf("Job %s %sed successfully", jobID, req.Action))
}

// cancelJob handles DELETE /v1/jobs/:id
func (h *Handler) cancelJob(w http.ResponseWriter, r *http.Request, jobID string) {
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

	// Verify job belongs to user's organisation
	var orgID string
	err = h.DB.GetDB().QueryRowContext(r.Context(), `
		SELECT organisation_id FROM jobs WHERE id = $1
	`, jobID).Scan(&orgID)

	if err != nil {
		NotFound(w, r, "Job not found")
		return
	}

	if user.OrganisationID == nil || *user.OrganisationID != orgID {
		Unauthorised(w, r, "Job access denied")
		return
	}

	err = h.JobsManager.CancelJob(r.Context(), jobID)
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to cancel job")
		InternalError(w, r, err)
		return
	}

	WriteSuccess(w, r, map[string]string{"id": jobID, "status": "cancelled"}, "Job cancelled successfully")
}

// TaskResponse represents a task in API responses
type TaskResponse struct {
	ID           string  `json:"id"`
	JobID        string  `json:"job_id"`
	Path         string  `json:"path"`
	Status       string  `json:"status"`
	StatusCode   *int    `json:"status_code,omitempty"`
	ResponseTime *int    `json:"response_time,omitempty"`
	CacheStatus  *string `json:"cache_status,omitempty"`
	ContentType  *string `json:"content_type,omitempty"`
	Error        *string `json:"error,omitempty"`
	SourceType   *string `json:"source_type,omitempty"`
	CreatedAt    string  `json:"created_at"`
	StartedAt    *string `json:"started_at,omitempty"`
	CompletedAt  *string `json:"completed_at,omitempty"`
	RetryCount   int     `json:"retry_count"`
}

// getJobTasks handles GET /v1/jobs/:id/tasks
func (h *Handler) getJobTasks(w http.ResponseWriter, r *http.Request, jobID string) {
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

	// Verify job belongs to user's organisation
	var orgID string
	err = h.DB.GetDB().QueryRowContext(r.Context(), `
		SELECT organisation_id FROM jobs WHERE id = $1
	`, jobID).Scan(&orgID)

	if err != nil {
		NotFound(w, r, "Job not found")
		return
	}

	if user.OrganisationID == nil || *user.OrganisationID != orgID {
		Unauthorised(w, r, "Job access denied")
		return
	}

	// Parse query parameters
	limit := 50 // default
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 200 {
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	status := r.URL.Query().Get("status") // Optional status filter

	// Build query with optional status filter
	baseQuery := `
		SELECT t.id, t.job_id, p.path, t.status, t.status_code, t.response_time, 
		       t.cache_status, t.content_type, t.error, t.source_type,
		       t.created_at, t.started_at, t.completed_at, t.retry_count
		FROM tasks t
		JOIN pages p ON t.page_id = p.id
		WHERE t.job_id = $1`
	
	countQuery := `
		SELECT COUNT(*) 
		FROM tasks t 
		WHERE t.job_id = $1`

	args := []interface{}{jobID}
	
	if status != "" {
		baseQuery += ` AND t.status = $2`
		countQuery += ` AND t.status = $2`
		args = append(args, status)
	}

	baseQuery += ` ORDER BY t.created_at DESC LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)
	args = append(args, limit, offset)

	// Get total count
	var total int
	countArgs := args[:len(args)-2] // Remove limit and offset for count query
	err = h.DB.GetDB().QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&total)
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to count tasks")
		DatabaseError(w, r, err)
		return
	}

	// Get tasks
	rows, err := h.DB.GetDB().QueryContext(r.Context(), baseQuery, args...)
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to get tasks")
		DatabaseError(w, r, err)
		return
	}
	defer rows.Close()

	var tasks []TaskResponse
	for rows.Next() {
		var task TaskResponse
		var startedAt, completedAt, createdAt sql.NullTime
		var statusCode, responseTime sql.NullInt32
		var cacheStatus, contentType, errorMsg, sourceType sql.NullString

		err := rows.Scan(
			&task.ID, &task.JobID, &task.Path, &task.Status,
			&statusCode, &responseTime, &cacheStatus, &contentType, &errorMsg, &sourceType,
			&createdAt, &startedAt, &completedAt, &task.RetryCount,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan task row")
			continue
		}

		// Handle nullable fields
		if statusCode.Valid {
			sc := int(statusCode.Int32)
			task.StatusCode = &sc
		}
		if responseTime.Valid {
			rt := int(responseTime.Int32)
			task.ResponseTime = &rt
		}
		if cacheStatus.Valid {
			task.CacheStatus = &cacheStatus.String
		}
		if contentType.Valid {
			task.ContentType = &contentType.String
		}
		if errorMsg.Valid {
			task.Error = &errorMsg.String
		}
		if sourceType.Valid {
			task.SourceType = &sourceType.String
		}
		if startedAt.Valid {
			sa := startedAt.Time.Format(time.RFC3339)
			task.StartedAt = &sa
		}
		if completedAt.Valid {
			ca := completedAt.Time.Format(time.RFC3339)
			task.CompletedAt = &ca
		}

		// Format created_at
		if createdAt.Valid {
			task.CreatedAt = createdAt.Time.Format(time.RFC3339)
		}

		tasks = append(tasks, task)
	}

	// Calculate pagination info
	hasNext := offset+limit < total
	hasPrev := offset > 0

	// Prepare response
	response := map[string]interface{}{
		"tasks": tasks,
		"pagination": map[string]interface{}{
			"limit":     limit,
			"offset":    offset,
			"total":     total,
			"has_next":  hasNext,
			"has_prev":  hasPrev,
		},
	}

	WriteSuccess(w, r, response, "Tasks retrieved successfully")
}


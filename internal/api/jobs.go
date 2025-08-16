package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
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
	Domain       string  `json:"domain"`
	UseSitemap   *bool   `json:"use_sitemap,omitempty"`
	FindLinks    *bool   `json:"find_links,omitempty"`
	Concurrency  *int    `json:"concurrency,omitempty"`
	MaxPages     *int    `json:"max_pages,omitempty"`
	SourceType   *string `json:"source_type,omitempty"`
	SourceDetail *string `json:"source_detail,omitempty"`
	SourceInfo   *string `json:"source_info,omitempty"`
}

// JobResponse represents a job in API responses
type JobResponse struct {
	ID             string  `json:"id"`
	Domain         string  `json:"domain"`
	Status         string  `json:"status"`
	TotalTasks     int     `json:"total_tasks"`
	CompletedTasks int     `json:"completed_tasks"`
	FailedTasks    int     `json:"failed_tasks"`
	SkippedTasks   int     `json:"skipped_tasks"`
	Progress       float64 `json:"progress"`
	CreatedAt      string  `json:"created_at"`
	StartedAt      *string `json:"started_at,omitempty"`
	CompletedAt    *string `json:"completed_at,omitempty"`
}

// listJobs handles GET /v1/jobs
func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	// Auto-create user if they don't exist (handles new signups)
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		log.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
		InternalError(w, r, err)
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

	status := r.URL.Query().Get("status")   // Optional status filter
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
			"limit":    limit,
			"offset":   offset,
			"total":    total,
			"has_next": hasNext,
			"has_prev": hasPrev,
		},
	}

	if include != "" {
		// Add additional data based on include parameter
		response["include"] = include
	}

	WriteSuccess(w, r, response, "Jobs retrieved successfully")
}

// createJobFromRequest creates a job from a CreateJobRequest with user context
func (h *Handler) createJobFromRequest(ctx context.Context, user *db.User, req CreateJobRequest) (*jobs.Job, error) {
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
		SourceType:     req.SourceType,
		SourceDetail:   req.SourceDetail,
		SourceInfo:     req.SourceInfo,
	}

	return h.JobsManager.CreateJob(ctx, opts)
}

// createJob handles POST /v1/jobs
func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	// Auto-create user if they don't exist (handles new signups)
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		log.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
		InternalError(w, r, err)
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

	// Set source information if not provided (dashboard creation)
	if req.SourceType == nil {
		sourceType := "dashboard"
		req.SourceType = &sourceType
	}
	if req.SourceDetail == nil {
		sourceDetail := "create_job"
		req.SourceDetail = &sourceDetail
	}
	if req.SourceInfo == nil {
		sourceInfoData := map[string]interface{}{
			"ip":        getClientIP(r),
			"userAgent": r.UserAgent(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"endpoint":  r.URL.Path,
			"method":    r.Method,
		}
		sourceInfoBytes, _ := json.Marshal(sourceInfoData)
		sourceInfo := string(sourceInfoBytes)
		req.SourceInfo = &sourceInfo
	}

	job, err := h.createJobFromRequest(r.Context(), user, req)
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

	// Auto-create user if they don't exist (handles new signups)
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		log.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
		InternalError(w, r, err)
		return
	}

	var total, completed, failed, skipped int
	var status, domain string
	var createdAt, startedAt, completedAt sql.NullTime
	err = h.DB.GetDB().QueryRowContext(r.Context(), `
		SELECT j.total_tasks, j.completed_tasks, j.failed_tasks, j.skipped_tasks, j.status,
		       d.name as domain, j.created_at, j.started_at, j.completed_at
		FROM jobs j
		JOIN domains d ON j.domain_id = d.id
		WHERE j.id = $1 AND j.organisation_id = $2
	`, jobID, user.OrganisationID).Scan(&total, &completed, &failed, &skipped, &status, &domain, &createdAt, &startedAt, &completedAt)

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
		Domain:         domain,
		Status:         status,
		TotalTasks:     total,
		CompletedTasks: completed,
		FailedTasks:    failed,
		SkippedTasks:   skipped,
		Progress:       progress,
	}

	// Add timestamps if available
	if createdAt.Valid {
		response.CreatedAt = createdAt.Time.Format(time.RFC3339)
	}
	if startedAt.Valid {
		startedTime := startedAt.Time.Format(time.RFC3339)
		response.StartedAt = &startedTime
	}
	if completedAt.Valid {
		completedTime := completedAt.Time.Format(time.RFC3339)
		response.CompletedAt = &completedTime
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

	// Auto-create user if they don't exist (handles new signups)
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		log.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
		InternalError(w, r, err)
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

	// Auto-create user if they don't exist (handles new signups)
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		log.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
		InternalError(w, r, err)
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

// TaskQueryParams holds parameters for task listing queries
type TaskQueryParams struct {
	Limit   int
	Offset  int
	Status  string
	OrderBy string
}

// parseTaskQueryParams extracts and validates query parameters for task listing
func parseTaskQueryParams(r *http.Request) TaskQueryParams {
	// Parse limit parameter
	limit := 50 // default
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 200 {
			limit = parsedLimit
		}
	}

	// Parse offset parameter
	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// Parse status filter
	status := r.URL.Query().Get("status") // Optional status filter
	
	// Parse sort parameter
	sortParam := r.URL.Query().Get("sort") // Optional sort parameter
	orderBy := "t.created_at DESC" // default
	if sortParam != "" {
		// Handle sort direction prefix
		direction := "DESC"
		column := sortParam
		if strings.HasPrefix(sortParam, "-") {
			direction = "DESC"
			column = strings.TrimPrefix(sortParam, "-")
		} else {
			direction = "ASC"
		}

		// Map column names to actual SQL columns
		switch column {
		case "path":
			orderBy = "p.path " + direction
		case "status":
			orderBy = "t.status " + direction
		case "response_time":
			orderBy = "t.response_time " + direction + " NULLS LAST"
		case "cache_status":
			orderBy = "t.cache_status " + direction + " NULLS LAST"
		case "second_response_time":
			orderBy = "t.second_response_time " + direction + " NULLS LAST"
		case "status_code":
			orderBy = "t.status_code " + direction + " NULLS LAST"
		case "created_at":
			orderBy = "t.created_at " + direction
		default:
			orderBy = "t.created_at DESC" // fallback to default
		}
	}

	return TaskQueryParams{
		Limit:   limit,
		Offset:  offset,
		Status:  status,
		OrderBy: orderBy,
	}
}

// validateJobAccess validates user authentication and job access permissions
// Returns the user if validation succeeds, or writes HTTP error and returns nil
func (h *Handler) validateJobAccess(w http.ResponseWriter, r *http.Request, jobID string) *db.User {
	// Extract user claims from context
	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return nil
	}

	// Auto-create user if they don't exist (handles new signups)
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		log.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
		InternalError(w, r, err)
		return nil
	}

	// Verify job belongs to user's organisation
	var orgID string
	err = h.DB.GetDB().QueryRowContext(r.Context(), `
		SELECT organisation_id FROM jobs WHERE id = $1
	`, jobID).Scan(&orgID)

	if err != nil {
		NotFound(w, r, "Job not found")
		return nil
	}

	if user.OrganisationID == nil || *user.OrganisationID != orgID {
		Unauthorised(w, r, "Job access denied")
		return nil
	}

	return user
}

// TaskQueryBuilder holds the SQL queries and arguments for task retrieval
type TaskQueryBuilder struct {
	SelectQuery string
	CountQuery  string
	Args        []interface{}
}

// buildTaskQuery constructs SQL queries for task retrieval with filters and pagination
func buildTaskQuery(jobID string, params TaskQueryParams) TaskQueryBuilder {
	baseQuery := `
		SELECT t.id, t.job_id, p.path, d.name as domain, t.status, t.status_code, t.response_time, 
		       t.cache_status, t.second_response_time, t.second_cache_status, t.content_type, t.error, t.source_type, t.source_url,
		       t.created_at, t.started_at, t.completed_at, t.retry_count
		FROM tasks t
		JOIN pages p ON t.page_id = p.id
		JOIN jobs j ON t.job_id = j.id
		JOIN domains d ON j.domain_id = d.id
		WHERE t.job_id = $1`

	countQuery := `
		SELECT COUNT(*) 
		FROM tasks t 
		WHERE t.job_id = $1`

	args := []interface{}{jobID}

	// Add status filter if provided
	if params.Status != "" {
		baseQuery += ` AND t.status = $2`
		countQuery += ` AND t.status = $2`
		args = append(args, params.Status)
	}

	// Add ordering, limit, and offset
	baseQuery += ` ORDER BY ` + params.OrderBy + ` LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)
	args = append(args, params.Limit, params.Offset)

	return TaskQueryBuilder{
		SelectQuery: baseQuery,
		CountQuery:  countQuery,
		Args:        args,
	}
}

func getClientIP(r *http.Request) string {
	// Check for X-Forwarded-For header first (common with proxies/load balancers)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs, use the first one
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check for X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// TaskResponse represents a task in API responses
type TaskResponse struct {
	ID                 string  `json:"id"`
	JobID              string  `json:"job_id"`
	Path               string  `json:"path"`
	URL                string  `json:"url"`
	Status             string  `json:"status"`
	StatusCode         *int    `json:"status_code,omitempty"`
	ResponseTime       *int    `json:"response_time,omitempty"`
	CacheStatus        *string `json:"cache_status,omitempty"`
	SecondResponseTime *int    `json:"second_response_time,omitempty"`
	SecondCacheStatus  *string `json:"second_cache_status,omitempty"`
	ContentType        *string `json:"content_type,omitempty"`
	Error              *string `json:"error,omitempty"`
	SourceType         *string `json:"source_type,omitempty"`
	SourceURL          *string `json:"source_url,omitempty"`
	CreatedAt          string  `json:"created_at"`
	StartedAt          *string `json:"started_at,omitempty"`
	CompletedAt        *string `json:"completed_at,omitempty"`
	RetryCount         int     `json:"retry_count"`
}

// getJobTasks handles GET /v1/jobs/:id/tasks
func (h *Handler) getJobTasks(w http.ResponseWriter, r *http.Request, jobID string) {
	// Validate user authentication and job access
	user := h.validateJobAccess(w, r, jobID)
	if user == nil {
		return // validateJobAccess already wrote the error response
	}

	// Parse query parameters and build queries
	params := parseTaskQueryParams(r)
	queries := buildTaskQuery(jobID, params)

	// Get total count
	var total int
	countArgs := queries.Args[:len(queries.Args)-2] // Remove limit and offset for count query
	err := h.DB.GetDB().QueryRowContext(r.Context(), queries.CountQuery, countArgs...).Scan(&total)
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to count tasks")
		DatabaseError(w, r, err)
		return
	}

	// Get tasks
	rows, err := h.DB.GetDB().QueryContext(r.Context(), queries.SelectQuery, queries.Args...)
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to get tasks")
		DatabaseError(w, r, err)
		return
	}
	defer rows.Close()

	var tasks []TaskResponse
	for rows.Next() {
		var task TaskResponse
		var domain string
		var startedAt, completedAt, createdAt sql.NullTime
		var statusCode, responseTime, secondResponseTime sql.NullInt32
		var cacheStatus, secondCacheStatus, contentType, errorMsg, sourceType, sourceURL sql.NullString

		err := rows.Scan(
			&task.ID, &task.JobID, &task.Path, &domain, &task.Status,
			&statusCode, &responseTime, &cacheStatus, &secondResponseTime, &secondCacheStatus, &contentType, &errorMsg, &sourceType, &sourceURL,
			&createdAt, &startedAt, &completedAt, &task.RetryCount,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan task row")
			continue
		}

		// Construct full URL from domain and path
		task.URL = fmt.Sprintf("https://%s%s", domain, task.Path)

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
		if secondResponseTime.Valid {
			srt := int(secondResponseTime.Int32)
			task.SecondResponseTime = &srt
		}
		if secondCacheStatus.Valid {
			task.SecondCacheStatus = &secondCacheStatus.String
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
		if sourceURL.Valid {
			task.SourceURL = &sourceURL.String
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
	hasNext := params.Offset+params.Limit < total
	hasPrev := params.Offset > 0

	// Prepare response
	response := map[string]interface{}{
		"tasks": tasks,
		"pagination": map[string]interface{}{
			"limit":    params.Limit,
			"offset":   params.Offset,
			"total":    total,
			"has_next": hasNext,
			"has_prev": hasPrev,
		},
	}

	WriteSuccess(w, r, response, "Tasks retrieved successfully")
}

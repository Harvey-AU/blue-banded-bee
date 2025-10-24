package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/jobs"
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
		case "export":
			h.exportJobTasks(w, r, jobID)
			return
		case "share-links":
			if len(parts) == 2 {
				switch r.Method {
				case http.MethodPost:
					h.createJobShareLink(w, r, jobID)
					return
				case http.MethodGet:
					h.getJobShareLink(w, r, jobID)
					return
				}
				MethodNotAllowed(w, r)
				return
			}

			if len(parts) == 3 {
				token := parts[2]
				if r.Method == http.MethodDelete {
					h.revokeJobShareLink(w, r, jobID, token)
					return
				}
				MethodNotAllowed(w, r)
				return
			}

			NotFound(w, r, "Endpoint not found")
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
	// Additional fields for dashboard
	DurationSeconds       *int                   `json:"duration_seconds,omitempty"`
	AvgTimePerTaskSeconds *float64               `json:"avg_time_per_task_seconds,omitempty"`
	Stats                 map[string]interface{} `json:"stats,omitempty"`
}

// listJobs handles GET /v1/jobs
func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	logger := loggerWithRequest(r)

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	// Auto-create user if they don't exist (handles new signups)
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		if HandlePoolSaturation(w, r, err) {
			return
		}
		logger.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
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
	timezone := r.URL.Query().Get("tz")     // Optional timezone (e.g., "Australia/Sydney")
	include := r.URL.Query().Get("include") // Optional includes (domain, progress, etc.)

	// Default timezone to UTC if not provided
	if timezone == "" {
		timezone = "UTC"
	}

	// Get jobs from database
	orgID := ""
	if user.OrganisationID != nil {
		orgID = *user.OrganisationID
	}
	jobs, total, err := h.DB.ListJobs(orgID, limit, offset, status, dateRange, timezone)
	if err != nil {
		if HandlePoolSaturation(w, r, err) {
			return
		}
		logger.Error().Err(err).Str("organisation_id", orgID).Msg("Failed to list jobs")
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
	logger := loggerWithRequest(r)

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	// Auto-create user if they don't exist (handles new signups)
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		if HandlePoolSaturation(w, r, err) {
			return
		}
		logger.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
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
		if HandlePoolSaturation(w, r, err) {
			return
		}
		logger.Error().Err(err).Str("domain", req.Domain).Msg("Failed to create job")
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
	logger := loggerWithRequest(r)

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	// Auto-create user if they don't exist (handles new signups)
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		if HandlePoolSaturation(w, r, err) {
			return
		}
		logger.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
		InternalError(w, r, err)
		return
	}

	response, err := h.fetchJobResponse(r.Context(), jobID, user.OrganisationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			NotFound(w, r, "Job not found")
			return
		}
		if HandlePoolSaturation(w, r, err) {
			return
		}

		logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to fetch job details")
		InternalError(w, r, err)
		return
	}

	WriteSuccess(w, r, response, "Job retrieved successfully")
}

func (h *Handler) fetchJobResponse(ctx context.Context, jobID string, organisationID *string) (JobResponse, error) {
	var total, completed, failed, skipped int
	var status, domain string
	var createdAt, startedAt, completedAt sql.NullTime
	var durationSeconds sql.NullInt64
	var avgTimePerTaskSeconds sql.NullFloat64
	var statsJSON []byte

	query := `
		SELECT j.total_tasks, j.completed_tasks, j.failed_tasks, j.skipped_tasks, j.status,
		       d.name as domain, j.created_at, j.started_at, j.completed_at,
		       EXTRACT(EPOCH FROM (j.completed_at - j.started_at))::INTEGER as duration_seconds,
		       CASE WHEN j.completed_tasks > 0 THEN
		           EXTRACT(EPOCH FROM (j.completed_at - j.started_at)) / j.completed_tasks
		       END as avg_time_per_task_seconds,
		       j.stats
		FROM jobs j
		JOIN domains d ON j.domain_id = d.id
		WHERE j.id = $1`

	args := []interface{}{jobID}
	if organisationID != nil {
		query += ` AND j.organisation_id = $2`
		args = append(args, *organisationID)
	}

	row := h.DB.GetDB().QueryRowContext(ctx, query, args...)
	err := row.Scan(&total, &completed, &failed, &skipped, &status, &domain, &createdAt, &startedAt, &completedAt, &durationSeconds, &avgTimePerTaskSeconds, &statsJSON)
	if err != nil {
		return JobResponse{}, err
	}

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

	if durationSeconds.Valid {
		duration := int(durationSeconds.Int64)
		response.DurationSeconds = &duration
	}
	if avgTimePerTaskSeconds.Valid {
		avgTime := avgTimePerTaskSeconds.Float64
		response.AvgTimePerTaskSeconds = &avgTime
	}

	if len(statsJSON) > 0 {
		var stats map[string]interface{}
		if err := json.Unmarshal(statsJSON, &stats); err == nil {
			response.Stats = stats
		}
	}

	if createdAt.Valid {
		response.CreatedAt = createdAt.Time.Format(time.RFC3339)
	} else {
		response.CreatedAt = time.Now().Format(time.RFC3339)
	}
	if startedAt.Valid {
		started := startedAt.Time.Format(time.RFC3339)
		response.StartedAt = &started
	}
	if completedAt.Valid {
		completed := completedAt.Time.Format(time.RFC3339)
		response.CompletedAt = &completed
	}

	return response, nil
}

// JobActionRequest represents actions that can be performed on jobs
type JobActionRequest struct {
	Action string `json:"action"`
}

// updateJob handles PUT /v1/jobs/:id for job actions
func (h *Handler) updateJob(w http.ResponseWriter, r *http.Request, jobID string) {
	logger := loggerWithRequest(r)

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	// Auto-create user if they don't exist (handles new signups)
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		logger.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
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
		logger.Error().Err(err).Str("job_id", jobID).Str("action", req.Action).Msg("Failed to perform job action")
		InternalError(w, r, err)
		return
	}

	// Get updated job status
	job, err := h.JobsManager.GetJobStatus(r.Context(), jobID)
	if err != nil {
		logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job status after action")
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
	logger := loggerWithRequest(r)

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	// Auto-create user if they don't exist (handles new signups)
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		logger.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
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
		logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to cancel job")
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
	orderBy := "t.created_at DESC"         // default
	if sortParam != "" {
		// Handle sort direction prefix
		var direction string
		var column string
		if strings.HasPrefix(sortParam, "-") {
			direction = "DESC"
			column = strings.TrimPrefix(sortParam, "-")
		} else {
			direction = "ASC"
			column = sortParam
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
	logger := loggerWithRequest(r)

	// Extract user claims from context
	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return nil
	}

	// Auto-create user if they don't exist (handles new signups)
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		logger.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
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

// formatTasksFromRows converts database rows into TaskResponse slice
func formatTasksFromRows(rows *sql.Rows) ([]TaskResponse, error) {
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
			return nil, fmt.Errorf("failed to scan task row: %w", err)
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

	return tasks, nil
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

// ExportColumn describes a column in exported task datasets
type ExportColumn struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

func taskExportColumns(exportType string) []ExportColumn {
	switch exportType {
	case "broken-links":
		return []ExportColumn{
			{Key: "source_url", Label: "Found on"},
			{Key: "url", Label: "Broken link"},
			{Key: "status", Label: "Status"},
			{Key: "created_at", Label: "Date"},
			{Key: "source_type", Label: "Source Type"},
		}
	case "slow-pages":
		return []ExportColumn{
			{Key: "url", Label: "Page"},
			{Key: "content_type", Label: "Content Type"},
			{Key: "cache_status", Label: "Cache Status"},
			{Key: "response_time", Label: "Load Time (ms)"},
			{Key: "second_response_time", Label: "Load Time 2nd try (ms)"},
			{Key: "created_at", Label: "Date"},
		}
	default: // "job" (all tasks)
		return []ExportColumn{
			{Key: "id", Label: "Task ID"},
			{Key: "job_id", Label: "Job ID"},
			{Key: "path", Label: "Page path"},
			{Key: "url", Label: "Page URL"},
			{Key: "content_type", Label: "Content Type"},
			{Key: "status", Label: "Status"},
			{Key: "cache_status", Label: "Cache Status"},
			{Key: "status_code", Label: "Status Code"},
			{Key: "response_time", Label: "Load Time (ms)"},
			{Key: "second_cache_status", Label: "Second Cache Status"},
			{Key: "second_response_time", Label: "Load Response Time (ms)"},
			{Key: "retry_count", Label: "Retry Count"},
			{Key: "error", Label: "Error"},
			{Key: "source_type", Label: "Source"},
			{Key: "source_url", Label: "Source page"},
			{Key: "created_at", Label: "Created At"},
			{Key: "started_at", Label: "Started At"},
			{Key: "completed_at", Label: "Completed At"},
		}
	}
}

// getJobTasks handles GET /v1/jobs/:id/tasks
func (h *Handler) getJobTasks(w http.ResponseWriter, r *http.Request, jobID string) {
	logger := loggerWithRequest(r)

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
		if HandlePoolSaturation(w, r, err) {
			return
		}
		logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to count tasks")
		DatabaseError(w, r, err)
		return
	}

	// Get tasks
	rows, err := h.DB.GetDB().QueryContext(r.Context(), queries.SelectQuery, queries.Args...)
	if err != nil {
		if HandlePoolSaturation(w, r, err) {
			return
		}
		logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get tasks")
		DatabaseError(w, r, err)
		return
	}
	defer rows.Close()

	// Format tasks from database rows
	tasks, err := formatTasksFromRows(rows)
	if err != nil {
		logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to format tasks")
		DatabaseError(w, r, err)
		return
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

// exportJobTasks handles GET /v1/jobs/:id/export
func (h *Handler) exportJobTasks(w http.ResponseWriter, r *http.Request, jobID string) {
	h.serveJobExport(w, r, jobID, true)
}

func (h *Handler) serveJobExport(w http.ResponseWriter, r *http.Request, jobID string, requireAuth bool) {
	logger := loggerWithRequest(r)

	if requireAuth {
		if h.validateJobAccess(w, r, jobID) == nil {
			return
		}
	}

	// Get export type from query parameter
	exportType := r.URL.Query().Get("type")
	if exportType == "" {
		exportType = "job" // Default to all tasks
	}

	// Build query based on export type
	var whereClause string

	switch exportType {
	case "broken-links":
		whereClause = " AND t.status = 'failed'"
	case "slow-pages":
		// Use second_response_time (cache HIT) when available, fallback to response_time
		whereClause = " AND COALESCE(t.second_response_time, t.response_time) > 3000"
	case "job":
		// Export all tasks
		whereClause = ""
	default:
		BadRequest(w, r, fmt.Sprintf("Invalid export type: %s", exportType))
		return
	}

	// Query tasks
	query := fmt.Sprintf(`
		SELECT
			t.id, t.job_id, p.path, d.name as domain,
			t.status, t.status_code, t.response_time, t.cache_status,
			t.second_response_time, t.second_cache_status,
			t.content_type, t.error, t.source_type, t.source_url,
			t.created_at, t.started_at, t.completed_at, t.retry_count
		FROM tasks t
		JOIN pages p ON t.page_id = p.id
		JOIN domains d ON p.domain_id = d.id
		WHERE t.job_id = $1%s
		ORDER BY t.created_at DESC
		LIMIT 10000
	`, whereClause)

	rows, err := h.DB.GetDB().QueryContext(r.Context(), query, jobID)
	if err != nil {
		logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to export tasks")
		DatabaseError(w, r, err)
		return
	}
	defer rows.Close()

	// Format tasks from database rows
	tasks, err := formatTasksFromRows(rows)
	if err != nil {
		logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to format export tasks")
		DatabaseError(w, r, err)
		return
	}

	// Get job details
	var domain, status string
	var createdAt time.Time
	var completedAt sql.NullTime
	err = h.DB.GetDB().QueryRowContext(r.Context(), `
		SELECT d.name, j.status, j.created_at, j.completed_at
		FROM jobs j
		JOIN domains d ON j.domain_id = d.id
		WHERE j.id = $1
	`, jobID).Scan(&domain, &status, &createdAt, &completedAt)

	if err != nil {
		logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job details for export")
		DatabaseError(w, r, err)
		return
	}

	// Prepare export response
	response := map[string]interface{}{
		"job_id":      jobID,
		"domain":      domain,
		"status":      status,
		"created_at":  createdAt.Format(time.RFC3339),
		"export_type": exportType,
		"export_time": time.Now().UTC().Format(time.RFC3339),
		"total_tasks": len(tasks),
		"columns":     taskExportColumns(exportType),
		"tasks":       tasks,
	}
	if completedAt.Valid {
		response["completed_at"] = completedAt.Time.Format(time.RFC3339)
	} else {
		response["completed_at"] = nil
	}

	WriteSuccess(w, r, response, fmt.Sprintf("Exported %d tasks for job %s", len(tasks), jobID))
}

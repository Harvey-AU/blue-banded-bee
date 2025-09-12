package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// JobStats represents job statistics for the dashboard
type JobStats struct {
	TotalJobs         int     `json:"total_jobs"`
	RunningJobs       int     `json:"running_jobs"`
	CompletedJobs     int     `json:"completed_jobs"`
	FailedJobs        int     `json:"failed_jobs"`
	TotalTasks        int     `json:"total_tasks"`
	AvgCompletionTime float64 `json:"avg_completion_time"`
}

// ActivityPoint represents a data point for activity charts
type ActivityPoint struct {
	Timestamp  string `json:"timestamp"`
	JobsCount  int    `json:"jobs_count"`
	TasksCount int    `json:"tasks_count"`
}

// GetJobStats retrieves job statistics for the dashboard
func (db *DB) GetJobStats(organisationID string, startDate, endDate *time.Time) (*JobStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_jobs,
			SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) as running_jobs,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed_jobs,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_jobs,
			SUM(COALESCE(total_tasks, 0)) as total_tasks,
			AVG(
				CASE WHEN status = 'completed' AND started_at IS NOT NULL AND completed_at IS NOT NULL 
				THEN EXTRACT(EPOCH FROM (completed_at - started_at))
				ELSE NULL END
			) as avg_completion_time
		FROM jobs 
		WHERE organisation_id = $1`

	args := []interface{}{organisationID}
	argCount := 1

	// Add date filtering if provided
	if startDate != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args = append(args, *startDate)
	}
	if endDate != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args = append(args, *endDate)
	}

	var stats JobStats
	var avgCompletionTime sql.NullFloat64

	err := db.client.QueryRow(query, args...).Scan(
		&stats.TotalJobs,
		&stats.RunningJobs,
		&stats.CompletedJobs,
		&stats.FailedJobs,
		&stats.TotalTasks,
		&avgCompletionTime,
	)

	if err != nil {
		log.Error().Err(err).Str("organisation_id", organisationID).Msg("Failed to get job stats")
		return nil, err
	}

	if avgCompletionTime.Valid {
		stats.AvgCompletionTime = avgCompletionTime.Float64
	}

	return &stats, nil
}

// GetJobActivity retrieves job activity data for charts
func (db *DB) GetJobActivity(organisationID string, startDate, endDate *time.Time) ([]ActivityPoint, error) {
	// Determine the appropriate time grouping based on date range
	var timeGroup string
	var intervalStr string

	if startDate != nil && endDate != nil {
		duration := endDate.Sub(*startDate)
		if duration <= 24*time.Hour {
			// Less than 1 day: group by hour
			timeGroup = "DATE_TRUNC('hour', created_at)"
			intervalStr = "1 hour"
		} else if duration <= 7*24*time.Hour {
			// Less than 1 week: group by 6 hours
			timeGroup = "DATE_TRUNC('hour', created_at) + INTERVAL '6 hours' * FLOOR(EXTRACT(HOUR FROM created_at) / 6)"
			intervalStr = "6 hours"
		} else if duration <= 30*24*time.Hour {
			// Less than 1 month: group by day
			timeGroup = "DATE_TRUNC('day', created_at)"
			intervalStr = "1 day"
		} else {
			// More than 1 month: group by week
			timeGroup = "DATE_TRUNC('week', created_at)"
			intervalStr = "1 week"
		}
	} else {
		// Default to daily grouping
		timeGroup = "DATE_TRUNC('day', created_at)"
		intervalStr = "1 day"
	}

	query := fmt.Sprintf(`
		WITH time_series AS (
			SELECT generate_series(
				COALESCE($2, DATE_TRUNC('day', NOW() - INTERVAL '7 days')),
				COALESCE($3, DATE_TRUNC('day', NOW())),
				INTERVAL '%s'
			) as time_bucket
		),
		job_activity AS (
			SELECT 
				%s as time_bucket,
				COUNT(*) as jobs_count,
				SUM(COALESCE(total_tasks, 0)) as tasks_count
			FROM jobs 
			WHERE organisation_id = $1`, intervalStr, timeGroup)

	args := []interface{}{organisationID, startDate, endDate}
	argCount := 3

	// Add date filtering if provided
	if startDate != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args = append(args, *startDate)
	}
	if endDate != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args = append(args, *endDate)
	}

	query += `
			GROUP BY ` + timeGroup + `
		)
		SELECT 
			ts.time_bucket,
			COALESCE(ja.jobs_count, 0) as jobs_count,
			COALESCE(ja.tasks_count, 0) as tasks_count
		FROM time_series ts
		LEFT JOIN job_activity ja ON ts.time_bucket = ja.time_bucket
		ORDER BY ts.time_bucket`

	rows, err := db.client.Query(query, args...)
	if err != nil {
		log.Error().Err(err).Str("organisation_id", organisationID).Msg("Failed to get job activity")
		return nil, err
	}
	defer rows.Close()

	var activity []ActivityPoint
	for rows.Next() {
		var point ActivityPoint
		var timestamp time.Time

		err := rows.Scan(&timestamp, &point.JobsCount, &point.TasksCount)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan activity row")
			continue
		}

		point.Timestamp = timestamp.Format(time.RFC3339)
		activity = append(activity, point)
	}

	if err = rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating activity rows")
		return nil, err
	}

	return activity, nil
}

// JobListItem represents a job in the list view
type JobListItem struct {
	ID                    string   `json:"id"`
	Status                string   `json:"status"`
	Progress              float64  `json:"progress"`
	TotalTasks            int      `json:"total_tasks"`
	CompletedTasks        int      `json:"completed_tasks"`
	FailedTasks           int      `json:"failed_tasks"`
	SitemapTasks          int      `json:"sitemap_tasks"`
	FoundTasks            int      `json:"found_tasks"`
	CreatedAt             string   `json:"created_at"`
	StartedAt             *string  `json:"started_at,omitempty"`
	CompletedAt           *string  `json:"completed_at,omitempty"`
	Domain                *string  `json:"domains,omitempty"` // For compatibility with frontend
	DurationSeconds       *int     `json:"duration_seconds,omitempty"`
	AvgTimePerTaskSeconds *float64 `json:"avg_time_per_task_seconds,omitempty"`
}

// Domain represents the domain information for jobs
type Domain struct {
	Name string `json:"name"`
}

// JobWithDomain represents a job with domain information
type JobWithDomain struct {
	JobListItem
	Domains *Domain `json:"domains"`
}

// ListJobs retrieves a paginated list of jobs for an organisation
func (db *DB) ListJobs(organisationID string, limit, offset int, status, dateRange string) ([]JobWithDomain, int, error) {
	// Build the base query
	baseQuery := `
		FROM jobs j
		LEFT JOIN domains d ON j.domain_id = d.id
		WHERE j.organisation_id = $1`

	args := []interface{}{organisationID}
	argCount := 1

	// Add status filter if provided
	if status != "" {
		argCount++
		baseQuery += fmt.Sprintf(" AND j.status = $%d", argCount)
		args = append(args, status)
	}

	// Add date range filter if provided
	if dateRange != "" {
		startDate, endDate := calculateDateRangeForList(dateRange)
		if startDate != nil {
			argCount++
			baseQuery += fmt.Sprintf(" AND j.created_at >= $%d", argCount)
			args = append(args, *startDate)
		}
		if endDate != nil {
			argCount++
			baseQuery += fmt.Sprintf(" AND j.created_at <= $%d", argCount)
			args = append(args, *endDate)
		}
	}

	// Get total count
	countQuery := "SELECT COUNT(*) " + baseQuery
	var total int
	err := db.client.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count jobs: %w", err)
	}

	// Get jobs with pagination
	selectQuery := `
		SELECT 
			j.id, j.status, j.progress, j.total_tasks, j.completed_tasks, 
			j.failed_tasks, j.sitemap_tasks, j.found_tasks, j.created_at,
			j.started_at, j.completed_at, d.name as domain_name,
			j.duration_seconds, j.avg_time_per_task_seconds
		` + baseQuery + `
		ORDER BY j.created_at DESC
		LIMIT $` + fmt.Sprintf("%d", argCount+1) + ` OFFSET $` + fmt.Sprintf("%d", argCount+2)

	args = append(args, limit, offset)

	rows, err := db.client.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []JobWithDomain
	for rows.Next() {
		var job JobWithDomain
		var startedAt, completedAt sql.NullString
		var domainName sql.NullString

		err := rows.Scan(
			&job.ID, &job.Status, &job.Progress, &job.TotalTasks, &job.CompletedTasks,
			&job.FailedTasks, &job.SitemapTasks, &job.FoundTasks, &job.CreatedAt,
			&startedAt, &completedAt, &domainName,
			&job.DurationSeconds, &job.AvgTimePerTaskSeconds,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan job row")
			continue
		}

		// Handle nullable fields
		if startedAt.Valid {
			job.StartedAt = &startedAt.String
		}
		if completedAt.Valid {
			job.CompletedAt = &completedAt.String
		}
		if domainName.Valid {
			job.Domains = &Domain{Name: domainName.String}
		}

		jobs = append(jobs, job)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating job rows: %w", err)
	}

	return jobs, total, nil
}

// calculateDateRangeForList is a helper function for list queries
func calculateDateRangeForList(dateRange string) (*time.Time, *time.Time) {
	now := time.Now().UTC()
	var startDate, endDate *time.Time

	switch dateRange {
	case "today":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, time.UTC)
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

// SlowPage represents a slow-loading page for dashboard analysis
type SlowPage struct {
	URL                string `json:"url"`
	Domain             string `json:"domain"`
	Path               string `json:"path"`
	SecondResponseTime int64  `json:"second_response_time"` // milliseconds after cache retry
	JobID              string `json:"job_id"`
	CompletedAt        string `json:"completed_at"`
}

// ExternalRedirect represents a page that redirects to an external domain
type ExternalRedirect struct {
	URL         string `json:"url"`
	Domain      string `json:"domain"`
	Path        string `json:"path"`
	RedirectURL string `json:"redirect_url"`
	JobID       string `json:"job_id"`
	CompletedAt string `json:"completed_at"`
}

// GetSlowPages retrieves the slowest pages after cache retry attempts
// Returns top 10 absolute slowest and 10% slowest from user's organisation
func (db *DB) GetSlowPages(organisationID string, startDate, endDate *time.Time) ([]SlowPage, error) {
	query := `
		WITH user_tasks AS (
			SELECT 
				'https://' || d.name || p.path as url,
				d.name as domain,
				p.path,
				t.second_response_time,
				t.job_id,
				t.completed_at
			FROM tasks t
			JOIN jobs j ON t.job_id = j.id
			JOIN pages p ON t.page_id = p.id
			JOIN domains d ON p.domain_id = d.id
			WHERE j.organisation_id = $1
				AND t.status = 'completed'
				AND t.second_response_time IS NOT NULL
				AND t.second_response_time > 0
				AND ($2::timestamp IS NULL OR t.completed_at >= $2)
				AND ($3::timestamp IS NULL OR t.completed_at <= $3)
		),
		top_10_absolute AS (
			SELECT *, 'absolute' as category
			FROM user_tasks
			ORDER BY second_response_time DESC
			LIMIT 10
		),
		slowest_percentile AS (
			SELECT *, 'percentile' as category
			FROM user_tasks
			WHERE second_response_time >= (
				SELECT PERCENTILE_CONT(0.9) WITHIN GROUP (ORDER BY second_response_time)
				FROM user_tasks
			)
			ORDER BY second_response_time DESC
			LIMIT 10
		)
		SELECT DISTINCT 
			url, domain, path, second_response_time, job_id, 
			completed_at::timestamp AT TIME ZONE 'UTC' as completed_at
		FROM (
			SELECT * FROM top_10_absolute
			UNION ALL
			SELECT * FROM slowest_percentile
		) combined
		ORDER BY second_response_time DESC
		LIMIT 20;
	`

	rows, err := db.client.Query(query, organisationID, startDate, endDate)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query slow pages")
		return nil, err
	}
	defer rows.Close()

	var slowPages []SlowPage
	for rows.Next() {
		var page SlowPage
		var completedAt sql.NullTime

		err := rows.Scan(
			&page.URL,
			&page.Domain,
			&page.Path,
			&page.SecondResponseTime,
			&page.JobID,
			&completedAt,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan slow page row")
			return nil, err
		}

		if completedAt.Valid {
			page.CompletedAt = completedAt.Time.Format(time.RFC3339)
		}

		slowPages = append(slowPages, page)
	}

	if err = rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating slow pages rows")
		return nil, err
	}

	return slowPages, nil
}

// GetExternalRedirects retrieves pages that redirect to external domains
func (db *DB) GetExternalRedirects(organisationID string, startDate, endDate *time.Time) ([]ExternalRedirect, error) {
	query := `
		SELECT 
			'https://' || d.name || p.path as url,
			d.name as domain,
			p.path,
			t.redirect_url,
			t.job_id,
			t.completed_at::timestamp AT TIME ZONE 'UTC' as completed_at
		FROM tasks t
		JOIN jobs j ON t.job_id = j.id
		JOIN pages p ON t.page_id = p.id
		JOIN domains d ON p.domain_id = d.id
		WHERE j.organisation_id = $1
			AND t.status = 'completed'
			AND t.redirect_url IS NOT NULL
			AND t.redirect_url != ''
			-- Check if redirect URL is external (different domain)
			AND NOT (
				t.redirect_url LIKE 'http://' || d.name || '%' OR
				t.redirect_url LIKE 'https://' || d.name || '%' OR
				t.redirect_url LIKE '//%' || d.name || '%' OR
				t.redirect_url LIKE '/' || '%'  -- relative paths
			)
			AND ($2::timestamp IS NULL OR t.completed_at >= $2)
			AND ($3::timestamp IS NULL OR t.completed_at <= $3)
		ORDER BY t.completed_at DESC
		LIMIT 100;
	`

	rows, err := db.client.Query(query, organisationID, startDate, endDate)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query external redirects")
		return nil, err
	}
	defer rows.Close()

	var redirects []ExternalRedirect
	for rows.Next() {
		var redirect ExternalRedirect
		var completedAt sql.NullTime

		err := rows.Scan(
			&redirect.URL,
			&redirect.Domain,
			&redirect.Path,
			&redirect.RedirectURL,
			&redirect.JobID,
			&completedAt,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan external redirect row")
			return nil, err
		}

		if completedAt.Valid {
			redirect.CompletedAt = completedAt.Time.Format(time.RFC3339)
		}

		redirects = append(redirects, redirect)
	}

	if err = rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating external redirects rows")
		return nil, err
	}

	return redirects, nil
}

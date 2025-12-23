package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type shareLinkSuccessResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Exists    bool   `json:"exists"`
		Token     string `json:"token"`
		ShareLink string `json:"share_link"`
	} `json:"data"`
}

func decodeShareLinkSuccess(t *testing.T, rec *httptest.ResponseRecorder) shareLinkSuccessResponse {
	var payload shareLinkSuccessResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	return payload
}

type shareLinkRevokeResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Revoked bool `json:"revoked"`
	} `json:"data"`
}

func decodeShareLinkRevoke(t *testing.T, rec *httptest.ResponseRecorder) shareLinkRevokeResponse {
	var payload shareLinkRevokeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	return payload
}

func TestCreateJobShareLinkCreatesNewToken(t *testing.T) {
	handler, mockDB, _ := createTestHandler()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	orgID := "org-123"
	user := &db.User{
		ID:             "user-123",
		Email:          "test@example.com",
		OrganisationID: &orgID,
	}

	mockDB.On("GetOrCreateUser", user.ID, user.Email, (*string)(nil)).Return(user, nil)
	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
		WithArgs("job-123").
		WillReturnRows(sqlmock.NewRows([]string{"organisation_id"}).AddRow(orgID))

	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT token\s+FROM job_share_links`).
		WithArgs("job-123").
		WillReturnError(sql.ErrNoRows)

	mock.ExpectExec(`INSERT INTO job_share_links`).
		WithArgs("job-123", sqlmock.AnyArg(), user.ID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req := createAuthenticatedRequest(http.MethodPost, "/v1/jobs/job-123/share-links", nil)
	req.Host = "example.com"
	ctx := context.WithValue(req.Context(), auth.UserKey, &auth.UserClaims{
		UserID: user.ID,
		Email:  user.Email,
	})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.createJobShareLink(rec, req, "job-123")

	assert.Equal(t, http.StatusOK, rec.Code)

	resp := decodeShareLinkSuccess(t, rec)
	assert.Equal(t, "success", resp.Status)
	assert.NotEmpty(t, resp.Data.Token)
	assert.Contains(t, resp.Data.ShareLink, resp.Data.Token)
	assert.Contains(t, resp.Data.ShareLink, "example.com")

	assert.NoError(t, mock.ExpectationsWereMet())
	mockDB.AssertExpectations(t)
}

func TestCreateJobShareLinkReturnsExistingToken(t *testing.T) {
	handler, mockDB, _ := createTestHandler()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	orgID := "org-789"
	user := &db.User{
		ID:             "user-456",
		Email:          "test@example.com",
		OrganisationID: &orgID,
	}

	mockDB.On("GetOrCreateUser", user.ID, user.Email, (*string)(nil)).Return(user, nil)
	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
		WithArgs("job-456").
		WillReturnRows(sqlmock.NewRows([]string{"organisation_id"}).AddRow(orgID))

	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT token\s+FROM job_share_links`).
		WithArgs("job-456").
		WillReturnRows(sqlmock.NewRows([]string{"token"}).AddRow("existing-token"))

	req := createAuthenticatedRequest(http.MethodPost, "/v1/jobs/job-456/share-links", nil)
	req.Host = "dashboard.example"
	ctx := context.WithValue(req.Context(), auth.UserKey, &auth.UserClaims{
		UserID: user.ID,
		Email:  user.Email,
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.createJobShareLink(rec, req, "job-456")

	assert.Equal(t, http.StatusOK, rec.Code)

	resp := decodeShareLinkSuccess(t, rec)
	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, "existing-token", resp.Data.Token)
	assert.Contains(t, resp.Data.ShareLink, "existing-token")
	assert.Equal(t, "Share link already exists", resp.Message)

	assert.NoError(t, mock.ExpectationsWereMet())
	mockDB.AssertExpectations(t)
}

func TestGetJobShareLinkSuccess(t *testing.T) {
	handler, mockDB, _ := createTestHandler()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	orgID := "org-222"
	user := &db.User{
		ID:             "user-222",
		Email:          "viewer@example.com",
		OrganisationID: &orgID,
	}

	mockDB.On("GetOrCreateUser", user.ID, user.Email, (*string)(nil)).Return(user, nil)
	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
		WithArgs("job-222").
		WillReturnRows(sqlmock.NewRows([]string{"organisation_id"}).AddRow(orgID))

	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT token\s+FROM job_share_links`).
		WithArgs("job-222").
		WillReturnRows(sqlmock.NewRows([]string{"token"}).AddRow("active-token"))

	req := createAuthenticatedRequest(http.MethodGet, "/v1/jobs/job-222/share-links", nil)
	req.Host = "app.example"
	ctx := context.WithValue(req.Context(), auth.UserKey, &auth.UserClaims{
		UserID: user.ID,
		Email:  user.Email,
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.getJobShareLink(rec, req, "job-222")

	assert.Equal(t, http.StatusOK, rec.Code)

	resp := decodeShareLinkSuccess(t, rec)
	assert.Equal(t, "success", resp.Status)
	assert.True(t, resp.Data.Exists)
	assert.Equal(t, "active-token", resp.Data.Token)
	assert.Contains(t, resp.Data.ShareLink, "active-token")
	assert.Contains(t, resp.Data.ShareLink, "app.example")

	assert.NoError(t, mock.ExpectationsWereMet())
	mockDB.AssertExpectations(t)
}

func TestGetJobShareLinkNotFound(t *testing.T) {
	handler, mockDB, _ := createTestHandler()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	orgID := "org-333"
	user := &db.User{
		ID:             "user-333",
		Email:          "viewer@example.com",
		OrganisationID: &orgID,
	}

	mockDB.On("GetOrCreateUser", user.ID, user.Email, (*string)(nil)).Return(user, nil)
	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
		WithArgs("job-333").
		WillReturnRows(sqlmock.NewRows([]string{"organisation_id"}).AddRow(orgID))

	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT token\s+FROM job_share_links`).
		WithArgs("job-333").
		WillReturnError(sql.ErrNoRows)

	req := createAuthenticatedRequest(http.MethodGet, "/v1/jobs/job-333/share-links", nil)
	ctx := context.WithValue(req.Context(), auth.UserKey, &auth.UserClaims{
		UserID: user.ID,
		Email:  user.Email,
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.getJobShareLink(rec, req, "job-333")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"exists":false`)
	assert.Contains(t, rec.Body.String(), "No active share link")

	assert.NoError(t, mock.ExpectationsWereMet())
	mockDB.AssertExpectations(t)
}

func TestRevokeJobShareLinkSuccess(t *testing.T) {
	handler, mockDB, _ := createTestHandler()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	orgID := "org-001"
	user := &db.User{
		ID:             "user-001",
		Email:          "revoker@example.com",
		OrganisationID: &orgID,
	}

	mockDB.On("GetOrCreateUser", user.ID, user.Email, (*string)(nil)).Return(user, nil)
	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
		WithArgs("job-001").
		WillReturnRows(sqlmock.NewRows([]string{"organisation_id"}).AddRow(orgID))

	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectExec(`UPDATE job_share_links`).
		WithArgs("job-001", "token-001").
		WillReturnResult(sqlmock.NewResult(1, 1))

	req := createAuthenticatedRequest(http.MethodDelete, "/v1/jobs/job-001/share-links/token-001", nil)
	ctx := context.WithValue(req.Context(), auth.UserKey, &auth.UserClaims{
		UserID: user.ID,
		Email:  user.Email,
	})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.revokeJobShareLink(rec, req, "job-001", "token-001")

	assert.Equal(t, http.StatusOK, rec.Code)

	resp := decodeShareLinkRevoke(t, rec)
	assert.Equal(t, "success", resp.Status)
	assert.True(t, resp.Data.Revoked)

	assert.NoError(t, mock.ExpectationsWereMet())
	mockDB.AssertExpectations(t)
}

func TestRevokeJobShareLinkNotFound(t *testing.T) {
	handler, mockDB, _ := createTestHandler()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	orgID := "org-404"
	user := &db.User{
		ID:             "user-404",
		Email:          "missing@example.com",
		OrganisationID: &orgID,
	}

	mockDB.On("GetOrCreateUser", user.ID, user.Email, (*string)(nil)).Return(user, nil)
	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
		WithArgs("job-404").
		WillReturnRows(sqlmock.NewRows([]string{"organisation_id"}).AddRow(orgID))

	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectExec(`UPDATE job_share_links`).
		WithArgs("job-404", "missing-token").
		WillReturnResult(sqlmock.NewResult(1, 0))

	req := createAuthenticatedRequest(http.MethodDelete, "/v1/jobs/job-404/share-links/missing-token", nil)
	ctx := context.WithValue(req.Context(), auth.UserKey, &auth.UserClaims{
		UserID: user.ID,
		Email:  user.Email,
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.revokeJobShareLink(rec, req, "job-404", "missing-token")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "Share link not found")

	assert.NoError(t, mock.ExpectationsWereMet())
	mockDB.AssertExpectations(t)
}

func TestSharedJobHandlerReturnsJob(t *testing.T) {
	handler, mockDB, _ := createTestHandler()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT job_id, expires_at, revoked_at FROM job_share_links`).
		WithArgs("token-123").
		WillReturnRows(sqlmock.NewRows([]string{"job_id", "expires_at", "revoked_at"}).AddRow("job-123", nil, nil))

	mockDB.On("GetDB").Return(sqlDB)
	now := time.Now()
	mock.ExpectQuery(`SELECT j.total_tasks, j.completed_tasks`).
		WithArgs("job-123").
		WillReturnRows(sqlmock.NewRows([]string{
			"total_tasks", "completed_tasks", "failed_tasks", "skipped_tasks", "status",
			"domain", "created_at", "started_at", "completed_at", "duration_seconds",
			"avg_time_per_task_seconds", "stats", "scheduler_id",
			"concurrency", "max_pages", "source_type",
			"crawl_delay_seconds", "adaptive_delay_seconds",
		}).AddRow(
			10, 8, 1, 1, "completed",
			"example.com", now, now, now,
			int64(120), float64(12.5), []byte(`{}`), nil,
			5, 50, "sitemap",
			nil, 0,
		))

	req := httptest.NewRequest(http.MethodGet, "/v1/shared/jobs/token-123", nil)
	rec := httptest.NewRecorder()

	handler.SharedJobHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var payload map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &payload)
	require.NoError(t, err)

	data, ok := payload["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "example.com", data["domain"])
	assert.Equal(t, float64(10), data["total_tasks"])

	assert.NoError(t, mock.ExpectationsWereMet())
	mockDB.AssertExpectations(t)
}

func TestSharedJobHandlerRevokedLink(t *testing.T) {
	handler, mockDB, _ := createTestHandler()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT job_id, expires_at, revoked_at FROM job_share_links`).
		WithArgs("revoked-token").
		WillReturnRows(sqlmock.NewRows([]string{"job_id", "expires_at", "revoked_at"}).
			AddRow("job-999", nil, time.Now()))

	req := httptest.NewRequest(http.MethodGet, "/v1/shared/jobs/revoked-token", nil)
	rec := httptest.NewRecorder()

	handler.SharedJobHandler(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "Share link has been revoked")

	assert.NoError(t, mock.ExpectationsWereMet())
	mockDB.AssertExpectations(t)
}

func TestGetSharedJobTasksSuccess(t *testing.T) {
	handler, mockDB, _ := createTestHandler()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT job_id, expires_at, revoked_at FROM job_share_links`).
		WithArgs("token-tasks").
		WillReturnRows(sqlmock.NewRows([]string{"job_id", "expires_at", "revoked_at"}).
			AddRow("job-tasks", nil, nil))

	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM tasks t`).
		WithArgs("job-tasks").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT\s+t.id, t.job_id, p.path`).
		WithArgs("job-tasks", 50, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "job_id", "path", "domain", "status",
			"status_code", "response_time", "cache_status",
			"second_response_time", "second_cache_status",
			"content_type", "error", "source_type", "source_url",
			"created_at", "started_at", "completed_at", "retry_count",
		}).AddRow(
			"task-1", "job-tasks", "/home", "example.com", "completed",
			200, 1500, "HIT", 800, "HIT",
			"text/html", nil, "crawler", "https://example.com",
			time.Now(), time.Now(), time.Now(), 0,
		))

	req := httptest.NewRequest(http.MethodGet, "/v1/shared/jobs/token-tasks/tasks", nil)
	rec := httptest.NewRecorder()

	handler.SharedJobHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var payload map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &payload)
	require.NoError(t, err)

	data, ok := payload["data"].(map[string]interface{})
	require.True(t, ok)
	tasks, ok := data["tasks"].([]interface{})
	require.True(t, ok)
	assert.Len(t, tasks, 1)

	assert.NoError(t, mock.ExpectationsWereMet())
	mockDB.AssertExpectations(t)
}

func TestExportSharedJobTasksSuccess(t *testing.T) {
	handler, mockDB, _ := createTestHandler()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT job_id, expires_at, revoked_at FROM job_share_links`).
		WithArgs("token-export").
		WillReturnRows(sqlmock.NewRows([]string{"job_id", "expires_at", "revoked_at"}).
			AddRow("job-export", nil, nil))

	// serveJobExport: tasks query
	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT\s+t.id, t.job_id, p.path`).
		WithArgs("job-export").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "job_id", "path", "domain",
			"status", "status_code", "response_time", "cache_status",
			"second_response_time", "second_cache_status",
			"content_type", "error", "source_type", "source_url",
			"created_at", "started_at", "completed_at", "retry_count",
		}).AddRow(
			"task-1", "job-export", "/home", "example.com",
			"completed", 200, 1400, "HIT",
			900, "HIT", "text/html", nil, "crawler", "https://example.com",
			time.Now(), time.Now(), time.Now(), 0,
		))

	// serveJobExport: job metadata query
	mockDB.On("GetDB").Return(sqlDB)
	mock.ExpectQuery(`SELECT d.name, j.status, j.created_at, j.completed_at FROM jobs j`).
		WithArgs("job-export").
		WillReturnRows(sqlmock.NewRows([]string{"name", "status", "created_at", "completed_at"}).
			AddRow("example.com", "completed", time.Now(), sql.NullTime{}))

	req := httptest.NewRequest(http.MethodGet, "/v1/shared/jobs/token-export/export?type=job", nil)
	rec := httptest.NewRecorder()

	handler.SharedJobHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var payload map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &payload)
	require.NoError(t, err)

	data, ok := payload["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "job-export", data["job_id"])
	assert.Equal(t, float64(1), data["total_tasks"])

	assert.NoError(t, mock.ExpectationsWereMet())
	mockDB.AssertExpectations(t)
}

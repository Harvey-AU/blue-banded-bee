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
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDatabaseHealthCheck_WithSqlMock tests DB health endpoint with sqlmock
func TestDatabaseHealthCheck_WithSqlMock(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]any
	}{
		{
			name: "healthy database",
			setupMock: func(mock sqlmock.Sqlmock) {
				// Expect ping to succeed
				mock.ExpectPing()
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]any{
				"status":  "healthy",
				"service": "postgresql",
			},
		},
		{
			name: "unhealthy database",
			setupMock: func(mock sqlmock.Sqlmock) {
				// Expect ping to fail
				mock.ExpectPing().WillReturnError(sql.ErrConnDone)
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody: map[string]any{
				"status":  "unhealthy",
				"service": "postgresql",
				"error":   "sql: connection is already closed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			mockDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
			require.NoError(t, err)
			defer mockDB.Close()

			// Setup expectations
			tt.setupMock(mock)

			// Create a mock DB wrapper
			database := &MockDBWithRealDB{
				sqlDB: mockDB,
			}

			// Create handler with mock DB
			handler := &Handler{
				DB: database,
			}

			// Make request
			req := httptest.NewRequest(http.MethodGet, "/health/db", nil)
			rec := httptest.NewRecorder()

			handler.DatabaseHealthCheck(rec, req)

			// Check status code
			assert.Equal(t, tt.expectedStatus, rec.Code)

			// Check response body
			var response map[string]any
			err = json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)

			for key, expectedValue := range tt.expectedBody {
				assert.Equal(t, expectedValue, response[key], "Key: %s", key)
			}

			// Verify all expectations were met
			err = mock.ExpectationsWereMet()
			assert.NoError(t, err)
		})
	}
}

// MockDBWithRealDB wraps a real *sql.DB for testing
type MockDBWithRealDB struct {
	sqlDB *sql.DB
}

func (m *MockDBWithRealDB) GetDB() *sql.DB {
	return m.sqlDB
}

func (m *MockDBWithRealDB) GetOrCreateUser(userID, email string, orgID *string) (*db.User, error) {
	return nil, nil
}

func (m *MockDBWithRealDB) GetJobStats(organisationID string, startDate, endDate *time.Time) (*db.JobStats, error) {
	return nil, nil
}

func (m *MockDBWithRealDB) GetJobActivity(organisationID string, startDate, endDate *time.Time) ([]db.ActivityPoint, error) {
	return nil, nil
}

func (m *MockDBWithRealDB) GetUserByWebhookToken(token string) (*db.User, error) {
	return nil, nil
}

// Satisfy additional DBClient methods not used in these tests
func (m *MockDBWithRealDB) GetUser(userID string) (*db.User, error) { return nil, nil }
func (m *MockDBWithRealDB) ResetSchema() error                      { return nil }
func (m *MockDBWithRealDB) ResetDataOnly() error                    { return nil }
func (m *MockDBWithRealDB) CreateUser(userID, email string, fullName *string, orgName string) (*db.User, *db.Organisation, error) {
	return nil, nil, nil
}
func (m *MockDBWithRealDB) GetOrganisation(organisationID string) (*db.Organisation, error) {
	return nil, nil
}
func (m *MockDBWithRealDB) ListJobs(organisationID string, limit, offset int, status, dateRange, timezone string) ([]db.JobWithDomain, int, error) {
	return nil, 0, nil
}
func (m *MockDBWithRealDB) ListJobsWithOffset(organisationID string, limit, offset int, status, dateRange string, tzOffsetMinutes int) ([]db.JobWithDomain, int, error) {
	return nil, 0, nil
}
func (m *MockDBWithRealDB) GetSlowPages(organisationID string, startDate, endDate *time.Time) ([]db.SlowPage, error) {
	return nil, nil
}
func (m *MockDBWithRealDB) GetExternalRedirects(organisationID string, startDate, endDate *time.Time) ([]db.ExternalRedirect, error) {
	return nil, nil
}

func (m *MockDBWithRealDB) CreateScheduler(ctx context.Context, scheduler *db.Scheduler) error {
	return nil
}

func (m *MockDBWithRealDB) GetScheduler(ctx context.Context, schedulerID string) (*db.Scheduler, error) {
	return nil, nil
}

func (m *MockDBWithRealDB) ListSchedulers(ctx context.Context, organisationID string) ([]*db.Scheduler, error) {
	return nil, nil
}

func (m *MockDBWithRealDB) UpdateScheduler(ctx context.Context, schedulerID string, updates *db.Scheduler) error {
	return nil
}

func (m *MockDBWithRealDB) DeleteScheduler(ctx context.Context, schedulerID string) error {
	return nil
}

func (m *MockDBWithRealDB) GetSchedulersReadyToRun(ctx context.Context, limit int) ([]*db.Scheduler, error) {
	return nil, nil
}

func (m *MockDBWithRealDB) UpdateSchedulerNextRun(ctx context.Context, schedulerID string, nextRun time.Time) error {
	return nil
}

func (m *MockDBWithRealDB) GetDomainNameByID(ctx context.Context, domainID int) (string, error) {
	return "", nil
}

func (m *MockDBWithRealDB) GetDomainNames(ctx context.Context, domainIDs []int) (map[int]string, error) {
	return nil, nil
}

func (m *MockDBWithRealDB) GetLastJobStartTimeForScheduler(ctx context.Context, schedulerID string) (*time.Time, error) {
	return nil, nil
}

func (m *MockDBWithRealDB) ListUserOrganisations(userID string) ([]db.UserOrganisation, error) {
	return nil, nil
}

func (m *MockDBWithRealDB) ValidateOrganisationMembership(userID, organisationID string) (bool, error) {
	return false, nil
}

func (m *MockDBWithRealDB) SetActiveOrganisation(userID, organisationID string) error {
	return nil
}

func (m *MockDBWithRealDB) GetEffectiveOrganisationID(user *db.User) string {
	if user.ActiveOrganisationID != nil && *user.ActiveOrganisationID != "" {
		return *user.ActiveOrganisationID
	}
	if user.OrganisationID != nil {
		return *user.OrganisationID
	}
	return ""
}

func (m *MockDBWithRealDB) CreateOrganisation(name string) (*db.Organisation, error) {
	return nil, nil
}

func (m *MockDBWithRealDB) AddOrganisationMember(userID, organisationID string) error {
	return nil
}

// Slack integration methods

func (m *MockDBWithRealDB) CreateSlackConnection(ctx context.Context, conn *db.SlackConnection) error {
	return nil
}

func (m *MockDBWithRealDB) GetSlackConnection(ctx context.Context, connectionID string) (*db.SlackConnection, error) {
	return nil, nil
}

func (m *MockDBWithRealDB) ListSlackConnections(ctx context.Context, organisationID string) ([]*db.SlackConnection, error) {
	return nil, nil
}

func (m *MockDBWithRealDB) DeleteSlackConnection(ctx context.Context, connectionID, organisationID string) error {
	return nil
}

func (m *MockDBWithRealDB) CreateSlackUserLink(ctx context.Context, link *db.SlackUserLink) error {
	return nil
}

func (m *MockDBWithRealDB) GetSlackUserLink(ctx context.Context, userID, connectionID string) (*db.SlackUserLink, error) {
	return nil, nil
}

func (m *MockDBWithRealDB) UpdateSlackUserLinkNotifications(ctx context.Context, userID, connectionID string, dmNotifications bool) error {
	return nil
}

func (m *MockDBWithRealDB) DeleteSlackUserLink(ctx context.Context, userID, connectionID string) error {
	return nil
}

func (m *MockDBWithRealDB) StoreSlackToken(ctx context.Context, connectionID, token string) error {
	return nil
}

func (m *MockDBWithRealDB) GetSlackToken(ctx context.Context, connectionID string) (string, error) {
	return "", nil
}

func (m *MockDBWithRealDB) ListNotifications(ctx context.Context, organisationID string, limit, offset int, unreadOnly bool) ([]*db.Notification, int, error) {
	return nil, 0, nil
}

func (m *MockDBWithRealDB) GetUnreadNotificationCount(ctx context.Context, organisationID string) (int, error) {
	return 0, nil
}

func (m *MockDBWithRealDB) MarkNotificationRead(ctx context.Context, notificationID, organisationID string) error {
	return nil
}

func (m *MockDBWithRealDB) MarkAllNotificationsRead(ctx context.Context, organisationID string) error {
	return nil
}

// Webflow integration methods
func (m *MockDBWithRealDB) CreateWebflowConnection(ctx context.Context, conn *db.WebflowConnection) error {
	return nil
}
func (m *MockDBWithRealDB) GetWebflowConnection(ctx context.Context, connectionID string) (*db.WebflowConnection, error) {
	return nil, nil
}
func (m *MockDBWithRealDB) ListWebflowConnections(ctx context.Context, organisationID string) ([]*db.WebflowConnection, error) {
	return nil, nil
}
func (m *MockDBWithRealDB) DeleteWebflowConnection(ctx context.Context, connectionID, organisationID string) error {
	return nil
}
func (m *MockDBWithRealDB) StoreWebflowToken(ctx context.Context, connectionID, token string) error {
	return nil
}
func (m *MockDBWithRealDB) GetWebflowToken(ctx context.Context, connectionID string) (string, error) {
	return "", nil
}

func (m *MockDBWithRealDB) UpsertPlatformOrgMapping(ctx context.Context, mapping *db.PlatformOrgMapping) error {
	return nil
}

func (m *MockDBWithRealDB) GetPlatformOrgMapping(ctx context.Context, platform, platformID string) (*db.PlatformOrgMapping, error) {
	return nil, nil
}

// Webflow site settings methods
func (m *MockDBWithRealDB) CreateOrUpdateSiteSetting(ctx context.Context, setting *db.WebflowSiteSetting) error {
	return nil
}
func (m *MockDBWithRealDB) GetSiteSetting(ctx context.Context, orgID, siteID string) (*db.WebflowSiteSetting, error) {
	return nil, nil
}
func (m *MockDBWithRealDB) GetSiteSettingByID(ctx context.Context, settingID string) (*db.WebflowSiteSetting, error) {
	return nil, nil
}
func (m *MockDBWithRealDB) ListConfiguredSiteSettings(ctx context.Context, orgID string) ([]*db.WebflowSiteSetting, error) {
	return nil, nil
}
func (m *MockDBWithRealDB) ListAllSiteSettings(ctx context.Context, orgID string) ([]*db.WebflowSiteSetting, error) {
	return nil, nil
}
func (m *MockDBWithRealDB) ListSiteSettingsByConnection(ctx context.Context, connectionID string) ([]*db.WebflowSiteSetting, error) {
	return nil, nil
}
func (m *MockDBWithRealDB) UpdateSiteSchedule(ctx context.Context, orgID, siteID string, intervalHours *int, schedulerID string) error {
	return nil
}
func (m *MockDBWithRealDB) UpdateSiteAutoPublish(ctx context.Context, orgID, siteID string, enabled bool, webhookID string) error {
	return nil
}
func (m *MockDBWithRealDB) DeleteSiteSetting(ctx context.Context, orgID, siteID string) error {
	return nil
}
func (m *MockDBWithRealDB) DeleteSiteSettingsByConnection(ctx context.Context, connectionID string) error {
	return nil
}
func (m *MockDBWithRealDB) GetSiteSettingBySiteID(ctx context.Context, orgID, webflowSiteID string) (*db.WebflowSiteSetting, error) {
	return nil, nil
}

// Google Analytics integration mock methods
func (m *MockDBWithRealDB) CreateGoogleConnection(ctx context.Context, conn *db.GoogleAnalyticsConnection) error {
	return nil
}

func (m *MockDBWithRealDB) GetGoogleConnection(ctx context.Context, connectionID string) (*db.GoogleAnalyticsConnection, error) {
	return nil, nil
}

func (m *MockDBWithRealDB) ListGoogleConnections(ctx context.Context, organisationID string) ([]*db.GoogleAnalyticsConnection, error) {
	return nil, nil
}

func (m *MockDBWithRealDB) DeleteGoogleConnection(ctx context.Context, connectionID, organisationID string) error {
	return nil
}

func (m *MockDBWithRealDB) UpdateGoogleConnectionStatus(ctx context.Context, connectionID, organisationID, status string) error {
	return nil
}

func (m *MockDBWithRealDB) StoreGoogleToken(ctx context.Context, connectionID, token string) error {
	return nil
}

func (m *MockDBWithRealDB) GetGoogleToken(ctx context.Context, connectionID string) (string, error) {
	return "", nil
}

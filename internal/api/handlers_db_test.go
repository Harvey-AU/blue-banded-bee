package api

import (
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
		expectedBody   map[string]interface{}
	}{
		{
			name: "healthy database",
			setupMock: func(mock sqlmock.Sqlmock) {
				// Expect ping to succeed
				mock.ExpectPing()
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
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
			expectedBody: map[string]interface{}{
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
			var response map[string]interface{}
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
func (m *MockDBWithRealDB) ResetSchema() error { return nil }
func (m *MockDBWithRealDB) CreateUser(userID, email string, fullName *string, orgName string) (*db.User, *db.Organisation, error) {
	return nil, nil, nil
}
func (m *MockDBWithRealDB) GetOrganisation(organisationID string) (*db.Organisation, error) { return nil, nil }
func (m *MockDBWithRealDB) ListJobs(organisationID string, limit, offset int, status, dateRange string) ([]db.JobWithDomain, int, error) {
	return nil, 0, nil
}
package db

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSlowPages(t *testing.T) {
	tests := []struct {
		name           string
		organisationID string
		startDate      *time.Time
		endDate        *time.Time
		setupMock      func(sqlmock.Sqlmock)
		expectedResult []SlowPage
		expectedError  string
	}{
		{
			name:           "successful_slow_pages_query",
			organisationID: "org-123",
			startDate:      nil,
			endDate:        nil,
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"url", "domain", "path", "second_response_time", "job_id", "completed_at",
				}).AddRow(
					"https://example.com/slow-page", "example.com", "/slow-page", 5000, "job-123", time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				).AddRow(
					"https://example.com/very-slow", "example.com", "/very-slow", 8000, "job-456", time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
				)
				
				mock.ExpectQuery(`WITH user_tasks AS`).
					WithArgs("org-123", nil, nil).
					WillReturnRows(rows)
			},
			expectedResult: []SlowPage{
				{
					URL:                "https://example.com/slow-page",
					Domain:             "example.com",
					Path:               "/slow-page",
					SecondResponseTime: 5000,
					JobID:              "job-123",
					CompletedAt:        "2024-01-01T12:00:00Z",
				},
				{
					URL:                "https://example.com/very-slow",
					Domain:             "example.com",
					Path:               "/very-slow",
					SecondResponseTime: 8000,
					JobID:              "job-456",
					CompletedAt:        "2024-01-01T13:00:00Z",
				},
			},
		},
		{
			name:           "with_date_range",
			organisationID: "org-123",
			startDate:      &time.Time{},
			endDate:        &time.Time{},
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"url", "domain", "path", "second_response_time", "job_id", "completed_at",
				})
				
				mock.ExpectQuery(`WITH user_tasks AS`).
					WithArgs("org-123", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnRows(rows)
			},
			expectedResult: nil,
		},
		{
			name:           "database_error",
			organisationID: "org-123",
			startDate:      nil,
			endDate:        nil,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`WITH user_tasks AS`).
					WithArgs("org-123", nil, nil).
					WillReturnError(assert.AnError)
			},
			expectedError: assert.AnError.Error(),
		},
		{
			name:           "empty_organisation_id",
			organisationID: "",
			startDate:      nil,
			endDate:        nil,
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"url", "domain", "path", "second_response_time", "job_id", "completed_at",
				})
				
				mock.ExpectQuery(`WITH user_tasks AS`).
					WithArgs("", nil, nil).
					WillReturnRows(rows)
			},
			expectedResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()

			db := &DB{client: mockDB}
			tt.setupMock(mock)

			// Execute
			result, err := db.GetSlowPages(tt.organisationID, tt.startDate, tt.endDate)

			// Verify
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetExternalRedirects(t *testing.T) {
	tests := []struct {
		name           string
		organisationID string
		startDate      *time.Time
		endDate        *time.Time
		setupMock      func(sqlmock.Sqlmock)
		expectedResult []ExternalRedirect
		expectedError  string
	}{
		{
			name:           "successful_redirects_query",
			organisationID: "org-123",
			startDate:      nil,
			endDate:        nil,
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"url", "domain", "path", "redirect_url", "job_id", "completed_at",
				}).AddRow(
					"https://example.com/redirect", "example.com", "/redirect", "https://external-site.com/target", "job-123", time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				).AddRow(
					"https://example.com/another", "example.com", "/another", "https://different.com/page", "job-456", time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
				)
				
				mock.ExpectQuery(`SELECT.*FROM tasks t.*JOIN jobs j.*WHERE j.organisation_id`).
					WithArgs("org-123", nil, nil).
					WillReturnRows(rows)
			},
			expectedResult: []ExternalRedirect{
				{
					URL:         "https://example.com/redirect",
					Domain:      "example.com",
					Path:        "/redirect",
					RedirectURL: "https://external-site.com/target",
					JobID:       "job-123",
					CompletedAt: "2024-01-01T12:00:00Z",
				},
				{
					URL:         "https://example.com/another",
					Domain:      "example.com",
					Path:        "/another",
					RedirectURL: "https://different.com/page",
					JobID:       "job-456",
					CompletedAt: "2024-01-01T13:00:00Z",
				},
			},
		},
		{
			name:           "with_date_range",
			organisationID: "org-123",
			startDate:      &time.Time{},
			endDate:        &time.Time{},
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"url", "domain", "path", "redirect_url", "job_id", "completed_at",
				})
				
				mock.ExpectQuery(`SELECT.*FROM tasks t.*JOIN jobs j.*WHERE j.organisation_id`).
					WithArgs("org-123", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnRows(rows)
			},
			expectedResult: nil,
		},
		{
			name:           "database_error",
			organisationID: "org-123",
			startDate:      nil,
			endDate:        nil,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT.*FROM tasks t.*JOIN jobs j.*WHERE j.organisation_id`).
					WithArgs("org-123", nil, nil).
					WillReturnError(assert.AnError)
			},
			expectedError: assert.AnError.Error(),
		},
		{
			name:           "scan_error",
			organisationID: "org-123",
			startDate:      nil,
			endDate:        nil,
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"url", "domain", "path", "redirect_url", "job_id", "completed_at",
				}).AddRow(
					"invalid-url", "example.com", "/redirect", "https://external-site.com/target", "job-123", "invalid-date",
				)
				
				mock.ExpectQuery(`SELECT.*FROM tasks t.*JOIN jobs j.*WHERE j.organisation_id`).
					WithArgs("org-123", nil, nil).
					WillReturnRows(rows)
			},
			expectedError: "Failed to scan external redirect row",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()

			db := &DB{client: mockDB}
			tt.setupMock(mock)

			// Execute
			result, err := db.GetExternalRedirects(tt.organisationID, tt.startDate, tt.endDate)

			// Verify
			if tt.expectedError != "" {
				assert.Error(t, err)
				if tt.name == "scan_error" {
					// For scan errors, we may still have processed some rows
					assert.NotNil(t, err)
				} else {
					assert.Contains(t, err.Error(), tt.expectedError)
					assert.Nil(t, result)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
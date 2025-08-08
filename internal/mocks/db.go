package mocks

import (
	"context"
	"database/sql"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/mock"
)

// MockDB is a mock implementation of the database interface
type MockDB struct {
	mock.Mock
}

// GetDB mocks the GetDB method to return underlying *sql.DB
func (m *MockDB) GetDB() *sql.DB {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*sql.DB)
}

// Close mocks the Close method for database connection cleanup
func (m *MockDB) Close() error {
	args := m.Called()
	return args.Error(0)
}

// GetConfig mocks the GetConfig method to return database configuration
func (m *MockDB) GetConfig() *db.Config {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*db.Config)
}

// RecalculateJobStats mocks the RecalculateJobStats method
func (m *MockDB) RecalculateJobStats(ctx context.Context, jobID string) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

// ResetSchema mocks the ResetSchema method for testing database resets
func (m *MockDB) ResetSchema() error {
	args := m.Called()
	return args.Error(0)
}
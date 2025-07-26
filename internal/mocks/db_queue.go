package mocks

import (
	"context"
	"database/sql"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/mock"
)

// MockDbQueue is a mock implementation of DbQueueProvider
type MockDbQueue struct {
	mock.Mock
}

// Execute mocks the Execute method
func (m *MockDbQueue) Execute(ctx context.Context, fn func(*sql.Tx) error) error {
	args := m.Called(ctx, fn)
	
	// If the test wants to execute the function, it can provide a nil error
	// This allows us to test the transaction logic
	if args.Error(0) == nil && fn != nil {
		// Create a dummy transaction for the function to use
		// In real tests, we might want to pass a mock transaction
		return fn(nil)
	}
	
	return args.Error(0)
}

// EnqueueURLs mocks the EnqueueURLs method
func (m *MockDbQueue) EnqueueURLs(ctx context.Context, jobID string, pages []db.Page, sourceType string, sourceURL string) error {
	args := m.Called(ctx, jobID, pages, sourceType, sourceURL)
	return args.Error(0)
}

// CleanupStuckJobs mocks the CleanupStuckJobs method
func (m *MockDbQueue) CleanupStuckJobs(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
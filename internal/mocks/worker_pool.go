package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockWorkerPool is a mock implementation of WorkerPool
type MockWorkerPool struct {
	mock.Mock
}

// Start mocks the Start method
func (m *MockWorkerPool) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// Stop mocks the Stop method
func (m *MockWorkerPool) Stop() error {
	args := m.Called()
	return args.Error(0)
}

// CancelJobTasks mocks the CancelJobTasks method
func (m *MockWorkerPool) CancelJobTasks(jobID string) error {
	args := m.Called(jobID)
	return args.Error(0)
}
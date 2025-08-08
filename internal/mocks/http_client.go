package mocks

import (
	"net/http"
	"time"

	"github.com/stretchr/testify/mock"
)

// MockHTTPClient is a mock implementation of HTTP client interface
type MockHTTPClient struct {
	mock.Mock
}

// Do mocks the Do method of http.Client
func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// Get mocks the Get method of http.Client
func (m *MockHTTPClient) Get(url string) (*http.Response, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// Post mocks the Post method of http.Client
func (m *MockHTTPClient) Post(url, contentType string, body interface{}) (*http.Response, error) {
	args := m.Called(url, contentType, body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// PostForm mocks the PostForm method of http.Client
func (m *MockHTTPClient) PostForm(url string, data interface{}) (*http.Response, error) {
	args := m.Called(url, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// Head mocks the Head method of http.Client
func (m *MockHTTPClient) Head(url string) (*http.Response, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// MockRoundTripper is a mock implementation of http.RoundTripper interface
type MockRoundTripper struct {
	mock.Mock
}

// RoundTrip mocks the RoundTrip method of http.RoundTripper
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// HTTPClientInterface defines the interface for HTTP client operations
type HTTPClientInterface interface {
	Do(req *http.Request) (*http.Response, error)
	Get(url string) (*http.Response, error)
	Post(url, contentType string, body interface{}) (*http.Response, error)
	PostForm(url string, data interface{}) (*http.Response, error)
	Head(url string) (*http.Response, error)
}

// MockTransport is a mock implementation for HTTP transport testing
type MockTransport struct {
	mock.Mock
}

// RoundTrip implements http.RoundTripper interface for transport mocking
func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// NewMockHTTPClientWithTimeout creates a mock HTTP client with timeout configuration
func NewMockHTTPClientWithTimeout(timeout time.Duration) *MockHTTPClient {
	client := &MockHTTPClient{}
	// Pre-configure common expectations if needed
	return client
}

// CreateMockResponse creates a mock HTTP response for testing
func CreateMockResponse(statusCode int, body string, headers map[string]string) *http.Response {
	resp := &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
	}
	
	// Add headers if provided
	for key, value := range headers {
		resp.Header.Set(key, value)
	}
	
	return resp
}
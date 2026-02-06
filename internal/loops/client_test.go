package loops

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient creates a client pointed at a test server.
func newTestClient(handler http.HandlerFunc) (*Client, *httptest.Server) {
	server := httptest.NewServer(handler)
	client := New("test-api-key")
	// Override base URL by replacing the httpClient transport
	client.httpClient = server.Client()
	return client, server
}

// overrideBaseURL temporarily replaces the package-level baseURL for testing.
// Returns a cleanup function to restore the original.
func overrideBaseURL(url string) func() {
	// We can't override a const, so we'll use a helper approach:
	// The test server URL is used directly via a custom RoundTripper.
	return func() {}
}

// testRoundTripper redirects all requests to the test server.
type testRoundTripper struct {
	serverURL string
	transport http.RoundTripper
}

func (t *testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace the Loops API base URL with our test server
	req.URL.Scheme = "http"
	req.URL.Host = t.serverURL
	return t.transport.RoundTrip(req)
}

// newClientWithServer creates a client that routes requests to a test server.
func newClientWithServer(handler http.HandlerFunc) (*Client, *httptest.Server) {
	server := httptest.NewServer(handler)
	client := New("test-api-key")
	client.httpClient = &http.Client{
		Transport: &testRoundTripper{
			serverURL: server.Listener.Addr().String(),
			transport: http.DefaultTransport,
		},
	}
	return client, server
}

func TestSendTransactional_Success(t *testing.T) {
	var receivedBody map[string]any
	var receivedAuth string

	client, server := newClientWithServer(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/transactional", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success": true}`))
	})
	defer server.Close()

	err := client.SendTransactional(context.Background(), &TransactionalRequest{
		Email:           "user@example.com",
		TransactionalID: "tmpl_123",
		DataVariables:   map[string]any{"name": "Test"},
	})

	require.NoError(t, err)
	assert.Equal(t, "Bearer test-api-key", receivedAuth)
	assert.Equal(t, "user@example.com", receivedBody["email"])
	assert.Equal(t, "tmpl_123", receivedBody["transactionalId"])
}

func TestSendTransactional_WithIdempotencyKey(t *testing.T) {
	var receivedKey string

	client, server := newClientWithServer(func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success": true}`))
	})
	defer server.Close()

	err := client.SendTransactional(context.Background(), &TransactionalRequest{
		Email:           "user@example.com",
		TransactionalID: "tmpl_123",
		IdempotencyKey:  "unique-key-abc",
	})

	require.NoError(t, err)
	assert.Equal(t, "unique-key-abc", receivedKey)
}

func TestSendTransactional_APIError(t *testing.T) {
	client, server := newClientWithServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"success": false, "message": "Invalid email address"}`))
	})
	defer server.Close()

	err := client.SendTransactional(context.Background(), &TransactionalRequest{
		Email:           "bad-email",
		TransactionalID: "tmpl_123",
	})

	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "Invalid email address", apiErr.Message)
}

func TestSendTransactional_NotFound(t *testing.T) {
	client, server := newClientWithServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success": false, "message": "Transactional email not found"}`))
	})
	defer server.Close()

	err := client.SendTransactional(context.Background(), &TransactionalRequest{
		Email:           "user@example.com",
		TransactionalID: "tmpl_nonexistent",
	})

	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
}

func TestSendTransactional_RateLimited(t *testing.T) {
	client, server := newClientWithServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message": "Rate limit exceeded"}`))
	})
	defer server.Close()

	err := client.SendTransactional(context.Background(), &TransactionalRequest{
		Email:           "user@example.com",
		TransactionalID: "tmpl_123",
	})

	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
}

func TestSendEvent_Success(t *testing.T) {
	var receivedBody map[string]any

	client, server := newClientWithServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/events/send", r.URL.Path)

		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success": true}`))
	})
	defer server.Close()

	err := client.SendEvent(context.Background(), &EventRequest{
		Email:     "user@example.com",
		EventName: "job_completed",
		EventProperties: map[string]any{
			"job_id": "abc123",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "user@example.com", receivedBody["email"])
	assert.Equal(t, "job_completed", receivedBody["eventName"])
}

func TestCreateContact_Success(t *testing.T) {
	var receivedBody map[string]any

	client, server := newClientWithServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/contacts/create", r.URL.Path)

		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success": true}`))
	})
	defer server.Close()

	err := client.CreateContact(context.Background(), &ContactRequest{
		Email:     "user@example.com",
		FirstName: "Test",
		LastName:  "User",
	})

	require.NoError(t, err)
	assert.Equal(t, "user@example.com", receivedBody["email"])
	assert.Equal(t, "Test", receivedBody["firstName"])
}

func TestUpdateContact_Success(t *testing.T) {
	client, server := newClientWithServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/api/v1/contacts/update", r.URL.Path)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success": true}`))
	})
	defer server.Close()

	err := client.UpdateContact(context.Background(), &ContactRequest{
		Email:     "user@example.com",
		FirstName: "Updated",
	})

	require.NoError(t, err)
}

func TestContextCancellation(t *testing.T) {
	client, server := newClientWithServer(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response — context should cancel before this returns
		<-r.Context().Done()
	})
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := client.SendTransactional(ctx, &TransactionalRequest{
		Email:           "user@example.com",
		TransactionalID: "tmpl_123",
	})

	require.Error(t, err)
}

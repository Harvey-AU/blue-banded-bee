package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// NOTE: Webflow Client ID/Secret should be loaded from config/env
func getWebflowClientID() string {
	return os.Getenv("WEBFLOW_CLIENT_ID")
}

func getWebflowClientSecret() string {
	return os.Getenv("WEBFLOW_CLIENT_SECRET")
}

func getWebflowRedirectURI() string {
	return getAppURL() + "/v1/integrations/webflow/callback"
}

// WebflowTokenResponse represents the response from Webflow's token endpoint
type WebflowTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	// Webflow doesn't always return expires_in or refresh_token in all flows, but normally standard OAuth
}

// InitiateWebflowOAuth starts the OAuth flow
func (h *Handler) InitiateWebflowOAuth(w http.ResponseWriter, r *http.Request) {
	logger := loggerWithRequest(r)

	if r.Method != http.MethodPost {
		MethodNotAllowed(w, r)
		return
	}

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		logger.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
		InternalError(w, r, err)
		return
	}

	if user.OrganisationID == nil {
		BadRequest(w, r, "User must belong to an organisation")
		return
	}

	if getWebflowClientID() == "" {
		logger.Error().Msg("WEBFLOW_CLIENT_ID not configured")
		InternalError(w, r, fmt.Errorf("webflow integration not configured"))
		return
	}

	if getOAuthStateSecret() == "" {
		logger.Error().Msg("SUPABASE_JWT_SECRET not configured for OAuth state signing")
		InternalError(w, r, fmt.Errorf("webflow integration not configured"))
		return
	}

	// Sign state with JWT Secret
	state, err := h.generateOAuthState(userClaims.UserID, *user.OrganisationID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to generate OAuth state")
		InternalError(w, r, err)
		return
	}

	// Scopes: workspaces:read (for name), sites:read, sites:write (for webhooks), cms:read
	scopes := "workspaces:read sites:read sites:write cms:read"

	// Build Webflow OAuth URL
	authURL := fmt.Sprintf(
		"https://webflow.com/oauth/authorize?client_id=%s&response_type=code&scope=%s&redirect_uri=%s&state=%s",
		url.QueryEscape(getWebflowClientID()),
		url.QueryEscape(scopes),
		url.QueryEscape(getWebflowRedirectURI()),
		url.QueryEscape(state), // Webflow returns this state in callback
	)

	WriteSuccess(w, r, map[string]string{"auth_url": authURL}, "Redirect to this URL to connect Webflow")
}

// HandleWebflowOAuthCallback processes the OAuth callback from Webflow
func (h *Handler) HandleWebflowOAuthCallback(w http.ResponseWriter, r *http.Request) {
	logger := loggerWithRequest(r)

	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	code := r.URL.Query().Get("code")
	stateParam := r.URL.Query().Get("state")
	errParam := r.URL.Query().Get("error")

	if errParam != "" {
		logger.Warn().Str("error", errParam).Msg("Webflow OAuth denied")
		h.redirectToDashboardWithError(w, r, "Webflow", "Webflow connection was cancelled")
		return
	}

	if code == "" || stateParam == "" {
		BadRequest(w, r, "Missing code or state parameter")
		return
	}

	// Validate state
	state, err := h.validateOAuthState(stateParam)
	if err != nil {
		logger.Warn().Err(err).Msg("Invalid OAuth state")
		h.redirectToDashboardWithError(w, r, "Webflow", "Invalid or expired state")
		return
	}

	// Exchange code for access token
	tokenResp, err := h.exchangeWebflowCode(code)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to exchange Webflow OAuth code")
		h.redirectToDashboardWithError(w, r, "Webflow", "Failed to connect to Webflow")
		return
	}

	// Fetch user/workspace info from Webflow
	authInfo, err := h.fetchWebflowAuthInfo(r.Context(), tokenResp.AccessToken)
	if err != nil {
		// Log but don't fail - we can still create the connection with empty values
		logger.Warn().Err(err).Msg("Failed to fetch Webflow auth info, proceeding with empty values")
	}

	// Extract user and workspace info
	authedUserID := ""
	workspaceID := ""
	workspaceName := ""
	if authInfo != nil {
		authedUserID = authInfo.UserID
		workspaceName = authInfo.WorkspaceName
		if len(authInfo.WorkspaceIDs) > 0 {
			workspaceID = authInfo.WorkspaceIDs[0] // Use first workspace
		}
	}

	now := time.Now().UTC()
	conn := &db.WebflowConnection{
		ID:                 uuid.New().String(),
		OrganisationID:     state.OrgID,
		AuthedUserID:       authedUserID,
		WebflowWorkspaceID: workspaceID,
		WorkspaceName:      workspaceName,
		InstallingUserID:   state.UserID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := h.DB.CreateWebflowConnection(r.Context(), conn); err != nil {
		logger.Error().Err(err).Msg("Failed to save Webflow connection")
		h.redirectToDashboardWithError(w, r, "Webflow", "Failed to save connection")
		return
	}

	// Store access token in Supabase Vault
	if err := h.DB.StoreWebflowToken(r.Context(), conn.ID, tokenResp.AccessToken); err != nil {
		logger.Error().Err(err).Msg("Failed to store access token in vault")
		h.redirectToDashboardWithError(w, r, "Webflow", "Failed to secure connection")
		return
	}

	// AUTO-REGISTER WEBHOOKS (Run on Publish)
	// 1. Get Sites
	// 2. For each site, register 'site_publish' webhook pointing to /v1/webhooks/webflow/{token}?
	// Wait, existing webhook handler uses a "user" token.
	// We should probably register a webhook pointing to a new generic handler or reuse the existing logic if we can map it.
	// Ideally: webhook url = https://app.../v1/webhooks/webflow-oauth/{connID}
	// Updated plan said: "Automatically call Webflow API to register site_publish webhook"
	go h.registerWebflowWebhooksSafe(state.UserID, tokenResp.AccessToken)

	logger.Info().
		Str("organisation_id", state.OrgID).
		Str("webflow_workspace_id", workspaceID).
		Str("webflow_user_id", authedUserID).
		Msg("Webflow connection established")

	// Redirect to dashboard with success
	h.redirectToDashboardWithSuccess(w, r, "Webflow", "Webflow Connection", conn.ID)
}

func (h *Handler) exchangeWebflowCode(code string) (*WebflowTokenResponse, error) {
	values := url.Values{}
	values.Set("client_id", getWebflowClientID())
	values.Set("client_secret", getWebflowClientSecret())
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	values.Set("redirect_uri", getWebflowRedirectURI())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm("https://api.webflow.com/oauth/access_token", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webflow API returned status: %d", resp.StatusCode)
	}

	var tokenResp WebflowTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// WebflowAuthInfo contains user and workspace info from Webflow's token introspection
type WebflowAuthInfo struct {
	UserID        string
	WorkspaceIDs  []string
	WorkspaceName string // Display name of the first workspace
}

// fetchWebflowAuthInfo calls Webflow's token introspect endpoint to get user/workspace info
func (h *Handler) fetchWebflowAuthInfo(ctx context.Context, token string) (*WebflowAuthInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.webflow.com/v2/token/introspect", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create introspect request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call introspect endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("introspect endpoint returned status: %d", resp.StatusCode)
	}

	var introspectResp struct {
		Authorization struct {
			AuthorizedTo struct {
				UserID       string   `json:"userId"`
				WorkspaceIDs []string `json:"workspaceIds"`
			} `json:"authorizedTo"`
		} `json:"authorization"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&introspectResp); err != nil {
		return nil, fmt.Errorf("failed to decode introspect response: %w", err)
	}

	authInfo := &WebflowAuthInfo{
		UserID:       introspectResp.Authorization.AuthorizedTo.UserID,
		WorkspaceIDs: introspectResp.Authorization.AuthorizedTo.WorkspaceIDs,
	}

	// Fetch workspace name if we have a workspace ID
	if len(authInfo.WorkspaceIDs) > 0 {
		workspaceName, err := h.fetchWebflowWorkspaceName(ctx, token, authInfo.WorkspaceIDs[0])
		if err != nil {
			// Log but don't fail - workspace name is nice to have
			log.Warn().Err(err).Str("workspace_id", authInfo.WorkspaceIDs[0]).Msg("Failed to fetch workspace name")
		} else {
			authInfo.WorkspaceName = workspaceName
		}
	}

	return authInfo, nil
}

// fetchWebflowWorkspaceName fetches the display name of a Webflow workspace
// Uses the list endpoint and finds the matching workspace by ID
func (h *Handler) fetchWebflowWorkspaceName(ctx context.Context, token, workspaceID string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.webflow.com/v2/workspaces", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create workspaces request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call workspaces endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("workspaces endpoint returned status: %d", resp.StatusCode)
	}

	var listResp struct {
		Workspaces []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"workspaces"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return "", fmt.Errorf("failed to decode workspaces response: %w", err)
	}

	// Find workspace by ID
	for _, ws := range listResp.Workspaces {
		if ws.ID == workspaceID {
			return ws.DisplayName, nil
		}
	}

	// If no match, return first workspace name if available
	if len(listResp.Workspaces) > 0 {
		return listResp.Workspaces[0].DisplayName, nil
	}

	return "", fmt.Errorf("no workspaces found")
}

func (h *Handler) registerWebflowWebhooksSafe(userID, token string) {
	// Simple wrapper to run in background and log errors
	logger := log.With().Str("user_id", userID).Logger()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	logger.Info().Msg("Starting Webflow webhook registration")

	// 1. Get user to get webhook token
	user, err := h.DB.GetUser(userID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get user for webhook registration")
		return
	}

	if user.WebhookToken == nil || *user.WebhookToken == "" {
		logger.Warn().Msg("User has no webhook token, cannot register webhooks")
		return
	}

	webhookURL := fmt.Sprintf("%s/v1/webhooks/webflow/%s", getAppURL(), *user.WebhookToken)

	// 2. List sites
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.webflow.com/v2/sites", nil)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create request for Webflow sites")
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to list Webflow sites")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error().Int("status", resp.StatusCode).Msg("Webflow API returned error listing sites")
		return
	}

	var sitesResp struct {
		Sites []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"sites"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sitesResp); err != nil {
		logger.Error().Err(err).Msg("Failed to decode Webflow sites response")
		return
	}

	// 3. Register webhook for each site
	for _, site := range sitesResp.Sites {
		h.registerSiteWebhook(ctx, site.ID, token, webhookURL, logger)
	}
}

func (h *Handler) registerSiteWebhook(ctx context.Context, siteID, token, webhookURL string, logger zerolog.Logger) {
	payload := map[string]string{
		"triggerType": "site_publish",
		"url":         webhookURL,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		logger.Error().Err(err).Str("site_id", siteID).Msg("Failed to marshal webhook payload")
		return
	}

	reqURL := fmt.Sprintf("https://api.webflow.com/v2/sites/%s/webhooks", siteID)
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(body))
	if err != nil {
		logger.Error().Err(err).Str("site_id", siteID).Msg("Failed to create webhook registration request")
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error().Err(err).Str("site_id", siteID).Msg("Failed to register webhook for site")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		logger.Info().Str("site_id", siteID).Msg("Successfully registered Webflow webhook")
	} else {
		// Log error but continue for other sites
		logger.Warn().Int("status", resp.StatusCode).Str("site_id", siteID).Msg("Webflow API returned error registering webhook")
	}
}

// WebflowConnectionResponse represents a Webflow connection in API responses
type WebflowConnectionResponse struct {
	ID                 string `json:"id"`
	WebflowWorkspaceID string `json:"webflow_workspace_id,omitempty"`
	WorkspaceName      string `json:"workspace_name,omitempty"`
	CreatedAt          string `json:"created_at"`
}

// WebflowConnectionsHandler handles requests to /v1/integrations/webflow
func (h *Handler) WebflowConnectionsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listWebflowConnections(w, r)
	case http.MethodPost:
		h.InitiateWebflowOAuth(w, r)
	default:
		MethodNotAllowed(w, r)
	}
}

// WebflowConnectionHandler handles requests to /v1/integrations/webflow/:id
func (h *Handler) WebflowConnectionHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/integrations/webflow/")
	if path == "" {
		BadRequest(w, r, "Connection ID is required")
		return
	}

	// Handle callback separately (no auth required)
	if path == "callback" {
		if r.Method == http.MethodGet {
			h.HandleWebflowOAuthCallback(w, r)
			return
		}
		MethodNotAllowed(w, r)
		return
	}

	connectionID := strings.Split(path, "/")[0]
	if _, err := uuid.Parse(connectionID); err != nil {
		BadRequest(w, r, "Invalid connection ID format")
		return
	}

	switch r.Method {
	case http.MethodDelete:
		h.deleteWebflowConnection(w, r, connectionID)
	default:
		MethodNotAllowed(w, r)
	}
}

// listWebflowConnections lists all Webflow connections for the user's organisation
func (h *Handler) listWebflowConnections(w http.ResponseWriter, r *http.Request) {
	logger := loggerWithRequest(r)

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		logger.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
		InternalError(w, r, err)
		return
	}

	if user.OrganisationID == nil {
		WriteSuccess(w, r, []WebflowConnectionResponse{}, "No organisation")
		return
	}

	connections, err := h.DB.ListWebflowConnections(r.Context(), *user.OrganisationID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to list Webflow connections")
		InternalError(w, r, err)
		return
	}

	response := make([]WebflowConnectionResponse, 0, len(connections))
	for _, conn := range connections {
		response = append(response, WebflowConnectionResponse{
			ID:                 conn.ID,
			WebflowWorkspaceID: conn.WebflowWorkspaceID,
			WorkspaceName:      conn.WorkspaceName,
			CreatedAt:          conn.CreatedAt.Format(time.RFC3339),
		})
	}

	WriteSuccess(w, r, response, "")
}

// deleteWebflowConnection deletes a Webflow connection
func (h *Handler) deleteWebflowConnection(w http.ResponseWriter, r *http.Request, connectionID string) {
	logger := loggerWithRequest(r)

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		logger.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
		InternalError(w, r, err)
		return
	}

	if user.OrganisationID == nil {
		Forbidden(w, r, "User must belong to an organisation")
		return
	}

	err = h.DB.DeleteWebflowConnection(r.Context(), connectionID, *user.OrganisationID)
	if err != nil {
		if errors.Is(err, db.ErrWebflowConnectionNotFound) {
			NotFound(w, r, "Webflow connection not found")
			return
		}
		logger.Error().Err(err).Msg("Failed to delete Webflow connection")
		InternalError(w, r, err)
		return
	}

	logger.Info().Str("connection_id", connectionID).Msg("Webflow connection deleted")
	WriteNoContent(w, r)
}

// Trigger deployment

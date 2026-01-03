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

// Webflow OAuth credentials loaded from environment variables
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

	// Scopes: authorized_user:read (for user name), sites:read, sites:write (for webhooks), cms:read
	// Note: workspaces:read is Enterprise-only, so we use authorized_user:read to identify the connection
	scopes := "authorized_user:read sites:read sites:write cms:read"

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
	displayName := ""
	if authInfo != nil {
		authedUserID = authInfo.UserID
		displayName = authInfo.DisplayName // User's name or email
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
		WorkspaceName:      displayName, // User's name or email from authorized_by endpoint
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

	// Register site_publish webhooks in background for "Run on Publish" feature
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

// WebflowAuthInfo contains user and workspace info from Webflow's API
type WebflowAuthInfo struct {
	UserID       string
	WorkspaceIDs []string
	// User info from authorized_by endpoint
	UserEmail     string
	UserFirstName string
	UserLastName  string
	DisplayName   string // Combined name for display (e.g., "Simon Chua" or email)
}

// fetchWebflowAuthInfo calls Webflow's API to get user and workspace info
func (h *Handler) fetchWebflowAuthInfo(ctx context.Context, token string) (*WebflowAuthInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// First, get authorisation info (workspace IDs)
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

	// Fetch user info from authorized_by endpoint
	userInfo, err := h.fetchWebflowUserInfo(ctx, client, token)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to fetch Webflow user info")
	} else {
		authInfo.UserEmail = userInfo.Email
		authInfo.UserFirstName = userInfo.FirstName
		authInfo.UserLastName = userInfo.LastName
		authInfo.DisplayName = userInfo.DisplayName
	}

	return authInfo, nil
}

// WebflowUserInfo contains user details from the authorized_by endpoint
type WebflowUserInfo struct {
	ID          string
	Email       string
	FirstName   string
	LastName    string
	DisplayName string
}

// fetchWebflowUserInfo fetches the authorising user's info
func (h *Handler) fetchWebflowUserInfo(ctx context.Context, client *http.Client, token string) (*WebflowUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.webflow.com/v2/token/authorized_by", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create authorized_by request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call authorized_by endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authorized_by endpoint returned status: %d", resp.StatusCode)
	}

	var userResp struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		return nil, fmt.Errorf("failed to decode authorized_by response: %w", err)
	}

	userInfo := &WebflowUserInfo{
		ID:        userResp.ID,
		Email:     userResp.Email,
		FirstName: userResp.FirstName,
		LastName:  userResp.LastName,
	}

	// Build display name: prefer "FirstName LastName", fall back to email
	if userResp.FirstName != "" || userResp.LastName != "" {
		userInfo.DisplayName = strings.TrimSpace(userResp.FirstName + " " + userResp.LastName)
	} else if userResp.Email != "" {
		userInfo.DisplayName = userResp.Email
	} else {
		userInfo.DisplayName = "Webflow User"
	}

	return userInfo, nil
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
		BadRequest(w, r, "User must belong to an organisation")
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

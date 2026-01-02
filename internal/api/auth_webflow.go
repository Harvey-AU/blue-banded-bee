package api

import (
	"context"
	"encoding/json"
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
	// Fallback or env var, should ideally match what was registered in Webflow App
	// TODO: Make this dynamic based on environment if needed, but Webflow requires exact match
	// For now assuming PROD URL structure or LOCAL based on APP_URL
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "http://localhost:8080"
	}
	return fmt.Sprintf("%s/v1/integrations/webflow/callback", appURL)
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

	// Sign state with JWT Secret (reusing Slack state logic helper if possible, or duplicate)
	// Replicating generateOAuthState logic from slack.go to avoid coupling or cyclic deps if not shared
	state, err := h.generateOAuthState(userClaims.UserID, *user.OrganisationID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to generate OAuth state")
		InternalError(w, r, err)
		return
	}

	// Scopes: sites:read, sites:write (for webhooks), cms:read
	scopes := "sites:read sites:write cms:read"

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

	// Get Authorized User/Workspace Info?
	// Webflow API: GET /authenticated_user (requires authorized_user:read which we might not have asked for?)
	// Actually, we usually need to call GET /sites to see what we have access to contextually.
	// For now, we will store the connection.

	// Get Auth User ID or Workspace ID if available (Webflow might not return this in token resp)
	// We will assume 1 connection per org for MVP, but let's query /sites to get a site/workspace context if possible.
	// Or just store generic connection.

	now := time.Now().UTC()
	conn := &db.WebflowConnection{
		ID:                 uuid.New().String(),
		OrganisationID:     state.OrgID,
		AuthedUserID:       "unknown", // Populate if we call user info
		WebflowWorkspaceID: "unknown",
		InstallingUserID:   state.UserID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	// Optional: Fetch User Info or Sites to populate metadata
	// ... logic to call Webflow API ...

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
	// redirect_uri is optional in some flows but safer to include? check docs. explicit is better.
	// Webflow docs say: redirect_uri is required if it was included in the authorization request.
	// values.Set("redirect_uri", getWebflowRedirectURI())

	resp, err := http.PostForm("https://api.webflow.com/oauth/access_token", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webflow api returned status: %d", resp.StatusCode)
	}

	var tokenResp WebflowTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

func (h *Handler) registerWebflowWebhooksSafe(userID, token string) {
	// Simple wrapper to run in background and log errors
	logger := log.With().Str("user_id", userID).Logger()
	ctx := context.Background()

	logger.Info().Msg("Starting Webflow webhook registration")

	// 1. Get user to get webhook token
	user, err := h.DB.GetUser(userID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get user for webhook registration")
		return
	}

	if user.WebhookToken == "" {
		logger.Warn().Msg("User has no webhook token, cannot register webhooks")
		return
	}

	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "http://localhost:8080"
	}
	webhookURL := fmt.Sprintf("%s/v1/webhooks/webflow/%s", appURL, user.WebhookToken)

	// 2. List sites
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.webflow.com/v2/sites", nil)
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
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("https://api.webflow.com/v2/sites/%s/webhooks", siteID)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
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

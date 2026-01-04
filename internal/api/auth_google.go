package api

import (
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
	"github.com/rs/zerolog/log"
)

// Google OAuth credentials loaded from environment variables
func getGoogleClientID() string {
	return os.Getenv("GOOGLE_CLIENT_ID")
}

func getGoogleClientSecret() string {
	return os.Getenv("GOOGLE_CLIENT_SECRET")
}

func getGoogleRedirectURI() string {
	return getAppURL() + "/v1/integrations/google/callback"
}

// GoogleTokenResponse represents the response from Google's token endpoint
type GoogleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

// GA4Property represents a Google Analytics 4 property
type GA4Property struct {
	PropertyID   string `json:"property_id"`   // e.g., "123456789"
	DisplayName  string `json:"display_name"`  // e.g., "My Website"
	PropertyType string `json:"property_type"` // e.g., "PROPERTY_TYPE_ORDINARY"
}

// GA4Account represents a Google Analytics account
type GA4Account struct {
	AccountID   string `json:"account_id"`
	DisplayName string `json:"display_name"`
}

// GoogleUserInfo contains user info from Google
type GoogleUserInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// InitiateGoogleOAuth starts the OAuth flow
func (h *Handler) InitiateGoogleOAuth(w http.ResponseWriter, r *http.Request) {
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

	orgID := h.DB.GetEffectiveOrganisationID(user)
	if orgID == "" {
		BadRequest(w, r, "User must belong to an organisation")
		return
	}

	if getGoogleClientID() == "" {
		logger.Error().Msg("GOOGLE_CLIENT_ID not configured")
		InternalError(w, r, fmt.Errorf("google integration not configured"))
		return
	}

	if getOAuthStateSecret() == "" {
		logger.Error().Msg("SUPABASE_JWT_SECRET not configured for OAuth state signing")
		InternalError(w, r, fmt.Errorf("google integration not configured"))
		return
	}

	// Sign state with JWT Secret
	state, err := h.generateOAuthState(userClaims.UserID, orgID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to generate OAuth state")
		InternalError(w, r, err)
		return
	}

	// Scopes needed:
	// - analytics.readonly: Read GA4 data
	// - userinfo.email: Get user's email for display
	scopes := "https://www.googleapis.com/auth/analytics.readonly https://www.googleapis.com/auth/userinfo.email"

	// Build Google OAuth URL
	authURL := fmt.Sprintf(
		"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&access_type=offline&prompt=consent&state=%s",
		url.QueryEscape(getGoogleClientID()),
		url.QueryEscape(getGoogleRedirectURI()),
		url.QueryEscape(scopes),
		url.QueryEscape(state),
	)

	WriteSuccess(w, r, map[string]string{"auth_url": authURL}, "Redirect to this URL to connect Google Analytics")
}

// HandleGoogleOAuthCallback processes the OAuth callback from Google
// After successful auth, it fetches the user's GA4 properties and returns them
func (h *Handler) HandleGoogleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	logger := loggerWithRequest(r)

	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	code := r.URL.Query().Get("code")
	stateParam := r.URL.Query().Get("state")
	errParam := r.URL.Query().Get("error")

	if errParam != "" {
		logger.Warn().Str("error", errParam).Msg("Google OAuth denied")
		h.redirectToDashboardWithError(w, r, "Google", "Google connection was cancelled")
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
		h.redirectToDashboardWithError(w, r, "Google", "Invalid or expired state")
		return
	}

	// Exchange code for access token
	tokenResp, err := h.exchangeGoogleCode(code)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to exchange Google OAuth code")
		h.redirectToDashboardWithError(w, r, "Google", "Failed to connect to Google")
		return
	}

	// Store tokens temporarily in session/URL params for property selection
	// For now, we'll redirect to a property picker page with the tokens encoded
	// In production, you'd want to store these in a temporary session

	// Fetch user info
	userInfo, err := h.fetchGoogleUserInfo(r.Context(), tokenResp.AccessToken)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to fetch Google user info")
	}

	// Fetch GA4 properties
	properties, err := h.fetchGA4Properties(r.Context(), tokenResp.AccessToken)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch GA4 properties")
		h.redirectToDashboardWithError(w, r, "Google", "Failed to fetch Google Analytics properties. Ensure GA4 is set up.")
		return
	}

	if len(properties) == 0 {
		h.redirectToDashboardWithError(w, r, "Google", "No Google Analytics 4 properties found. Please set up GA4 first.")
		return
	}

	// For a simple implementation, if there's only one property, auto-select it
	// Otherwise, redirect to a property selection page
	if len(properties) == 1 {
		// Auto-select the single property
		err = h.saveGoogleConnection(r.Context(), state, tokenResp, userInfo, properties[0])
		if err != nil {
			logger.Error().Err(err).Msg("Failed to save Google connection")
			h.redirectToDashboardWithError(w, r, "Google", "Failed to save connection")
			return
		}
		h.redirectToDashboardWithSuccess(w, r, "Google", properties[0].DisplayName, "")
		return
	}

	// Multiple properties - for now, redirect with property list as JSON in URL
	// This is janky but works for testing. In production, use a proper session.
	propertiesJSON, _ := json.Marshal(properties)
	params := url.Values{}
	params.Set("google_properties", string(propertiesJSON))
	params.Set("google_state", stateParam)
	if userInfo != nil {
		params.Set("google_email", userInfo.Email)
	}
	// Store refresh token temporarily (in production, use encrypted session)
	params.Set("google_refresh_token", tokenResp.RefreshToken)
	params.Set("google_access_token", tokenResp.AccessToken)

	http.Redirect(w, r, getDashboardURL()+"?"+params.Encode(), http.StatusSeeOther)
}

// SaveGoogleProperty saves the selected GA4 property
func (h *Handler) SaveGoogleProperty(w http.ResponseWriter, r *http.Request) {
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

	orgID := h.DB.GetEffectiveOrganisationID(user)
	if orgID == "" {
		BadRequest(w, r, "User must belong to an organisation")
		return
	}

	// Parse request body
	var req struct {
		PropertyID   string `json:"property_id"`
		PropertyName string `json:"property_name"`
		RefreshToken string `json:"refresh_token"`
		AccessToken  string `json:"access_token"`
		GoogleEmail  string `json:"google_email"`
		GoogleUserID string `json:"google_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, r, "Invalid request body")
		return
	}

	if req.PropertyID == "" || req.RefreshToken == "" {
		BadRequest(w, r, "Property ID and refresh token are required")
		return
	}

	// Create the connection
	now := time.Now().UTC()
	conn := &db.GoogleAnalyticsConnection{
		ID:               uuid.New().String(),
		OrganisationID:   orgID,
		GA4PropertyID:    req.PropertyID,
		GA4PropertyName:  req.PropertyName,
		GoogleUserID:     req.GoogleUserID,
		GoogleEmail:      req.GoogleEmail,
		InstallingUserID: userClaims.UserID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := h.DB.CreateGoogleConnection(r.Context(), conn); err != nil {
		logger.Error().Err(err).Msg("Failed to save Google Analytics connection")
		InternalError(w, r, err)
		return
	}

	// Store refresh token in Supabase Vault
	if err := h.DB.StoreGoogleToken(r.Context(), conn.ID, req.RefreshToken); err != nil {
		logger.Error().Err(err).Msg("Failed to store refresh token in vault")
		InternalError(w, r, err)
		return
	}

	logger.Info().
		Str("organisation_id", orgID).
		Str("ga4_property_id", req.PropertyID).
		Str("google_email", req.GoogleEmail).
		Msg("Google Analytics connection established")

	WriteSuccess(w, r, map[string]string{
		"connection_id": conn.ID,
		"property_id":   req.PropertyID,
		"property_name": req.PropertyName,
	}, "Google Analytics connected successfully")
}

func (h *Handler) exchangeGoogleCode(code string) (*GoogleTokenResponse, error) {
	values := url.Values{}
	values.Set("client_id", getGoogleClientID())
	values.Set("client_secret", getGoogleClientSecret())
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	values.Set("redirect_uri", getGoogleRedirectURI())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm("https://oauth2.googleapis.com/token", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google API returned status: %d", resp.StatusCode)
	}

	var tokenResp GoogleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

func (h *Handler) fetchGoogleUserInfo(ctx context.Context, accessToken string) (*GoogleUserInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call userinfo endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo endpoint returned status: %d", resp.StatusCode)
	}

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode userinfo response: %w", err)
	}

	return &userInfo, nil
}

func (h *Handler) fetchGA4Properties(ctx context.Context, accessToken string) ([]GA4Property, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// First, list all accounts
	req, err := http.NewRequestWithContext(ctx, "GET", "https://analyticsadmin.googleapis.com/v1beta/accounts", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create accounts request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("accounts endpoint returned status: %d", resp.StatusCode)
	}

	var accountsResp struct {
		Accounts []struct {
			Name        string `json:"name"` // e.g., "accounts/123456"
			DisplayName string `json:"displayName"`
		} `json:"accounts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&accountsResp); err != nil {
		return nil, fmt.Errorf("failed to decode accounts response: %w", err)
	}

	// Now list properties for each account
	var allProperties []GA4Property

	for _, account := range accountsResp.Accounts {
		properties, err := h.fetchPropertiesForAccount(ctx, client, accessToken, account.Name)
		if err != nil {
			log.Warn().Err(err).Str("account", account.Name).Msg("Failed to fetch properties for account")
			continue
		}
		allProperties = append(allProperties, properties...)
	}

	return allProperties, nil
}

func (h *Handler) fetchPropertiesForAccount(ctx context.Context, client *http.Client, accessToken, accountName string) ([]GA4Property, error) {
	url := fmt.Sprintf("https://analyticsadmin.googleapis.com/v1beta/properties?filter=parent:%s", accountName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create properties request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list properties: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("properties endpoint returned status: %d", resp.StatusCode)
	}

	var propertiesResp struct {
		Properties []struct {
			Name         string `json:"name"` // e.g., "properties/123456789"
			DisplayName  string `json:"displayName"`
			PropertyType string `json:"propertyType"`
		} `json:"properties"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&propertiesResp); err != nil {
		return nil, fmt.Errorf("failed to decode properties response: %w", err)
	}

	var properties []GA4Property
	for _, p := range propertiesResp.Properties {
		// Extract property ID from name (e.g., "properties/123456789" -> "123456789")
		propertyID := strings.TrimPrefix(p.Name, "properties/")
		properties = append(properties, GA4Property{
			PropertyID:   propertyID,
			DisplayName:  p.DisplayName,
			PropertyType: p.PropertyType,
		})
	}

	return properties, nil
}

func (h *Handler) saveGoogleConnection(ctx context.Context, state *OAuthState, tokenResp *GoogleTokenResponse, userInfo *GoogleUserInfo, property GA4Property) error {
	now := time.Now().UTC()
	conn := &db.GoogleAnalyticsConnection{
		ID:               uuid.New().String(),
		OrganisationID:   state.OrgID,
		GA4PropertyID:    property.PropertyID,
		GA4PropertyName:  property.DisplayName,
		InstallingUserID: state.UserID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if userInfo != nil {
		conn.GoogleUserID = userInfo.ID
		conn.GoogleEmail = userInfo.Email
	}

	if err := h.DB.CreateGoogleConnection(ctx, conn); err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	// Store refresh token in Supabase Vault
	if err := h.DB.StoreGoogleToken(ctx, conn.ID, tokenResp.RefreshToken); err != nil {
		return fmt.Errorf("failed to store token: %w", err)
	}

	log.Info().
		Str("organisation_id", state.OrgID).
		Str("ga4_property_id", property.PropertyID).
		Msg("Google Analytics connection established")

	return nil
}

// GoogleConnectionResponse represents a Google Analytics connection in API responses
type GoogleConnectionResponse struct {
	ID              string `json:"id"`
	GA4PropertyID   string `json:"ga4_property_id,omitempty"`
	GA4PropertyName string `json:"ga4_property_name,omitempty"`
	GoogleEmail     string `json:"google_email,omitempty"`
	CreatedAt       string `json:"created_at"`
}

// GoogleConnectionsHandler handles requests to /v1/integrations/google
func (h *Handler) GoogleConnectionsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listGoogleConnections(w, r)
	case http.MethodPost:
		h.InitiateGoogleOAuth(w, r)
	default:
		MethodNotAllowed(w, r)
	}
}

// GoogleConnectionHandler handles requests to /v1/integrations/google/:id
func (h *Handler) GoogleConnectionHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/integrations/google/")
	if path == "" {
		BadRequest(w, r, "Connection ID is required")
		return
	}

	// Handle callback separately (no auth required)
	if path == "callback" {
		if r.Method == http.MethodGet {
			h.HandleGoogleOAuthCallback(w, r)
			return
		}
		MethodNotAllowed(w, r)
		return
	}

	// Handle save-property endpoint
	if path == "save-property" {
		if r.Method == http.MethodPost {
			h.SaveGoogleProperty(w, r)
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
		h.deleteGoogleConnection(w, r, connectionID)
	default:
		MethodNotAllowed(w, r)
	}
}

// listGoogleConnections lists all Google Analytics connections for the user's organisation
func (h *Handler) listGoogleConnections(w http.ResponseWriter, r *http.Request) {
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

	orgID := h.DB.GetEffectiveOrganisationID(user)
	if orgID == "" {
		WriteSuccess(w, r, []GoogleConnectionResponse{}, "No organisation")
		return
	}

	connections, err := h.DB.ListGoogleConnections(r.Context(), orgID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to list Google Analytics connections")
		InternalError(w, r, err)
		return
	}

	response := make([]GoogleConnectionResponse, 0, len(connections))
	for _, conn := range connections {
		response = append(response, GoogleConnectionResponse{
			ID:              conn.ID,
			GA4PropertyID:   conn.GA4PropertyID,
			GA4PropertyName: conn.GA4PropertyName,
			GoogleEmail:     conn.GoogleEmail,
			CreatedAt:       conn.CreatedAt.Format(time.RFC3339),
		})
	}

	WriteSuccess(w, r, response, "")
}

// deleteGoogleConnection deletes a Google Analytics connection
func (h *Handler) deleteGoogleConnection(w http.ResponseWriter, r *http.Request, connectionID string) {
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

	orgID := h.DB.GetEffectiveOrganisationID(user)
	if orgID == "" {
		BadRequest(w, r, "User must belong to an organisation")
		return
	}

	err = h.DB.DeleteGoogleConnection(r.Context(), connectionID, orgID)
	if err != nil {
		if errors.Is(err, db.ErrGoogleConnectionNotFound) {
			NotFound(w, r, "Google Analytics connection not found")
			return
		}
		logger.Error().Err(err).Msg("Failed to delete Google Analytics connection")
		InternalError(w, r, err)
		return
	}

	logger.Info().Str("connection_id", connectionID).Msg("Google Analytics connection deleted")
	WriteNoContent(w, r)
}

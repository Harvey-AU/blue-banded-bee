package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/google/uuid"
	"github.com/slack-go/slack"
)

// SlackConnectionResponse represents a Slack connection in API responses
type SlackConnectionResponse struct {
	ID            string `json:"id"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	CreatedAt     string `json:"created_at"`
}

// SlackUserLinkResponse represents a user link in API responses
type SlackUserLinkResponse struct {
	ID              string `json:"id"`
	SlackUserID     string `json:"slack_user_id"`
	DMNotifications bool   `json:"dm_notifications"`
	CreatedAt       string `json:"created_at"`
}

// SlackConnectRequest represents the request to initiate OAuth
type SlackConnectRequest struct {
	RedirectURI string `json:"redirect_uri,omitempty"`
}

// SlackConnectResponse returns the OAuth URL to redirect to
type SlackConnectResponse struct {
	AuthURL string `json:"auth_url"`
}

// SlackLinkUserRequest represents the request to link a Slack user
type SlackLinkUserRequest struct {
	ConnectionID    string `json:"connection_id"`
	SlackUserID     string `json:"slack_user_id,omitempty"` // Optional if using email match
	DMNotifications bool   `json:"dm_notifications"`
}

// SlackUpdateNotificationsRequest updates notification preferences
type SlackUpdateNotificationsRequest struct {
	DMNotifications bool `json:"dm_notifications"`
}

// OAuthState contains signed state data for CSRF protection
type OAuthState struct {
	UserID    string `json:"u"`
	OrgID     string `json:"o"`
	Timestamp int64  `json:"t"`
	Nonce     string `json:"n"`
}

const (
	// slackOAuthScopes defines the permissions requested from Slack during OAuth
	slackOAuthScopes = "chat:write,im:write,users:read,users:read.email"
	// slackAPITimeout is the timeout for Slack API calls
	slackAPITimeout = 30 * time.Second
	// oauthStateExpiry is how long OAuth state tokens are valid (15 minutes)
	oauthStateExpiry = 900
	// defaultAppURL is the fallback URL when APP_URL env var is not set
	defaultAppURL = "https://app.bluebandedbee.co"
)

// getSlackClientID returns the Slack OAuth client ID
func getSlackClientID() string {
	return os.Getenv("SLACK_CLIENT_ID")
}

// getSlackClientSecret returns the Slack OAuth client secret
func getSlackClientSecret() string {
	return os.Getenv("SLACK_CLIENT_SECRET")
}

// getSlackStateSecret returns the secret used for HMAC signing OAuth state
// Returns empty string if not configured - callers should validate
func getSlackStateSecret() string {
	return os.Getenv("SUPABASE_JWT_SECRET")
}

// SlackConnectionsHandler handles requests to /v1/integrations/slack
func (h *Handler) SlackConnectionsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listSlackConnections(w, r)
	case http.MethodPost:
		h.initiateSlackOAuth(w, r)
	default:
		MethodNotAllowed(w, r)
	}
}

// SlackConnectionHandler handles requests to /v1/integrations/slack/:id and sub-routes
func (h *Handler) SlackConnectionHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/integrations/slack/")
	if path == "" {
		BadRequest(w, r, "Path is required")
		return
	}

	parts := strings.Split(path, "/")

	// Handle special routes first (non-connection-specific)
	switch parts[0] {
	case "callback":
		if r.Method == http.MethodGet {
			h.handleSlackOAuthCallback(w, r)
			return
		}
		MethodNotAllowed(w, r)
		return
	case "connect":
		if r.Method == http.MethodPost {
			h.initiateSlackOAuth(w, r)
			return
		}
		MethodNotAllowed(w, r)
		return
	}

	// Validate connection ID
	connectionID := parts[0]
	if _, err := uuid.Parse(connectionID); err != nil {
		BadRequest(w, r, "Invalid connection ID format")
		return
	}

	// Handle sub-routes for a specific connection
	if len(parts) > 1 {
		switch parts[1] {
		case "link-user":
			switch r.Method {
			case http.MethodPost:
				h.linkSlackUser(w, r, connectionID)
			case http.MethodDelete:
				h.unlinkSlackUser(w, r, connectionID)
			case http.MethodPut:
				h.updateSlackUserNotifications(w, r, connectionID)
			default:
				MethodNotAllowed(w, r)
			}
			return
		case "user-link":
			if r.Method == http.MethodGet {
				h.getSlackUserLink(w, r, connectionID)
				return
			}
			MethodNotAllowed(w, r)
			return
		case "users":
			if r.Method == http.MethodGet {
				h.listSlackWorkspaceUsers(w, r, connectionID)
				return
			}
			MethodNotAllowed(w, r)
			return
		}
	}

	// Handle connection-level operations
	switch r.Method {
	case http.MethodGet:
		h.getSlackConnection(w, r, connectionID)
	case http.MethodDelete:
		h.deleteSlackConnection(w, r, connectionID)
	default:
		MethodNotAllowed(w, r)
	}
}

// SlackOAuthCallback handles the OAuth callback (no auth middleware)
func (h *Handler) SlackOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}
	h.handleSlackOAuthCallback(w, r)
}

// initiateSlackOAuth starts the OAuth flow
func (h *Handler) initiateSlackOAuth(w http.ResponseWriter, r *http.Request) {
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

	if getSlackClientID() == "" {
		logger.Error().Msg("SLACK_CLIENT_ID not configured")
		InternalError(w, r, fmt.Errorf("slack integration not configured"))
		return
	}

	// Generate state token
	state, err := h.generateOAuthState(userClaims.UserID, *user.OrganisationID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to generate OAuth state")
		InternalError(w, r, err)
		return
	}

	// Build Slack OAuth URL
	authURL := fmt.Sprintf(
		"https://slack.com/oauth/v2/authorize?client_id=%s&scope=%s&redirect_uri=%s&state=%s",
		url.QueryEscape(getSlackClientID()),
		url.QueryEscape(slackOAuthScopes),
		url.QueryEscape(getSlackRedirectURI()),
		url.QueryEscape(state),
	)

	WriteSuccess(w, r, SlackConnectResponse{AuthURL: authURL}, "Redirect to this URL to connect Slack")
}

// handleSlackOAuthCallback processes the OAuth callback from Slack
func (h *Handler) handleSlackOAuthCallback(w http.ResponseWriter, r *http.Request) {
	logger := loggerWithRequest(r)

	// Check for error from Slack
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		logger.Warn().Str("error", errParam).Msg("Slack OAuth denied")
		h.redirectToDashboardWithError(w, r, "Slack connection was cancelled")
		return
	}

	code := r.URL.Query().Get("code")
	stateParam := r.URL.Query().Get("state")

	if code == "" || stateParam == "" {
		BadRequest(w, r, "Missing code or state parameter")
		return
	}

	// Validate state
	state, err := h.validateOAuthState(stateParam)
	if err != nil {
		logger.Warn().Err(err).Msg("Invalid OAuth state")
		h.redirectToDashboardWithError(w, r, "Invalid or expired state")
		return
	}

	// Exchange code for access token
	httpClient := &http.Client{Timeout: slackAPITimeout}
	resp, err := slack.GetOAuthV2Response(
		httpClient,
		getSlackClientID(),
		getSlackClientSecret(),
		code,
		getSlackRedirectURI(),
	)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to exchange OAuth code")
		h.redirectToDashboardWithError(w, r, "Failed to connect to Slack")
		return
	}

	// Store connection
	now := time.Now().UTC()
	conn := &db.SlackConnection{
		ID:               uuid.New().String(),
		OrganisationID:   state.OrgID,
		WorkspaceID:      resp.Team.ID,
		WorkspaceName:    resp.Team.Name,
		BotUserID:        resp.BotUserID,
		InstallingUserID: &state.UserID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := h.DB.CreateSlackConnection(r.Context(), conn); err != nil {
		logger.Error().Err(err).Msg("Failed to save Slack connection")
		h.redirectToDashboardWithError(w, r, "Failed to save connection")
		return
	}

	// Store access token in Supabase Vault
	if err := h.DB.StoreSlackToken(r.Context(), conn.ID, resp.AccessToken); err != nil {
		logger.Error().Err(err).Msg("Failed to store access token in vault")
		h.redirectToDashboardWithError(w, r, "Failed to secure connection")
		return
	}

	logger.Info().
		Str("workspace_id", resp.Team.ID).
		Str("workspace_name", resp.Team.Name).
		Str("organisation_id", state.OrgID).
		Msg("Slack workspace connected")

	// Redirect to dashboard with success (includes connection ID for auto-linking)
	h.redirectToDashboardWithSuccess(w, r, resp.Team.Name, conn.ID)
}

// listSlackConnections lists all Slack connections for the user's organisation
func (h *Handler) listSlackConnections(w http.ResponseWriter, r *http.Request) {
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
		WriteSuccess(w, r, []SlackConnectionResponse{}, "No organisation")
		return
	}

	connections, err := h.DB.ListSlackConnections(r.Context(), *user.OrganisationID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to list Slack connections")
		InternalError(w, r, err)
		return
	}

	response := make([]SlackConnectionResponse, 0, len(connections))
	for _, conn := range connections {
		response = append(response, SlackConnectionResponse{
			ID:            conn.ID,
			WorkspaceID:   conn.WorkspaceID,
			WorkspaceName: conn.WorkspaceName,
			CreatedAt:     conn.CreatedAt.Format(time.RFC3339),
		})
	}

	WriteSuccess(w, r, response, "")
}

// getSlackConnection retrieves a specific Slack connection
func (h *Handler) getSlackConnection(w http.ResponseWriter, r *http.Request, connectionID string) {
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

	conn, err := h.DB.GetSlackConnection(r.Context(), connectionID)
	if err != nil {
		if errors.Is(err, db.ErrSlackConnectionNotFound) {
			NotFound(w, r, "Slack connection not found")
			return
		}
		logger.Error().Err(err).Msg("Failed to get Slack connection")
		InternalError(w, r, err)
		return
	}

	// Verify org ownership
	if user.OrganisationID == nil || *user.OrganisationID != conn.OrganisationID {
		Forbidden(w, r, "You don't have access to this connection")
		return
	}

	// Get user link for this connection if it exists
	var userLink *SlackUserLinkResponse
	link, err := h.DB.GetSlackUserLink(r.Context(), userClaims.UserID, connectionID)
	if err == nil {
		userLink = &SlackUserLinkResponse{
			ID:              link.ID,
			SlackUserID:     link.SlackUserID,
			DMNotifications: link.DMNotifications,
			CreatedAt:       link.CreatedAt.Format(time.RFC3339),
		}
	}

	response := struct {
		Connection SlackConnectionResponse `json:"connection"`
		UserLink   *SlackUserLinkResponse  `json:"user_link,omitempty"`
	}{
		Connection: SlackConnectionResponse{
			ID:            conn.ID,
			WorkspaceID:   conn.WorkspaceID,
			WorkspaceName: conn.WorkspaceName,
			CreatedAt:     conn.CreatedAt.Format(time.RFC3339),
		},
		UserLink: userLink,
	}

	WriteSuccess(w, r, response, "")
}

// deleteSlackConnection disconnects a Slack workspace
func (h *Handler) deleteSlackConnection(w http.ResponseWriter, r *http.Request, connectionID string) {
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

	err = h.DB.DeleteSlackConnection(r.Context(), connectionID, *user.OrganisationID)
	if err != nil {
		if errors.Is(err, db.ErrSlackConnectionNotFound) {
			NotFound(w, r, "Slack connection not found")
			return
		}
		logger.Error().Err(err).Msg("Failed to delete Slack connection")
		InternalError(w, r, err)
		return
	}

	logger.Info().Str("connection_id", connectionID).Msg("Slack connection deleted")
	WriteNoContent(w, r)
}

// linkSlackUser links the current user to their Slack identity
// Note: Users are auto-linked when they sign in with Slack OIDC, so this endpoint
// is mainly used for manual overrides or when the auto-link didn't apply
func (h *Handler) linkSlackUser(w http.ResponseWriter, r *http.Request, connectionID string) {
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

	// Verify connection belongs to user's org
	conn, err := h.DB.GetSlackConnection(r.Context(), connectionID)
	if err != nil {
		if errors.Is(err, db.ErrSlackConnectionNotFound) {
			NotFound(w, r, "Slack connection not found")
			return
		}
		logger.Error().Err(err).Msg("Failed to get Slack connection")
		InternalError(w, r, err)
		return
	}

	if user.OrganisationID == nil || *user.OrganisationID != conn.OrganisationID {
		Forbidden(w, r, "You don't have access to this connection")
		return
	}

	// Get slack_user_id from user profile (populated via Slack OIDC login)
	// or from request body for manual override
	// or look up by email as fallback
	var req SlackLinkUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		// Log but continue - body is optional
		logger.Debug().Err(err).Msg("Failed to decode link-user request body")
	}

	slackUserID := req.SlackUserID
	if slackUserID == "" && user.SlackUserID != nil {
		slackUserID = *user.SlackUserID
	}

	// If still no Slack user ID, try to find by email
	if slackUserID == "" && userClaims.Email != "" {
		token, err := h.DB.GetSlackToken(r.Context(), connectionID)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to get Slack token for user lookup")
			InternalError(w, r, err)
			return
		}

		httpClient := &http.Client{Timeout: slackAPITimeout}
		client := slack.New(token, slack.OptionHTTPClient(httpClient))
		slackUser, err := client.GetUserByEmail(userClaims.Email)
		if err != nil {
			logger.Warn().Err(err).Msg("Could not find Slack user by email")
			BadRequest(w, r, "Could not find your Slack user. Make sure your email matches your Slack account.")
			return
		}
		slackUserID = slackUser.ID
		logger.Info().Str("slack_user_id", slackUserID).Msg("Found Slack user by email lookup")
	}

	if slackUserID == "" {
		BadRequest(w, r, "No Slack user ID available and no email to look up")
		return
	}

	// Create link
	now := time.Now().UTC()
	link := &db.SlackUserLink{
		ID:                uuid.New().String(),
		UserID:            userClaims.UserID,
		SlackConnectionID: connectionID,
		SlackUserID:       slackUserID,
		DMNotifications:   true, // Default to enabled
		CreatedAt:         now,
	}

	if err := h.DB.CreateSlackUserLink(r.Context(), link); err != nil {
		logger.Error().Err(err).Msg("Failed to create Slack user link")
		InternalError(w, r, err)
		return
	}

	logger.Info().
		Str("user_id", userClaims.UserID).
		Str("slack_user_id", slackUserID).
		Str("connection_id", connectionID).
		Msg("Slack user linked")

	WriteCreated(w, r, SlackUserLinkResponse{
		ID:              link.ID,
		SlackUserID:     link.SlackUserID,
		DMNotifications: link.DMNotifications,
		CreatedAt:       link.CreatedAt.Format(time.RFC3339),
	}, "Slack user linked successfully")
}

// unlinkSlackUser removes the link between a user and their Slack identity
func (h *Handler) unlinkSlackUser(w http.ResponseWriter, r *http.Request, connectionID string) {
	logger := loggerWithRequest(r)

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	err := h.DB.DeleteSlackUserLink(r.Context(), userClaims.UserID, connectionID)
	if err != nil {
		if errors.Is(err, db.ErrSlackUserLinkNotFound) {
			NotFound(w, r, "User link not found")
			return
		}
		logger.Error().Err(err).Msg("Failed to delete Slack user link")
		InternalError(w, r, err)
		return
	}

	logger.Info().Str("user_id", userClaims.UserID).Str("connection_id", connectionID).Msg("Slack user unlinked")
	WriteNoContent(w, r)
}

// updateSlackUserNotifications updates notification preferences
func (h *Handler) updateSlackUserNotifications(w http.ResponseWriter, r *http.Request, connectionID string) {
	logger := loggerWithRequest(r)

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	var req SlackUpdateNotificationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, r, "Invalid JSON request body")
		return
	}

	err := h.DB.UpdateSlackUserLinkNotifications(r.Context(), userClaims.UserID, connectionID, req.DMNotifications)
	if err != nil {
		if errors.Is(err, db.ErrSlackUserLinkNotFound) {
			NotFound(w, r, "User link not found")
			return
		}
		logger.Error().Err(err).Msg("Failed to update notification preferences")
		InternalError(w, r, err)
		return
	}

	WriteSuccess(w, r, nil, "Notification preferences updated")
}

// listSlackWorkspaceUsers lists users in a Slack workspace for linking
func (h *Handler) listSlackWorkspaceUsers(w http.ResponseWriter, r *http.Request, connectionID string) {
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

	conn, err := h.DB.GetSlackConnection(r.Context(), connectionID)
	if err != nil {
		if errors.Is(err, db.ErrSlackConnectionNotFound) {
			NotFound(w, r, "Slack connection not found")
			return
		}
		logger.Error().Err(err).Msg("Failed to get Slack connection")
		InternalError(w, r, err)
		return
	}

	if user.OrganisationID == nil || *user.OrganisationID != conn.OrganisationID {
		Forbidden(w, r, "You don't have access to this connection")
		return
	}

	// Get token from Vault
	token, err := h.DB.GetSlackToken(r.Context(), connectionID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get access token from vault")
		InternalError(w, r, err)
		return
	}

	// List users from Slack with pagination for large workspaces
	httpClient := &http.Client{Timeout: slackAPITimeout}
	client := slack.New(token, slack.OptionHTTPClient(httpClient))

	var users []slack.User
	pager := client.GetUsersPaginated(slack.GetUsersOptionLimit(200))
	for {
		page, err := pager.Next(r.Context())
		if pager.Done(err) {
			break
		}
		if err != nil {
			logger.Error().Err(err).Msg("Failed to fetch Slack users")
			InternalError(w, r, fmt.Errorf("failed to fetch Slack users: %w", err))
			return
		}
		users = append(users, page.Users...)
		pager = page
	}

	// Filter to real users (not bots, not deleted)
	type SlackUserInfo struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		RealName    string `json:"real_name"`
		Email       string `json:"email,omitempty"`
		DisplayName string `json:"display_name"`
	}

	result := make([]SlackUserInfo, 0)
	for _, u := range users {
		if u.Deleted || u.IsBot || u.ID == "USLACKBOT" {
			continue
		}
		result = append(result, SlackUserInfo{
			ID:          u.ID,
			Name:        u.Name,
			RealName:    u.RealName,
			Email:       u.Profile.Email,
			DisplayName: u.Profile.DisplayName,
		})
	}

	WriteSuccess(w, r, result, "")
}

// getSlackUserLink returns the current user's link to a Slack connection
func (h *Handler) getSlackUserLink(w http.ResponseWriter, r *http.Request, connectionID string) {
	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	link, err := h.DB.GetSlackUserLink(r.Context(), userClaims.UserID, connectionID)
	if err != nil {
		if errors.Is(err, db.ErrSlackUserLinkNotFound) {
			NotFound(w, r, "User not linked to this Slack connection")
			return
		}
		InternalError(w, r, err)
		return
	}

	WriteSuccess(w, r, SlackUserLinkResponse{
		ID:              link.ID,
		SlackUserID:     link.SlackUserID,
		DMNotifications: link.DMNotifications,
		CreatedAt:       link.CreatedAt.Format(time.RFC3339),
	}, "")
}

// Helper functions

func (h *Handler) generateOAuthState(userID, orgID string) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	state := OAuthState{
		UserID:    userID,
		OrgID:     orgID,
		Timestamp: time.Now().Unix(),
		Nonce:     base64.URLEncoding.EncodeToString(nonce),
	}

	data, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("failed to marshal state: %w", err)
	}

	// Sign with HMAC
	mac := hmac.New(sha256.New, []byte(getSlackStateSecret()))
	mac.Write(data)
	sig := mac.Sum(nil)

	// Combine data + signature
	payload := append(data, sig...)
	return base64.URLEncoding.EncodeToString(payload), nil
}

func (h *Handler) validateOAuthState(stateParam string) (*OAuthState, error) {
	payload, err := base64.URLEncoding.DecodeString(stateParam)
	if err != nil {
		return nil, fmt.Errorf("invalid state encoding: %w", err)
	}

	if len(payload) < 32 {
		return nil, fmt.Errorf("state too short")
	}

	data := payload[:len(payload)-32]
	sig := payload[len(payload)-32:]

	// Verify HMAC
	mac := hmac.New(sha256.New, []byte(getSlackStateSecret()))
	mac.Write(data)
	expectedSig := mac.Sum(nil)

	if !hmac.Equal(sig, expectedSig) {
		return nil, fmt.Errorf("invalid state signature")
	}

	var state OAuthState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("invalid state data: %w", err)
	}

	// Check timestamp (15 minute expiry)
	if time.Now().Unix()-state.Timestamp > oauthStateExpiry {
		return nil, fmt.Errorf("state expired")
	}

	return &state, nil
}

// getSlackRedirectURI returns the OAuth callback URL for Slack
func getSlackRedirectURI() string {
	if uri := os.Getenv("SLACK_REDIRECT_URI"); uri != "" {
		return uri
	}
	return getAppURL() + "/v1/integrations/slack/callback"
}

// getAppURL returns the application URL, defaulting to production
func getAppURL() string {
	if appURL := os.Getenv("APP_URL"); appURL != "" {
		return appURL
	}
	return defaultAppURL
}

// getDashboardURL returns the dashboard URL
func getDashboardURL() string {
	if dashURL := os.Getenv("DASHBOARD_URL"); dashURL != "" {
		return dashURL
	}
	return getAppURL() + "/dashboard"
}

func (h *Handler) redirectToDashboardWithError(w http.ResponseWriter, r *http.Request, errMsg string) {
	http.Redirect(w, r, getDashboardURL()+"?slack_error="+url.QueryEscape(errMsg), http.StatusSeeOther)
}

func (h *Handler) redirectToDashboardWithSuccess(w http.ResponseWriter, r *http.Request, workspaceName string, connectionID string) {
	redirectURL := getDashboardURL() + "?slack_connected=" + url.QueryEscape(workspaceName)
	if connectionID != "" {
		redirectURL += "&slack_connection_id=" + url.QueryEscape(connectionID)
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

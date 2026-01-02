package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// OAuthState contains signed state data for CSRF protection
// Shared between Slack and Webflow
type OAuthState struct {
	UserID    string `json:"u"`
	OrgID     string `json:"o"`
	Timestamp int64  `json:"t"`
	Nonce     string `json:"n"`
}

// getOAuthStateSecret returns the secret used for HMAC signing OAuth state
func getOAuthStateSecret() string {
	// Reusing SUPABASE_JWT_SECRET as the signing key for convenience
	return os.Getenv("SUPABASE_JWT_SECRET")
}

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
	mac := hmac.New(sha256.New, []byte(getOAuthStateSecret()))
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

	if len(payload) < sha256.Size {
		return nil, fmt.Errorf("state too short")
	}

	data := payload[:len(payload)-sha256.Size]
	sig := payload[len(payload)-sha256.Size:]

	// Verify HMAC
	mac := hmac.New(sha256.New, []byte(getOAuthStateSecret()))
	mac.Write(data)
	expectedSig := mac.Sum(nil)

	if !hmac.Equal(sig, expectedSig) {
		return nil, fmt.Errorf("invalid state signature")
	}

	var state OAuthState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("invalid state data: %w", err)
	}

	// Check expiry (15 mins)
	if time.Now().Unix()-state.Timestamp > 900 {
		return nil, fmt.Errorf("state expired")
	}

	return &state, nil
}

// getAppURL returns the application URL, defaulting to production
func getAppURL() string {
	if appURL := os.Getenv("APP_URL"); appURL != "" {
		return appURL
	}
	// defaultAppURL is defined as a constant
	return "https://app.bluebandedbee.co"
}

// getDashboardURL returns the dashboard URL
func getDashboardURL() string {
	if dashURL := os.Getenv("DASHBOARD_URL"); dashURL != "" {
		return dashURL
	}
	return getAppURL() + "/dashboard"
}

func (h *Handler) redirectToDashboardWithError(w http.ResponseWriter, r *http.Request, integration, errMsg string) {
	paramName := fmt.Sprintf("%s_error", strings.ToLower(integration))
	params := url.Values{}
	params.Set(paramName, errMsg)
	http.Redirect(w, r, getDashboardURL()+"?"+params.Encode(), http.StatusSeeOther)
}

func (h *Handler) redirectToDashboardWithSuccess(w http.ResponseWriter, r *http.Request, integration, label, connectionID string) {
	paramName := fmt.Sprintf("%s_connected", strings.ToLower(integration))
	params := url.Values{}
	params.Set(paramName, label)
	if connectionID != "" {
		idParamName := fmt.Sprintf("%s_connection_id", strings.ToLower(integration))
		params.Set(idParamName, connectionID)
	}
	http.Redirect(w, r, getDashboardURL()+"?"+params.Encode(), http.StatusSeeOther)
}

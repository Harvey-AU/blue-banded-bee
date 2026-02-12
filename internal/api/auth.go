package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	emailverifier "github.com/AfterShip/email-verifier"
	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/getsentry/sentry-go"
)

var (
	verifier = emailverifier.NewVerifier()
)

// AuthRegisterRequest represents a user registration request
type AuthRegisterRequest struct {
	UserID   string  `json:"user_id"`
	Email    string  `json:"email"`
	FullName *string `json:"full_name,omitempty"`
	OrgName  *string `json:"org_name,omitempty"`
}

// AuthSessionRequest represents a session validation request
type AuthSessionRequest struct {
	Token string `json:"token"`
}

// UserResponse represents a user in API responses
type UserResponse struct {
	ID             string  `json:"id"`
	Email          string  `json:"email"`
	FullName       *string `json:"full_name"`
	OrganisationID *string `json:"organisation_id"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// OrganisationResponse represents an organisation in API responses
type OrganisationResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// AuthRegister handles POST /v1/auth/register
func (h *Handler) AuthRegister(w http.ResponseWriter, r *http.Request) {
	logger := loggerWithRequest(r)

	if r.Method != http.MethodPost {
		MethodNotAllowed(w, r)
		return
	}

	var req AuthRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, r, "Invalid JSON request body")
		return
	}

	if req.UserID == "" || req.Email == "" {
		BadRequest(w, r, "user_id and email are required")
		return
	}

	var orgName string

	// 1. Org name if explicitly provided
	if req.OrgName != nil && *req.OrgName != "" {
		orgName = *req.OrgName
	}

	// 2. Domain name if not generic (and org name not already set)
	if orgName == "" {
		result, err := verifier.Verify(req.Email)
		if err != nil {
			logger.Warn().Err(err).Msg("Email verifier failed")
		} else if !result.Free {
			// Not a free provider, so use the domain name
			if emailParts := strings.Split(req.Email, "@"); len(emailParts) == 2 {
				domain := emailParts[1]
				domainName := strings.Split(domain, ".")[0]
				if len(domainName) > 0 {
					// Capitalise first letter of domain name
					orgName = strings.ToUpper(string(domainName[0])) + domainName[1:]
				}
			}
		}
	}

	// 3. Person's full name as fallback
	if orgName == "" && req.FullName != nil && *req.FullName != "" {
		orgName = *req.FullName
	}

	// 4. Final default if nothing else worked
	if orgName == "" {
		orgName = "Personal Organisation"
	}

	// Create user with organisation automatically
	user, org, err := h.DB.CreateUser(req.UserID, req.Email, req.FullName, orgName)
	if err != nil {
		sentry.CaptureException(err)
		logger.Error().Err(err).Str("user_id", req.UserID).Msg("Failed to create user with organisation")
		InternalError(w, r, err)
		return
	}

	userResp := UserResponse{
		ID:             user.ID,
		Email:          user.Email,
		FullName:       user.FullName,
		OrganisationID: user.OrganisationID,
		CreatedAt:      user.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      user.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	orgResp := OrganisationResponse{
		ID:        org.ID,
		Name:      org.Name,
		CreatedAt: org.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: org.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	WriteCreated(w, r, map[string]any{
		"user":         userResp,
		"organisation": orgResp,
	}, "User registered successfully")
}

// AuthSession handles POST /v1/auth/session
func (h *Handler) AuthSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		MethodNotAllowed(w, r)
		return
	}

	var req AuthSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, r, "Invalid JSON request body")
		return
	}

	if req.Token == "" {
		BadRequest(w, r, "token is required")
		return
	}

	sessionInfo := auth.ValidateSession(req.Token)
	WriteSuccess(w, r, sessionInfo, "Session validated")
}

type AuthProfileUpdateRequest struct {
	FullName *string `json:"full_name"`
}

// AuthProfile handles GET/PATCH /v1/auth/profile
func (h *Handler) AuthProfile(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getAuthProfile(w, r)
	case http.MethodPatch:
		h.updateAuthProfile(w, r)
	default:
		MethodNotAllowed(w, r)
	}
}

func (h *Handler) getAuthProfile(w http.ResponseWriter, r *http.Request) {
	logger := loggerWithRequest(r)

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	claimsFullName := fullNameFromClaims(userClaims)

	// Auto-create user if they don't exist
	user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, claimsFullName)
	if err != nil {
		sentry.CaptureException(err)
		logger.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get or create user")
		InternalError(w, r, err)
		return
	}

	if (user.FullName == nil || strings.TrimSpace(*user.FullName) == "") && claimsFullName != nil {
		if err := h.DB.UpdateUserFullName(userClaims.UserID, claimsFullName); err == nil {
			user.FullName = claimsFullName
		}
	}

	userResp := UserResponse{
		ID:             user.ID,
		Email:          user.Email,
		FullName:       user.FullName,
		OrganisationID: user.OrganisationID,
		CreatedAt:      user.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      user.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	response := map[string]any{
		"user":         userResp,
		"auth_methods": authMethodsFromClaims(userClaims),
	}

	// Get organisation if user has one
	if user.OrganisationID != nil {
		org, err := h.DB.GetOrganisation(*user.OrganisationID)
		if err != nil {
			logger.Warn().Err(err).Str("organisation_id", *user.OrganisationID).Msg("Failed to get organisation")
		} else {
			orgResp := OrganisationResponse{
				ID:        org.ID,
				Name:      org.Name,
				CreatedAt: org.CreatedAt.Format("2006-01-02T15:04:05Z"),
				UpdatedAt: org.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			}
			response["organisation"] = orgResp
		}
	}

	WriteSuccess(w, r, response, "Profile retrieved successfully")
}

func (h *Handler) updateAuthProfile(w http.ResponseWriter, r *http.Request) {
	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	var req AuthProfileUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, r, "Invalid JSON request body")
		return
	}

	if req.FullName == nil {
		BadRequest(w, r, "full_name is required")
		return
	}

	name := strings.TrimSpace(*req.FullName)
	if name == "" {
		req.FullName = nil
	} else {
		if len(name) > 120 {
			BadRequest(w, r, "full_name must be 120 characters or fewer")
			return
		}
		req.FullName = &name
	}

	if _, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, req.FullName); err != nil {
		InternalError(w, r, err)
		return
	}

	if err := h.DB.UpdateUserFullName(userClaims.UserID, req.FullName); err != nil {
		InternalError(w, r, err)
		return
	}

	user, err := h.DB.GetUser(userClaims.UserID)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	userResp := UserResponse{
		ID:             user.ID,
		Email:          user.Email,
		FullName:       user.FullName,
		OrganisationID: user.OrganisationID,
		CreatedAt:      user.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      user.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	WriteSuccess(w, r, map[string]any{
		"user":         userResp,
		"auth_methods": authMethodsFromClaims(userClaims),
	}, "Profile updated successfully")
}

func fullNameFromClaims(userClaims *auth.UserClaims) *string {
	if userClaims == nil {
		return nil
	}

	for _, key := range []string{"full_name", "name"} {
		value, ok := userClaims.UserMetadata[key]
		if !ok {
			continue
		}
		name, ok := value.(string)
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if name != "" {
			return &name
		}
	}

	return nil
}

func authMethodsFromClaims(userClaims *auth.UserClaims) []string {
	if userClaims == nil {
		return []string{"email"}
	}

	var methods []string
	if providersRaw, ok := userClaims.AppMetadata["providers"]; ok {
		if providers, ok := providersRaw.([]any); ok {
			for _, providerRaw := range providers {
				provider, ok := providerRaw.(string)
				if !ok {
					continue
				}
				provider = strings.TrimSpace(strings.ToLower(provider))
				if provider != "" && !slices.Contains(methods, provider) {
					methods = append(methods, provider)
				}
			}
		}
	}

	if providerRaw, ok := userClaims.AppMetadata["provider"]; ok {
		if provider, ok := providerRaw.(string); ok {
			provider = strings.TrimSpace(strings.ToLower(provider))
			if provider != "" && !slices.Contains(methods, provider) {
				methods = append(methods, provider)
			}
		}
	}

	if len(methods) == 0 {
		methods = append(methods, "email")
	}

	return methods
}

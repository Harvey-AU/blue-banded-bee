package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog/log"
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

	// Default organisation name logic
	orgName := "Personal Organisation" // Ultimate fallback
	if req.OrgName != nil && *req.OrgName != "" {
		orgName = *req.OrgName
	} else if req.FullName != nil && *req.FullName != "" {
		orgName = *req.FullName
	} else {
		// Extract domain from email for organisation name
		if emailParts := strings.Split(req.Email, "@"); len(emailParts) == 2 {
			domain := emailParts[1]
			// Remove common TLDs to get a cleaner name
			domainName := strings.Split(domain, ".")[0]
			// Capitalise first letter manually
			if len(domainName) > 0 {
				orgName = strings.ToUpper(domainName[:1]) + domainName[1:]
			}
		}
	}

	// Create user with organisation automatically
	user, org, err := h.DB.CreateUser(req.UserID, req.Email, req.FullName, orgName)
	if err != nil {
		sentry.CaptureException(err)
		log.Error().Err(err).Str("user_id", req.UserID).Msg("Failed to create user with organisation")
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

	WriteCreated(w, r, map[string]interface{}{
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

// AuthProfile handles GET /v1/auth/profile
func (h *Handler) AuthProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	user, err := h.DB.GetUser(userClaims.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			NotFound(w, r, "User not found")
			return
		}
		sentry.CaptureException(err)
		log.Error().Err(err).Str("user_id", userClaims.UserID).Msg("Failed to get user")
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

	response := map[string]interface{}{
		"user": userResp,
	}

	// Get organisation if user has one
	if user.OrganisationID != nil {
		org, err := h.DB.GetOrganisation(*user.OrganisationID)
		if err != nil {
			log.Warn().Err(err).Str("organisation_id", *user.OrganisationID).Msg("Failed to get organisation")
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
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
)

// OrganisationsHandler handles GET and POST /v1/organisations
func (h *Handler) OrganisationsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listUserOrganisations(w, r)
	case http.MethodPost:
		h.createOrganisation(w, r)
	default:
		MethodNotAllowed(w, r)
	}
}

// listUserOrganisations returns all organisations the user is a member of
func (h *Handler) listUserOrganisations(w http.ResponseWriter, r *http.Request) {
	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	// Ensure user exists in database
	_, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	orgs, err := h.DB.ListUserOrganisations(userClaims.UserID)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	// Format organisations for response
	type OrgResponse struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
	}

	formattedOrgs := make([]OrgResponse, len(orgs))
	for i, org := range orgs {
		formattedOrgs[i] = OrgResponse{
			ID:        org.ID,
			Name:      org.Name,
			CreatedAt: org.CreatedAt.Format(time.RFC3339),
		}
	}

	WriteSuccess(w, r, map[string]interface{}{
		"organisations": formattedOrgs,
	}, "Organisations retrieved successfully")
}

// CreateOrganisationRequest represents the request to create an organisation
type CreateOrganisationRequest struct {
	Name string `json:"name"`
}

// createOrganisation creates a new organisation and adds the user as a member
func (h *Handler) createOrganisation(w http.ResponseWriter, r *http.Request) {
	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	var req CreateOrganisationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, r, "Invalid JSON request body")
		return
	}

	// Validate name
	name := strings.TrimSpace(req.Name)
	if name == "" {
		BadRequest(w, r, "name is required")
		return
	}
	if len(name) > 100 {
		BadRequest(w, r, "name must be 100 characters or fewer")
		return
	}

	// Create the organisation
	org, err := h.DB.CreateOrganisation(name)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	// Add user as member
	err = h.DB.AddOrganisationMember(userClaims.UserID, org.ID)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	// Set as active organisation
	err = h.DB.SetActiveOrganisation(userClaims.UserID, org.ID)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	WriteSuccess(w, r, map[string]interface{}{
		"organisation": map[string]interface{}{
			"id":         org.ID,
			"name":       org.Name,
			"created_at": org.CreatedAt.Format(time.RFC3339),
			"updated_at": org.UpdatedAt.Format(time.RFC3339),
		},
	}, "Organisation created successfully")
}

// SwitchOrganisationHandler handles POST /v1/organisations/switch
func (h *Handler) SwitchOrganisationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		MethodNotAllowed(w, r)
		return
	}
	h.switchOrganisation(w, r)
}

// SwitchOrganisationRequest represents the request to switch active organisation
type SwitchOrganisationRequest struct {
	OrganisationID string `json:"organisation_id"`
}

func (h *Handler) switchOrganisation(w http.ResponseWriter, r *http.Request) {
	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	var req SwitchOrganisationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, r, "Invalid JSON request body")
		return
	}

	if req.OrganisationID == "" {
		BadRequest(w, r, "organisation_id is required")
		return
	}

	// Validate membership
	valid, err := h.DB.ValidateOrganisationMembership(userClaims.UserID, req.OrganisationID)
	if err != nil {
		InternalError(w, r, err)
		return
	}
	if !valid {
		Forbidden(w, r, "Not a member of this organisation")
		return
	}

	// Set active organisation
	err = h.DB.SetActiveOrganisation(userClaims.UserID, req.OrganisationID)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	// Get organisation details for response
	org, err := h.DB.GetOrganisation(req.OrganisationID)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	WriteSuccess(w, r, map[string]interface{}{
		"organisation": map[string]interface{}{
			"id":         org.ID,
			"name":       org.Name,
			"created_at": org.CreatedAt.Format(time.RFC3339),
			"updated_at": org.UpdatedAt.Format(time.RFC3339),
		},
	}, "Organisation switched successfully")
}

// UsageHandler handles GET /v1/usage
// Returns current usage statistics for the user's active organisation
func (h *Handler) UsageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	// Get user's active organisation using the helper
	orgID := h.GetActiveOrganisation(w, r)
	if orgID == "" {
		return // Error already written by helper
	}

	// Get usage stats from database
	stats, err := h.DB.GetOrganisationUsageStats(r.Context(), orgID)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	WriteSuccess(w, r, map[string]interface{}{
		"usage": stats,
	}, "Usage statistics retrieved successfully")
}

// PlansHandler handles GET /v1/plans
// Returns available subscription plans
func (h *Handler) PlansHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	plans, err := h.DB.GetActivePlans(r.Context())
	if err != nil {
		InternalError(w, r, err)
		return
	}

	WriteSuccess(w, r, map[string]interface{}{
		"plans": plans,
	}, "Plans retrieved successfully")
}

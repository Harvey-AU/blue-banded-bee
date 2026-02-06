package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/google/uuid"
)

// errInviteEmailExists is returned when the Supabase /invite endpoint reports
// the user already has an account. The invite record is still valid — the
// caller should fall back to a magic-link email so the existing user can log
// in and accept.
var errInviteEmailExists = errors.New("user already registered")

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

	// Add user as admin member
	err = h.DB.AddOrganisationMember(userClaims.UserID, org.ID, "admin")
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
	logger := loggerWithRequest(r)

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

	logger.Info().
		Str("organisation_id", orgID).
		Int("daily_used", stats.DailyUsed).
		Int("daily_limit", stats.DailyLimit).
		Msg("Usage statistics retrieved")

	WriteSuccess(w, r, map[string]interface{}{
		"usage": stats,
	}, "Usage statistics retrieved successfully")
}

// PublicPlan is a DTO for the public /v1/plans endpoint
// Excludes internal metadata fields (is_active, sort_order, created_at)
type PublicPlan struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	DisplayName       string `json:"display_name"`
	DailyPageLimit    int    `json:"daily_page_limit"`
	MonthlyPriceCents int    `json:"monthly_price_cents"`
}

// PlansHandler handles GET /v1/plans
// Returns available subscription plans (public endpoint for pricing page)
func (h *Handler) PlansHandler(w http.ResponseWriter, r *http.Request) {
	logger := loggerWithRequest(r)

	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	plans, err := h.DB.GetActivePlans(r.Context())
	if err != nil {
		InternalError(w, r, err)
		return
	}

	// Transform to public DTOs (filter out internal metadata)
	publicPlans := make([]PublicPlan, len(plans))
	for i, p := range plans {
		publicPlans[i] = PublicPlan{
			ID:                p.ID,
			Name:              p.Name,
			DisplayName:       p.DisplayName,
			DailyPageLimit:    p.DailyPageLimit,
			MonthlyPriceCents: p.MonthlyPriceCents,
		}
	}

	logger.Info().
		Int("plan_count", len(publicPlans)).
		Msg("Plans retrieved")

	WriteSuccess(w, r, map[string]interface{}{
		"plans": publicPlans,
	}, "Plans retrieved successfully")
}

// OrganisationMembersHandler handles GET /v1/organisations/members
func (h *Handler) OrganisationMembersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	orgID := h.GetActiveOrganisation(w, r)
	if orgID == "" {
		return
	}

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

	members, err := h.DB.ListOrganisationMembers(r.Context(), orgID)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	currentRole, err := h.DB.GetOrganisationMemberRole(r.Context(), userClaims.UserID, orgID)
	if err != nil {
		Forbidden(w, r, "Not a member of this organisation")
		return
	}

	responseMembers := make([]map[string]interface{}, 0, len(members))
	for _, member := range members {
		responseMembers = append(responseMembers, map[string]interface{}{
			"id":         member.UserID,
			"email":      member.Email,
			"full_name":  member.FullName,
			"role":       member.Role,
			"created_at": member.CreatedAt.Format(time.RFC3339),
		})
	}

	WriteSuccess(w, r, map[string]interface{}{
		"members":           responseMembers,
		"current_user_id":   userClaims.UserID,
		"current_user_role": currentRole,
	}, "Organisation members retrieved successfully")
}

// OrganisationMemberHandler handles DELETE /v1/organisations/members/:id
func (h *Handler) OrganisationMemberHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		MethodNotAllowed(w, r)
		return
	}

	orgID := h.GetActiveOrganisation(w, r)
	if orgID == "" {
		return
	}

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	if ok := h.requireOrganisationAdmin(w, r, orgID, userClaims.UserID); !ok {
		return
	}

	memberID := strings.TrimPrefix(r.URL.Path, "/v1/organisations/members/")
	if memberID == "" {
		BadRequest(w, r, "member ID is required")
		return
	}

	memberRole, err := h.DB.GetOrganisationMemberRole(r.Context(), memberID, orgID)
	if err != nil {
		BadRequest(w, r, "Member not found")
		return
	}

	if memberRole == "admin" {
		adminCount, err := h.DB.CountOrganisationAdmins(r.Context(), orgID)
		if err != nil {
			InternalError(w, r, err)
			return
		}
		if adminCount <= 1 {
			Forbidden(w, r, "Organisation must have at least one admin")
			return
		}
	}

	if err := h.DB.RemoveOrganisationMember(r.Context(), memberID, orgID); err != nil {
		InternalError(w, r, err)
		return
	}

	WriteSuccess(w, r, map[string]interface{}{
		"member_id": memberID,
	}, "Organisation member removed successfully")
}

// OrganisationInvitesHandler handles GET/POST /v1/organisations/invites
func (h *Handler) OrganisationInvitesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listOrganisationInvites(w, r)
	case http.MethodPost:
		h.createOrganisationInvite(w, r)
	default:
		MethodNotAllowed(w, r)
	}
}

// OrganisationInviteHandler handles DELETE /v1/organisations/invites/:id
func (h *Handler) OrganisationInviteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		MethodNotAllowed(w, r)
		return
	}

	orgID := h.GetActiveOrganisation(w, r)
	if orgID == "" {
		return
	}

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	if ok := h.requireOrganisationAdmin(w, r, orgID, userClaims.UserID); !ok {
		return
	}

	inviteID := strings.TrimPrefix(r.URL.Path, "/v1/organisations/invites/")
	if inviteID == "" {
		BadRequest(w, r, "invite ID is required")
		return
	}

	if err := h.DB.RevokeOrganisationInvite(r.Context(), inviteID, orgID); err != nil {
		InternalError(w, r, err)
		return
	}

	WriteSuccess(w, r, map[string]interface{}{
		"invite_id": inviteID,
	}, "Invite revoked successfully")
}

// OrganisationInviteAcceptHandler handles POST /v1/organisations/invites/accept
func (h *Handler) OrganisationInviteAcceptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		MethodNotAllowed(w, r)
		return
	}

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, r, "Invalid JSON request body")
		return
	}

	if req.Token == "" {
		BadRequest(w, r, "token is required")
		return
	}

	// Ensure user exists in database
	_, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	invite, err := h.DB.GetOrganisationInviteByToken(r.Context(), req.Token)
	if err != nil {
		BadRequest(w, r, "Invite not found")
		return
	}

	if !strings.EqualFold(invite.Email, userClaims.Email) {
		Forbidden(w, r, "Invite email does not match this account")
		return
	}

	acceptedInvite, err := h.DB.AcceptOrganisationInvite(r.Context(), req.Token, userClaims.UserID)
	if err != nil {
		BadRequest(w, r, err.Error())
		return
	}

	if err := h.DB.SetActiveOrganisation(userClaims.UserID, acceptedInvite.OrganisationID); err != nil {
		logger := loggerWithRequest(r)
		logger.Warn().Err(err).Msg("Failed to set active organisation after invite acceptance")
	}

	WriteSuccess(w, r, map[string]interface{}{
		"organisation_id": acceptedInvite.OrganisationID,
		"role":            acceptedInvite.Role,
	}, "Invite accepted successfully")
}

// OrganisationPlanHandler handles PUT /v1/organisations/plan
func (h *Handler) OrganisationPlanHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		MethodNotAllowed(w, r)
		return
	}

	orgID := h.GetActiveOrganisation(w, r)
	if orgID == "" {
		return
	}

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	if ok := h.requireOrganisationAdmin(w, r, orgID, userClaims.UserID); !ok {
		return
	}

	var req struct {
		PlanID string `json:"plan_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, r, "Invalid JSON request body")
		return
	}

	if req.PlanID == "" {
		BadRequest(w, r, "plan_id is required")
		return
	}

	if err := h.DB.SetOrganisationPlan(r.Context(), orgID, req.PlanID); err != nil {
		BadRequest(w, r, err.Error())
		return
	}

	WriteSuccess(w, r, map[string]interface{}{
		"plan_id": req.PlanID,
	}, "Organisation plan updated successfully")
}

// UsageHistoryHandler handles GET /v1/usage/history
func (h *Handler) UsageHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	orgID := h.GetActiveOrganisation(w, r)
	if orgID == "" {
		return
	}

	queryDays := r.URL.Query().Get("days")
	days := 30
	if queryDays != "" {
		if parsed, err := strconv.Atoi(queryDays); err == nil && parsed > 0 && parsed <= 365 {
			days = parsed
		}
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	startDate := today.AddDate(0, 0, -(days - 1))

	entries, err := h.DB.ListDailyUsage(r.Context(), orgID, startDate, today)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	response := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		response = append(response, map[string]interface{}{
			"usage_date":      entry.UsageDate.Format("2006-01-02"),
			"pages_processed": entry.PagesProcessed,
			"jobs_created":    entry.JobsCreated,
		})
	}

	WriteSuccess(w, r, map[string]interface{}{
		"days":  days,
		"usage": response,
	}, "Usage history retrieved successfully")
}

type organisationInviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (h *Handler) listOrganisationInvites(w http.ResponseWriter, r *http.Request) {
	orgID := h.GetActiveOrganisation(w, r)
	if orgID == "" {
		return
	}

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	if ok := h.requireOrganisationAdmin(w, r, orgID, userClaims.UserID); !ok {
		return
	}

	invites, err := h.DB.ListOrganisationInvites(r.Context(), orgID)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	responseInvites := make([]map[string]interface{}, 0, len(invites))
	for _, invite := range invites {
		inviteParams := url.Values{}
		inviteParams.Set("invite_token", invite.Token)
		inviteLink := buildSettingsURL("team", inviteParams, "invites")

		responseInvites = append(responseInvites, map[string]interface{}{
			"id":          invite.ID,
			"email":       invite.Email,
			"role":        invite.Role,
			"invite_link": inviteLink,
			"created_at":  invite.CreatedAt.Format(time.RFC3339),
			"expires_at":  invite.ExpiresAt.Format(time.RFC3339),
		})
	}

	WriteSuccess(w, r, map[string]interface{}{
		"invites": responseInvites,
	}, "Organisation invites retrieved successfully")
}

func (h *Handler) createOrganisationInvite(w http.ResponseWriter, r *http.Request) {
	orgID := h.GetActiveOrganisation(w, r)
	if orgID == "" {
		return
	}

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	if ok := h.requireOrganisationAdmin(w, r, orgID, userClaims.UserID); !ok {
		return
	}

	var req organisationInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, r, "Invalid JSON request body")
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" {
		BadRequest(w, r, "Valid email is required")
		return
	}
	parsedEmail, err := mail.ParseAddress(email)
	if err != nil {
		BadRequest(w, r, "Valid email is required")
		return
	}
	if parsedEmail != nil && parsedEmail.Address != "" {
		email = parsedEmail.Address
	}

	role := strings.TrimSpace(strings.ToLower(req.Role))
	if role == "" {
		role = "member"
	}
	if role != "admin" && role != "member" {
		BadRequest(w, r, "Role must be admin or member")
		return
	}

	isMember, err := h.DB.IsOrganisationMemberEmail(r.Context(), orgID, email)
	if err != nil {
		InternalError(w, r, err)
		return
	}
	if isMember {
		BadRequest(w, r, "User is already a member of this organisation")
		return
	}

	inviteToken := uuid.NewString()
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	invite, err := h.DB.CreateOrganisationInvite(r.Context(), &db.OrganisationInvite{
		OrganisationID: orgID,
		Email:          email,
		Role:           role,
		Token:          inviteToken,
		CreatedBy:      userClaims.UserID,
		ExpiresAt:      expiresAt,
	})
	if err != nil {
		if strings.Contains(err.Error(), "organisation_invites_unique_pending") {
			BadRequest(w, r, "Invite already pending for this email")
			return
		}
		InternalError(w, r, err)
		return
	}

	redirectParams := url.Values{}
	redirectParams.Set("invite_token", inviteToken)
	redirectURL := buildSettingsURL("team", redirectParams, "invites")

	emailDelivery := "sent"
	responseMsg := "Invite sent successfully"

	if err := sendSupabaseInviteEmail(r.Context(), email, redirectURL, map[string]interface{}{
		"organisation_id": orgID,
		"role":            role,
	}); err != nil {
		if errors.Is(err, errInviteEmailExists) {
			// User already has a Supabase Auth account — send a magic link
			// so they receive an email and can log in to accept the invite.
			if mlErr := sendSupabaseMagicLink(r.Context(), email, redirectURL); mlErr != nil {
				logger := loggerWithRequest(r)
				logger.Warn().Err(mlErr).Msg("Failed to send magic link for existing user invite")
				// Invite record is still valid; don't revoke. The invitee
				// can log in manually and accept via the settings page.
				emailDelivery = "failed"
				responseMsg = "Invite created but email delivery failed — the user can log in and accept manually"
			}
		} else {
			if revokeErr := h.DB.RevokeOrganisationInvite(r.Context(), invite.ID, orgID); revokeErr != nil {
				logger := loggerWithRequest(r)
				logger.Warn().Err(revokeErr).Msg("Failed to revoke invite after email failure")
			}
			InternalError(w, r, err)
			return
		}
	}

	WriteCreated(w, r, map[string]interface{}{
		"invite": map[string]interface{}{
			"id":             invite.ID,
			"email":          invite.Email,
			"role":           invite.Role,
			"email_delivery": emailDelivery,
			"created_at":     invite.CreatedAt.Format(time.RFC3339),
			"expires_at":     invite.ExpiresAt.Format(time.RFC3339),
		},
	}, responseMsg)
}

func (h *Handler) requireOrganisationAdmin(w http.ResponseWriter, r *http.Request, organisationID, userID string) bool {
	role, err := h.DB.GetOrganisationMemberRole(r.Context(), userID, organisationID)
	if err != nil {
		Forbidden(w, r, "Not a member of this organisation")
		return false
	}
	if role != "admin" {
		Forbidden(w, r, "Organisation administrator access required")
		return false
	}
	return true
}

type supabaseInviteRequest struct {
	Email      string                 `json:"email"`
	RedirectTo string                 `json:"redirect_to,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
}

// resolveSupabaseAuthURL returns the Supabase Auth base URL, preferring
// SUPABASE_AUTH_URL and falling back to SUPABASE_URL + "/auth/v1".
func resolveSupabaseAuthURL() (string, error) {
	authURL := strings.TrimSuffix(os.Getenv("SUPABASE_AUTH_URL"), "/")
	if authURL == "" {
		legacyURL := strings.TrimSuffix(os.Getenv("SUPABASE_URL"), "/")
		if legacyURL != "" {
			authURL = legacyURL + "/auth/v1"
		}
	}
	if authURL == "" {
		return "", fmt.Errorf("supabase auth URL is not configured")
	}
	if !strings.Contains(authURL, "/auth/") {
		authURL = authURL + "/auth/v1"
	}
	return authURL, nil
}

// supabaseServiceKey returns the service role key or an error.
func supabaseServiceKey() (string, error) {
	key := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	if key == "" {
		return "", fmt.Errorf("supabase service role key is not configured")
	}
	return key, nil
}

// maxErrorBodyBytes caps how much of an error response body we read.
const maxErrorBodyBytes = 4096

func sendSupabaseInviteEmail(ctx context.Context, email, redirectTo string, data map[string]interface{}) error {
	authURL, err := resolveSupabaseAuthURL()
	if err != nil {
		return err
	}

	serviceKey, err := supabaseServiceKey()
	if err != nil {
		return err
	}

	payload, err := json.Marshal(supabaseInviteRequest{
		Email:      email,
		RedirectTo: redirectTo,
		Data:       data,
	})
	if err != nil {
		return fmt.Errorf("failed to build invite payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL+"/invite", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create invite request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+serviceKey)
	req.Header.Set("apikey", serviceKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send invite request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		bodyStr := strings.TrimSpace(string(body))

		// If the user already exists in Supabase Auth, the invite endpoint
		// returns 422 email_exists. Parse the JSON response structurally
		// rather than relying on substring matching.
		if resp.StatusCode == http.StatusUnprocessableEntity {
			var errResp struct {
				ErrorCode string `json:"error_code"`
			}
			if json.Unmarshal(body, &errResp) == nil && errResp.ErrorCode == "email_exists" {
				return errInviteEmailExists
			}
		}

		return fmt.Errorf("supabase invite failed: %s", bodyStr)
	}

	return nil
}

// sendSupabaseMagicLink sends a magic-link email to an existing Supabase Auth
// user. The link logs them in and redirects to redirectTo (the invite
// acceptance page). GoTrue reads redirect_to from the query string.
func sendSupabaseMagicLink(ctx context.Context, email, redirectTo string) error {
	authURL, err := resolveSupabaseAuthURL()
	if err != nil {
		return err
	}

	serviceKey, err := supabaseServiceKey()
	if err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]string{
		"email": email,
	})
	if err != nil {
		return fmt.Errorf("failed to build magic link payload: %w", err)
	}

	endpoint := authURL + "/magiclink"
	if redirectTo != "" {
		endpoint += "?redirect_to=" + url.QueryEscape(redirectTo)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create magic link request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+serviceKey)
	req.Header.Set("apikey", serviceKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send magic link request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return fmt.Errorf("supabase magic link failed: %s", strings.TrimSpace(string(body)))
	}

	return nil
}

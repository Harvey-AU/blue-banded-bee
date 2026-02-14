package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
)

type billingCheckoutRequest struct {
	PlanID string `json:"plan_id"`
}

// BillingHandler handles GET /v1/billing.
func (h *Handler) BillingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	orgID := h.GetActiveOrganisation(w, r)
	if orgID == "" {
		return
	}

	row := h.DB.GetDB().QueryRowContext(r.Context(), `
		SELECT
			o.plan_id,
			p.display_name,
			p.monthly_price_cents,
			o.subscription_status,
			o.paddle_customer_id,
			o.paddle_subscription_id,
			o.current_period_ends_at
		FROM organisations o
		JOIN plans p ON p.id = o.plan_id
		WHERE o.id = $1
	`, orgID)

	var (
		planID             string
		planDisplayName    string
		monthlyPriceCents  int
		subscriptionStatus string
		paddleCustomerID   sql.NullString
		paddleSubID        sql.NullString
		currentPeriodEnds  sql.NullTime
	)
	if err := row.Scan(
		&planID,
		&planDisplayName,
		&monthlyPriceCents,
		&subscriptionStatus,
		&paddleCustomerID,
		&paddleSubID,
		&currentPeriodEnds,
	); err != nil {
		InternalError(w, r, fmt.Errorf("failed to load billing overview: %w", err))
		return
	}

	billingEnabled := strings.TrimSpace(os.Getenv("PADDLE_API_KEY")) != ""

	response := map[string]any{
		"plan_id":              planID,
		"plan_display_name":    planDisplayName,
		"monthly_price_cents":  monthlyPriceCents,
		"subscription_status":  subscriptionStatus,
		"billing_enabled":      billingEnabled,
		"has_customer_account": paddleCustomerID.Valid && paddleCustomerID.String != "",
	}
	if paddleSubID.Valid {
		response["subscription_id"] = paddleSubID.String
	}
	if currentPeriodEnds.Valid {
		response["current_period_ends_at"] = currentPeriodEnds.Time.UTC().Format(time.RFC3339)
	}

	WriteSuccess(w, r, map[string]any{"billing": response}, "Billing overview retrieved successfully")
}

// BillingInvoicesHandler handles GET /v1/billing/invoices.
func (h *Handler) BillingInvoicesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	orgID := h.GetActiveOrganisation(w, r)
	if orgID == "" {
		return
	}

	rows, err := h.DB.GetDB().QueryContext(r.Context(), `
		SELECT invoice_number, status, currency_code, total_amount_cents, billed_at, invoice_url
		FROM billing_invoices
		WHERE organisation_id = $1
		ORDER BY billed_at DESC NULLS LAST, created_at DESC
		LIMIT 50
	`, orgID)
	if err != nil {
		InternalError(w, r, fmt.Errorf("failed to list billing invoices: %w", err))
		return
	}
	defer rows.Close()

	invoices := make([]map[string]any, 0)
	for rows.Next() {
		var (
			number   sql.NullString
			status   string
			currency sql.NullString
			total    int
			billedAt sql.NullTime
			url      sql.NullString
		)
		if err := rows.Scan(&number, &status, &currency, &total, &billedAt, &url); err != nil {
			InternalError(w, r, fmt.Errorf("failed to scan invoice row: %w", err))
			return
		}
		entry := map[string]any{
			"status":              status,
			"currency_code":       strings.ToUpper(strings.TrimSpace(currency.String)),
			"total_amount_cents":  total,
			"invoice_number":      number.String,
			"invoice_url":         url.String,
			"invoice_available":   url.Valid && strings.TrimSpace(url.String) != "",
			"billed_at_timestamp": nil,
		}
		if billedAt.Valid {
			entry["billed_at"] = billedAt.Time.UTC().Format("2006-01-02")
			entry["billed_at_timestamp"] = billedAt.Time.UTC().Format(time.RFC3339)
		}
		invoices = append(invoices, entry)
	}

	if err := rows.Err(); err != nil {
		InternalError(w, r, fmt.Errorf("failed to iterate invoice rows: %w", err))
		return
	}

	WriteSuccess(w, r, map[string]any{"invoices": invoices}, "Invoices retrieved successfully")
}

// BillingCheckoutHandler handles POST /v1/billing/checkout.
func (h *Handler) BillingCheckoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		MethodNotAllowed(w, r)
		return
	}

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	orgID := h.GetActiveOrganisation(w, r)
	if orgID == "" {
		return
	}
	if ok := h.requireOrganisationAdmin(w, r, orgID, userClaims.UserID); !ok {
		return
	}

	var req billingCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, r, "Invalid JSON request body")
		return
	}
	req.PlanID = strings.TrimSpace(req.PlanID)
	if req.PlanID == "" {
		BadRequest(w, r, "plan_id is required")
		return
	}

	var (
		planName          string
		planDisplayName   string
		monthlyPriceCents int
		paddlePriceID     sql.NullString
	)
	err := h.DB.GetDB().QueryRowContext(r.Context(), `
		SELECT name, display_name, monthly_price_cents, paddle_price_id
		FROM plans
		WHERE id = $1 AND is_active = true
	`, req.PlanID).Scan(&planName, &planDisplayName, &monthlyPriceCents, &paddlePriceID)
	if err != nil {
		if err == sql.ErrNoRows {
			BadRequest(w, r, "Plan not found")
			return
		}
		InternalError(w, r, fmt.Errorf("failed to load plan: %w", err))
		return
	}

	// Free tier changes are applied immediately without checkout.
	if monthlyPriceCents == 0 {
		if err := h.DB.SetOrganisationPlan(r.Context(), orgID, req.PlanID); err != nil {
			BadRequest(w, r, err.Error())
			return
		}
		WriteSuccess(w, r, map[string]any{
			"plan_updated": true,
			"plan_name":    planDisplayName,
		}, "Plan updated successfully")
		return
	}

	if strings.TrimSpace(os.Getenv("PADDLE_API_KEY")) == "" {
		ServiceUnavailable(w, r, "Billing is not configured")
		return
	}
	if !paddlePriceID.Valid || strings.TrimSpace(paddlePriceID.String) == "" {
		BadRequest(w, r, fmt.Sprintf("Plan %q is not configured for checkout", planName))
		return
	}

	payload := map[string]any{
		"items": []map[string]any{
			{
				"price_id": paddlePriceID.String,
				"quantity": 1,
			},
		},
		"custom_data": map[string]any{
			"organisation_id": orgID,
			"requested_by":    userClaims.UserID,
			"plan_id":         req.PlanID,
		},
	}

	data, err := h.callPaddleAPI(r.Context(), http.MethodPost, "/transactions", payload)
	if err != nil {
		InternalError(w, r, fmt.Errorf("failed to create checkout transaction: %w", err))
		return
	}

	checkoutURL := extractString(data, "checkout", "url")
	if checkoutURL == "" {
		// Fallback for alternate API payloads.
		checkoutURL = extractString(data, "checkout_url")
	}
	if checkoutURL == "" {
		InternalError(w, r, fmt.Errorf("paddle response missing checkout URL"))
		return
	}

	WriteSuccess(w, r, map[string]any{
		"checkout_url": checkoutURL,
		"plan_name":    planDisplayName,
	}, "Checkout session created")
}

// BillingPortalHandler handles POST /v1/billing/portal.
func (h *Handler) BillingPortalHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		MethodNotAllowed(w, r)
		return
	}

	userClaims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		Unauthorised(w, r, "User information not found")
		return
	}

	orgID := h.GetActiveOrganisation(w, r)
	if orgID == "" {
		return
	}
	if ok := h.requireOrganisationAdmin(w, r, orgID, userClaims.UserID); !ok {
		return
	}

	var customerID sql.NullString
	if err := h.DB.GetDB().QueryRowContext(r.Context(), `
		SELECT paddle_customer_id
		FROM organisations
		WHERE id = $1
	`, orgID).Scan(&customerID); err != nil {
		InternalError(w, r, fmt.Errorf("failed to load billing customer: %w", err))
		return
	}
	if !customerID.Valid || strings.TrimSpace(customerID.String) == "" {
		BadRequest(w, r, "No active billing customer found for this organisation")
		return
	}

	payload := map[string]any{
		"return_url": getAppURL() + "/settings/billing",
	}
	data, err := h.callPaddleAPI(
		r.Context(),
		http.MethodPost,
		fmt.Sprintf("/customers/%s/portal-sessions", customerID.String),
		payload,
	)
	if err != nil {
		InternalError(w, r, fmt.Errorf("failed to create billing portal session: %w", err))
		return
	}

	portalURL := extractString(data, "url")
	if portalURL == "" {
		portalURL = extractString(data, "urls", "general", "overview")
	}
	if portalURL == "" {
		InternalError(w, r, fmt.Errorf("paddle response missing portal URL"))
		return
	}

	WriteSuccess(w, r, map[string]any{
		"portal_url": portalURL,
	}, "Billing portal session created")
}

// PaddleWebhook handles POST /v1/webhooks/paddle.
func (h *Handler) PaddleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		MethodNotAllowed(w, r)
		return
	}

	webhookSecret := strings.TrimSpace(os.Getenv("PADDLE_WEBHOOK_SECRET"))
	if webhookSecret == "" {
		ServiceUnavailable(w, r, "Paddle webhook secret is not configured")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		BadRequest(w, r, "Failed to read webhook payload")
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	if !verifyPaddleSignature(r.Header.Get("Paddle-Signature"), body, webhookSecret) {
		Unauthorised(w, r, "Invalid webhook signature")
		return
	}

	var event struct {
		EventID   string         `json:"event_id"`
		EventType string         `json:"event_type"`
		Data      map[string]any `json:"data"`
		Meta      map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		BadRequest(w, r, "Invalid webhook payload")
		return
	}
	if strings.TrimSpace(event.EventID) == "" || strings.TrimSpace(event.EventType) == "" {
		BadRequest(w, r, "Webhook payload missing event metadata")
		return
	}

	insertRes, err := h.DB.GetDB().ExecContext(r.Context(), `
		INSERT INTO paddle_webhook_events (event_id, event_type, status, received_at)
		VALUES ($1, $2, 'processing', NOW())
		ON CONFLICT (event_id) DO NOTHING
	`, event.EventID, event.EventType)
	if err != nil {
		InternalError(w, r, fmt.Errorf("failed to track webhook event: %w", err))
		return
	}
	if rows, _ := insertRes.RowsAffected(); rows == 0 {
		WriteSuccess(w, r, nil, "Webhook already processed")
		return
	}

	processErr := h.processPaddleWebhookEvent(r.Context(), event.EventType, event.Data)
	status := "processed"
	errMsg := ""
	if processErr != nil {
		status = "failed"
		errMsg = processErr.Error()
	}

	_, _ = h.DB.GetDB().ExecContext(r.Context(), `
		UPDATE paddle_webhook_events
		SET status = $2, processed_at = NOW(), error_message = NULLIF($3, '')
		WHERE event_id = $1
	`, event.EventID, status, errMsg)

	if processErr != nil {
		InternalError(w, r, processErr)
		return
	}

	WriteSuccess(w, r, nil, "Webhook processed successfully")
}

func (h *Handler) processPaddleWebhookEvent(ctx context.Context, eventType string, data map[string]any) error {
	if len(data) == 0 {
		return nil
	}

	orgID := extractString(data, "custom_data", "organisation_id")
	customerID := extractString(data, "customer_id")
	subscriptionID := extractString(data, "id")
	if strings.HasPrefix(eventType, "transaction.") {
		subscriptionID = extractString(data, "subscription_id")
	}
	if orgID == "" && subscriptionID != "" {
		_ = h.DB.GetDB().QueryRowContext(ctx, `
			SELECT id
			FROM organisations
			WHERE paddle_subscription_id = $1
			LIMIT 1
		`, subscriptionID).Scan(&orgID)
	}
	if orgID == "" && customerID != "" {
		_ = h.DB.GetDB().QueryRowContext(ctx, `
			SELECT id
			FROM organisations
			WHERE paddle_customer_id = $1
			LIMIT 1
		`, customerID).Scan(&orgID)
	}
	if orgID == "" {
		return nil
	}

	if strings.HasPrefix(eventType, "subscription.") {
		priceID := extractString(data, "items", "0", "price", "id")
		if priceID == "" {
			priceID = extractString(data, "items", "0", "price_id")
		}
		if subscriptionID == "" {
			subscriptionID = extractString(data, "subscription_id")
		}
		status := extractString(data, "status")
		if status == "" {
			status = "active"
		}
		periodEnd := parseAnyTimestamp(
			extractString(data, "next_billed_at"),
			extractString(data, "current_billing_period", "ends_at"),
		)

		_, err := h.DB.GetDB().ExecContext(ctx, `
			UPDATE organisations o
			SET
				paddle_customer_id = COALESCE(NULLIF($2, ''), o.paddle_customer_id),
				paddle_subscription_id = COALESCE(NULLIF($3, ''), o.paddle_subscription_id),
				subscription_status = COALESCE(NULLIF($4, ''), o.subscription_status),
				current_period_ends_at = COALESCE($5, o.current_period_ends_at),
				plan_id = COALESCE((SELECT id FROM plans WHERE paddle_price_id = NULLIF($6, '') LIMIT 1), o.plan_id),
				paddle_updated_at = NOW(),
				updated_at = NOW()
			WHERE o.id = $1
		`, orgID, customerID, subscriptionID, status, periodEnd, priceID)
		return err
	}

	if strings.HasPrefix(eventType, "transaction.") {
		txID := extractString(data, "id")
		if txID == "" {
			return nil
		}

		status := extractString(data, "status")
		if status == "" {
			status = "paid"
		}
		invoiceID := extractString(data, "invoice_id")
		invoiceNumber := extractString(data, "details", "invoice_number")
		currency := extractString(data, "currency_code")
		if currency == "" {
			currency = extractString(data, "details", "totals", "currency_code")
		}
		totalCents := parseAnyInt(
			extractString(data, "details", "totals", "grand_total"),
			extractString(data, "details", "totals", "total"),
			extractString(data, "totals", "grand_total"),
		)
		billedAt := parseAnyTimestamp(
			extractString(data, "billed_at"),
			extractString(data, "updated_at"),
			extractString(data, "created_at"),
		)
		invoiceURL := extractString(data, "invoice_url")
		if invoiceURL == "" {
			invoiceURL = extractString(data, "details", "receipt_url")
		}

		_, err := h.DB.GetDB().ExecContext(ctx, `
			INSERT INTO billing_invoices (
				organisation_id,
				paddle_transaction_id,
				paddle_invoice_id,
				invoice_number,
				status,
				currency_code,
				total_amount_cents,
				billed_at,
				invoice_url,
				updated_at
			) VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, NULLIF($6, ''), $7, $8, NULLIF($9, ''), NOW())
			ON CONFLICT (paddle_transaction_id)
			DO UPDATE SET
				paddle_invoice_id = EXCLUDED.paddle_invoice_id,
				invoice_number = EXCLUDED.invoice_number,
				status = EXCLUDED.status,
				currency_code = EXCLUDED.currency_code,
				total_amount_cents = EXCLUDED.total_amount_cents,
				billed_at = EXCLUDED.billed_at,
				invoice_url = EXCLUDED.invoice_url,
				updated_at = NOW()
		`, orgID, txID, invoiceID, invoiceNumber, status, currency, totalCents, billedAt, invoiceURL)
		return err
	}

	return nil
}

func (h *Handler) callPaddleAPI(ctx context.Context, method, path string, payload map[string]any) (map[string]any, error) {
	apiKey := strings.TrimSpace(os.Getenv("PADDLE_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("PADDLE_API_KEY is not configured")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("PADDLE_API_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = "https://api.paddle.com"
	}

	var body io.Reader = http.NoBody
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal paddle payload: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to build paddle request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("paddle API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read paddle response: %w", err)
	}

	var parsed map[string]any
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return nil, fmt.Errorf("failed to decode paddle response: %w", err)
		}
	} else {
		parsed = map[string]any{}
	}

	if resp.StatusCode >= 300 {
		msg := extractString(parsed, "error", "detail")
		if msg == "" {
			msg = extractString(parsed, "error", "message")
		}
		if msg == "" {
			msg = string(respBody)
		}
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("paddle API error (%d): %s", resp.StatusCode, msg)
	}

	if dataRaw, ok := parsed["data"]; ok {
		if dataMap, ok := dataRaw.(map[string]any); ok {
			return dataMap, nil
		}
	}
	return parsed, nil
}

func verifyPaddleSignature(signatureHeader string, body []byte, secret string) bool {
	signatureHeader = strings.TrimSpace(signatureHeader)
	if signatureHeader == "" || secret == "" {
		return false
	}

	parts := strings.Split(signatureHeader, ";")
	var (
		ts string
		h1 string
	)
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "ts":
			ts = kv[1]
		case "h1":
			h1 = kv[1]
		}
	}
	if ts == "" || h1 == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte(":"))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(h1))
}

func parseAnyTimestamp(candidates ...string) *time.Time {
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if ts, err := time.Parse(time.RFC3339, c); err == nil {
			t := ts.UTC()
			return &t
		}
	}
	return nil
}

func parseAnyInt(candidates ...string) int {
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if n, err := strconv.Atoi(c); err == nil {
			return n
		}
	}
	return 0
}

func extractString(m map[string]any, path ...string) string {
	if len(path) == 0 || m == nil {
		return ""
	}
	var cur any = m
	for _, segment := range path {
		switch typed := cur.(type) {
		case map[string]any:
			cur = typed[segment]
		case []any:
			idx, err := strconv.Atoi(segment)
			if err != nil || idx < 0 || idx >= len(typed) {
				return ""
			}
			cur = typed[idx]
		default:
			return ""
		}
	}
	switch v := cur.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return ""
	}
}

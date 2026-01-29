package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/rs/zerolog/log"
)

// GA4 API limits and constraints
const (
	// GA4MaxRowsPerRequest is the maximum number of rows the GA4 Data API can return per request
	GA4MaxRowsPerRequest = 250000

	// GA4TokensPerRequest is the approximate number of quota tokens consumed per runReport call
	GA4TokensPerRequest = 10

	// GA4Phase1Limit is the number of top pages fetched immediately (blocking)
	// These pages are available before job processing starts
	GA4Phase1Limit = 100

	// GA4Phase2Limit is the number of pages fetched in the second phase (background)
	// Fetches pages ranked 101-1000 by page views
	GA4Phase2Limit = 900

	// GA4Phase3Limit is the number of pages fetched in the third phase (background)
	// Fetches pages ranked 1001-2000 by page views
	GA4Phase3Limit = 1000

	// Date range lookback periods for analytics queries
	GA4Lookback7Days   = 7
	GA4Lookback28Days  = 28
	GA4Lookback180Days = 180
	GA4Lookback365Days = 365
)

// GA4Client is an HTTP client for the Google Analytics 4 Data API
type GA4Client struct {
	mu           sync.RWMutex
	httpClient   *http.Client
	accessToken  string
	clientID     string
	clientSecret string
}

// PageViewData represents analytics data for a single page
type PageViewData struct {
	HostName      string
	PagePath      string
	PageViews7d   int64
	PageViews28d  int64
	PageViews180d int64
}

// ga4RunReportRequest is the request structure for the GA4 runReport API
type ga4RunReportRequest struct {
	DateRanges []dateRange `json:"dateRanges"`
	Dimensions []dimension `json:"dimensions"`
	Metrics    []metric    `json:"metrics"`
	OrderBys   []orderBy   `json:"orderBys"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
}

type dateRange struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type dimension struct {
	Name string `json:"name"`
}

type metric struct {
	Name string `json:"name"`
}

type orderBy struct {
	Metric metricOrderBy `json:"metric"`
	Desc   bool          `json:"desc"`
}

type metricOrderBy struct {
	MetricName string `json:"metricName"`
}

// ga4RunReportResponse is the response structure from the GA4 runReport API
type ga4RunReportResponse struct {
	Rows []struct {
		DimensionValues []struct {
			Value string `json:"value"`
		} `json:"dimensionValues"`
		MetricValues []struct {
			Value string `json:"value"`
		} `json:"metricValues"`
	} `json:"rows"`
	RowCount int `json:"rowCount"`
}

// tokenRefreshResponse is the OAuth token refresh response
type tokenRefreshResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}

// NewGA4Client creates a new GA4 Data API client
func NewGA4Client(accessToken, clientID, clientSecret string) *GA4Client {
	return &GA4Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		accessToken:  accessToken,
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// RefreshAccessToken exchanges a refresh token for a new access token
// Uses application/x-www-form-urlencoded as required by OAuth 2.0 RFC 6749
func (c *GA4Client) RefreshAccessToken(ctx context.Context, refreshToken string) (string, error) {
	// Build form data per OAuth 2.0 spec (RFC 6749)
	formData := url.Values{}
	formData.Set("client_id", c.clientID)
	formData.Set("client_secret", c.clientSecret)
	formData.Set("refresh_token", refreshToken)
	formData.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(formData.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute token refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenRefreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token refresh response: %w", err)
	}

	log.Debug().
		Int("expires_in", tokenResp.ExpiresIn).
		Msg("Successfully refreshed Google access token")

	return tokenResp.AccessToken, nil
}

// FetchTopPages fetches top N pages ordered by screenPageViews descending
// Returns page data for 7-day, 28-day, and 180-day lookback periods
func (c *GA4Client) FetchTopPages(ctx context.Context, propertyID string, limit, offset int) ([]PageViewData, error) {
	start := time.Now()

	// GA4 supports multiple date ranges in a single request
	// Metric values are returned in the same order as date ranges
	req := ga4RunReportRequest{
		DateRanges: []dateRange{
			{StartDate: "7daysAgo", EndDate: "today"},
			{StartDate: "28daysAgo", EndDate: "today"},
			{StartDate: "180daysAgo", EndDate: "today"},
		},
		Dimensions: []dimension{
			{Name: "hostName"},
			{Name: "pagePath"},
		},
		Metrics: []metric{
			{Name: "screenPageViews"},
		},
		OrderBys: []orderBy{
			{
				Metric: metricOrderBy{MetricName: "screenPageViews"},
				Desc:   true,
			},
		},
		Limit:  limit,
		Offset: offset,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal runReport request: %w", err)
	}

	url := fmt.Sprintf("https://analyticsdata.googleapis.com/v1beta/properties/%s:runReport", propertyID)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create runReport request: %w", err)
	}

	c.mu.RLock()
	token := c.accessToken
	c.mu.RUnlock()

	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	log.Debug().
		Str("property_id", propertyID).
		Int("limit", limit).
		Int("offset", offset).
		Msg("Fetching GA4 report")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute runReport request: %w", err)
	}
	defer resp.Body.Close()

	// Handle non-200 responses
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GA4 API returned status %d: %s", resp.StatusCode, string(body))
	}

	var reportResp ga4RunReportResponse
	if err := json.NewDecoder(resp.Body).Decode(&reportResp); err != nil {
		return nil, fmt.Errorf("failed to decode runReport response: %w", err)
	}

	// Parse response into PageViewData structs
	// Each row contains metric values for each date range in order: 7d, 28d, 180d
	pages := make([]PageViewData, 0, len(reportResp.Rows))
	for _, row := range reportResp.Rows {
		if len(row.DimensionValues) < 2 || len(row.MetricValues) < 3 {
			log.Warn().
				Int("dimensions", len(row.DimensionValues)).
				Int("metrics", len(row.MetricValues)).
				Msg("Skipping malformed GA4 row with insufficient dimensions or metrics")
			continue
		}

		pageViews7d, err := strconv.ParseInt(row.MetricValues[0].Value, 10, 64)
		if err != nil {
			log.Warn().
				Str("value", row.MetricValues[0].Value).
				Err(err).
				Msg("Failed to parse 7d page views as integer")
			pageViews7d = 0
		}

		pageViews28d, err := strconv.ParseInt(row.MetricValues[1].Value, 10, 64)
		if err != nil {
			log.Warn().
				Str("value", row.MetricValues[1].Value).
				Err(err).
				Msg("Failed to parse 28d page views as integer")
			pageViews28d = 0
		}

		pageViews180d, err := strconv.ParseInt(row.MetricValues[2].Value, 10, 64)
		if err != nil {
			log.Warn().
				Str("value", row.MetricValues[2].Value).
				Err(err).
				Msg("Failed to parse 180d page views as integer")
			pageViews180d = 0
		}

		pages = append(pages, PageViewData{
			HostName:      row.DimensionValues[0].Value,
			PagePath:      row.DimensionValues[1].Value,
			PageViews7d:   pageViews7d,
			PageViews28d:  pageViews28d,
			PageViews180d: pageViews180d,
		})
	}

	elapsed := time.Since(start)
	log.Info().
		Str("property_id", propertyID).
		Int("pages_count", len(pages)).
		Int("total_rows", reportResp.RowCount).
		Dur("duration", elapsed).
		Msg("GA4 data fetch completed")

	return pages, nil
}

// FetchTopPagesWithRetry fetches top pages with automatic token refresh on 401
func (c *GA4Client) FetchTopPagesWithRetry(ctx context.Context, propertyID, refreshToken string, limit, offset int) ([]PageViewData, error) {
	pages, err := c.FetchTopPages(ctx, propertyID, limit, offset)
	if err != nil {
		// Check if error is 401 Unauthorised (token expired)
		if isUnauthorisedError(err) {
			log.Info().Msg("Access token expired, refreshing and retrying")

			// Refresh access token
			newAccessToken, refreshErr := c.RefreshAccessToken(ctx, refreshToken)
			if refreshErr != nil {
				return nil, fmt.Errorf("failed to refresh access token: %w", refreshErr)
			}

			c.mu.Lock()
			c.accessToken = newAccessToken
			c.mu.Unlock()

			// Retry request with new token
			pages, err = c.FetchTopPages(ctx, propertyID, limit, offset)
			if err != nil {
				return nil, fmt.Errorf("request failed after token refresh: %w", err)
			}
		} else {
			return nil, err
		}
	}

	return pages, nil
}

// isUnauthorisedError checks if an error indicates a 401 Unauthorised response
func isUnauthorisedError(err error) bool {
	if err == nil {
		return false
	}
	// Check if error message contains "status 401"
	return strings.Contains(err.Error(), "status 401")
}

// DBInterfaceGA4 defines the database operations needed by the progressive fetcher
type DBInterfaceGA4 interface {
	GetActiveGAConnectionForDomain(ctx context.Context, organisationID string, domainID int) (*db.GoogleAnalyticsConnection, error)
	GetGoogleToken(ctx context.Context, connectionID string) (string, error)
	UpdateConnectionLastSync(ctx context.Context, connectionID string) error
	MarkConnectionInactive(ctx context.Context, connectionID, reason string) error
	UpsertPageWithAnalytics(ctx context.Context, organisationID string, domainID int, path string, pageViews map[string]int64, connectionID string) (int, error)
}

// ProgressiveFetcher orchestrates GA4 data fetching in multiple phases
type ProgressiveFetcher struct {
	db           DBInterfaceGA4
	clientID     string
	clientSecret string
}

// NewProgressiveFetcher creates a new progressive fetcher instance
func NewProgressiveFetcher(database DBInterfaceGA4, clientID, clientSecret string) *ProgressiveFetcher {
	return &ProgressiveFetcher{
		db:           database,
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// FetchAndUpdatePages fetches GA4 data in 3 phases and updates the pages table
// Phase 1 is blocking (top 100 pages), phases 2-3 run in background goroutines
func (pf *ProgressiveFetcher) FetchAndUpdatePages(ctx context.Context, organisationID string, domainID int) error {
	start := time.Now()

	// 1. Get active GA4 connection for this domain
	conn, err := pf.db.GetActiveGAConnectionForDomain(ctx, organisationID, domainID)
	if err != nil {
		return fmt.Errorf("failed to get GA4 connection for domain: %w", err)
	}

	// No active connection is not an error - just skip GA4 integration
	if conn == nil {
		log.Debug().
			Str("organisation_id", organisationID).
			Msg("No active GA4 connection, skipping analytics fetch")
		return nil
	}

	// 2. Get refresh token from vault
	refreshToken, err := pf.db.GetGoogleToken(ctx, conn.ID)
	if err != nil {
		log.Error().
			Err(err).
			Str("connection_id", conn.ID).
			Msg("Failed to get refresh token from vault")
		return fmt.Errorf("failed to get refresh token: %w", err)
	}

	// 3. Create GA4 client and refresh access token
	client := NewGA4Client("", pf.clientID, pf.clientSecret)
	accessToken, err := client.RefreshAccessToken(ctx, refreshToken)
	if err != nil {
		// Mark connection inactive on auth failure
		log.Error().
			Err(err).
			Str("connection_id", conn.ID).
			Msg("Failed to refresh access token")

		if markErr := pf.db.MarkConnectionInactive(ctx, conn.ID, "token refresh failed"); markErr != nil {
			log.Error().Err(markErr).Msg("Failed to mark connection inactive after token refresh failure")
		}

		return fmt.Errorf("failed to refresh access token: %w", err)
	}
	client.mu.Lock()
	client.accessToken = accessToken
	client.mu.Unlock()

	// 4. PHASE 1: Fetch top 100 pages (BLOCKING)
	log.Info().
		Str("organisation_id", organisationID).
		Str("property_id", conn.GA4PropertyID).
		Int("phase", 1).
		Msg("Fetching top 100 pages from GA4")

	phase1Data, err := client.FetchTopPagesWithRetry(ctx, conn.GA4PropertyID, refreshToken, GA4Phase1Limit, 0)
	if err != nil {
		log.Error().
			Err(err).
			Str("property_id", conn.GA4PropertyID).
			Int("phase", 1).
			Msg("Failed to fetch GA4 data for phase 1")
		return fmt.Errorf("failed to fetch phase 1 data: %w", err)
	}

	// 5. Upsert phase 1 data immediately
	if err := pf.upsertPageData(ctx, organisationID, domainID, conn.ID, phase1Data); err != nil {
		log.Error().
			Err(err).
			Int("domain_id", domainID).
			Int("pages_count", len(phase1Data)).
			Msg("Failed to upsert phase 1 page data")
		return fmt.Errorf("failed to upsert phase 1 data: %w", err)
	}

	// Log sample of top pages for verification
	sampleSize := 5
	if len(phase1Data) < sampleSize {
		sampleSize = len(phase1Data)
	}
	for i := 0; i < sampleSize; i++ {
		log.Info().
			Str("path", phase1Data[i].PagePath).
			Int64("page_views_7d", phase1Data[i].PageViews7d).
			Str("hostname", phase1Data[i].HostName).
			Msgf("GA4 top page #%d", i+1)
	}

	log.Info().
		Str("organisation_id", organisationID).
		Int("pages_count", len(phase1Data)).
		Dur("duration", time.Since(start)).
		Msg("Phase 1 GA4 fetch completed")

	// 6. PHASE 2 & 3: Fetch remaining pages in background goroutines
	// Use context.Background() so they complete even during shutdown
	// (analytics data is best-effort and shouldn't block graceful shutdown)
	go pf.fetchPhase2Background(context.Background(), organisationID, conn.GA4PropertyID, domainID, conn.ID, client, refreshToken)
	go pf.fetchPhase3Background(context.Background(), organisationID, conn.GA4PropertyID, domainID, conn.ID, client, refreshToken)

	// 7. Update last sync timestamp
	if err := pf.db.UpdateConnectionLastSync(ctx, conn.ID); err != nil {
		// Log but don't fail - this is not critical
		log.Warn().
			Err(err).
			Str("connection_id", conn.ID).
			Msg("Failed to update last sync timestamp")
	}

	return nil
}

// upsertPageData upserts page analytics data into the page_analytics table
func (pf *ProgressiveFetcher) upsertPageData(ctx context.Context, organisationID string, domainID int, connectionID string, pages []PageViewData) error {
	for _, page := range pages {
		pageViews := map[string]int64{
			"7d":   page.PageViews7d,
			"28d":  page.PageViews28d,
			"180d": page.PageViews180d,
		}

		_, err := pf.db.UpsertPageWithAnalytics(ctx, organisationID, domainID, page.PagePath, pageViews, connectionID)
		if err != nil {
			// Log error but continue with other pages
			log.Error().
				Err(err).
				Str("organisation_id", organisationID).
				Int("domain_id", domainID).
				Str("path", page.PagePath).
				Msg("Failed to upsert page with analytics")
		}
	}

	return nil
}

// fetchPhase2Background fetches pages 101-1000 in a background goroutine
func (pf *ProgressiveFetcher) fetchPhase2Background(ctx context.Context, organisationID, propertyID string, domainID int, connectionID string, client *GA4Client, refreshToken string) {
	start := time.Now()

	log.Info().
		Str("property_id", propertyID).
		Int("phase", 2).
		Msg("Starting phase 2 background fetch")

	pages, err := client.FetchTopPagesWithRetry(ctx, propertyID, refreshToken, GA4Phase2Limit, GA4Phase1Limit)
	if err != nil {
		log.Error().
			Err(err).
			Str("property_id", propertyID).
			Int("phase", 2).
			Msg("Failed to fetch GA4 data for phase 2")
		return
	}

	if err := pf.upsertPageData(ctx, organisationID, domainID, connectionID, pages); err != nil {
		log.Error().
			Err(err).
			Int("domain_id", domainID).
			Int("pages_count", len(pages)).
			Msg("Failed to upsert phase 2 page data")
		return
	}

	log.Info().
		Str("property_id", propertyID).
		Int("phase", 2).
		Int("pages_count", len(pages)).
		Dur("duration", time.Since(start)).
		Msg("Phase 2 GA4 fetch completed")
}

// fetchPhase3Background fetches pages 1001-2000 in a background goroutine
func (pf *ProgressiveFetcher) fetchPhase3Background(ctx context.Context, organisationID, propertyID string, domainID int, connectionID string, client *GA4Client, refreshToken string) {
	start := time.Now()

	log.Info().
		Str("property_id", propertyID).
		Int("phase", 3).
		Msg("Starting phase 3 background fetch")

	offset := GA4Phase1Limit + GA4Phase2Limit
	pages, err := client.FetchTopPagesWithRetry(ctx, propertyID, refreshToken, GA4Phase3Limit, offset)
	if err != nil {
		log.Error().
			Err(err).
			Str("property_id", propertyID).
			Int("phase", 3).
			Msg("Failed to fetch GA4 data for phase 3")
		return
	}

	if err := pf.upsertPageData(ctx, organisationID, domainID, connectionID, pages); err != nil {
		log.Error().
			Err(err).
			Int("domain_id", domainID).
			Int("pages_count", len(pages)).
			Msg("Failed to upsert phase 3 page data")
		return
	}

	log.Info().
		Str("property_id", propertyID).
		Int("phase", 3).
		Int("pages_count", len(pages)).
		Dur("duration", time.Since(start)).
		Msg("Phase 3 GA4 fetch completed")
}

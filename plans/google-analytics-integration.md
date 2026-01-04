# Google Analytics 4 Integration Plan

This document outlines the technical plan for integrating Google Analytics 4 (GA4) with Blue Banded Bee to track and store traffic volume data by page/path.

## Table of Contents

1. [OAuth User Consent Flow](#oauth-user-consent-flow)
2. [Comprehensive Implementation Plan](#comprehensive-implementation-plan)
3. [Architecture Overview](#architecture-overview)
4. [Database Schema](#database-schema)
5. [Go Implementation](#go-implementation)
6. [Next Steps](#next-steps)

---

## OAuth User Consent Flow

This is the **recommended approach** for allowing users to connect their own Google Analytics accounts.

### User Experience Flow

1. **User clicks "Connect GA"** → Redirected to Google
2. **Log in to Google** (if not already)
3. **See consent screen** → "Blue Banded Bee wants to view your Analytics"
4. **Click Allow** → Redirected back to your app
5. **See list of their GA4 properties** → "Select a property"
6. **Choose property** → Click "Connect"
7. **Done!** → Your app can now query their GA data

### 1. Google Cloud Console Setup (One-time)

#### a) Create OAuth 2.0 Credentials

- Go to [Google Cloud Console](https://console.cloud.google.com) → APIs & Services → Credentials
- Click "Create Credentials" → "OAuth client ID"
- Application type: "Web application"
- Add authorised redirect URI: `https://yourdomain.com/api/auth/google/callback`
- Save **Client ID** and **Client Secret**

#### b) Enable Required APIs

- **Google Analytics Admin API** (to list accounts/properties)
- **Google Analytics Data API** (to query analytics data)

#### c) Configure OAuth Consent Screen

- Add your app name, logo, privacy policy, etc.
- Add scopes:
  - `https://www.googleapis.com/auth/analytics.readonly`
  - `https://www.googleapis.com/auth/analytics.edit` (if you need write access)

### 2. OAuth Flow Technical Steps

#### Step 1: User clicks "Connect GA" button

```
User → Blue Banded Bee dashboard → "Connect GA" button
```

#### Step 2: Redirect to Google OAuth consent screen

```
Your backend generates OAuth URL:
https://accounts.google.com/o/oauth2/v2/auth?
  client_id=YOUR_CLIENT_ID
  &redirect_uri=https://yourdomain.com/api/auth/google/callback
  &response_type=code
  &scope=https://www.googleapis.com/auth/analytics.readonly
  &access_type=offline      ← Important: gets refresh token
  &prompt=consent           ← Forces consent screen (for refresh token)
```

#### Step 3: User authorises on Google

- User selects their Google account
- Sees consent screen: "Blue Banded Bee wants to view your Google Analytics"
- User clicks "Allow"

#### Step 4: Google redirects back to your app

```
https://yourdomain.com/api/auth/google/callback?code=AUTHORIZATION_CODE
```

#### Step 5: Exchange code for tokens

```
Your backend:
POST https://oauth2.googleapis.com/token
  code=AUTHORIZATION_CODE
  client_id=YOUR_CLIENT_ID
  client_secret=YOUR_CLIENT_SECRET
  redirect_uri=https://yourdomain.com/api/auth/google/callback
  grant_type=authorization_code

Response:
{
  "access_token": "ya29.a0...",
  "refresh_token": "1//0g...",  ← Store this!
  "expires_in": 3600,
  "token_type": "Bearer"
}
```

#### Step 6: List user's GA4 properties

```
Your backend calls Admin API:
GET https://analyticsadmin.googleapis.com/v1beta/accounts
Authorization: Bearer ACCESS_TOKEN

Then for each account:
GET https://analyticsadmin.googleapis.com/v1beta/properties
  ?filter=parent:accounts/ACCOUNT_ID

Response:
{
  "properties": [
    {
      "name": "properties/123456789",
      "displayName": "My Website",
      "propertyType": "PROPERTY_TYPE_ORDINARY"
    }
  ]
}
```

#### Step 7: Show property picker to user

```
User sees list:
[ ] My Website (123456789)
[ ] My Blog (987654321)
[ ] Company Site (555666777)

User selects one → Click "Connect"
```

#### Step 8: Store tokens and property ID

```sql
-- Store in database
INSERT INTO user_ga_connections (
  user_id,
  organisation_id,
  domain_id,
  ga4_property_id,
  access_token,
  refresh_token,
  token_expires_at
) VALUES (...)
```

#### Step 9: Query analytics data

```
Now you can call Data API:
POST https://analyticsdata.googleapis.com/v1beta/properties/123456789:runReport
Authorization: Bearer ACCESS_TOKEN

{
  "dateRanges": [{"startDate": "30daysAgo", "endDate": "today"}],
  "dimensions": [{"name": "pagePath"}],
  "metrics": [{"name": "screenPageViews"}]
}
```

### 3. Required Go Packages

```bash
go get golang.org/x/oauth2
go get golang.org/x/oauth2/google
go get google.golang.org/api/analyticsadmin/v1beta
go get google.golang.org/api/analyticsdata/v1beta
```

### 4. API Endpoints to Implement

```go
// 1. Initiate OAuth flow
GET /api/auth/google/connect
→ Redirect to Google OAuth consent screen

// 2. OAuth callback handler
GET /api/auth/google/callback?code=...
→ Exchange code for tokens
→ Fetch user's GA4 properties
→ Return property list to frontend

// 3. Save selected property
POST /api/auth/google/save
Body: {"property_id": "123456789"}
→ Store tokens + property ID in database

// 4. Query analytics (later)
GET /api/analytics/page-views?domain_id=123
→ Use stored tokens to query Data API
```

### 5. Basic OAuth Configuration (Go)

```go
import (
    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
)

var googleOAuthConfig = &oauth2.Config{
    ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
    ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
    RedirectURL:  "https://yourdomain.com/api/auth/google/callback",
    Scopes: []string{
        "https://www.googleapis.com/auth/analytics.readonly",
    },
    Endpoint: google.Endpoint,
}

// Handler for "Connect GA" button
func handleGoogleConnect(w http.ResponseWriter, r *http.Request) {
    // Generate state token (for CSRF protection)
    state := generateRandomState()
    saveStateToken(state, r)  // Store in session/cookie

    url := googleOAuthConfig.AuthCodeURL(state,
        oauth2.AccessTypeOffline,  // Gets refresh token
        oauth2.ApprovalForce)      // Forces consent screen

    http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// OAuth callback handler
func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
    // 1. Verify state token (CSRF protection)
    // 2. Exchange code for tokens
    code := r.URL.Query().Get("code")
    token, err := googleOAuthConfig.Exchange(context.Background(), code)

    // 3. Fetch user's GA4 properties
    properties, err := listGA4Properties(token.AccessToken)

    // 4. Return to frontend with property list
    json.NewEncoder(w).Encode(properties)
}
```

### 6. Token Refresh Logic

Access tokens expire after 1 hour. You'll need to refresh them:

```go
func getValidToken(ctx context.Context, conn *UserGAConnection) (*oauth2.Token, error) {
    token := &oauth2.Token{
        AccessToken:  conn.AccessToken,
        RefreshToken: conn.RefreshToken,
        Expiry:       conn.TokenExpiresAt,
    }

    // Check if expired
    if token.Expiry.Before(time.Now()) {
        // Refresh the token
        tokenSource := googleOAuthConfig.TokenSource(ctx, token)
        newToken, err := tokenSource.Token()
        if err != nil {
            return nil, err
        }

        // Update database with new token
        updateTokenInDB(conn.ID, newToken)

        return newToken, nil
    }

    return token, nil
}
```

---

## Comprehensive Implementation Plan

This section covers the full architecture and implementation details beyond just the OAuth flow.

### How GA Integrations Work

Google Analytics 4 (GA4) provides a **Data API** that allows you to programmatically query analytics data. The basic flow is:

1. **Authentication**: Use OAuth 2.0 to get user consent and access tokens
2. **API Requests**: Make HTTP requests to GA4's Data API using the `runReport` method
3. **Data Retrieval**: Query specific dimensions (like `pagePath`) and metrics (like `screenPageViews`, `sessions`)
4. **Storage**: Store the returned metrics in your database

### What Data You Can Track

The GA4 Data API provides access to various **page-level metrics**:

#### Metrics (quantitative data)

- `screenPageViews` - Total page views
- `sessions` - Number of sessions that included the page
- `activeUsers` - Active users who viewed the page
- `engagementRate` - User engagement rate
- `averageSessionDuration` - Average time spent

#### Dimensions (how to segment)

- `pagePath` - The page URL path (e.g., `/blog/my-article`)
- `pagePathPlusQueryString` - Path with query parameters
- `date` / `dateHour` - Time-based grouping

You can query historical data (e.g., "last 30 days") and aggregate by day, week, or month.

### Integration Architecture

```
┌─────────────────────────────────────────────────────┐
│  Blue Banded Bee                                    │
│                                                     │
│  ┌──────────────┐      ┌──────────────────────┐   │
│  │   Scheduler  │ ───> │  GA Sync Service     │   │
│  │  (periodic)  │      │  (new component)     │   │
│  └──────────────┘      └──────────────────────┘   │
│                              │                      │
│                              ↓                      │
│                        ┌──────────┐                │
│                        │ GA4 API  │                │
│                        │  Client  │                │
│                        └──────────┘                │
│                              │                      │
│                              ↓                      │
│                        ┌──────────┐                │
│                        │ Database │                │
│                        │  pages   │                │
│                        │  table   │                │
│                        └──────────┘                │
└─────────────────────────────────────────────────────┘
                                │
                                ↓
                    ┌─────────────────────┐
                    │ Google Analytics 4  │
                    │    Data API         │
                    └─────────────────────┘
```

---

## Database Schema

### OAuth Connection Table

```sql
CREATE TABLE user_ga_connections (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    organisation_id UUID NOT NULL REFERENCES organisations(id),
    domain_id INTEGER REFERENCES domains(id),  -- Optional: link to specific domain

    -- GA4 info
    ga4_property_id TEXT NOT NULL,
    ga4_property_name TEXT,

    -- OAuth tokens
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_expires_at TIMESTAMPTZ NOT NULL,

    -- Metadata
    connected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_synced_at TIMESTAMPTZ,

    UNIQUE(organisation_id, ga4_property_id)
);

CREATE INDEX idx_user_ga_connections_user ON user_ga_connections(user_id);
CREATE INDEX idx_user_ga_connections_org ON user_ga_connections(organisation_id);
CREATE INDEX idx_user_ga_connections_domain ON user_ga_connections(domain_id);
```

### Analytics Data Storage

**Option 1: Add columns directly to pages table (simple approach)**

```sql
-- Migration: add_ga_analytics_to_pages.sql
ALTER TABLE pages
ADD COLUMN IF NOT EXISTS last_synced_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS page_views_30d INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS page_views_7d INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS sessions_30d INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS sessions_7d INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS avg_engagement_time_30d REAL DEFAULT 0;
```

**Option 2: Create separate analytics table (historical tracking)**

```sql
CREATE TABLE IF NOT EXISTS page_analytics_snapshots (
    id SERIAL PRIMARY KEY,
    page_id INTEGER REFERENCES pages(id) ON DELETE CASCADE,
    snapshot_date DATE NOT NULL,
    page_views INTEGER DEFAULT 0,
    sessions INTEGER DEFAULT 0,
    active_users INTEGER DEFAULT 0,
    avg_engagement_time REAL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(page_id, snapshot_date)
);

CREATE INDEX idx_page_analytics_page_date
ON page_analytics_snapshots(page_id, snapshot_date DESC);
```

**Recommended: Use both approaches**
- Recent metrics on `pages` for quick access
- Historical snapshots for trend analysis

### Multi-Domain Support

```sql
-- Map each domain to its GA4 property ID
CREATE TABLE domain_ga4_configs (
    domain_id INTEGER PRIMARY KEY REFERENCES domains(id),
    ga4_property_id TEXT NOT NULL,
    ga4_connection_id INTEGER REFERENCES user_ga_connections(id),
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_domain_ga4_configs_property
ON domain_ga4_configs(ga4_property_id);
```

---

## Go Implementation

### Package Structure

```
internal/
├── analytics/
│   ├── ga4_client.go       # GA4 Data API client
│   ├── admin_client.go     # GA4 Admin API client (list properties)
│   ├── oauth.go            # OAuth flow handlers
│   ├── sync.go             # Sync service for periodic data fetching
│   └── models.go           # Data models
└── handlers/
    └── auth_handlers.go    # HTTP handlers for OAuth flow
```

### GA4 Client Implementation

```go
// internal/analytics/ga4_client.go
package analytics

import (
    "context"
    "fmt"
    "strconv"

    "google.golang.org/api/analyticsdata/v1beta"
    "google.golang.org/api/option"
)

type GA4Client struct {
    service    *analyticsdata.Service
    propertyID string
}

func NewGA4Client(ctx context.Context, accessToken, propertyID string) (*GA4Client, error) {
    service, err := analyticsdata.NewService(ctx,
        option.WithTokenSource(oauth2.StaticTokenSource(&oauth2.Token{
            AccessToken: accessToken,
        })))
    if err != nil {
        return nil, err
    }

    return &GA4Client{
        service:    service,
        propertyID: fmt.Sprintf("properties/%s", propertyID),
    }, nil
}

type PageMetrics struct {
    Path            string
    PageViews       int64
    Sessions        int64
    ActiveUsers     int64
    EngagementTime  float64
}

func (c *GA4Client) GetPageMetrics(ctx context.Context, startDate, endDate string) ([]PageMetrics, error) {
    request := &analyticsdata.RunReportRequest{
        DateRanges: []*analyticsdata.DateRange{{
            StartDate: startDate,  // e.g., "30daysAgo" or "2026-01-01"
            EndDate:   endDate,    // e.g., "today" or "2026-01-31"
        }},
        Dimensions: []*analyticsdata.Dimension{
            {Name: "pagePath"},
        },
        Metrics: []*analyticsdata.Metric{
            {Name: "screenPageViews"},
            {Name: "sessions"},
            {Name: "activeUsers"},
            {Name: "userEngagementDuration"},
        },
        Limit: 10000,  // Adjust based on your site size
    }

    response, err := c.service.Properties.RunReport(c.propertyID, request).Context(ctx).Do()
    if err != nil {
        return nil, fmt.Errorf("running report: %w", err)
    }

    results := make([]PageMetrics, 0, len(response.Rows))
    for _, row := range response.Rows {
        if len(row.DimensionValues) == 0 || len(row.MetricValues) < 4 {
            continue
        }

        pageViews, _ := strconv.ParseInt(row.MetricValues[0].Value, 10, 64)
        sessions, _ := strconv.ParseInt(row.MetricValues[1].Value, 10, 64)
        activeUsers, _ := strconv.ParseInt(row.MetricValues[2].Value, 10, 64)
        engagementTime, _ := strconv.ParseFloat(row.MetricValues[3].Value, 64)

        results = append(results, PageMetrics{
            Path:            row.DimensionValues[0].Value,
            PageViews:       pageViews,
            Sessions:        sessions,
            ActiveUsers:     activeUsers,
            EngagementTime:  engagementTime,
        })
    }

    return results, nil
}
```

### Sync Service Implementation

```go
// internal/analytics/sync.go
package analytics

import (
    "context"
    "database/sql"
    "fmt"
    "time"

    "github.com/rs/zerolog/log"
)

type SyncService struct {
    db *sql.DB
}

func NewSyncService(db *sql.DB) *SyncService {
    return &SyncService{db: db}
}

func (s *SyncService) SyncDomainAnalytics(ctx context.Context, domainID int) error {
    // 1. Get GA connection for this domain
    var conn UserGAConnection
    err := s.db.QueryRowContext(ctx, `
        SELECT id, ga4_property_id, access_token, refresh_token, token_expires_at
        FROM user_ga_connections ugc
        JOIN domain_ga4_configs dgc ON ugc.id = dgc.ga4_connection_id
        WHERE dgc.domain_id = $1 AND dgc.enabled = TRUE
    `, domainID).Scan(&conn.ID, &conn.GA4PropertyID, &conn.AccessToken, &conn.RefreshToken, &conn.TokenExpiresAt)

    if err == sql.ErrNoRows {
        return fmt.Errorf("no GA connection found for domain %d", domainID)
    }
    if err != nil {
        return fmt.Errorf("querying GA connection: %w", err)
    }

    // 2. Get valid access token (refresh if needed)
    token, err := getValidToken(ctx, &conn)
    if err != nil {
        return fmt.Errorf("getting valid token: %w", err)
    }

    // 3. Create GA4 client
    client, err := NewGA4Client(ctx, token.AccessToken, conn.GA4PropertyID)
    if err != nil {
        return fmt.Errorf("creating GA4 client: %w", err)
    }

    // 4. Fetch metrics for different time periods
    metrics30d, err := client.GetPageMetrics(ctx, "30daysAgo", "today")
    if err != nil {
        return fmt.Errorf("fetching 30d metrics: %w", err)
    }

    metrics7d, err := client.GetPageMetrics(ctx, "7daysAgo", "today")
    if err != nil {
        return fmt.Errorf("fetching 7d metrics: %w", err)
    }

    // 5. Create lookup map for 7d metrics
    metrics7dMap := make(map[string]PageMetrics)
    for _, m := range metrics7d {
        metrics7dMap[m.Path] = m
    }

    // 6. Update pages table
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("starting transaction: %w", err)
    }
    defer tx.Rollback()

    for _, m30d := range metrics30d {
        m7d := metrics7dMap[m30d.Path]  // Will be zero values if not present

        _, err := tx.ExecContext(ctx, `
            INSERT INTO pages (domain_id, path, page_views_30d, page_views_7d, sessions_30d, sessions_7d, avg_engagement_time_30d, last_synced_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
            ON CONFLICT (domain_id, path)
            DO UPDATE SET
                page_views_30d = EXCLUDED.page_views_30d,
                page_views_7d = EXCLUDED.page_views_7d,
                sessions_30d = EXCLUDED.sessions_30d,
                sessions_7d = EXCLUDED.sessions_7d,
                avg_engagement_time_30d = EXCLUDED.avg_engagement_time_30d,
                last_synced_at = NOW()
        `, domainID, m30d.Path, m30d.PageViews, m7d.PageViews, m30d.Sessions, m7d.Sessions, m30d.EngagementTime)

        if err != nil {
            return fmt.Errorf("updating page %s: %w", m30d.Path, err)
        }
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("committing transaction: %w", err)
    }

    // 7. Update last_synced_at on connection
    _, err = s.db.ExecContext(ctx, `
        UPDATE user_ga_connections
        SET last_synced_at = NOW()
        WHERE id = $1
    `, conn.ID)

    log.Info().
        Int("domain_id", domainID).
        Int("pages_updated", len(metrics30d)).
        Msg("GA analytics synced")

    return nil
}
```

### Environment Configuration

Add to your application configuration:

```bash
# Google OAuth 2.0 Credentials
GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-client-secret

# OAuth Callback URL
GOOGLE_OAUTH_REDIRECT_URL=https://yourdomain.com/api/auth/google/callback

# GA Sync Configuration
GA4_SYNC_INTERVAL_HOURS=24        # How often to sync analytics data
GA4_LOOKBACK_DAYS_30D=30          # Days to look back for 30d metrics
GA4_LOOKBACK_DAYS_7D=7            # Days to look back for 7d metrics
```

### Scheduled Sync Integration

Add to your main application startup:

```go
// cmd/app/main.go
func main() {
    // ... existing setup ...

    // Start GA analytics sync scheduler
    if config.GA4SyncEnabled {
        syncService := analytics.NewSyncService(db)
        go startGAAnalyticsSync(syncService)
    }
}

func startGAAnalyticsSync(syncService *analytics.SyncService) {
    interval := time.Duration(config.GA4SyncIntervalHours) * time.Hour
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    // Run immediately on startup
    if err := syncAllDomains(syncService); err != nil {
        log.Error().Err(err).Msg("initial GA sync failed")
    }

    // Then run on schedule
    for range ticker.C {
        if err := syncAllDomains(syncService); err != nil {
            log.Error().Err(err).Msg("scheduled GA sync failed")
        }
    }
}

func syncAllDomains(syncService *analytics.SyncService) error {
    ctx := context.Background()

    // Get all domains with GA enabled
    domains, err := getDomainsWithGAEnabled(ctx)
    if err != nil {
        return err
    }

    for _, domainID := range domains {
        if err := syncService.SyncDomainAnalytics(ctx, domainID); err != nil {
            log.Error().Err(err).Int("domain_id", domainID).Msg("domain GA sync failed")
            // Continue with other domains
        }
    }

    return nil
}
```

---

## Considerations & Trade-offs

### Pros

- ✅ Prioritise cache warming for high-traffic pages
- ✅ Historical trend analysis
- ✅ ROI metrics for cache warming effectiveness
- ✅ Automated, no manual data entry
- ✅ User controls their own data via OAuth

### Cons

- ⚠️ Requires GA4 to be installed on the site
- ⚠️ API quota limits (apply for higher limits if needed)
- ⚠️ Data is ~24-48 hours delayed in GA4
- ⚠️ Additional complexity in codebase
- ⚠️ OAuth token management and refresh logic required

### API Quotas (Google Analytics Data API)

- **Free tier**: 200,000 requests/day
- Each `runReport` call = 1 request
- Typically 1-2 requests per domain per sync is sufficient
- For high-volume usage, apply for increased quota limits

### Security Considerations

1. **Token Storage**: Encrypt access tokens and refresh tokens at rest
2. **CSRF Protection**: Use state parameter in OAuth flow
3. **Scope Minimisation**: Only request `analytics.readonly` unless write access is needed
4. **Token Rotation**: Implement automatic token refresh
5. **Audit Logging**: Log all GA API access and data syncs

---

## Next Steps

### Phase 1: OAuth Integration (Week 1-2)

1. ✅ Set up Google Cloud Console project and OAuth credentials
2. ✅ Create database migrations for `user_ga_connections` table
3. ✅ Implement OAuth flow handlers (`/connect`, `/callback`, `/save`)
4. ✅ Implement Admin API integration to list accounts/properties
5. ✅ Build frontend UI for "Connect GA" flow
6. ✅ Test with one GA4 property

### Phase 2: Data Sync (Week 2-3)

1. ✅ Create database migrations for analytics columns/tables
2. ✅ Implement GA4 Data API client
3. ✅ Implement sync service with token refresh logic
4. ✅ Add scheduled background sync job
5. ✅ Test data accuracy against GA4 dashboard
6. ✅ Add error handling and retry logic

### Phase 3: Integration & UI (Week 3-4)

1. ✅ Expose analytics data via API endpoints
2. ✅ Update dashboard to show page traffic metrics
3. ✅ Add ability to disconnect/reconnect GA
4. ✅ Implement multi-domain support
5. ✅ Add admin controls for sync frequency
6. ✅ Document user-facing features

### Phase 4: Optimisation (Week 4-5)

1. ✅ Use analytics data to prioritise cache warming (high-traffic pages first)
2. ✅ Add trend analysis (compare 30d vs 7d traffic)
3. ✅ Create reports showing cache warming ROI
4. ✅ Optimise database queries with proper indexes
5. ✅ Monitor API quota usage
6. ✅ Add Sentry alerting for sync failures

---

## References

### Official Documentation

- [Google Analytics Data API Overview](https://developers.google.com/analytics/devguides/reporting/data/v1)
- [GA4 Admin API: List Accounts](https://developers.google.com/analytics/devguides/config/admin/v1/rest/v1beta/accounts/list)
- [GA4 Admin API: List Properties](https://developers.google.com/analytics/devguides/config/admin/v1/rest/v1beta/properties/list)
- [Using OAuth 2.0 for Web Server Applications](https://developers.google.com/identity/protocols/oauth2/web-server)
- [API Dimensions & Metrics Reference](https://developers.google.com/analytics/devguides/reporting/data/v1/api-schema)
- [GA4 Dimensions & Metrics Explorer](https://ga-dev-tools.google/ga4/dimensions-metrics-explorer/)

### Go Libraries

- [Google Analytics Data API v1beta Go Package](https://pkg.go.dev/google.golang.org/api/analyticsdata/v1beta)
- [Google Analytics Admin API v1beta Go Package](https://pkg.go.dev/google.golang.org/api/analyticsadmin/v1beta)
- [Go OAuth2 Package](https://pkg.go.dev/golang.org/x/oauth2)

### Tutorials & Guides

- [OAuth 2.0 for Google Analytics](https://medium.com/@abhiruchichaudhari/oauth-2-0-for-google-analytics-d0d121a68259)
- [Getting Started with OAuth2 in Go](https://medium.com/@pliutau/getting-started-with-oauth2-in-go-2c9fae55d187)
- [Golang Google Analytics: Practical Implementation Guide](https://www.ksred.com/tracking-with-google-analytics-in-go-a-practical-guide/)
- [The Top API Metric and Dimension Combos for GA4](https://calibrate-analytics.com/insights/2023/07/24/The-Top-API-Metric-and-Dimension-Combos-for-Google-Analytics-4/)

---

**Document Version**: 1.0
**Last Updated**: 2026-01-04
**Status**: Planning Phase

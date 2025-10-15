# JWT Signing Keys Migration

Migration from Supabase legacy JWT secrets to new asymmetric JWT signing keys
(RS256).

## Overview

This migration moves from HMAC (HS256) shared secret validation to asymmetric
key (RS256) validation using JWKS (JSON Web Key Set). This provides:

- Better security (no shared secrets to leak)
- Faster verification (no Auth server round-trip needed)
- Better key rotation capabilities
- Compliance with security best practices

## Two-Phase Approach

### Phase 1: Migration (No Code Changes)

Click "Migrate JWT secret" in Supabase dashboard. This imports your existing
HMAC secret into the new system and creates a standby asymmetric key. **All
tokens continue using HMAC - no code changes needed.**

### Phase 2: Rotation (Code Changes Required)

Click "Rotate keys" to start issuing tokens with the asymmetric key. **This
requires updating backend JWT validation code.**

**We're doing both phases in one go since we have no active users.**

## Implementation Steps

### 1. Add JWKS Library (1 minute)

```bash
go get github.com/MicahParks/keyfunc/v3@latest
go mod tidy
```

This library (per
[keyfunc README](https://github.com/MicahParks/keyfunc/tree/v3)) handles:

- JWKS endpoint fetching (`keyfunc.Get`)
- Configurable caching/refresh aligned with Supabase’s 10 minute cache headers
- `kid` (Key ID) matching
- Key rotation
- Thread-safe key lookups

### 2. Update Code (20 minutes)

#### **File: `internal/auth/middleware.go`**

Replace `validateSupabaseToken` function with the JWKS-backed implementation:

```go
import (
    "context"
    "net/http"
    "os"
    "strings"
    "sync"
    "time"

    "github.com/MicahParks/keyfunc/v3"
    "github.com/golang-jwt/jwt/v5"
)

var (
    jwksOnce    sync.Once
    jwksCache   keyfunc.Keyfunc
    jwksInitErr error
)

// getJWKS returns a cached JWKS client bound to Supabase's signing certs.
func getJWKS() (keyfunc.Keyfunc, error) {
    jwksOnce.Do(func() {
        supabaseURL := strings.TrimSuffix(os.Getenv("SUPABASE_URL"), "/")
        if supabaseURL == "" {
            jwksInitErr = fmt.Errorf("SUPABASE_URL environment variable not set")
            return
        }

        jwksURL := fmt.Sprintf("%s/auth/v1/certs", supabaseURL)

        override := keyfunc.Override{
            Client:        &http.Client{Timeout: 5 * time.Second},
            HTTPTimeout:   5 * time.Second,
            RefreshInterval: 10 * time.Minute,
            RefreshErrorHandlerFunc: func(url string) func(ctx context.Context, err error) {
                return func(ctx context.Context, err error) {
                    log.Error().Err(err).Str("jwks_url", url).Msg("JWKS refresh failed")
                }
            },
        }

        childCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        jwksCache, jwksInitErr = keyfunc.NewDefaultOverrideCtx(childCtx, []string{jwksURL}, override)
    })

    if jwksInitErr != nil {
        return nil, jwksInitErr
    }
    return jwksCache, nil
}

func validateSupabaseToken(ctx context.Context, tokenString string) (*UserClaims, error) {
    if ctx == nil {
        ctx = context.Background()
    }
    select {
    case <-ctx.Done():
        return nil, fmt.Errorf("request context cancelled: %w", ctx.Err())
    default:
    }

    jwks, err := getJWKS()
    if err != nil {
        return nil, fmt.Errorf("failed to initialise JWKS: %w", err)
    }

    supabaseURL := strings.TrimSuffix(os.Getenv("SUPABASE_URL"), "/")
    if supabaseURL == "" {
        return nil, fmt.Errorf("SUPABASE_URL environment variable not set")
    }
    issuer := fmt.Sprintf("%s/auth/v1", supabaseURL)

    token, err := jwt.ParseWithClaims(
        tokenString,
        &UserClaims{},
        jwks.Keyfunc,
        jwt.WithIssuer(issuer),
        jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Name}),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to parse token: %w", err)
    }

    claims, ok := token.Claims.(*UserClaims)
    if !ok || !token.Valid {
        return nil, fmt.Errorf("invalid token claims")
    }

    audiences, err := claims.GetAudience()
    if err != nil {
        return nil, fmt.Errorf("failed to read audience: %w", err)
    }
    if len(audiences) == 0 {
        return nil, fmt.Errorf("token missing audience")
    }

    validAudience := false
    for _, aud := range audiences {
        if aud == "authenticated" || aud == "service_role" {
            validAudience = true
            break
        }
    }
    if !validAudience {
        return nil, fmt.Errorf("token has unexpected audience: %v", audiences)
    }

    return claims, nil
}
```

Key points verified against
[Supabase signing-keys docs](https://supabase.com/docs/guides/auth/signing-keys):

- JWKS lives at `https://<project>.supabase.co/auth/v1/certs`
- Access tokens are signed with `RS256`
- `iss` is `https://<project>.supabase.co/auth/v1`
- Client tokens default to `aud: "authenticated"` (service role tokens use
  `aud: "service_role"`; add additional checks if we start validating those)

Update error handling in `AuthMiddlewareWithClient`:

```go
// Remove the SUPABASE_JWT_SECRET error check at line 101-106
// Replace with generic error handling:
if err != nil {
    log.Warn().Err(err).Str("token_prefix", tokenString[:min(10, len(tokenString))]).Msg("JWT validation failed")

    errorMsg := "Invalid authentication token"
    statusCode := http.StatusUnauthorized

    if strings.Contains(err.Error(), "expired") {
        errorMsg = "Authentication token has expired"
        // Don't capture expired tokens - this is normal user behaviour
    } else if strings.Contains(err.Error(), "signature") {
        errorMsg = "Invalid token signature"
        sentry.CaptureException(err)
    } else if strings.Contains(err.Error(), "JWKS") {
        errorMsg = "Authentication service misconfigured"
        statusCode = http.StatusInternalServerError
        sentry.CaptureException(err)
    }

    writeAuthError(w, r, errorMsg, statusCode)
    return
}
```

#### **File: `internal/auth/config.go`**

Remove JWT secret field:

```go
// Config holds Supabase authentication configuration
type Config struct {
    SupabaseURL     string
    SupabaseAnonKey string
    // JWTSecret field removed - no longer needed with JWKS
}

// NewConfigFromEnv creates auth config from environment variables
func NewConfigFromEnv() (*Config, error) {
    config := &Config{
        SupabaseURL:     os.Getenv("SUPABASE_URL"),
        SupabaseAnonKey: os.Getenv("SUPABASE_ANON_KEY"),
    }

    // Validate required environment variables
    if config.SupabaseURL == "" {
        return nil, fmt.Errorf("SUPABASE_URL environment variable is required")
    }
    if config.SupabaseAnonKey == "" {
        return nil, fmt.Errorf("SUPABASE_ANON_KEY environment variable is required")
    }

    return config, nil
}

// Validate ensures all required configuration is present
func (c *Config) Validate() error {
    if c.SupabaseURL == "" {
        return fmt.Errorf("SupabaseURL is required")
    }
    if c.SupabaseAnonKey == "" {
        return fmt.Errorf("SupabaseAnonKey is required")
    }
    return nil
}
```

#### **File: `internal/auth/auth_test.go`**

Update tests to remove JWT secret assertions:

```go
// Update TestNewConfigFromEnv test cases
// Remove envVar "SUPABASE_JWT_SECRET" from all test cases
// Remove config.JWTSecret assertions

// Update TestConfig_Validate
// Remove JWTSecret validation tests
```

Add RS256 verification coverage (see the committed `internal/auth/auth_test.go`
for reference):

- Generate a temporary RSA keypair and publish the public key via
  `httptest.NewServer` serving `/auth/v1/certs`
- Reset the JWKS cache between test runs (`resetJWKSForTest`)
- Exercise happy path, invalid audience, invalid signature, and
  cancelled-context scenarios so failures surface clearly

#### **File: `internal/mocks/auth_client.go`**

Update mock config:

```go
// NewMockAuthConfig creates a mock auth configuration for testing
func NewMockAuthConfig() *auth.Config {
    return &auth.Config{
        SupabaseURL:     "https://test.supabase.co",
        SupabaseAnonKey: "test_anon_key",
        // JWTSecret removed
    }
}
```

#### **File: `.env.example`**

Remove JWT secret line:

```diff
 # Supabase Auth (Production)
 SUPABASE_URL=https://your-project.supabase.co
 SUPABASE_ANON_KEY=your_production_anon_key
-SUPABASE_JWT_SECRET=your_production_jwt_secret
```

### 3. Supabase Dashboard Actions (5 minutes)

1. Go to **Settings** → **API** → **JWT Settings**
2. Click **"Migrate JWT secret"** button
3. Wait for migration to complete (~30 seconds)
4. Click **"Rotate keys"** button to switch to asymmetric keys
5. Done!

### 4. Remove Environment Variables (2 minutes)

Remove `SUPABASE_JWT_SECRET` from:

- Local `.env` file (if you have one)
- Fly.io secrets: `flyctl secrets unset SUPABASE_JWT_SECRET -a blue-banded-bee`
- Any CI/CD environment variables

### 5. Deploy and Test (10 minutes)

1. Run tests: `go test ./internal/auth/...`
2. Test locally: `air`
3. Commit changes
4. Push to trigger deployment
5. Monitor logs for any JWT validation errors
   - Full `go test ./...` may fail in constrained environments because crawler
     integration tests hit `https://httpbin.org`. Running
     `go test ./internal/auth/...` is sufficient to validate this migration.

## What Continues Working

- `auth.GetUserFromContext(r.Context())` - extracts from context, not JWT
- Database user lookups - unchanged
- Existing middleware flow - same behaviour
- RLS policies - same Postgres roles used

## Rollback Plan

If issues occur:

1. In Supabase dashboard, move the asymmetric key to "Previously used"
2. Move the HMAC key back to "In use"
3. Revert code changes
4. Redeploy

The rotation is designed to be reversible.

## Total Time Estimate

- Add dependency: 1 min
- Code updates: 20 min
- Dashboard actions: 5 min
- Environment cleanup: 2 min
- Deploy and test: 10 min

### Total: ~40 minutes

## References

- [Supabase JWT Signing Keys Docs](https://supabase.com/docs/guides/auth/signing-keys)
- [Supabase JWT Signing Keys Blog](https://supabase.com/blog/jwt-signing-keys)
- [keyfunc Library](https://github.com/MicahParks/keyfunc)

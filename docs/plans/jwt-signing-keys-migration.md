# JWT Signing Keys Migration

Quick migration from Supabase legacy JWT secrets to new JWT signing keys.

## Simple 3-Step Process

### 1. Supabase Dashboard (2 minutes)

1. Go to **Settings** → **API** → **JWT Settings**
2. Enable **"Use JWT signing keys"**
3. Done - Supabase now signs tokens with public/private keys

### 2. Remove Environment Variable (1 minute)

Remove `SUPABASE_JWT_SECRET` from:

- `.env` file
- Production environment
- `.env.example`

### 3. Update Code (15 minutes)

**File: `internal/auth/middleware.go`**

Replace the **key function** in `validateSupabaseToken`:

```go
func validateSupabaseToken(tokenString string) (*UserClaims, error) {
    // Parse and validate the token with JWKS public key instead of shared secret
    token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
        // Validate signing method (RSA instead of HMAC)
        if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }

        // Get project ID from SUPABASE_URL
        supabaseURL := os.Getenv("SUPABASE_URL")
        projectID := strings.Split(strings.TrimPrefix(supabaseURL, "https://"), ".")[0]

        // Fetch public key from JWKS endpoint
        resp, err := http.Get(fmt.Sprintf("https://%s.supabase.co/.well-known/jwks.json", projectID))
        if err != nil {
            return nil, err
        }
        defer resp.Body.Close()

        var jwks struct {
            Keys []struct {
                Kty string `json:"kty"`
                N   string `json:"n"`
                E   string `json:"e"`
            } `json:"keys"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
            return nil, err
        }

        // Convert first key to RSA public key (simplified - should match kid)
        return jwkToRSAPublicKey(jwks.Keys[0])
    })

    if err != nil {
        return nil, fmt.Errorf("failed to parse token: %w", err)
    }

    if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
        return claims, nil
    }

    return nil, fmt.Errorf("invalid token claims")
}

// Helper function to convert JWK to RSA public key
func jwkToRSAPublicKey(jwk struct{ Kty, N, E string }) (*rsa.PublicKey, error) {
    // Implementation to convert base64 N,E to RSA public key
    // (Use crypto/rsa and encoding/base64)
}
```

**File: `internal/auth/config.go`**

Remove JWT secret references:

- Remove `JWTSecret` field from `Config` struct
- Remove JWT secret validation

## Additional Considerations

**Database User Lookups**: The existing pattern of:

```go
userClaims, ok := auth.GetUserFromContext(r.Context())
user, err := h.DB.GetOrCreateUser(userClaims.UserID, userClaims.Email, nil)
```

Should continue working unchanged since `GetUserFromContext` just extracts from
request context, not from JWT validation directly.

## That's It!

Total time: ~20 minutes

- No feature flags needed
- No complex migration strategy
- No users to worry about
- Just flip the switch and update the code

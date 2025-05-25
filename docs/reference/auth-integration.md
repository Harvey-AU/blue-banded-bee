# Authentication Integration with Supabase

This document outlines how we leverage Supabase Auth for user authentication in Blue Banded Bee.

## Overview

Blue Banded Bee uses Supabase Auth for authentication, which provides:

- JWT-based authentication
- Social login providers
- Email/password authentication
- Row Level Security (RLS) integration with PostgreSQL
- Session management

## Architecture

1. **Frontend Authentication Flow**:
   - Users authenticate via Supabase Auth client libraries
   - Upon successful authentication, Supabase issues a JWT
   - JWT is stored securely and sent with subsequent requests

2. **Backend Authentication Flow**:
   - Go backend receives JWT in request headers
   - JWT is validated using Supabase JWT secret
   - User claims are extracted and used for authorisation
   - Row Level Security in PostgreSQL further enforces access controls

3. **Integration with User Management**:
   - User data stored in Supabase's `auth` schema
   - Additional user metadata stored in application tables
   - Row Level Security policies control data access at database level

## Implementation Details

### JWT Validation in Go

```go
// JWT validation middleware for Go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract the JWT from the Authorization header
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
            http.Error(w, "Unauthorised", http.StatusUnauthorized)
            return
        }

        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        
        // Validate the JWT using Supabase JWT secret
        claims, err := validateToken(tokenString)
        if err != nil {
            http.Error(w, "Unauthorised: invalid token", http.StatusUnauthorized)
            return
        }
        
        // Add user claims to context
        ctx := context.WithValue(r.Context(), UserContextKey, claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func validateToken(tokenString string) (map[string]interface{}, error) {
    // Parse and validate the token
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        // Validate signing method
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        
        // Get JWT secret from environment
        jwtSecret := []byte(os.Getenv("SUPABASE_JWT_SECRET"))
        return jwtSecret, nil
    })
    
    if err != nil {
        return nil, err
    }
    
    // Extract and return claims
    if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
        return claims, nil
    }
    
    return nil, fmt.Errorf("invalid token")
}
```

### Webflow Integration with Supabase Auth

For Webflow integration:

1. Users authenticate with our app through Supabase Auth
2. Our app obtains Webflow OAuth credentials, which are stored in the database
3. JWT tokens from Supabase Auth are used to authenticate API requests
4. The Designer Extension uses these JWTs for secure server communication

## Environment Configuration

Required environment variables:

```
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_ANON_KEY=your_anon_key
SUPABASE_JWT_SECRET=your_jwt_secret
```

## Database Schema

User-related tables will use Row Level Security (RLS) policies:

```sql
-- Example RLS policy for user data
CREATE POLICY "Users can only access their own data"
ON user_preferences
FOR ALL
USING (auth.uid() = user_id);
```

## Security Considerations

1. **Token Storage**:
   - Store JWTs securely (HTTPOnly cookies recommended)
   - Implement proper token refresh handling

2. **JWT Validation**:
   - Always verify token signature
   - Validate token expiration
   - Check required claims

3. **Row Level Security**:
   - Use RLS policies to enforce data access control
   - Never bypass RLS policies in application code

## Transition Plan

The transition from Clerk to Supabase Auth includes:

1. Update environment variables
2. Implement JWT validation middleware
3. Configure user management tables with RLS
4. Update API handlers to use auth middleware
5. Configure Supabase Auth settings
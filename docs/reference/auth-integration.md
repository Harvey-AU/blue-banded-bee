# Authentication Integration with Supabase

This document outlines how we leverage Supabase Auth for user authentication in Blue Banded Bee.

## Overview

Blue Banded Bee uses Supabase Auth for authentication, which provides:

- JWT-based authentication
- Social login providers (Slack, GitHub, Google, Microsoft, Figma, Facebook, LinkedIn)
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

### Project Structure

Authentication is implemented in the `internal/auth/` package following project conventions:

- `internal/auth/middleware.go` - JWT validation middleware and user context handling
- `internal/auth/config.go` - Supabase configuration management
- `internal/db/users.go` - User-related database operations (planned)

### JWT Validation Implementation

The actual implementation uses structured claims and proper error handling:

```go
// Located in internal/auth/middleware.go
type UserClaims struct {
    jwt.RegisteredClaims
    UserID       string `json:"sub"`
    Email        string `json:"email"`
    AppMetadata  map[string]interface{} `json:"app_metadata"`
    UserMetadata map[string]interface{} `json:"user_metadata"`
    Role         string `json:"role"`
}

func AuthMiddleware(next http.Handler) http.Handler {
    // Implementation validates Supabase JWTs and adds structured UserClaims to context
}

func validateSupabaseToken(tokenString string) (*UserClaims, error) {
    // Uses jwt.ParseWithClaims for proper Supabase JWT structure
}

func GetUserFromContext(ctx context.Context) (*UserClaims, bool) {
    // Helper to extract user claims from request context
}
```

### Social Login Providers

Supported authentication methods (all available to all users):
- **Email/Password** (always available)
- **Google**
- **GitHub**  
- **Microsoft**
- **Slack**
- **Figma**
- **Facebook**
- **LinkedIn**

### Account Linking Strategy

- **One account per user**: Users have a single account with a unique UUID regardless of login method
- **Multiple providers**: Users can link multiple auth providers to their account
- **Smart linking**: Attempts to link accounts via email, but users can manually link or change emails
- **Flexible email**: Users can update their email address; the UUID remains the permanent identifier
- **Provider independence**: Account exists independently of any specific auth provider

### Provider Configuration

Configuration is handled in Supabase dashboard for each provider:

#### Implementation Priority
1. **Phase 1 (MVP)**: Email/Password, Google, GitHub
2. **Phase 2 (Enhanced)**: Microsoft, Slack
3. **Phase 3 (Complete)**: Figma, Facebook, LinkedIn

#### Required Setup per Provider
- **Google**: OAuth 2.0 client credentials from Google Cloud Console
- **GitHub**: OAuth App registration with client ID/secret
- **Microsoft**: Azure AD app registration with client ID/secret
- **Slack**: Slack app with OAuth permissions
- **Figma**: Figma developer app credentials
- **Facebook**: Facebook App ID and secret
- **LinkedIn**: LinkedIn application credentials

### Configuration Management

```go
// Located in internal/auth/config.go
type Config struct {
    SupabaseURL     string
    SupabaseAnonKey string
    JWTSecret       string
}

func NewConfigFromEnv() (*Config, error) {
    // Validates all required environment variables are present
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

User-related tables are added to the existing schema in `internal/db/db.go`:

### Planned Tables

```sql
-- Users table (extends Supabase auth.users)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY REFERENCES auth.users(id),
    email TEXT NOT NULL UNIQUE,
    full_name TEXT,
    organisation_id UUID REFERENCES organisations(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Organisations table for simple sharing model
CREATE TABLE IF NOT EXISTS organisations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Add user/organisation references to existing tables
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id);
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS organisation_id UUID REFERENCES organisations(id);
```

### Row Level Security Policies

```sql
-- Users can only access their own data
CREATE POLICY "Users can access own data" ON users
FOR ALL USING (auth.uid() = id);

-- Organisation members can access shared data
CREATE POLICY "Organisation members can access jobs" ON jobs
FOR ALL USING (
    organisation_id IN (
        SELECT organisation_id FROM users WHERE id = auth.uid()
    )
);
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

## Error Handling

### Authentication Errors

Standardised error responses for authentication failures:

```json
{
  "error": {
    "message": "Authentication token has expired",
    "code": 401,
    "type": "authentication_error"
  }
}
```

Common error messages:
- "Missing or invalid Authorization header"
- "Authentication token has expired" 
- "Invalid token signature"
- "Authentication service misconfigured"

### Session Management

- **Token validation**: Check token validity before each request
- **Automatic refresh**: Frontend should refresh tokens when `refresh_needed: true`
- **Graceful degradation**: Optional auth allows unauthenticated access to public endpoints
- **Error logging**: Invalid tokens are logged with truncated token prefix for debugging

## API Endpoints

### User Registration
`POST /api/auth/register`

Automatically creates a user with an organisation:

```json
{
  "user_id": "uuid-from-supabase",
  "email": "user@company.com", 
  "full_name": "John Smith",
  "org_name": "My Company" // optional
}
```

Response:
```json
{
  "success": true,
  "user": {
    "id": "uuid",
    "email": "user@company.com",
    "full_name": "John Smith",
    "organisation_id": "org-uuid",
    "created_at": "2023-..."
  },
  "organisation": {
    "id": "org-uuid", 
    "name": "My Company",
    "created_at": "2023-..."
  }
}
```

### User Profile
`GET /api/auth/profile?user_id=uuid`

Returns user information with organisation details.

### Job Creation (Protected)
`GET /site?domain=example.com&...`

**Authentication Required**: Bearer token in Authorization header

Creates cache warming jobs linked to the authenticated user and their organisation. All job parameters remain the same, but jobs are now automatically associated with:
- `user_id`: From the authenticated user's JWT token
- `organisation_id`: From the user's organisation membership

Jobs are visible to all members of the same organisation via Row Level Security policies.

### Session Validation
`POST /api/auth/session`

Validates a JWT token and returns session information:

```json
{
  "token": "jwt-token-here"
}
```

Response:
```json
{
  "is_valid": true,
  "expires_at": 1234567890,
  "refresh_needed": false,
  "user_id": "uuid",
  "email": "user@company.com"
}
```

## Organisation Auto-Creation

When users register:
1. If `org_name` provided, uses that name
2. Otherwise, extracts organisation name from email domain
3. Creates organisation automatically and links user
4. User becomes the first member of their organisation

## Planned Security Enhancements

### Audit Logging (Future Implementation)

**Login Activity Tracking:**
- IP addresses for all login events
- Login/logout timestamps
- Device/browser fingerprinting
- Failed login attempt monitoring
- Suspicious activity detection

**Account Change Auditing:**
- Email address changes
- Password resets and changes
- Organisation membership modifications
- Provider linking/unlinking events
- Account deletion requests

**Compliance Features:**
- Data retention policies for audit logs
- User data export capabilities (GDPR)
- Account deletion audit trails

### Session Management & Rate Limiting

**Current Implementation:**
- Basic JWT validation and expiry checking
- Individual request rate limiting (5 req/sec per IP)

**Planned Enhancements:**
- **Concurrent session limits** per user account (prevent 1000+ simultaneous logins)
- **Active job limits** per organisation (prevent system flooding)
- **IP-based login monitoring** to detect unusual access patterns
- **Session invalidation** for security events

**Security Rationale:**
Prevent abuse scenarios where single accounts spawn excessive concurrent sessions to overwhelm the system with job creation requests.

## Transition Plan

The transition from Clerk to Supabase Auth includes:

1. Update environment variables
2. Implement JWT validation middleware
3. Configure user management tables with RLS
4. Update API handlers to use auth middleware
5. Configure Supabase Auth settings
6. Implement session tracking and abuse prevention (Phase 2)
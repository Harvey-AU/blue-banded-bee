# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability within Cache Warmer, please send an email to [hello@teamharvey.co](mailto:hello@teamharvey.co). All security vulnerabilities will be promptly addressed.

Please do not report security vulnerabilities through public GitHub issues.

## Supported Versions

Only the latest version is currently supported with security updates.

## Security Best Practices

When deploying Cache Warmer:

1. Never commit environment files (.env)
2. Use secure environment variables for all secrets
3. Set up proper authentication
4. Follow the principle of least privilege
5. Regularly update dependencies

## System Administrator Role

Blue Banded Bee distinguishes between two types of administrative access:

- **System Administrator** (`system_role: "system_admin"`) - Blue Banded Bee operators with system-level access
- **Organisation Administrator** (`role: "admin"` or `"owner"`) - Client administrators within their organisation

### Setting Up a System Administrator

System administrators can only be configured server-side for security reasons:

1. **In Supabase Dashboard:**
   - Navigate to Authentication > Users
   - Select the user who needs system administrator privileges
   - In the "Raw app_metadata" section, add:
     ```json
     {
       "system_role": "system_admin"
     }
     ```
   - Save the changes

2. **Security Requirements:**
   - System administrator privileges cannot be granted client-side
   - Only Supabase project administrators can modify `app_metadata`
   - Regular users cannot elevate their own permissions

### System Administrator Capabilities

System administrators have access to restricted endpoints such as:

- `/admin/reset-db` - Database schema reset (development environments only)
- Other system-level operations (as implemented)

**Important Security Notes:**
- System administrator endpoints are hidden in production environments
- Database reset functionality requires both `system_role: "system_admin"` and explicit environment configuration:
  - `APP_ENV != "production"` 
  - `ALLOW_DB_RESET=true` environment variable
- All system administrator actions are logged and tracked in Sentry for audit purposes

## Configuration

Sensitive configuration should be set via environment variables, never in code or config files.

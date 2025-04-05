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

## Configuration

Sensitive configuration should be set via environment variables, never in code or config files.

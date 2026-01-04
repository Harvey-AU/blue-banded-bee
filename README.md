[![Fly Deploy](https://github.com/Harvey-AU/blue-banded-bee/actions/workflows/fly-deploy.yml/badge.svg)](https://github.com/Harvey-AU/blue-banded-bee/actions/workflows/fly-deploy.yml)
[![Tests](https://github.com/Harvey-AU/blue-banded-bee/actions/workflows/test.yml/badge.svg)](https://github.com/Harvey-AU/blue-banded-bee/actions/workflows/test.yml)
[![codecov](https://codecov.io/github/harvey-au/blue-banded-bee/graph/badge.svg?token=EC0JW5IU7X)](https://codecov.io/github/harvey-au/blue-banded-bee)
[![Go Report Card](https://goreportcard.com/badge/github.com/Harvey-AU/blue-banded-bee?style=flat)](https://goreportcard.com/report/github.com/Harvey-AU/blue-banded-bee)
[![Go Reference](https://pkg.go.dev/badge/github.com/Harvey-AU/blue-banded-bee.svg)](https://pkg.go.dev/github.com/Harvey-AU/blue-banded-bee)
[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Maintenance](https://img.shields.io/badge/Maintained%3F-yes-green.svg)](https://github.com/Harvey-AU/blue-banded-bee/graphs/commit-activity)

# Blue Banded Bee 🐝

A comprehensive website health and performance tool designed to monitor your site's health by detecting broken links, identifying slow pages, and speeding up slow page by warming cache for optimal performance after publishing, and ensure your visitors always get the best experience. Works with any site and integrates seamlessly with Webflow, Shopify and Wordpress.

Schedule your site audit as often as you need and when your site is published.

Named after
[a special little bee](https://www.aussiebee.com.au/blue-banded-bee-information.html)
native to where we live in Castlemaine, Victoria, Australia.

## Key Features

### Site Health Monitoring

- 🔍 Broken link detection across your entire site
- 🚨 Identify 404s, timeouts, and redirect chains
- 🐌 Detect slow-loading pages and performance bottlenecks
- 📈 Track broken link and performance over time
- ⚡ Lightning fast speed, without being blocked or spamming your site

### Cache Warming

- 🔥 Smart warming with automatic retry on cache MISS
- 🥇 Priority processing - homepage and critical pages first
- ⚡ Improved initial page load times after publishing
- 🤖 Robots.txt compliance with crawl-delay honouring

### Automation & Integration

- 🔄 Scheduled crawls to monitor site health continuously
- 🚀 Automatic runs triggered by Webflow/Shopify publishing events
- 📊 Real-time dashboard with job progress tracking
- 🔔 Slack notifications when jobs complete
- 🔐 Multi-tenant architecture with Supabase Auth
- 🔌 API-first architecture for platform integrations

## Quick Start

```bash
# Clone the repository
git clone https://github.com/Harvey-AU/blue-banded-bee.git
cd blue-banded-bee

# Enable pre-commit hooks for automatic formatting
git config core.hooksPath .githooks

# Start development environment
# Windows:
dev              # Clean output (info level)
dev debug        # Verbose output (debug level)

# Mac/Linux:
./dev.sh         # Clean output (info level)
./dev.sh mac debug  # Verbose output (debug level)
```

One command starts everything:

- ✅ Checks prerequisites (Docker + Supabase CLI)
- 🐳 Starts local Supabase database
- 🔄 Auto-applies migrations
- 🔥 Hot reloading on port 8847
- 📊 Displays helpful URLs for homepage, dashboard, and Supabase Studio
- 🚀 Completely isolated from production
- 🔇 Clean logging by default, verbose mode available

## Status

**Stage 4 of 7** - Core Authentication & MVP Interface (mostly complete)

In progress: Platform integrations (Webflow/Shopify apps) and subscriptions. See
[roadmap](./Roadmap.md) for details.

## Tech Stack

- **Backend**: Go with PostgreSQL
- **Infrastructure**: Fly.io, Cloudflare CDN, Supabase DB and Auth
- **Monitoring**: Sentry, Grafana Cloud (traces), Codecov

## Documentation

- [Getting Started](docs/development/DEVELOPMENT.md)
- [API Reference](docs/architecture/API.md)
- [Architecture Overview](docs/architecture/ARCHITECTURE.md)
- [Supabase Realtime](docs/development/SUPABASE-REALTIME.md)
- [Observability & Tracing](docs/operations/OBSERVABILITY.md)
- [All Documentation →](docs/)

## Support

- [Report Issues](https://github.com/Harvey-AU/blue-banded-bee/issues)
- [Security Policy](SECURITY.md)
- Email: <hello@teamharvey.co>

## License

MIT - See [LICENSE](LICENSE)

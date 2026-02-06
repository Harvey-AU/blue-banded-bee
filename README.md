[![Fly Deploy](https://github.com/Harvey-AU/blue-banded-bee/actions/workflows/fly-deploy.yml/badge.svg)](https://github.com/Harvey-AU/blue-banded-bee/actions/workflows/fly-deploy.yml)
[![Tests](https://github.com/Harvey-AU/blue-banded-bee/actions/workflows/test.yml/badge.svg)](https://github.com/Harvey-AU/blue-banded-bee/actions/workflows/test.yml)
[![codecov](https://codecov.io/github/harvey-au/blue-banded-bee/graph/badge.svg?token=EC0JW5IU7X)](https://codecov.io/github/harvey-au/blue-banded-bee)
[![Go Report Card](https://goreportcard.com/badge/github.com/Harvey-AU/blue-banded-bee?style=flat)](https://goreportcard.com/report/github.com/Harvey-AU/blue-banded-bee)
[![Go Reference](https://pkg.go.dev/badge/github.com/Harvey-AU/blue-banded-bee.svg)](https://pkg.go.dev/github.com/Harvey-AU/blue-banded-bee)
[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Maintenance](https://img.shields.io/badge/Maintained%3F-yes-green.svg)](https://github.com/Harvey-AU/blue-banded-bee/graphs/commit-activity)

# Blue Banded Bee 🐝

A comprehensive website health and performance tool that monitors site health,
detects broken links, identifies slow pages, and warms cache for optimal
performance after publishing. Integrates seamlessly with Webflow via OAuth with
automated scheduling and webhook-triggered crawls.

Keep your site fast and healthy with continuous monitoring and intelligent cache
warming.

Named after
[a special little bee](https://www.aussiebee.com.au/blue-banded-bee-information.html)
native to where we live in Castlemaine, Victoria, Australia.

## Key Features

### Site Health Monitoring

- 🔍 Broken link detection across your entire site
- 🚨 Identify 404s, timeouts, and redirect chains
- 🐌 Detect slow-loading pages and performance bottlenecks
- 📈 Track broken links and performance over time
- ⚡ Lightning fast speed, without being blocked or spamming your site

### Cache Warming

- 🔥 Smart warming with automatic retry on cache MISS
- 🥇 Priority processing - homepage and critical pages first
- ⚡ Improved initial page load times after publishing
- 🤖 Robots.txt compliance with crawl-delay honouring

### Automation & Integration

- 🔄 Scheduled crawls (6/12/24/48 hour intervals) per site
- 🚀 Webflow OAuth integration with auto-crawl on publish webhooks
- 📊 Real-time dashboard with live job progress via WebSockets
- 🔔 Slack notifications via DMs when jobs complete or fail
- 🔐 Multi-organisation support with Supabase Auth and RLS
- 🔌 RESTful API for platform integrations
- 🏷️ Technology detection (CMS, CDN, frameworks)

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

**~65% Complete** - Stage 4 of 7 (Core Authentication & MVP Interface)

**Recent milestones:**

- ✅ Webflow OAuth integration with per-site scheduling and webhooks (v0.23.0)
- ✅ Slack notifications and real-time dashboard updates (v0.20.x)
- ✅ Multi-organisation support with context switching (v0.19.0)
- ✅ Security and compliance testing with CI/CD (Go Report Card: A)

**In progress:** Google Analytics integration, payment infrastructure, platform
SDK

See [Roadmap.md](./Roadmap.md) for detailed progress tracking.

## Tech Stack

- **Backend**: Go 1.25 with PostgreSQL (Supabase)
- **Frontend**: Vanilla JavaScript with data-binding (no build process)
- **Infrastructure**: Fly.io (app + DB), Cloudflare CDN, Supabase (auth +
  realtime)
- **Monitoring**: Sentry (errors), Grafana Cloud (traces), Codecov (coverage)

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

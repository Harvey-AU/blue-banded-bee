[![Fly Deploy](https://github.com/Harvey-AU/blue-banded-bee/actions/workflows/fly-deploy.yml/badge.svg)](https://github.com/Harvey-AU/blue-banded-bee/actions/workflows/fly-deploy.yml)
[![Tests](https://github.com/Harvey-AU/blue-banded-bee/actions/workflows/test.yml/badge.svg)](https://github.com/Harvey-AU/blue-banded-bee/actions/workflows/test.yml)
[![codecov](https://codecov.io/github/harvey-au/blue-banded-bee/graph/badge.svg?token=EC0JW5IU7X)](https://codecov.io/github/harvey-au/blue-banded-bee)
[![Go Report Card](https://goreportcard.com/badge/github.com/Harvey-AU/blue-banded-bee?style=flat)](https://goreportcard.com/report/github.com/Harvey-AU/blue-banded-bee)
[![Go Reference](https://pkg.go.dev/badge/github.com/Harvey-AU/blue-banded-bee.svg)](https://pkg.go.dev/github.com/Harvey-AU/blue-banded-bee)
[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Maintenance](https://img.shields.io/badge/Maintained%3F-yes-green.svg)](https://github.com/Harvey-AU/blue-banded-bee/graphs/commit-activity)

# Blue Banded Bee 🐝

Automatically warm site caches (especially built for [Webflow](https://www.webflow.com)) after publishing to improve initial page load times. Named after [a special little bee](https://www.aussiebee.com.au/blue-banded-bee-information.html) native to where we live in Castlemaine, Victoria, Australia.

## Key Features

### Cache Warming

- 🚀 Concurrent crawling with configurable worker pools
- 🔥 Smart warming with automatic retry on cache MISS
- 🥇 Priority processing - homepage and critical pages first
- 🤖 Robots.txt compliance with crawl-delay honouring

### Integration & Monitoring

- 🔌 Webflow webhook for automatic warming on publish
- 📊 Real-time dashboard with job progress tracking
- 🔐 Multi-tenant architecture with Supabase Auth
- 🎨 Web Components for Webflow integration

## Quick Start

```bash
# Clone the repository
git clone https://github.com/Harvey-AU/blue-banded-bee.git
cd blue-banded-bee

# Set up environment
cp .env.example .env
# Edit .env with your credentials

# Run the service
go run ./cmd/app/main.go
```

## Status

**Stage 4 of 7** - Core Authentication & MVP Interface ✅

Currently working on Stage 5: Subscriptions & Monetisation. See [roadmap](./Roadmap.md) for details.

## Tech Stack

- **Backend**: Go with PostgreSQL
- **Infrastructure**: Fly.io, Cloudflare CDN, Supabase Auth
- **Monitoring**: Sentry, Codecov

## Documentation

- [Getting Started](docs/development/DEVELOPMENT.md)
- [API Reference](docs/architecture/API.md)
- [Architecture Overview](docs/architecture/ARCHITECTURE.md)
- [All Documentation →](docs/)

## Support

- [Report Issues](https://github.com/Harvey-AU/blue-banded-bee/issues)
- [Security Policy](SECURITY.md)
- Email: <hello@teamharvey.co>

## License

MIT - See [LICENSE](LICENSE)

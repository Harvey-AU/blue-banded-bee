[![Fly Deploy](https://github.com/Harvey-AU/blue-banded-bee/actions/workflows/fly-deploy.yml/badge.svg)](https://github.com/Harvey-AU/blue-banded-bee/actions/workflows/fly-deploy.yml)
[![codecov](https://codecov.io/gh/Harvey-AU/blue-banded-bee/graph/badge.svg)](https://codecov.io/gh/Harvey-AU/blue-banded-bee)
[![Go Report Card](https://goreportcard.com/badge/github.com/Harvey-AU/blue-banded-bee)](https://goreportcard.com/report/github.com/Harvey-AU/blue-banded-bee)
[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

# Blue Banded Bee 🐝

Automatically warm site caches (especially built for [Webflow](https://www.webflow.com)) after publishing to improve initial page load times. Named after [a special little bee](https://www.aussiebee.com.au/blue-banded-bee-information.html) native to where we live in Castlemaine, Victoria, Australia.

## Features

- 🚀 Concurrent URL crawling with configurable limits
- 📊 Response time and cache status monitoring
- 🔒 Built-in rate limiting and security features
- 📝 Comprehensive logging and error tracking
- 🗄️ Persistent storage with PostgreSQL database
- 🌐 Intelligent sitemap processing and URL discovery
- 🔄 Automatic link extraction to discover and warm additional pages
- 🔥 Smart cache warming with automatic re-requests on cache MISS
- 🥇 Prioritised task processing to crawl important pages first
- 🔌 Webhook integration for automatic crawling (e.g., on Webflow site publish)
- 🧩 Clean architecture with proper dependency injection
- 🔐 Secure authentication via Supabase Auth with JWT
- 🎨 Template + data binding system for flexible dashboard development
- 📝 Complete form handling with real-time validation
- 🌐 Web Components and data binding library for seamless Webflow integration

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

## Development Status

Current development stage: Stage 4 - Core Authentication & MVP Interface (Complete)

### Project Stages

- ✅ Stage 0: Project Setup & Infrastructure
- ✅ Stage 1: Core Setup & Basic Crawling
- ✅ Stage 2: Multi-domain Support & Job Queue Architecture
- ✅ Stage 3: PostgreSQL Migration & Performance Optimisation
- ✅ Stage 4: Core Authentication & MVP Interface
  - ✅ Supabase authentication system implemented
  - ✅ Core backend and frontend infrastructure complete
- 🔄 Stage 5: Subscriptions & Monetisation
- 🔄 Stage 6: Platform Optimisation & Advanced Features
- 🔄 Stage 7: Feature Refinement & Launch Preparation

See our [detailed roadmap](./Roadmap.md) for more information.

## What's New

- **Webflow Webhook Integration (v0.5.11)**: Automatically trigger cache warming jobs when a Webflow site is published, ensuring your cache is always fresh.
- **Advanced Task Prioritisation (v0.5.19)**: Intelligently prioritises URLs, crawling critical homepage and header links first to warm the most important parts of your site immediately.
- **Enhanced Performance (v0.5.22)**: Switched to the high-performance `pgx` database driver and added in-memory caching for page lookups to dramatically speed up job processing.
- **Smarter Crawling (v0.5.18)**: The crawler now performs comprehensive visibility checks to avoid processing hidden links, reducing unnecessary work.

## Tech Stack

- Backend: Go
- Database: PostgreSQL
- Hosting: Fly.io
- Error Tracking: Sentry
- Cache Layer: Cloudflare
- Authentication: Supabase Auth
- Documentation: Obsidian

## Documentation

### Core Documentation

- **[Architecture](docs/ARCHITECTURE.md)** - System design, components, worker pools, job lifecycle, and technical concepts
- **[Development Guide](docs/DEVELOPMENT.md)** - Setup, local development, testing, debugging, and contributing guidelines
- **[API Reference](docs/API.md)** - Complete REST API endpoints, authentication, and response formats
- **[Database Reference](docs/DATABASE.md)** - PostgreSQL schema, queries, performance optimisation, and operations
- **[Flight Recorder](docs/flight-recorder.md)** - Performance debugging with Go's built-in flight recorder

### Future Plans

- **[UI Implementation](docs/plans/ui-implementation.md)** - Web interface development with Web Components
- **[Webflow Integration](docs/plans/webflow-integration.md)** - Webflow marketplace and Designer extension strategy
- **[Scaling Strategy](docs/plans/_archive/scaling-strategy.md)** - Dynamic worker scaling, priority systems, and performance optimisation

### Project Status

See **[Roadmap.md](./Roadmap.md)** for current development status and completed features.

## Security

See [SECURITY.md](SECURITY.md) for security policy and best practices.

## License

MIT License - See [LICENSE](LICENSE) for details.

## Contact

- Website: [Harvey](https://www.teamharvey.co)
- Support: [hello@teamharvey.co](mailto:hello@teamharvey.co)

For bug reports, please open an issue on GitHub.

# Blue Banded Bee 🐝

Automatically warm site caches (especially built for [Webflow](https://www.webflow.com)) after publishing to improve initial page load times. Named after [a special little bee](https://www.aussiebee.com.au/blue-banded-bee-information.html) that is native to where we live in Castlemaine, Victoria Australia.
## Features

- 🚀 Concurrent URL crawling with configurable limits
- 📊 Response time and cache status monitoring
- 🔒 Built-in rate limiting and security features
- 📝 Comprehensive logging and error tracking
- 🗄️ Persistent storage with PostgreSQL database

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

Current development stage: Stage 3 - PostgreSQL Migration & Performance Optimisation

### Project Stages

- ✅ Stage 0: Project Setup & Infrastructure
- ✅ Stage 1: Core Setup & Basic Crawling
- ✅ Stage 2: Multi-domain Support & Job Queue Architecture
- 🟡 Stage 3: PostgreSQL Migration & Performance Optimisation
- ⭕ Stage 4: Auth & User Management
- ⭕ Stage 5: Billing & Subscriptions
- ⭕ Stage 6: Webflow Integration & Launch

See our [detailed roadmap](./ROADMAP.md) for more information.

## Tech Stack

- Backend: Go
- Database: PostgreSQL
- Hosting: Fly.io
- Error Tracking: Sentry
- Cache Layer: Cloudflare
- Documentation: Obsidian

## Documentation

Our documentation is organized under `docs/`:

- [Codebase Structure](docs/reference/codebase-structure.md) - Overview of the codebase structure
- [File Map](/docs/reference/file-map) - List of files in project
- [API Reference](docs/reference/api-reference.md) - API endpoints and usage
- [Development Guide](docs/guides/development.md) - Setup and local development
- [Deployment Guide](docs/guides/deployment.md) - Deployment instructions
- [Core Concepts](docs/architecture/mental-model.md) - Core concepts and design
- [Implementation Details](docs/architecture/implementation-details.md) - System architecture

## Security

See [SECURITY.md](SECURITY.md) for security policy and best practices.

## License

MIT License - See [LICENSE](LICENSE) for details.

## Contact

- Website: [Team Harvey](https://www.teamharvey.co)
- Support: [hello@teamharvey.co](mailto:hello@teamharvey.co)

For bug reports, please open an issue on GitHub.

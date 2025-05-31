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
  - ✅ RESTful API infrastructure complete
  - ✅ Multi-tenant organisation model
  - ✅ Template + data binding system complete (v0.5.4)
  - ✅ Web Components MVP interface complete
- 🔄 Stage 5: Billing & Subscriptions
- 🔄 Stage 6: Multi-Interface Expansion & Launch

See our [detailed roadmap](./Roadmap.md) for more information.

## Recent Improvements

- **Template + Data Binding System (v0.5.4)**: Complete data binding library with `data-bb-bind`, `data-bb-template`, and `data-bb-form` attributes for flexible dashboard development.
- **Form Processing**: Real-time validation, authentication integration, and automatic API submission for job creation and profile management.
- **Authentication Integration**: Conditional rendering with `data-bb-auth` attributes and seamless Supabase Auth integration.
- **Enhanced Examples**: Complete working examples demonstrating all data binding features with production-ready templates.

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

### Future Plans

- **[UI Implementation](docs/plans/ui-implementation.md)** - Web interface development with Web Components
- **[Webflow Integration](docs/plans/webflow-integration.md)** - Webflow marketplace and Designer extension strategy
- **[Scaling Strategy](docs/plans/scaling-strategy.md)** - Dynamic worker scaling, priority systems, and performance optimisation

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

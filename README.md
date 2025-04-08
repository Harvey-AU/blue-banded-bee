# Blue Banded Bee

Automatically warm Webflow site caches (and other websites) after publishing to improve initial page load times.

## Features

- 🚀 Concurrent URL crawling with configurable limits
- 📊 Response time and cache status monitoring
- 🔒 Built-in rate limiting and security features
- 📝 Comprehensive logging and error tracking
- 🗄️ Persistent storage with Turso database

## Quick Start

```bash
# Clone the repository
git clone https://github.com/Harvey-AU/blue-banded-bee.git
cd blue-banded-bee

# Set up environment
cp .env.example .env
# Edit .env with your credentials

# Run the service
go run src/main.go
```

## Development Status

🚧 Currently in initial development

### Implemented

- ✅ URL crawling with concurrent requests
- ✅ Database integration with Turso
- ✅ Basic error handling
- ✅ Test coverage for core components
- ✅ Rate limiting
- ✅ Fly.io deployment

### Coming Soon

- 🔄 Retry logic
- 🔄 Cache validation improvements
- 🔄 Advanced metrics collection

## Tech Stack

- Backend: Go
- Database: Turso
- Hosting: Fly.io
- Error Tracking: Sentry
- Documentation: Obsidian

## Documentation

Our documentation is maintained in the `docs/` directory:

- [API Reference](docs/api.md) - API endpoints and usage
- [Development Guide](docs/development.md) - Setup and local development
- [Deployment Guide](docs/deployment.md) - Deployment instructions
- [Technical Concepts](docs/concepts.md) - Core concepts and design
- [Architecture](docs/architecture.md) - System architecture

## Security

See [SECURITY.md](SECURITY.md) for security policy and best practices.

## License

MIT License - See [LICENSE](LICENSE) for details.

## Contact

- Website: [Team Harvey](https://www.teamharvey.co)
- Support: [hello@teamharvey.co](mailto:hello@teamharvey.co)

For bug reports, please open an issue on GitHub.

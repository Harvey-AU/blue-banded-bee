# Cache Warmer

Automatically warm Webflow site caches (and other websites) after publishing to improve initial page load times.

## Overview

Cache Warmer crawls your Webflow site after publishing, ensuring all pages are cached and responsive for your users. Integrates directly with Webflow's publishing workflow.

## Development Status

🚧 Currently in initial development

Core functionality implemented:

- ✅ URL crawling with concurrent requests
- ✅ Database integration with Turso
- ✅ Basic error handling
- ✅ Test coverage for core components

Next up:

- 🔄 Rate limiting and retry logic
- 🔄 Fly.io deployment
- 🔄 Cache validation improvements

## Tech Stack

- Backend: Go
- Database: Turso
- Hosting: Fly.io (coming soon)
- Auth: Clerk (planned)
- Payments: Paddle (planned)
- Frontend: Webflow (planned)

## Local Development

### Prerequisites

- Go 1.23 or later
- A Turso database account
- Git

### Setup

1. Clone the repository:

   ```bash
   git clone https://github.com/teamharvey/cache-warmer.git
   cd cache-warmer
   ```

2. Set up environment:

   ```bash
   cp .env.example .env
   # Edit .env with your Turso credentials
   ```

3. Install dependencies:

   ```bash
   go mod download
   ```

4. Run tests:
   ```bash
   go test ./... -v
   ```

### Project Structure

## Environment Setup

- Development: [Coming soon]
- Production: [Coming soon]

## Security

This project is open source but requires secure configuration for deployment:

1. Environment Variables
   - Copy `.env.example` to `.env`
   - Fill in all required credentials
   - Never commit `.env` file
2. API Keys

   - Generate new API keys for each environment
   - Use test keys for development
   - Rotate production keys regularly

3. Deployment
   - Follow security best practices
   - Set up proper monitoring
   - Enable security features in all services

See [SECURITY.md](SECURITY.md) for more details.

## Contributing

Project is currently in initial setup phase.

## License

MIT License

Copyright (c) 2025 Team Harvey

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

## Contact

For support or inquiries:

- Website: [Team Harvey](https://www.teamharvey.co)

For bug reports, please open an issue on GitHub.

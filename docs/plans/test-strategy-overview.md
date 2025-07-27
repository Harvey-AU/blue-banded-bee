# Test Strategy Overview

## Current Testing Implementation

The project has successfully implemented integration testing with real database connections:

### 1. Integration Tests (Implemented)
- **Purpose**: Test database operations and business logic flows
- **Method**: Real Supabase test branch database
- **Infrastructure**: 
  - Local: `.env.test` configuration
  - CI: GitHub Actions with Supabase pooler URLs (IPv4 support)
- **Examples**: 
  - `TestGetJob` - Read a job from DB
  - `TestCreateJob` - Create a job in DB
  - `TestCancelJob` - Update job status in DB
  - `TestProcessSitemapFallback` - Sitemap processing logic
  - `TestEnqueueJobURLs` - Task queue operations

### 2. Unit Tests (Planned)
- **Purpose**: Test complex business logic in isolation
- **Method**: Mocks for dependencies (DB, crawler, etc.)
- **Focus**: Edge cases, error handling, algorithmic correctness
- **Target Areas**:
  - URL validation and normalisation
  - Priority calculation algorithms
  - Error classification logic
  - Retry strategies

## Test Coverage Matrix

| Functionality | Integration Test | Unit Test | Status |
| :------------ | :--------------- | :-------- | :------ |
| Get job from DB | ✅ TestGetJob | ❌ Not needed | Done |
| Create job in DB | ✅ TestCreateJob | ❌ Not needed | Done |
| Cancel job state | ✅ TestCancelJob | ❌ Not needed | Done |
| Sitemap fallback logic | ✅ TestProcessSitemapFallback | 🔴 Planned | Partial |
| Task enqueueing | ✅ TestEnqueueJobURLs | ❌ Not needed | Done |
| Duplicate job detection | ❌ Not suitable | 🔴 Planned | Todo |
| Link prioritisation | ❌ Not suitable | 🔴 Planned | Todo |
| Error classification | ✅ TestIsBlockingError | ✅ Done | Done |
| Retry logic | ❌ Not suitable | 🔴 Planned | Todo |

## Current Status

- **Completed**: Core integration tests with real database
- **CI/CD**: Fully automated testing pipeline with GitHub Actions
- **Coverage**: Tracked via Codecov with badges in README
- **Next Steps**: 
  - Fix remaining test failures (SQL scanning issues)
  - Add unit tests for complex business logic
  - Increase overall test coverage
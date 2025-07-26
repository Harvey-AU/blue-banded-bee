# Test Strategy Overview

## Two-Pronged Testing Approach

We have two complementary testing strategies:

### 1. Integration Tests (`increase-test-coverage.md`)
- **Purpose**: Test database operations and basic CRUD flows
- **Method**: Real Supabase branch database, no mocks
- **Focus**: Simple, practical tests that verify DB interactions work correctly
- **Examples**: 
  - `TestGetJob` - Read a job from DB
  - `TestCreateJob` - Create a job in DB
  - `TestCancelJob` - Update job status in DB

### 2. Unit Tests (`unit-testing-with-testify.md`)
- **Purpose**: Test complex business logic in isolation
- **Method**: Mocks for dependencies (DB, crawler, etc.)
- **Focus**: Edge cases, error handling, algorithmic correctness
- **Examples**:
  - `TestProcessSitemap` - Sitemap fallback logic
  - `TestProcessTask` - Link discovery and prioritization
  - `TestCreateJob` - Duplicate detection logic

## Test Coverage Matrix

| Functionality | Integration Test | Unit Test |
| :------------ | :--------------- | :-------- |
| Get job from DB | ✅ TestGetJob | ❌ Not needed |
| Create job in DB | 🔴 TestCreateJob | ❌ Not needed |
| Cancel job state | ⚪ TestCancelJob | ❌ Not needed |
| Sitemap fallback logic | ⚪ TestProcessSitemapFallback | ⚪ TestProcessSitemap |
| Duplicate job detection | ❌ Not suitable | ⚪ TestCreateJob |
| Link prioritization | ❌ Not suitable | ⚪ TestProcessTask |

## Current Status

- **Active**: Working on integration tests first (simpler to implement)
- **Next**: Will add unit tests for complex logic after basic coverage
- **Goal**: Both test types working together for comprehensive coverage
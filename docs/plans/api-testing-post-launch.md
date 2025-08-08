# API Testing Plan (Post-Launch)

# Edit to test workflows.

This plan documents the comprehensive API testing strategy to be implemented **after** the product launches and has active users. Currently, the project has 17.4% test coverage which adequately covers critical paths for pre-launch.

## Current Test Coverage Status

### ✅ Already Tested (17.4% coverage)

- **Crawler Package**: 30.5% coverage
- **Jobs Package**: 20.9% coverage (integration tests)
- **Main Application**: 12.3% coverage
- **Database operations**: Core functionality covered
- **Critical business logic**: Job management, task processing

### 🚨 Not Yet Tested (0% coverage)

- **API Package** (`internal/api`)
- **Authentication** (`internal/auth`)
- **Database Layer** (`internal/db`)
- **Cache** (`internal/cache`)
- **URL Utilities** (`internal/util`)

## Priority 1: API Testing (Post-Launch)

### Why API Testing is Critical

- Primary user interface - all interactions go through API
- Security implications for authentication endpoints
- Data integrity for job management
- External integrations (Webflow webhooks)

### Test Implementation Strategy

#### Phase 1: Authentication Endpoints

**Complexity: MEDIUM**

- JWT token generation helpers
- Database state management between tests
- Mock external services (email verifier, Sentry)

```go
// Test structure example
//go:build integration

package api_test

func TestAuthRegistration(t *testing.T) {
    // Setup test database
    // Test user registration
    // Verify organisation creation
    // Check JWT response
}

func TestAuthLogin(t *testing.T) {
    // Create test user
    // Test valid login
    // Test invalid credentials
    // Verify JWT tokens
}
```

#### Phase 2: Job Management Endpoints

**Complexity: MEDIUM**

- CRUD operations with authentication
- Progress tracking and status updates
- Webhook handling

```go
func TestCreateJob(t *testing.T) {
    // Setup authenticated request
    // Test job creation
    // Verify database state
    // Check response format
}

func TestJobStatus(t *testing.T) {
    // Create test job
    // Test status retrieval
    // Verify organisation scoping
}
```

#### Phase 3: Dashboard & Reporting

**Complexity: LOW**

- Statistics aggregation
- Activity charts
- Performance metrics

## Priority 2: Database Layer Testing

### Critical Areas

- **Queue Operations**: Concurrent task claiming
- **Transaction Management**: Rollback scenarios
- **RLS Policies**: Multi-tenant data isolation

### Implementation Approach

```go
//go:build integration

func TestConcurrentTaskClaiming(t *testing.T) {
    // Spawn multiple workers
    // Verify no double-processing
    // Check transaction isolation
}
```

## Priority 3: Security Testing

### Authentication Middleware

- JWT validation edge cases
- Token expiry handling
- Invalid token formats
- Session management

### Authorisation

- Organisation boundaries
- User permissions
- API key scoping (future)

## Test Infrastructure Requirements

### 1. Test Helpers

```go
// testutil/api_helpers.go
func CreateTestUser(t *testing.T) (*User, string) // Returns user and JWT
func CreateTestOrganisation(t *testing.T) *Organisation
func CreateTestJob(t *testing.T, userID string) *Job
```

### 2. JWT Generation

```go
// testutil/jwt_helpers.go
func GenerateTestJWT(userID, email string) string
func GenerateExpiredJWT(userID string) string
func GenerateInvalidJWT() string
```

### 3. Database Fixtures

```go
// testutil/fixtures.go
func SeedTestData(t *testing.T)
func CleanupTestData(t *testing.T)
func ResetDatabase(t *testing.T)
```

## Implementation Timeline

### When to Start

- **Trigger**: First paying customer OR 10+ active users
- **Estimated Time**: 2-3 days for full implementation
- **Priority Order**: Auth → Jobs → Database → Others

### Resource Estimate

- **Initial Setup**: 2-3 hours (JWT helpers, test structure)
- **Per Endpoint**: 30-60 minutes
- **Full Coverage**: 1-2 days for API package

## Testing Approach Decision

### Recommended: Integration Tests

**Rationale:**

- Infrastructure already exists (`testutil.LoadTestEnv`)
- Real database behaviour testing
- Matches production scenarios
- Existing patterns to follow

### Alternative: Unit Tests with Mocks

**When to Use:**

- Specific business logic isolation
- Complex algorithmic testing
- Performance-critical code paths

## Coverage Goals

### Phase 1 (Launch + 1 month)

- API authentication endpoints: 80%
- Critical job operations: 80%
- Overall target: 25-30%

### Phase 2 (3 months post-launch)

- All API endpoints: 90%
- Database operations: 70%
- Overall target: 40-50%

### Phase 3 (6 months post-launch)

- Full API coverage: 95%
- Security edge cases: 90%
- Overall target: 60-70%

## Monitoring & Metrics

### What to Track

- Test execution time
- Coverage trends
- Flaky test identification
- Performance regression

### Tools

- Codecov flags (already configured)
- GitHub Actions metrics
- Test result trends

## Risk Mitigation

### Without Tests (Current State)

- **Acceptable Risk**: Pre-launch, no users
- **Mitigation**: Manual testing, careful deployments

### Post-Launch Requirements

- **Unacceptable Risk**: Breaking changes affect users
- **Solution**: Implement this test plan

## Notes

1. **Don't Over-Engineer**: Start simple, add complexity as needed
2. **Focus on User Paths**: Test what users actually do
3. **Maintain Test Speed**: Keep integration tests fast (<5 min total)
4. **Document Patterns**: Create examples for team consistency

## References

- [Current Test Documentation](../testing/README.md)
- [API Documentation](../API.md)
- [Development Guide](../DEVELOPMENT.md)

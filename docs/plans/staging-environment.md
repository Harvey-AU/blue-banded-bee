# Staging Environment Setup Plan

## Overview

This plan outlines the setup of a staging environment using Fly.io for application hosting and Supabase for database services. The staging environment will serve as a production mirror for testing deployments before they reach production.

## Goals

- **100% Production Mirror**: Identical codebase, configuration, and resource allocation
- **Safe Deployment Testing**: Test changes in production-like environment before main deployment
- **Automated Deployment**: Auto-deploy from `staging` branch
- **Data Isolation**: Complete separation between staging and production data

## Architecture

### Application Hosting (Fly.io)
- **Production App**: `blue-banded-bee` (existing)
- **Staging App**: `blue-banded-bee-staging` (new)
- **Resources**: Identical VM specifications to production
- **Region**: Same region as production for consistency

### Database (Supabase)
- **Production**: Existing Supabase project
- **Staging**: New separate Supabase project
- **Rationale**: Complete isolation, clean migration testing, independent scaling

## Implementation Plan

### Phase 1: Supabase Staging Setup

1. **Create New Supabase Project**
   - Project name: `blue-banded-bee-staging`
   - Same region as production
   - Pro plan for production-level features

2. **Schema Migration**
   - Export production schema
   - Apply to staging database
   - Set up identical table structure

3. **Environment Variables**
   ```bash
   STAGING_DATABASE_URL=postgresql://[staging-connection-string]
   ```

### Phase 2: Fly.io Staging App

1. **Create Staging Application**
   ```bash
   fly apps create blue-banded-bee-staging
   ```

2. **Configuration**
   - Copy production `fly.toml` as base
   - Modify app name to `blue-banded-bee-staging`
   - Maintain identical resource allocation

3. **Environment Secrets**
   ```bash
   fly secrets set DATABASE_URL="[staging-supabase-url]" -a blue-banded-bee-staging
   fly secrets set APP_ENV="staging" -a blue-banded-bee-staging
   fly secrets set LOG_LEVEL="debug" -a blue-banded-bee-staging
   fly secrets set SENTRY_DSN="[staging-sentry-dsn]" -a blue-banded-bee-staging
   ```

### Phase 3: CI/CD Pipeline

1. **Branch Strategy**
   - `main` branch → Production deployment
   - `staging` branch → Staging deployment
   - Feature branches → No auto-deployment

2. **GitHub Actions Workflow**
   ```yaml
   name: Deploy Staging
   on:
     push:
       branches: [staging]
   jobs:
     deploy-staging:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v3
         - name: Deploy to Fly.io Staging
           run: fly deploy -a blue-banded-bee-staging
   ```

3. **Deployment Flow**
   - Developer creates feature branch
   - Merge feature → `staging` branch
   - Auto-deploy to staging
   - Test staging environment
   - Merge `staging` → `main` for production

### Phase 4: Testing & Validation

1. **Staging Testing Checklist**
   - [ ] Application starts without errors
   - [ ] Database connections working
   - [ ] All API endpoints functional
   - [ ] Worker pool processing tasks
   - [ ] Job creation and management
   - [ ] Cache warming functionality

2. **Performance Validation**
   - Resource utilisation monitoring
   - Response time benchmarks
   - Database query performance
   - Worker concurrency testing

## Configuration Files

### fly.staging.toml
```toml
app = "blue-banded-bee-staging"
primary_region = "syd"

[build]

[env]
  APP_ENV = "staging"
  LOG_LEVEL = "debug"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = false
  auto_start_machines = true
  min_machines_running = 1
  processes = ["app"]

[[vm]]
  memory = "1gb"
  cpu_kind = "shared"
  cpus = 1
```

### .github/workflows/staging.yml
```yaml
name: Deploy to Staging
on:
  push:
    branches: [staging]

jobs:
  deploy:
    name: Deploy to Staging
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Fly
        uses: superfly/flyctl-actions/setup-flyctl@master
        
      - name: Deploy to Staging
        run: flyctl deploy --app blue-banded-bee-staging --config fly.staging.toml
        env:
          FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}
```

## Cost Analysis

### Monthly Costs (Estimated)

**Supabase Staging Project**
- Pro Plan: $25/month
- Database compute: ~$10-15/month
- **Subtotal**: ~$35-40/month

**Fly.io Staging App**
- VM costs: Match production (~$20-30/month)
- **Subtotal**: ~$20-30/month

**Total Estimated Cost**: ~$55-70/month

### Cost Justification
- **Risk Mitigation**: Prevent production outages from untested deployments
- **Development Velocity**: Faster, safer feature delivery
- **Quality Assurance**: Production-like testing environment
- **Migration Safety**: Test database changes before production

## Benefits

1. **Risk Reduction**
   - Test deployments in production-like environment
   - Catch issues before they reach users
   - Safe database migration testing

2. **Development Workflow**
   - Clear promotion path: feature → staging → production
   - Automated testing pipeline
   - Rollback capabilities

3. **Operational Excellence**
   - Production monitoring and alerting testing
   - Performance benchmarking
   - Load testing capabilities

## Next Steps

1. **Immediate Actions**
   - Create Supabase staging project
   - Set up Fly.io staging app
   - Configure CI/CD pipeline

2. **Future Enhancements**
   - Add staging-specific monitoring
   - Implement automated testing suite
   - Set up load testing scenarios

## Success Criteria

- [ ] Staging environment deployed and accessible
- [ ] Auto-deployment from `staging` branch working
- [ ] All application functionality working in staging
- [ ] Database migrations can be tested safely
- [ ] Performance matches production expectations
- [ ] CI/CD pipeline functioning correctly

---

**Status**: Planning Phase  
**Owner**: Development Team  
**Timeline**: 1-2 weeks implementation  
**Priority**: Medium-High
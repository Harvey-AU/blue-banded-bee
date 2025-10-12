# Simple Load Test Script

Just a bash script that calls your `/v1/jobs` API endpoint to create jobs at
regular intervals.

**Domain Selection:** Shuffles all domains once at start, then cycles through
them sequentially across all batches. This guarantees **all unique domains**
(wraps around if needed).

## Quick Start

```bash
# 1. Get your auth token from the dashboard or API
export AUTH_TOKEN="your-jwt-token-here"

# 2. Run the script (defaults: 5 hours, 30-min intervals, 7 jobs/batch)
./scripts/load-test-simple.sh
```

## Configuration

Set environment variables to customise:

```bash
export API_URL="https://app.bluebandedbee.co"  # Default: http://localhost:8080
export AUTH_TOKEN="your-token"                  # Required!
export BATCH_INTERVAL_MINUTES=30                # Default: 30
export TEST_DURATION_HOURS=5                    # Default: 5
export JOBS_PER_BATCH=7                         # Default: 7

./scripts/load-test-simple.sh
```

## What It Does

1. Picks random domains from a hardcoded list
2. Creates `JOBS_PER_BATCH` jobs via `POST /v1/jobs`
3. Waits `BATCH_INTERVAL_MINUTES` minutes
4. Repeats for `TEST_DURATION_HOURS` hours
5. Logs all created job IDs to `load_test_jobs.csv`

## Example: Quick 1-Hour Test

```bash
export AUTH_TOKEN="eyJ..."
export BATCH_INTERVAL_MINUTES=15
export TEST_DURATION_HOURS=1
export JOBS_PER_BATCH=5

./scripts/load-test-simple.sh
```

This creates 4 batches of 5 jobs = 20 jobs over 1 hour.

## Output

Creates `load_test_jobs.csv`:

```csv
batch,domain,job_id,created_at
1,example.com,abc123,2025-10-12T14:30:00Z
1,fly.io,def456,2025-10-12T14:30:02Z
...
```

## Get Auth Token

### From Dashboard (Browser DevTools)

1. Log into dashboard
2. Open DevTools → Application → Local Storage
3. Copy the `supabase.auth.token` value

### From API

```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"your@email.com","password":"yourpass"}'
```

## Monitor Jobs

```bash
# List all jobs
curl -H "Authorization: Bearer $AUTH_TOKEN" \
  http://localhost:8080/v1/jobs | jq '.data[] | {id, domain, status, progress}'

# Get specific job
curl -H "Authorization: Bearer $AUTH_TOKEN" \
  http://localhost:8080/v1/jobs/abc123 | jq '.data'
```

## Edit Domains

Edit the `DOMAINS` array in the script to use your own test domains:

```bash
DOMAINS=(
  "your-site-1.com"
  "your-site-2.com"
  "your-site-3.com"
)
```

## Stop Early

Press `Ctrl+C` - current batch will finish, then script exits.

## Production Use

For production testing:

```bash
export API_URL="https://app.bluebandedbee.co"
export AUTH_TOKEN="your-production-token"
export BATCH_INTERVAL_MINUTES=60  # Less aggressive
export JOBS_PER_BATCH=3           # Fewer jobs

./scripts/load-test-simple.sh
```

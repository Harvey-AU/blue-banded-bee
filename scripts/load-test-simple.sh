#!/bin/bash
set -e

# Simple Load Test Script
# Calls the /v1/jobs API endpoint to create real jobs at intervals

# Configuration
API_URL="${API_URL:-http://localhost:8080}"
AUTH_TOKEN="${AUTH_TOKEN:-}"
BATCH_INTERVAL_MINUTES="${BATCH_INTERVAL_MINUTES:-30}"
TEST_DURATION_HOURS="${TEST_DURATION_HOURS:-5}"
JOBS_PER_BATCH="${JOBS_PER_BATCH:-7}"

# Colours
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Test domains - 70 diverse real-world sites (mostly small-medium, a few larger)
DOMAINS=(
  # Australian businesses (10) - mostly small-medium corporate sites
  "bankaust.com.au" "australiansuper.com" "qantas.com" "woolworths.com.au" "bunnings.com.au"
  "jbhifi.com.au" "realestate.com.au" "seek.com.au" "kmart.com.au" "bigw.com.au"

  # E-commerce & retail (10) - DTC brands, smaller catalogs
  "merrypeople.com" "aesop.com" "allbirds.com" "everlane.com" "warbyparker.com"
  "casper.com" "glossier.com" "away.com" "brooklinen.com" "kotn.com"

  # Media & blogs (10) - smaller publications, not massive news sites
  "techcrunch.com" "theverge.com" "axios.com" "mashable.com" "lifehacker.com"
  "gizmodo.com" "engadget.com" "polygon.com" "kotaku.com" "theonion.com"

  # WordPress blogs & design sites (10) - typical WP installs
  "smashingmagazine.com" "css-tricks.com" "webdesignerdepot.com" "sitepoint.com" "alistapart.com"
  "designmodo.com" "creativebloq.com" "awwwards.com" "onextrapixel.com" "hongkiat.com"

  # Small business / agency sites (10) - small Aussie agencies
  "studiothink.com.au" "zeroseven.com.au" "humaan.com.au" "noice.com.au" "willandco.com.au"
  "thecontentlab.com.au" "makebold.com.au" "thisisgold.com.au" "wethecollective.com.au" "tworedshoes.com.au"

  # Developer docs (8) - docs sites, not entire platforms
  "docs.github.com" "docs.gitlab.com" "fly.io" "railway.app" "render.com"
  "tailwindcss.com" "nextjs.org" "react.dev"

  # Medium-sized SaaS marketing sites (8) - just marketing sites, not full apps
  "linear.app" "cal.com" "resend.com" "upstash.com" "neon.tech"
  "turso.tech" "convex.dev" "clerk.com"

  # Larger sites for stress testing (4) - these are 5k-15k pages
  "docs.stripe.com" "supabase.com" "vercel.com" "netlify.com"
)

# Check prerequisites
if [ -z "$AUTH_TOKEN" ]; then
  echo -e "${RED}ERROR: AUTH_TOKEN not set${NC}"
  echo "Get a token from your app and run:"
  echo "  export AUTH_TOKEN='your-jwt-token'"
  exit 1
fi

echo -e "${GREEN}=== Simple Load Test ===${NC}"
echo "API URL:           $API_URL"
echo "Batch interval:    $BATCH_INTERVAL_MINUTES minutes"
echo "Test duration:     $TEST_DURATION_HOURS hours"
echo "Jobs per batch:    $JOBS_PER_BATCH"
echo "Available domains: ${#DOMAINS[@]}"
echo ""

# Calculate batches
BATCH_INTERVAL_SECONDS=$((BATCH_INTERVAL_MINUTES * 60))
TEST_DURATION_SECONDS=$((TEST_DURATION_HOURS * 3600))
TOTAL_BATCHES=$((TEST_DURATION_SECONDS / BATCH_INTERVAL_SECONDS))
TOTAL_JOBS=$((TOTAL_BATCHES * JOBS_PER_BATCH))

echo "Will create $TOTAL_JOBS jobs across $TOTAL_BATCHES batches"
echo ""
read -p "Continue? [y/N] " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Cancelled"
  exit 0
fi

# Create job function
create_job() {
  local domain=$1
  local batch_num=$2

  echo -e "${YELLOW}Creating job for $domain (batch $batch_num)${NC}"

  response=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/v1/jobs" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"domain\": \"$domain\",
      \"use_sitemap\": true,
      \"find_links\": true,
      \"max_pages\": 5000,
      \"concurrency\": 3
    }")

  http_code=$(echo "$response" | tail -n1)
  body=$(echo "$response" | sed '$d')

  if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
    job_id=$(echo "$body" | jq -r '.data.id // .job.id // .id' 2>/dev/null || echo "unknown")
    echo -e "${GREEN}✓ Created job $job_id for $domain${NC}"
    echo "$batch_num,$domain,$job_id,$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> load_test_jobs.csv
  else
    echo -e "${RED}✗ Failed to create job for $domain (HTTP $http_code)${NC}"
    echo "$body" | jq '.' 2>/dev/null || echo "$body"
  fi
}

# Initialize results file
echo "batch,domain,job_id,created_at" > load_test_jobs.csv

# Shuffle all domains once at the start for global uniqueness
all_shuffled=("${DOMAINS[@]}")
for ((i=${#all_shuffled[@]}-1; i>0; i--)); do
  j=$((RANDOM % (i+1)))
  temp="${all_shuffled[i]}"
  all_shuffled[i]="${all_shuffled[j]}"
  all_shuffled[j]="$temp"
done

# Track which domain index we're up to
domain_index=0

# Run batches
for ((batch=1; batch<=TOTAL_BATCHES; batch++)); do
  echo ""
  echo -e "${GREEN}=== Batch $batch/$TOTAL_BATCHES ===${NC}"
  echo "$(date)"

  # Select next N domains from shuffled list (wrap around if needed)
  selected_domains=()
  for ((i=0; i<JOBS_PER_BATCH; i++)); do
    selected_domains+=("${all_shuffled[$domain_index]}")
    domain_index=$(( (domain_index + 1) % ${#all_shuffled[@]} ))
  done

  # Create jobs
  for domain in "${selected_domains[@]}"; do
    create_job "$domain" "$batch"
    sleep 2  # Small delay between creates
  done

  # Wait for next batch (unless this is the last one)
  if [ $batch -lt $TOTAL_BATCHES ]; then
    echo ""
    echo -e "${YELLOW}Waiting $BATCH_INTERVAL_MINUTES minutes until next batch...${NC}"
    sleep $BATCH_INTERVAL_SECONDS
  fi
done

echo ""
echo -e "${GREEN}=== Load test complete ===${NC}"
echo "Created jobs logged to: load_test_jobs.csv"
echo ""
echo "Check job status with:"
echo "  curl -H 'Authorization: Bearer \$AUTH_TOKEN' $API_URL/v1/jobs"

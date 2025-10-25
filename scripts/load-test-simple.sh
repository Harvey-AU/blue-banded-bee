#!/bin/bash
set -e

# Simple Load Test Script
# Calls the /v1/jobs API endpoint to create real jobs at intervals
# Creates one job per unique domain in the DOMAINS array
# Usage: ./load-test-simple.sh [interval:VALUE] [jobs:N]
#  - VALUE can be minutes (default), e.g. `interval:2` or `interval:2m`
#  - or seconds via an `s` suffix, e.g. `interval:45s`
#  - `batch:N` remains as a backwards-compatible alias for minutes
# Example: ./load-test-simple.sh interval:1m jobs:10

# Parse command line arguments
BATCH_INTERVAL_SECONDS=180
JOBS_PER_BATCH=3

parse_interval_seconds() {
  local raw="$1"

  if [[ "$raw" =~ ^[0-9]+[sS]$ ]]; then
    echo "${raw%[sS]}"
  elif [[ "$raw" =~ ^[0-9]+[mM]$ ]]; then
    echo $(( ${raw%[mM]} * 60 ))
  elif [[ "$raw" =~ ^[0-9]+$ ]]; then
    echo $(( raw * 60 ))
  else
    return 1
  fi
}

format_interval() {
  local seconds=$1

  if (( seconds <= 0 )); then
    echo "0s"
  elif (( seconds < 60 )); then
    echo "${seconds}s"
  elif (( seconds % 60 == 0 )); then
    local minutes=$(( seconds / 60 ))
    if (( minutes == 1 )); then
      echo "1 minute"
    else
      echo "$minutes minutes"
    fi
  else
    local minutes=$(( seconds / 60 ))
    local remaining=$(( seconds % 60 ))
    echo "${minutes}m ${remaining}s"
  fi
}

for arg in "$@"; do
  case $arg in
    batch:*)
      minutes="${arg#*:}"
      if [[ ! "$minutes" =~ ^[0-9]+$ ]]; then
        echo "Invalid batch interval: $minutes (expected integer minutes)"
        exit 1
      fi
      BATCH_INTERVAL_SECONDS=$(( minutes * 60 ))
      ;;
    interval:*)
      interval_value="${arg#*:}"
      if ! parsed_seconds=$(parse_interval_seconds "$interval_value"); then
        echo "Invalid interval format: $interval_value"
        echo "Use integers (minutes), suffix with m for minutes, or s for seconds (e.g. interval:30s, interval:2m)"
        exit 1
      fi
      BATCH_INTERVAL_SECONDS=$parsed_seconds
      ;;
    jobs:*)
      JOBS_PER_BATCH="${arg#*:}"
      ;;
    *)
      echo "Unknown argument: $arg"
      echo "Usage: $0 [interval:VALUE] [jobs:N] [batch:N]"
      exit 1
      ;;
  esac
done

if [ "$BATCH_INTERVAL_SECONDS" -le 0 ]; then
  echo "Interval must be greater than zero seconds."
  exit 1
fi

# Configuration (can still be overridden by environment variables)
API_URL="${API_URL:-https://blue-banded-bee.fly.dev}"
AUTH_TOKEN="${AUTH_TOKEN:-}"

# Colours
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Test domains - 115 diverse real-world sites (all under 50k pages)
DOMAINS=(
  # Australian businesses (6) - small-medium corporate sites
  "bankaust.com.au" "australiansuper.com" "bunnings.com.au"
  "jbhifi.com.au" "kmart.com.au" "officeworks.com.au"

  # E-commerce & retail (10) - DTC brands, smaller catalogs
  "merrypeople.com" "aesop.com" "allbirds.com" "everlane.com" "warbyparker.com"
  "casper.com" "glossier.com" "away.com" "brooklinen.com" "kotn.com"

  # Tech blogs & publications (5) - niche editorial sites
  "csswizardry.com" "heydesigner.com" "sidebar.io" "stefanjudis.com" "smolblog.com"

  # WordPress blogs & design sites (10) - typical WP installs
  "smashingmagazine.com" "css-tricks.com" "webdesignerdepot.com" "sitepoint.com" "alistapart.com"
  "designmodo.com" "creativebloq.com" "awwwards.com" "onextrapixel.com" "hongkiat.com"

  # Small business / agency sites (10) - small Aussie agencies
  "studiothink.com.au" "zeroseven.com.au" "humaan.com.au" "noice.com.au" "willandco.com.au"
  "thecontentlab.com.au" "makebold.com.au" "thisisgold.com.au" "wethecollective.com.au" "tworedshoes.com.au"

  # Developer docs & tools (8) - compact doc sites
  "fly.io" "railway.app" "render.com" "tailwindcss.com"
  "nextjs.org" "react.dev" "astro.build" "svelte.dev"

  # Additional dev frameworks & tooling (30) - focused documentation hubs
  "vitejs.dev" "nuxt.com" "remix.run" "solidjs.com" "qwik.dev"
  "parceljs.org" "rollupjs.org" "esbuild.github.io" "bun.sh" "deno.com"
  "playwright.dev" "cypress.io" "vitest.dev" "pnpm.io" "turbo.build"
  "nx.dev" "oclif.io" "temporal.io" "directus.io" "strapi.io"
  "sanity.io" "payloadcms.com" "pocketbase.io" "supabase.com" "plane.so"
  "appsmith.com" "tooljet.com" "budibase.com" "windmill.dev" "tauri.app"

  # SaaS & productivity apps (12) - lean marketing sites
  "linear.app" "height.app" "reclaim.ai" "mem.ai" "reflect.app"
  "cron.com" "retool.com" "cal.com" "around.co" "raycast.com"
  "warp.dev" "cursor.so"

  # Niche e-commerce & DTC brands (17) - smaller catalogs
  "studioneat.com" "feals.com" "magicspoon.com" "atlascoffeeclub.com"
  "blueland.com" "publicgoods.com" "outerknown.com" "grovemade.com"
  "ridgewallet.com" "ouraring.com" "carawayhome.com" "maap.cc"
  "bellroy.com" "ritual.com" "cuyana.com" "thesill.com" "parachutehome.com"

  # Indie analytics & SaaS (7) - focused product sites
  "plausible.io" "simpleanalytics.com" "savvycal.com" "commandbar.com"
  "pirsch.io" "clarityflow.com" "swapcard.com"
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
echo "Batch interval:    $(format_interval "$BATCH_INTERVAL_SECONDS")"
echo "Jobs per batch:    $JOBS_PER_BATCH"
echo "Available domains: ${#DOMAINS[@]}"
echo ""

# Calculate batches based on domain count, not time
TOTAL_DOMAINS=${#DOMAINS[@]}
TOTAL_BATCHES=$(( (TOTAL_DOMAINS + JOBS_PER_BATCH - 1) / JOBS_PER_BATCH ))  # Round up
ESTIMATED_DURATION_SECONDS=$(( (TOTAL_BATCHES - 1) * BATCH_INTERVAL_SECONDS ))

echo "Will create $TOTAL_DOMAINS jobs (one per unique domain) across $TOTAL_BATCHES batches"
echo "Estimated duration: $(format_interval "$ESTIMATED_DURATION_SECONDS")"
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
    echo "$batch_num,$domain,$job_id,$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> ./logs/load_test_jobs.log
  else
    echo -e "${RED}✗ Failed to create job for $domain (HTTP $http_code)${NC}"
    echo "$body" | jq '.' 2>/dev/null || echo "$body"

    # Exit immediately on auth errors
    if [ "$http_code" -eq 401 ]; then
      echo -e "${RED}Authentication failed. Please check your AUTH_TOKEN.${NC}"
      exit 1
    fi
  fi
}

mkdir -p ./logs
# Initialize results file
echo "batch,domain,job_id,created_at" > ./logs/load_test_jobs.log

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

  # Select next N domains from shuffled list (stop when we run out)
  selected_domains=()
  for ((i=0; i<JOBS_PER_BATCH && domain_index<${#all_shuffled[@]}; i++)); do
    selected_domains+=("${all_shuffled[$domain_index]}")
    ((domain_index++))
  done

  # Create jobs
  for domain in "${selected_domains[@]}"; do
    create_job "$domain" "$batch"
    sleep 2  # Small delay between creates
  done

  # Wait for next batch (unless this is the last one)
  if [ $batch -lt $TOTAL_BATCHES ]; then
    echo ""
    echo -e "${YELLOW}Waiting $(format_interval "$BATCH_INTERVAL_SECONDS") until next batch...${NC}"
    sleep $BATCH_INTERVAL_SECONDS
  fi
done

echo ""
echo -e "${GREEN}=== Load test complete ===${NC}"
echo "Created jobs logged to: ./logs/load_test_jobs.log"
echo ""
echo "Check job status with:"
echo "  curl -H 'Authorization: Bearer \$AUTH_TOKEN' $API_URL/v1/jobs"

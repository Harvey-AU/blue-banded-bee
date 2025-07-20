# Crawler Blocking Mitigation Plan

## Current State

Blue Banded Bee crawler experiences blocking, particularly on Shopify sites:

- **User-Agent**: `"Blue Banded Bee (Cache-warmer)"` - Obviously identifies as bot
- **Settings**: 10 concurrent connections, 3 req/sec - Too aggressive
- **Headers**: Only User-Agent set - Missing browser-like headers
- **Detection**: No 403/429 detection - Can't adapt to blocking
- **Rate Limiting**: Global only - No per-domain controls
- **Robots.txt**: Only reads for sitemap discovery - Ignores Crawl-delay and Disallow rules

## Key Insights from Real-World robots.txt

### Shopify Pattern
- **Default**: No crawl-delay (allows fast crawling)
- **Known scrapers**: 10-second delays (AhrefsBot, MJ12bot)
- **Social bots**: 1-second delay (Pinterest)
- **Extensive Disallows**: Cart, checkout, admin paths

### ABC (Australian Broadcasting Corporation) Pattern
- **Search engines**: 5-second delays (Googlebot, MSNBot, Slurp)
- **AI/ML bots**: Completely blocked (ChatGPT, Claude, GPTBot)
- **News aggregators**: 2-second delay (FlipboardProxy)
- **Compressed sitemap**: Uses .gz compression

### Implications for Blue Banded Bee
- Sites differentiate between search engines, scrapers, and AI bots
- Professional bots that identify honestly get reasonable limits
- Unknown/aggressive bots get blocked or heavily throttled
- We must handle Disallow rules to avoid re-crawling blocked paths

## Phase 1: Quick Wins (1-2 hours)

Immediate changes with minimal code modification:

- [ ] **Professional User-Agent**: `"BlueBandedBee/1.0 (+https://bluebandedbee.co/bot)"`
  - Location: `internal/crawler/config.go:31`
- [ ] **Browser-Like Headers** in Colly OnRequest:
  - `Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8`
  - `Accept-Language: en-US,en;q=0.9`
  - `Accept-Encoding: gzip, deflate, br`
  - Location: `internal/crawler/crawler.go:139`
- [ ] **Bot Information Page**: Create `/bot` page on website explaining bot, purpose, approach, safety
- [ ] **Enhanced Error Detection**: Add 403/429 to `isRetryableError`
  - Location: `internal/jobs/worker.go:1057`

**Expected Impact**: 50-70% blocking reduction, minimal performance impact

## Phase 1.5: Robots.txt Compliance

Critical for respectful crawling - many sites use this to communicate limits:

- [ ] **Parse robots.txt** at job start: (should this happen before sitemap.xml crawl?)
  - Extract Crawl-delay directive if present for other bots/crawlers
  - Build list of Disallow patterns, set tasks to skip so workers don't pickup tasks
- [ ] **Honor Crawl-delay**:
  - Convert to rate limit (e.g., Crawl-delay: 10 = 0.1 req/sec)
  - Override default settings when specified
  - Log when applying crawl-delay & update domains table (schema change required)
- [ ] **Filter Disallowed URLs**:
  - Skip URLs that match - don't attempt to crawl
  - Common patterns: `/cart`, `/checkout`, `/admin`, etc.
  - **Important**: Store disallowed patterns in memory during job
  - When `find_links` discovers URLs, check against patterns to prevent re-adding
  - Consider adding `skip_reason` field to tasks table for tracking
- [ ] **Cache robots.txt**:
  - Parse once per job, not per URL
  - Store in memory during job execution

**Expected Impact**: 90%+ success rate on sites with robots.txt rules (like Shopify)

## Phase 2: Simple Slowdown

Immediate response to blocking without database changes:

- [ ] **Blocking Detection**: Track 403/429 responses per job
- [ ] **Dynamic Concurrency**: Reduce job concurrency when blocking detected
  - From 10 → 1 then slowly increase 1 → 2 → 3 → 5 → 10 and monitor error rate, stop increaseing if blocking increases
- [ ] **Logging**: Track blocking events for analysis

**Expected Impact**: 80-90% success rate on problematic sites

## Phase 3: Advanced Features (Future)

More sophisticated solutions if needed:

### 3.1 Per-Domain Learning
- [ ] Database schema for domain configurations
- [ ] Track success/failure rates per domain
- [ ] Gradually optimise settings based on history
- [ ] Share learning across jobs for same domain

### 3.2 Advanced Detection
- [ ] Detect soft blocks (empty responses, redirects to captcha)
- [ ] Monitor response time patterns for throttling
- [ ] Implement exponential backoff with jitter

### 3.3 Advanced Robots.txt Features
- [ ] Support for Allow rules (override Disallow)
- [ ] Wildcard pattern matching (* and $)
- [ ] Sitemap discovery from robots.txt (already implemented)
- [ ] Per-User-agent rule precedence

### 3.4 Premium Features
- [ ] Custom headers per domain
- [ ] Session/cookie management for authenticated crawling
- [ ] Proxy rotation for high-volume customers

## Success Metrics

- **Completion Rate**: 95%+ (from ~70% on Shopify)
- **Performance**: <2x slower for blocked sites
- **Ethics**: Transparent bot identification
- **Monitoring**: Dashboard showing blocking rates by domain
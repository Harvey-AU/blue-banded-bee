# Sitemap Index Support - Bug Fix

## Issue Description

The current sitemap crawler fails to handle sitemap index files (sitemapindex) that contain references to multiple sub-sitemaps. This is a common pattern used by CMS platforms like Wix, WordPress, and others to organize large numbers of URLs.

## Current Behaviour

**Site:** www.cpsn.org.au  
**Problem:** The system finds the main `/sitemap.xml` but it's a sitemap index containing 9 sub-sitemaps:
- blog-categories-sitemap.xml
- blog-posts-sitemap.xml 
- event-pages-sitemap.xml
- dynamic-cpsn-staff-sitemap.xml
- dynamic-landing-sitemap.xml
- dynamic-cpsn-board-sitemap.xml
- dynamic-cpsn-elt-sitemap.xml
- dynamic-pages-collection-sitemap.xml
- pages-sitemap.xml

**Current Result:** System parses index as if it contains URLs, finds 238 "URLs" (which are actually sitemap references), fails with database constraint errors.

## Expected Behaviour

1. Detect when a sitemap is a sitemap index (`<sitemapindex>` root element)
2. Parse sub-sitemap URLs from `<sitemap><loc>` elements
3. Recursively fetch and parse each sub-sitemap for actual page URLs
4. Combine all URLs from sub-sitemaps into the job queue

## Technical Implementation

### Current Code Location
- File: `internal/crawler/sitemap.go`
- Function: `ParseSitemapURLs()` - needs to detect and handle sitemap index format

### Required Changes

1. **Detection Logic:**
   - Check if XML root element is `<sitemapindex>` vs `<urlset>`
   - Handle both sitemap index and regular sitemap formats

2. **Recursive Parsing:**
   - Extract sub-sitemap URLs from `<sitemap><loc>` elements
   - Fetch each sub-sitemap HTTP request
   - Parse each sub-sitemap for actual page URLs using existing logic
   - Aggregate all URLs before returning

3. **Error Handling:**
   - Handle failed sub-sitemap requests gracefully
   - Log which sub-sitemaps succeed/fail
   - Continue processing other sub-sitemaps if one fails

### XML Structure Examples

**Sitemap Index:**
```xml
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap>
    <loc>https://example.com/sitemap1.xml</loc>
    <lastmod>2025-05-21</lastmod>
  </sitemap>
</sitemapindex>
```

**Regular Sitemap:**
```xml
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://example.com/page1</loc>
    <lastmod>2025-05-21</lastmod>
  </url>
</urlset>
```

## Additional Benefits

This fix will support many other sites that use sitemap indexes:
- Large Wix sites
- WordPress sites with sitemap plugins
- E-commerce sites with product/category sitemaps
- Multi-language sites with per-language sitemaps

## Success Criteria

1. **www.cpsn.org.au** successfully processes all 9 sub-sitemaps
2. No database constraint errors from sitemap parsing
3. All pages from sub-sitemaps are queued for crawling
4. Backwards compatibility with regular single sitemaps maintained

## Implementation Priority

**Priority:** Medium  
**Complexity:** Low-Medium  
**Impact:** High (enables crawling of many currently failing sites)

This is a common sitemap pattern that will unlock successful crawling for a significant number of sites currently failing silently.
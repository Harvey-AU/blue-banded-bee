package api

import (
	"net/http"
)

// MetricInfo represents metadata about a metric
type MetricInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InfoHTML    string `json:"info_html"`
}

// GetMetricsMetadata returns metadata for all dashboard metrics
func GetMetricsMetadata() map[string]MetricInfo {
	return map[string]MetricInfo{
		// Job Stats
		"total_jobs": {
			Name:        "Total Jobs",
			Description: "Total number of cache warming jobs created",
			InfoHTML:    "The total number of cache warming jobs you've created. Each job represents a single crawl and cache warming operation for a domain.",
		},
		"total_tasks": {
			Name:        "Total Tasks",
			Description: "Total number of URLs processed across all jobs",
			InfoHTML:    "The total number of individual URLs that have been crawled and cache warmed across all your jobs.",
		},
		"total_pages_warmed": {
			Name:        "Pages Warmed",
			Description: "Number of pages successfully cache warmed",
			InfoHTML:    "The number of pages that have been successfully cache warmed. A page is considered warmed when it has been successfully requested and cached.",
		},
		"failed_tasks": {
			Name:        "Failed Tasks",
			Description: "Number of tasks that failed during processing",
			InfoHTML:    "The number of URLs that failed to be processed. Common reasons include 404 errors, timeouts, or server errors.",
		},

		// Cache Performance
		"cache_hit_rate": {
			Name:        "Cache Hit Rate",
			Description: "Percentage of requests served from cache",
			InfoHTML:    "The percentage of requests that were served from cache on the second request. Higher is better - it means your cache warming is effective. <a href='https://developers.cloudflare.com/cache/about/cache-hit-ratio/' target='_blank'>Learn more about cache hit rates</a>",
		},
		"cache_hits": {
			Name:        "Cache Hits",
			Description: "Number of requests served from cache",
			InfoHTML:    "The number of page requests that were served directly from the cache (HIT status) on the second request, indicating successful cache warming.",
		},
		"cache_misses": {
			Name:        "Cache Misses",
			Description: "Number of requests not served from cache",
			InfoHTML:    "The number of page requests that were not served from cache (MISS status) on the second request. This may indicate pages that cannot be cached or cache configuration issues.",
		},

		// Response Time Metrics
		"avg_response_time": {
			Name:        "Average Response Time",
			Description: "Mean response time across all requests",
			InfoHTML:    "The average time taken to load pages on the second request. This represents typical cached performance. Lower times indicate better performance.",
		},
		"p25_response_time": {
			Name:        "P25 Response Time",
			Description: "25th percentile response time",
			InfoHTML:    "The response time at which 25% of requests are faster. This represents your fastest quarter of pages.",
		},
		"median_response_time": {
			Name:        "Median Response Time",
			Description: "50th percentile response time (median)",
			InfoHTML:    "The middle value of all response times - half of your pages load faster than this, half load slower. This is often more representative than the average as it's not affected by outliers.",
		},
		"p75_response_time": {
			Name:        "P75 Response Time",
			Description: "75th percentile response time",
			InfoHTML:    "The response time at which 75% of requests are faster. This helps identify your slower pages.",
		},
		"p95_response_time": {
			Name:        "P95 Response Time",
			Description: "95th percentile response time",
			InfoHTML:    "The response time at which 95% of requests are faster. This represents your slowest 5% of pages and helps identify performance problems.",
		},
		"p99_response_time": {
			Name:        "P99 Response Time",
			Description: "99th percentile response time",
			InfoHTML:    "The response time at which 99% of requests are faster. This represents your slowest 1% of pages.",
		},

		// Response Time Distribution
		"response_time_under_1s": {
			Name:        "Under 1 Second",
			Description: "Pages loading in under 1 second",
			InfoHTML:    "The number of pages that load in under 1 second. This is considered excellent performance for cached content.",
		},
		"response_time_1_to_3s": {
			Name:        "1-3 Seconds",
			Description: "Pages loading between 1-3 seconds",
			InfoHTML:    "The number of pages that load between 1 and 3 seconds. This is considered good performance.",
		},
		"response_time_3_to_5s": {
			Name:        "3-5 Seconds",
			Description: "Pages loading between 3-5 seconds",
			InfoHTML:    "The number of pages that load between 3 and 5 seconds. This is considered acceptable but could be improved.",
		},
		"response_time_5_to_10s": {
			Name:        "5-10 Seconds",
			Description: "Pages loading between 5-10 seconds",
			InfoHTML:    "The number of pages that load between 5 and 10 seconds. This is considered slow and may impact user experience.",
		},
		"response_time_over_10s": {
			Name:        "Over 10 Seconds",
			Description: "Pages loading over 10 seconds",
			InfoHTML:    "The number of pages that take over 10 seconds to load. This is considered very slow and should be investigated.",
		},

		// Slow Pages
		"slow_pages_total": {
			Name:        "Total Slow Pages",
			Description: "Pages with response time over 3 seconds",
			InfoHTML:    "The total number of pages that take more than 3 seconds to load on the second (cached) request. These pages may benefit from optimization.",
		},
		"slow_pages_over_3s": {
			Name:        "Slow Pages (>3s)",
			Description: "Pages loading over 3 seconds",
			InfoHTML:    "Pages taking more than 3 seconds on the second request. This threshold represents the point where users may notice slowness.",
		},
		"slow_pages_over_5s": {
			Name:        "Very Slow Pages (>5s)",
			Description: "Pages loading over 5 seconds",
			InfoHTML:    "Pages taking more than 5 seconds on the second request. These pages significantly impact user experience.",
		},
		"slow_pages_over_10s": {
			Name:        "Extremely Slow Pages (>10s)",
			Description: "Pages loading over 10 seconds",
			InfoHTML:    "Pages taking more than 10 seconds on the second request. These pages require immediate attention.",
		},

		// Task Details
		"task_path": {
			Name:        "Path",
			Description: "The URL path of the page",
			InfoHTML:    "The URL path that was crawled and cache warmed. Click to open in a new tab.",
		},
		"task_status": {
			Name:        "Status",
			Description: "Current status of the task",
			InfoHTML:    "The current processing status: <strong>completed</strong> (successfully processed), <strong>pending</strong> (queued or in progress), or <strong>failed</strong> (error occurred).",
		},
		"task_response_time": {
			Name:        "Response Time",
			Description: "Time taken to load the page",
			InfoHTML:    "The response time from the second request (when available), which represents cached performance. This is the time users will typically experience.",
		},
		"task_cache_status": {
			Name:        "Cache Status",
			Description: "Whether the page was served from cache",
			InfoHTML:    "The cache status from the first request: <strong>HIT</strong> (served from cache), <strong>MISS</strong> (not in cache), or other Cloudflare cache statuses.",
		},
		"task_second_request": {
			Name:        "2nd Request",
			Description: "Response time of the second request",
			InfoHTML:    "The response time from the second request to the same URL. This shows how fast the page loads when served from cache after warming.",
		},
		"task_status_code": {
			Name:        "Status Code",
			Description: "HTTP status code returned",
			InfoHTML:    "The HTTP status code returned by the server (200 = success, 404 = not found, 500 = server error, etc.).",
		},

		// Source Types
		"from_sitemap": {
			Name:        "From Sitemap",
			Description: "URLs discovered from sitemap",
			InfoHTML:    "The number of URLs that were discovered by crawling the site's XML sitemap. Sitemaps are the most reliable source for finding all pages.",
		},
		"from_links": {
			Name:        "From Links",
			Description: "URLs discovered from page links",
			InfoHTML:    "The number of URLs that were discovered by following links on pages. This helps find pages not listed in the sitemap.",
		},
	}
}

// MetadataHandler handles GET /v1/metadata/metrics
func (h *Handler) MetadataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, r)
		return
	}

	metadata := GetMetricsMetadata()

	WriteSuccess(w, r, metadata, "Metrics metadata retrieved successfully")
}

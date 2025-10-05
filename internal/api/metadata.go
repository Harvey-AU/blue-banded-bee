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
		"job_id": {
			Name:        "Job ID",
			Description: "Unique identifier for this job",
			InfoHTML:    "A unique identifier for this cache warming job. Use this ID when referencing the job in support requests.",
		},
		"job_domain": {
			Name:        "Domain",
			Description: "The domain being cache warmed",
			InfoHTML:    "The website domain that this job is crawling and cache warming.",
		},
		"job_status": {
			Name:        "Status",
			Description: "Current status of the job",
			InfoHTML:    "The current state of this job: <strong>pending</strong> (not started), <strong>running</strong> (in progress), <strong>completed</strong> (finished successfully), <strong>failed</strong> (encountered errors), or <strong>cancelled</strong> (stopped by user).",
		},
		"job_progress": {
			Name:        "Progress",
			Description: "Percentage of tasks completed",
			InfoHTML:    "The percentage of URLs that have been processed. This shows how far through the job we are.",
		},
		"total_tasks": {
			Name:        "Total Tasks",
			Description: "Total number of URLs processed across all jobs",
			InfoHTML:    "The total number of individual URLs that have been crawled and cache warmed across all your jobs.",
		},
		"completed_tasks": {
			Name:        "Completed",
			Description: "Number of tasks successfully completed",
			InfoHTML:    "The number of URLs that have been successfully crawled and cache warmed.",
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
		"start_time": {
			Name:        "Start Time",
			Description: "When the job started processing",
			InfoHTML:    "The date and time when this job began crawling and warming URLs.",
		},
		"end_time": {
			Name:        "End Time",
			Description: "When the job finished processing",
			InfoHTML:    "The date and time when this job completed all tasks.",
		},
		"total_time": {
			Name:        "Total Time",
			Description: "Total duration of the job",
			InfoHTML:    "The total time taken to complete this job from start to finish.",
		},
		"avg_time_per_task": {
			Name:        "Average Time Per Task",
			Description: "Average time spent on each URL",
			InfoHTML:    "The average time it took to process each URL, including crawling and cache warming.",
		},
		"time_saved": {
			Name:        "Time Saved",
			Description: "Total time saved through cache warming",
			InfoHTML:    "The total time saved across all pages by serving them from cache instead of generating them fresh. This is calculated as the difference between first request time and second (cached) request time.",
		},
		"total_broken_links": {
			Name:        "Broken Links",
			Description: "Total number of broken links found",
			InfoHTML:    "The number of URLs that returned 4xx client errors (like 404 Not Found). These pages may be missing or have incorrect links.",
		},
		"total_404s": {
			Name:        "404 Errors",
			Description: "Number of pages not found",
			InfoHTML:    "The number of URLs that returned a 404 Not Found error. These pages don't exist and may need to be removed or redirected.",
		},
		"slow_pages_over_3s": {
			Name:        "Slow Pages (>3s)",
			Description: "Pages loading over 3 seconds after cache warming",
			InfoHTML:    "The number of pages that take more than 3 seconds to load even after being cache warmed. These pages may benefit from optimisation.",
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
		"avg_saved_per_page": {
			Name:        "Avg Saved/Page",
			Description: "Average time saved per page through caching",
			InfoHTML:    "The average amount of time saved per page by serving from cache instead of generating fresh. This is calculated as the difference between first and second request times.",
		},
		"improvement_rate": {
			Name:        "Improvement Rate",
			Description: "Percentage of pages that loaded faster on second request",
			InfoHTML:    "The percentage of pages that loaded faster on the second (cached) request compared to the first request. Higher percentages indicate more effective cache warming.",
		},
		"hit_rate": {
			Name:        "Hit Rate",
			Description: "Cache hit rate percentage",
			InfoHTML:    "The percentage of requests that were served from cache. Higher rates mean better cache effectiveness.",
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
		"p25": {
			Name:        "P25",
			Description: "25th percentile response time",
			InfoHTML:    "The response time at which 25% of requests are faster after cache warming. This represents your fastest quarter of pages.",
		},
		"median_response_time": {
			Name:        "Median Response Time",
			Description: "50th percentile response time (median)",
			InfoHTML:    "The middle value of all response times - half of your pages load faster than this, half load slower. This is often more representative than the average as it's not affected by outliers.",
		},
		"median": {
			Name:        "Median",
			Description: "50th percentile response time (median)",
			InfoHTML:    "The middle value of all response times after cache warming - half of your pages load faster than this, half load slower. This is often more representative than the average.",
		},
		"p75_response_time": {
			Name:        "P75 Response Time",
			Description: "75th percentile response time",
			InfoHTML:    "The response time at which 75% of requests are faster. This helps identify your slower pages.",
		},
		"p75": {
			Name:        "P75",
			Description: "75th percentile response time",
			InfoHTML:    "The response time at which 75% of requests are faster after cache warming. This helps identify your slower quarter of pages.",
		},
		"p90": {
			Name:        "P90",
			Description: "90th percentile response time",
			InfoHTML:    "The response time at which 90% of requests are faster after cache warming. This represents your slowest 10% of pages.",
		},
		"p95_response_time": {
			Name:        "P95 Response Time",
			Description: "95th percentile response time",
			InfoHTML:    "The response time at which 95% of requests are faster. This represents your slowest 5% of pages and helps identify performance problems.",
		},
		"p95": {
			Name:        "P95",
			Description: "95th percentile response time",
			InfoHTML:    "The response time at which 95% of requests are faster after cache warming. This represents your slowest 5% of pages and helps identify performance problems.",
		},
		"p99_response_time": {
			Name:        "P99 Response Time",
			Description: "99th percentile response time",
			InfoHTML:    "The response time at which 99% of requests are faster. This represents your slowest 1% of pages.",
		},
		"p99": {
			Name:        "P99",
			Description: "99th percentile response time",
			InfoHTML:    "The response time at which 99% of requests are faster after cache warming. This represents your slowest 1% of pages.",
		},

		// Response Time Distribution
		"response_time_under_500ms": {
			Name:        "Under 500ms",
			Description: "Pages loading in under 500 milliseconds",
			InfoHTML:    "The number of pages that load in under 500ms after cache warming. This is excellent performance.",
		},
		"response_time_500ms_to_1s": {
			Name:        "500ms-1s",
			Description: "Pages loading between 500ms and 1 second",
			InfoHTML:    "The number of pages that load between 500ms and 1 second after cache warming. This is very good performance.",
		},
		"response_time_1_to_1_5s": {
			Name:        "1-1.5s",
			Description: "Pages loading between 1 and 1.5 seconds",
			InfoHTML:    "The number of pages that load between 1 and 1.5 seconds after cache warming. This is good performance.",
		},
		"response_time_1_5_to_2s": {
			Name:        "1.5-2s",
			Description: "Pages loading between 1.5 and 2 seconds",
			InfoHTML:    "The number of pages that load between 1.5 and 2 seconds after cache warming. This is acceptable performance.",
		},
		"response_time_2_to_3s": {
			Name:        "2-3s",
			Description: "Pages loading between 2 and 3 seconds",
			InfoHTML:    "The number of pages that load between 2 and 3 seconds after cache warming. Performance could be improved.",
		},
		"response_time_3_to_5s": {
			Name:        "3-5s",
			Description: "Pages loading between 3 and 5 seconds",
			InfoHTML:    "The number of pages that load between 3 and 5 seconds after cache warming. These pages are considered slow.",
		},
		"response_time_5_to_10s": {
			Name:        "5-10s",
			Description: "Pages loading between 5 and 10 seconds",
			InfoHTML:    "The number of pages that load between 5 and 10 seconds after cache warming. These pages are very slow and should be optimised.",
		},
		"response_time_over_10s": {
			Name:        "Over 10s",
			Description: "Pages loading over 10 seconds",
			InfoHTML:    "The number of pages that take over 10 seconds to load after cache warming. These pages require immediate attention.",
		},
		"total_slow_over_3s": {
			Name:        "Total Slow (>3s)",
			Description: "Total pages loading over 3 seconds",
			InfoHTML:    "The total number of pages that take more than 3 seconds to load after cache warming. This is the sum of all slow pages.",
		},
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

		// Slow Pages
		"slow_pages_total": {
			Name:        "Total Slow Pages",
			Description: "Pages with response time over 3 seconds",
			InfoHTML:    "The total number of pages that take more than 3 seconds to load on the second (cached) request. These pages may benefit from optimization.",
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

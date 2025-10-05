-- Update job stats calculation to use second_response_time (cache HIT) instead of response_time (cache MISS)
-- This provides more accurate metrics for cache performance

CREATE OR REPLACE FUNCTION calculate_job_stats()
RETURNS TRIGGER AS $$
DECLARE
    v_stats JSONB;
BEGIN
    -- Only calculate stats when job transitions to completed status
    IF NEW.status = 'completed' AND OLD.status != 'completed' THEN

        -- Calculate comprehensive statistics from tasks
        WITH task_stats AS (
            SELECT
                -- Response status breakdowns
                COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500) AS total_broken_links,
                COUNT(*) FILTER (WHERE status_code = 404) AS total_404s,
                COUNT(*) FILTER (WHERE status_code >= 500) AS total_server_errors,

                -- Detailed slow page buckets (using second_response_time with fallback)
                COUNT(*) FILTER (WHERE COALESCE(second_response_time, response_time) > 10000) AS pages_over_10s,
                COUNT(*) FILTER (WHERE COALESCE(second_response_time, response_time) >= 5000 AND COALESCE(second_response_time, response_time) < 10000) AS pages_5_to_10s,
                COUNT(*) FILTER (WHERE COALESCE(second_response_time, response_time) >= 3000 AND COALESCE(second_response_time, response_time) < 5000) AS pages_3_to_5s,
                COUNT(*) FILTER (WHERE COALESCE(second_response_time, response_time) >= 2000 AND COALESCE(second_response_time, response_time) < 3000) AS pages_2_to_3s,
                COUNT(*) FILTER (WHERE COALESCE(second_response_time, response_time) >= 1500 AND COALESCE(second_response_time, response_time) < 2000) AS pages_1_5_to_2s,
                COUNT(*) FILTER (WHERE COALESCE(second_response_time, response_time) >= 1000 AND COALESCE(second_response_time, response_time) < 1500) AS pages_1_to_1_5s,
                COUNT(*) FILTER (WHERE COALESCE(second_response_time, response_time) >= 500 AND COALESCE(second_response_time, response_time) < 1000) AS pages_500ms_to_1s,
                COUNT(*) FILTER (WHERE COALESCE(second_response_time, response_time) < 500) AS pages_under_500ms,

                -- Cache metrics
                COUNT(*) FILTER (WHERE cache_status = 'HIT') AS total_cache_hits,
                COUNT(*) FILTER (WHERE cache_status = 'MISS') AS total_cache_misses,
                COUNT(*) FILTER (WHERE cache_status = 'BYPASS') AS total_cache_bypass,

                -- Redirect tracking
                COUNT(*) FILTER (WHERE status_code >= 300 AND status_code < 400) AS total_redirects,
                COUNT(*) FILTER (WHERE status_code = 301) AS total_301_redirects,
                COUNT(*) FILTER (WHERE status_code = 302) AS total_302_redirects,

                -- Response time statistics with second_response_time (cache HIT) - use COALESCE for fallback
                AVG(COALESCE(second_response_time, response_time)) FILTER (WHERE COALESCE(second_response_time, response_time) IS NOT NULL) AS avg_response_time_ms,
                MIN(COALESCE(second_response_time, response_time)) FILTER (WHERE COALESCE(second_response_time, response_time) IS NOT NULL) AS min_response_time_ms,
                MAX(COALESCE(second_response_time, response_time)) FILTER (WHERE COALESCE(second_response_time, response_time) IS NOT NULL) AS max_response_time_ms,
                PERCENTILE_CONT(0.25) WITHIN GROUP (ORDER BY COALESCE(second_response_time, response_time)) FILTER (WHERE COALESCE(second_response_time, response_time) IS NOT NULL) AS p25_response_time_ms,
                PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY COALESCE(second_response_time, response_time)) FILTER (WHERE COALESCE(second_response_time, response_time) IS NOT NULL) AS median_response_time_ms,
                PERCENTILE_CONT(0.75) WITHIN GROUP (ORDER BY COALESCE(second_response_time, response_time)) FILTER (WHERE COALESCE(second_response_time, response_time) IS NOT NULL) AS p75_response_time_ms,
                PERCENTILE_CONT(0.90) WITHIN GROUP (ORDER BY COALESCE(second_response_time, response_time)) FILTER (WHERE COALESCE(second_response_time, response_time) IS NOT NULL) AS p90_response_time_ms,
                PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY COALESCE(second_response_time, response_time)) FILTER (WHERE COALESCE(second_response_time, response_time) IS NOT NULL) AS p95_response_time_ms,
                PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY COALESCE(second_response_time, response_time)) FILTER (WHERE COALESCE(second_response_time, response_time) IS NOT NULL) AS p99_response_time_ms,

                -- Second request performance (for cache warming validation)
                AVG(second_response_time) FILTER (WHERE second_response_time IS NOT NULL) AS avg_second_response_time_ms,
                COUNT(*) FILTER (WHERE second_response_time IS NOT NULL) AS total_second_requests,
                COUNT(*) FILTER (WHERE second_response_time < response_time) AS total_improved_on_second,

                -- Total cache time savings calculation
                SUM(GREATEST(0, response_time - second_response_time)) FILTER (WHERE second_response_time IS NOT NULL) AS total_time_saved_ms,
                AVG(GREATEST(0, response_time - second_response_time)) FILTER (WHERE second_response_time IS NOT NULL) AS avg_time_saved_per_page_ms,

                -- Task completion metrics
                COUNT(*) AS total_tasks_processed,
                COUNT(*) FILTER (WHERE status = 'completed') AS total_completed,
                COUNT(*) FILTER (WHERE status = 'failed') AS total_failed,
                COUNT(*) FILTER (WHERE error IS NOT NULL) AS total_with_errors,

                -- URL discovery metrics
                COUNT(DISTINCT source_url) AS unique_source_urls,
                COUNT(*) FILTER (WHERE source_type = 'sitemap') AS from_sitemap,
                COUNT(*) FILTER (WHERE source_type = 'discovered') AS from_discovery,
                COUNT(*) FILTER (WHERE source_type = 'manual') AS from_manual

            FROM tasks
            WHERE job_id = NEW.id
        ),
        response_time_buckets AS (
            -- Response time distribution for histogram (using second_response_time with fallback)
            SELECT
                jsonb_build_object(
                    'under_100ms', COUNT(*) FILTER (WHERE COALESCE(second_response_time, response_time) < 100),
                    '100_500ms', COUNT(*) FILTER (WHERE COALESCE(second_response_time, response_time) >= 100 AND COALESCE(second_response_time, response_time) < 500),
                    '500_1000ms', COUNT(*) FILTER (WHERE COALESCE(second_response_time, response_time) >= 500 AND COALESCE(second_response_time, response_time) < 1000),
                    '1_3s', COUNT(*) FILTER (WHERE COALESCE(second_response_time, response_time) >= 1000 AND COALESCE(second_response_time, response_time) < 3000),
                    'over_3s', COUNT(*) FILTER (WHERE COALESCE(second_response_time, response_time) >= 3000)
                ) as buckets
            FROM tasks
            WHERE job_id = NEW.id AND COALESCE(second_response_time, response_time) IS NOT NULL
        ),
        status_code_distribution AS (
            -- Status code distribution
            SELECT
                jsonb_object_agg(
                    status_code::text,
                    count
                ) as distribution
            FROM (
                SELECT status_code, COUNT(*) as count
                FROM tasks
                WHERE job_id = NEW.id AND status_code IS NOT NULL
                GROUP BY status_code
            ) sc
        )
        SELECT jsonb_build_object(
            -- Basic counts
            'total_broken_links', COALESCE(total_broken_links, 0),
            'total_404s', COALESCE(total_404s, 0),
            'total_server_errors', COALESCE(total_server_errors, 0),

            -- Detailed slow page breakdown
            'slow_page_buckets', jsonb_build_object(
                'over_10s', COALESCE(pages_over_10s, 0),
                '5_to_10s', COALESCE(pages_5_to_10s, 0),
                '3_to_5s', COALESCE(pages_3_to_5s, 0),
                '2_to_3s', COALESCE(pages_2_to_3s, 0),
                '1_5_to_2s', COALESCE(pages_1_5_to_2s, 0),
                '1_to_1_5s', COALESCE(pages_1_to_1_5s, 0),
                '500ms_to_1s', COALESCE(pages_500ms_to_1s, 0),
                'under_500ms', COALESCE(pages_under_500ms, 0),
                'total_slow_over_3s', COALESCE(pages_over_10s, 0) + COALESCE(pages_5_to_10s, 0) + COALESCE(pages_3_to_5s, 0)
            ),

            -- Cache metrics
            'cache_stats', jsonb_build_object(
                'hits', COALESCE(total_cache_hits, 0),
                'misses', COALESCE(total_cache_misses, 0),
                'bypass', COALESCE(total_cache_bypass, 0),
                'hit_rate', CASE
                    WHEN COALESCE(total_cache_hits, 0) + COALESCE(total_cache_misses, 0) > 0
                    THEN ROUND((COALESCE(total_cache_hits, 0)::numeric / NULLIF(COALESCE(total_cache_hits, 0) + COALESCE(total_cache_misses, 0), 0)::numeric * 100), 2)
                    ELSE 0
                END
            ),

            -- Redirect metrics
            'redirect_stats', jsonb_build_object(
                'total', COALESCE(total_redirects, 0),
                '301_permanent', COALESCE(total_301_redirects, 0),
                '302_temporary', COALESCE(total_302_redirects, 0)
            ),

            -- Performance metrics with full percentile breakdown (now using second_response_time)
            'response_times', jsonb_build_object(
                'avg_ms', ROUND(COALESCE(avg_response_time_ms, 0)::numeric, 2),
                'min_ms', COALESCE(min_response_time_ms, 0),
                'max_ms', COALESCE(max_response_time_ms, 0),
                'p25_ms', ROUND(COALESCE(p25_response_time_ms, 0)::numeric, 2),
                'median_ms', ROUND(COALESCE(median_response_time_ms, 0)::numeric, 2),
                'p75_ms', ROUND(COALESCE(p75_response_time_ms, 0)::numeric, 2),
                'p90_ms', ROUND(COALESCE(p90_response_time_ms, 0)::numeric, 2),
                'p95_ms', ROUND(COALESCE(p95_response_time_ms, 0)::numeric, 2),
                'p99_ms', ROUND(COALESCE(p99_response_time_ms, 0)::numeric, 2)
            ),

            -- Cache warming effectiveness and time savings
            'cache_warming_effect', jsonb_build_object(
                'avg_second_request_ms', ROUND(COALESCE(avg_second_response_time_ms, 0)::numeric, 2),
                'total_validated', COALESCE(total_second_requests, 0),
                'total_improved', COALESCE(total_improved_on_second, 0),
                'total_time_saved_ms', ROUND(COALESCE(total_time_saved_ms, 0)::numeric, 2),
                'total_time_saved_seconds', ROUND(COALESCE(total_time_saved_ms, 0)::numeric / 1000, 2),
                'avg_time_saved_per_page_ms', ROUND(COALESCE(avg_time_saved_per_page_ms, 0)::numeric, 2),
                'improvement_rate', CASE
                    WHEN COALESCE(total_second_requests, 0) > 0
                    THEN ROUND((COALESCE(total_improved_on_second, 0)::numeric / total_second_requests::numeric * 100), 2)
                    ELSE 0
                END
            ),

            -- Task breakdown
            'task_summary', jsonb_build_object(
                'processed', COALESCE(total_tasks_processed, 0),
                'completed', COALESCE(total_completed, 0),
                'failed', COALESCE(total_failed, 0),
                'with_errors', COALESCE(total_with_errors, 0)
            ),

            -- Discovery source breakdown
            'discovery_sources', jsonb_build_object(
                'sitemap', COALESCE(from_sitemap, 0),
                'discovered', COALESCE(from_discovery, 0),
                'manual', COALESCE(from_manual, 0),
                'unique_sources', COALESCE(unique_source_urls, 0)
            ),

            -- Response time distribution (now using second_response_time)
            'response_time_distribution', COALESCE(rtb.buckets, '{}'::jsonb),

            -- Status code distribution
            'status_code_distribution', COALESCE(scd.distribution, '{}'::jsonb),

            -- Metadata
            'calculated_at', NOW() AT TIME ZONE 'UTC',
            'calculation_version', '2.0'
        ) INTO v_stats
        FROM task_stats
        CROSS JOIN response_time_buckets rtb
        CROSS JOIN status_code_distribution scd;

        -- Update the job with calculated stats
        NEW.stats := v_stats;

    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Update calculation version to v2.0 to track this change
COMMENT ON FUNCTION calculate_job_stats() IS 'Calculate comprehensive job statistics using second_response_time (cache HIT) for accurate performance metrics. Updated to v2.0 to use COALESCE(second_response_time, response_time) for all calculations.';

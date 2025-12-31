-- Fix: Use realtime.send() for private broadcast support
-- The previous migration used broadcast_changes() which only supports public channels
-- This corrects it to use realtime.send() with private=TRUE to match the frontend subscription

CREATE OR REPLACE FUNCTION notify_job_status_change()
RETURNS TRIGGER
SECURITY DEFINER
LANGUAGE plpgsql
AS $$
DECLARE
    job_domain_name TEXT;
    duration_secs INTEGER;
    duration_text TEXT;
    notification_id UUID;
    stats JSONB;
    msg_lines TEXT[];
    sitemap_count INTEGER;
    discovered_count INTEGER;
    cache_hit_rate NUMERIC;
    cache_miss_rate NUMERIC;
    cache_bypass_pct NUMERIC;
    total_cacheable INTEGER;
    avg_response_ms NUMERIC;
    time_saved_secs NUMERIC;
    pages_improved INTEGER;
    slow_pages INTEGER;
    broken_links INTEGER;
    notification_record RECORD;
BEGIN
    IF OLD.status = NEW.status THEN
        RETURN NEW;
    END IF;

    IF NEW.organisation_id IS NULL THEN
        RETURN NEW;
    END IF;

    SELECT d.name INTO job_domain_name
    FROM domains d
    WHERE d.id = NEW.domain_id;

    duration_secs := EXTRACT(EPOCH FROM (COALESCE(NEW.completed_at, NOW()) - NEW.started_at))::INTEGER;

    IF duration_secs < 60 THEN
        duration_text := duration_secs || 's';
    ELSIF duration_secs < 3600 THEN
        duration_text := (duration_secs / 60) || 'm ' || (duration_secs % 60) || 's';
    ELSE
        duration_text := (duration_secs / 3600) || 'h ' || ((duration_secs % 3600) / 60) || 'm';
    END IF;

    stats := COALESCE(NEW.stats, '{}'::jsonb);

    -- Job completed
    IF OLD.status != 'completed' AND NEW.status = 'completed' THEN
        notification_id := gen_random_uuid();

        sitemap_count := COALESCE((stats->'discovery_sources'->>'sitemap')::INTEGER, 0);
        discovered_count := COALESCE((stats->'discovery_sources'->>'discovered')::INTEGER, 0);
        avg_response_ms := COALESCE((stats->'response_times'->>'avg_ms')::NUMERIC, 0);
        cache_hit_rate := COALESCE((stats->'cache_stats'->>'hit_rate')::NUMERIC, 0);
        time_saved_secs := COALESCE((stats->'cache_warming_effect'->>'total_time_saved_seconds')::NUMERIC, 0);
        pages_improved := COALESCE((stats->'cache_warming_effect'->>'total_improved')::INTEGER, 0);
        broken_links := COALESCE((stats->>'total_broken_links')::INTEGER, 0);

        slow_pages := COALESCE((stats->'slow_page_buckets'->>'over_10s')::INTEGER, 0) +
                      COALESCE((stats->'slow_page_buckets'->>'5_to_10s')::INTEGER, 0) +
                      COALESCE((stats->'slow_page_buckets'->>'3_to_5s')::INTEGER, 0) +
                      COALESCE((stats->'slow_page_buckets'->>'2_to_3s')::INTEGER, 0);

        total_cacheable := COALESCE((stats->'cache_stats'->>'hits')::INTEGER, 0) +
                          COALESCE((stats->'cache_stats'->>'misses')::INTEGER, 0);
        IF total_cacheable > 0 THEN
            cache_miss_rate := 100 - cache_hit_rate;
        ELSE
            cache_miss_rate := 0;
        END IF;
        cache_bypass_pct := CASE
            WHEN NEW.completed_tasks > 0 THEN
                ROUND((COALESCE((stats->'cache_stats'->>'bypass')::INTEGER, 0)::NUMERIC / NEW.completed_tasks * 100), 0)
            ELSE 0
        END;

        msg_lines := ARRAY[
            format('Total pages: %s (sitemap: %s, discovered: %s)%s',
                NEW.completed_tasks, sitemap_count, discovered_count,
                CASE WHEN NEW.max_pages > 0 THEN format(' (max: %s)', NEW.max_pages) ELSE '' END),
            '',
            format('Average page speed: %sms', ROUND(avg_response_ms)),
            format('Cached: %s%%, miss→hit: %s%% (time saved: %ss), uncacheable: %s%%',
                ROUND(cache_hit_rate), ROUND(cache_miss_rate), ROUND(time_saved_secs, 1), cache_bypass_pct),
            '',
            format('Pages improved: %s, >2s pages: %s, Broken links: %s', pages_improved, slow_pages, broken_links)
        ];

        INSERT INTO notifications (id, organisation_id, user_id, type, subject, preview, message, link, data, created_at)
        VALUES (
            notification_id, NEW.organisation_id, NEW.user_id, 'job_complete',
            COALESCE(job_domain_name, 'Unknown') || ' completed',
            format('%s URLs processed in %s', NEW.completed_tasks, duration_text),
            array_to_string(msg_lines, E'\n'),
            '/jobs/' || NEW.id,
            jsonb_build_object('job_id', NEW.id, 'domain', COALESCE(job_domain_name, 'Unknown'),
                'completed_tasks', NEW.completed_tasks, 'failed_tasks', NEW.failed_tasks, 'duration', duration_text),
            NOW()
        ) RETURNING * INTO notification_record;

        -- Broadcast via private channel using realtime.send()
        PERFORM realtime.send(
            jsonb_build_object(
                'type', 'notification',
                'record', to_jsonb(notification_record)
            ),
            'notification',
            'notifications:' || NEW.organisation_id::text,
            TRUE
        );
    END IF;

    -- Job failed
    IF OLD.status NOT IN ('failed', 'cancelled') AND NEW.status = 'failed' THEN
        notification_id := gen_random_uuid();

        INSERT INTO notifications (id, organisation_id, user_id, type, subject, preview, link, data, created_at)
        VALUES (
            notification_id, NEW.organisation_id, NEW.user_id, 'job_failed',
            COALESCE(job_domain_name, 'Unknown') || ' failed',
            format('%s URLs completed, %s failed', NEW.completed_tasks, NEW.failed_tasks),
            '/jobs/' || NEW.id,
            jsonb_build_object('job_id', NEW.id, 'domain', COALESCE(job_domain_name, 'Unknown'),
                'completed_tasks', NEW.completed_tasks, 'failed_tasks', NEW.failed_tasks,
                'error_message', 'Job processing failed'),
            NOW()
        ) RETURNING * INTO notification_record;

        -- Broadcast via private channel using realtime.send()
        PERFORM realtime.send(
            jsonb_build_object(
                'type', 'notification',
                'record', to_jsonb(notification_record)
            ),
            'notification',
            'notifications:' || NEW.organisation_id::text,
            TRUE
        );
    END IF;

    RETURN NEW;
END;
$$;

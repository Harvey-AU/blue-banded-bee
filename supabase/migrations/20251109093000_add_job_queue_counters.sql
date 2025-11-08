-- Add per-job pending/waiting task counters
ALTER TABLE jobs
    ADD COLUMN pending_tasks INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN waiting_tasks INTEGER NOT NULL DEFAULT 0;

-- Backfill counters from existing task data
WITH task_summary AS (
    SELECT
        job_id,
        COUNT(*) FILTER (WHERE status = 'pending') AS pending,
        COUNT(*) FILTER (WHERE status = 'waiting') AS waiting
    FROM tasks
    GROUP BY job_id
)
UPDATE jobs
SET pending_tasks = COALESCE(ts.pending, 0),
    waiting_tasks = COALESCE(ts.waiting, 0)
FROM task_summary ts
WHERE jobs.id = ts.job_id;

-- Function to keep pending/waiting counters in sync
CREATE OR REPLACE FUNCTION update_job_queue_counters()
RETURNS TRIGGER AS $$
DECLARE
    pending_delta INTEGER := 0;
    waiting_delta INTEGER := 0;
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.status = 'pending' THEN
            pending_delta := 1;
        ELSIF NEW.status = 'waiting' THEN
            waiting_delta := 1;
        END IF;

        IF pending_delta <> 0 OR waiting_delta <> 0 THEN
            UPDATE jobs
            SET pending_tasks = pending_tasks + pending_delta,
                waiting_tasks = waiting_tasks + waiting_delta
            WHERE id = NEW.job_id;
        END IF;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        IF OLD.status = 'pending' THEN
            pending_delta := -1;
        ELSIF OLD.status = 'waiting' THEN
            waiting_delta := -1;
        END IF;

        IF pending_delta <> 0 OR waiting_delta <> 0 THEN
            UPDATE jobs
            SET pending_tasks = GREATEST(0, pending_tasks + pending_delta),
                waiting_tasks = GREATEST(0, waiting_tasks + waiting_delta)
            WHERE id = OLD.job_id;
        END IF;
        RETURN OLD;
    ELSE
        IF NEW.job_id = OLD.job_id THEN
            IF NEW.status <> OLD.status THEN
                IF NEW.status = 'pending' THEN
                    pending_delta := pending_delta + 1;
                ELSIF NEW.status = 'waiting' THEN
                    waiting_delta := waiting_delta + 1;
                END IF;

                IF OLD.status = 'pending' THEN
                    pending_delta := pending_delta - 1;
                ELSIF OLD.status = 'waiting' THEN
                    waiting_delta := waiting_delta - 1;
                END IF;

                IF pending_delta <> 0 OR waiting_delta <> 0 THEN
                    UPDATE jobs
                    SET pending_tasks = GREATEST(0, pending_tasks + pending_delta),
                        waiting_tasks = GREATEST(0, waiting_tasks + waiting_delta)
                    WHERE id = NEW.job_id;
                END IF;
            END IF;
        ELSE
            -- Job reassignment: remove from old job, add to new job
            IF OLD.status = 'pending' THEN
                UPDATE jobs
                SET pending_tasks = GREATEST(0, pending_tasks - 1)
                WHERE id = OLD.job_id;
            ELSIF OLD.status = 'waiting' THEN
                UPDATE jobs
                SET waiting_tasks = GREATEST(0, waiting_tasks - 1)
                WHERE id = OLD.job_id;
            END IF;

            IF NEW.status = 'pending' THEN
                UPDATE jobs
                SET pending_tasks = pending_tasks + 1
                WHERE id = NEW.job_id;
            ELSIF NEW.status = 'waiting' THEN
                UPDATE jobs
                SET waiting_tasks = waiting_tasks + 1
                WHERE id = NEW.job_id;
            END IF;
        END IF;

        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_update_job_queue_counters ON tasks;
CREATE TRIGGER trg_update_job_queue_counters
AFTER INSERT OR UPDATE OR DELETE ON tasks
FOR EACH ROW
EXECUTE FUNCTION update_job_queue_counters();

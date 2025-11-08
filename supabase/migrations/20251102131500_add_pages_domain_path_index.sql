-- Optimise page lookups by domain/path composite index
-- Supports hot query: SELECT id FROM pages WHERE domain_id = $1 AND path = $2
-- Also accelerates path = ANY(...) checks during task enqueue

CREATE INDEX IF NOT EXISTS idx_pages_domain_path
  ON pages (domain_id, path);

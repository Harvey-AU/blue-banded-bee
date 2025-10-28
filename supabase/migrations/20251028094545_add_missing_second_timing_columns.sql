-- Add missing second_* timing columns to tasks table
-- These columns were in the initial schema but were lost during a database reset
-- when migrations were cleared and re-run. They track cache warming retry metrics.

-- Add all second_* timing columns with IF NOT EXISTS to be idempotent
ALTER TABLE tasks
ADD COLUMN IF NOT EXISTS second_response_time BIGINT,
ADD COLUMN IF NOT EXISTS second_cache_status TEXT,
ADD COLUMN IF NOT EXISTS second_content_length BIGINT,
ADD COLUMN IF NOT EXISTS second_headers JSONB,
ADD COLUMN IF NOT EXISTS second_dns_lookup_time INTEGER,
ADD COLUMN IF NOT EXISTS second_tcp_connection_time INTEGER,
ADD COLUMN IF NOT EXISTS second_tls_handshake_time INTEGER,
ADD COLUMN IF NOT EXISTS second_ttfb INTEGER,
ADD COLUMN IF NOT EXISTS second_content_transfer_time INTEGER;

-- Add comments explaining these columns
COMMENT ON COLUMN tasks.second_response_time IS 'Total response time in milliseconds for cache retry/validation request';
COMMENT ON COLUMN tasks.second_cache_status IS 'Cache status (HIT/MISS) from the second request to validate cache warming';
COMMENT ON COLUMN tasks.second_content_length IS 'Content length in bytes from second request';
COMMENT ON COLUMN tasks.second_headers IS 'Response headers from second request as JSONB';
COMMENT ON COLUMN tasks.second_dns_lookup_time IS 'DNS lookup time in milliseconds for second request';
COMMENT ON COLUMN tasks.second_tcp_connection_time IS 'TCP connection time in milliseconds for second request';
COMMENT ON COLUMN tasks.second_tls_handshake_time IS 'TLS handshake time in milliseconds for second request';
COMMENT ON COLUMN tasks.second_ttfb IS 'Time to first byte in milliseconds for second request';
COMMENT ON COLUMN tasks.second_content_transfer_time IS 'Content transfer time in milliseconds for second request';

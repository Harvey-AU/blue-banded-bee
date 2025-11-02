-- Add adaptive delay tracking columns for per-domain throttling
ALTER TABLE domains
  ADD COLUMN IF NOT EXISTS adaptive_delay_seconds INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS adaptive_delay_floor_seconds INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN domains.adaptive_delay_seconds IS 'Learned baseline delay between requests (seconds) based on prior throttling';
COMMENT ON COLUMN domains.adaptive_delay_floor_seconds IS 'Minimum safe delay established after probing (seconds)';

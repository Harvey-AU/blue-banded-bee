-- Add inactive_reason for GA connection audit trail
ALTER TABLE google_analytics_connections
    ADD COLUMN inactive_reason TEXT;

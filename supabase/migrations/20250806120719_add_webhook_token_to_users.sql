-- Add webhook_token fields to users table
ALTER TABLE users 
ADD COLUMN IF NOT EXISTS webhook_token TEXT UNIQUE DEFAULT ('wh_' || replace(replace(replace(encode(gen_random_bytes(24), 'base64'), '+', ''), '/', ''), '=', '')),
ADD COLUMN IF NOT EXISTS webhook_token_updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();

-- Create index on webhook_token for faster lookups (optional, but good practice for unique lookups)
-- Note: UNIQUE constraint already creates an implicit index, so this is just being explicit
COMMENT ON COLUMN users.webhook_token IS 'Unique token for webhook authentication, auto-generated';
COMMENT ON COLUMN users.webhook_token_updated_at IS 'Timestamp of when webhook token was last updated';
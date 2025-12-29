-- Create storage bucket for technology detection HTML samples
INSERT INTO storage.buckets (id, name, public, file_size_limit, allowed_mime_types)
VALUES (
    'tech-samples',
    'tech-samples',
    false,  -- Private bucket - accessed via service role
    5242880,  -- 5MB max file size
    ARRAY['text/html', 'application/octet-stream']::text[]
)
ON CONFLICT (id) DO NOTHING;

-- Allow service role full access to tech-samples bucket
-- Note: RLS is disabled by default for storage, service role bypasses anyway
CREATE POLICY "Service role can manage tech samples"
ON storage.objects
FOR ALL
TO service_role
USING (bucket_id = 'tech-samples')
WITH CHECK (bucket_id = 'tech-samples');

-- Update domains table: change tech_html_sample to store path instead of content
ALTER TABLE domains
    DROP COLUMN IF EXISTS tech_html_sample;

ALTER TABLE domains
    ADD COLUMN IF NOT EXISTS tech_html_path TEXT DEFAULT NULL;

COMMENT ON COLUMN domains.tech_html_path IS 'Path to HTML sample in tech-samples storage bucket';

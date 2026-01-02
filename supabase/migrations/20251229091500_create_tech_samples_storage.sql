-- Create storage bucket for crawled page HTML
INSERT INTO storage.buckets (id, name, public, file_size_limit, allowed_mime_types)
VALUES (
    'page-crawls',
    'page-crawls',
    false,  -- Private bucket - accessed via service role
    5242880,  -- 5MB max file size
    ARRAY['text/html', 'application/octet-stream']::text[]
)
ON CONFLICT (id) DO NOTHING;

-- Allow service role full access to page-crawls bucket
-- Note: RLS is disabled by default for storage, service role bypasses anyway
DROP POLICY IF EXISTS "Service role can manage page crawls" ON storage.objects;
CREATE POLICY "Service role can manage page crawls"
ON storage.objects
FOR ALL
TO service_role
USING (bucket_id = 'page-crawls')
WITH CHECK (bucket_id = 'page-crawls');

-- Update domains table: change tech_html_sample to store path instead of content
ALTER TABLE domains
    DROP COLUMN IF EXISTS tech_html_sample;

ALTER TABLE domains
    ADD COLUMN IF NOT EXISTS tech_html_path TEXT DEFAULT NULL;

COMMENT ON COLUMN domains.tech_html_path IS 'Path to HTML sample in page-crawls storage bucket';

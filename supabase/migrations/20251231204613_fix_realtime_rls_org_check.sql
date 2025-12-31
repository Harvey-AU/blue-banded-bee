-- Fix RLS policy on realtime.messages to check org membership
-- The previous policy allowed all authenticated users to read all broadcasts
-- This restricts it to only users who are members of the organisation in the topic

-- Drop the overly permissive policy
DROP POLICY IF EXISTS "Authenticated users can receive broadcasts" ON "realtime"."messages";

-- Create a policy that checks org membership based on topic
-- Topic format: notifications:{org_id}
CREATE POLICY "Users can receive broadcasts for their organisations"
ON "realtime"."messages"
FOR SELECT
TO authenticated
USING (
    -- Extract org_id from topic like 'notifications:uuid' and check membership
    CASE
        WHEN topic LIKE 'notifications:%' THEN
            substring(topic from 'notifications:(.+)')::uuid IN (SELECT public.user_organisations())
        ELSE
            false
    END
);

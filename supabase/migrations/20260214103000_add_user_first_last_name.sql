-- Add explicit first/last name columns while retaining full_name.
ALTER TABLE users
ADD COLUMN IF NOT EXISTS first_name TEXT,
ADD COLUMN IF NOT EXISTS last_name TEXT;

-- Keep historical full_name data usable if needed.
UPDATE users
SET
  first_name = COALESCE(NULLIF(trim(first_name), ''), NULLIF(split_part(trim(full_name), ' ', 1), '')),
  last_name = COALESCE(
    NULLIF(trim(last_name), ''),
    NULLIF(
      CASE
        WHEN strpos(trim(full_name), ' ') > 0
          THEN trim(substr(trim(full_name), strpos(trim(full_name), ' ') + 1))
        ELSE ''
      END,
      ''
    )
  )
WHERE full_name IS NOT NULL
  AND trim(full_name) <> '';

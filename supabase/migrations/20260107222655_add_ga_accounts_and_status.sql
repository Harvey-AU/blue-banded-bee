-- Add Google Analytics accounts table and status column for two-step selection flow
--
-- Flow:
-- 1. OAuth → Fetch accounts → Store in google_analytics_accounts
-- 2. User selects account → Fetch properties → Store in google_analytics_connections with status='inactive'
-- 3. User toggles properties active/inactive

-- ============================================================================
-- 1. Create google_analytics_accounts table
-- ============================================================================
CREATE TABLE IF NOT EXISTS google_analytics_accounts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
  google_account_id TEXT NOT NULL,           -- GA account ID (e.g., "accounts/123456")
  google_account_name TEXT,                   -- Display name of the account
  google_user_id TEXT,                        -- Google user ID who authorised
  google_email TEXT,                          -- Google email for display
  vault_secret_name TEXT,                     -- Shared token stored in Vault
  installing_user_id UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(organisation_id, google_account_id)
);

CREATE INDEX IF NOT EXISTS idx_ga_accounts_org ON google_analytics_accounts(organisation_id);

-- Enable RLS
ALTER TABLE google_analytics_accounts ENABLE ROW LEVEL SECURITY;

-- RLS policies for accounts
CREATE POLICY "ga_accounts_select_own_org" ON google_analytics_accounts
  FOR SELECT USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

CREATE POLICY "ga_accounts_insert_own_org" ON google_analytics_accounts
  FOR INSERT WITH CHECK (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

CREATE POLICY "ga_accounts_update_own_org" ON google_analytics_accounts
  FOR UPDATE USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

CREATE POLICY "ga_accounts_delete_own_org" ON google_analytics_accounts
  FOR DELETE USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

-- Updated at trigger
CREATE TRIGGER update_ga_accounts_updated_at
  BEFORE UPDATE ON google_analytics_accounts
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- 2. Add status and account reference to connections
-- ============================================================================
ALTER TABLE google_analytics_connections
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'inactive',
  ADD COLUMN IF NOT EXISTS google_account_id TEXT;

-- Index for filtering by status
CREATE INDEX IF NOT EXISTS idx_ga_connections_status ON google_analytics_connections(organisation_id, status);

-- Add constraint for valid status values
ALTER TABLE google_analytics_connections
  DROP CONSTRAINT IF EXISTS ga_connections_status_check;
ALTER TABLE google_analytics_connections
  ADD CONSTRAINT ga_connections_status_check CHECK (status IN ('active', 'inactive'));

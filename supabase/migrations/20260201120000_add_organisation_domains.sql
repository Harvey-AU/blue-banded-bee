-- Map organisations to registered domains (including non-job domains)
CREATE TABLE IF NOT EXISTS organisation_domains (
  organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
  domain_id INT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (organisation_id, domain_id)
);

CREATE INDEX IF NOT EXISTS idx_org_domains_org ON organisation_domains(organisation_id);
CREATE INDEX IF NOT EXISTS idx_org_domains_domain ON organisation_domains(domain_id);

ALTER TABLE organisation_domains ENABLE ROW LEVEL SECURITY;

CREATE POLICY "org_domains_select_own_org" ON organisation_domains
  FOR SELECT USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

CREATE POLICY "org_domains_insert_own_org" ON organisation_domains
  FOR INSERT WITH CHECK (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

CREATE POLICY "org_domains_delete_own_org" ON organisation_domains
  FOR DELETE USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

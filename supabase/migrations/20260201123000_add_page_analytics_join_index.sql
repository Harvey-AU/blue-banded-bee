-- Composite index to support page_analytics joins by organisation, domain, and path
CREATE INDEX IF NOT EXISTS idx_page_analytics_org_domain_path
  ON page_analytics(organisation_id, domain_id, path);

-- Add workspace_name column to webflow_connections
ALTER TABLE webflow_connections ADD COLUMN workspace_name TEXT;

COMMENT ON COLUMN webflow_connections.workspace_name IS 'Display name of the Webflow workspace';

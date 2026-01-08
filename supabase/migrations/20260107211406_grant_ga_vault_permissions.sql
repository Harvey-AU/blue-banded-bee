-- Grant execute permissions on GA vault functions
-- Matching the pattern from Webflow functions in 20260104160000_fix_vault_authorisation.sql

-- Grant execute to service_role (used by backend API)
GRANT EXECUTE ON FUNCTION store_ga_token(UUID, TEXT) TO service_role;
GRANT EXECUTE ON FUNCTION get_ga_token(UUID) TO service_role;
GRANT EXECUTE ON FUNCTION delete_ga_token(UUID) TO service_role;

-- Grant execute to authenticated (for direct client access if needed)
GRANT EXECUTE ON FUNCTION store_ga_token(UUID, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION get_ga_token(UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION delete_ga_token(UUID) TO authenticated;

COMMENT ON FUNCTION store_ga_token IS 'Stores Google Analytics refresh token securely in vault';
COMMENT ON FUNCTION get_ga_token IS 'Retrieves Google Analytics refresh token from vault';
COMMENT ON FUNCTION delete_ga_token IS 'Deletes Google Analytics refresh token from vault';

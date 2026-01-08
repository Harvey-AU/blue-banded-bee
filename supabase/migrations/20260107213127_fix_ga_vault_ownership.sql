-- Fix GA vault function ownership and permissions
-- On preview branches, functions may be owned by a user without vault access
-- Ensure functions are owned by postgres and have proper grants

-- Set function ownership to postgres (has vault access)
ALTER FUNCTION store_ga_token(UUID, TEXT) OWNER TO postgres;
ALTER FUNCTION get_ga_token(UUID) OWNER TO postgres;
ALTER FUNCTION delete_ga_token(UUID) OWNER TO postgres;
ALTER FUNCTION cleanup_ga_vault_secret() OWNER TO postgres;

-- Ensure grants are in place
GRANT EXECUTE ON FUNCTION store_ga_token(UUID, TEXT) TO service_role;
GRANT EXECUTE ON FUNCTION get_ga_token(UUID) TO service_role;
GRANT EXECUTE ON FUNCTION delete_ga_token(UUID) TO service_role;
GRANT EXECUTE ON FUNCTION store_ga_token(UUID, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION get_ga_token(UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION delete_ga_token(UUID) TO authenticated;

-- Also ensure the cleanup trigger function can be executed
GRANT EXECUTE ON FUNCTION cleanup_ga_vault_secret() TO postgres;

/**
 * Supabase configuration - SINGLE SOURCE OF TRUTH
 * Update these values here and they'll be used everywhere
 */

export const SUPABASE_CONFIG = {
  url: 'https://auth.bluebandedbee.co',
  anonKey: 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imdwemp0Ymd0ZGp4bmFjZGZ1anZ4Iiwicm9sZSI6ImFub24iLCJpYXQiOjE3NDUwNjYxNjMsImV4cCI6MjA2MDY0MjE2M30.eJjM2-3X8oXsFex_lQKvFkP1-_yLMHsueIn7_hCF6YI'
};

// For direct HTML use (copy this exactly):
export const SUPABASE_HTML_CONFIG = `
window.supabase = window.supabase.createClient(
  '${SUPABASE_CONFIG.url}',
  '${SUPABASE_CONFIG.anonKey}'
);
`;
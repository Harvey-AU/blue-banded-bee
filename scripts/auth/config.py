"""Shared CLI auth configuration.

This module centralises the default Supabase settings used by CLI utilities so
they can run out of the box. The anon key below is the same publishable key
embedded in the web auth modal; rotate it here if Supabase credentials change.
"""

SUPABASE_URL = "https://auth.bluebandedbee.co"
DEFAULT_SUPABASE_ANON_KEY = (
    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imdwemp0Ymd0ZGp4bmFjZGZ1anZ4Iiwicm9sZSI6ImFub24iLCJpYXQiOjE3NDUwNjYxNjMsImV4cCI6MjA2MDY0MjE2M30.eJjM2-3X8oXsFex_lQKvFkP1-_yLMHsueIn7_hCF6YI"
)

"""Shared CLI auth configuration.

This module centralises the default Supabase settings used by CLI utilities so
they can run out of the box. The anon key below is the same publishable key
embedded in the web auth modal; rotate it here if Supabase credentials change.
"""

import os

SUPABASE_URL = "https://auth.bluebandedbee.co"
DEFAULT_SUPABASE_ANON_KEY = os.environ.get(
    "SUPABASE_ANON_KEY",
    # Fallback to empty in production/CI if not set, preventing leak
    "",
)

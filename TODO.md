# TODO - Link Extraction Verification

## Critical Test Required

**VERIFY LINK EXTRACTION FIX WORKS**

Test case: `https://www.teamharvey.co/stories`

Expected behavior:
- Should find pagination links: `?b84bb98f_page=1`, `?b84bb98f_page=3`
- Job should show `found_tasks > 0` (not 0)
- Check in Supabase: SELECT found_tasks FROM jobs WHERE domain = 'teamharvey.co' ORDER BY created_at DESC LIMIT 1

If still showing `found_tasks = 0`, then the context bug fix didn't work and need to investigate further:
1. Check if `find_links` context is being set properly in `WarmURL()` 
2. Add more debug logging to see exactly what's happening in link extraction
3. Possibly the issue is elsewhere in the pipeline

## Next Steps if Fix Doesn't Work

1. Add debug logging to see context values being set
2. Check if Colly OnHTML handlers are even being triggered
3. Verify the specific HTML structure matches our selectors
4. Test with simpler page first
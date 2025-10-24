# Security Fix: Shared Resource UPDATE Policies

**Date:** 2025-10-24 **Issue:** Cross-tenant data corruption vulnerability in
domains/pages tables

## Problem Identified

Initial implementation allowed tenant UPDATE policies on shared resources
(domains, pages), creating a critical security vulnerability:

### Attack Vector

1. **Domains Table:** Multiple organisations can reference the same `domains.id`
2. **Pages Table:** Multiple organisations can reference pages for the same
   domain
3. **Vulnerable Policy:** Any user with a job referencing a domain gained UPDATE
   rights
4. **Impact:** User from Org A could modify domain metadata used by Org B

### Example Attack

```sql
-- Attacker from Org A creates job for victim's domain
INSERT INTO jobs (domain_id, organisation_id) VALUES (123, 'org-a');

-- Policy grants UPDATE because attacker has a job for domain 123
UPDATE domains SET name = 'malicious', robots_crawl_delay = 0
WHERE id = 123;

-- Victim in Org B sees corrupted domain data
SELECT * FROM domains WHERE id = 123; -- Shows attacker's changes
```

## Solution Implemented

Removed UPDATE policies for shared resources. Only service role can modify:

### Before (Vulnerable)

```sql
-- VULNERABLE: Allows cross-tenant mutation
CREATE POLICY "Users can update domains via jobs"
ON domains FOR UPDATE
USING (
  EXISTS (
    SELECT 1 FROM jobs
    WHERE jobs.domain_id = domains.id
      AND jobs.organisation_id = (...)
  )
);
```

### After (Secure)

```sql
-- SECURE: No UPDATE policy = service role only
-- NO UPDATE POLICY: Domains are shared resources
-- Service role only can update to prevent cross-tenant data corruption
```

## Policy Matrix

| Table   | Operation | Policy              | Reasoning                                 |
| ------- | --------- | ------------------- | ----------------------------------------- |
| domains | SELECT    | Restricted via jobs | Users only see domains they have jobs for |
| domains | INSERT    | Authenticated users | Workers need to create during discovery   |
| domains | UPDATE    | Service role only   | Prevents cross-tenant mutation            |
| domains | DELETE    | Service role only   | Prevents data loss                        |
| pages   | SELECT    | Restricted via jobs | Users only see pages via their jobs       |
| pages   | INSERT    | Authenticated users | Workers discover during crawling          |
| pages   | UPDATE    | Service role only   | Prevents cross-tenant mutation            |
| pages   | DELETE    | Service role only   | Prevents data loss                        |

## Impact

### Security

- ✅ Prevents cross-tenant data corruption
- ✅ Maintains strict read isolation (users only see their jobs' data)
- ✅ Allows workers to create new shared resources
- ✅ Protects shared metadata integrity

### Functionality

- ✅ Workers can still INSERT domains/pages (service role)
- ✅ Users can still SELECT domains/pages via their jobs
- ✅ Backend can UPDATE via service role when needed
- ⚠️ Dashboard/API cannot UPDATE domains/pages directly

## Migration Applied

Both migration file and bootstrap code updated:

1. `supabase/migrations/20251024113328_optimise_rls_policies.sql`
2. `internal/db/db.go` - `setupRLSPolicies()` function

## Testing

- ✅ All unit tests passing
- ✅ Build verification successful
- ✅ RLS policy tests passing
- ✅ Bootstrap code matches migration

## Related Issues

- Original optimisation:
  [Performance Optimisation Summary](./performance-optimisation-summary.md)
- Database architecture: [DATABASE.md](../architecture/DATABASE.md)

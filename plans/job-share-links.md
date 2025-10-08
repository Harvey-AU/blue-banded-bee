# Job Share Link Plan

## Overview

The backend infrastructure for public job share links is in place, but the end-to-end experience still needs UI, polish, and test coverage. This plan captures the current state and the remaining work so we can resume quickly.

## Completed

- **Database + Migrations**
  - `job_share_links` table (token, created/revoked/expiry metadata) with indexes.
- **API Endpoints**
  - Authenticated routes: `POST /v1/jobs/{id}/share-links` (create or return active link) and `DELETE /v1/jobs/{id}/share-links/{token}` (revoke).
  - Public routes: `GET /v1/shared/jobs/{token}`, `/tasks`, `/export` that reuse the job/task formatting pipeline.
  - Shared endpoints share `fetchJobResponse` and `serveJobExport` with the private API.
- **Routing**
  - `Handler.SetupRoutes` exposes `/v1/shared/jobs/…`.
- **Job Page Wiring (partial)**
  - Backend logic available in `job-page.js` to copy the current page URL (no integration with share APIs yet). 

## Outstanding

- **Frontend Controls**
  - Add “Generate link” / “Copy link” / “Revoke link” buttons (dashboard list + job page).
  - Display link state (active token, expiry if implemented) and confirmation toasts.
  - Handle API errors (rate limiting, no permission, revoked/expired).
- **Public Job View**
  - Decide on URL (`/shared/jobs/{token}` page vs `jobs/{id}?token=...` switch).
  - Build read-only template that consumes `/v1/shared/jobs/{token}` + `/tasks` + `/export`.
  - Hide private controls (restart/cancel) and sensitive metadata.
  - Ensure exports initiated from public page hit the shared endpoints.
- **Security / Expiry Enhancements**
  - Optional expiry handling in create/revoke flow.
  - Consider RLS / Supabase policies for additional defence-in-depth.
- **Testing**
  - Unit tests for share link creation/revocation (Go). 
  - API tests covering valid token, revoked, expired, invalid.
  - Shared export parity tests.
- **Documentation**
  - Update README/CLAUDE instructions for share link workflow.
  - Add API examples (create/revoke/share) + UI usage notes.

## Next Steps

1. Finalise UX for the share controls and public job page.
2. Implement dashboard & job page UI with API wiring and copy/revoke flow.
3. Build the public read-only job view consuming the shared endpoints.
4. Add automated tests (Go unit + API) for share link lifecycle.
5. Update documentation, ensure monitoring/Sentry covers new flows.


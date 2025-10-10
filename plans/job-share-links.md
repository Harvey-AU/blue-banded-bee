# Job Share Link Plan

## Overview

The backend infrastructure for public job share links is in place and the standalone public view is live. Remaining work focuses on surfacing controls in the dashboard, optional expiry/policy hardening, and documenting the workflow.

## Completed

- **Database + Migrations**
  - `job_share_links` table (token, created/revoked/expiry metadata) with indexes.
- **API Endpoints**
  - Authenticated routes: `POST /v1/jobs/{id}/share-links` (create or return active link) and `DELETE /v1/jobs/{id}/share-links/{token}` (revoke).
  - Public routes: `GET /v1/shared/jobs/{token}`, `/tasks`, `/export` that reuse the job/task formatting pipeline.
  - Shared endpoints share `fetchJobResponse` and `serveJobExport` with the private API.
- **Routing**
  - `Handler.SetupRoutes` exposes `/v1/shared/jobs/…`.
- **Frontend Controls**
  - Job details page now generates, copies, and revokes share links via authenticated API calls.
  - Active link state is displayed with contextual toasts and defensive error handling.
- **Public Job View**
  - `/shared/jobs/{token}` reuses the standalone job page in a read-only mode driven by the shared endpoints.
  - Owner-only controls remain hidden while exports use the shared pipeline.
- **Testing**
  - Share link lifecycle covered with Go tests for create/reuse/revoke and the public shared endpoints (tasks + export).

## Outstanding

- **Dashboard Integration**
  - Surface share controls from the main dashboard list / modal for quicker access.
- **Security / Expiry Enhancements**
  - Optional expiry handling in create/revoke flow (current stance: tokens do not expire automatically; revoke manually when needed).
  - No additional RLS changes required—shared endpoints already bypass tenant auth by design.
- **Documentation**
  - Update README/CLAUDE instructions for share link workflow.
  - Add API examples (create/revoke/share) + UI usage notes.

## Next Steps

1. Extend dashboard list/modal with quick generate/copy controls.
2. Decide on expiry / policy requirements and implement if needed.
3. Update documentation and rollout notes, including monitoring guidance.

# Frontend HTML population review

## Recommendations for tidy-up issue

- Standardise template population: schedules and integrations manually clone
  `bbb-template` nodes while job lists use the binder. Converging on one
  approach reduces duplicated mapping logic.
- Unify conditional visibility: job details uses `bbb-show` attributes but
  visibility is toggled by `job-page.js`. Pick one mechanism to avoid mixed
  expectations.
- Reduce `innerHTML` usage: notifications list and job tasks table are built
  with `innerHTML`. Prefer `document.createElement` or a binder template to
  reduce string assembly.
- Centralise auth modal ownership: `auth-modal.html` includes inline script
  while `web/static/js/auth.js` manages the modal. Remove inline scripts after
  confirming parity.

## Purpose

Provide a clear map of how HTML is populated by JavaScript across the frontend,
noting where the data binder is used versus direct DOM manipulation (including
class/data attributes).

## Binding systems in play

- **BBDataBinder** (`web/static/js/bb-data-binder.js`)
  - Supports `bbb-*` and legacy `data-bb-*` attributes for text, styles,
    attributes, templates, auth visibility, and conditional rendering.
  - Core methods used by pages: `scanAndBind()`, `updateElements()`, and
    `bindTemplates()`.
- **Auth modal injection** (`web/static/js/auth.js`)
  - Fetches and injects `/auth-modal.html` into `#authModalContainer` using
    `innerHTML`, then controls the modal using `textContent`, `style.display`,
    and event delegation.
- **Inline scripts** (notably in `dashboard.html` and `homepage.html`)
  - Populate UI elements directly via DOM APIs and `innerHTML` for dynamic
    blocks (notifications, org list, quota, etc).

## Page-by-page population map

## Quick reference table

| Page        | Primary HTML                     | Population method(s)                                 | Primary JS sources                                                                                                                                                                     | Notes                                                                                    |
| ----------- | -------------------------------- | ---------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| Job details | `web/templates/job-details.html` | Binder + direct DOM + `innerHTML`                    | `web/static/js/bb-data-binder.js`, `web/static/js/job-page.js`, `web/static/js/bb-metadata.js`                                                                                         | Tasks table and metrics visibility are manual; binder handles most labels.               |
| Dashboard   | `dashboard.html`                 | Binder + template cloning + direct DOM + `innerHTML` | `web/static/js/bb-auth-extension.js`, `web/static/js/bb-dashboard-actions.js`, `web/static/js/bb-slack.js`, `web/static/js/bb-webflow.js`, `web/static/js/bb-google.js`, inline script | Notifications and org switcher are inline DOM; integrations use manual template cloning. |
| Homepage    | `homepage.html`                  | Direct DOM                                           | Inline script + `web/static/js/auth.js`                                                                                                                                                | No binder usage; triggers auth modal.                                                    |
| Auth modal  | `auth-modal.html`                | `innerHTML` injection + direct DOM                   | `web/static/js/auth.js`                                                                                                                                                                | Modal HTML is fetched and inserted at runtime.                                           |
| CLI login   | `cli-login.html`                 | Direct DOM                                           | `web/static/js/auth.js`                                                                                                                                                                | Shares auth modal injection logic.                                                       |

### Job details

**HTML**: `web/templates/job-details.html`

**Binder-driven content**

- `bbb-text` for job fields and metrics (e.g. `bbb-text="job.domain"`,
  `bbb-text="metrics.cache.hits"`).
- `bbb-class` for status pill (e.g.
  `bbb-class="status-pill {job.status_class}"`).
- `bbb-help` for metric tooltips (metadata injected by `bb-metadata.js`).

**Direct DOM updates (non-binder)**

- **Metrics visibility**: `data-metric-group` and `data-metric-field` sections
  are shown/hidden by `applyMetricsVisibility()` in `web/static/js/job-page.js`.
- **Action buttons**: `bbb-show` attributes exist in HTML, but actual visibility
  is set by `updateActionButtons()` in `web/static/js/job-page.js`.
- **Tasks table**: built with `innerHTML` in `renderTasksTable()` and headers
  with `innerHTML` in `renderTaskHeader()`.
- **Pagination summary**: binder handles `bbb-text="tasks.pagination.summary"`,
  but the pagination controls themselves are toggled in `updatePagination()`.
- **Share controls**: `textContent`, `href`, and `style.display` updated in
  `updateShareControls()`.

**Key sources**

- Binding library: `web/static/js/bb-data-binder.js`
- Page logic: `web/static/js/job-page.js`
- Tooltips: `web/static/js/bb-metadata.js`

### Dashboard

**HTML**: `dashboard.html`

**Binder-driven content**

- **Stats cards**: `bbb-text="stats.*"` populated via `dataBinder.loadAndBind()`
  in `web/static/js/bb-auth-extension.js`.
- **Recent jobs list**: `bbb-template="job"` and
  `bbb-text`/`bbb-class`/`bbb-style` populated via `dataBinder.bindTemplates()`
  in `web/static/js/bb-auth-extension.js`.

**Template markup reused with manual DOM updates (not binder)**

- **Schedules list**: `bbb-template="schedule"` exists in HTML, but
  `web/static/js/bb-dashboard-actions.js` clones the template and sets
  `textContent`, `setAttribute`, and `bbb-id` directly.
- **Slack connections**: `bbb-template="slack-connection"` in HTML, but
  `web/static/js/bb-slack.js` clones and fills via `textContent` and
  `setAttribute`.
- **Webflow connections**: `bbb-template="webflow-connection"` in HTML, but
  `web/static/js/bb-webflow.js` clones and fills via `textContent` and
  `setAttribute`.
- **Webflow site rows**: `bbb-template="webflow-site"` in HTML, but
  `web/static/js/bb-webflow.js` clones rows, sets `dataset` values,
  `textContent`, and hooks event listeners.
- **Google connections**: `bbb-template="google-connection"` in HTML, but
  `web/static/js/bb-google.js` clones and fills via `textContent`, `classList`,
  and `setAttribute`.

**Inline dashboard scripts (direct DOM manipulation)**

- **Org switcher**: creates buttons with `dataset.orgId`/`dataset.orgName`, sets
  `textContent`, and toggles `classList` in the inline script in
  `dashboard.html`.
- **Notifications**: builds notification list with `innerHTML` and uses
  `data-id`/`data-link` attributes for click handling.
- **Quota display**: updates plan and usage via `textContent`, toggles classes
  based on usage percentage.
- **Modal/feedback UI**: error banners and modal visibility controlled via
  `innerHTML`, `classList`, and `style.display`.

**Key sources**

- Binder and dashboard refresh: `web/static/js/bb-data-binder.js`,
  `web/static/js/bb-auth-extension.js`
- Actions and schedules: `web/static/js/bb-dashboard-actions.js`
- Integrations: `web/static/js/bb-slack.js`, `web/static/js/bb-webflow.js`,
  `web/static/js/bb-google.js`
- Inline scripts: `dashboard.html`

### Homepage

**HTML**: `homepage.html`

**Direct DOM updates (non-binder)**

- Uses plain DOM handlers to read inputs, validate, and trigger auth modal.
- Stores `bb_pending_domain` in `sessionStorage` and relies on auth flow to
  resume.

**Key sources**

- Inline script in `homepage.html`
- Auth modal loaded by `web/static/js/auth.js` (via `web/static/js/core.js`)

### Auth modal

**HTML**: `auth-modal.html` (loaded into `#authModalContainer`)

**Direct DOM updates**

- Toggled by `web/static/js/auth.js` using `textContent` and `style.display`.
- Modal is injected using `innerHTML` when fetched.
- The modal markup includes an inline script with DOM manipulation (legacy); in
  practice, `auth.js` provides the primary runtime handlers.

**Key sources**

- `web/static/js/auth.js` (loads and controls modal)
- `auth-modal.html` (markup + inline legacy handlers)

### CLI login

**HTML**: `cli-login.html`

**Direct DOM updates**

- CLI status text and classes updated in `initCliAuthPage()` (part of
  `web/static/js/auth.js`).
- Auth modal injected into `#authModalContainer` using the same `auth.js` flow.

## Summary of population mechanisms

- **Binder-based**: `bbb-text`, `bbb-style:*`, `bbb-class`, `bbb-href`,
  `bbb-template`, `bbb-auth` in `BBDataBinder`.
- **Template cloning with manual fill**: schedule and integration lists use
  `bbb-template` markup but are populated via DOM APIs in their respective
  scripts.
- **Direct DOM (textContent/setAttribute/classList)**: org switcher, quota,
  integrations, job actions, share links, metrics visibility.
- **Direct DOM (innerHTML)**: notifications list, tasks table (job details), and
  some error banners.

## Notes on attribute-driven behaviour

- The codebase uses **both** binding attributes and **class/data attributes** as
  state carriers.
- Some elements include `bbb-show` in HTML, but the actual visibility on page is
  driven by page scripts (`job-page.js`) rather than the binder’s conditional
  evaluator.
- Templates are frequently reused for layout, but not always populated through
  the binder.

# Attribute System Migration Plan

## Current State Analysis

### Files Affected

- **HTML files**: 1 file with attributes (`dashboard.html` - 74 occurrences)
- **JavaScript files**: 6 files using attribute selectors

### Attribute Usage Breakdown

#### In HTML (`dashboard.html`)

- `data-bb-bind`: 36 occurrences
- `bb-action`: 21 occurrences
- `data-bb-show-if`: 5 occurrences
- `data-bb-template`: 4 occurrences
- `data-bb-info`: 4 occurrences
- `data-bb-bind-attr`: 4 occurrences
- `data-bb-auth`: 4 occurrences
- `data-bb-bind-style`: 1 occurrence
- `data-bb-tooltip`: 1 occurrence

#### In JavaScript

Core files using attribute selectors:

- `bb-data-binder.js` - Main data binding library
- `bb-dashboard-actions.js` - Action delegation system
- `bb-metadata.js` - Metadata/info tooltip system
- `bb-auth-extension.js` - Auth state management
- `auth.js` - Auth visibility controls
- `bb-components.js` - Reusable components

Additional attributes found in JavaScript (forms/validation):

- `data-bb-form`, `data-bb-validate`, `data-bb-endpoint`, etc.

## Migration Strategy

### Phase 1: JavaScript Backwards Compatibility (Days 1-2)

**Goal**: Update all JavaScript to recognize BOTH old and new attributes
simultaneously.

**Files to Update:**

1. **`web/static/js/bb-data-binder.js`**
   - Update `scanAndBind()` to query both `[data-bb-bind]` and `[bbb-text]`
   - Update `registerBindElement()` to check both attribute names
   - Update style binding to recognize both `data-bb-bind-style` and
     `bbb-style:`
   - Update attr binding to recognize both `data-bb-bind-attr` and
     `bbb-class/bbb-href/bbb-attr:`
   - Update template system to recognize both `data-bb-template` and
     `bbb-template`
   - Update conditional rendering to recognize both `data-bb-show-if` and
     `bbb-show/bbb-hide/bbb-if`
   - Update auth system to recognize both `data-bb-auth` and `bbb-auth`

2. **`web/static/js/bb-dashboard-actions.js`**
   - Update action delegation to recognize both `bb-action` and `bbb-action`
   - Update attribute reading for job IDs, etc.

3. **`web/static/js/bb-metadata.js`**
   - Update `initializeInfoIcons()` to query both `[data-bb-info]` and
     `[bbb-help]`
   - Update tooltip attribute checks

4. **`web/static/js/auth.js`**
   - Update auth element queries to recognize both `[data-bb-auth]` and
     `[bbb-auth]`

5. **`web/static/js/bb-auth-extension.js`**
   - Update any auth attribute references

6. **`web/static/js/bb-components.js`**
   - Update component attribute selectors

**Testing Phase 1:**

- Run existing dashboard
- Verify all current functionality works
- No visual or functional changes should occur

### Phase 2: HTML Migration (Days 3-5)

**Goal**: Migrate HTML incrementally, section by section, testing after each.

**Migration Order for `dashboard.html`:**

#### Section 1: Header & Stats (Lines ~1265-1420)

- Auth visibility: `data-bb-auth` → `bbb-auth`
- Stats cards: `data-bb-bind` → `bbb-text`
- Actions: `bb-action` → `bbb-action`

**Attributes to migrate:**

```
data-bb-auth="guest" → bbb-auth="guest"
data-bb-auth="required" → bbb-auth="required"
data-bb-bind="stats.total_jobs" → bbb-text="stats.total_jobs"
bb-action="refresh-dashboard" → bbb-action="refresh-dashboard"
bb-action="create-job" → bbb-action="create-job"
```

**Test**: Verify header shows/hides based on auth, stats populate correctly

#### Section 2: Job Cards Template (Lines ~1436-1467)

- Template definition: `data-bb-template` → `bbb-template`
- Text binding: `data-bb-bind` → `bbb-text`
- Attribute binding: `data-bb-bind-attr` → `bbb-class`, `bbb-attr:`
- Style binding: `data-bb-bind-style` → `bbb-style:`
- Conditional visibility: `data-bb-show-if` → `bbb-show`
- Actions with IDs: `bb-action` → `bbb-action`, add `bbb-id`

**Attributes to migrate:**

```
data-bb-template="job" → bbb-template="job"
data-bb-bind="domain" → bbb-text="domain"
data-bb-bind-attr="class:bb-status-{status}" → bbb-class="bb-status-{status}"
data-bb-bind-style="width:{progress}%" → bbb-style:width="{progress}%"
data-bb-show-if="status=completed,failed,cancelled" → bbb-show="status=completed,failed,cancelled"
bb-action="view-job-details" → bbb-action="view-job-details"
bb-data-job-id="{id}" → bbb-id="{id}"
```

**Test**: Verify job cards render, actions work, conditional buttons show/hide

#### Section 3: Analysis Sections (Lines ~1468-1527)

- Slow pages template
- External redirects template
- Conditional empty states

**Attributes to migrate:**

```
data-bb-template="slow_page" → bbb-template="slow_page"
data-bb-template="external_redirect" → bbb-template="external_redirect"
data-bb-bind="url" → bbb-text="url"
data-bb-show-if="slow_pages.length=0" → bbb-show="slow_pages.length=0"
```

**Test**: Verify slow pages and redirects display correctly

#### Section 4: Job Modal (Lines ~1529-1669)

- Modal header and stats
- Info tooltips: `data-bb-info` → `bbb-help`
- Task table (dynamically generated in JS - already updated)
- Modal actions

**Attributes to migrate:**

```
data-bb-bind="id" → bbb-text="id"
data-bb-bind="domain" → bbb-text="domain"
data-bb-bind="status" → bbb-text="status"
data-bb-info="total_tasks" → bbb-help="total_tasks"
data-bb-info="cache_hit_rate" → bbb-help="cache_hit_rate"
data-bb-info="avg_response_time" → bbb-help="avg_response_time"
data-bb-info="p95_response_time" → bbb-help="p95_response_time"
bb-action="close-modal" → bbb-action="close-modal"
bb-action="restart-job-modal" → bbb-action="restart-job-modal"
bb-action="cancel-job-modal" → bbb-action="cancel-job-modal"
bb-action="toggle-export-menu" → bbb-action="toggle-export-menu"
bb-action="export-job" → bbb-action="export-job"
bb-action="export-broken-links" → bbb-action="export-broken-links"
bb-action="export-slow-pages" → bbb-action="export-slow-pages"
bb-action="refresh-tasks" → bbb-action="refresh-tasks"
bb-action="tasks-prev-page" → bbb-action="tasks-prev-page"
bb-action="tasks-next-page" → bbb-action="tasks-next-page"
```

**Test**: Verify modal opens, info icons appear with tooltips, export works,
pagination works

#### Section 5: Create Job Modal (Lines ~1670-1714)

- Form attributes
- Modal actions

**Attributes to migrate:**

```
bb-action="close-create-job-modal" → bbb-action="close-create-job-modal"
```

**Test**: Verify create job modal opens, form submits

**Other HTML files**: Currently no other files use the attribute system, so no
migration needed.

### Phase 3: JavaScript Cleanup (Day 6)

**Goal**: Simplify JavaScript to only support new attributes.

**After confirming HTML migration is complete:**

1. **Remove old attribute selectors** from all JavaScript files
2. **Simplify code** - no need to check both old and new
3. **Update comments/documentation** in code

**Files to update:**

- `bb-data-binder.js` - Remove `data-bb-bind`, `data-bb-bind-attr`, etc.
- `bb-dashboard-actions.js` - Remove `bb-action` (old)
- `bb-metadata.js` - Remove `data-bb-info`
- `auth.js` - Remove `data-bb-auth`
- `bb-auth-extension.js` - Remove old auth attributes
- `bb-components.js` - Remove old component attributes

### Phase 4: Final Testing & Documentation (Day 7)

1. **Comprehensive testing**
   - Test all dashboard functionality
   - Test in different browsers (Chrome, Firefox, Safari, Edge)
   - Test mobile responsiveness
   - Test with different auth states (logged in, logged out)
   - Test all action handlers
   - Test all tooltips and help icons

2. **Update documentation**
   - Mark migration as complete in MIGRATION_PLAN.md
   - Update README if attribute examples are shown
   - Update any developer onboarding docs

3. **Create PR**
   - Title: "Migrate to bbb- attribute system"
   - Include migration summary
   - Link to ATTRIBUTE_SYSTEM.md

## Migration Commands

### Useful Find/Replace Commands

**For HTML migration** (use carefully, review each):

```bash
# Backup first
cp dashboard.html dashboard.html.backup

# Simple replacements (can be automated)
sed -i 's/data-bb-auth="/bbb-auth="/g' dashboard.html
sed -i 's/data-bb-template="/bbb-template="/g' dashboard.html
sed -i 's/bb-action="/bbb-action="/g' dashboard.html
```

**For complex replacements** (do manually):

- `data-bb-bind="field"` → `bbb-text="field"` (context-dependent)
- `data-bb-bind-attr="class:value-{field}"` → `bbb-class="value-{field}"`
- `data-bb-bind-style="width:{progress}%"` → `bbb-style:width="{progress}%"`
- `data-bb-show-if="condition"` → `bbb-show="condition"`
- `data-bb-info="key"` → `bbb-help="key"`

## Rollback Plan

If issues arise during migration:

1. **Phase 1 rollback**: Revert JavaScript changes via git
2. **Phase 2 rollback**: Use backup files (`dashboard.html.backup`)
3. **Phase 3 rollback**: Revert to Phase 2 state (old attributes still work)

## Risk Assessment

**Low Risk:**

- JavaScript changes maintain backwards compatibility
- Incremental HTML migration allows testing each section
- Old attributes continue working during transition

**Medium Risk:**

- Complex attribute bindings (attr, style) require manual review
- Dynamic table generation in JavaScript needs careful testing

**High Risk:**

- None identified - migration is well-isolated

## Success Criteria

- [ ] All JavaScript files support new `bbb-` attributes
- [ ] All HTML uses new `bbb-` attributes exclusively
- [ ] All dashboard functionality works as before
- [ ] All tooltips/help icons display correctly
- [ ] All actions and event handlers work
- [ ] All conditional visibility works
- [ ] All data binding populates correctly
- [ ] Tests pass (manual testing checklist)
- [ ] Code is cleaner and more maintainable

## Timeline

- **Day 1-2**: Phase 1 - JavaScript backwards compatibility
- **Day 3-5**: Phase 2 - HTML migration (incremental, tested)
- **Day 6**: Phase 3 - JavaScript cleanup
- **Day 7**: Phase 4 - Final testing and documentation

**Total estimated time**: 7 days of focused work

## Notes

- Migration can be paused between sections
- Each section is independently testable
- Old system remains functional until Phase 3
- Can be done incrementally over multiple PRs if preferred

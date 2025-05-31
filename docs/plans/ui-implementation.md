# UI Implementation Plan

## Overview

Blue Banded Bee provides a template + data binding system that allows users to build custom HTML layouts whilst the JavaScript handles data fetching, authentication, and real-time updates.

## User Interface Strategy

**Primary Interface:** Template + data binding system for building Blue Banded Bee's own product dashboard
**Secondary Interfaces:** 
- **Webflow App:** Installed in user's Webflow workspace, shows crawl status and controls for their sites
- **Slack Bot:** Simple commands (`/crawl sitedomain.com`) with threaded progress updates

**Integration Philosophy:** 
- **Template binding** allows flexible dashboard design for Blue Banded Bee's own product
- **External integrations** provide simple, focused functionality within user's existing workflows

## Architecture Approach

### Template + Data Binding System

Blue Banded Bee's own dashboard pages use `data-bb-bind` attributes. The JavaScript library finds these elements and populates them with live data from the API.

**Template system controls:**
- All HTML structure and CSS styling for Blue Banded Bee's dashboard
- Page layout and design positioning
- Visual appearance and branding

**JavaScript handles:**
- Data fetching from API endpoints
- Authentication with Supabase
- Real-time updates and live syncing
- Finding and populating template elements

### Integration Method

```html
<!-- Blue Banded Bee dashboard HTML design -->
<div class="bb-dashboard-design">
  <div class="stat-card">
    <h3>Total Jobs</h3>
    <span class="big-number" data-bb-bind="total_jobs">0</span>
  </div>
  
  <div class="job-list">
    <div class="job-template" data-bb-template="job">
      <h4 data-bb-bind="domain">Domain loading...</h4>
      <div class="progress-bar">
        <div class="fill" data-bb-bind-style="width:{progress}%"></div>
      </div>
      <span data-bb-bind="status">pending</span>
    </div>
  </div>
</div>

<!-- Single script inclusion -->
<script src="https://app.bluebandedbee.co/js/bb-data-binder.js"></script>
```

## Data Binding Attributes

### Basic Data Binding
- `data-bb-bind="field_name"` - Binds element's text content to API data field
- `data-bb-bind-attr="href:{url}"` - Binds element attributes
- `data-bb-bind-style="width:{progress}%"` - Binds CSS styles with formatting

### Template Binding
- `data-bb-template="template_name"` - Marks element as template for repeated data
- Templates are cloned and populated for each data item

### Authentication Elements
- `data-bb-auth="required"` - Shows element only when authenticated
- `data-bb-auth="guest"` - Shows element only when not authenticated

## API Integration

### Data Sources
The JavaScript automatically fetches data from these endpoints:

**Dashboard Data:**
- `/v1/dashboard/stats` - Job statistics and counts
- `/v1/jobs` - Recent jobs list with progress

**Real-time Updates:**
- Supabase Realtime for live job progress
- Automatic re-fetch on data changes

### Authentication Flow
1. Supabase Auth handles login/logout
2. JWT tokens automatically included in API requests
3. Page elements shown/hidden based on auth state
4. Template binding paused until authenticated

## Implementation Phases

### Phase 1: Core Data Binding
- Basic `data-bb-bind` attribute support
- Authentication integration
- Simple template population
- Dashboard statistics binding

### Phase 2: Advanced Features
- Real-time updates via Supabase
- Form handling for job creation
- Progress indicators and live updates
- Error handling and user feedback

### Phase 3: Enhanced Integration
- Webflow-specific optimisations
- Performance improvements
- Advanced template features
- Custom event handling

## Development Approach

### JavaScript Library Structure
```javascript
class BBDataBinder {
  constructor() {
    this.authManager = new AuthManager();
    this.apiClient = new APIClient();
    this.templateEngine = new TemplateEngine();
  }

  async init() {
    await this.authManager.init();
    this.bindElements();
    this.setupRealtime();
  }

  bindElements() {
    // Find and bind data-bb-bind elements
    const elements = document.querySelectorAll('[data-bb-bind]');
    elements.forEach(el => this.bindElement(el));
  }

  async fetchData(endpoint) {
    return this.apiClient.get(endpoint);
  }
}
```

### Real-time Updates
```javascript
// Real-time communication flow
User Action → Template Update → API Request → Database
     ↓              ↑              ↓           ↓
UI Update ← Supabase Realtime ← Database Trigger
```

## External Integrations

### Webflow App Integration

**User Experience:**
1. User installs Blue Banded Bee app in their Webflow workspace
2. Opens app within Webflow Designer interface  
3. Logs in with existing Supabase Auth (same as main website)
4. Views last crawl status for current Webflow site
5. Can trigger "Crawl Now" or enable "Auto-crawl on publish"

**Technical Implementation:**
- Uses existing `/v1/jobs` API endpoints
- Integrates with Supabase Auth (no separate auth system)
- Webhook integration for automatic crawling on site publish
- Site detection from Webflow context

### Slack Bot Integration

**User Experience:**
1. Install Blue Banded Bee Slack app in workspace
2. Use `/crawl sitedomain.com` command to start cache warming
3. Receive progress updates as thread replies
4. Get completion summary with link to main dashboard

**Technical Implementation:**
- Uses existing `/v1/jobs` API endpoints
- Integrates with Supabase Auth system
- Real-time updates via existing job status APIs

## Performance Considerations

### Loading Strategy
- Lightweight JavaScript library (~50KB)
- Progressive enhancement approach
- Only fetches data when elements are present
- Efficient DOM querying and updates

### API Optimisation
- Intelligent caching of static data
- Batched API requests where possible
- Debounced real-time updates
- Minimal DOM manipulation

## Security Implementation

### Authentication
- JWT tokens with automatic refresh
- Secure token storage
- Authentication state management
- Protected data binding (auth-only content)

### Data Protection
- Input sanitisation for all bound data
- XSS protection in template rendering
- CSRF protection for API requests

## Launch Checklist

### Core Features
- [ ] Data binding system implementation
- [ ] Authentication integration
- [ ] Template engine for repeated content
- [ ] Real-time updates via Supabase

### Webflow Integration
- [ ] Test embedding in Webflow pages
- [ ] Verify no CSS conflicts
- [ ] Responsive design compatibility
- [ ] Performance optimisation

### Documentation
- [ ] Integration guide for users
- [ ] Data binding reference
- [ ] Example templates and layouts
- [ ] Troubleshooting guide
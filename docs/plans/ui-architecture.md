# User Interface Architecture

This document outlines the architecture for the Blue Banded Bee user interface, which will be built as a JavaScript application embedded within Webflow pages.

For detailed data flow and communication architecture, see [UI Data Flow](./ui-data-flow.md).

## Overview

The Blue Banded Bee interface consists of two core parts:

1. **Marketing Website**: Built entirely in Webflow
2. **Application Interface**: JavaScript application embedded within Webflow pages

This approach allows us to leverage Webflow's strengths for marketing content while maintaining a cohesive user experience throughout the product.

## Architecture Approach

### Technology Stack

- **Webflow**: Primary platform for all pages and content
- **JavaScript**: Custom application embedded within Webflow pages
- **Web Components / Custom Elements**: For encapsulated UI components
- **Supabase Auth**: Authentication and session management
- **Supabase Realtime**: For real-time job monitoring and updates

### Integration Method

We will use a component-based architecture built with native Web Components to create an application that:

1. Can be embedded directly into Webflow pages
2. Is fully encapsulated to avoid style conflicts
3. Provides a rich, interactive experience
4. Integrates with our Go-based API

## Implementation Details

### Web Components Architecture

The UI will be built using HTML Custom Elements (Web Components) which provide:

- Shadow DOM for style encapsulation
- HTML templates for reusable components
- Custom element lifecycle hooks
- Cross-framework compatibility

Example component structure:

```javascript
class JobDashboard extends HTMLElement {
  constructor() {
    super();
    this.attachShadow({ mode: "open" });
    // Initialize component
  }

  connectedCallback() {
    // Component mounted to DOM
    this.render();
    this.fetchJobs();
  }

  async fetchJobs() {
    // Fetch jobs from API
    const response = await fetch("/api/jobs", {
      headers: {
        Authorization: `Bearer ${this.getAuthToken()}`,
      },
    });
    this.jobs = await response.json();
    this.render();
  }

  render() {
    // Render component to shadow DOM
    this.shadowRoot.innerHTML = `
      <style>
        /* Component-specific styles */
      </style>
      <div class="dashboard">
        <h2>Your Jobs</h2>
        <div class="job-list">
          ${this.renderJobs()}
        </div>
      </div>
    `;
  }

  // Additional methods...
}

customElements.define("job-dashboard", JobDashboard);
```

### Webflow Integration

The application will be integrated into Webflow using these methods:

1. **Page Structure**: Create dashboard pages in Webflow with designated containers for the application
2. **Custom Code Embed**: Use Webflow's Custom Code element to embed our application
3. **Global Script**: Add application core in the site-wide "before </body>" script section
4. **Component Initialization**: Initialize components based on page context

Example Webflow integration:

```html
<!-- In Webflow Custom Code element -->
<div id="app-container">
  <job-dashboard></job-dashboard>
</div>
```

### Authentication Flow

1. **User Login**: Through Webflow Memberships or custom form
2. **Token Generation**: JWT generated via Supabase Auth
3. **Token Storage**: Securely stored in localStorage with appropriate expiry
4. **API Authentication**: JWT included in headers for API requests
5. **Session Management**: Background refresh of tokens as needed

### User Flows

#### Dashboard Experience

1. User logs in through Webflow Memberships
2. Dashboard loads with overview of user's jobs
3. Real-time updates show job progress
4. Interactive visualisations display cache performance

#### Job Creation

1. User navigates to "New Job" section
2. Enters domain and configuration options
3. Submits job which is sent to API
4. Receives immediate feedback and redirects to job monitoring

## Key UI Components

1. **Dashboard**: Overview of all jobs and account status
2. **Job Creator**: Interface for creating and configuring new jobs
3. **Job Monitor**: Real-time status and progress of running jobs
4. **Results Viewer**: visualisations of completed job data
5. **Account Manager**: User profile and subscription management
6. **Usage Stats**: visualisation of usage metrics and limits

## Responsive Design

The application will be fully responsive, adapting to:

- Desktop screens (primary workspace)
- Tablet devices (monitoring on the go)
- Mobile phones (quick status checks)

Responsive strategy:

- Fluid layouts using CSS grid and flexbox
- Component-specific media queries within Shadow DOM
- Touch-friendly controls for mobile devices

## Performance Considerations

1. **Lazy Loading**: Components load only when needed
2. **Code Splitting**: Separate core application from specialized components
3. **Lightweight Dependencies**: Minimal external libraries
4. **Shadow DOM**: Style encapsulation to prevent conflicts
5. **Optimized Rendering**: Efficient DOM updates for real-time data

## Development Workflow

1. Develop components locally with isolated testing
2. Build and minify code for production
3. Deploy to CDN for faster loading
4. Embed in Webflow via externally hosted script references

## Future Extensibility

The component-based architecture allows for:

1. Progressive enhancement with new features
2. Easy replacement of individual components
3. Potential migration path to a full SPA if needed
4. Integration with Webflow Designer extension in Stage 6

This approach provides a balance of immediate implementation speed (leveraging Webflow for UI) with the flexibility needed for a complex application interface.

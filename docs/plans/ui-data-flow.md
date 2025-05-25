# UI Data Flow Architecture

This document outlines the data flow architecture between the Webflow UI, Go backend, and Supabase database for Blue Banded Bee.

## Architecture Overview

```
+--------------------+       +----------------+       +--------------------+
|                    |       |                |       |                    |
|  Webflow Frontend  | <---> |  Go Backend   | <---> |  Supabase Database |
|  (JS Application)  |       |  API Server   |       |                    |
|                    |       |                |       |                    |
+--------------------+       +----------------+       +--------------------+
```

## Component Interactions

### 1. Initial Load & Authentication

```
+-------------+     +--------------+     +------------+     +-----------+
| User Visits |---->| Webflow Page |---->| Auth Check |---->| Load App  |
| Website     |     | Loads        |     | (Supabase) |     | Components|
+-------------+     +--------------+     +------------+     +-----------+
```

1. User visits the Webflow site
2. Webflow loads the page with embedded JavaScript
3. JavaScript checks for authentication token in local storage
4. If not authenticated, shows login interface
5. If authenticated, initializes and loads dashboard components

### 2. Authentication Flow

```
+-----------+     +------------+     +------------+     +----------------+
| User Login|---->| Supabase   |---->| JWT Token  |---->| Store Token    |
| Form      |     | Auth API   |     | Generated  |     | (localStorage) |
+-----------+     +------------+     +------------+     +----------------+
```

1. User enters credentials in login form
2. JavaScript sends credentials to Supabase Auth API directly
3. Supabase validates credentials and returns JWT token
4. Token is stored in localStorage with appropriate expiry
5. UI updates to show authenticated state

### 3. Data Request Pattern

```
+---------------+     +----------------+     +----------------+     +-----------------+
| UI Component  |---->| API Request    |---->| Go Backend    |---->| SQL Query to    |
| Needs Data    |     | with JWT Token |     | Validates JWT |     | Supabase/Postgres|
+---------------+     +----------------+     +----------------+     +-----------------+
        ^                                                                   |
        |                                                                   |
        +-------------------------------------------------------------------+
                                 Data Response
```

1. UI component initializes and needs data (e.g., list of jobs)
2. JavaScript makes HTTP request to Go backend API with JWT token in Authorization header
3. Go backend validates JWT token with Supabase JWT secret
4. If valid, Go backend queries PostgreSQL database directly
5. Go backend processes data (filtering, formatting, etc.)
6. Go backend returns JSON data to frontend
7. UI component updates to display the data

### 4. Real-time Updates

For real-time job progress, we'll use a hybrid approach combining:

#### A. Supabase Realtime (Primary Method)

```
+----------------+     +----------------+     +---------------+     +----------------+
| Go Backend     |---->| Database       |---->| Supabase      |---->| UI Component   |
| Updates Status |     | Update (jobs)  |     | Realtime      |     | Updates Display|
+----------------+     +----------------+     +---------------+     +----------------+
```

1. Go backend processes job tasks and updates job status in PostgreSQL
2. Frontend subscribes to Supabase Realtime channels for specific jobs
3. When job data changes in the database, Supabase Realtime sends update to frontend
4. UI components update in real-time without polling

Subscription setup in JavaScript:
```javascript
// Subscribe to job updates
const jobSubscription = supabase
  .channel('job-updates')
  .on(
    'postgres_changes',
    { event: 'UPDATE', schema: 'public', table: 'jobs', filter: `id=eq.${jobId}` },
    (payload) => updateJobProgress(payload.new)
  )
  .subscribe()
```

#### B. Polling (Fallback Method)

```
+---------------+     +----------------+     +--------------+     +----------------+
| UI Sets       |---->| Periodic API   |---->| Go Backend  |---->| UI Updates     |
| Polling Timer |     | Requests       |     | Returns Data|     | Based on Data  |
+---------------+     +----------------+     +--------------+     +----------------+
```

1. For compatibility or when websockets aren't available, use polling as a fallback
2. JavaScript sets an interval to request job status every X seconds
3. Go backend responds with current job status
4. UI components update based on the response

### 5. Job Creation Flow

```
+----------------+     +----------------+     +----------------+     +----------------+
| User Fills Job |---->| JS Validates   |---->| API Request to |---->| Go Backend    |
| Creation Form  |     | Form Data      |     | Create Job     |     | Creates Job   |
+----------------+     +----------------+     +----------------+     +----------------+
        ^                                                                   |
        |                                                                   |
        +-------------------------------------------------------------------+
                               Redirect to Job Monitor
```

1. User fills out job creation form in UI
2. JavaScript validates form data client-side
3. On submission, JavaScript sends API request to Go backend
4. Go backend processes request, creates job in PostgreSQL
5. Go backend returns job ID and initial status
6. UI redirects to job monitoring page for the new job

## HTML/CSS Generation

HTML and CSS are managed through these approaches:

### 1. Static Structure (Webflow)

- **Marketing pages**: 100% Webflow-designed and rendered
- **Dashboard container**: Basic structure and layout in Webflow
- **Application placeholders**: Designated containers where JS components will render

### 2. Dynamic Content (JavaScript Components)

- **Web Components**: Create shadow DOM with encapsulated styles
- **HTML Generation**: JavaScript generates HTML structure within components
- **Styling**: Component-specific CSS encapsulated within shadow DOM
- **Responsive Layout**: Responsive design handled within components

Example component rendering:
```javascript
class JobCard extends HTMLElement {
  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
  }
  
  connectedCallback() {
    // Job data is passed as attributes or properties
    const { id, domain, status, progress } = this;
    
    // Generate HTML and CSS within shadow DOM
    this.shadowRoot.innerHTML = `
      <style>
        .job-card {
          border-radius: 8px;
          padding: 16px;
          margin-bottom: 16px;
          background: white;
          box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .progress-bar {
          height: 8px;
          background: #eee;
          border-radius: 4px;
          overflow: hidden;
        }
        .progress-fill {
          height: 100%;
          width: ${progress}%;
          background: #4CAF50;
          transition: width 0.3s ease;
        }
        /* More component-specific styles */
      </style>
      
      <div class="job-card">
        <h3>${domain}</h3>
        <p>Status: ${status}</p>
        <div class="progress-bar">
          <div class="progress-fill"></div>
        </div>
        <p>${progress}% complete</p>
        <button class="view-details" data-id="${id}">View Details</button>
      </div>
    `;
    
    // Add event listeners
    this.shadowRoot.querySelector('.view-details')
      .addEventListener('click', () => this.viewDetails());
  }
  
  viewDetails() {
    // Navigate to job details page
    window.location.href = `/dashboard/job/${this.id}`;
  }
  
  // Update component when job data changes
  updateProgress(progress) {
    this.progress = progress;
    const progressFill = this.shadowRoot.querySelector('.progress-fill');
    progressFill.style.width = `${progress}%`;
    this.shadowRoot.querySelector('p:last-of-type').textContent = `${progress}% complete`;
  }
}

customElements.define('job-card', JobCard);
```

## Data Flow Examples

### Dashboard Loading Sequence

1. User navigates to dashboard page in Webflow
2. JavaScript initializes and checks authentication
3. If authenticated, JavaScript requests job list from Go backend
4. Go backend validates JWT and queries database
5. Go backend returns job list as JSON
6. JavaScript renders job components for each job
7. JavaScript sets up Supabase Realtime subscriptions for active jobs
8. UI updates in real-time as job statuses change

### Creating a New Job

1. User navigates to job creation page
2. JavaScript initializes form components
3. User fills form and submits
4. JavaScript sends form data to Go backend
5. Go backend validates input and creates job in database
6. Go backend starts processing job (adding tasks to queue)
7. Go backend returns job ID and initial status
8. JavaScript redirects to monitoring page for new job
9. Job monitoring page sets up Realtime subscription
10. UI updates as job progress changes

## Technology Details

### Frontend (JavaScript/Webflow)

- **Custom Elements API**: For component encapsulation
- **Fetch API**: For REST API requests to backend
- **Supabase JavaScript Client**: For auth and Realtime
- **Shadow DOM**: For style encapsulation
- **localStorage**: For token storage

### Backend (Go)

- **HTTP API**: RESTful endpoints for all operations
- **JWT Validation**: Using Supabase JWT secret
- **PostgreSQL Driver**: Direct database access
- **Transaction Management**: For data consistency
- **Worker Pool**: For job processing

### Database (PostgreSQL/Supabase)

- **Supabase Authentication**: User management
- **PostgreSQL Tables**: Jobs, tasks, domains, etc.
- **Row-Level Security**: Data access control
- **Realtime Functionality**: Change notifications

## Pros & Cons of This Architecture

### Pros

1. **Clear Separation of Concerns**:
   - Frontend handles UI rendering and user interaction
   - Backend handles business logic and data processing
   - Database handles storage and access control

2. **Leverages Webflow's Strengths**:
   - Marketing pages built in Webflow
   - Visual design managed in Webflow
   - Content updates easy through Webflow CMS

3. **Real-time Updates**:
   - Efficient push-based updates via Supabase Realtime
   - Minimizes unnecessary polling
   - Better user experience with live updates

4. **Encapsulated Components**:
   - Shadow DOM prevents style conflicts with Webflow
   - Self-contained components are easier to maintain
   - Can be developed and tested independently

### Cons

1. **Complexity**:
   - Multiple systems need to work together
   - Debugging may involve multiple layers

2. **Dependency on Supabase Realtime**:
   - Need fallback for browsers with limited WebSocket support
   - Connection management adds complexity

3. **Shadow DOM Limitations**:
   - Some styling challenges with global themes
   - Potential performance impact with many components

4. **Authentication Flow Complexity**:
   - Needs careful implementation to handle token refresh
   - Security considerations for token storage

## Alternative Approaches Considered

1. **Direct Supabase Access from Frontend**:
   - Frontend could access Supabase directly for data
   - Eliminates need for Go backend API for some operations
   - Rejected due to security concerns and business logic needs

2. **WebSockets from Go Backend**:
   - Go backend could push updates directly via WebSockets
   - Provides more control over real-time messaging
   - Rejected in favor of Supabase Realtime to simplify infrastructure

3. **Full Webflow CMS Integration**:
   - Store job data in Webflow CMS
   - Use Webflow's built-in collection lists
   - Rejected due to limitations for complex data and real-time updates
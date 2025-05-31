/**
 * Job Dashboard Component for Blue Banded Bee
 * Provides comprehensive job summary with status overview cards and performance metrics
 */

import { BBBaseComponent } from '../utils/base-component.js';
import { api } from '../utils/api.js';
import { authManager, requireAuth } from '../auth/simple-auth.js';

export class BBJobDashboard extends BBBaseComponent {
  static get observedAttributes() {
    return ['auto-load', 'refresh-interval', 'show-charts', 'date-range', 'limit'];
  }

  constructor() {
    super();
    this.refreshTimer = null;
    this.dashboardData = null;
    this.charts = new Map();
  }

  connectedCallback() {
    super.connectedCallback();
    
    if (!requireAuth(this)) {
      return;
    }

    this.render();

    if (this.getBooleanAttribute('auto-load')) {
      this.loadDashboard();
    }

    this.setupRefreshTimer();
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this.clearRefreshTimer();
    this.destroyCharts();
  }

  handleAttributeChange(name, oldValue, newValue) {
    switch (name) {
      case 'date-range':
      case 'limit':
        if (this.getBooleanAttribute('auto-load')) {
          this.loadDashboard();
        }
        break;
      case 'refresh-interval':
        this.setupRefreshTimer();
        break;
    }
  }

  render() {
    this.innerHTML = `
      <div class="bb-dashboard">
        ${this.getLoadingHTML()}
        ${this.getErrorHTML()}
        
        <!-- Stats Overview Cards -->
        <div class="bb-stats-grid" data-bind-container="stats">
          ${this.getStatsLoadingHTML()}
        </div>

        <!-- Recent Jobs Section -->
        <div class="bb-jobs-section">
          <div class="bb-section-header">
            <h3>Recent Jobs</h3>
            <div class="bb-section-actions">
              <button class="bb-btn bb-btn-secondary" data-action="refresh">
                <span class="bb-icon">↻</span> Refresh
              </button>
              <button class="bb-btn bb-btn-primary" data-action="create-job">
                <span class="bb-icon">+</span> New Job
              </button>
            </div>
          </div>
          
          <div class="bb-jobs-list" data-bind-container="jobs">
            ${this.getJobsLoadingHTML()}
          </div>
        </div>

        <!-- Performance Chart (if enabled) -->
        ${this.getBooleanAttribute('show-charts') ? this.getChartsHTML() : ''}
      </div>

      <!-- Job Card Template -->
      <template class="bb-job-card-template">
        <div class="bb-job-card" data-job-id="{id}">
          <div class="bb-job-header">
            <div class="bb-job-domain" data-bind="domains.name">{domains.name}</div>
            <div class="bb-job-status bb-status-{status}" data-bind="status">{status}</div>
          </div>
          
          <div class="bb-job-progress">
            <div class="bb-progress-bar">
              <div class="bb-progress-fill" data-style-bind="width:{progress}%"></div>
            </div>
            <div class="bb-progress-text">
              <span data-bind="completed_tasks">{completed_tasks}</span> / 
              <span data-bind="total_tasks">{total_tasks}</span> tasks
              (<span data-bind="progress">{progress}</span>%)
            </div>
          </div>

          <div class="bb-job-meta">
            <div class="bb-job-time">
              <span class="bb-label">Started:</span>
              <span data-bind="started_at|formatDate">{started_at}</span>
            </div>
            <div class="bb-job-actions">
              <button class="bb-btn-link" data-link="job-details" data-job-id="{id}">
                View Details
              </button>
              ${this.getJobActionButtons()}
            </div>
          </div>
        </div>
      </template>

      <!-- Stats Card Template -->
      <template class="bb-stats-card-template">
        <div class="bb-stat-card bb-stat-{type}">
          <div class="bb-stat-value" data-bind="value">{value}</div>
          <div class="bb-stat-label" data-bind="label">{label}</div>
          <div class="bb-stat-trend" data-bind="trend" data-show-if="trend">
            <span class="bb-trend-{trend.direction}" data-bind="trend.text">{trend.text}</span>
          </div>
        </div>
      </template>

      ${this.getDashboardStyles()}
    `;

    this.setupEventHandlers();
  }

  getJobActionButtons() {
    return `
      <button class="bb-btn-link bb-btn-cancel" data-link="cancel-job" data-job-id="{id}" data-show-if="status=running">
        Cancel
      </button>
      <button class="bb-btn-link bb-btn-retry" data-link="retry-job" data-job-id="{id}" data-show-if="status=failed">
        Retry
      </button>
    `;
  }

  getChartsHTML() {
    return `
      <div class="bb-charts-section">
        <div class="bb-chart-container">
          <h4>Job Activity</h4>
          <canvas class="bb-activity-chart" width="400" height="200"></canvas>
        </div>
      </div>
    `;
  }

  getStatsLoadingHTML() {
    return `
      <div class="bb-loading-stats">
        <div class="bb-skeleton bb-skeleton-card"></div>
        <div class="bb-skeleton bb-skeleton-card"></div>
        <div class="bb-skeleton bb-skeleton-card"></div>
        <div class="bb-skeleton bb-skeleton-card"></div>
      </div>
    `;
  }

  getJobsLoadingHTML() {
    return `
      <div class="bb-loading-jobs">
        <div class="bb-skeleton bb-skeleton-job"></div>
        <div class="bb-skeleton bb-skeleton-job"></div>
        <div class="bb-skeleton bb-skeleton-job"></div>
      </div>
    `;
  }

  setupEventHandlers() {
    // Action button handlers
    this.addEventListener('click', (e) => {
      const action = e.target.dataset.action;
      if (action) {
        this.handleAction(action, e.target);
      }

      const link = e.target.dataset.link;
      if (link) {
        e.preventDefault();
        this.handleLinkClick(link, e.target);
      }
    });
  }

  handleAction(action, element) {
    switch (action) {
      case 'refresh':
        this.loadDashboard();
        break;
      case 'create-job':
        this.dispatchCustomEvent('create-job-requested');
        break;
    }
  }

  handleLinkClick(linkType, element) {
    const jobId = element.dataset.jobId;
    
    switch (linkType) {
      case 'job-details':
        this.dispatchCustomEvent('job-details-requested', { jobId });
        break;
      case 'cancel-job':
        this.cancelJob(jobId);
        break;
      case 'retry-job':
        this.retryJob(jobId);
        break;
    }
  }

  async loadDashboard() {
    this.setLoading('dashboard', true);
    this.setError('dashboard', null);

    try {
      // Load dashboard data in parallel
      const [statsData, jobsData] = await Promise.all([
        this.loadStats(),
        this.loadJobs()
      ]);

      this.dashboardData = {
        stats: statsData,
        jobs: jobsData
      };

      this.updateStats(statsData);
      this.updateJobs(jobsData);

      if (this.getBooleanAttribute('show-charts')) {
        await this.updateCharts();
      }

      this.dispatchCustomEvent('dashboard-loaded', { data: this.dashboardData });

    } catch (error) {
      this.setError('dashboard', error);
      this.dispatchCustomEvent('dashboard-error', { error });
    } finally {
      this.setLoading('dashboard', false);
    }
  }

  async loadStats() {
    const dateRange = this.getAttribute('date-range') || 'last7';
    const response = await api.get(`/v1/dashboard/stats?range=${dateRange}`);
    return response.data;
  }

  async loadJobs() {
    const limit = this.getNumberAttribute('limit') || 10;
    const dateRange = this.getAttribute('date-range') || 'last7';
    const response = await api.get(`/v1/jobs?limit=${limit}&range=${dateRange}&include=domain,progress`);
    return response.data;
  }

  updateStats(statsData) {
    const container = this.querySelector('[data-bind-container="stats"]');
    if (!container) return;

    // Clear loading state
    const loading = container.querySelector('.bb-loading-stats');
    if (loading) loading.style.display = 'none';

    // Prepare stats for template population
    const stats = [
      { type: 'total', value: statsData.total_jobs || 0, label: 'Total Jobs', trend: statsData.total_trend },
      { type: 'running', value: statsData.running_jobs || 0, label: 'Running', trend: statsData.running_trend },
      { type: 'completed', value: statsData.completed_jobs || 0, label: 'Completed', trend: statsData.completed_trend },
      { type: 'failed', value: statsData.failed_jobs || 0, label: 'Failed', trend: statsData.failed_trend }
    ];

    // Clear existing cards
    const existingCards = container.querySelectorAll('.bb-stat-card');
    existingCards.forEach(card => card.remove());

    // Populate stats cards
    stats.forEach(stat => {
      this.populateTemplate('.bb-stats-card-template', stat, '[data-bind-container="stats"]');
    });
  }

  updateJobs(jobsData) {
    const container = this.querySelector('[data-bind-container="jobs"]');
    if (!container) return;

    // Clear loading state
    const loading = container.querySelector('.bb-loading-jobs');
    if (loading) loading.style.display = 'none';

    // Clear existing jobs
    const existingJobs = container.querySelectorAll('.bb-job-card');
    existingJobs.forEach(job => job.remove());

    // Handle empty state
    if (!jobsData.jobs || jobsData.jobs.length === 0) {
      container.innerHTML = `
        <div class="bb-empty-state">
          <div class="bb-empty-icon">📋</div>
          <h4>No Jobs Found</h4>
          <p>Get started by creating your first cache warming job.</p>
          <button class="bb-btn bb-btn-primary" data-action="create-job">
            <span class="bb-icon">+</span> Create First Job
          </button>
        </div>
      `;
      return;
    }

    // Populate job cards
    jobsData.jobs.forEach(job => {
      // Format job data for template
      const formattedJob = {
        ...job,
        progress: Math.round(job.progress || 0),
        started_at: this.formatDate(job.started_at),
        completed_at: this.formatDate(job.completed_at)
      };

      this.populateTemplate('.bb-job-card-template', formattedJob, '[data-bind-container="jobs"]');
    });
  }

  async updateCharts() {
    if (!this.getBooleanAttribute('show-charts')) return;

    try {
      const chartData = await api.get('/v1/dashboard/activity');
      this.renderActivityChart(chartData.data);
    } catch (error) {
      console.warn('Failed to load chart data:', error);
    }
  }

  renderActivityChart(data) {
    const canvas = this.querySelector('.bb-activity-chart');
    if (!canvas) return;

    // Basic chart implementation (could be enhanced with Chart.js)
    const ctx = canvas.getContext('2d');
    // Simple bar chart implementation here...
  }

  async cancelJob(jobId) {
    if (!confirm('Are you sure you want to cancel this job?')) return;

    try {
      this.setLoading('cancel', true);
      await api.post(`/v1/jobs/${jobId}/cancel`);
      this.loadDashboard(); // Refresh data
      this.dispatchCustomEvent('job-cancelled', { jobId });
    } catch (error) {
      this.setError('cancel', error);
    } finally {
      this.setLoading('cancel', false);
    }
  }

  async retryJob(jobId) {
    try {
      this.setLoading('retry', true);
      await api.post(`/v1/jobs/${jobId}/retry`);
      this.loadDashboard(); // Refresh data
      this.dispatchCustomEvent('job-retried', { jobId });
    } catch (error) {
      this.setError('retry', error);
    } finally {
      this.setLoading('retry', false);
    }
  }

  formatDate(dateStr) {
    if (!dateStr) return '-';
    const date = new Date(dateStr + (dateStr.includes('Z') ? '' : 'Z'));
    return date.toLocaleString('en-AU', {
      day: '2-digit',
      month: '2-digit', 
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  }

  setupRefreshTimer() {
    this.clearRefreshTimer();
    
    const interval = this.getNumberAttribute('refresh-interval');
    if (interval > 0) {
      this.refreshTimer = setInterval(() => {
        if (this._isComponentConnected && !this.isLoading()) {
          this.loadDashboard();
        }
      }, interval * 1000);
    }
  }

  clearRefreshTimer() {
    if (this.refreshTimer) {
      clearInterval(this.refreshTimer);
      this.refreshTimer = null;
    }
  }

  destroyCharts() {
    this.charts.forEach(chart => {
      if (chart && chart.destroy) {
        chart.destroy();
      }
    });
    this.charts.clear();
  }

  getDashboardStyles() {
    return `
      <style>
        .bb-dashboard {
          max-width: 1200px;
          margin: 0 auto;
          padding: 20px;
          font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
        }

        .bb-stats-grid {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
          gap: 20px;
          margin-bottom: 30px;
        }

        .bb-stat-card {
          background: white;
          padding: 24px;
          border-radius: 12px;
          box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
          text-align: center;
          border: 1px solid #f0f0f0;
        }

        .bb-stat-value {
          font-size: 2.5em;
          font-weight: 700;
          margin-bottom: 8px;
          color: #1a1a1a;
        }

        .bb-stat-label {
          color: #666;
          font-size: 14px;
          font-weight: 500;
          text-transform: uppercase;
          letter-spacing: 0.5px;
        }

        .bb-stat-trend {
          margin-top: 8px;
          font-size: 12px;
        }

        .bb-trend-up { color: #22c55e; }
        .bb-trend-down { color: #ef4444; }
        .bb-trend-stable { color: #6b7280; }

        .bb-stat-running .bb-stat-value { color: #3b82f6; }
        .bb-stat-completed .bb-stat-value { color: #22c55e; }
        .bb-stat-failed .bb-stat-value { color: #ef4444; }

        .bb-jobs-section {
          background: white;
          border-radius: 12px;
          box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
          overflow: hidden;
          margin-bottom: 30px;
        }

        .bb-section-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          padding: 24px 24px 0 24px;
          margin-bottom: 20px;
        }

        .bb-section-header h3 {
          margin: 0;
          font-size: 1.25em;
          font-weight: 600;
        }

        .bb-section-actions {
          display: flex;
          gap: 12px;
        }

        .bb-jobs-list {
          padding: 0 24px 24px 24px;
        }

        .bb-job-card {
          background: #f8f9fa;
          border: 1px solid #e9ecef;
          border-radius: 8px;
          padding: 20px;
          margin-bottom: 16px;
          transition: all 0.2s ease;
        }

        .bb-job-card:hover {
          box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
          transform: translateY(-1px);
        }

        .bb-job-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          margin-bottom: 16px;
        }

        .bb-job-domain {
          font-weight: 600;
          font-size: 1.1em;
          color: #1a1a1a;
        }

        .bb-job-status {
          padding: 6px 12px;
          border-radius: 20px;
          font-size: 12px;
          font-weight: 600;
          text-transform: uppercase;
          letter-spacing: 0.5px;
        }

        .bb-status-running { background: #dbeafe; color: #1d4ed8; }
        .bb-status-completed { background: #dcfce7; color: #16a34a; }
        .bb-status-failed { background: #fee2e2; color: #dc2626; }
        .bb-status-pending { background: #fef3c7; color: #d97706; }

        .bb-job-progress {
          margin-bottom: 16px;
        }

        .bb-progress-bar {
          width: 100%;
          height: 8px;
          background: #e5e7eb;
          border-radius: 4px;
          overflow: hidden;
          margin-bottom: 8px;
        }

        .bb-progress-fill {
          height: 100%;
          background: linear-gradient(90deg, #3b82f6, #1d4ed8);
          transition: width 0.3s ease;
        }

        .bb-progress-text {
          font-size: 14px;
          color: #6b7280;
        }

        .bb-job-meta {
          display: flex;
          justify-content: space-between;
          align-items: center;
          font-size: 14px;
        }

        .bb-job-time {
          color: #6b7280;
        }

        .bb-label {
          font-weight: 500;
        }

        .bb-job-actions {
          display: flex;
          gap: 12px;
        }

        .bb-btn {
          padding: 8px 16px;
          border-radius: 6px;
          border: none;
          font-weight: 500;
          cursor: pointer;
          transition: all 0.2s ease;
          display: inline-flex;
          align-items: center;
          gap: 6px;
          text-decoration: none;
        }

        .bb-btn-primary {
          background: #3b82f6;
          color: white;
        }

        .bb-btn-primary:hover {
          background: #2563eb;
        }

        .bb-btn-secondary {
          background: #f3f4f6;
          color: #374151;
        }

        .bb-btn-secondary:hover {
          background: #e5e7eb;
        }

        .bb-btn-link {
          background: none;
          border: none;
          color: #3b82f6;
          padding: 0;
          font-size: 14px;
          cursor: pointer;
        }

        .bb-btn-link:hover {
          color: #2563eb;
          text-decoration: underline;
        }

        .bb-btn-cancel { color: #ef4444; }
        .bb-btn-cancel:hover { color: #dc2626; }

        .bb-empty-state {
          text-align: center;
          padding: 60px 20px;
          color: #6b7280;
        }

        .bb-empty-icon {
          font-size: 3em;
          margin-bottom: 16px;
        }

        .bb-empty-state h4 {
          margin: 0 0 8px 0;
          color: #374151;
        }

        .bb-empty-state p {
          margin: 0 0 24px 0;
        }

        .bb-skeleton {
          background: linear-gradient(90deg, #f0f0f0 25%, #e0e0e0 50%, #f0f0f0 75%);
          background-size: 200% 100%;
          animation: bb-skeleton-loading 1.5s infinite;
          border-radius: 6px;
        }

        .bb-skeleton-card {
          height: 100px;
          margin-bottom: 16px;
        }

        .bb-skeleton-job {
          height: 120px;
          margin-bottom: 16px;
        }

        @keyframes bb-skeleton-loading {
          0% { background-position: 200% 0; }
          100% { background-position: -200% 0; }
        }

        .bb-charts-section {
          background: white;
          border-radius: 12px;
          box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
          padding: 24px;
        }

        .bb-chart-container h4 {
          margin: 0 0 20px 0;
          font-size: 1.1em;
          font-weight: 600;
        }

        @media (max-width: 768px) {
          .bb-dashboard {
            padding: 16px;
          }

          .bb-stats-grid {
            grid-template-columns: repeat(2, 1fr);
            gap: 16px;
          }

          .bb-section-header {
            flex-direction: column;
            align-items: flex-start;
            gap: 16px;
          }

          .bb-job-meta {
            flex-direction: column;
            align-items: flex-start;
            gap: 12px;
          }
        }
      </style>
    `;
  }

  // Public API
  refresh() {
    this.loadDashboard();
  }

  getDashboardData() {
    return this.dashboardData;
  }
}

customElements.define('bb-job-dashboard', BBJobDashboard);
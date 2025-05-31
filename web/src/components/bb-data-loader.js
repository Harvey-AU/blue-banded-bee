/**
 * Core data loading component for Blue Banded Bee
 * Fetches data from API and populates Webflow templates
 */

import { BBBaseComponent } from '../utils/base-component.js';
import { api } from '../utils/api.js';
import { authManager, requireAuth } from '../auth/simple-auth.js';

export class BBDataLoader extends BBBaseComponent {
  static get observedAttributes() {
    return ['endpoint', 'template', 'target', 'auto-load', 'require-auth', 'refresh-interval'];
  }

  constructor() {
    super();
    this.refreshTimer = null;
    this.data = null;
  }

  connectedCallback() {
    super.connectedCallback();
    
    if (this.getBooleanAttribute('require-auth') && !requireAuth(this)) {
      return;
    }

    if (this.getBooleanAttribute('auto-load')) {
      this.loadData();
    }

    this.setupRefreshTimer();
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this.clearRefreshTimer();
  }

  handleAttributeChange(name, oldValue, newValue) {
    switch (name) {
      case 'endpoint':
        if (this.getBooleanAttribute('auto-load')) {
          this.loadData();
        }
        break;
      case 'refresh-interval':
        this.setupRefreshTimer();
        break;
    }
  }

  render() {
    if (!this.innerHTML.trim()) {
      this.innerHTML = `
        ${this.getLoadingHTML()}
        ${this.getErrorHTML()}
      `;
    }
  }

  async loadData() {
    const endpoint = this.getAttribute('endpoint');
    if (!endpoint) {
      this.setError('endpoint', new Error('No endpoint specified'));
      return;
    }

    this.setLoading('data', true);
    this.setError('endpoint', null);

    try {
      const response = await api.get(endpoint);
      this.data = response.data;
      
      this.populateTemplates();
      this.dispatchCustomEvent('data-loaded', { data: this.data, endpoint });
      
    } catch (error) {
      this.setError('endpoint', error);
      this.dispatchCustomEvent('data-error', { error, endpoint });
    } finally {
      this.setLoading('data', false);
    }
  }

  populateTemplates() {
    const templateSelector = this.getAttribute('template');
    const targetSelector = this.getAttribute('target');
    
    if (!templateSelector || !this.data) {
      return;
    }

    // Clear existing data (keep template)
    if (targetSelector) {
      const target = document.querySelector(targetSelector);
      if (target) {
        const existingItems = target.querySelectorAll(':not(.template)');
        existingItems.forEach(item => item.remove());
      }
    }

    // Handle array data (common case)
    if (Array.isArray(this.data)) {
      this.data.forEach(item => {
        this.populateTemplate(templateSelector, item, targetSelector);
      });
    } 
    // Handle object data
    else if (typeof this.data === 'object') {
      this.populateTemplate(templateSelector, this.data, targetSelector);
    }
  }

  // Enhanced template population with event handling
  populateTemplate(templateSelector, data, targetSelector = null) {
    const element = super.populateTemplate(templateSelector, data, targetSelector);
    
    if (element) {
      // Add click handlers for links
      const links = element.querySelectorAll('[data-link]');
      links.forEach(link => {
        link.addEventListener('click', (e) => {
          e.preventDefault();
          const linkType = link.dataset.link;
          this.handleLinkClick(linkType, data, link);
        });
      });

      // Add form handlers
      const forms = element.querySelectorAll('[data-form]');
      forms.forEach(form => {
        form.addEventListener('submit', (e) => {
          e.preventDefault();
          const formType = form.dataset.form;
          this.handleFormSubmit(formType, data, form);
        });
      });
    }

    return element;
  }

  handleLinkClick(linkType, data, element) {
    switch (linkType) {
      case 'job-details':
        window.location.href = `/jobs?id=${data.id}`;
        break;
      case 'cancel-job':
        this.cancelJob(data.id);
        break;
      default:
        this.dispatchCustomEvent('link-click', { linkType, data, element });
    }
  }

  handleFormSubmit(formType, data, form) {
    this.dispatchCustomEvent('form-submit', { formType, data, form });
  }

  // Job-specific actions
  async cancelJob(jobId) {
    try {
      this.setLoading('cancel', true);
      await api.post(`/v1/jobs/${jobId}/cancel`);
      this.loadData(); // Refresh data
      this.dispatchCustomEvent('job-cancelled', { jobId });
    } catch (error) {
      this.setError('cancel', error);
    } finally {
      this.setLoading('cancel', false);
    }
  }

  // Refresh timer management
  setupRefreshTimer() {
    this.clearRefreshTimer();
    
    const interval = this.getNumberAttribute('refresh-interval');
    if (interval > 0) {
      this.refreshTimer = setInterval(() => {
        if (this._isComponentConnected && !this.isLoading()) {
          this.loadData();
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

  // Public API
  refresh() {
    this.loadData();
  }

  getData() {
    return this.data;
  }
}

customElements.define('bb-data-loader', BBDataLoader);
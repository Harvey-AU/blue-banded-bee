(function () {
  'use strict';

  /**
   * API utility for Blue Banded Bee components
   */

  const API_BASE = window.location.hostname === 'localhost' 
    ? 'http://localhost:8080'
    : 'https://blue-banded-bee.fly.dev';

  class BBApi {
    constructor() {
      this.baseUrl = API_BASE;
      this.token = null;
    }

    setToken(token) {
      this.token = token;
    }

    async request(endpoint, options = {}) {
      const url = `${this.baseUrl}${endpoint}`;
      const headers = {
        'Content-Type': 'application/json',
        ...options.headers
      };

      if (this.token) {
        headers.Authorization = `Bearer ${this.token}`;
      }

      const config = {
        ...options,
        headers
      };

      try {
        const response = await fetch(url, config);
        const data = await response.json();

        if (!response.ok) {
          throw new Error(data.error?.message || `HTTP ${response.status}`);
        }

        return data;
      } catch (error) {
        console.error('API request failed:', error);
        throw error;
      }
    }

    async get(endpoint) {
      return this.request(endpoint, { method: 'GET' });
    }

    async post(endpoint, body) {
      return this.request(endpoint, {
        method: 'POST',
        body: JSON.stringify(body)
      });
    }

    async put(endpoint, body) {
      return this.request(endpoint, {
        method: 'PUT',
        body: JSON.stringify(body)
      });
    }

    async delete(endpoint) {
      return this.request(endpoint, { method: 'DELETE' });
    }

    // Specific API methods
    async getJobs(params = {}) {
      const query = new URLSearchParams(params).toString();
      const endpoint = query ? `/v1/jobs?${query}` : '/v1/jobs';
      return this.get(endpoint);
    }

    async getJob(jobId) {
      return this.get(`/v1/jobs/${jobId}`);
    }

    async createJob(jobData) {
      return this.post('/v1/jobs', jobData);
    }

    async getJobTasks(jobId, params = {}) {
      const query = new URLSearchParams(params).toString();
      const endpoint = query ? `/v1/jobs/${jobId}/tasks?${query}` : `/v1/jobs/${jobId}/tasks`;
      return this.get(endpoint);
    }

    async getUserProfile() {
      return this.get('/v1/auth/profile');
    }
  }

  const api = new BBApi();

  var api$1 = /*#__PURE__*/Object.freeze({
    __proto__: null,
    api: api
  });

  /**
   * Base class for Blue Banded Bee Web Components
   */

  class BBBaseComponent extends HTMLElement {
    constructor() {
      super();
      this._isComponentConnected = false;
      this.loadingStates = new Map();
      this.errorStates = new Map();
    }

    connectedCallback() {
      this._isComponentConnected = true;
      this.render();
      this.setupEventListeners();
    }

    disconnectedCallback() {
      this._isComponentConnected = false;
      this.cleanup();
    }

    attributeChangedCallback(name, oldValue, newValue) {
      if (oldValue !== newValue && this._isComponentConnected) {
        this.handleAttributeChange(name, oldValue, newValue);
      }
    }

    // Override in subclasses
    render() {
      // Default implementation
    }

    setupEventListeners() {
      // Override in subclasses
    }

    cleanup() {
      // Override in subclasses for cleanup
    }

    handleAttributeChange(name, oldValue, newValue) {
      // Override in subclasses
    }

    // Utility methods
    getAttribute(name, defaultValue = null) {
      return super.getAttribute(name) || defaultValue;
    }

    getBooleanAttribute(name) {
      return this.hasAttribute(name) && this.getAttribute(name) !== 'false';
    }

    getNumberAttribute(name, defaultValue = 0) {
      const value = this.getAttribute(name);
      return value ? parseInt(value, 10) : defaultValue;
    }

    // Loading state management
    setLoading(key, isLoading) {
      this.loadingStates.set(key, isLoading);
      this.updateLoadingState();
    }

    isLoading(key = null) {
      if (key) {
        return this.loadingStates.get(key) || false;
      }
      return Array.from(this.loadingStates.values()).some(state => state);
    }

    updateLoadingState() {
      const isLoading = this.isLoading();
      this.classList.toggle('bb-loading', isLoading);
      
      // Update loading indicators
      const loadingElements = this.querySelectorAll('.bb-loading-indicator');
      loadingElements.forEach(el => {
        el.style.display = isLoading ? 'block' : 'none';
      });
    }

    // Error state management
    setError(key, error) {
      if (error) {
        this.errorStates.set(key, error);
      } else {
        this.errorStates.delete(key);
      }
      this.updateErrorState();
    }

    hasError(key = null) {
      if (key) {
        return this.errorStates.has(key);
      }
      return this.errorStates.size > 0;
    }

    updateErrorState() {
      const hasError = this.hasError();
      this.classList.toggle('bb-error', hasError);
      
      // Update error displays
      const errorElements = this.querySelectorAll('.bb-error-message');
      if (hasError) {
        const firstError = Array.from(this.errorStates.values())[0];
        errorElements.forEach(el => {
          el.textContent = firstError.message || firstError.toString();
          el.style.display = 'block';
        });
      } else {
        errorElements.forEach(el => {
          el.style.display = 'none';
        });
      }
    }

    // Template and data binding utilities
    populateTemplate(templateSelector, data, targetSelector = null) {
      const template = this.querySelector(templateSelector);
      const target = targetSelector ? this.querySelector(targetSelector) : this;
      
      if (!template) {
        console.warn(`Template not found: ${templateSelector}`);
        return null;
      }

      const clone = template.cloneNode(true);
      clone.classList.remove('template');
      clone.style.display = '';

      // Replace data placeholders
      this.bindDataToElement(clone, data);

      if (target && target !== this) {
        target.appendChild(clone);
      }

      return clone;
    }

    bindDataToElement(element, data) {
      // Find elements with data attributes and populate them
      const bindableElements = element.querySelectorAll('[data-bind]');
      
      bindableElements.forEach(el => {
        const bindPath = el.dataset.bind;
        const value = this.getNestedValue(data, bindPath);
        
        if (value !== undefined) {
          if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA') {
            el.value = value;
          } else {
            el.textContent = value;
          }
        }
      });

      // Handle style bindings (e.g., width for progress bars)
      const styleBindings = element.querySelectorAll('[data-style-bind]');
      styleBindings.forEach(el => {
        const bindings = el.dataset.styleBind.split(',');
        bindings.forEach(binding => {
          const [property, path] = binding.split(':');
          const value = this.getNestedValue(data, path.trim());
          if (value !== undefined) {
            el.style[property.trim()] = value;
          }
        });
      });
    }

    getNestedValue(obj, path) {
      return path.split('.').reduce((current, key) => current?.[key], obj);
    }

    // Event handling utilities
    dispatchCustomEvent(eventName, detail = {}) {
      const event = new CustomEvent(`bb:${eventName}`, {
        detail,
        bubbles: true,
        cancelable: true
      });
      this.dispatchEvent(event);
    }

    // Common loading/error HTML
    getLoadingHTML() {
      return `<div class="bb-loading-indicator" style="display: none;">Loading...</div>`;
    }

    getErrorHTML() {
      return `<div class="bb-error-message" style="display: none;"></div>`;
    }
  }

  /**
   * Simple authentication manager without external dependencies
   * Expects Supabase to be loaded via CDN
   */


  class SimpleAuthManager {
    constructor() {
      this.user = null;
      this.session = null;
      this.listeners = new Set();
      this.supabase = null;
      this.init();
    }

    async init() {
      // Wait for Supabase to be available on window
      await this.waitForSupabase();
      
      // Get initial session
      const { data: { session } } = await this.supabase.auth.getSession();
      this.setSession(session);

      // Listen for auth changes
      this.supabase.auth.onAuthStateChange((event, session) => {
        this.setSession(session);
        this.notifyListeners(event, session);
      });
    }

    async waitForSupabase() {
      // Check if Supabase is already loaded
      if (window.supabase) {
        this.supabase = window.supabase;
        return;
      }

      // Wait for it to load
      return new Promise((resolve) => {
        const checkSupabase = () => {
          if (window.supabase) {
            this.supabase = window.supabase;
            resolve();
          } else {
            setTimeout(checkSupabase, 100);
          }
        };
        checkSupabase();
      });
    }

    setSession(session) {
      this.session = session;
      this.user = session?.user || null;
      
      // Update API token
      if (session?.access_token) {
        api.setToken(session.access_token);
      } else {
        api.setToken(null);
      }
    }

    // Authentication methods
    async signIn(email, password) {
      if (!this.supabase) await this.waitForSupabase();
      
      const { data, error } = await this.supabase.auth.signInWithPassword({
        email,
        password
      });
      
      if (error) throw error;
      return data;
    }

    async signUp(email, password, metadata = {}) {
      if (!this.supabase) await this.waitForSupabase();
      
      const { data, error } = await this.supabase.auth.signUp({
        email,
        password,
        options: {
          data: metadata
        }
      });
      
      if (error) throw error;
      return data;
    }

    async signInWithProvider(provider) {
      if (!this.supabase) await this.waitForSupabase();
      
      const { data, error } = await this.supabase.auth.signInWithOAuth({
        provider,
        options: {
          redirectTo: window.location.origin + '/dashboard'
        }
      });
      
      if (error) throw error;
      return data;
    }

    async signOut() {
      if (!this.supabase) await this.waitForSupabase();
      
      const { error } = await this.supabase.auth.signOut();
      if (error) throw error;
    }

    async resetPassword(email) {
      if (!this.supabase) await this.waitForSupabase();
      
      const { data, error } = await this.supabase.auth.resetPasswordForEmail(email, {
        redirectTo: window.location.origin + '/reset-password'
      });
      
      if (error) throw error;
      return data;
    }

    // State checking
    isAuthenticated() {
      return !!this.session;
    }

    getUser() {
      return this.user;
    }

    getSession() {
      return this.session;
    }

    getToken() {
      return this.session?.access_token || null;
    }

    // Event handling
    onAuthStateChange(callback) {
      this.listeners.add(callback);
      
      // Return unsubscribe function
      return () => {
        this.listeners.delete(callback);
      };
    }

    notifyListeners(event, session) {
      this.listeners.forEach(callback => {
        try {
          callback(event, session);
        } catch (error) {
          console.error('Auth listener error:', error);
        }
      });
    }

    // User profile management
    async getUserProfile() {
      if (!this.isAuthenticated()) {
        throw new Error('User not authenticated');
      }
      
      try {
        return await api.getUserProfile();
      } catch (error) {
        console.error('Failed to get user profile:', error);
        throw error;
      }
    }

    async updateProfile(updates) {
      if (!this.supabase) await this.waitForSupabase();
      
      const { data, error } = await this.supabase.auth.updateUser(updates);
      if (error) throw error;
      return data;
    }
  }

  // Create singleton instance
  const authManager = new SimpleAuthManager();

  // Utility function for components
  function requireAuth(component) {
    if (!authManager.isAuthenticated()) {
      component.innerHTML = `
      <div class="bb-auth-required">
        <p>Please log in to access this feature.</p>
        <bb-auth-login></bb-auth-login>
      </div>
    `;
      return false;
    }
    return true;
  }

  var simpleAuth = /*#__PURE__*/Object.freeze({
    __proto__: null,
    authManager: authManager,
    requireAuth: requireAuth
  });

  /**
   * Core data loading component for Blue Banded Bee
   * Fetches data from API and populates Webflow templates
   */


  class BBDataLoader extends BBBaseComponent {
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

  /**
   * Login component for Blue Banded Bee
   */


  class BBAuthLogin extends BBBaseComponent {
    static get observedAttributes() {
      return ['redirect-url', 'show-providers', 'compact'];
    }

    constructor() {
      super();
      this.unsubscribeAuth = null;
    }

    connectedCallback() {
      super.connectedCallback();
      
      // Listen for auth state changes
      this.unsubscribeAuth = authManager.onAuthStateChange((event, session) => {
        if (session && this.hasAttribute('redirect-url')) {
          this.handleSuccessfulLogin();
        }
      });

      // Check if already logged in - only redirect if redirect-url is explicitly set
      if (authManager.isAuthenticated() && this.hasAttribute('redirect-url')) {
        this.handleSuccessfulLogin();
      }
    }

    disconnectedCallback() {
      super.disconnectedCallback();
      if (this.unsubscribeAuth) {
        this.unsubscribeAuth();
      }
    }

    render() {
      const compact = this.getBooleanAttribute('compact');
      const showProviders = this.getBooleanAttribute('show-providers');
      
      this.innerHTML = `
      <div class="bb-auth-login ${compact ? 'compact' : ''}">
        ${this.getErrorHTML()}
        ${this.getLoadingHTML()}
        
        <form class="bb-login-form" data-form="login">
          <div class="form-group">
            <label for="bb-email">Email</label>
            <input 
              type="email" 
              id="bb-email" 
              name="email" 
              required
              placeholder="Enter your email"
            >
          </div>
          
          <div class="form-group">
            <label for="bb-password">Password</label>
            <input 
              type="password" 
              id="bb-password" 
              name="password" 
              required
              placeholder="Enter your password"
            >
          </div>
          
          <button type="submit" class="bb-btn bb-btn-primary">
            Sign In
          </button>
          
          <div class="bb-auth-links">
            <a href="#" data-action="forgot-password">Forgot password?</a>
            <a href="#" data-action="show-signup">Need an account? Sign up</a>
          </div>
        </form>
        
        ${showProviders ? this.renderSocialProviders() : ''}
      </div>
    `;
    }

    renderSocialProviders() {
      return `
      <div class="bb-social-login">
        <div class="bb-divider">
          <span>or continue with</span>
        </div>
        
        <div class="bb-social-buttons">
          <button type="button" class="bb-btn bb-btn-social" data-provider="google">
            <span class="bb-social-icon">G</span>
            Google
          </button>
          
          <button type="button" class="bb-btn bb-btn-social" data-provider="github">
            <span class="bb-social-icon">GH</span>
            GitHub
          </button>
          
          <button type="button" class="bb-btn bb-btn-social" data-provider="slack">
            <span class="bb-social-icon">S</span>
            Slack
          </button>
        </div>
      </div>
    `;
    }

    setupEventListeners() {
      // Form submission
      const form = this.querySelector('.bb-login-form');
      if (form) {
        form.addEventListener('submit', (e) => this.handleLogin(e));
      }

      // Social login buttons
      const socialButtons = this.querySelectorAll('[data-provider]');
      socialButtons.forEach(button => {
        button.addEventListener('click', (e) => {
          const provider = e.currentTarget.dataset.provider;
          this.handleSocialLogin(provider);
        });
      });

      // Action links
      const actionLinks = this.querySelectorAll('[data-action]');
      actionLinks.forEach(link => {
        link.addEventListener('click', (e) => {
          e.preventDefault();
          const action = e.currentTarget.dataset.action;
          this.handleAction(action);
        });
      });
    }

    async handleLogin(event) {
      event.preventDefault();
      
      const formData = new FormData(event.target);
      const email = formData.get('email');
      const password = formData.get('password');

      this.setLoading('login', true);
      this.setError('login', null);

      try {
        await authManager.signIn(email, password);
        // Success is handled by auth state change listener
      } catch (error) {
        this.setError('login', error);
      } finally {
        this.setLoading('login', false);
      }
    }

    async handleSocialLogin(provider) {
      this.setLoading('social', true);
      this.setError('social', null);

      try {
        await authManager.signInWithProvider(provider);
        // Success is handled by auth state change listener
      } catch (error) {
        this.setError('social', error);
        this.setLoading('social', false);
      }
    }

    handleAction(action) {
      switch (action) {
        case 'forgot-password':
          this.showForgotPassword();
          break;
        case 'show-signup':
          this.dispatchCustomEvent('show-signup');
          break;
      }
    }

    showForgotPassword() {
      const email = this.querySelector('#bb-email').value;
      
      if (email) {
        this.sendPasswordReset(email);
      } else {
        alert('Please enter your email address first.');
      }
    }

    async sendPasswordReset(email) {
      this.setLoading('reset', true);

      try {
        await authManager.resetPassword(email);
        alert('Password reset email sent! Check your inbox.');
      } catch (error) {
        this.setError('reset', error);
      } finally {
        this.setLoading('reset', false);
      }
    }

    handleSuccessfulLogin() {
      const redirectUrl = this.getAttribute('redirect-url');
      
      this.dispatchCustomEvent('login-success', { 
        user: authManager.getUser(),
        redirectUrl 
      });

      // Only redirect if redirect-url attribute is explicitly set
      if (redirectUrl && redirectUrl !== '' && redirectUrl !== window.location.pathname) {
        setTimeout(() => {
          window.location.href = redirectUrl;
        }, 100);
      }
    }
  }

  customElements.define('bb-auth-login', BBAuthLogin);

  /**
   * Job Dashboard Component for Blue Banded Bee
   * Provides comprehensive job summary with status overview cards and performance metrics
   */


  class BBJobDashboard extends BBBaseComponent {
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
      canvas.getContext('2d');
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

  /**
   * Blue Banded Bee Web Components
   * Main entry point for all components
   */


  // Initialize global BBComponents namespace
  window.BBComponents = {
    version: '1.0.0',
    
    // Component registry
    components: {
      'bb-data-loader': 'BBDataLoader',
      'bb-auth-login': 'BBAuthLogin',
      'bb-job-dashboard': 'BBJobDashboard'
    },
    
    // Utility functions
    async loadData(endpoint) {
      const { api } = await Promise.resolve().then(function () { return api$1; });
      return api.get(endpoint);
    },
    
    getAuthManager() {
      return Promise.resolve().then(function () { return simpleAuth; }).then(m => m.authManager);
    }
  };

  // Auto-initialize components when DOM is ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initializeComponents);
  } else {
    initializeComponents();
  }

  function initializeComponents() {
    console.log('Blue Banded Bee Web Components loaded successfully');
    
    // Add global styles
    if (!document.querySelector('#bb-components-styles')) {
      const styles = document.createElement('style');
      styles.id = 'bb-components-styles';
      styles.textContent = getGlobalStyles();
      document.head.appendChild(styles);
    }
  }

  function getGlobalStyles() {
    return `
    /* Blue Banded Bee Component Styles */
    .bb-loading {
      opacity: 0.7;
      pointer-events: none;
    }
    
    .bb-loading-indicator {
      padding: 20px;
      text-align: center;
      color: #666;
    }
    
    .bb-error {
      border: 1px solid #ff6b6b;
      border-radius: 4px;
      background: #fff5f5;
    }
    
    .bb-error-message {
      padding: 10px;
      color: #d63031;
      background: #fff5f5;
      border-radius: 4px;
      margin: 10px 0;
    }
    
    .bb-btn {
      display: inline-block;
      padding: 12px 24px;
      border: none;
      border-radius: 6px;
      font-size: 14px;
      font-weight: 500;
      text-decoration: none;
      cursor: pointer;
      transition: all 0.2s ease;
    }
    
    .bb-btn-primary {
      background: #0066ff;
      color: white;
    }
    
    .bb-btn-primary:hover {
      background: #0052cc;
    }
    
    .bb-btn-social {
      background: white;
      border: 1px solid #ddd;
      color: #333;
      display: flex;
      align-items: center;
      gap: 8px;
      width: 100%;
      justify-content: center;
      margin: 5px 0;
    }
    
    .bb-btn-social:hover {
      background: #f8f9fa;
    }
    
    .bb-social-icon {
      font-weight: bold;
      font-size: 16px;
    }
    
    .bb-auth-login .form-group {
      margin-bottom: 16px;
    }
    
    .bb-auth-login label {
      display: block;
      margin-bottom: 4px;
      font-weight: 500;
    }
    
    .bb-auth-login input {
      width: 100%;
      padding: 12px;
      border: 1px solid #ddd;
      border-radius: 6px;
      font-size: 14px;
    }
    
    .bb-auth-login input:focus {
      outline: none;
      border-color: #0066ff;
      box-shadow: 0 0 0 3px rgba(0, 102, 255, 0.1);
    }
    
    .bb-auth-links {
      margin-top: 16px;
      text-align: center;
    }
    
    .bb-auth-links a {
      color: #0066ff;
      text-decoration: none;
      font-size: 14px;
      margin: 0 8px;
    }
    
    .bb-divider {
      margin: 20px 0;
      text-align: center;
      position: relative;
      color: #666;
      font-size: 14px;
    }
    
    .bb-divider::before {
      content: '';
      position: absolute;
      top: 50%;
      left: 0;
      right: 0;
      height: 1px;
      background: #ddd;
      z-index: 1;
    }
    
    .bb-divider span {
      background: white;
      padding: 0 16px;
      position: relative;
      z-index: 2;
    }
    
    .template {
      display: none !important;
    }
  `;
  }

})();

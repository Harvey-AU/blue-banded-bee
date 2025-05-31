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
      return ['redirect-url', 'show-providers', 'compact', 'test-mode'];
    }

    constructor() {
      super();
      this.unsubscribeAuth = null;
    }

    connectedCallback() {
      super.connectedCallback();
      
      // Listen for auth state changes
      this.unsubscribeAuth = authManager.onAuthStateChange((event, session) => {
        if (session) {
          this.handleSuccessfulLogin();
        }
      });

      // Check if already logged in (skip in test mode)
      if (authManager.isAuthenticated() && !this.getBooleanAttribute('test-mode')) {
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
      const redirectUrl = this.hasAttribute('redirect-url') ? this.getAttribute('redirect-url') : '/dashboard';
      const testMode = this.getBooleanAttribute('test-mode');
      
      this.dispatchCustomEvent('login-success', { 
        user: authManager.getUser(),
        redirectUrl 
      });

      // Skip redirect in test mode
      if (testMode) {
        console.log('🧪 Test mode: Login successful but redirect prevented');
        return;
      }

      // Redirect if not prevented by parent
      setTimeout(() => {
        if (redirectUrl && redirectUrl !== '' && redirectUrl !== window.location.pathname) {
          window.location.href = redirectUrl;
        }
      }, 100);
    }
  }

  customElements.define('bb-auth-login', BBAuthLogin);

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
      'bb-auth-login': 'BBAuthLogin'
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

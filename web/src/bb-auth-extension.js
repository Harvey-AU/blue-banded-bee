/**
 * BB Data Binder Authentication Extension
 * 
 * Extends the existing BBDataBinder with authentication handlers and attributes.
 * Integrates with the AuthManager for unified auth state management.
 * 
 * Usage:
 *   <div data-bb-auth="required">Content for authenticated users</div>
 *   <div data-bb-auth="guest">Content for non-authenticated users</div>
 *   <button data-bb-action="show-login">Sign In</button>
 *   <form data-bb-form="start-crawling" data-bb-auth-required="true">...</form>
 */

(function() {
  'use strict';

  // Store original BBDataBinder class reference
  const OriginalBBDataBinder = window.BBDataBinder;

  if (!OriginalBBDataBinder) {
    console.error('BBDataBinder not found. Auth extension requires bb-data-binder.js to be loaded first.');
    return;
  }

  /**
   * Extended BBDataBinder with authentication capabilities
   */
  class BBDataBinderWithAuth extends OriginalBBDataBinder {
    constructor(config = {}) {
      super(config);
      
      this.authManager = null;
      this.authModalHTML = null;
      this.pendingAuthActions = [];
      
      // Auth configuration
      this.authConfig = {
        modalContainerId: config.authModalContainerId || 'authModalContainer',
        modalId: config.modalId || 'authModal',
        autoLoadAuthModal: config.autoLoadAuthModal !== false,
        authModalUrl: config.authModalUrl || '/auth-modal.html',
        ...config.auth
      };
    }

    /**
     * Override init to add auth functionality
     */
    async init() {
      // Call parent init
      await super.init();
      
      // Initialize auth manager
      await this.initAuthManager();
      
      // Load auth modal if configured
      if (this.authConfig.autoLoadAuthModal) {
        await this.loadAuthModal();
      }
      
      // Scan for auth-specific attributes
      this.scanAuthAttributes();
      
      // Set up global action handler
      this.setupActionHandler();
      
      this.log('BBDataBinder with auth initialized');
    }

    /**
     * Initialize AuthManager
     */
    async initAuthManager() {
      if (!window.AuthManager) {
        console.error('AuthManager not found. Please load auth.js before using auth extension.');
        return;
      }

      try {
        this.authManager = new window.AuthManager({
          debug: this.debug,
          apiBaseUrl: this.apiBaseUrl,
          ...this.authConfig
        });

        // Listen for auth state changes
        this.authManager.onAuthStateChange((isAuthenticated, user, session) => {
          this.updateAuthElements();
          this.processPendingAuthActions(isAuthenticated);
        });

        this.log('AuthManager initialized successfully');
      } catch (error) {
        console.error('AuthManager initialization failed:', error);
      }
    }

    /**
     * Load auth modal HTML
     */
    async loadAuthModal() {
      try {
        const container = document.getElementById(this.authConfig.modalContainerId);
        if (!container) {
          this.log('Auth modal container not found, creating one');
          const newContainer = document.createElement('div');
          newContainer.id = this.authConfig.modalContainerId;
          document.body.appendChild(newContainer);
        }

        if (!this.authModalHTML) {
          const response = await fetch(this.authConfig.authModalUrl);
          if (!response.ok) {
            throw new Error(`Failed to load auth modal: ${response.status}`);
          }
          this.authModalHTML = await response.text();
        }

        const targetContainer = document.getElementById(this.authConfig.modalContainerId);
        if (targetContainer && this.authModalHTML) {
          targetContainer.innerHTML = this.authModalHTML;
          this.log('Auth modal loaded successfully');
        }
      } catch (error) {
        console.error('Failed to load auth modal:', error);
      }
    }

    /**
     * Scan for auth-specific attributes
     */
    scanAuthAttributes() {
      // Scan for auth visibility elements
      const authElements = document.querySelectorAll('[data-bb-auth]');
      authElements.forEach(element => this.registerAuthElement(element));

      // Scan for auth actions
      const actionElements = document.querySelectorAll('[data-bb-action]');
      actionElements.forEach(element => this.registerActionElement(element));

      // Scan for auth-protected forms
      const authForms = document.querySelectorAll('[data-bb-form][data-bb-auth-required]');
      authForms.forEach(form => this.registerAuthForm(form));

      this.log('Auth attributes scanned', {
        authElements: authElements.length,
        actionElements: actionElements.length,
        authForms: authForms.length
      });
    }

    /**
     * Register auth visibility element
     */
    registerAuthElement(element) {
      const authType = element.getAttribute('data-bb-auth');
      this.log('Registering auth element', { type: authType, element });
      
      // Store original display style if not already stored
      if (!element.hasAttribute('data-bb-original-display')) {
        const computedStyle = window.getComputedStyle(element);
        element.setAttribute('data-bb-original-display', computedStyle.display);
      }

      this.updateAuthElement(element);
    }

    /**
     * Update single auth element visibility
     */
    updateAuthElement(element) {
      const authType = element.getAttribute('data-bb-auth');
      const originalDisplay = element.getAttribute('data-bb-original-display') || 'block';
      let shouldShow = true;

      switch (authType) {
        case 'required':
          shouldShow = this.authManager?.isAuthenticated || false;
          break;
        case 'guest':
          shouldShow = !this.authManager?.isAuthenticated;
          break;
        case 'loading':
          shouldShow = !this.authManager; // Show loading state while auth manager initializes
          break;
        default:
          shouldShow = true;
      }

      element.style.display = shouldShow ? originalDisplay : 'none';
    }

    /**
     * Update all auth elements
     */
    updateAuthElements() {
      const authElements = document.querySelectorAll('[data-bb-auth]');
      authElements.forEach(element => this.updateAuthElement(element));
      this.log('Auth elements updated', { count: authElements.length });
    }

    /**
     * Register action element
     */
    registerActionElement(element) {
      const action = element.getAttribute('data-bb-action');
      if (this.isAuthAction(action)) {
        element.addEventListener('click', (e) => {
          e.preventDefault();
          this.handleAuthAction(action, element);
        });
        this.log('Registered auth action', { action, element });
      }
    }

    /**
     * Check if action is auth-related
     */
    isAuthAction(action) {
      const authActions = [
        'show-login', 'show-signup', 'show-reset-password',
        'sign-in', 'sign-up', 'sign-out', 'reset-password',
        'oauth-google', 'oauth-github'
      ];
      return authActions.includes(action);
    }

    /**
     * Handle auth action
     */
    async handleAuthAction(action, element) {
      this.log('Handling auth action', { action, element });

      switch (action) {
        case 'show-login':
          this.showAuthModal('login');
          break;
        case 'show-signup':
          this.showAuthModal('signup');
          break;
        case 'show-reset-password':
          this.showAuthModal('reset');
          break;
        case 'sign-out':
          await this.handleSignOut();
          break;
        case 'oauth-google':
          await this.handleOAuth('google');
          break;
        case 'oauth-github':
          await this.handleOAuth('github');
          break;
        default:
          this.log('Unknown auth action', action);
      }
    }

    /**
     * Show auth modal with specific form
     */
    showAuthModal(formType = 'login') {
      const modal = document.getElementById(this.authConfig.modalId);
      if (!modal) {
        console.error('Auth modal not found. Ensure auth modal is loaded.');
        return;
      }

      modal.classList.add('show');
      
      // Call the form switcher if available
      if (window.showAuthForm) {
        window.showAuthForm(formType);
      } else if (window[`show${formType.charAt(0).toUpperCase() + formType.slice(1)}Form`]) {
        window[`show${formType.charAt(0).toUpperCase() + formType.slice(1)}Form`]();
      }
    }

    /**
     * Handle sign out
     */
    async handleSignOut() {
      if (!this.authManager) {
        console.error('AuthManager not available');
        return;
      }

      const result = await this.authManager.signOut();
      if (result.success) {
        this.log('Sign out successful');
        // Optionally redirect or refresh
        if (window.location.pathname !== '/') {
          window.location.href = '/';
        }
      } else {
        console.error('Sign out failed:', result.error);
      }
    }

    /**
     * Handle OAuth login
     */
    async handleOAuth(provider) {
      if (!this.authManager) {
        console.error('AuthManager not available');
        return;
      }

      const result = await this.authManager.signInWithOAuth(provider);
      if (!result.success) {
        console.error(`${provider} OAuth failed:`, result.error);
      }
    }

    /**
     * Register auth-protected form
     */
    registerAuthForm(form) {
      const formAction = form.getAttribute('data-bb-form');
      const originalSubmitHandler = this.getFormSubmitHandler(form);

      // Override form submission to check auth first
      form.addEventListener('submit', (e) => {
        e.preventDefault();

        if (!this.authManager?.isAuthenticated) {
          // Store form data for after auth
          const formData = this.collectFormData(form);
          this.addPendingAuthAction('form-submit', {
            form,
            formAction,
            formData
          });

          // Show auth modal
          this.showAuthModal('login');
          return;
        }

        // User is authenticated, proceed with original handler
        if (originalSubmitHandler) {
          originalSubmitHandler(e);
        }
      });

      this.log('Registered auth-protected form', { action: formAction, form });
    }

    /**
     * Set up global action handler for non-auth actions
     */
    setupActionHandler() {
      document.addEventListener('click', (e) => {
        const actionElement = e.target.closest('[data-bb-action]');
        if (!actionElement) return;

        const action = actionElement.getAttribute('data-bb-action');
        if (this.isAuthAction(action)) return; // Already handled

        // Handle non-auth actions that might require auth
        if (actionElement.hasAttribute('data-bb-auth-required') && !this.authManager?.isAuthenticated) {
          e.preventDefault();
          this.addPendingAuthAction('action', { action, element: actionElement });
          this.showAuthModal('login');
          return;
        }
      });
    }

    /**
     * Add pending auth action
     */
    addPendingAuthAction(type, data) {
      this.pendingAuthActions.push({
        type,
        data,
        timestamp: Date.now()
      });
      this.log('Added pending auth action', { type, data });
    }

    /**
     * Process pending auth actions after successful authentication
     */
    processPendingAuthActions(isAuthenticated) {
      if (!isAuthenticated || this.pendingAuthActions.length === 0) {
        return;
      }

      const actionsToProcess = [...this.pendingAuthActions];
      this.pendingAuthActions = [];

      actionsToProcess.forEach(({ type, data }) => {
        try {
          switch (type) {
            case 'form-submit':
              this.processPendingFormSubmit(data);
              break;
            case 'action':
              this.processPendingAction(data);
              break;
            default:
              this.log('Unknown pending action type', type);
          }
        } catch (error) {
          console.error('Error processing pending action:', error);
        }
      });

      this.log('Processed pending auth actions', { count: actionsToProcess.length });
    }

    /**
     * Process pending form submit
     */
    async processPendingFormSubmit({ form, formAction, formData }) {
      // Close auth modal
      const modal = document.getElementById(this.authConfig.modalId);
      if (modal) {
        modal.classList.remove('show');
      }

      // Handle special form actions
      if (formAction === 'start-crawling' && formData.domain) {
        await this.authManager.handleStartCrawling({
          domain: formData.domain,
          maxPages: formData.max_pages || 0,
          concurrency: formData.concurrency || 5
        });
        return;
      }

      // Trigger the original form submission
      const submitEvent = new Event('submit', { cancelable: true });
      form.dispatchEvent(submitEvent);
    }

    /**
     * Process pending action
     */
    processPendingAction({ action, element }) {
      // Close auth modal
      const modal = document.getElementById(this.authConfig.modalId);
      if (modal) {
        modal.classList.remove('show');
      }

      // Trigger the action
      element.click();
    }

    /**
     * Get form submit handler (compatibility with existing code)
     */
    getFormSubmitHandler(form) {
      // This is a simplified approach - in practice, you might need
      // to integrate more deeply with existing form handling
      return null;
    }

    /**
     * Override parent's handleFormSubmit to integrate with auth
     */
    async handleFormSubmit(form, action) {
      // Check if auth is required for this form
      if (form.hasAttribute('data-bb-auth-required') && !this.authManager?.isAuthenticated) {
        const formData = this.collectFormData(form);
        this.addPendingAuthAction('form-submit', {
          form,
          formAction: action,
          formData
        });
        this.showAuthModal('login');
        return;
      }

      // Call parent method
      return super.handleFormSubmit(form, action);
    }

    /**
     * Enhanced logging with auth context
     */
    log(message, data = null) {
      if (this.debug) {
        const authContext = {
          isAuthenticated: this.authManager?.isAuthenticated || false,
          userEmail: this.authManager?.user?.email || null
        };
        console.log(`[BBDataBinder+Auth] ${message}`, data, authContext);
      }
    }
  }

  // Replace the global BBDataBinder with our extended version
  window.BBDataBinder = BBDataBinderWithAuth;

  // Export for module systems
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = BBDataBinderWithAuth;
  }

})();
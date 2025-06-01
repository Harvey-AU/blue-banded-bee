(function () {
  'use strict';

  /**
   * Blue Banded Bee Data Binding Library
   * 
   * Provides template + data binding system for Blue Banded Bee dashboard pages.
   * Automatically finds and populates elements with data-bb-bind attributes.
   */

  class BBDataBinder {
    constructor(options = {}) {
      this.apiBaseUrl = options.apiBaseUrl || '';
      this.authManager = null;
      this.refreshInterval = options.refreshInterval || 0;
      this.refreshTimer = null;
      this.debug = options.debug || false;
      
      // Store bound elements for efficient updates
      this.boundElements = new Map();
      this.templates = new Map();
      
      this.log('BBDataBinder initialized', options);
    }

    /**
     * Initialize the data binder
     */
    async init() {
      this.log('Initializing data binder...');
      
      // Initialize authentication if available
      if (window.supabase) {
        await this.initAuth();
      }
      
      // Scan and bind all elements
      this.scanAndBind();
      
      // Set up auto-refresh if configured
      if (this.refreshInterval > 0) {
        this.startAutoRefresh();
      }
      
      this.log('Data binder initialized successfully');
    }

    /**
     * Initialize Supabase authentication
     */
    async initAuth() {
      try {
        const { data: { session } } = await window.supabase.auth.getSession();
        this.authManager = {
          session,
          isAuthenticated: !!session,
          user: session?.user || null
        };
        
        // Listen for auth changes
        window.supabase.auth.onAuthStateChange((event, session) => {
          this.authManager.session = session;
          this.authManager.isAuthenticated = !!session;
          this.authManager.user = session?.user || null;
          
          // Re-scan conditional auth elements
          this.updateAuthElements();
        });
        
        this.log('Auth initialized', { authenticated: this.authManager.isAuthenticated });
      } catch (error) {
        this.log('Auth initialization failed', error);
      }
    }

    /**
     * Scan the DOM and bind all data binding attributes
     */
    scanAndBind() {
      this.log('Scanning DOM for data binding attributes...');
      
      // Find all elements with data binding attributes
      const bindElements = document.querySelectorAll('[data-bb-bind]');
      const styleElements = document.querySelectorAll('[data-bb-bind-style]');
      const attrElements = document.querySelectorAll('[data-bb-bind-attr]');
      const templateElements = document.querySelectorAll('[data-bb-template]');
      const authElements = document.querySelectorAll('[data-bb-auth]');
      const formElements = document.querySelectorAll('[data-bb-form]');
      
      this.log('Found elements', {
        bind: bindElements.length,
        style: styleElements.length,
        attr: attrElements.length,
        template: templateElements.length,
        auth: authElements.length,
        forms: formElements.length
      });
      
      // Process data binding elements
      bindElements.forEach(el => this.registerBindElement(el));
      styleElements.forEach(el => this.registerStyleElement(el));
      attrElements.forEach(el => this.registerAttrElement(el));
      
      // Process template elements
      templateElements.forEach(el => this.registerTemplate(el));
      
      // Process auth elements
      authElements.forEach(el => this.updateAuthElement(el));
      
      // Process form elements
      formElements.forEach(el => this.registerForm(el));
    }

    /**
     * Register an element for data binding
     */
    registerBindElement(element) {
      const bindPath = element.getAttribute('data-bb-bind');
      if (!bindPath) return;
      
      if (!this.boundElements.has(bindPath)) {
        this.boundElements.set(bindPath, []);
      }
      
      this.boundElements.get(bindPath).push({
        element,
        type: 'text',
        path: bindPath
      });
      
      this.log('Registered bind element', { path: bindPath, element });
    }

    /**
     * Register an element for style binding
     */
    registerStyleElement(element) {
      const styleBinding = element.getAttribute('data-bb-bind-style');
      if (!styleBinding) return;
      
      // Parse style binding format: "width:{progress}%"
      const match = styleBinding.match(/^([^:]+):(.+)$/);
      if (!match) return;
      
      const [, property, template] = match;
      const pathMatches = template.match(/\{([^}]+)\}/g);
      
      if (pathMatches) {
        pathMatches.forEach(pathMatch => {
          const path = pathMatch.slice(1, -1); // Remove { }
          
          if (!this.boundElements.has(path)) {
            this.boundElements.set(path, []);
          }
          
          this.boundElements.get(path).push({
            element,
            type: 'style',
            property,
            template,
            path
          });
        });
      }
      
      this.log('Registered style element', { property, template, element });
    }

    /**
     * Register an element for attribute binding
     */
    registerAttrElement(element) {
      const attrBinding = element.getAttribute('data-bb-bind-attr');
      if (!attrBinding) return;
      
      // Parse attribute binding format: "href:{url}"
      const match = attrBinding.match(/^([^:]+):(.+)$/);
      if (!match) return;
      
      const [, attribute, template] = match;
      const pathMatches = template.match(/\{([^}]+)\}/g);
      
      if (pathMatches) {
        pathMatches.forEach(pathMatch => {
          const path = pathMatch.slice(1, -1); // Remove { }
          
          if (!this.boundElements.has(path)) {
            this.boundElements.set(path, []);
          }
          
          this.boundElements.get(path).push({
            element,
            type: 'attribute',
            attribute,
            template,
            path
          });
        });
      }
      
      this.log('Registered attribute element', { attribute, template, element });
    }

    /**
     * Register a template element for repeated content
     */
    registerTemplate(element) {
      const templateName = element.getAttribute('data-bb-template');
      if (!templateName) return;
      
      // Store the template
      this.templates.set(templateName, {
        element,
        originalHTML: element.outerHTML,
        parent: element.parentElement
      });
      
      // Hide the template element
      element.style.display = 'none';
      
      this.log('Registered template', { name: templateName, element });
    }

    /**
     * Update authentication-conditional elements
     */
    updateAuthElements() {
      const authElements = document.querySelectorAll('[data-bb-auth]');
      authElements.forEach(el => this.updateAuthElement(el));
    }

    /**
     * Update a single auth element
     */
    updateAuthElement(element) {
      const authCondition = element.getAttribute('data-bb-auth');
      let shouldShow = false;
      
      switch (authCondition) {
        case 'required':
          shouldShow = this.authManager?.isAuthenticated || false;
          break;
        case 'guest':
          shouldShow = !this.authManager?.isAuthenticated;
          break;
        default:
          shouldShow = true;
      }
      
      element.style.display = shouldShow ? '' : 'none';
    }

    /**
     * Register a form for handling
     */
    registerForm(form) {
      const formAction = form.getAttribute('data-bb-form');
      if (!formAction) return;
      
      this.log('Registering form', { action: formAction, form });
      
      // Set up form submission handler
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        await this.handleFormSubmit(form, formAction);
      });
      
      // Set up real-time validation if configured
      const validateOnChange = form.getAttribute('data-bb-validate') === 'live';
      if (validateOnChange) {
        const inputs = form.querySelectorAll('input, select, textarea');
        inputs.forEach(input => {
          input.addEventListener('input', () => this.validateFormField(input));
          input.addEventListener('blur', () => this.validateFormField(input));
        });
      }
    }

    /**
     * Handle form submission
     */
    async handleFormSubmit(form, action) {
      try {
        this.log('Form submission started', { action });
        
        // Set loading state
        this.setFormLoading(form, true);
        
        // Validate form
        const isValid = this.validateForm(form);
        if (!isValid) {
          this.setFormLoading(form, false);
          return;
        }
        
        // Collect form data
        const formData = this.collectFormData(form);
        
        // Determine API endpoint
        const endpoint = this.getFormEndpoint(action, formData);
        
        // Submit form
        const result = await this.submitForm(endpoint, formData, action);
        
        // Handle success
        this.handleFormSuccess(form, result, action);
        
      } catch (error) {
        this.log('Form submission failed', { action, error });
        this.handleFormError(form, error, action);
      } finally {
        this.setFormLoading(form, false);
      }
    }

    /**
     * Collect form data
     */
    collectFormData(form) {
      const formData = new FormData(form);
      const data = {};
      
      for (const [key, value] of formData.entries()) {
        // Handle multiple values for same key (checkboxes, etc.)
        if (data[key]) {
          if (Array.isArray(data[key])) {
            data[key].push(value);
          } else {
            data[key] = [data[key], value];
          }
        } else {
          data[key] = value;
        }
      }
      
      return data;
    }

    /**
     * Get API endpoint for form action
     */
    getFormEndpoint(action, data) {
      switch (action) {
        case 'create-job':
          return '/v1/jobs';
        case 'update-profile':
          return '/v1/auth/profile';
        case 'create-organisation':
          return '/v1/organisations';
        default:
          // Custom endpoint from data-bb-endpoint attribute
          const form = document.querySelector(`[data-bb-form="${action}"]`);
          return form?.getAttribute('data-bb-endpoint') || `/v1/${action}`;
      }
    }

    /**
     * Submit form data to API
     */
    async submitForm(endpoint, data, action) {
      const method = this.getFormMethod(action);
      const headers = {
        'Content-Type': 'application/json'
      };
      
      // Add auth header if available
      if (this.authManager?.session?.access_token) {
        headers['Authorization'] = `Bearer ${this.authManager.session.access_token}`;
      }
      
      const response = await fetch(`${this.apiBaseUrl}${endpoint}`, {
        method,
        headers,
        body: JSON.stringify(data)
      });
      
      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.message || `HTTP ${response.status}: ${response.statusText}`);
      }
      
      return await response.json();
    }

    /**
     * Get HTTP method for form action
     */
    getFormMethod(action) {
      switch (action) {
        case 'create-job':
        case 'create-organisation':
          return 'POST';
        case 'update-profile':
          return 'PUT';
        case 'delete-job':
          return 'DELETE';
        default:
          return 'POST';
      }
    }

    /**
     * Validate entire form
     */
    validateForm(form) {
      const inputs = form.querySelectorAll('input, select, textarea');
      let isValid = true;
      
      inputs.forEach(input => {
        if (!this.validateFormField(input)) {
          isValid = false;
        }
      });
      
      return isValid;
    }

    /**
     * Validate a single form field
     */
    validateFormField(input) {
      const rules = this.getValidationRules(input);
      const value = input.value.trim();
      const errors = [];
      
      // Required validation
      if (rules.required && !value) {
        errors.push('This field is required');
      }
      
      // Type-specific validation
      if (value && rules.type) {
        switch (rules.type) {
          case 'email':
            if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
              errors.push('Please enter a valid email address');
            }
            break;
          case 'url':
            try {
              new URL(value);
            } catch {
              errors.push('Please enter a valid URL');
            }
            break;
          case 'number':
            if (isNaN(Number(value))) {
              errors.push('Please enter a valid number');
            }
            break;
        }
      }
      
      // Length validation
      if (value && rules.minLength && value.length < rules.minLength) {
        errors.push(`Must be at least ${rules.minLength} characters`);
      }
      
      if (value && rules.maxLength && value.length > rules.maxLength) {
        errors.push(`Must be no more than ${rules.maxLength} characters`);
      }
      
      // Custom pattern validation
      if (value && rules.pattern && !new RegExp(rules.pattern).test(value)) {
        errors.push(rules.patternMessage || 'Invalid format');
      }
      
      // Update field UI
      this.updateFieldValidation(input, errors);
      
      return errors.length === 0;
    }

    /**
     * Get validation rules for an input
     */
    getValidationRules(input) {
      const rules = {
        required: input.hasAttribute('required'),
        type: input.getAttribute('data-bb-validate-type') || input.type,
        minLength: parseInt(input.getAttribute('data-bb-validate-min')) || null,
        maxLength: parseInt(input.getAttribute('data-bb-validate-max')) || null,
        pattern: input.getAttribute('data-bb-validate-pattern'),
        patternMessage: input.getAttribute('data-bb-validate-message')
      };
      
      return rules;
    }

    /**
     * Update field validation UI
     */
    updateFieldValidation(input, errors) {
      const isValid = errors.length === 0;
      
      // Remove existing validation classes and messages
      input.classList.remove('bb-field-valid', 'bb-field-invalid');
      const existingError = input.parentElement.querySelector('.bb-field-error');
      if (existingError) {
        existingError.remove();
      }
      
      // Add validation state
      if (input.value.trim()) {
        input.classList.add(isValid ? 'bb-field-valid' : 'bb-field-invalid');
        
        // Show error message
        if (!isValid) {
          const errorDiv = document.createElement('div');
          errorDiv.className = 'bb-field-error';
          errorDiv.textContent = errors[0]; // Show first error
          errorDiv.style.cssText = 'color: #dc2626; font-size: 12px; margin-top: 4px;';
          input.parentElement.appendChild(errorDiv);
        }
      }
    }

    /**
     * Set form loading state
     */
    setFormLoading(form, loading) {
      const submitButton = form.querySelector('button[type="submit"], input[type="submit"]');
      const loadingElements = form.querySelectorAll('[data-bb-loading]');
      
      if (submitButton) {
        submitButton.disabled = loading;
        if (loading) {
          submitButton.setAttribute('data-original-text', submitButton.textContent);
          submitButton.textContent = 'Loading...';
        } else {
          const originalText = submitButton.getAttribute('data-original-text');
          if (originalText) {
            submitButton.textContent = originalText;
            submitButton.removeAttribute('data-original-text');
          }
        }
      }
      
      loadingElements.forEach(el => {
        el.style.display = loading ? '' : 'none';
      });
    }

    /**
     * Handle form success
     */
    handleFormSuccess(form, result, action) {
      this.log('Form submission successful', { action, result });
      
      // Clear form if specified
      if (form.getAttribute('data-bb-clear-on-success') === 'true') {
        form.reset();
      }
      
      // Show success message
      this.showFormMessage(form, 'Success! Your request has been processed.', 'success');
      
      // Trigger custom success handler
      const successEvent = new CustomEvent('bb-form-success', {
        detail: { action, result, form }
      });
      form.dispatchEvent(successEvent);
      
      // Redirect if specified
      const redirectUrl = form.getAttribute('data-bb-redirect');
      if (redirectUrl) {
        setTimeout(() => {
          window.location.href = redirectUrl;
        }, 1000);
      }
    }

    /**
     * Handle form error
     */
    handleFormError(form, error, action) {
      this.log('Form submission error', { action, error });
      
      // Show error message
      this.showFormMessage(form, error.message || 'An error occurred. Please try again.', 'error');
      
      // Trigger custom error handler
      const errorEvent = new CustomEvent('bb-form-error', {
        detail: { action, error, form }
      });
      form.dispatchEvent(errorEvent);
    }

    /**
     * Show form message
     */
    showFormMessage(form, message, type) {
      // Remove existing messages
      const existingMessage = form.querySelector('.bb-form-message');
      if (existingMessage) {
        existingMessage.remove();
      }
      
      // Create message element
      const messageDiv = document.createElement('div');
      messageDiv.className = `bb-form-message bb-form-message-${type}`;
      messageDiv.textContent = message;
      
      // Style message
      const styles = {
        padding: '12px 16px',
        borderRadius: '6px',
        marginBottom: '16px',
        fontSize: '14px',
        fontWeight: '500'
      };
      
      if (type === 'success') {
        styles.background = '#dcfce7';
        styles.color = '#16a34a';
        styles.border = '1px solid #bbf7d0';
      } else {
        styles.background = '#fee2e2';
        styles.color = '#dc2626';
        styles.border = '1px solid #fecaca';
      }
      
      Object.assign(messageDiv.style, styles);
      
      // Insert at top of form
      form.insertBefore(messageDiv, form.firstChild);
      
      // Auto-remove after 5 seconds
      setTimeout(() => {
        if (messageDiv.parentElement) {
          messageDiv.remove();
        }
      }, 5000);
    }

    /**
     * Fetch data from API endpoint
     */
    async fetchData(endpoint, options = {}) {
      try {
        const headers = {
          'Content-Type': 'application/json',
          ...options.headers
        };
        
        // Add auth header if available
        if (this.authManager?.session?.access_token) {
          headers['Authorization'] = `Bearer ${this.authManager.session.access_token}`;
        }
        
        const fetchOptions = {
          method: 'GET',
          ...options,
          headers
        };
        
        const response = await fetch(`${this.apiBaseUrl}${endpoint}`, fetchOptions);
        
        if (!response.ok) {
          throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        
        const result = await response.json();
        return result.data || result;
        
      } catch (error) {
        this.log('API fetch failed', { endpoint, error });
        throw error;
      }
    }

    /**
     * Update bound elements with new data
     */
    updateElements(data) {
      this.log('Updating elements with data', data);
      
      // Update all bound elements
      for (const [path, elements] of this.boundElements) {
        const value = this.getValueByPath(data, path);
        
        elements.forEach(binding => {
          switch (binding.type) {
            case 'text':
              binding.element.textContent = value ?? binding.element.textContent;
              break;
              
            case 'style':
              const styleValue = this.processTemplate(binding.template, data);
              if (styleValue !== null) {
                binding.element.style[binding.property] = styleValue;
              }
              break;
              
            case 'attribute':
              const attrValue = this.processTemplate(binding.template, data);
              if (attrValue !== null) {
                binding.element.setAttribute(binding.attribute, attrValue);
              }
              break;
          }
        });
      }
    }

    /**
     * Render template with array data
     */
    renderTemplate(templateName, items) {
      const template = this.templates.get(templateName);
      if (!template) {
        this.log('Template not found', templateName);
        return;
      }
      
      this.log('Rendering template', { name: templateName, items: items?.length });
      
      // Remove existing instances
      const existing = template.parent.querySelectorAll(`[data-bb-template-instance="${templateName}"]`);
      existing.forEach(el => el.remove());
      
      // Create new instances
      if (Array.isArray(items) && items.length > 0) {
        items.forEach((item, index) => {
          const instance = this.createTemplateInstance(template, item, index);
          if (instance) {
            template.parent.appendChild(instance);
          }
        });
      }
    }

    /**
     * Create a template instance with data
     */
    createTemplateInstance(template, data, index) {
      const tempDiv = document.createElement('div');
      tempDiv.innerHTML = template.originalHTML;
      const instance = tempDiv.firstElementChild;
      
      if (!instance) return null;
      
      // Mark as template instance
      instance.setAttribute('data-bb-template-instance', template.element.getAttribute('data-bb-template'));
      instance.removeAttribute('data-bb-template');
      instance.style.display = '';
      
      // Bind data to instance elements
      const bindElements = instance.querySelectorAll('[data-bb-bind]');
      bindElements.forEach(el => {
        const path = el.getAttribute('data-bb-bind');
        const value = this.getValueByPath(data, path);
        if (value !== undefined) {
          el.textContent = value;
        }
      });
      
      // Handle style bindings
      const styleElements = instance.querySelectorAll('[data-bb-bind-style]');
      styleElements.forEach(el => {
        const styleBinding = el.getAttribute('data-bb-bind-style');
        const match = styleBinding.match(/^([^:]+):(.+)$/);
        if (match) {
          const [, property, template] = match;
          const value = this.processTemplate(template, data);
          if (value !== null) {
            el.style[property] = value;
          }
        }
      });
      
      // Handle attribute bindings
      const attrElements = instance.querySelectorAll('[data-bb-bind-attr]');
      attrElements.forEach(el => {
        const attrBinding = el.getAttribute('data-bb-bind-attr');
        const match = attrBinding.match(/^([^:]+):(.+)$/);
        if (match) {
          const [, attribute, template] = match;
          const value = this.processTemplate(template, data);
          if (value !== null) {
            el.setAttribute(attribute, value);
          }
        }
      });
      
      return instance;
    }

    /**
     * Process template string with data
     */
    processTemplate(template, data) {
      return template.replace(/\{([^}]+)\}/g, (match, path) => {
        const value = this.getValueByPath(data, path);
        return value !== undefined ? value : match;
      });
    }

    /**
     * Get value from object by dot notation path
     */
    getValueByPath(obj, path) {
      return path.split('.').reduce((current, key) => {
        return current && current[key] !== undefined ? current[key] : undefined;
      }, obj);
    }

    /**
     * Start auto-refresh timer
     */
    startAutoRefresh() {
      if (this.refreshTimer) {
        clearInterval(this.refreshTimer);
      }
      
      this.refreshTimer = setInterval(() => {
        this.refresh();
      }, this.refreshInterval * 1000);
      
      this.log('Auto-refresh started', { interval: this.refreshInterval });
    }

    /**
     * Stop auto-refresh timer
     */
    stopAutoRefresh() {
      if (this.refreshTimer) {
        clearInterval(this.refreshTimer);
        this.refreshTimer = null;
      }
      
      this.log('Auto-refresh stopped');
    }

    /**
     * Refresh all bound data
     */
    async refresh() {
      // This method should be overridden by implementations
      // or called with specific data endpoints
      this.log('Refresh called - override this method in your implementation');
    }

    /**
     * Load and bind data from specific endpoints
     */
    async loadAndBind(endpoints) {
      try {
        const promises = Object.entries(endpoints).map(async ([key, endpoint]) => {
          const data = await this.fetchData(endpoint);
          return [key, data];
        });
        
        const results = await Promise.all(promises);
        const combinedData = Object.fromEntries(results);
        
        this.updateElements(combinedData);
        
        return combinedData;
      } catch (error) {
        this.log('Load and bind failed', error);
        throw error;
      }
    }

    /**
     * Bind data to templates
     */
    bindTemplates(templateData) {
      Object.entries(templateData).forEach(([templateName, items]) => {
        this.renderTemplate(templateName, items);
      });
    }

    /**
     * Debug logging
     */
    log(message, data = null) {
      if (this.debug) {
        console.log(`[BBDataBinder] ${message}`, data);
      }
    }

    /**
     * Destroy the data binder
     */
    destroy() {
      this.stopAutoRefresh();
      this.boundElements.clear();
      this.templates.clear();
      this.log('Data binder destroyed');
    }
  }

  // Export for use as module or global
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = BBDataBinder;
  } else {
    window.BBDataBinder = BBDataBinder;
  }

})();

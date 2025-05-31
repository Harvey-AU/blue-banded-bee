/**
 * Base class for Blue Banded Bee Web Components
 */

export class BBBaseComponent extends HTMLElement {
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
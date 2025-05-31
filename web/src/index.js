/**
 * Blue Banded Bee Web Components
 * Main entry point for all components
 */

// Core utilities
import './utils/api.js';
import './utils/base-component.js';
import './auth/simple-auth.js';

// Components
import './components/bb-data-loader.js';
import './components/bb-auth-login.js';
import './components/bb-job-dashboard.js';

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
    const { api } = await import('./utils/api.js');
    return api.get(endpoint);
  },
  
  getAuthManager() {
    return import('./auth/simple-auth.js').then(m => m.authManager);
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
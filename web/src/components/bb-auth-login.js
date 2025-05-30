/**
 * Login component for Blue Banded Bee
 */

import { BBBaseComponent } from '../utils/base-component.js';
import { authManager } from '../auth/simple-auth.js';

export class BBAuthLogin extends BBBaseComponent {
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
      if (session) {
        this.handleSuccessfulLogin();
      }
    });

    // Check if already logged in
    if (authManager.isAuthenticated()) {
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
    const redirectUrl = this.getAttribute('redirect-url') || '/dashboard';
    
    this.dispatchCustomEvent('login-success', { 
      user: authManager.getUser(),
      redirectUrl 
    });

    // Redirect if not prevented by parent
    setTimeout(() => {
      if (redirectUrl && redirectUrl !== window.location.pathname) {
        window.location.href = redirectUrl;
      }
    }, 100);
  }
}

customElements.define('bb-auth-login', BBAuthLogin);
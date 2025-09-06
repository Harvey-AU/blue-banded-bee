/**
 * Blue Banded Bee Unified Authentication System
 * 
 * Centralised authentication management using declarative HTML attributes.
 * Handles Supabase auth, OAuth redirects, and cross-interface consistency.
 * 
 * Usage:
 *   <div data-bb-auth="required">Content for authenticated users</div>
 *   <div data-bb-auth="guest">Content for non-authenticated users</div>
 *   <button data-bb-action="show-login">Sign In</button>
 *   <form data-bb-form="start-crawling">...</form>
 */

class AuthManager {
  constructor(config = {}) {
    this.config = {
      supabaseUrl: config.supabaseUrl || 'https://auth.bluebandedbee.co',
      supabaseKey: config.supabaseKey || 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imdwemp0Ymd0ZGp4bmFjZGZ1anZ4Iiwicm9sZSI6ImFub24iLCJpYXQiOjE3NDUwNjYxNjMsImV4cCI6MjA2MDY0MjE2M30.eJjM2-3X8oXsFex_lQKvFkP1-_yLMHsueIn7_hCF6YI',
      apiBaseUrl: config.apiBaseUrl || '',
      debug: config.debug || false,
      ...config
    };

    this.supabase = null;
    this.user = null;
    this.isAuthenticated = false;
    this.authStateCallbacks = [];
    this.pendingAuth = null;

    this.init();
  }

  /**
   * Initialise authentication system
   */
  async init() {
    try {
      // Wait for Supabase to be available
      await this.waitForSupabase();
      
      // Create Supabase client
      this.supabase = window.supabase.createClient(this.config.supabaseUrl, this.config.supabaseKey);
      
      // Handle OAuth callback if present
      await this.handleAuthCallback();
      
      // Check current session
      const { data: { session } } = await this.supabase.auth.getSession();
      if (session?.user) {
        await this.setAuthState(session.user, session);
      }
      
      // Set up auth state change listener
      this.supabase.auth.onAuthStateChange(async (event, session) => {
        this.log('Auth state changed:', event, session?.user?.email);
        
        if (session?.user) {
          await this.setAuthState(session.user, session);
        } else {
          await this.setAuthState(null, null);
        }
        
        // Handle any pending auth actions
        if (this.pendingAuth && this.isAuthenticated) {
          await this.handlePendingAuth();
        }
      });
      
      this.log('AuthManager initialised successfully');
    } catch (error) {
      console.error('AuthManager initialisation failed:', error);
    }
  }

  /**
   * Wait for Supabase library to be available
   */
  async waitForSupabase() {
    return new Promise((resolve) => {
      const checkSupabase = () => {
        if (window.supabase && window.supabase.createClient) {
          resolve();
        } else {
          setTimeout(checkSupabase, 100);
        }
      };
      checkSupabase();
    });
  }

  /**
   * Handle OAuth callback from URL hash
   */
  async handleAuthCallback() {
    try {
      const hashParams = new URLSearchParams(window.location.hash.substring(1));
      const accessToken = hashParams.get('access_token');
      const refreshToken = hashParams.get('refresh_token');
      const state = hashParams.get('state');

      if (accessToken && refreshToken) {
        this.log('Processing OAuth callback...');
        
        // Set session in Supabase
        const { data: { session }, error } = await this.supabase.auth.setSession({
          access_token: accessToken,
          refresh_token: refreshToken
        });

        if (error) {
          console.error('OAuth session setup error:', error);
          return false;
        }

        // Clean up URL hash
        history.replaceState(null, null, window.location.pathname + window.location.search);

        // Handle return URL from state parameter
        if (state) {
          try {
            const returnData = JSON.parse(atob(state));
            if (returnData.returnUrl && this.isValidReturnUrl(returnData.returnUrl)) {
              this.log('Redirecting to return URL:', returnData.returnUrl);
              setTimeout(() => {
                window.location.href = returnData.returnUrl;
              }, 1000);
              return true;
            }
          } catch (e) {
            this.log('Invalid state parameter:', e);
          }
        }

        return true;
      }

      return false;
    } catch (error) {
      console.error('Auth callback error:', error);
      return false;
    }
  }

  /**
   * Validate return URL for security
   */
  isValidReturnUrl(url) {
    try {
      const parsedUrl = new URL(url);
      const allowedDomains = [
        'bluebandedbee.co',
        'app.bluebandedbee.co',
        'fly.dev',
        'localhost'
      ];
      
      return allowedDomains.some(domain => 
        parsedUrl.hostname === domain || 
        parsedUrl.hostname.endsWith('.' + domain)
      );
    } catch {
      return false;
    }
  }

  /**
   * Set authentication state
   */
  async setAuthState(user, session) {
    this.user = user;
    this.isAuthenticated = !!user;
    
    if (user && session) {
      // Register user with backend if needed
      await this.registerUserWithBackend(user, session);
    }
    
    // Notify all callbacks
    this.authStateCallbacks.forEach(callback => {
      try {
        callback(this.isAuthenticated, user, session);
      } catch (error) {
        console.error('Auth state callback error:', error);
      }
    });
    
    this.log('Auth state updated:', { isAuthenticated: this.isAuthenticated, email: user?.email });
  }

  /**
   * Register auth state change callback
   */
  onAuthStateChange(callback) {
    this.authStateCallbacks.push(callback);
    
    // Immediately call with current state
    if (this.supabase) {
      callback(this.isAuthenticated, this.user, null);
    }
    
    // Return unsubscribe function
    return () => {
      const index = this.authStateCallbacks.indexOf(callback);
      if (index > -1) {
        this.authStateCallbacks.splice(index, 1);
      }
    };
  }

  /**
   * Register user with backend database
   */
  async registerUserWithBackend(user, session) {
    if (!user?.id || !user?.email) {
      return false;
    }

    try {
      const response = await fetch(`${this.config.apiBaseUrl}/v1/auth/register`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${session.access_token}`
        },
        body: JSON.stringify({
          user_id: user.id,
          email: user.email,
          full_name: user.user_metadata?.full_name || null
        })
      });

      if (response.ok || response.status === 409) {
        this.log('User registered with backend');
        return true;
      } else {
        console.error('Backend registration failed:', response.status);
        return false;
      }
    } catch (error) {
      console.error('Backend registration error:', error);
      return false;
    }
  }

  /**
   * Sign in with email and password
   */
  async signInWithPassword(email, password) {
    try {
      const { data, error } = await this.supabase.auth.signInWithPassword({
        email,
        password
      });

      if (error) throw error;
      
      this.log('Email sign in successful');
      return { success: true, data };
    } catch (error) {
      this.log('Email sign in error:', error.message);
      return { success: false, error: error.message };
    }
  }

  /**
   * Sign up with email and password
   */
  async signUp(email, password, options = {}) {
    try {
      const { data, error } = await this.supabase.auth.signUp({
        email,
        password,
        options
      });

      if (error) throw error;
      
      this.log('Email sign up successful');
      return { success: true, data };
    } catch (error) {
      this.log('Email sign up error:', error.message);
      return { success: false, error: error.message };
    }
  }

  /**
   * Sign in with OAuth provider
   */
  async signInWithOAuth(provider, options = {}) {
    try {
      const currentUrl = window.location.href;
      const state = btoa(JSON.stringify({ 
        returnUrl: options.returnUrl || currentUrl,
        timestamp: Date.now()
      }));

      const { data, error } = await this.supabase.auth.signInWithOAuth({
        provider,
        options: {
          redirectTo: `${window.location.origin}/auth-callback.html`,
          queryParams: {
            state
          }
        }
      });

      if (error) throw error;
      
      this.log(`${provider} OAuth initiated`);
      return { success: true, data };
    } catch (error) {
      this.log(`${provider} OAuth error:`, error.message);
      return { success: false, error: error.message };
    }
  }

  /**
   * Reset password for email
   */
  async resetPassword(email) {
    try {
      const { error } = await this.supabase.auth.resetPasswordForEmail(email, {
        redirectTo: `${window.location.origin}/auth-callback.html`
      });

      if (error) throw error;
      
      this.log('Password reset email sent');
      return { success: true };
    } catch (error) {
      this.log('Password reset error:', error.message);
      return { success: false, error: error.message };
    }
  }

  /**
   * Sign out
   */
  async signOut() {
    try {
      const { error } = await this.supabase.auth.signOut();
      if (error) throw error;
      
      this.log('Sign out successful');
      return { success: true };
    } catch (error) {
      this.log('Sign out error:', error.message);
      return { success: false, error: error.message };
    }
  }

  /**
   * Store pending authentication action
   */
  setPendingAuth(action, data = {}) {
    this.pendingAuth = { action, data, timestamp: Date.now() };
    this.log('Pending auth set:', action);
  }

  /**
   * Handle pending authentication action
   */
  async handlePendingAuth() {
    if (!this.pendingAuth || !this.isAuthenticated) {
      return;
    }

    const { action, data } = this.pendingAuth;
    this.pendingAuth = null;

    try {
      switch (action) {
        case 'start-crawling':
          await this.handleStartCrawling(data);
          break;
        case 'redirect':
          if (data.url && this.isValidReturnUrl(data.url)) {
            window.location.href = data.url;
          }
          break;
        default:
          this.log('Unknown pending auth action:', action);
      }
    } catch (error) {
      console.error('Pending auth action error:', error);
    }
  }

  /**
   * Handle start crawling action after authentication
   */
  async handleStartCrawling(data) {
    if (!data.domain) {
      console.error('No domain provided for crawling');
      return;
    }

    try {
      const session = await this.supabase.auth.getSession();
      const response = await fetch(`${this.config.apiBaseUrl}/v1/jobs`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${session.data.session.access_token}`
        },
        body: JSON.stringify({
          domain: data.domain,
          use_sitemap: true,
          find_links: true,
          max_pages: data.maxPages || 0,
          concurrency: data.concurrency || 5
        })
      });

      if (!response.ok) {
        throw new Error(`Job creation failed: ${response.status}`);
      }

      this.log('Crawl job created successfully for:', data.domain);
      
      // Redirect to dashboard
      window.location.href = '/dashboard';
    } catch (error) {
      console.error('Start crawling error:', error);
      throw error;
    }
  }

  /**
   * Get current session
   */
  async getSession() {
    if (!this.supabase) {
      return null;
    }
    
    const { data } = await this.supabase.auth.getSession();
    return data.session;
  }

  /**
   * Debug logging
   */
  log(...args) {
    if (this.config.debug) {
      console.log('[AuthManager]', ...args);
    }
  }
}

// Export for module systems and global access
if (typeof module !== 'undefined' && module.exports) {
  module.exports = AuthManager;
} else {
  window.AuthManager = AuthManager;
}
/**
 * Simple authentication manager without external dependencies
 * Expects Supabase to be loaded via CDN
 */

import { api } from '../utils/api.js';

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
    // Check if Supabase is already loaded and client is created
    if (window.supabase && typeof window.supabase.createClient === 'function') {
      // Create the client with the correct credentials
      this.supabase = window.supabase.createClient(
        "https://auth.bluebandedbee.co",
        "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imdwemp0Ymd0ZGp4bmFjZGZ1anZ4Iiwicm9sZSI6ImFub24iLCJpYXQiOjE3NDUwNjYxNjMsImV4cCI6MjA2MDY0MjE2M30.eJjM2-3X8oXsFex_lQKvFkP1-_yLMHsueIn7_hCF6YI"
      );
      return;
    }

    // Wait for it to load
    return new Promise((resolve) => {
      const checkSupabase = () => {
        if (window.supabase && typeof window.supabase.createClient === 'function') {
          this.supabase = window.supabase.createClient(
            "https://auth.bluebandedbee.co",
            "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imdwemp0Ymd0ZGp4bmFjZGZ1anZ4Iiwicm9sZSI6ImFub24iLCJpYXQiOjE3NDUwNjYxNjMsImV4cCI6MjA2MDY0MjE2M30.eJjM2-3X8oXsFex_lQKvFkP1-_yLMHsueIn7_hCF6YI"
          );
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
export const authManager = new SimpleAuthManager();

// Utility function for components
export function requireAuth(component) {
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
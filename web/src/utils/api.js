/**
 * API utility for Blue Banded Bee components
 */

const API_BASE = window.location.hostname === 'localhost' 
  ? 'http://localhost:8080'
  : 'https://app.bluebandedbee.co';

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

    // Get current token from auth manager if available
    if (window.BBComponents?.authManager) {
      const session = await window.BBComponents.authManager.getSession();
      if (session?.access_token) {
        headers.Authorization = `Bearer ${session.access_token}`;
      }
    } else if (this.token) {
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

export const api = new BBApi();
/**
 * Blue Banded Bee Authentication Extension
 * Data binding integration for the unified authentication system
 *
 * This module provides integration between the core auth system (auth.js)
 * and the BBDataBinder for seamless authentication in dashboard applications.
 *
 * Features:
 * - Auth state monitoring and data binder integration
 * - Automatic dashboard refresh on auth state changes
 * - Pending domain handling for homepage-to-dashboard flow
 * - Network status monitoring
 * - Dashboard-specific auth UI updates
 */

/**
 * Initialise authentication with data binder integration
 * @param {Object} dataBinder - BBDataBinder instance
 * @param {Object} options - Configuration options
 * @returns {Promise<void>}
 */
async function initializeAuthWithDataBinder(dataBinder, options = {}) {
  const {
    debug = false,
    autoRefresh = true,
    networkMonitoring = true,
  } = options;

  // Ensure Supabase is ready
  if (!window.BBAuth.initializeSupabase()) {
    console.error("Supabase not available");
    return;
  }

  // Handle auth callback tokens
  const hasToken = await window.BBAuth.handleAuthCallback();

  // Check if user exists in backend on page load
  const {
    data: { session },
  } = await supabase.auth.getSession();

  if (session?.user) {
    await window.BBAuth.registerUserWithBackend(session.user);
  }

  // Update user info in header
  window.BBAuth.updateUserInfo();

  // Set initial auth state
  window.BBAuth.updateAuthState(!!session?.user);

  // Set up auth state change listener to update user info
  if (window.supabase) {
    window.supabase.auth.onAuthStateChange(async (event, session) => {
      if (debug) {
        console.log("Auth state changed:", event, session?.user?.email);
      }

      // Register user with backend on sign in (handles OAuth returns)
      if (
        (event === "SIGNED_IN" || event === "USER_UPDATED") &&
        session?.user
      ) {
        await window.BBAuth.registerUserWithBackend(session.user);
      }

      // Update auth state in UI
      window.BBAuth.updateAuthState(!!session?.user);

      // Wait a moment for the data binder to update its auth manager
      setTimeout(() => window.BBAuth.updateUserInfo(), 100);

      // Handle pending domain after successful auth
      if (session?.user) {
        await window.BBAuth.handlePendingDomain();
      }
    });
  }

  // Log auth state for debugging
  if (debug) {
    console.log("Auth state after init:", {
      hasAuth: !!dataBinder.authManager,
      isAuthenticated: dataBinder.authManager?.isAuthenticated,
      user: dataBinder.authManager?.user?.email,
    });
  }

  // Update auth state after data binder init
  const currentSession = await window.supabase.auth.getSession();
  window.BBAuth.updateAuthState(!!currentSession.data.session?.user);

  // Set up network monitoring if enabled
  if (networkMonitoring) {
    setupNetworkMonitoring(dataBinder);
  }
}

/**
 * Setup dashboard-specific refresh method override
 * @param {Object} dataBinder - BBDataBinder instance
 */
function setupDashboardRefresh(dataBinder) {
  // Override the refresh method to load dashboard data
  dataBinder.refresh = async function () {
    // Only load dashboard data if user is authenticated
    if (!this.authManager || !this.authManager.isAuthenticated) {
      console.log("User not authenticated, skipping dashboard data load");
      return;
    }

    try {
      console.log("Refreshing dashboard data...");

      // Show refresh indicator
      const statusIndicator = document.querySelector(".status-indicator");
      if (statusIndicator) {
        statusIndicator.innerHTML =
          '<span class="status-dot"></span><span>Refreshing...</span>';
      }

      // Load stats and jobs data
      let data;
      try {
        data = await this.loadAndBind({
          stats: "/v1/dashboard/stats?range=today",
        });
      } catch (error) {
        // Handle stats API errors gracefully
        console.log("Stats API error (likely no data yet):", error);
        data = {
          stats: {
            total_jobs: 0,
            running_jobs: 0,
            completed_jobs: 0,
            failed_jobs: 0,
          },
        };
      }

      // Load jobs separately for template binding
      let jobsResponse, jobs;
      try {
        jobsResponse = await this.fetchData("/v1/jobs?limit=10&range=today");
        jobs = jobsResponse.jobs || [];
      } catch (error) {
        console.log("Jobs API error (likely no jobs yet):", error);
        jobs = [];
      }

      // Process jobs data for better display
      const processedJobs = jobs.map((job) => ({
        ...job,
        domain: job.domains?.name || "Unknown Domain",
        progress: Math.round(job.progress || 0),
        started_at_formatted: job.started_at
          ? new Date(job.started_at).toLocaleString()
          : "-",
      }));

      // Load slow pages data
      let slowPagesResponse, slowPages;
      try {
        slowPagesResponse = await this.fetchData(
          "/v1/dashboard/slow-pages?range=today"
        );
        slowPages = slowPagesResponse.slow_pages || [];
      } catch (error) {
        console.log("Slow pages API error (likely no data yet):", error);
        slowPages = [];
      }

      // Process slow pages data for better display
      const processedSlowPages = slowPages.map((page) => ({
        ...page,
        completed_at: page.completed_at
          ? new Date(page.completed_at).toLocaleString()
          : "-",
      }));

      // Load external redirects data
      let redirectsResponse, externalRedirects;
      try {
        redirectsResponse = await this.fetchData(
          "/v1/dashboard/external-redirects?range=today"
        );
        externalRedirects = redirectsResponse.external_redirects || [];
      } catch (error) {
        console.log("Redirects API error (likely no data yet):", error);
        externalRedirects = [];
      }

      // Process external redirects data for better display
      const processedRedirects = externalRedirects.map((redirect) => ({
        ...redirect,
        completed_at: redirect.completed_at
          ? new Date(redirect.completed_at).toLocaleString()
          : "-",
      }));

      // Bind all templates
      this.bindTemplates({
        job: processedJobs,
        slow_page: processedSlowPages,
        external_redirect: processedRedirects,
      });

      // Show simple empty state if no jobs
      if (processedJobs.length === 0) {
        const jobsList = document.querySelector(".bb-jobs-list");
        if (jobsList) {
          jobsList.innerHTML = `
            <div style="text-align: center; padding: 40px 20px; color: #6b7280;">
              <div style="font-size: 48px; margin-bottom: 16px;">🐝</div>
              <h3 style="margin: 0 0 8px 0; color: #374151;">No jobs yet</h3>
              <p style="margin: 0; font-size: 14px;">Use the form above to start your first cache warming job</p>
            </div>
          `;
        }
      } else {
        // Update job action visibility and visual states
        setTimeout(() => {
          if (window.updateJobActionVisibility) {
            window.updateJobActionVisibility();
          }
          if (window.updateJobVisualStates) {
            window.updateJobVisualStates();
          }
        }, 100); // Small delay to ensure DOM updates are complete
      }

      console.log("Dashboard data refreshed", {
        stats: data.stats,
        jobs: processedJobs.length,
      });

      // Load metrics metadata after successful data load (only once)
      if (window.metricsMetadata && !window.metricsMetadata.isLoaded()) {
        try {
          await window.metricsMetadata.load();
          window.metricsMetadata.initializeInfoIcons();
        } catch (metadataError) {
          console.warn(
            "Failed to load metrics metadata (non-critical):",
            metadataError
          );
        }
      }
    } catch (error) {
      console.error("Dashboard refresh failed:", error);

      // Only show error if it's not a 404 or empty data response
      if (error.status !== 404 && !error.message?.includes("No jobs found")) {
        if (window.showDashboardError) {
          window.showDashboardError(
            "Unable to refresh dashboard data. Please check your connection and try again."
          );
        }
      }

      // Set error state for stats only if there's a real error
      if (error.status !== 404) {
        this.updateElements({
          stats: {
            total_jobs: "–",
            running_jobs: "–",
            completed_jobs: "–",
            failed_jobs: "–",
          },
        });
      } else {
        // For 404/no data, show zero stats instead of error state
        this.updateElements({
          stats: {
            total_jobs: "0",
            running_jobs: "0",
            completed_jobs: "0",
            failed_jobs: "0",
          },
        });
      }
    } finally {
      // Reset status indicator
      const statusIndicator = document.querySelector(".status-indicator");
      if (statusIndicator) {
        statusIndicator.innerHTML =
          '<span class="status-dot"></span><span>Live</span>';
      }
    }
  };
}

/**
 * Setup dashboard form handler for job creation
 */
function setupDashboardFormHandler() {
  const dashboardForm = document.getElementById("dashboardJobForm");
  if (dashboardForm) {
    dashboardForm.addEventListener("submit", handleDashboardJobCreation);
  }
}

/**
 * Handle dashboard job creation form
 * @param {Event} event - Form submit event
 */
async function handleDashboardJobCreation(event) {
  event.preventDefault();
  const formData = new FormData(event.target);

  const domain = formData.get("domain");
  const maxPages = parseInt(formData.get("max_pages"));

  // Basic validation
  if (!domain) {
    if (window.showDashboardError) {
      window.showDashboardError("Domain is required");
    }
    return;
  }

  if (maxPages < 0 || maxPages > 10000) {
    if (window.showDashboardError) {
      window.showDashboardError("Maximum pages must be between 0 and 10000");
    }
    return;
  }

  try {
    console.log("Creating job from dashboard form:", { domain, maxPages });

    const response = await window.dataBinder.fetchData("/v1/jobs", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        domain: domain,
        max_pages: maxPages,
        concurrency: 5,
        use_sitemap: true,
        find_links: true,
      }),
    });

    console.log("Dashboard job created successfully:", response);

    // Clear the form
    const domainField = document.getElementById("dashboardDomain");
    const maxPagesField = document.getElementById("dashboardMaxPages");
    if (domainField) domainField.value = "";
    if (maxPagesField) maxPagesField.value = "0";

    // Refresh dashboard to show new job
    if (window.dataBinder) {
      await window.dataBinder.refresh();
    }

    // Show success message
    if (window.showSuccessMessage) {
      window.showSuccessMessage(`Job created successfully for ${domain}`);
    }
  } catch (error) {
    console.error("Failed to create dashboard job:", error);
    if (window.showDashboardError) {
      window.showDashboardError(
        error.message || "Failed to create job. Please try again."
      );
    }
  }
}

/**
 * Setup network status monitoring
 * @param {Object} dataBinder - BBDataBinder instance
 */
function setupNetworkMonitoring(dataBinder) {
  // Check initial network status
  updateNetworkStatus();

  // Listen for network status changes
  window.addEventListener("online", () => {
    updateNetworkStatus();
    if (window.showInfoMessage) {
      window.showInfoMessage("Connection restored. Refreshing data...", 2000);
    }
    setTimeout(() => {
      if (dataBinder) {
        dataBinder.refresh();
      }
    }, 500);
  });

  window.addEventListener("offline", () => {
    updateNetworkStatus();
    if (window.showDashboardError) {
      window.showDashboardError(
        "Connection lost. Some features may not work.",
        "error",
        0
      );
    }
  });
}

/**
 * Update network status indicator
 */
function updateNetworkStatus() {
  const statusIndicator = document.querySelector(".status-indicator");
  if (statusIndicator && !navigator.onLine) {
    statusIndicator.innerHTML =
      '<span style="background: #ef4444;" class="status-dot"></span><span>Offline</span>';
  } else if (statusIndicator && navigator.onLine) {
    statusIndicator.innerHTML =
      '<span class="status-dot"></span><span>Live</span>';
  }
}

/**
 * Enhanced dashboard initialisation with full auth integration
 * @param {Object} config - Configuration options
 * @returns {Promise<Object>} Initialised data binder
 */
async function initializeDashboard(config = {}) {
  const {
    debug = false,
    refreshInterval = 10,
    apiBaseUrl = "",
    autoRefresh = true,
    networkMonitoring = true,
  } = config;

  console.log("Enhanced dashboard initialising...");

  // Load the shared authentication modal
  await window.BBAuth.loadAuthModal();

  // Wait for auth modal DOM to be ready
  await new Promise((resolve) => setTimeout(resolve, 50));

  // Create data binder with production config
  const dataBinder = new BBDataBinder({
    apiBaseUrl,
    debug,
    refreshInterval: autoRefresh ? refreshInterval : 0, // Disable auto-refresh if not wanted
  });

  // Expose the binder globally so shared handlers (e.g. auth, forms) can reuse the instance
  if (typeof window !== "undefined") {
    window.dataBinder = dataBinder;
  }

  // Initialise auth with data binder integration
  await initializeAuthWithDataBinder(dataBinder, {
    debug,
    autoRefresh,
    networkMonitoring,
  });

  // Initialise data binder
  await dataBinder.init();

  // Setup dashboard-specific refresh method
  setupDashboardRefresh(dataBinder);

  // Setup dashboard form handler
  setupDashboardFormHandler();

  // Setup authentication event handlers
  window.BBAuth.setupAuthHandlers();

  // Setup login page handlers
  window.BBAuth.setupLoginPageHandlers();

  // Initial load (only if authenticated)
  if (autoRefresh) {
    await dataBinder.refresh();
  }

  console.log("Enhanced dashboard initialised");

  return dataBinder;
}

/**
 * Quick setup function for basic auth integration
 * @param {Object} dataBinder - Existing BBDataBinder instance
 */
async function setupQuickAuth(dataBinder) {
  // Load auth modal
  await window.BBAuth.loadAuthModal();

  // Wait for DOM to be ready
  await new Promise((resolve) => setTimeout(resolve, 50));

  // Initialise auth
  await initializeAuthWithDataBinder(dataBinder, { debug: false });

  // Setup handlers
  window.BBAuth.setupAuthHandlers();
  window.BBAuth.setupLoginPageHandlers();

  console.log("Quick auth setup complete");
}

// Export functions for use by other modules
if (typeof module !== "undefined" && module.exports) {
  // Node.js environment
  module.exports = {
    initializeAuthWithDataBinder,
    setupDashboardRefresh,
    setupDashboardFormHandler,
    handleDashboardJobCreation,
    setupNetworkMonitoring,
    updateNetworkStatus,
    initializeDashboard,
    setupQuickAuth,
  };
} else {
  // Browser environment - make functions globally available
  window.BBAuthExtension = {
    initializeAuthWithDataBinder,
    setupDashboardRefresh,
    setupDashboardFormHandler,
    handleDashboardJobCreation,
    setupNetworkMonitoring,
    updateNetworkStatus,
    initializeDashboard,
    setupQuickAuth,
  };

  // Also make individual functions available globally for convenience
  window.initializeAuthWithDataBinder = initializeAuthWithDataBinder;
  window.setupDashboardRefresh = setupDashboardRefresh;
  window.setupDashboardFormHandler = setupDashboardFormHandler;
  window.handleDashboardJobCreation = handleDashboardJobCreation;
  window.setupNetworkMonitoring = setupNetworkMonitoring;
  window.updateNetworkStatus = updateNetworkStatus;
  window.initializeDashboard = initializeDashboard;
  window.setupQuickAuth = setupQuickAuth;
}

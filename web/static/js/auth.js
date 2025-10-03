/**
 * Blue Banded Bee Unified Authentication System
 * Core authentication logic extracted from dashboard.html
 *
 * Handles:
 * - Supabase authentication integration
 * - Email/password authentication
 * - Social login (Google, GitHub)
 * - Password strength validation with zxcvbn
 * - Cloudflare Turnstile CAPTCHA
 * - Backend user registration
 * - Auth state management
 * - Modal management
 * - Pending domain flow
 * - Sentry error tracking for auth failures
 */

// Supabase configuration
const SUPABASE_URL = "https://auth.bluebandedbee.co";
const SUPABASE_ANON_KEY = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imdwemp0Ymd0ZGp4bmFjZGZ1anZ4Iiwicm9sZSI6ImFub24iLCJpYXQiOjE3NDUwNjYxNjMsImV4cCI6MjA2MDY0MjE2M30.eJjM2-3X8oXsFex_lQKvFkP1-_yLMHsueIn7_hCF6YI";

// Global state
let supabase;
let captchaToken = null;

/**
 * Initialise Supabase client
 * @returns {boolean} Success status
 */
function initializeSupabase() {
  if (window.supabase && window.supabase.createClient) {
    supabase = window.supabase.createClient(SUPABASE_URL, SUPABASE_ANON_KEY);
    window.supabase = supabase; // Ensure it's globally available
    console.log("Supabase client created successfully");
    return true;
  }
  return false;
}

/**
 * Load shared authentication modal HTML
 */
async function loadAuthModal() {
  try {
    console.log("Loading auth modal...");
    const response = await fetch("/auth-modal.html");

    if (!response.ok) {
      throw new Error(`Failed to fetch auth modal: ${response.status} ${response.statusText}`);
    }

    const modalHTML = await response.text();
    console.log("Auth modal HTML loaded, length:", modalHTML.length);

    document.getElementById("authModalContainer").innerHTML = modalHTML;

    // Verify the modal was inserted
    const authModal = document.getElementById("authModal");
    console.log("Auth modal element after insertion:", authModal);

    // Set default to login form for dashboard
    setTimeout(() => {
      if (window.showLoginForm) {
        showLoginForm();
      }
    }, 10);
  } catch (error) {
    console.error("Failed to load auth modal:", error);
    if (window.Sentry) {
      window.Sentry.captureException(error, {
        tags: { component: 'auth', action: 'load_modal' }
      });
    }
  }
}

/**
 * Handle authentication callback tokens from OAuth redirects
 * @returns {Promise<boolean>} Whether tokens were processed
 */
async function handleAuthCallback() {
  try {
    // Check for error parameters in URL (from OAuth failures)
    const urlParams = new URLSearchParams(window.location.search);
    const error = urlParams.get("error");
    const errorDescription = urlParams.get("error_description");
    
    if (error) {
      console.error("OAuth error:", error, errorDescription);
      // Clear error from URL
      history.replaceState(null, null, window.location.pathname);
      // Show error to user
      if (window.showAuthError) {
        showAuthError("Authentication failed. Please try again.");
      }
      return false;
    }

    // Check if we have auth tokens in the URL hash
    const hashParams = new URLSearchParams(window.location.hash.substring(1));
    const accessToken = hashParams.get("access_token");
    const refreshToken = hashParams.get("refresh_token");

    if (accessToken) {
      console.log("Processing auth callback with tokens...");

      // Set the session in Supabase using the tokens
      const {
        data: { session },
        error,
      } = await supabase.auth.setSession({
        access_token: accessToken,
        refresh_token: refreshToken,
      });

      if (session) {
        console.log("User authenticated via callback:", session.user.email);

        // Clear the URL hash to clean up the URL
        history.replaceState(null, null, window.location.pathname);

        // Update user info will be called after dataBinder init
        return true;
      } else if (error) {
        console.error("Auth session setup error:", error);
      }
    } else {
      // Check if already authenticated
      const {
        data: { session },
      } = await supabase.auth.getSession();
      if (session) {
        console.log("User already authenticated:", session.user.email);
        return true;
      }
    }

    return false;
  } catch (error) {
    console.error("Auth callback processing error:", error);
    if (window.Sentry) {
      window.Sentry.captureException(error, {
        tags: { component: 'auth', action: 'process_callback' }
      });
    }
    return false;
  }
}

/**
 * Register user with backend database
 * @param {Object} user - Supabase user object
 * @returns {Promise<boolean>} Success status
 */
async function registerUserWithBackend(user) {
  if (!user || !user.id || !user.email) {
    console.error("Invalid user data for registration");
    return false;
  }

  try {
    const session = await supabase.auth.getSession();
    if (!session.data.session) {
      console.error("No session available for registration");
      return false;
    }

    const response = await fetch("/v1/auth/register", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${session.data.session.access_token}`,
      },
      body: JSON.stringify({
        user_id: user.id,
        email: user.email,
        full_name: user.user_metadata?.full_name || null,
      }),
    });

    if (!response.ok) {
      // If user already exists (409 Conflict), that's fine
      if (response.status === 409) {
        console.log("User already registered in backend");
        return true;
      }
      const errorData = await response.json();
      console.error("Backend registration failed:", errorData);
      return false;
    }

    const data = await response.json();
    console.log("User registered with backend:", data);
    return true;
  } catch (error) {
    console.error("Failed to register user with backend:", error);
    if (window.Sentry) {
      window.Sentry.captureException(error, {
        tags: { component: 'auth', action: 'backend_registration' },
        level: 'error'
      });
    }
    return false;
  }
}

/**
 * Update UI elements based on authentication state
 * @param {boolean} isAuthenticated - Whether user is authenticated
 */
function updateAuthState(isAuthenticated) {
  console.log("Updating auth state:", isAuthenticated);

  // Show/hide elements based on authentication
  const guestElements = document.querySelectorAll('[data-bb-auth="guest"]');
  const requiredElements = document.querySelectorAll('[data-bb-auth="required"]');

  guestElements.forEach((el) => {
    el.style.display = isAuthenticated ? "none" : el.dataset.originalDisplay || "block";
  });

  requiredElements.forEach((el) => {
    if (!isAuthenticated) {
      el.dataset.originalDisplay = el.style.display || "block";
    }
    el.style.display = isAuthenticated ? el.dataset.originalDisplay || "block" : "none";
  });

  // If user just authenticated and dataBinder exists, load dashboard data
  if (isAuthenticated && window.dataBinder) {
    setTimeout(() => window.dataBinder.refresh(), 100);
  }
  
  // Re-setup logout handler after elements become visible
  if (isAuthenticated) {
    setTimeout(() => {
      const logoutBtn = document.getElementById("logoutBtn");
      if (logoutBtn && !logoutBtn.hasAttribute("data-logout-handler-attached")) {
        logoutBtn.addEventListener("click", async () => {
          try {
            const { error } = await supabase.auth.signOut();
            if (error) {
              console.error("Logout error:", error);
              alert("Logout failed. Please try again.");
            } else {
              console.log("Logout successful");
              window.location.reload();
            }
          } catch (error) {
            console.error("Logout error:", error);
            alert("Logout failed. Please try again.");
          }
        });
        logoutBtn.setAttribute("data-logout-handler-attached", "true");
        console.log("Logout handler attached to visible button");
      }
    }, 150);
  }
}

/**
 * Update user info in header elements
 */
async function updateUserInfo() {
  const userEmailElement = document.getElementById("userEmail");
  const userAvatarElement = document.getElementById("userAvatar");

  if (!userEmailElement || !userAvatarElement) return;

  try {
    // Get current user from Supabase directly
    const {
      data: { session },
    } = await supabase.auth.getSession();

    if (session && session.user && session.user.email) {
      const email = session.user.email;

      // Update email display
      userEmailElement.textContent = email;

      // Update avatar with user initials
      const initials = getInitials(email);
      userAvatarElement.textContent = initials;

      console.log("User info updated:", email);
    } else {
      // No session, reset to defaults
      userEmailElement.textContent = "Loading...";
      userAvatarElement.textContent = "?";
    }
  } catch (error) {
    console.error("Failed to update user info:", error);
    userEmailElement.textContent = "Error";
    userAvatarElement.textContent = "?";
  }
}

/**
 * Generate initials from email address
 * @param {string} email - Email address
 * @returns {string} User initials
 */
function getInitials(email) {
  if (!email) return "?";

  // Try to get name parts from email or use email prefix
  const emailPrefix = email.split("@")[0];

  // Check if email has recognisable name patterns (firstname.lastname, etc.)
  if (emailPrefix.includes(".")) {
    const parts = emailPrefix.split(".");
    return parts
      .map((part) => part.charAt(0).toUpperCase())
      .slice(0, 2)
      .join("");
  } else if (emailPrefix.includes("_")) {
    const parts = emailPrefix.split("_");
    return parts
      .map((part) => part.charAt(0).toUpperCase())
      .slice(0, 2)
      .join("");
  } else {
    // Just use first two characters of email prefix
    return emailPrefix.slice(0, 2).toUpperCase();
  }
}

/**
 * Show authentication modal
 */
function showAuthModal() {
  const authModal = document.getElementById("authModal");
  if (authModal) {
    authModal.classList.add("show");
    showAuthForm("login");
  }
}

/**
 * Close authentication modal
 */
function closeAuthModal() {
  const authModal = document.getElementById("authModal");
  if (authModal) {
    authModal.classList.remove("show");
    clearAuthError();
    hideAuthLoading();
  }
}

/**
 * Show specific authentication form
 * @param {string} formType - Type of form to show: 'login', 'signup', 'reset'
 */
function showAuthForm(formType) {
  // Hide all forms
  const loginForm = document.getElementById("loginForm");
  const signupForm = document.getElementById("signupForm");
  const resetForm = document.getElementById("resetForm");
  
  if (loginForm) loginForm.style.display = "none";
  if (signupForm) signupForm.style.display = "none";
  if (resetForm) resetForm.style.display = "none";

  // Show selected form
  const titles = {
    login: "Sign In",
    signup: "Create Account",
    reset: "Reset Password",
  };

  const authModalTitle = document.getElementById("authModalTitle");
  if (authModalTitle) {
    authModalTitle.textContent = titles[formType];
  }

  const targetForm = document.getElementById(`${formType}Form`);
  if (targetForm) {
    targetForm.style.display = "block";
  }

  // Reset CAPTCHA state for signup form
  if (formType === "signup") {
    captchaToken = null;
    const signupBtn = document.getElementById("signupSubmitBtn");
    if (signupBtn) {
      signupBtn.disabled = true;
    }
    // Reset Turnstile widget if it exists
    if (window.turnstile) {
      const turnstileWidget = document.querySelector(".cf-turnstile");
      if (turnstileWidget) {
        window.turnstile.reset(turnstileWidget);
      }
    }
  }

  clearAuthError();
  hideAuthLoading();
}

/**
 * Handle email/password login
 * @param {Event} event - Form submit event
 */
async function handleEmailLogin(event) {
  event.preventDefault();
  const formData = new FormData(event.target);
  const email = formData.get("email");
  const password = formData.get("password");

  showAuthLoading();
  clearAuthError();

  try {
    const { data, error } = await supabase.auth.signInWithPassword({
      email,
      password,
    });

    if (error) throw error;

    console.log("Email login successful:", data.user.email);

    // Register user with backend (in case they don't exist)
    await registerUserWithBackend(data.user);

    closeAuthModal();

    // Update user info immediately
    updateUserInfo();

    // Update auth state
    updateAuthState(true);

    // Refresh dashboard data if dataBinder exists
    if (window.dataBinder) {
      await window.dataBinder.refresh();
    }

    // Handle any pending domain
    await handlePendingDomain();
  } catch (error) {
    console.error("Email login error:", error);
    if (window.Sentry) {
      window.Sentry.captureException(error, {
        tags: { component: 'auth', action: 'email_login' },
        level: 'warning'
      });
    }
    showAuthError(error.message || "Login failed. Please check your credentials.");
  } finally {
    hideAuthLoading();
  }
}

/**
 * Handle email signup
 * @param {Event} event - Form submit event
 */
async function handleEmailSignup(event) {
  event.preventDefault();
  const formData = new FormData(event.target);
  const email = formData.get("email");
  const password = formData.get("password");
  const passwordConfirm = formData.get("passwordConfirm");

  if (password !== passwordConfirm) {
    showAuthError("Passwords do not match.");
    return;
  }

  if (password.length < 6) {
    showAuthError("Password must be at least 6 characters long.");
    return;
  }

  if (!captchaToken) {
    showAuthError("Please complete the CAPTCHA verification.");
    return;
  }

  showAuthLoading();
  clearAuthError();

  try {
    const { data, error } = await supabase.auth.signUp({
      email,
      password,
      options: { captchaToken },
    });

    if (error) throw error;

    console.log("Email signup successful:", data.user?.email);

    if (data.user && !data.user.email_confirmed_at) {
      showAuthError("Please check your email and click the confirmation link before signing in.");
      showAuthForm("login");
    } else if (data.user) {
      // Register user with backend
      await registerUserWithBackend(data.user);

      closeAuthModal();
      // Update user info and refresh dashboard
      updateUserInfo();
      updateAuthState(true);
      if (window.dataBinder) {
        await window.dataBinder.refresh();
      }
      await handlePendingDomain();
    }
  } catch (error) {
    console.error("Email signup error:", error);
    if (window.Sentry) {
      window.Sentry.captureException(error, {
        tags: { component: 'auth', action: 'email_signup' },
        level: 'warning'
      });
    }
    showAuthError(error.message || "Signup failed. Please try again.");
  } finally {
    hideAuthLoading();
  }
}

/**
 * Handle password reset
 * @param {Event} event - Form submit event
 */
async function handlePasswordReset(event) {
  event.preventDefault();
  const formData = new FormData(event.target);
  const email = formData.get("email");

  showAuthLoading();
  clearAuthError();

  try {
    const { error } = await supabase.auth.resetPasswordForEmail(email, {
      redirectTo: `${window.location.origin}/dashboard`,
    });

    if (error) throw error;

    showAuthError("Password reset email sent! Check your inbox.", "success");
    setTimeout(() => {
      showAuthForm("login");
    }, 2000);
  } catch (error) {
    console.error("Password reset error:", error);
    showAuthError(error.message || "Failed to send reset email.");
  } finally {
    hideAuthLoading();
  }
}

/**
 * Handle social login (Google, GitHub)
 * @param {string} provider - OAuth provider name
 */
async function handleSocialLogin(provider) {
  showAuthLoading();
  clearAuthError();

  try {
    const { data, error } = await supabase.auth.signInWithOAuth({
      provider,
      options: {
        redirectTo: `${window.location.origin}/dashboard`,
      },
    });

    if (error) throw error;

    // OAuth will redirect, so no need to handle success here
  } catch (error) {
    console.error("Social login error:", error);
    if (window.Sentry) {
      window.Sentry.captureException(error, {
        tags: { component: 'auth', action: 'social_login', provider: provider },
        level: 'warning'
      });
    }
    showAuthError(error.message || `${provider} login failed.`);
    hideAuthLoading();
  }
}

/**
 * Handle pending domain after authentication
 */
async function handlePendingDomain() {
  const pendingDomain = sessionStorage.getItem("bb_pending_domain");
  if (pendingDomain && window.dataBinder?.authManager?.isAuthenticated) {
    console.log("Found pending domain after auth:", pendingDomain);

    // Clear the stored domain
    sessionStorage.removeItem("bb_pending_domain");

    // Auto-create job
    try {
      const response = await window.dataBinder.fetchData("/v1/jobs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          domain: pendingDomain,
          use_sitemap: true,
          find_links: true,
          max_pages: 0,
          concurrency: 5,
        }),
      });

      if (window.showSuccessMessage) {
        window.showSuccessMessage(`Started crawling ${pendingDomain}! Your cache warming has begun.`);
      }

      // Refresh dashboard to show new job
      if (window.dataBinder) {
        await window.dataBinder.refresh();
      }
    } catch (error) {
      console.error("Failed to create job from pending domain:", error);
      if (window.showDashboardError) {
        window.showDashboardError(`Failed to start crawling ${pendingDomain}. Please try creating the job manually.`);
      }
    }
  }
}

/**
 * Show authentication loading state
 */
function showAuthLoading() {
  const authLoading = document.getElementById("authLoading");
  const visibleForm = document.querySelector('.bb-auth-form:not([style*="display: none"])');
  
  if (authLoading) {
    authLoading.style.display = "block";
  }
  if (visibleForm) {
    visibleForm.style.display = "none";
  }
}

/**
 * Hide authentication loading state
 */
function hideAuthLoading() {
  const authLoading = document.getElementById("authLoading");
  if (authLoading) {
    authLoading.style.display = "none";
  }
  
  // Show appropriate form based on current title
  const authModalTitle = document.getElementById("authModalTitle");
  if (authModalTitle) {
    const title = authModalTitle.textContent;
    if (title === "Sign In") {
      const loginForm = document.getElementById("loginForm");
      if (loginForm) loginForm.style.display = "block";
    } else if (title === "Create Account") {
      const signupForm = document.getElementById("signupForm");
      if (signupForm) signupForm.style.display = "block";
    } else if (title === "Reset Password") {
      const resetForm = document.getElementById("resetForm");
      if (resetForm) resetForm.style.display = "block";
    }
  }
}

/**
 * Show authentication error message
 * @param {string} message - Error message to display
 * @param {string} type - Message type: 'error' or 'success'
 */
function showAuthError(message, type = "error") {
  const errorDiv = document.getElementById("authError");
  if (errorDiv) {
    errorDiv.textContent = message;
    errorDiv.style.display = "block";

    if (type === "success") {
      errorDiv.style.background = "#dcfce7";
      errorDiv.style.color = "#16a34a";
      errorDiv.style.borderColor = "#bbf7d0";
    } else {
      errorDiv.style.background = "#fee2e2";
      errorDiv.style.color = "#dc2626";
      errorDiv.style.borderColor = "#fecaca";
    }
  }
}

/**
 * Clear authentication error message
 */
function clearAuthError() {
  const errorDiv = document.getElementById("authError");
  if (errorDiv) {
    errorDiv.style.display = "none";
  }
}

/**
 * Setup password strength validation using zxcvbn
 */
function setupPasswordStrength() {
  const passwordInput = document.getElementById("signupPassword");
  const confirmInput = document.getElementById("signupPasswordConfirm");
  const strengthIndicator = document.getElementById("passwordStrength");
  const strengthFill = document.getElementById("strengthFill");
  const strengthText = document.getElementById("strengthText");
  const strengthFeedback = document.getElementById("strengthFeedback");

  if (!passwordInput || typeof zxcvbn === "undefined") {
    console.warn("Password strength checking not available");
    return;
  }

  // Show strength indicator when password field gets focus
  passwordInput.addEventListener("focus", () => {
    if (strengthIndicator) {
      strengthIndicator.style.display = "block";
    }
  });

  // Real-time password strength checking
  passwordInput.addEventListener("input", (e) => {
    const password = e.target.value;

    if (password.length === 0) {
      if (strengthIndicator) {
        strengthIndicator.style.display = "none";
      }
      return;
    }

    if (strengthIndicator) {
      strengthIndicator.style.display = "block";
    }

    // Use zxcvbn to evaluate password strength
    const result = zxcvbn(password);
    const score = result.score; // 0-4 scale

    // Clear previous classes
    if (strengthFill) {
      strengthFill.className = "bb-strength-fill";
    }
    if (strengthText) {
      strengthText.className = "bb-strength-text";
    }

    // Apply strength classes and text
    let strengthLabel = "";
    switch (score) {
      case 0:
      case 1:
        if (strengthFill) strengthFill.classList.add("weak");
        if (strengthText) strengthText.classList.add("weak");
        strengthLabel = "Weak";
        break;
      case 2:
        if (strengthFill) strengthFill.classList.add("fair");
        if (strengthText) strengthText.classList.add("fair");
        strengthLabel = "Fair";
        break;
      case 3:
        if (strengthFill) strengthFill.classList.add("good");
        if (strengthText) strengthText.classList.add("good");
        strengthLabel = "Good";
        break;
      case 4:
        if (strengthFill) strengthFill.classList.add("strong");
        if (strengthText) strengthText.classList.add("strong");
        strengthLabel = "Strong";
        break;
    }

    if (strengthText) {
      strengthText.textContent = `Password strength: ${strengthLabel}`;
    }

    // Show feedback and suggestions
    let feedback = "";
    if (result.feedback.warning) {
      feedback += result.feedback.warning + ". ";
    }
    if (result.feedback.suggestions.length > 0) {
      feedback += result.feedback.suggestions.join(". ");
    }
    if (password.length < 8) {
      feedback = "Password must be at least 8 characters long. " + feedback;
    }

    if (strengthFeedback) {
      strengthFeedback.textContent = feedback;
    }

    // Validate confirm password if it has content
    if (confirmInput && confirmInput.value) {
      validatePasswordMatch();
    }
  });

  // Real-time password confirmation checking
  if (confirmInput) {
    confirmInput.addEventListener("input", validatePasswordMatch);
  }

  function validatePasswordMatch() {
    const password = passwordInput.value;
    const confirm = confirmInput.value;

    // Remove existing validation styling
    confirmInput.classList.remove("bb-field-valid", "bb-field-invalid");
    const existingError = confirmInput.parentElement.querySelector(".bb-field-error");
    if (existingError) {
      existingError.remove();
    }

    if (confirm.length > 0) {
      if (password === confirm) {
        confirmInput.classList.add("bb-field-valid");
      } else {
        confirmInput.classList.add("bb-field-invalid");
        const errorDiv = document.createElement("div");
        errorDiv.className = "bb-field-error";
        errorDiv.textContent = "Passwords do not match";
        errorDiv.style.cssText = "color: #dc2626; font-size: 12px; margin-top: 4px;";
        confirmInput.parentElement.appendChild(errorDiv);
      }
    }
  }
}

/**
 * Setup authentication event handlers
 */
function setupAuthHandlers() {
  console.log("Setting up auth handlers...");

  // Use event delegation for main auth buttons that might not exist initially
  document.addEventListener("click", (e) => {
    const target = e.target;

    // Handle login button clicks (various IDs)
    if (target.id === "loginBtn" || target.id === "showLoginBtn") {
      e.preventDefault();
      console.log("Login button clicked via delegation");
      showAuthModal();
      showAuthForm("login");
    }

    // Handle signup button clicks
    if (target.id === "showSignupBtn") {
      e.preventDefault();
      console.log("Signup button clicked via delegation");
      showAuthModal();
      showAuthForm("signup");
    }

    // Handle logout button clicks
    if (target.id === "logoutBtn") {
      e.preventDefault();
      console.log("Logout button clicked via delegation");
      handleLogout();
    }
  });

  // Set up modal form handlers
  setupAuthModalHandlers();

  // Set up password strength checking
  setupPasswordStrength();
}

/**
 * Handle logout action
 */
async function handleLogout() {
  try {
    const { error } = await supabase.auth.signOut();
    if (error) {
      console.error("Logout error:", error);
      alert("Logout failed. Please try again.");
    } else {
      console.log("Logout successful");
      window.location.reload();
    }
  } catch (error) {
    console.error("Logout error:", error);
    alert("Logout failed. Please try again.");
  }
}

/**
 * Setup authentication modal form handlers
 */
function setupAuthModalHandlers() {
  // Use event delegation to handle form submissions even when modal loads later
  document.addEventListener("submit", (e) => {
    if (e.target.id === "emailLoginForm") {
      e.preventDefault();
      handleEmailLogin(e);
    } else if (e.target.id === "emailSignupForm") {
      e.preventDefault();
      handleEmailSignup(e);
    } else if (e.target.id === "passwordResetForm") {
      e.preventDefault();
      handlePasswordReset(e);
    }
  });

  // Use event delegation for social login buttons
  document.addEventListener("click", (e) => {
    if (e.target.closest(".bb-social-btn[data-provider]")) {
      e.preventDefault();
      const button = e.target.closest(".bb-social-btn[data-provider]");
      const provider = button.dataset.provider;
      handleSocialLogin(provider);
    }

    // Handle modal close
    if (e.target.closest(".bb-modal-close") || e.target.id === "authModal") {
      if (e.target.id === "authModal" && e.target === e.currentTarget) {
        // Only close if clicking the backdrop
        closeAuthModal();
      } else if (e.target.closest(".bb-modal-close")) {
        closeAuthModal();
      }
    }
  });
}

/**
 * Setup login page handlers (for homepage integration)
 */
function setupLoginPageHandlers() {
  // This is now handled by event delegation in setupAuthHandlers()
  // No need for direct element handlers since they're covered by delegation
  console.log("Login page handlers setup - using event delegation");
}

// CAPTCHA success callback (global function)
window.onTurnstileSuccess = function (token) {
  captchaToken = token;
  const signupBtn = document.getElementById("signupSubmitBtn");
  if (signupBtn) {
    signupBtn.disabled = false;
  }
};

// Export functions for use by other modules
if (typeof module !== 'undefined' && module.exports) {
  // Node.js environment
  module.exports = {
    initializeSupabase,
    loadAuthModal,
    handleAuthCallback,
    registerUserWithBackend,
    updateAuthState,
    updateUserInfo,
    getInitials,
    showAuthModal,
    closeAuthModal,
    showAuthForm,
    handleEmailLogin,
    handleEmailSignup,
    handlePasswordReset,
    handleSocialLogin,
    handlePendingDomain,
    showAuthLoading,
    hideAuthLoading,
    showAuthError,
    clearAuthError,
    setupPasswordStrength,
    setupAuthHandlers,
    setupAuthModalHandlers,
    setupLoginPageHandlers,
    handleLogout
  };
} else {
  // Browser environment - make functions globally available
  window.BBAuth = {
    initializeSupabase,
    loadAuthModal,
    handleAuthCallback,
    registerUserWithBackend,
    updateAuthState,
    updateUserInfo,
    getInitials,
    showAuthModal,
    closeAuthModal,
    showAuthForm,
    handleEmailLogin,
    handleEmailSignup,
    handlePasswordReset,
    handleSocialLogin,
    handlePendingDomain,
    showAuthLoading,
    hideAuthLoading,
    showAuthError,
    clearAuthError,
    setupPasswordStrength,
    setupAuthHandlers,
    setupAuthModalHandlers,
    setupLoginPageHandlers,
    handleLogout
  };

  // Also make individual functions available globally for backward compatibility
  window.initializeSupabase = initializeSupabase;
  window.loadAuthModal = loadAuthModal;
  window.handleAuthCallback = handleAuthCallback;
  window.registerUserWithBackend = registerUserWithBackend;
  window.updateAuthState = updateAuthState;
  window.updateUserInfo = updateUserInfo;
  window.getInitials = getInitials;
  window.showAuthModal = showAuthModal;
  window.closeAuthModal = closeAuthModal;
  window.showAuthForm = showAuthForm;
  window.handleEmailLogin = handleEmailLogin;
  window.handleEmailSignup = handleEmailSignup;
  window.handlePasswordReset = handlePasswordReset;
  window.handleSocialLogin = handleSocialLogin;
  window.handlePendingDomain = handlePendingDomain;
  window.showAuthLoading = showAuthLoading;
  window.hideAuthLoading = hideAuthLoading;
  window.showAuthError = showAuthError;
  window.clearAuthError = clearAuthError;
  window.setupPasswordStrength = setupPasswordStrength;
  window.setupAuthHandlers = setupAuthHandlers;
  window.setupAuthModalHandlers = setupAuthModalHandlers;
  window.setupLoginPageHandlers = setupLoginPageHandlers;
  window.handleLogout = handleLogout;
  
  // Convenience functions for common auth form actions
  window.showLoginForm = () => showAuthForm("login");
  window.showSignupForm = () => showAuthForm("signup");
  window.showResetForm = () => showAuthForm("reset");
}
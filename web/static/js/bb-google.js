/**
 * Google Analytics Integration Handler
 * Handles GA4 property connections with two-step account/property selection.
 * Flow: Connect -> OAuth -> Select Account (if multiple) -> Review Properties -> Save All
 */

/**
 * Formats a timestamp as a relative date string
 * @param {string} timestamp - ISO timestamp string
 * @returns {string} Formatted date string
 */
function formatGoogleDate(timestamp) {
  const date = new Date(timestamp);
  const now = new Date();
  const diffMs = now - date;
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffDays === 0) {
    return "today";
  } else if (diffDays === 1) {
    return "yesterday";
  } else if (diffDays < 7) {
    return `${diffDays} days ago`;
  } else {
    return date.toLocaleDateString("en-AU", {
      day: "numeric",
      month: "short",
      year: "numeric",
    });
  }
}
/**
 * Initialise Google Analytics integration UI handlers
 */
function setupGoogleIntegration() {
  document.addEventListener("click", (event) => {
    const element = event.target.closest("[bbb-action]");
    if (!element) {
      return;
    }

    const action = element.getAttribute("bbb-action");
    if (!action || !action.startsWith("google-")) {
      return;
    }

    event.preventDefault();
    handleGoogleAction(action, element);
  });
}

/**
 * Handle Google Analytics-specific actions
 * @param {string} action - The action to perform
 * @param {HTMLElement} element - The element that triggered the action
 */
function handleGoogleAction(action, element) {
  switch (action) {
    case "google-connect":
      connectGoogle();
      break;

    case "google-disconnect": {
      const connectionId = element.getAttribute("bbb-id");
      if (connectionId) {
        disconnectGoogle(connectionId);
      } else {
        console.warn("google-disconnect: missing bbb-id attribute");
      }
      break;
    }

    case "google-refresh":
      loadGoogleConnections();
      break;

    case "google-refresh-accounts":
      refreshGA4Accounts();
      break;

    case "google-select-account": {
      const accountId = element.getAttribute("data-account-id");
      if (accountId) {
        selectGoogleAccount(accountId);
      }
      break;
    }

    default:
      break;
  }
}

/**
 * Load and display Google Analytics connections for the current organisation
 */
async function loadGoogleConnections() {
  try {
    if (!window.dataBinder?.fetchData) {
      console.warn(
        "dataBinder not available, skipping Google connections load"
      );
      return;
    }

    // Load GA4 accounts from DB (for the account selector)
    await loadGA4AccountsFromDB();

    // Fetch organisation domains first (needed for domain tags)
    await loadOrganisationDomains();

    const connections = await window.dataBinder.fetchData(
      "/v1/integrations/google"
    );

    const connectionsList = document.getElementById("googleConnectionsList");
    const emptyState = document.getElementById("googleEmptyState");

    if (!connectionsList) {
      return;
    }

    const template = connectionsList.querySelector(
      '[bbb-template="google-connection"]'
    );

    if (!template) {
      console.error("Google connection template not found");
      return;
    }

    // Clear existing connections (except template)
    const existingConnections =
      connectionsList.querySelectorAll(".google-connection");
    existingConnections.forEach((el) => el.remove());

    if (!connections || connections.length === 0) {
      // No connections - show empty state message
      if (emptyState) emptyState.style.display = "block";
      return;
    }

    // Has connections - hide empty state message, show connections
    if (emptyState) emptyState.style.display = "none";

    // Build connection elements
    for (const conn of connections) {
      const clone = template.cloneNode(true);
      clone.style.display = "block";
      clone.removeAttribute("bbb-template");
      clone.classList.add("google-connection");

      // Set property name
      const nameEl = clone.querySelector(".google-name");
      if (nameEl) {
        if (conn.ga4_property_name) {
          nameEl.textContent = conn.ga4_property_name;
        } else if (conn.ga4_property_id) {
          nameEl.textContent = `Property ${conn.ga4_property_id}`;
        } else {
          nameEl.textContent = "Google Analytics Connection";
        }
      }

      // Set Google account name
      const emailEl = clone.querySelector(".google-account-name");
      const accountName = conn.google_account_name || conn.google_email;
      if (emailEl && accountName) {
        emailEl.textContent = accountName;
      }

      // Display domain tags instead of connected date
      const dateEl = clone.querySelector(".google-connected-date");
      if (dateEl) {
        // Clear existing content safely
        while (dateEl.firstChild) {
          dateEl.removeChild(dateEl.firstChild);
        }
        dateEl.style.cssText =
          "display: flex; flex-wrap: wrap; gap: 6px; align-items: center;";

        // Add domain tags
        if (conn.domain_ids && conn.domain_ids.length > 0) {
          conn.domain_ids.forEach((domainId) => {
            // Find domain name from organisationDomains
            const domain = organisationDomains.find((d) => d.id === domainId);
            const domainName = domain ? domain.name : `Domain #${domainId}`;

            const tag = document.createElement("span");
            tag.className = "domain-tag";
            tag.style.cssText =
              "display: inline-flex; align-items: center; gap: 4px; padding: 4px 8px; background: #e0e7ff; color: #3730a3; border-radius: 4px; font-size: 13px;";
            tag.textContent = domainName;

            const removeBtn = document.createElement("button");
            removeBtn.textContent = "×";
            removeBtn.style.cssText =
              "background: none; border: none; color: #6366f1; font-size: 16px; cursor: pointer; padding: 0; margin-left: 2px; line-height: 1;";
            removeBtn.type = "button";
            removeBtn.setAttribute("aria-label", `Remove ${domainName}`);
            removeBtn.title = "Remove domain";
            removeBtn.onclick = (e) => {
              e.stopPropagation();
              removeDomainFromConnection(conn.id, domainId, conn.domain_ids);
            };

            tag.appendChild(removeBtn);
            dateEl.appendChild(tag);
          });
        }

        renderInlineDomainAdder(dateEl, conn);
      }

      // Set status indicator
      const statusEl = clone.querySelector(".google-status");
      if (statusEl) {
        const isActive = conn.status === "active";
        statusEl.textContent = isActive ? "Active" : "Inactive";
        statusEl.classList.toggle("status-active", isActive);
        statusEl.classList.toggle("status-inactive", !isActive);
      }

      // Set connection ID on disconnect button
      const disconnectBtn = clone.querySelector(
        '[bbb-action="google-disconnect"]'
      );
      if (disconnectBtn) {
        disconnectBtn.setAttribute("bbb-id", conn.id);
      }

      // Set up status toggle if present
      const statusToggle = clone.querySelector(".google-status-toggle");
      const toggleContainer = clone.querySelector(".google-toggle-container");

      if (statusToggle && toggleContainer) {
        const isActive = conn.status === "active";
        statusToggle.checked = isActive;
        statusToggle.setAttribute("data-connection-id", conn.id);

        // Update toggle visual state
        const track = clone.querySelector(".google-toggle-track");
        const thumb = clone.querySelector(".google-toggle-thumb");
        if (track && thumb) {
          if (isActive) {
            track.style.backgroundColor = "#10b981";
            thumb.style.transform = "translateX(20px)";
          } else {
            track.style.backgroundColor = "#d1d5db";
            thumb.style.transform = "translateX(0)";
          }
        }

        // Listen on label container instead of hidden checkbox
        toggleContainer.addEventListener("click", async (e) => {
          e.preventDefault(); // Prevent default label behavior

          // Toggle the checkbox state
          const newActive = !statusToggle.checked;
          statusToggle.checked = newActive;

          // Update visual state immediately
          if (track && thumb) {
            if (newActive) {
              track.style.backgroundColor = "#10b981";
              thumb.style.transform = "translateX(20px)";
            } else {
              track.style.backgroundColor = "#d1d5db";
              thumb.style.transform = "translateX(0)";
            }
          }

          // Call API to persist change
          await toggleConnectionStatus(conn.id, newActive);
        });
      }

      connectionsList.appendChild(clone);
    }
  } catch (error) {
    console.error("Failed to load Google connections:", error);
  }
}

/**
 * Initiate Google OAuth flow
 */
async function connectGoogle() {
  try {
    if (!window.dataBinder?.fetchData) {
      showGoogleError("System not ready. Please refresh the page.");
      return;
    }
    const response = await window.dataBinder.fetchData(
      "/v1/integrations/google",
      { method: "POST" }
    );

    if (response && response.auth_url) {
      // Redirect to Google OAuth
      window.location.href = response.auth_url;
    } else {
      throw new Error("No OAuth URL returned");
    }
  } catch (error) {
    console.error("Failed to start Google OAuth:", error);
    showGoogleError("Failed to connect to Google. Please try again.");
  }
}

/**
 * Disconnect a Google Analytics connection
 * @param {string} connectionId - The connection ID to disconnect
 */
async function disconnectGoogle(connectionId) {
  if (!confirm("Are you sure you want to disconnect Google Analytics?")) {
    return;
  }

  try {
    const session = await window.supabase.auth.getSession();
    const token = session?.data?.session?.access_token;
    if (!token) {
      showGoogleError("Not authenticated. Please sign in.");
      return;
    }
    const response = await fetch(
      `/v1/integrations/google/${encodeURIComponent(connectionId)}`,
      {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || `HTTP ${response.status}`);
    }

    showGoogleSuccess("Google Analytics disconnected");
    loadGoogleConnections();
  } catch (error) {
    console.error("Failed to disconnect Google:", error);
    showGoogleError("Failed to disconnect Google Analytics");
  }
}

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function getSessionWithTimeout(timeoutMs) {
  return Promise.race([
    window.supabase.auth.getSession(),
    new Promise((_, reject) =>
      setTimeout(() => reject(new Error("Session timeout")), timeoutMs)
    ),
  ]);
}

async function getGoogleAuthToken() {
  if (!window.supabase?.auth) {
    return {
      token: "",
      message: "Auth not ready. Please refresh the page.",
      retryable: true,
    };
  }

  const attempts = 4;
  const baseDelayMs = 300;
  const timeoutMs = 800;
  let lastError;

  for (let attempt = 0; attempt < attempts; attempt += 1) {
    try {
      const sessionResult = await getSessionWithTimeout(timeoutMs);
      const token = sessionResult?.data?.session?.access_token;
      if (token) {
        return { token };
      }
    } catch (error) {
      lastError = error;
    }

    try {
      const userResult = await window.supabase.auth.getUser();
      const user = userResult?.data?.user;
      if (user) {
        return {
          token: "",
          message: "Session still initialising. Please try again.",
          retryable: true,
        };
      }
    } catch (error) {
      lastError = error;
    }

    await wait(baseDelayMs * (attempt + 1));
  }

  return {
    token: "",
    message: "Not authenticated. Please refresh and sign in.",
    retryable: true,
    error: lastError,
  };
}

async function loadOrganisationDomains() {
  if (!window.BBDomainSearch) {
    organisationDomains = [];
    return [];
  }

  try {
    const loadedDomains = await window.BBDomainSearch.loadOrganisationDomains();
    organisationDomains = Array.isArray(loadedDomains)
      ? loadedDomains
      : window.BBDomainSearch.getDomains();
  } catch (error) {
    console.warn("Failed to load organisation domains:", error);
    organisationDomains = window.BBDomainSearch.getDomains?.() || [];
  }

  return organisationDomains;
}

/**
 * Select a Google Analytics account and fetch its properties
 * @param {string} accountId - The GA account ID
 */
async function selectGoogleAccount(accountId) {
  try {
    const authResult = await getGoogleAuthToken();
    const token = authResult.token;
    if (!token) {
      const accountList = document.getElementById("googleAccountList");
      if (accountList) {
        accountList.innerHTML =
          '<div style="text-align: center; padding: 20px; color: #dc2626;">' +
          (authResult.message || "Not authenticated. Please sign in.") +
          "</div>";
      }
      showGoogleError(
        authResult.message || "Not authenticated. Please sign in."
      );
      return;
    }

    if (!pendingGASessionData || !pendingGASessionData.session_id) {
      showGoogleError("OAuth session expired. Please reconnect.");
      hideAccountSelection();
      return;
    }

    // Show loading state
    const accountList = document.getElementById("googleAccountList");
    if (accountList) {
      accountList.innerHTML =
        '<div style="text-align: center; padding: 20px;">Loading properties...</div>';
    }

    // Fetch properties for this account
    const fetchUrl = `/v1/integrations/google/pending-session/${pendingGASessionData.session_id}/accounts/${encodeURIComponent(accountId)}/properties`;

    const response = await fetch(fetchUrl, {
      headers: { Authorization: `Bearer ${token}` },
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || `HTTP ${response.status}`);
    }

    const result = await response.json();
    const properties = result.data?.properties || [];

    pendingGASessionData.selected_account_id = accountId;
    pendingGASessionData.properties = properties;

    await saveAllPropertiesForAccount(accountId, properties);
  } catch (error) {
    console.error("Failed to fetch properties for account:", error);
    showGoogleError("Failed to load properties. Please try again.");
  }
}

async function saveAllPropertiesForAccount(accountId, properties = null) {
  try {
    const authResult = await getGoogleAuthToken();
    const token = authResult.token;
    if (!token) {
      showGoogleError(
        authResult.message || "Not authenticated. Please sign in."
      );
      return;
    }

    if (!pendingGASessionData || !pendingGASessionData.session_id) {
      showGoogleError("OAuth session expired. Please reconnect.");
      hideAccountSelection();
      return;
    }

    let accountProperties = properties;
    if (!Array.isArray(accountProperties)) {
      const fetchUrl = `/v1/integrations/google/pending-session/${pendingGASessionData.session_id}/accounts/${encodeURIComponent(accountId)}/properties`;
      const response = await fetch(fetchUrl, {
        headers: { Authorization: `Bearer ${token}` },
      });

      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || `HTTP ${response.status}`);
      }

      const result = await response.json();
      accountProperties = result.data?.properties || [];
    }

    if (!Array.isArray(accountProperties) || accountProperties.length === 0) {
      showGoogleError("No properties found for this account");
      return;
    }

    const propertyDomainMap = {};
    accountProperties.forEach((prop) => {
      if (prop && prop.property_id) {
        propertyDomainMap[prop.property_id] = [];
      }
    });

    const saveResponse = await fetch(
      "/v1/integrations/google/save-properties",
      {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          session_id: pendingGASessionData.session_id,
          account_id: accountId,
          active_property_ids: [],
          property_domain_map: propertyDomainMap,
        }),
      }
    );

    if (!saveResponse.ok) {
      const text = await saveResponse.text();
      throw new Error(text || `HTTP ${saveResponse.status}`);
    }

    pendingGASessionData = null;
    hideAccountSelection();
    showGoogleSuccess("Google Analytics connected successfully!");
    await loadGoogleConnections();
  } catch (error) {
    console.error("Failed to save properties:", error);
    showGoogleError("Failed to save properties. Please try again.");
  }
}

/**
 * Toggle an existing connection's status (active/inactive)
 * @param {string} connectionId - The connection ID
 * @param {boolean} active - Whether to set active
 */
async function toggleConnectionStatus(connectionId, active) {
  try {
    const session = await window.supabase.auth.getSession();
    const token = session?.data?.session?.access_token;
    if (!token) {
      console.error("No auth token available");
      showGoogleError("Not authenticated. Please sign in.");
      return;
    }

    const response = await fetch(
      `/v1/integrations/google/${encodeURIComponent(connectionId)}/status`,
      {
        method: "PATCH",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          status: active ? "active" : "inactive",
        }),
      }
    );

    if (!response.ok) {
      const text = await response.text();
      console.error(`GA toggle API error: ${response.status}`, text);
      throw new Error(text || `HTTP ${response.status}`);
    }
    // Reload to update UI
    loadGoogleConnections();
  } catch (error) {
    console.error("Failed to toggle connection status:", error);
    showGoogleError("Failed to update status");
    loadGoogleConnections(); // Reload to reset toggle state
  }
}

let organisationDomains = window.BBDomainSearch
  ? window.BBDomainSearch.getDomains()
  : [];

/**
 * Show account selection UI when multiple accounts are available
 * Uses the searchable dropdown pattern
 * @param {Array} accounts - Array of GA accounts to choose from
 */
function showAccountSelection(accounts) {
  // Convert pending session accounts to the format used by storedGA4Accounts
  storedGA4Accounts = accounts.map((acc) => ({
    id: acc.account_id, // Use account_id as the ID for now
    google_account_id: acc.account_id,
    google_account_name: acc.display_name || acc.account_id,
    google_email: pendingGASessionData?.email || "",
  }));

  // Hide the old-style account selection UI if it exists
  const oldAccountUI = document.getElementById("googleAccountSelection");
  if (oldAccountUI) {
    oldAccountUI.style.display = "none";
  }

  // Hide empty state
  const emptyState = document.getElementById("googleEmptyState");
  if (emptyState) emptyState.style.display = "none";

  // Show the searchable account selector
  renderAccountSelector();
}

/**
 * Hide account selection UI
 */
function hideAccountSelection() {
  const accountUI = document.getElementById("googleAccountSelection");
  if (accountUI) {
    accountUI.style.display = "none";
  }
}

/**
 * Show a success message
 */
function showGoogleSuccess(message) {
  if (window.showIntegrationFeedback) {
    window.showIntegrationFeedback("google", "success", message);
  } else if (window.showDashboardSuccess) {
    window.showDashboardSuccess(message);
  } else {
    const successEl = document.getElementById("googleSuccessMessage");
    const textEl = document.getElementById("googleSuccessText");
    if (successEl && textEl) {
      textEl.textContent = message;
      successEl.style.display = "block";
      setTimeout(() => {
        successEl.style.display = "none";
      }, 5000);
    } else {
      alert(message);
    }
  }
}

/**
 * Show an error message
 */
function showGoogleError(message) {
  if (window.showIntegrationFeedback) {
    window.showIntegrationFeedback("google", "error", message);
  } else if (window.showDashboardError) {
    window.showDashboardError(message);
  } else {
    const errorEl = document.getElementById("googleErrorMessage");
    const textEl = document.getElementById("googleErrorText");
    if (errorEl && textEl) {
      textEl.textContent = message;
      errorEl.style.display = "block";
      setTimeout(() => {
        errorEl.style.display = "none";
      }, 5000);
    } else {
      alert(message);
    }
  }
}

// Store pending session data for property selection
let pendingGASessionData = null;

// Store GA4 accounts loaded from DB
let storedGA4Accounts = [];

/**
 * Load GA4 accounts from the database (not from Google API)
 * These are persisted accounts that can be displayed immediately on page load
 */
async function loadGA4AccountsFromDB() {
  try {
    const session = await window.supabase.auth.getSession();
    const token = session?.data?.session?.access_token;

    if (!token) {
      return [];
    }

    const response = await fetch("/v1/integrations/google/accounts", {
      headers: { Authorization: `Bearer ${token}` },
    });

    if (!response.ok) {
      console.error("Failed to load accounts:", response.status);
      storedGA4Accounts = [];
      selectedGA4Account = null;
      renderAccountSelector();
      return [];
    }

    const result = await response.json();
    storedGA4Accounts = result.data?.accounts || [];

    // Update the account selector UI
    renderAccountSelector();

    return storedGA4Accounts;
  } catch (error) {
    console.error("Error loading accounts from DB:", error);
    storedGA4Accounts = [];
    selectedGA4Account = null;
    renderAccountSelector();
    return [];
  }
}

// Currently selected GA4 account
let selectedGA4Account = null;

/**
 * Render the GA4 account selector dropdown
 */
function renderAccountSelector() {
  const selectorContainer = document.getElementById("googleAccountSelector");
  const searchInput = document.getElementById("googleAccountSearch");
  const dropdown = document.getElementById("googleAccountDropdown");

  if (!selectorContainer || !searchInput || !dropdown) {
    return;
  }

  // Show selector if we have accounts
  if (storedGA4Accounts.length > 0) {
    selectorContainer.style.display = "block";
  } else {
    selectorContainer.style.display = "none";
    return;
  }

  // If we have a selected account, show it in the input
  if (selectedGA4Account) {
    searchInput.value =
      selectedGA4Account.google_account_name || "Unnamed Account";
  }

  // Function to render dropdown options
  const renderDropdownOptions = (query) => {
    // Clear existing options
    while (dropdown.firstChild) {
      dropdown.removeChild(dropdown.firstChild);
    }

    const lowerQuery = (query || "").toLowerCase().trim();

    // Filter accounts
    const filtered = lowerQuery
      ? storedGA4Accounts.filter(
          (acc) =>
            acc.google_account_name?.toLowerCase().includes(lowerQuery) ||
            acc.google_email?.toLowerCase().includes(lowerQuery)
        )
      : storedGA4Accounts;

    if (filtered.length === 0) {
      const noResults = document.createElement("div");
      noResults.textContent = "No accounts found";
      noResults.style.cssText =
        "padding: 10px 16px; color: #6b7280; font-size: 14px;";
      dropdown.appendChild(noResults);
      dropdown.style.display = "block";
      return;
    }

    filtered.forEach((account) => {
      const option = document.createElement("div");
      option.style.cssText =
        "padding: 10px 16px; cursor: pointer; font-size: 14px; border-bottom: 1px solid #f3f4f6;";
      option.onmouseover = () => {
        option.style.background = "#f9fafb";
      };
      option.onmouseout = () => {
        option.style.background = "white";
      };

      const nameSpan = document.createElement("strong");
      nameSpan.textContent = account.google_account_name || "Unnamed Account";
      option.appendChild(nameSpan);

      if (account.google_email) {
        const emailSpan = document.createElement("span");
        emailSpan.textContent = " (" + account.google_email + ")";
        emailSpan.style.color = "#6b7280";
        option.appendChild(emailSpan);
      }

      option.onmousedown = (e) => {
        e.preventDefault();
        onAccountSelected(account);
        dropdown.style.display = "none";
        onDocumentClick({ target: document.body });
      };

      dropdown.appendChild(option);
    });

    dropdown.style.display = "block";
    ensureDocumentListener();
  };

  // Event listeners for search input
  let documentListenerActive = false;
  const onDocumentClick = (event) => {
    if (!selectorContainer.contains(event.target)) {
      dropdown.style.display = "none";
      if (selectedGA4Account) {
        searchInput.value =
          selectedGA4Account.google_account_name || "Unnamed Account";
      }
      if (documentListenerActive) {
        document.removeEventListener("click", onDocumentClick);
        documentListenerActive = false;
      }
    }
  };

  const ensureDocumentListener = () => {
    if (documentListenerActive) {
      return;
    }
    documentListenerActive = true;
    document.addEventListener("click", onDocumentClick);
  };

  searchInput.onfocus = () => {
    searchInput.select();
    renderDropdownOptions(searchInput.value);
    ensureDocumentListener();
  };
  searchInput.oninput = () => renderDropdownOptions(searchInput.value);
  searchInput.onclick = (e) => e.stopPropagation();

  // Auto-select first account if only one and none selected
  if (storedGA4Accounts.length === 1 && !selectedGA4Account) {
    onAccountSelected(storedGA4Accounts[0]);
  }
}

/**
 * Handle account selection - update input and load properties
 */
async function onAccountSelected(account) {
  selectedGA4Account = account;
  const searchInput = document.getElementById("googleAccountSearch");
  if (searchInput) {
    searchInput.value = account.google_account_name || "Unnamed Account";
  }

  // If we have a pending session (OAuth flow in progress), use the original selectGoogleAccount
  if (pendingGASessionData && pendingGASessionData.session_id) {
    // Use the existing flow that works
    selectGoogleAccount(account.google_account_id);
    return;
  }

  await loadGoogleConnections();
}

/**
 * Refresh GA4 accounts by fetching fresh data from Google API
 * This syncs the database with the current state of the user's GA accounts
 */
async function refreshGA4Accounts() {
  try {
    const session = await window.supabase.auth.getSession();
    const token = session?.data?.session?.access_token;

    if (!token) {
      showGoogleError("Not authenticated. Please sign in.");
      return;
    }

    // Show loading state
    const refreshBtn = document.querySelector(
      '[bbb-action="google-refresh-accounts"]'
    );
    if (refreshBtn) {
      refreshBtn.disabled = true;
      refreshBtn.textContent = "Refreshing...";
    }

    const response = await fetch("/v1/integrations/google/accounts/refresh", {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || `HTTP ${response.status}`);
    }

    const result = await response.json();

    if (result.data?.needs_reauth) {
      // Token expired or invalid - need to re-authenticate
      showGoogleError(
        result.data.message || "Please reconnect to Google Analytics."
      );
      // Optionally trigger OAuth flow
      // connectGoogle();
      return;
    }

    // Update stored accounts
    storedGA4Accounts = result.data?.accounts || [];
    showGoogleSuccess("Accounts refreshed successfully");

    // Reload connections to reflect any changes
    await loadGoogleConnections();
  } catch (error) {
    console.error("Error refreshing accounts:", error);
    showGoogleError("Failed to refresh accounts. Please try again.");
  } finally {
    const refreshBtn = document.querySelector(
      '[bbb-action="google-refresh-accounts"]'
    );
    if (refreshBtn) {
      refreshBtn.disabled = false;
      refreshBtn.textContent = "Refresh";
    }
  }
}

/**
 * Handle OAuth callback result checks
 */
async function handleGoogleOAuthCallback() {
  const params = new URLSearchParams(window.location.search);
  const googleConnected = params.get("google_connected");
  const googleError = params.get("google_error");
  const gaSession = params.get("ga_session");

  if (googleConnected) {
    // Clean up URL
    const url = new URL(window.location.href);
    url.searchParams.delete("google_connected");
    window.history.replaceState({}, "", url.toString());

    showGoogleSuccess("Google Analytics connected successfully!");
    loadGoogleConnections();
  } else if (gaSession) {
    // Fetch session data from server
    try {
      const session = await window.supabase.auth.getSession();
      const token = session?.data?.session?.access_token;
      if (!token) {
        showGoogleError("Not authenticated. Please sign in.");
        return;
      }
      const response = await fetch(
        `/v1/integrations/google/pending-session/${gaSession}`,
        {
          headers: { Authorization: `Bearer ${token}` },
        }
      );

      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || `HTTP ${response.status}`);
      }

      const result = await response.json();
      const sessionData = result.data;

      // Store session ID for subsequent requests
      sessionData.session_id = gaSession;
      pendingGASessionData = sessionData;

      // Open notifications modal (contains Google Analytics section)
      const notificationsModal = document.getElementById("notificationsModal");
      if (notificationsModal) {
        notificationsModal.classList.add("show");
      }

      // Determine which UI to show based on session data
      const accounts = sessionData.accounts || [];
      const properties = sessionData.properties || [];

      if (accounts.length > 1 && properties.length === 0) {
        // Multiple accounts, no properties yet - show account picker
        showAccountSelection(accounts);
      } else if (properties.length > 0 && accounts.length >= 1) {
        // Properties already fetched for single account - save them immediately
        await saveAllPropertiesForAccount(accounts[0].account_id, properties);
      } else if (accounts.length === 1) {
        // Single account but no properties - fetch them
        selectGoogleAccount(accounts[0].account_id);
      } else {
        throw new Error("No accounts or properties found");
      }

      // Clean up URL
      const url = new URL(window.location.href);
      url.searchParams.delete("ga_session");
      window.history.replaceState({}, "", url.toString());
    } catch (e) {
      console.error("Failed to load session:", e);
      showGoogleError("Session expired. Please reconnect to Google Analytics.");
      // Clean up URL
      const url = new URL(window.location.href);
      url.searchParams.delete("ga_session");
      window.history.replaceState({}, "", url.toString());
    }
  } else if (googleError) {
    showGoogleError(`Failed to connect Google Analytics: ${googleError}`);
    const url = new URL(window.location.href);
    url.searchParams.delete("google_error");
    window.history.replaceState({}, "", url.toString());
  }
}

/**
 * Remove a domain from a GA4 connection
 * @param {string} connectionId - The connection ID
 * @param {number} domainId - The domain ID to remove
 */
async function removeDomainFromConnection(
  connectionId,
  domainId,
  currentDomainIds = null
) {
  try {
    const session = await window.supabase.auth.getSession();
    const token = session?.data?.session?.access_token;

    if (!token) {
      showGoogleError("Please sign in to update connections");
      return;
    }

    // Get current connection to find existing domain_ids
    let updatedDomainIds;
    if (Array.isArray(currentDomainIds)) {
      updatedDomainIds = currentDomainIds.filter((id) => id !== domainId);
    } else {
      const connections = await window.dataBinder.fetchData(
        "/v1/integrations/google"
      );
      const connection = connections.find((c) => c.id === connectionId);

      if (!connection) {
        showGoogleError("Connection not found");
        return;
      }

      // Remove the domain from the array
      updatedDomainIds = (connection.domain_ids || []).filter(
        (id) => id !== domainId
      );
    }

    // Use dedicated PATCH endpoint to update domains
    const response = await fetch(`/v1/integrations/google/${connectionId}`, {
      method: "PATCH",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        domain_ids: updatedDomainIds, // Send updated array
      }),
    });

    if (!response.ok) {
      throw new Error(`Failed to update connection: ${response.status}`);
    }

    // Reload connections to show updated list
    await loadGoogleConnections();
  } catch (error) {
    console.error("Failed to remove domain:", error);
    showGoogleError("Failed to remove domain. Please try again.");
  }
}

async function addDomainToConnection(connectionId, currentDomainIds, domainId) {
  try {
    const session = await window.supabase.auth.getSession();
    const token = session?.data?.session?.access_token;

    if (!token) {
      showGoogleError("Please sign in to update connections");
      return;
    }

    const updatedDomainIds = Array.from(
      new Set([...(currentDomainIds || []), domainId])
    );

    const response = await fetch(`/v1/integrations/google/${connectionId}`, {
      method: "PATCH",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ domain_ids: updatedDomainIds }),
    });

    if (!response.ok) {
      throw new Error(`Failed to update connection: ${response.status}`);
    }

    await loadGoogleConnections();
  } catch (error) {
    console.error("Failed to add domain:", error);
    showGoogleError("Failed to add domain. Please try again.");
  }
}

async function createDomainInline(domainName) {
  const session = await window.supabase.auth.getSession();
  const token = session?.data?.session?.access_token;

  if (!token) {
    showGoogleError("Please sign in to create domains");
    return null;
  }

  const response = await fetch("/v1/domains", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ domain: domainName }),
  });

  if (!response.ok) {
    const errorData = await response.json();
    throw new Error(errorData.message || "Failed to create domain");
  }

  const result = await response.json();
  const rawDomainId = result?.data?.domain_id ?? result?.domain_id ?? null;
  const newDomainId = Number(rawDomainId);
  const newDomainName = result?.data?.domain ?? result?.domain ?? domainName;

  if (!Number.isFinite(newDomainId)) {
    throw new Error("Invalid domain ID in response");
  }

  organisationDomains.push({ id: newDomainId, name: newDomainName });
  return newDomainId;
}

function renderInlineDomainAdder(container, connection) {
  const form = document.createElement("form");
  form.style.cssText = "position: relative; margin-top: 8px;";

  const input = document.createElement("input");
  input.type = "text";
  input.placeholder = "Add domain...";
  input.setAttribute("aria-label", "Add domain");
  input.setAttribute("bbb-domain-create", "option");
  input.style.cssText =
    "width: 100%; padding: 6px 10px; border: 1px solid #d1d5db; border-radius: 6px; font-size: 13px; box-sizing: border-box;";

  const dropdown = window.BBDomainSearch
    ? window.BBDomainSearch.createDropdownElement()
    : document.createElement("div");
  if (!dropdown.style.cssText) {
    dropdown.style.cssText =
      "display: none; position: absolute; top: 100%; left: 0; right: 0; max-height: 200px; overflow-y: auto; background: white; border: 1px solid #d1d5db; border-radius: 6px; margin-top: 4px; z-index: 1000; box-shadow: 0 2px 8px rgba(0,0,0,0.1);";
  }

  const selectDomain = async (domain) => {
    const currentIds = connection.domain_ids || [];
    await addDomainToConnection(connection.id, currentIds, domain.id);
  };

  if (window.BBDomainSearch) {
    window.BBDomainSearch.setupDomainSearchInput({
      input,
      dropdown,
      container: form,
      form,
      getExcludedDomainIds: () => connection.domain_ids || [],
      onSelectDomain: selectDomain,
      onCreateDomain: selectDomain,
      clearOnSelect: true,
      autoCreateOnSubmit: true,
      onError: (message) => {
        showGoogleError(
          message || "Failed to create domain. Please try again."
        );
      },
    });
  } else {
    form.addEventListener("submit", (event) => {
      event.preventDefault();
    });
  }

  form.appendChild(input);
  form.appendChild(dropdown);
  container.appendChild(form);
}

// Export functions
if (typeof window !== "undefined") {
  window.setupGoogleIntegration = setupGoogleIntegration;
  window.loadGoogleConnections = loadGoogleConnections;
  window.handleGoogleOAuthCallback = handleGoogleOAuthCallback;
  window.loadGA4AccountsFromDB = loadGA4AccountsFromDB;
  window.refreshGA4Accounts = refreshGA4Accounts;
  window.renderAccountSelector = renderAccountSelector;
}

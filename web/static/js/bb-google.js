/**
 * Google Analytics Integration Handler
 * Handles GA4 property connections.
 * Flow: Connect -> OAuth -> Select Property -> Save Connection
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

    case "google-select-property": {
      const propertyId = element.getAttribute("data-property-id");
      const propertyName =
        element.getAttribute("data-property-name") || `Property ${propertyId}`;
      if (propertyId) {
        saveGoogleProperty(propertyId, propertyName);
      }
      break;
    }

    case "google-cancel-selection":
      hidePropertySelection();
      break;

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
      if (emptyState) emptyState.style.display = "block";
      return;
    }

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

      // Set Google email
      const emailEl = clone.querySelector(".google-email");
      if (emailEl && conn.google_email) {
        emailEl.textContent = conn.google_email;
      }

      // Set connected date
      const dateEl = clone.querySelector(".google-connected-date");
      if (dateEl) {
        dateEl.textContent = `Connected ${formatGoogleDate(conn.created_at)}`;
      }

      // Set connection ID on disconnect button
      const disconnectBtn = clone.querySelector(
        '[bbb-action="google-disconnect"]'
      );
      if (disconnectBtn) {
        disconnectBtn.setAttribute("bbb-id", conn.id);
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
    const { data: { session } = {} } = await window.supabase.auth.getSession();
    const token = session?.access_token;
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

/**
 * Save the selected GA4 property after OAuth
 * @param {string} propertyId - The GA4 property ID
 * @param {string} propertyName - The property display name
 */
async function saveGoogleProperty(propertyId, propertyName) {
  try {
    const { data: { session } = {} } = await window.supabase.auth.getSession();
    const token = session?.access_token;
    if (!token) {
      showGoogleError("Not authenticated. Please sign in.");
      return;
    }

    // Get the pending OAuth data from URL params
    const params = new URLSearchParams(window.location.search);
    const accessToken = params.get("ga_access_token");
    const refreshToken = params.get("ga_refresh_token");
    const googleUserId = params.get("ga_user_id");
    const googleEmail = params.get("ga_email");

    if (!accessToken || !refreshToken) {
      showGoogleError("OAuth session expired. Please reconnect.");
      hidePropertySelection();
      return;
    }

    const response = await fetch("/v1/integrations/google/save-property", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        property_id: propertyId,
        property_name: propertyName,
        access_token: accessToken,
        refresh_token: refreshToken,
        google_user_id: googleUserId,
        google_email: googleEmail,
      }),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || `HTTP ${response.status}`);
    }

    // Clean up URL
    const url = new URL(window.location.href);
    url.searchParams.delete("ga_access_token");
    url.searchParams.delete("ga_refresh_token");
    url.searchParams.delete("ga_user_id");
    url.searchParams.delete("ga_email");
    url.searchParams.delete("ga_properties");
    window.history.replaceState({}, "", url.toString());

    hidePropertySelection();
    showGoogleSuccess("Google Analytics connected successfully!");
    loadGoogleConnections();
  } catch (error) {
    console.error("Failed to save Google property:", error);
    showGoogleError("Failed to save property selection");
  }
}

/**
 * Show property selection UI when multiple properties are available
 * @param {Array} properties - Array of GA4 properties to choose from
 */
function showPropertySelection(properties) {
  const selectionUI = document.getElementById("googlePropertySelection");
  const list = document.getElementById("googlePropertyList");

  if (!selectionUI || !list) {
    console.error("Property selection UI not found");
    return;
  }

  // Clear existing items using safe DOM method
  while (list.firstChild) {
    list.removeChild(list.firstChild);
  }

  // Add property options using safe DOM methods
  for (const prop of properties) {
    const item = document.createElement("button");
    item.className = "bb-button";
    item.style.cssText =
      "display: block; width: 100%; text-align: left; margin-bottom: 8px; padding: 12px 16px;";
    item.setAttribute("bbb-action", "google-select-property");
    item.setAttribute("data-property-id", prop.property_id);
    item.setAttribute("data-property-name", prop.display_name);

    // Build content safely using DOM methods
    const strongEl = document.createElement("strong");
    strongEl.textContent = prop.display_name;
    item.appendChild(strongEl);

    const detailSpan = document.createElement("span");
    detailSpan.style.cssText =
      "color: #6b7280; font-size: 13px; display: block;";
    let detailText = `Property ID: ${prop.property_id}`;
    if (prop.account_name) {
      detailText += ` • ${prop.account_name}`;
    }
    detailSpan.textContent = detailText;
    item.appendChild(detailSpan);

    list.appendChild(item);
  }

  // Hide empty state and show selection
  const emptyState = document.getElementById("googleEmptyState");
  if (emptyState) emptyState.style.display = "none";
  selectionUI.style.display = "block";
}

/**
 * Hide property selection UI
 */
function hidePropertySelection() {
  const selectionUI = document.getElementById("googlePropertySelection");
  if (selectionUI) {
    selectionUI.style.display = "none";
  }
  // Show empty state again if no connections
  loadGoogleConnections();
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

/**
 * Handle OAuth callback result checks
 */
function handleGoogleOAuthCallback() {
  const params = new URLSearchParams(window.location.search);
  const googleConnected = params.get("google_connected");
  const googleError = params.get("google_error");
  const gaProperties = params.get("ga_properties");

  if (googleConnected) {
    // Clean up URL
    const url = new URL(window.location.href);
    url.searchParams.delete("google_connected");
    window.history.replaceState({}, "", url.toString());

    showGoogleSuccess("Google Analytics connected successfully!");
    loadGoogleConnections();
  } else if (gaProperties) {
    // Multiple properties returned - need user to select one
    try {
      const properties = JSON.parse(decodeURIComponent(gaProperties));
      showPropertySelection(properties);
    } catch (e) {
      console.error("Failed to parse properties:", e);
      showGoogleError("Failed to load properties. Please try again.");
    }
  } else if (googleError) {
    showGoogleError(`Failed to connect Google Analytics: ${googleError}`);
    const url = new URL(window.location.href);
    url.searchParams.delete("google_error");
    window.history.replaceState({}, "", url.toString());
  }
}

// Export functions
if (typeof window !== "undefined") {
  window.setupGoogleIntegration = setupGoogleIntegration;
  window.loadGoogleConnections = loadGoogleConnections;
  window.handleGoogleOAuthCallback = handleGoogleOAuthCallback;
}

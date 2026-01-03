/**
 * Webflow Integration Handler
 * Handles Webflow workspace/site connections.
 * Flow: Connect -> OAuth -> Auto-Register Webhooks -> List Connections
 */

/**
 * Formats a timestamp as a relative date string
 * @param {string} timestamp - ISO timestamp string
 * @returns {string} Formatted date string
 */
function formatWebflowDate(timestamp) {
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
 * Initialise Webflow integration UI handlers
 */
function setupWebflowIntegration() {
  document.addEventListener("click", (event) => {
    const element = event.target.closest("[bbb-action]");
    if (!element) {
      return;
    }

    const action = element.getAttribute("bbb-action");
    if (!action || !action.startsWith("webflow-")) {
      return;
    }

    event.preventDefault();
    handleWebflowAction(action, element);
  });

  console.log("Webflow integration handlers initialised");
}

/**
 * Handle Webflow-specific actions
 * @param {string} action - The action to perform
 * @param {HTMLElement} element - The element that triggered the action
 */
function handleWebflowAction(action, element) {
  switch (action) {
    case "webflow-connect":
      connectWebflow();
      break;

    case "webflow-disconnect": {
      const connectionId = element.getAttribute("bbb-id");
      if (connectionId) {
        disconnectWebflow(connectionId);
      } else {
        console.warn("webflow-disconnect: missing bbb-id attribute");
      }
      break;
    }

    case "webflow-refresh":
      loadWebflowConnections();
      break;

    default:
      console.log("Unhandled Webflow action:", action);
  }
}

/**
 * Load and display Webflow connections for the current organisation
 */
async function loadWebflowConnections() {
  try {
    if (!window.dataBinder?.fetchData) {
      console.warn(
        "dataBinder not available, skipping Webflow connections load"
      );
      return;
    }
    const connections = await window.dataBinder.fetchData(
      "/v1/integrations/webflow"
    );

    const connectionsList = document.getElementById("webflowConnectionsList");
    const emptyState = document.getElementById("webflowEmptyState");

    if (!connectionsList) {
      // It's possible the user hasn't opened the modal yet or element doesn't exist
      return;
    }

    const template = connectionsList.querySelector(
      '[bbb-template="webflow-connection"]'
    );

    if (!template) {
      console.error("Webflow connection template not found");
      return;
    }

    // Clear existing connections (except template)
    const existingConnections = connectionsList.querySelectorAll(
      ".webflow-connection"
    );
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
      clone.classList.add("webflow-connection");

      // Set workspace name - prefer display name, fall back to ID
      const nameEl = clone.querySelector(".webflow-name");
      if (nameEl) {
        if (conn.workspace_name) {
          nameEl.textContent = conn.workspace_name;
        } else if (conn.webflow_workspace_id) {
          nameEl.textContent = `Workspace ${conn.webflow_workspace_id}`;
        } else {
          nameEl.textContent = "Webflow Connection";
        }
      }

      // Set connected date
      const dateEl = clone.querySelector(".webflow-connected-date");
      if (dateEl) {
        dateEl.textContent = `Connected ${formatWebflowDate(conn.created_at)}`;
      }

      // Set connection ID on disconnect button
      const disconnectBtn = clone.querySelector(
        '[bbb-action="webflow-disconnect"]'
      );
      if (disconnectBtn) {
        disconnectBtn.setAttribute("bbb-id", conn.id);
      }

      connectionsList.appendChild(clone);
    }
  } catch (error) {
    console.error("Failed to load Webflow connections:", error);
    // Don't show alert flow on simple load failure, just log
  }
}

/**
 * Initiate Webflow OAuth flow
 */
async function connectWebflow() {
  try {
    if (!window.dataBinder?.fetchData) {
      showWebflowError("System not ready. Please refresh the page.");
      return;
    }
    const response = await window.dataBinder.fetchData(
      "/v1/integrations/webflow",
      { method: "POST" }
    );

    if (response && response.auth_url) {
      // Redirect to Webflow OAuth
      window.location.href = response.auth_url;
    } else {
      throw new Error("No OAuth URL returned");
    }
  } catch (error) {
    console.error("Failed to start Webflow OAuth:", error);
    showWebflowError("Failed to connect to Webflow. Please try again.");
  }
}

/**
 * Disconnect a Webflow connection
 * @param {string} connectionId - The connection ID to disconnect
 */
async function disconnectWebflow(connectionId) {
  if (
    !confirm(
      "Are you sure you want to disconnect Webflow? Run on Publish will stop working."
    )
  ) {
    return;
  }

  try {
    const { data: { session } = {} } = await window.supabase.auth.getSession();
    const token = session?.access_token;
    if (!token) {
      showWebflowError("Not authenticated. Please sign in.");
      return;
    }
    const response = await fetch(
      `/v1/integrations/webflow/${encodeURIComponent(connectionId)}`,
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

    showWebflowSuccess("Webflow disconnected");
    loadWebflowConnections();
  } catch (error) {
    console.error("Failed to disconnect Webflow:", error);
    showWebflowError("Failed to disconnect Webflow");
  }
}

/**
 * Show a success message
 * Uses dashboard's generic integration feedback helper if available
 */
function showWebflowSuccess(message) {
  if (window.showIntegrationFeedback) {
    window.showIntegrationFeedback("webflow", "success", message);
  } else if (window.showDashboardSuccess) {
    window.showDashboardSuccess(message);
  } else {
    alert(message);
  }
}

/**
 * Show an error message
 * Uses dashboard's generic integration feedback helper if available
 */
function showWebflowError(message) {
  if (window.showIntegrationFeedback) {
    window.showIntegrationFeedback("webflow", "error", message);
  } else if (window.showDashboardError) {
    window.showDashboardError(message);
  } else {
    alert(message);
  }
}

/**
 * Handle OAuth callback result checks (if user returns here)
 */
function handleWebflowOAuthCallback() {
  const params = new URLSearchParams(window.location.search);
  const webflowConnected = params.get("webflow_connected");
  const webflowError = params.get("webflow_error");

  if (webflowConnected) {
    // Clean up URL
    const url = new URL(window.location.href);
    url.searchParams.delete("webflow_connected");
    window.history.replaceState({}, "", url.toString());

    showWebflowSuccess("Webflow connected successfully!");
    // Trigger reload of connections if the modal is open, or just wait for user to open it
    loadWebflowConnections();
  } else if (webflowError) {
    showWebflowError(`Failed to connect Webflow: ${webflowError}`);
    const url = new URL(window.location.href);
    url.searchParams.delete("webflow_error");
    window.history.replaceState({}, "", url.toString());
  }
}

// Export functions
if (typeof window !== "undefined") {
  window.setupWebflowIntegration = setupWebflowIntegration;
  window.loadWebflowConnections = loadWebflowConnections;
  window.handleWebflowOAuthCallback = handleWebflowOAuthCallback;
}

/**
 * Slack Integration Handler
 * Handles Slack workspace connections and user linking for notifications.
 */

/**
 * Formats a timestamp as a relative date string
 * @param {string} timestamp - ISO timestamp string
 * @returns {string} Formatted date string
 */
function formatSlackDate(timestamp) {
  const date = new Date(timestamp);
  const now = new Date();
  const diffMs = now - date;
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffDays === 0) {
    return "Today";
  } else if (diffDays === 1) {
    return "Yesterday";
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
 * Initialise Slack integration UI handlers
 */
function setupSlackIntegration() {
  document.addEventListener("click", (event) => {
    const element = event.target.closest("[bbb-action]");
    if (!element) {
      return;
    }

    const action = element.getAttribute("bbb-action");
    if (!action || !action.startsWith("slack-")) {
      return;
    }

    event.preventDefault();
    handleSlackAction(action, element);
  });

  console.log("Slack integration handlers initialised");
}

/**
 * Handle Slack-specific actions
 * @param {string} action - The action to perform
 * @param {HTMLElement} element - The element that triggered the action
 */
function handleSlackAction(action, element) {
  switch (action) {
    case "slack-connect":
      connectSlackWorkspace();
      break;

    case "slack-disconnect": {
      const connectionId = element.getAttribute("bbb-id");
      if (connectionId) {
        disconnectSlackWorkspace(connectionId);
      } else {
        console.warn("slack-disconnect: missing bbb-id attribute");
      }
      break;
    }

    case "slack-link-user": {
      const connectionId = element.getAttribute("bbb-id");
      if (connectionId) {
        linkSlackUser(connectionId);
      } else {
        console.warn("slack-link-user: missing bbb-id attribute");
      }
      break;
    }

    case "slack-unlink-user": {
      const connectionId = element.getAttribute("bbb-id");
      if (connectionId) {
        unlinkSlackUser(connectionId);
      } else {
        console.warn("slack-unlink-user: missing bbb-id attribute");
      }
      break;
    }

    case "slack-toggle-notifications": {
      const connectionId = element.getAttribute("bbb-id");
      if (connectionId) {
        toggleSlackNotifications(connectionId, element);
      } else {
        console.warn("slack-toggle-notifications: missing bbb-id attribute");
      }
      break;
    }

    case "slack-refresh":
      loadSlackConnections();
      break;

    default:
      console.log("Unhandled Slack action:", action);
  }
}

/**
 * Load and display Slack connections for the current organisation
 */
async function loadSlackConnections() {
  try {
    const connections = await window.dataBinder.fetchData(
      "/v1/integrations/slack"
    );

    const connectionsList = document.getElementById("slackConnectionsList");
    const emptyState = document.getElementById("slackEmpty");
    const connectButton = document.getElementById("slackConnectButton");

    if (!connectionsList) {
      console.error("Slack connections list element not found");
      return;
    }

    const template = connectionsList.querySelector(
      '[bbb-template="slack-connection"]'
    );

    if (!template) {
      console.error("Slack connection template not found");
      return;
    }

    // Clear existing connections (except template)
    const existingConnections = connectionsList.querySelectorAll(
      '.slack-connection:not([bbb-template="slack-connection"])'
    );
    existingConnections.forEach((el) => el.remove());

    if (!connections || connections.length === 0) {
      if (emptyState) emptyState.style.display = "block";
      if (connectButton) connectButton.style.display = "inline-block";
      return;
    }

    if (emptyState) emptyState.style.display = "none";
    // Still show connect button for additional workspaces
    if (connectButton) connectButton.style.display = "inline-block";

    // Build all connection elements first
    const clones = [];
    for (const conn of connections) {
      const clone = template.cloneNode(true);
      clone.style.display = "block";
      clone.removeAttribute("bbb-template");

      // Set workspace name
      const nameEl = clone.querySelector(".slack-workspace-name");
      if (nameEl)
        nameEl.textContent = conn.workspace_name || "Unknown Workspace";

      // Set connected date
      const dateEl = clone.querySelector(".slack-connected-date");
      if (dateEl)
        dateEl.textContent = `Connected ${formatSlackDate(conn.created_at)}`;

      // Set connection ID on action buttons
      const disconnectBtn = clone.querySelector(
        '[bbb-action="slack-disconnect"]'
      );
      if (disconnectBtn) disconnectBtn.setAttribute("bbb-id", conn.id);

      const linkBtn = clone.querySelector('[bbb-action="slack-link-user"]');
      if (linkBtn) linkBtn.setAttribute("bbb-id", conn.id);

      const unlinkBtn = clone.querySelector('[bbb-action="slack-unlink-user"]');
      if (unlinkBtn) unlinkBtn.setAttribute("bbb-id", conn.id);

      const toggleBtn = clone.querySelector(
        '[bbb-action="slack-toggle-notifications"]'
      );
      if (toggleBtn) toggleBtn.setAttribute("bbb-id", conn.id);

      clones.push({ clone, connectionId: conn.id });
      connectionsList.appendChild(clone);
    }

    // Update user link status in parallel for better performance
    await Promise.all(
      clones.map(({ clone, connectionId }) =>
        updateUserLinkStatus(clone, connectionId)
      )
    );
  } catch (error) {
    console.error("Failed to load Slack connections:", error);
    showSlackError("Failed to load Slack connections");
  }
}

/**
 * Update UI to show current user's link status for a connection
 * @param {HTMLElement} connectionEl - The connection element
 * @param {string} connectionId - The connection ID
 */
async function updateUserLinkStatus(connectionEl, connectionId) {
  try {
    const link = await window.dataBinder.fetchData(
      `/v1/integrations/slack/${connectionId}/user-link`
    );

    const linkBtn = connectionEl.querySelector(
      '[bbb-action="slack-link-user"]'
    );
    const unlinkBtn = connectionEl.querySelector(
      '[bbb-action="slack-unlink-user"]'
    );
    const toggleBtn = connectionEl.querySelector(
      '[bbb-action="slack-toggle-notifications"]'
    );
    const statusEl = connectionEl.querySelector(".slack-link-status");

    if (link && link.id) {
      // User is linked
      if (linkBtn) linkBtn.style.display = "none";
      if (unlinkBtn) unlinkBtn.style.display = "inline-block";
      if (toggleBtn) {
        toggleBtn.style.display = "inline-block";
        toggleBtn.textContent = link.dm_notifications
          ? "Disable notifications"
          : "Enable notifications";
        toggleBtn.classList.toggle("active", link.dm_notifications);
      }
      if (statusEl) {
        statusEl.textContent = link.dm_notifications
          ? "Notifications enabled"
          : "Notifications disabled";
        statusEl.className = `slack-link-status ${link.dm_notifications ? "enabled" : "disabled"}`;
      }
    } else {
      // User is not linked
      if (linkBtn) linkBtn.style.display = "inline-block";
      if (unlinkBtn) unlinkBtn.style.display = "none";
      if (toggleBtn) toggleBtn.style.display = "none";
      if (statusEl) {
        statusEl.textContent = "Not linked";
        statusEl.className = "slack-link-status not-linked";
      }
    }
  } catch (error) {
    // Only treat 404 as "not linked"; log other errors
    if (error.status && error.status !== 404) {
      console.warn("Failed to fetch user link status:", error);
    }
    // User not linked - show link button
    const linkBtn = connectionEl.querySelector(
      '[bbb-action="slack-link-user"]'
    );
    const unlinkBtn = connectionEl.querySelector(
      '[bbb-action="slack-unlink-user"]'
    );
    const toggleBtn = connectionEl.querySelector(
      '[bbb-action="slack-toggle-notifications"]'
    );

    if (linkBtn) linkBtn.style.display = "inline-block";
    if (unlinkBtn) unlinkBtn.style.display = "none";
    if (toggleBtn) toggleBtn.style.display = "none";
  }
}

/**
 * Initiate Slack OAuth flow to connect a new workspace
 */
async function connectSlackWorkspace() {
  try {
    const response = await window.dataBinder.fetchData(
      "/v1/integrations/slack/connect",
      { method: "POST" }
    );

    if (response && response.auth_url) {
      // Redirect to Slack OAuth
      window.location.href = response.auth_url;
    } else {
      throw new Error("No OAuth URL returned");
    }
  } catch (error) {
    console.error("Failed to start Slack OAuth:", error);
    showSlackError("Failed to connect to Slack. Please try again.");
  }
}

/**
 * Disconnect a Slack workspace
 * @param {string} connectionId - The connection ID to disconnect
 */
async function disconnectSlackWorkspace(connectionId) {
  if (
    !confirm(
      "Are you sure you want to disconnect this Slack workspace? All user links will be removed."
    )
  ) {
    return;
  }

  try {
    await window.dataBinder.fetchData(
      `/v1/integrations/slack/${encodeURIComponent(connectionId)}`,
      { method: "DELETE" }
    );

    showSlackSuccess("Slack workspace disconnected");
    loadSlackConnections();
  } catch (error) {
    console.error("Failed to disconnect Slack workspace:", error);
    showSlackError("Failed to disconnect Slack workspace");
  }
}

/**
 * Link current user to Slack for a connection
 * @param {string} connectionId - The connection ID
 */
async function linkSlackUser(connectionId) {
  try {
    await window.dataBinder.fetchData(
      `/v1/integrations/slack/${encodeURIComponent(connectionId)}/link-user`,
      { method: "POST" }
    );

    showSlackSuccess(
      "Your account has been linked to Slack. You will now receive job notifications."
    );
    loadSlackConnections();
  } catch (error) {
    console.error("Failed to link Slack user:", error);
    // Check for 404 or "not found" message to give specific feedback
    if (
      error.status === 404 ||
      (error.message && error.message.toLowerCase().includes("not found"))
    ) {
      showSlackError(
        "Could not find your Slack user. Make sure your email matches your Slack account."
      );
    } else {
      showSlackError("Failed to link your account to Slack");
    }
  }
}

/**
 * Unlink current user from Slack for a connection
 * @param {string} connectionId - The connection ID
 */
async function unlinkSlackUser(connectionId) {
  if (
    !confirm(
      "Are you sure you want to unlink your Slack account? You will no longer receive job notifications."
    )
  ) {
    return;
  }

  try {
    await window.dataBinder.fetchData(
      `/v1/integrations/slack/${encodeURIComponent(connectionId)}/link-user`,
      { method: "DELETE" }
    );

    showSlackSuccess("Your account has been unlinked from Slack");
    loadSlackConnections();
  } catch (error) {
    console.error("Failed to unlink Slack user:", error);
    showSlackError("Failed to unlink your account from Slack");
  }
}

/**
 * Toggle DM notifications for current user
 * @param {string} connectionId - The connection ID
 * @param {HTMLElement} element - The toggle button element
 */
async function toggleSlackNotifications(connectionId, element) {
  try {
    // Get current state from button class
    const currentlyEnabled = element.classList.contains("active");
    const newState = !currentlyEnabled;

    await window.dataBinder.fetchData(
      `/v1/integrations/slack/${encodeURIComponent(connectionId)}/link-user`,
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ dm_notifications: newState }),
      }
    );

    showSlackSuccess(
      newState ? "Notifications enabled" : "Notifications disabled"
    );
    loadSlackConnections();
  } catch (error) {
    console.error("Failed to toggle notifications:", error);
    showSlackError("Failed to update notification settings");
  }
}

/**
 * Show a success message for Slack operations
 * @param {string} message - The message to display
 */
function showSlackSuccess(message) {
  if (window.showDashboardSuccess) {
    window.showDashboardSuccess(message);
  } else {
    alert(message);
  }
}

/**
 * Show an error message for Slack operations
 * @param {string} message - The message to display
 */
function showSlackError(message) {
  if (window.showDashboardError) {
    window.showDashboardError(message);
  } else {
    alert(message);
  }
}

/**
 * Handle OAuth callback result from URL parameters
 */
function handleSlackOAuthCallback() {
  const params = new URLSearchParams(window.location.search);
  const slackConnected = params.get("slack_connected");
  const slackError = params.get("slack_error");

  if (slackConnected) {
    showSlackSuccess(
      `Slack workspace "${slackConnected}" connected successfully!`
    );
    // Clean up URL
    const url = new URL(window.location.href);
    url.searchParams.delete("slack_connected");
    window.history.replaceState({}, "", url.toString());
  } else if (slackError) {
    showSlackError(`Failed to connect Slack: ${slackError}`);
    // Clean up URL
    const url = new URL(window.location.href);
    url.searchParams.delete("slack_error");
    window.history.replaceState({}, "", url.toString());
  }
}

// Export functions to window for use in Webflow
if (typeof window !== "undefined") {
  window.setupSlackIntegration = setupSlackIntegration;
  window.loadSlackConnections = loadSlackConnections;
  window.handleSlackOAuthCallback = handleSlackOAuthCallback;
  window.showSlackSuccess = showSlackSuccess;
  window.showSlackError = showSlackError;
}

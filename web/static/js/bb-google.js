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

    case "google-select-account": {
      const accountId = element.getAttribute("data-account-id");
      if (accountId) {
        selectGoogleAccount(accountId);
      }
      break;
    }

    case "google-save-properties":
      saveGoogleProperties();
      break;

    case "google-cancel-selection":
      hidePropertySelection();
      hideAccountSelection();
      loadGoogleConnections();
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

    // Fetch organisation domains first (needed for domain tags)
    try {
      const { data: { session } = {} } =
        await window.supabase.auth.getSession();
      const token = session?.access_token;

      if (token) {
        const domainsResponse = await fetch("/v1/integrations/google/domains", {
          headers: { Authorization: `Bearer ${token}` },
        });

        if (domainsResponse.ok) {
          const domainsData = await domainsResponse.json();
          organisationDomains = domainsData.domains || [];
          console.log(
            "[GA Debug] Loaded domains for connections:",
            organisationDomains
          );
        }
      }
    } catch (error) {
      console.error("Failed to fetch organisation domains:", error);
      organisationDomains = [];
    }

    const connections = await window.dataBinder.fetchData(
      "/v1/integrations/google"
    );

    const connectionsList = document.getElementById("googleConnectionsList");
    const emptyState = document.getElementById("googleEmptyState");
    const propertySelection = document.getElementById(
      "googlePropertySelection"
    );

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
      // No connections - show empty state message, hide property selection
      if (propertySelection) propertySelection.style.display = "none";
      if (emptyState) emptyState.style.display = "block";
      return;
    }

    // Has connections - hide empty state message AND property selection, show connections
    if (emptyState) emptyState.style.display = "none";
    if (propertySelection) propertySelection.style.display = "none";

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
            removeBtn.title = "Remove domain";
            removeBtn.onclick = (e) => {
              e.stopPropagation();
              removeDomainFromConnection(conn.id, domainId);
            };

            tag.appendChild(removeBtn);
            dateEl.appendChild(tag);
          });
        }

        // Add "Add domain" button
        const addBtn = document.createElement("button");
        addBtn.className = "add-domain-btn";
        addBtn.style.cssText =
          "padding: 4px 8px; background: #f3f4f6; color: #6b7280; border: 1px dashed #d1d5db; border-radius: 4px; font-size: 13px; cursor: pointer;";
        addBtn.textContent = "+ Add domain";
        addBtn.onclick = () =>
          showDomainSelector(conn.id, conn.domain_ids || []);
        dateEl.appendChild(addBtn);
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
 * Select a Google Analytics account and fetch its properties
 * @param {string} accountId - The GA account ID
 */
async function selectGoogleAccount(accountId) {
  console.log("[GA Debug] selectGoogleAccount called with:", accountId);
  try {
    const { data: { session } = {} } = await window.supabase.auth.getSession();
    const token = session?.access_token;
    if (!token) {
      showGoogleError("Not authenticated. Please sign in.");
      return;
    }

    if (!pendingGASessionData || !pendingGASessionData.session_id) {
      console.log("[GA Debug] No pending session data:", pendingGASessionData);
      showGoogleError("OAuth session expired. Please reconnect.");
      hideAccountSelection();
      return;
    }

    console.log("[GA Debug] Session ID:", pendingGASessionData.session_id);
    console.log(
      "[GA Debug] Available accounts:",
      pendingGASessionData.accounts
    );

    // Show loading state
    const accountList = document.getElementById("googleAccountList");
    if (accountList) {
      accountList.innerHTML =
        '<div style="text-align: center; padding: 20px;">Loading properties...</div>';
    }

    // Fetch properties for this account
    const fetchUrl = `/v1/integrations/google/pending-session/${pendingGASessionData.session_id}/accounts/${encodeURIComponent(accountId)}/properties`;
    console.log("[GA Debug] Fetching URL:", fetchUrl);

    const response = await fetch(fetchUrl, {
      headers: { Authorization: `Bearer ${token}` },
    });

    console.log("[GA Debug] Response status:", response.status);

    if (!response.ok) {
      const text = await response.text();
      console.log("[GA Debug] Error response:", text);
      throw new Error(text || `HTTP ${response.status}`);
    }

    const result = await response.json();
    console.log("[GA Debug] Full response:", JSON.stringify(result, null, 2));
    const properties = result.data?.properties || [];

    // Store selected account and properties
    pendingGASessionData.selected_account_id = accountId;
    pendingGASessionData.properties = properties;

    // Fetch organisation's domains for domain selection
    try {
      const domainsResponse = await fetch("/v1/integrations/google/domains", {
        headers: { Authorization: `Bearer ${token}` },
      });

      if (domainsResponse.ok) {
        const domainsData = await domainsResponse.json();
        organisationDomains = domainsData.data.domains || [];
        console.log("[GA Debug] Fetched domains:", organisationDomains);
      } else {
        console.warn(
          "[GA Debug] Failed to fetch domains, continuing without them"
        );
        organisationDomains = [];
      }
    } catch (error) {
      console.error("Failed to fetch organisation domains:", error);
      organisationDomains = [];
    }

    // Hide account selection, show property selection
    hideAccountSelection();
    showPropertySelection(properties);
  } catch (error) {
    console.error("Failed to fetch properties for account:", error);
    showGoogleError("Failed to load properties. Please try again.");
  }
}

/**
 * Save all properties (bulk save with active/inactive status)
 */
async function saveGoogleProperties() {
  try {
    const { data: { session } = {} } = await window.supabase.auth.getSession();
    const token = session?.access_token;
    if (!token) {
      showGoogleError("Not authenticated. Please sign in.");
      return;
    }

    if (!pendingGASessionData) {
      showGoogleError("OAuth session expired. Please reconnect.");
      hidePropertySelection();
      return;
    }

    // Get selected (active) property IDs
    const selectedItems = document.querySelectorAll(
      "#googlePropertyList .selected[data-property-id]"
    );
    const activePropertyIds = Array.from(selectedItems).map((item) =>
      item.getAttribute("data-property-id")
    );

    // Build property -> domain_ids mapping from temp storage
    const propertyDomainMap = {};
    allGoogleProperties.forEach((property) => {
      const isActive = activePropertyIds.includes(property.property_id);
      if (!isActive) {
        propertyDomainMap[property.property_id] = [];
        return;
      }

      // Get selected domains from temporary storage
      const domainIds =
        window.tempPropertyDomains?.[property.property_id] || [];
      propertyDomainMap[property.property_id] = domainIds;
    });

    console.log("[GA Debug] Property domain mapping:", propertyDomainMap);

    // Show saving state
    const saveBtn = document.querySelector(
      '[bbb-action="google-save-properties"]'
    );
    if (saveBtn) {
      saveBtn.disabled = true;
      saveBtn.textContent = "Saving...";
    }

    const response = await fetch("/v1/integrations/google/save-properties", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        session_id: pendingGASessionData.session_id,
        account_id:
          pendingGASessionData.selected_account_id ||
          pendingGASessionData.accounts?.[0]?.account_id,
        active_property_ids: activePropertyIds,
        property_domain_map: propertyDomainMap,
      }),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || `HTTP ${response.status}`);
    }

    // Clear stored session data
    pendingGASessionData = null;

    hidePropertySelection();
    const activeCount = activePropertyIds.length;
    const totalCount = allGoogleProperties.length;

    // Clear temporary domain selections
    window.tempPropertyDomains = {};

    showGoogleSuccess(
      `Saved ${totalCount} properties (${activeCount} active, ${totalCount - activeCount} inactive)`
    );
    loadGoogleConnections();
  } catch (error) {
    console.error("Failed to save Google properties:", error);
    showGoogleError("Failed to save properties");
  } finally {
    const saveBtn = document.querySelector(
      '[bbb-action="google-save-properties"]'
    );
    if (saveBtn) {
      saveBtn.disabled = false;
      saveBtn.textContent = "Save Properties";
    }
  }
}

/**
 * Toggle an existing connection's status (active/inactive)
 * @param {string} connectionId - The connection ID
 * @param {boolean} active - Whether to set active
 */
async function toggleConnectionStatus(connectionId, active) {
  console.log(
    `[GA Toggle] Toggling ${connectionId} to ${active ? "active" : "inactive"}`
  );

  try {
    const { data: { session } = {} } = await window.supabase.auth.getSession();
    const token = session?.access_token;
    if (!token) {
      console.error("[GA Toggle] No auth token available");
      showGoogleError("Not authenticated. Please sign in.");
      return;
    }

    console.log(
      `[GA Toggle] Making PATCH request to /v1/integrations/google/${connectionId}/status`
    );
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
      console.error(`[GA Toggle] API error: ${response.status}`, text);
      throw new Error(text || `HTTP ${response.status}`);
    }

    console.log("[GA Toggle] Status updated successfully");
    // Reload to update UI
    loadGoogleConnections();
  } catch (error) {
    console.error("[GA Toggle] Failed to toggle connection status:", error);
    showGoogleError("Failed to update status");
    loadGoogleConnections(); // Reload to reset toggle state
  }
}

// Store all properties for filtering
let allGoogleProperties = [];
let organisationDomains = [];
const MAX_VISIBLE_PROPERTIES = 10;

/**
 * Render filtered property list with toggle selection
 * @param {Array} properties - Filtered properties to display
 * @param {number} totalCount - Total number of properties before filtering
 */
function renderPropertyList(properties, totalCount) {
  const list = document.getElementById("googlePropertyList");
  if (!list) return;

  // Clear existing items
  while (list.firstChild) {
    list.removeChild(list.firstChild);
  }

  // Show count info and instructions
  const countInfo = document.createElement("div");
  countInfo.style.cssText =
    "color: #6b7280; font-size: 13px; margin-bottom: 12px;";
  if (properties.length === 0) {
    countInfo.textContent = "No properties match your search";
  } else if (properties.length < totalCount) {
    countInfo.textContent = `Showing ${properties.length} of ${totalCount} properties. Click to toggle active/inactive.`;
  } else {
    countInfo.textContent = `${totalCount} properties found. Click to toggle active/inactive.`;
  }
  list.appendChild(countInfo);

  // Add property options with toggle functionality
  for (const prop of properties) {
    const item = document.createElement("div");
    item.className = "bb-job-card";
    item.style.cssText =
      "display: flex; align-items: center; width: 100%; margin-bottom: 8px; padding: 12px 16px; background: #f8f9fa; border: 1px solid #e9ecef; border-radius: 8px;";
    item.setAttribute("data-property-id", prop.property_id);

    // Property details
    const details = document.createElement("div");
    details.style.cssText = "flex: 1;";

    const strongEl = document.createElement("strong");
    strongEl.textContent = prop.display_name;
    strongEl.style.fontSize = "15px";
    details.appendChild(strongEl);

    const detailSpan = document.createElement("span");
    detailSpan.style.cssText =
      "color: #6b7280; font-size: 13px; display: block; margin-top: 2px;";
    detailSpan.textContent = `Property ID: ${prop.property_id}`;
    details.appendChild(detailSpan);

    item.appendChild(details);

    // Toggle switch
    const toggleLabel = document.createElement("label");
    toggleLabel.className = "property-toggle-container";
    toggleLabel.style.cssText =
      "display: inline-flex; align-items: center; cursor: pointer; user-select: none;";

    const toggleInput = document.createElement("input");
    toggleInput.type = "checkbox";
    toggleInput.className = "property-status-toggle";
    toggleInput.style.display = "none";
    toggleInput.setAttribute("data-property-id", prop.property_id);

    const track = document.createElement("div");
    track.className = "property-toggle-track";
    track.style.cssText =
      "position: relative; width: 44px; height: 24px; background-color: #d1d5db; border-radius: 12px; transition: background-color 0.2s;";

    const thumb = document.createElement("div");
    thumb.className = "property-toggle-thumb";
    thumb.style.cssText =
      "position: absolute; top: 2px; left: 2px; width: 20px; height: 20px; background-color: white; border-radius: 10px; transition: transform 0.2s; box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);";

    track.appendChild(thumb);
    toggleLabel.appendChild(toggleInput);
    toggleLabel.appendChild(track);
    item.appendChild(toggleLabel);

    // Add click handler
    toggleLabel.addEventListener("click", (e) => {
      e.preventDefault();
      const newActive = !toggleInput.checked;
      toggleInput.checked = newActive;

      if (newActive) {
        track.style.backgroundColor = "#10b981";
        thumb.style.transform = "translateX(20px)";
        item.classList.add("selected");
      } else {
        track.style.backgroundColor = "#d1d5db";
        thumb.style.transform = "translateX(0)";
        item.classList.remove("selected");
      }

      // Show/hide domain selection based on active state
      const domainSection = item.querySelector(".domain-selection-section");
      if (domainSection) {
        domainSection.style.display = newActive ? "block" : "none";
      }
    });

    // Create domain selection section
    const domainSection = document.createElement("div");
    domainSection.className = "domain-selection-section";
    domainSection.style.cssText =
      "display: none; margin-top: 12px; padding: 12px; background-color: #f9fafb; border-radius: 6px; border: 1px solid #e5e7eb;";
    domainSection.setAttribute("data-property-id", prop.property_id);

    const domainHeader = document.createElement("div");
    domainHeader.style.cssText =
      "font-size: 14px; font-weight: 500; color: #374151; margin-bottom: 8px;";
    domainHeader.textContent = "Select domains tracked by this property:";
    domainSection.appendChild(domainHeader);

    // Search input container
    const inputContainer = document.createElement("div");
    inputContainer.style.cssText = "position: relative; margin-bottom: 8px;";

    const searchInput = document.createElement("input");
    searchInput.type = "text";
    searchInput.placeholder = "Search or add new domain...";
    searchInput.style.cssText =
      "width: 100%; padding: 8px 12px; border: 1px solid #d1d5db; border-radius: 6px; font-size: 14px; box-sizing: border-box;";
    searchInput.setAttribute("data-property-id", prop.property_id);

    // Dropdown list
    const dropdown = document.createElement("div");
    dropdown.style.cssText =
      "display: none; position: absolute; top: 100%; left: 0; right: 0; max-height: 200px; overflow-y: auto; background: white; border: 1px solid #d1d5db; border-radius: 6px; margin-top: 4px; z-index: 1000; box-shadow: 0 2px 8px rgba(0,0,0,0.1);";

    // Selected domains display
    const selectedContainer = document.createElement("div");
    selectedContainer.style.cssText =
      "display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; min-height: 24px;";
    selectedContainer.id = `domains-for-${prop.property_id}`;

    // Initialize temp storage for this property
    if (!window.tempPropertyDomains) window.tempPropertyDomains = {};
    let selectedDomainIds = window.tempPropertyDomains[prop.property_id] || [];

    // Function to render selected tags
    const renderSelectedTags = () => {
      while (selectedContainer.firstChild) {
        selectedContainer.removeChild(selectedContainer.firstChild);
      }

      selectedDomainIds.forEach((domainId) => {
        const domain = organisationDomains.find((d) => d.id === domainId);
        if (!domain) return;

        const tag = document.createElement("span");
        tag.style.cssText =
          "display: inline-flex; align-items: center; gap: 4px; padding: 4px 8px; background: #e0e7ff; color: #3730a3; border-radius: 4px; font-size: 13px;";
        tag.textContent = domain.name;

        const removeBtn = document.createElement("button");
        removeBtn.textContent = "×";
        removeBtn.style.cssText =
          "background: none; border: none; color: #6366f1; font-size: 16px; cursor: pointer; padding: 0; margin-left: 2px;";
        removeBtn.onclick = () => {
          selectedDomainIds = selectedDomainIds.filter((id) => id !== domainId);
          window.tempPropertyDomains[prop.property_id] = selectedDomainIds;
          renderSelectedTags();
        };

        tag.appendChild(removeBtn);
        selectedContainer.appendChild(tag);
      });
    };

    // Function to filter and render dropdown options
    const renderDropdown = (query) => {
      while (dropdown.firstChild) {
        dropdown.removeChild(dropdown.firstChild);
      }

      const lowerQuery = query.toLowerCase().trim();

      console.log(
        "[GA Debug] renderDropdown called:",
        "query=",
        query,
        "organisationDomains=",
        organisationDomains
      );

      // Filter domains that aren't already selected
      const availableDomains = organisationDomains.filter(
        (d) => !selectedDomainIds.includes(d.id)
      );

      // Filter by search query
      const filtered = lowerQuery
        ? availableDomains.filter((d) =>
            d.name.toLowerCase().includes(lowerQuery)
          )
        : availableDomains;

      // Show options
      if (filtered.length > 0) {
        filtered.forEach((domain) => {
          const option = document.createElement("div");
          option.textContent = domain.name;
          option.style.cssText =
            "padding: 10px 16px; cursor: pointer; font-size: 14px; border-bottom: 1px solid #f3f4f6;";
          option.onmouseover = () => {
            option.style.background = "#f9fafb";
          };
          option.onmouseout = () => {
            option.style.background = "white";
          };
          option.onclick = () => {
            if (!selectedDomainIds.includes(domain.id)) {
              selectedDomainIds.push(domain.id);
              window.tempPropertyDomains[prop.property_id] = selectedDomainIds;
              renderSelectedTags();
            }
            searchInput.value = "";
            dropdown.style.display = "none";
          };
          dropdown.appendChild(option);
        });
        dropdown.style.display = "block";
      } else if (lowerQuery) {
        // Show "Add new domain" option
        const addOption = document.createElement("div");
        addOption.textContent = `Add new domain: ${lowerQuery}`;
        addOption.style.cssText =
          "padding: 10px 16px; cursor: pointer; font-size: 14px; color: #6366f1; font-weight: 500;";
        addOption.onmouseover = () => {
          addOption.style.background = "#f9fafb";
        };
        addOption.onmouseout = () => {
          addOption.style.background = "white";
        };
        addOption.onclick = async () => {
          await createDomainInline(lowerQuery, prop.property_id);
          searchInput.value = "";
          dropdown.style.display = "none";
        };
        dropdown.appendChild(addOption);
        dropdown.style.display = "block";
      } else {
        dropdown.style.display = "none";
      }
    };

    // Create domain function
    const createDomainInline = async (domainName, propertyId) => {
      try {
        const { data: { session } = {} } =
          await window.supabase.auth.getSession();
        const token = session?.access_token;

        if (!token) {
          showGoogleError("Please sign in to create domains");
          return;
        }

        const response = await fetch("/v1/jobs", {
          method: "POST",
          headers: {
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            domain: domainName,
            source_type: "sitemap",
            concurrency: 1,
            max_pages: 10,
          }),
        });

        if (!response.ok) {
          throw new Error("Failed to create domain");
        }

        const result = await response.json();
        const newDomainId = result.domain_id;

        organisationDomains.push({ id: newDomainId, name: domainName });

        if (!selectedDomainIds.includes(newDomainId)) {
          selectedDomainIds.push(newDomainId);
          window.tempPropertyDomains[propertyId] = selectedDomainIds;
          renderSelectedTags();
        }
      } catch (error) {
        console.error("Failed to create domain:", error);
        showGoogleError("Failed to create domain. Please try again.");
      }
    };

    // Event listeners
    searchInput.addEventListener("focus", () => {
      renderDropdown(searchInput.value);
    });

    searchInput.addEventListener("input", () => {
      renderDropdown(searchInput.value);
    });

    searchInput.addEventListener("click", (e) => {
      e.stopPropagation();
    });

    document.addEventListener("click", (e) => {
      if (!inputContainer.contains(e.target)) {
        dropdown.style.display = "none";
      }
    });

    inputContainer.appendChild(searchInput);
    inputContainer.appendChild(dropdown);
    domainSection.appendChild(inputContainer);
    domainSection.appendChild(selectedContainer);

    // Render initial tags
    renderSelectedTags();

    item.appendChild(domainSection);

    list.appendChild(item);
  }

  // Add save button if not already present
  let saveContainer = document.getElementById("googlePropertySaveContainer");
  if (!saveContainer && properties.length > 0) {
    saveContainer = document.createElement("div");
    saveContainer.id = "googlePropertySaveContainer";
    saveContainer.style.cssText =
      "margin-top: 16px; padding-top: 16px; border-top: 1px solid #e5e7eb;";

    const saveBtn = document.createElement("button");
    saveBtn.className = "bb-button bb-button-primary";
    saveBtn.setAttribute("bbb-action", "google-save-properties");
    saveBtn.style.cssText = "width: 100%; padding: 12px;";
    saveBtn.textContent = "Save Properties";
    saveContainer.appendChild(saveBtn);

    const cancelBtn = document.createElement("button");
    cancelBtn.className = "bb-button";
    cancelBtn.setAttribute("bbb-action", "google-cancel-selection");
    cancelBtn.style.cssText =
      "width: 100%; padding: 12px; margin-top: 8px; background: transparent;";
    cancelBtn.textContent = "Cancel";
    saveContainer.appendChild(cancelBtn);

    list.parentNode.appendChild(saveContainer);
  }
}

/**
 * Filter properties based on search query
 * @param {string} query - Search query
 */
function filterGoogleProperties(query) {
  const lowerQuery = query.toLowerCase().trim();
  if (!lowerQuery) {
    renderPropertyList(allGoogleProperties, allGoogleProperties.length);
    return;
  }

  const filtered = allGoogleProperties.filter(
    (prop) =>
      prop.display_name?.toLowerCase().includes(lowerQuery) ||
      prop.property_id?.toLowerCase().includes(lowerQuery) ||
      prop.account_name?.toLowerCase().includes(lowerQuery)
  );
  renderPropertyList(filtered, allGoogleProperties.length);
}

/**
 * Show property selection UI when multiple properties are available
 * @param {Array} properties - Array of GA4 properties to choose from
 */
function showPropertySelection(properties) {
  console.log(
    "[GA OAuth] showPropertySelection called with",
    properties?.length,
    "properties"
  );
  const selectionUI = document.getElementById("googlePropertySelection");
  const list = document.getElementById("googlePropertyList");

  console.log("[GA OAuth] DOM elements found:", {
    selectionUI: !!selectionUI,
    list: !!list,
  });

  if (!selectionUI || !list) {
    console.error("Property selection UI not found");
    return;
  }

  // Store all properties for filtering
  allGoogleProperties = properties;

  // Add search input if not already present
  let searchContainer = document.getElementById("googlePropertySearch");
  if (!searchContainer) {
    searchContainer = document.createElement("div");
    searchContainer.id = "googlePropertySearch";
    searchContainer.style.cssText = "margin-bottom: 16px;";

    const searchInput = document.createElement("input");
    searchInput.type = "text";
    searchInput.placeholder = "Search properties...";
    searchInput.style.cssText =
      "width: 100%; padding: 10px 12px; border: 1px solid #d1d5db; border-radius: 6px; font-size: 14px;";
    searchInput.addEventListener("input", (e) => {
      filterGoogleProperties(e.target.value);
    });

    searchContainer.appendChild(searchInput);
    list.parentNode.insertBefore(searchContainer, list);
  } else {
    // Clear existing search
    const input = searchContainer.querySelector("input");
    if (input) input.value = "";
  }

  // Render initial list (max 10)
  renderPropertyList(properties, properties.length);

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
  // Clear search input if present
  const searchInput = document.querySelector("#googlePropertySearch input");
  if (searchInput) {
    searchInput.value = "";
  }
  // Remove save container if present
  const saveContainer = document.getElementById("googlePropertySaveContainer");
  if (saveContainer) {
    saveContainer.remove();
  }
  // Clear stored properties
  allGoogleProperties = [];
}

/**
 * Show account selection UI when multiple accounts are available
 * @param {Array} accounts - Array of GA accounts to choose from
 */
function showAccountSelection(accounts) {
  console.log(
    "[GA OAuth] showAccountSelection called with",
    accounts?.length,
    "accounts"
  );

  // Create or get the account selection UI
  let accountUI = document.getElementById("googleAccountSelection");
  if (!accountUI) {
    // Create the UI dynamically
    const propertySelection = document.getElementById(
      "googlePropertySelection"
    );
    if (propertySelection) {
      accountUI = document.createElement("div");
      accountUI.id = "googleAccountSelection";
      accountUI.style.cssText = "padding: 16px;";
      propertySelection.parentNode.insertBefore(accountUI, propertySelection);
    } else {
      console.error("Cannot find googlePropertySelection to insert account UI");
      return;
    }
  }

  // Build the account list
  accountUI.innerHTML = `
    <h3 style="margin: 0 0 8px 0; font-size: 16px; font-weight: 600;">Select Google Analytics Account</h3>
    <p style="color: #6b7280; font-size: 14px; margin: 0 0 16px 0;">
      You have access to ${accounts.length} accounts. Select one to view its properties.
    </p>
    <div id="googleAccountList"></div>
  `;

  const list = document.getElementById("googleAccountList");
  for (const account of accounts) {
    const item = document.createElement("button");
    item.type = "button";
    item.className = "bb-button";
    item.style.cssText =
      "display: block; width: 100%; text-align: left; margin-bottom: 8px; padding: 12px 16px; cursor: pointer;";
    item.setAttribute("bbb-action", "google-select-account");
    item.setAttribute("data-account-id", account.account_id);

    const strongEl = document.createElement("strong");
    strongEl.textContent = account.display_name || account.account_id;
    item.appendChild(strongEl);

    const detailSpan = document.createElement("span");
    detailSpan.style.cssText =
      "color: #6b7280; font-size: 13px; display: block;";
    detailSpan.textContent = `Account ID: ${account.account_id.replace("accounts/", "")}`;
    item.appendChild(detailSpan);

    list.appendChild(item);
  }

  // Add cancel button
  const cancelBtn = document.createElement("button");
  cancelBtn.className = "bb-button";
  cancelBtn.setAttribute("bbb-action", "google-cancel-selection");
  cancelBtn.style.cssText =
    "width: 100%; padding: 12px; margin-top: 8px; background: transparent;";
  cancelBtn.textContent = "Cancel";
  list.appendChild(cancelBtn);

  // Hide empty state and show account selection
  const emptyState = document.getElementById("googleEmptyState");
  if (emptyState) emptyState.style.display = "none";
  accountUI.style.display = "block";
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

/**
 * Handle OAuth callback result checks
 */
async function handleGoogleOAuthCallback() {
  const params = new URLSearchParams(window.location.search);
  const googleConnected = params.get("google_connected");
  const googleError = params.get("google_error");
  const gaSession = params.get("ga_session");

  console.log("[GA OAuth] Checking URL params:", {
    googleConnected,
    googleError,
    gaSession,
  });

  if (googleConnected) {
    // Clean up URL
    const url = new URL(window.location.href);
    url.searchParams.delete("google_connected");
    window.history.replaceState({}, "", url.toString());

    showGoogleSuccess("Google Analytics connected successfully!");
    loadGoogleConnections();
  } else if (gaSession) {
    // Fetch session data from server
    console.log("[GA OAuth] Found ga_session, fetching from server...");
    try {
      const { data: { session } = {} } =
        await window.supabase.auth.getSession();
      const token = session?.access_token;
      console.log("[GA OAuth] Supabase token:", token ? "present" : "missing");
      if (!token) {
        showGoogleError("Not authenticated. Please sign in.");
        return;
      }

      console.log("[GA OAuth] Fetching pending session:", gaSession);
      const response = await fetch(
        `/v1/integrations/google/pending-session/${gaSession}`,
        {
          headers: { Authorization: `Bearer ${token}` },
        }
      );

      console.log("[GA OAuth] Response status:", response.status);
      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || `HTTP ${response.status}`);
      }

      const result = await response.json();
      const sessionData = result.data;
      console.log("[GA OAuth] Session data received:", {
        hasAccounts: !!sessionData?.accounts,
        accountCount: sessionData?.accounts?.length,
        hasProperties: !!sessionData?.properties,
        propertyCount: sessionData?.properties?.length,
        email: sessionData?.email,
      });

      // Store session ID for subsequent requests
      sessionData.session_id = gaSession;
      pendingGASessionData = sessionData;

      // Open notifications modal (contains Google Analytics section)
      const notificationsModal = document.getElementById("notificationsModal");
      console.log("[GA OAuth] Opening modal:", !!notificationsModal);
      if (notificationsModal) {
        notificationsModal.classList.add("show");
      }

      // Determine which UI to show based on session data
      const accounts = sessionData.accounts || [];
      const properties = sessionData.properties || [];

      if (accounts.length > 1 && properties.length === 0) {
        // Multiple accounts, no properties yet - show account picker
        console.log("[GA OAuth] Multiple accounts, showing account picker");
        showAccountSelection(accounts);
      } else if (properties.length > 0) {
        // Single account with properties already fetched, or properties from selected account
        console.log(
          "[GA OAuth] Showing property selection with",
          properties.length,
          "properties"
        );
        showPropertySelection(properties);
      } else if (accounts.length === 1) {
        // Single account but no properties - should not happen normally
        console.log("[GA OAuth] Single account, no properties - fetching...");
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
async function removeDomainFromConnection(connectionId, domainId) {
  try {
    const { data: { session } = {} } = await window.supabase.auth.getSession();
    const token = session?.access_token;

    if (!token) {
      showGoogleError("Please sign in to update connections");
      return;
    }

    // Get current connection to find existing domain_ids
    const connections = await window.dataBinder.fetchData(
      "/v1/integrations/google"
    );
    const connection = connections.find((c) => c.id === connectionId);

    if (!connection) {
      showGoogleError("Connection not found");
      return;
    }

    // Remove the domain from the array
    const updatedDomainIds = (connection.domain_ids || []).filter(
      (id) => id !== domainId
    );

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

/**
 * Show domain selector modal for adding domains to a connection
 * @param {string} connectionId - The connection ID
 * @param {Array<number>} currentDomainIds - Currently selected domain IDs
 */
async function showDomainSelector(connectionId, currentDomainIds) {
  // Create modal overlay
  const overlay = document.createElement("div");
  overlay.style.cssText =
    "position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 10000;";

  // Create modal container
  const modal = document.createElement("div");
  modal.style.cssText =
    "background: white; border-radius: 8px; padding: 24px; width: 90%; max-width: 500px; box-shadow: 0 4px 12px rgba(0,0,0,0.15);";

  // Header
  const header = document.createElement("h3");
  header.textContent = "Add Domains to Property";
  header.style.cssText = "margin: 0 0 16px; font-size: 18px; font-weight: 600;";

  // Search input container
  const inputContainer = document.createElement("div");
  inputContainer.style.cssText = "position: relative; margin-bottom: 16px;";

  const searchInput = document.createElement("input");
  searchInput.type = "text";
  searchInput.placeholder = "Search or add new domain...";
  searchInput.style.cssText =
    "width: 100%; padding: 12px 16px; border: 1px solid #d1d5db; border-radius: 6px; font-size: 14px; box-sizing: border-box;";

  // Dropdown list
  const dropdown = document.createElement("div");
  dropdown.style.cssText =
    "display: none; position: absolute; top: 100%; left: 0; right: 0; max-height: 200px; overflow-y: auto; background: white; border: 1px solid #d1d5db; border-radius: 6px; margin-top: 4px; z-index: 1000; box-shadow: 0 2px 8px rgba(0,0,0,0.1);";

  // Selected domains display
  const selectedContainer = document.createElement("div");
  selectedContainer.style.cssText =
    "display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 16px; min-height: 32px;";

  // Track selected domain IDs
  let selectedDomainIds = [...currentDomainIds];

  // Function to render selected tags
  const renderSelectedTags = () => {
    // Clear existing tags
    while (selectedContainer.firstChild) {
      selectedContainer.removeChild(selectedContainer.firstChild);
    }

    selectedDomainIds.forEach((domainId) => {
      const domain = organisationDomains.find((d) => d.id === domainId);
      if (!domain) return;

      const tag = document.createElement("span");
      tag.style.cssText =
        "display: inline-flex; align-items: center; gap: 4px; padding: 4px 8px; background: #e0e7ff; color: #3730a3; border-radius: 4px; font-size: 13px;";
      tag.textContent = domain.name;

      const removeBtn = document.createElement("button");
      removeBtn.textContent = "×";
      removeBtn.style.cssText =
        "background: none; border: none; color: #6366f1; font-size: 16px; cursor: pointer; padding: 0; margin-left: 2px;";
      removeBtn.onclick = () => {
        selectedDomainIds = selectedDomainIds.filter((id) => id !== domainId);
        renderSelectedTags();
      };

      tag.appendChild(removeBtn);
      selectedContainer.appendChild(tag);
    });
  };

  // Function to filter and render dropdown options
  const renderDropdown = (query) => {
    // Clear dropdown
    while (dropdown.firstChild) {
      dropdown.removeChild(dropdown.firstChild);
    }

    const lowerQuery = query.toLowerCase().trim();

    // Filter domains that aren't already selected
    const availableDomains = organisationDomains.filter(
      (d) => !selectedDomainIds.includes(d.id)
    );

    // Filter by search query
    const filtered = lowerQuery
      ? availableDomains.filter((d) =>
          d.name.toLowerCase().includes(lowerQuery)
        )
      : availableDomains;

    // Show options
    if (filtered.length > 0) {
      filtered.forEach((domain) => {
        const option = document.createElement("div");
        option.textContent = domain.name;
        option.style.cssText =
          "padding: 10px 16px; cursor: pointer; font-size: 14px; border-bottom: 1px solid #f3f4f6;";
        option.onmouseover = () => {
          option.style.background = "#f9fafb";
        };
        option.onmouseout = () => {
          option.style.background = "white";
        };
        option.onclick = () => {
          if (!selectedDomainIds.includes(domain.id)) {
            selectedDomainIds.push(domain.id);
            renderSelectedTags();
          }
          searchInput.value = "";
          dropdown.style.display = "none";
        };
        dropdown.appendChild(option);
      });
      dropdown.style.display = "block";
    } else if (lowerQuery) {
      // Show "Add new domain" option
      const addOption = document.createElement("div");
      addOption.textContent = `Add new domain: ${lowerQuery}`;
      addOption.style.cssText =
        "padding: 10px 16px; cursor: pointer; font-size: 14px; color: #6366f1; font-weight: 500;";
      addOption.onmouseover = () => {
        addOption.style.background = "#f9fafb";
      };
      addOption.onmouseout = () => {
        addOption.style.background = "white";
      };
      addOption.onclick = async () => {
        await createAndSelectDomain(lowerQuery);
        searchInput.value = "";
        dropdown.style.display = "none";
      };
      dropdown.appendChild(addOption);
      dropdown.style.display = "block";
    } else {
      dropdown.style.display = "none";
    }
  };

  // Function to create a new domain and add it to selection
  const createAndSelectDomain = async (domainName) => {
    try {
      const { data: { session } = {} } =
        await window.supabase.auth.getSession();
      const token = session?.access_token;

      if (!token) {
        showGoogleError("Please sign in to create domains");
        return;
      }

      // Create domain via job creation endpoint (reusing existing logic)
      const response = await fetch("/v1/jobs", {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          domain: domainName,
          source_type: "sitemap", // Default values just to create domain
          concurrency: 1,
          max_pages: 10,
        }),
      });

      if (!response.ok) {
        throw new Error("Failed to create domain");
      }

      const result = await response.json();
      const newDomainId = result.domain_id;

      // Add to organisationDomains array
      organisationDomains.push({ id: newDomainId, name: domainName });

      // Add to selected domains
      selectedDomainIds.push(newDomainId);
      renderSelectedTags();
    } catch (error) {
      console.error("Failed to create domain:", error);
      showGoogleError("Failed to create domain. Please try again.");
    }
  };

  // Event listeners
  searchInput.addEventListener("focus", () => {
    renderDropdown(searchInput.value);
  });

  searchInput.addEventListener("input", () => {
    renderDropdown(searchInput.value);
  });

  // Prevent click inside input from closing dropdown
  searchInput.addEventListener("click", (e) => {
    e.stopPropagation();
  });

  // Close dropdown when clicking outside
  document.addEventListener("click", (e) => {
    if (!inputContainer.contains(e.target)) {
      dropdown.style.display = "none";
    }
  });

  // Buttons container
  const buttonsContainer = document.createElement("div");
  buttonsContainer.style.cssText =
    "display: flex; gap: 8px; justify-content: flex-end; margin-top: 16px;";

  const cancelBtn = document.createElement("button");
  cancelBtn.textContent = "Cancel";
  cancelBtn.style.cssText =
    "padding: 8px 16px; background: #f3f4f6; border: none; border-radius: 6px; cursor: pointer; font-size: 14px;";
  cancelBtn.onclick = () => {
    document.body.removeChild(overlay);
  };

  const saveBtn = document.createElement("button");
  saveBtn.textContent = "Save";
  saveBtn.style.cssText =
    "padding: 8px 16px; background: #6366f1; color: white; border: none; border-radius: 6px; cursor: pointer; font-size: 14px;";
  saveBtn.onclick = async () => {
    await saveDomainSelection(connectionId, selectedDomainIds);
    document.body.removeChild(overlay);
  };

  // Assemble modal
  inputContainer.appendChild(searchInput);
  inputContainer.appendChild(dropdown);
  buttonsContainer.appendChild(cancelBtn);
  buttonsContainer.appendChild(saveBtn);

  modal.appendChild(header);
  modal.appendChild(selectedContainer);
  modal.appendChild(inputContainer);
  modal.appendChild(buttonsContainer);
  overlay.appendChild(modal);

  document.body.appendChild(overlay);

  // Render initial selected tags
  renderSelectedTags();

  // Focus input
  searchInput.focus();
}

/**
 * Save domain selection for a connection
 * @param {string} connectionId - The connection ID
 * @param {Array<number>} domainIds - Selected domain IDs
 */
async function saveDomainSelection(connectionId, domainIds) {
  try {
    const { data: { session } = {} } = await window.supabase.auth.getSession();
    const token = session?.access_token;

    if (!token) {
      showGoogleError("Please sign in to update connections");
      return;
    }

    // Use dedicated PATCH endpoint to update domains
    const response = await fetch(`/v1/integrations/google/${connectionId}`, {
      method: "PATCH",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        domain_ids: domainIds, // Send array directly
      }),
    });

    if (!response.ok) {
      throw new Error(`Failed to update connection: ${response.status}`);
    }

    // Reload connections
    await loadGoogleConnections();
  } catch (error) {
    console.error("Failed to save domain selection:", error);
    showGoogleError("Failed to save domains. Please try again.");
  }
}

/**
 * Update domain tags display for a property during initial setup
 * @param {string} propertyId - GA4 property ID
 */
function updateDomainTags(propertyId) {
  const container = document.getElementById(`domains-for-${propertyId}`);
  if (!container) return;

  // Clear existing tags
  while (container.firstChild) {
    container.removeChild(container.firstChild);
  }

  const domainIds = window.tempPropertyDomains?.[propertyId] || [];
  domainIds.forEach((domainId) => {
    const domain = organisationDomains.find((d) => d.id === domainId);
    if (!domain) return;

    const tag = document.createElement("span");
    tag.style.cssText =
      "display: inline-flex; align-items: center; gap: 4px; padding: 4px 8px; background: #e0e7ff; color: #3730a3; border-radius: 4px; font-size: 13px;";
    tag.textContent = domain.name;

    const removeBtn = document.createElement("button");
    removeBtn.textContent = "×";
    removeBtn.style.cssText =
      "background: none; border: none; color: #6366f1; font-size: 16px; cursor: pointer; padding: 0; margin-left: 2px;";
    removeBtn.onclick = () => {
      // Remove from temp storage
      if (!window.tempPropertyDomains) window.tempPropertyDomains = {};
      window.tempPropertyDomains[propertyId] = (
        window.tempPropertyDomains[propertyId] || []
      ).filter((id) => id !== domainId);
      updateDomainTags(propertyId);
    };

    tag.appendChild(removeBtn);
    container.appendChild(tag);
  });
}

/**
 * Show domain selector modal for a property during initial setup
 * @param {string} propertyId - GA4 property ID
 * @param {Array<number>} currentDomainIds - Currently selected domain IDs
 */
async function showDomainSelectorForProperty(propertyId, currentDomainIds) {
  // Create modal overlay
  const overlay = document.createElement("div");
  overlay.style.cssText =
    "position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 10000;";

  // Create modal container
  const modal = document.createElement("div");
  modal.style.cssText =
    "background: white; border-radius: 8px; padding: 24px; width: 90%; max-width: 500px; box-shadow: 0 4px 12px rgba(0,0,0,0.15);";

  // Header
  const header = document.createElement("h3");
  header.textContent = "Select Domains for Property";
  header.style.cssText = "margin: 0 0 16px; font-size: 18px; font-weight: 600;";

  // Search input container
  const inputContainer = document.createElement("div");
  inputContainer.style.cssText = "position: relative; margin-bottom: 16px;";

  const searchInput = document.createElement("input");
  searchInput.type = "text";
  searchInput.placeholder = "Search or add new domain...";
  searchInput.style.cssText =
    "width: 100%; padding: 12px 16px; border: 1px solid #d1d5db; border-radius: 6px; font-size: 14px; box-sizing: border-box;";

  // Dropdown list
  const dropdown = document.createElement("div");
  dropdown.style.cssText =
    "display: none; position: absolute; top: 100%; left: 0; right: 0; max-height: 200px; overflow-y: auto; background: white; border: 1px solid #d1d5db; border-radius: 6px; margin-top: 4px; z-index: 1000; box-shadow: 0 2px 8px rgba(0,0,0,0.1);";

  // Selected domains display
  const selectedContainer = document.createElement("div");
  selectedContainer.style.cssText =
    "display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 16px; min-height: 32px;";

  // Track selected domain IDs
  let selectedDomainIds = [...currentDomainIds];

  // Function to render selected tags
  const renderSelectedTags = () => {
    // Clear existing tags
    while (selectedContainer.firstChild) {
      selectedContainer.removeChild(selectedContainer.firstChild);
    }

    selectedDomainIds.forEach((domainId) => {
      const domain = organisationDomains.find((d) => d.id === domainId);
      if (!domain) return;

      const tag = document.createElement("span");
      tag.style.cssText =
        "display: inline-flex; align-items: center; gap: 4px; padding: 4px 8px; background: #e0e7ff; color: #3730a3; border-radius: 4px; font-size: 13px;";
      tag.textContent = domain.name;

      const removeBtn = document.createElement("button");
      removeBtn.textContent = "×";
      removeBtn.style.cssText =
        "background: none; border: none; color: #6366f1; font-size: 16px; cursor: pointer; padding: 0; margin-left: 2px;";
      removeBtn.onclick = () => {
        selectedDomainIds = selectedDomainIds.filter((id) => id !== domainId);
        renderSelectedTags();
      };

      tag.appendChild(removeBtn);
      selectedContainer.appendChild(tag);
    });
  };

  // Function to filter and render dropdown options
  const renderDropdown = (query) => {
    // Clear dropdown
    while (dropdown.firstChild) {
      dropdown.removeChild(dropdown.firstChild);
    }

    const lowerQuery = query.toLowerCase().trim();

    // Filter domains that aren't already selected
    const availableDomains = organisationDomains.filter(
      (d) => !selectedDomainIds.includes(d.id)
    );

    // Filter by search query
    const filtered = lowerQuery
      ? availableDomains.filter((d) =>
          d.name.toLowerCase().includes(lowerQuery)
        )
      : availableDomains;

    // Show options
    if (filtered.length > 0) {
      filtered.forEach((domain) => {
        const option = document.createElement("div");
        option.textContent = domain.name;
        option.style.cssText =
          "padding: 10px 16px; cursor: pointer; font-size: 14px; border-bottom: 1px solid #f3f4f6;";
        option.onmouseover = () => {
          option.style.background = "#f9fafb";
        };
        option.onmouseout = () => {
          option.style.background = "white";
        };
        option.onclick = () => {
          if (!selectedDomainIds.includes(domain.id)) {
            selectedDomainIds.push(domain.id);
            renderSelectedTags();
          }
          searchInput.value = "";
          dropdown.style.display = "none";
        };
        dropdown.appendChild(option);
      });
      dropdown.style.display = "block";
    } else if (lowerQuery) {
      // Show "Add new domain" option
      const addOption = document.createElement("div");
      addOption.textContent = `Add new domain: ${lowerQuery}`;
      addOption.style.cssText =
        "padding: 10px 16px; cursor: pointer; font-size: 14px; color: #6366f1; font-weight: 500;";
      addOption.onmouseover = () => {
        addOption.style.background = "#f9fafb";
      };
      addOption.onmouseout = () => {
        addOption.style.background = "white";
      };
      addOption.onclick = async () => {
        await createAndSelectDomainTemp(lowerQuery);
        searchInput.value = "";
        dropdown.style.display = "none";
      };
      dropdown.appendChild(addOption);
      dropdown.style.display = "block";
    } else {
      dropdown.style.display = "none";
    }
  };

  // Function to create a new domain and add it to selection
  const createAndSelectDomainTemp = async (domainName) => {
    try {
      const { data: { session } = {} } =
        await window.supabase.auth.getSession();
      const token = session?.access_token;

      if (!token) {
        showGoogleError("Please sign in to create domains");
        return;
      }

      // Create domain via job creation endpoint (reusing existing logic)
      const response = await fetch("/v1/jobs", {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          domain: domainName,
          source_type: "sitemap", // Default values just to create domain
          concurrency: 1,
          max_pages: 10,
        }),
      });

      if (!response.ok) {
        throw new Error("Failed to create domain");
      }

      const result = await response.json();
      const newDomainId = result.domain_id;

      // Add to organisationDomains array
      organisationDomains.push({ id: newDomainId, name: domainName });

      // Add to selection
      if (!selectedDomainIds.includes(newDomainId)) {
        selectedDomainIds.push(newDomainId);
        renderSelectedTags();
      }
    } catch (error) {
      console.error("Failed to create domain:", error);
      showGoogleError("Failed to create domain. Please try again.");
    }
  };

  // Search input events
  searchInput.addEventListener("input", (e) => {
    renderDropdown(e.target.value);
  });

  searchInput.addEventListener("focus", () => {
    renderDropdown(searchInput.value);
  });

  // Prevent click inside input from closing dropdown
  searchInput.addEventListener("click", (e) => {
    e.stopPropagation();
  });

  // Close dropdown when clicking outside
  document.addEventListener("click", (e) => {
    if (!inputContainer.contains(e.target)) {
      dropdown.style.display = "none";
    }
  });

  // Buttons
  const buttonContainer = document.createElement("div");
  buttonContainer.style.cssText =
    "display: flex; gap: 8px; justify-content: flex-end;";

  const cancelBtn = document.createElement("button");
  cancelBtn.textContent = "Cancel";
  cancelBtn.style.cssText =
    "padding: 8px 16px; background: #f3f4f6; color: #374151; border: 1px solid #d1d5db; border-radius: 6px; cursor: pointer;";
  cancelBtn.onclick = () => {
    document.body.removeChild(overlay);
  };

  const saveBtn = document.createElement("button");
  saveBtn.textContent = "Save";
  saveBtn.style.cssText =
    "padding: 8px 16px; background: #6366f1; color: white; border: none; border-radius: 6px; cursor: pointer;";
  saveBtn.onclick = () => {
    // Save to temporary storage
    if (!window.tempPropertyDomains) window.tempPropertyDomains = {};
    window.tempPropertyDomains[propertyId] = [...selectedDomainIds];
    updateDomainTags(propertyId);
    document.body.removeChild(overlay);
  };

  buttonContainer.appendChild(cancelBtn);
  buttonContainer.appendChild(saveBtn);

  // Assemble modal
  inputContainer.appendChild(searchInput);
  inputContainer.appendChild(dropdown);
  modal.appendChild(header);
  modal.appendChild(selectedContainer);
  modal.appendChild(inputContainer);
  modal.appendChild(buttonContainer);
  overlay.appendChild(modal);

  // Render initial state
  renderSelectedTags();

  // Show modal
  document.body.appendChild(overlay);
}

// Export functions
if (typeof window !== "undefined") {
  window.setupGoogleIntegration = setupGoogleIntegration;
  window.loadGoogleConnections = loadGoogleConnections;
  window.handleGoogleOAuthCallback = handleGoogleOAuthCallback;
}

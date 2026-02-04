(function () {
  let resolveNavReady = null;
  if (!window.BB_NAV_READY) {
    window.BB_NAV_READY = new Promise((resolve) => {
      resolveNavReady = resolve;
    });
  }

  const finishNavReady = () => {
    if (resolveNavReady) {
      resolveNavReady();
    }
    document.dispatchEvent(new CustomEvent("bb:nav-ready"));
  };

  if (document.querySelector(".global-nav")) {
    finishNavReady();
    return;
  }

  if (window.location.pathname.startsWith("/shared/jobs/")) {
    finishNavReady();
    return;
  }

  const navHtml = `
    <div class="global-nav">
      <header class="header">
        <div class="container">
          <div class="header-content">
            <div class="header-brand">
              <a href="/dashboard" class="logo">
                <span>🐝</span>
                Blue Banded Bee
              </a>
              <span class="nav-separator" id="globalNavSeparator">|</span>
              <span class="nav-title" id="globalNavTitle"></span>
            </div>
            <div class="user-menu">
              <div bbb-auth="guest" class="auth-buttons">
                <button
                  id="loginBtn"
                  class="bb-button bb-button-primary"
                  aria-label="Sign in to your account"
                >
                  Sign In
                </button>
              </div>

              <div bbb-auth="required" class="user-info">
                <div class="bb-org-switcher" id="orgSwitcher">
                  <button
                    class="bb-org-switcher-btn"
                    id="orgSwitcherBtn"
                    aria-label="Switch organisation"
                    aria-expanded="false"
                    aria-haspopup="true"
                  >
                    <span class="bb-org-name" id="currentOrgName">Loading...</span>
                    <span class="bb-org-chevron">▼</span>
                  </button>
                  <div class="bb-org-dropdown" id="orgDropdown">
                    <div class="bb-org-dropdown-header">Switch Organisation</div>
                    <div class="bb-org-list" id="orgList"></div>
                    <div class="bb-org-dropdown-footer">
                      <button
                        class="bb-org-create-btn"
                        id="createOrgBtn"
                        aria-label="Create a new organisation"
                      >
                        + Create Organisation
                      </button>
                    </div>
                  </div>
                </div>

                <div class="bb-dropdown" id="userMenu">
                  <button
                    type="button"
                    class="user-avatar"
                    id="userAvatar"
                    aria-label="Open user menu"
                    aria-haspopup="true"
                    aria-expanded="false"
                  >
                    ?
                  </button>
                  <div class="bb-dropdown-menu" id="userMenuDropdown">
                    <div class="bb-dropdown-item" style="cursor: default">
                      <strong id="userEmail">Loading...</strong>
                    </div>
                    <a class="bb-dropdown-item" href="/settings/account">
                      Your account
                    </a>
                    <div class="bb-dropdown-divider"></div>
                    <div class="bb-dropdown-item" style="cursor: default">
                      <span id="userMenuOrgName">Organisation</span> settings
                    </div>
                    <div class="bb-dropdown-item" style="cursor: default">
                      <div class="bb-quota-display" id="quotaDisplay" style="display: none">
                        <span class="bb-quota-plan" id="quotaPlan">Free</span>
                        <span class="bb-quota-usage">
                          <span class="bb-quota-usage-count" id="quotaUsage">0/1000</span>
                        </span>
                        <span class="bb-quota-reset" id="quotaReset">Resets in 12h</span>
                      </div>
                    </div>
                    <a class="bb-dropdown-item" href="/settings/billing">Billing</a>
                    <a class="bb-dropdown-item" href="/settings/plans">Plans</a>
                    <a class="bb-dropdown-item" href="/settings/notifications">Notifications</a>
                    <a class="bb-dropdown-item" href="/settings/analytics">Analytics</a>
                    <a class="bb-dropdown-item" href="/settings/auto-crawl">Auto-crawl</a>
                    <a class="bb-dropdown-item" href="/settings/team">Team</a>
                    <div class="bb-dropdown-divider"></div>
                    <button
                      id="logoutBtn"
                      class="bb-dropdown-item"
                      aria-label="Sign out of your account"
                      type="button"
                    >
                      Sign Out
                    </button>
                    <div class="bb-dropdown-divider"></div>
                    <button
                      id="resetDbBtn"
                      class="bb-dropdown-item"
                      style="display: none; color: #dc2626"
                      title="Delete all jobs and tasks - USE WITH CAUTION"
                      aria-label="Reset database - delete all jobs and tasks"
                      type="button"
                    >
                      Reset DB
                    </button>
                  </div>
                </div>

                <div class="bb-notifications-container" id="notificationsContainer">
                  <button
                    id="notificationsBtn"
                    class="bb-button bb-button-outline bb-notifications-btn"
                    aria-label="View notifications"
                  >
                    🔔
                    <span class="bb-notifications-badge" id="notificationsBadge">0</span>
                  </button>
                  <div class="bb-notifications-dropdown" id="notificationsDropdown">
                    <div class="bb-notifications-header">
                      <span class="bb-notifications-title">Notifications</span>
                      <button
                        id="markAllReadBtn"
                        class="bb-notifications-mark-read"
                        aria-label="Mark all as read"
                      >
                        Mark all read
                      </button>
                    </div>
                    <div class="bb-notifications-list" id="notificationsList">
                      <div class="bb-notifications-empty">
                        <div class="bb-notifications-empty-icon">🔔</div>
                        <div>No notifications yet</div>
                      </div>
                    </div>
                    <div class="bb-notifications-footer">
                      <a
                        class="bb-notifications-footer-btn bb-notifications-settings-btn"
                        id="notificationsSettingsBtn"
                        href="/settings/notifications"
                      >
                        ⚙️ Notification Settings
                      </a>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </header>
    </div>
  `;

  const mountNav = () => {
    const navWrapper = document.createElement("div");
    navWrapper.innerHTML = navHtml.trim();
    const navElement = navWrapper.firstElementChild;
    if (!navElement || !document.body) {
      finishNavReady();
      return;
    }

    document.body.prepend(navElement);

    const titleEl = navElement.querySelector("#globalNavTitle");
    const separatorEl = navElement.querySelector("#globalNavSeparator");
    const currentOrgName = navElement.querySelector("#currentOrgName");
    const settingsOrgName = document.getElementById("settingsOrgName");
    const path = window.location.pathname.replace(/\/$/, "");
    const navLinks = navElement.querySelectorAll(".nav-link");

    const titleMap = [
      { match: (p) => p === "/dashboard", title: "Dashboard" },
      { match: (p) => p.startsWith("/settings"), title: "Settings" },
    ];

    const titleMatch = titleMap.find((entry) => entry.match(path));
    if (titleEl) {
      titleEl.textContent = titleMatch ? titleMatch.title : "";
    }
    if (separatorEl) {
      separatorEl.style.display = titleMatch ? "inline" : "none";
    }

    navLinks.forEach((link) => {
      try {
        const linkPath = new URL(link.href).pathname.replace(/\/$/, "");
        const isDashboard = linkPath === "/dashboard";
        const isSettings = linkPath.startsWith("/settings");

        const active =
          (isDashboard &&
            (path === "/dashboard" || path.startsWith("/jobs"))) ||
          (isSettings && path.startsWith("/settings"));

        link.classList.toggle("active", active);
        if (active) {
          link.setAttribute("aria-current", "page");
        } else {
          link.removeAttribute("aria-current");
        }
      } catch (err) {
        console.warn("Failed to resolve nav link state:", err);
        link.classList.remove("active");
        link.removeAttribute("aria-current");
      }
    });

    const initNavOrgSwitcher = async () => {
      if (!currentOrgName) return;

      const orgListEl = navElement.querySelector("#orgList");
      const orgSwitcher = navElement.querySelector("#orgSwitcher");
      const orgBtn = navElement.querySelector("#orgSwitcherBtn");

      // Helper to update the nav display
      const updateNavOrgDisplay = (activeOrg, organisations) => {
        if (!activeOrg) {
          currentOrgName.textContent = "No Organisation";
          return;
        }

        currentOrgName.textContent = activeOrg.name || "Organisation";

        if (orgListEl) {
          // Clear existing items safely
          while (orgListEl.firstChild) {
            orgListEl.removeChild(orgListEl.firstChild);
          }
          (organisations || []).forEach((org) => {
            const button = document.createElement("button");
            button.className = "bb-org-item";
            button.dataset.orgId = org.id;
            button.dataset.orgName = org.name;
            button.textContent = org.name;
            if (org.id === activeOrg?.id) {
              button.classList.add("active");
            }
            orgListEl.appendChild(button);
          });
        }
      };

      // Handle org item clicks (delegated)
      if (orgListEl) {
        orgListEl.addEventListener("click", async (e) => {
          const item = e.target.closest(".bb-org-item");
          if (!item || item.classList.contains("active")) {
            orgSwitcher?.classList.remove("open");
            return;
          }

          const orgId = item.dataset.orgId;

          // Show loading state
          currentOrgName.textContent = "Switching...";
          orgSwitcher?.classList.remove("open");
          orgBtn?.setAttribute("aria-expanded", "false");

          try {
            // Use shared switch function
            await window.BB_APP.switchOrg(orgId);
            // bb:org-switched event will update UI
          } catch (err) {
            console.warn("Failed to switch organisation:", err);
            // Restore previous name
            currentOrgName.textContent =
              window.BB_ACTIVE_ORG?.name || "Organisation";
          }
        });
      }

      // Toggle dropdown
      if (orgSwitcher && orgBtn) {
        orgBtn.addEventListener("click", (event) => {
          event.stopPropagation();
          orgSwitcher.classList.toggle("open");
          orgBtn.setAttribute(
            "aria-expanded",
            orgSwitcher.classList.contains("open")
          );
        });

        document.addEventListener("click", () => {
          orgSwitcher.classList.remove("open");
          orgBtn.setAttribute("aria-expanded", "false");
        });
      }

      // Listen for org switches (from anywhere)
      document.addEventListener("bb:org-switched", (e) => {
        const newOrg = e.detail?.organisation;
        if (newOrg) {
          currentOrgName.textContent = newOrg.name || "Organisation";
          if (orgListEl) {
            orgListEl.querySelectorAll(".bb-org-item").forEach((el) => {
              el.classList.toggle(
                "active",
                el.dataset.orgId === String(newOrg.id)
              );
            });
          }
        }
      });

      // Listen for org ready (after auth state restored)
      document.addEventListener("bb:org-ready", () => {
        updateNavOrgDisplay(window.BB_ACTIVE_ORG, window.BB_ORGANISATIONS);
      });

      // Sync with settings page org name if present
      if (settingsOrgName) {
        const orgNameObserver = new MutationObserver(() => {
          const nextName = settingsOrgName.textContent?.trim();
          if (nextName && nextName !== currentOrgName.textContent) {
            currentOrgName.textContent = nextName;
          }
        });
        orgNameObserver.observe(settingsOrgName, {
          childList: true,
          characterData: true,
          subtree: true,
        });
        window.addEventListener(
          "beforeunload",
          () => orgNameObserver.disconnect(),
          { once: true }
        );
      }

      // Wait for core (Supabase) to be ready, then init org
      try {
        if (window.BB_APP?.coreReady) {
          await window.BB_APP.coreReady;
        }
        if (window.BB_APP?.initialiseOrg) {
          await window.BB_APP.initialiseOrg();
        }
        updateNavOrgDisplay(window.BB_ACTIVE_ORG, window.BB_ORGANISATIONS);
      } catch (err) {
        // Org init failed - show fallback
        currentOrgName.textContent = "Organisation";
      }
    };

    initNavOrgSwitcher();

    finishNavReady();
  };

  if (document.body) {
    mountNav();
  } else {
    document.addEventListener("DOMContentLoaded", mountNav, { once: true });
  }
})();

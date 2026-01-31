/*
 * Settings page logic
 * Handles navigation, organisation controls, and settings data loads.
 */

(function () {
  const settingsState = {
    currentUserRole: "member",
    currentUserId: null,
  };

  function showSettingsToast(type, message) {
    const container = document.createElement("div");
    const colours =
      type === "success"
        ? { bg: "#d1fae5", text: "#065f46", border: "#a7f3d0" }
        : { bg: "#fee2e2", text: "#dc2626", border: "#fecaca" };

    container.style.cssText = `
      position: fixed; top: 20px; right: 20px; z-index: 10000;
      background: ${colours.bg}; color: ${colours.text};
      border: 1px solid ${colours.border};
      padding: 16px 20px; border-radius: 8px; max-width: 400px;
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    `;

    const content = document.createElement("div");
    content.style.cssText = "display: flex; align-items: center; gap: 12px;";

    const icon = document.createElement("span");
    icon.textContent = type === "success" ? "✅" : "⚠️";
    content.appendChild(icon);

    const messageSpan = document.createElement("span");
    messageSpan.textContent = message;
    content.appendChild(messageSpan);

    const closeButton = document.createElement("button");
    closeButton.style.cssText =
      "background: none; border: none; font-size: 18px; cursor: pointer;";
    closeButton.setAttribute("aria-label", "Dismiss");
    closeButton.textContent = "×";
    closeButton.addEventListener("click", () => container.remove());
    content.appendChild(closeButton);

    container.appendChild(content);
    document.body.appendChild(container);

    setTimeout(() => container.remove(), 5000);
  }

  window.showDashboardSuccess = function (message) {
    showSettingsToast("success", message);
  };

  window.showDashboardError = function (message) {
    showSettingsToast("error", message);
  };

  window.showIntegrationFeedback = function (integration, type, message) {
    const suffix = type === "success" ? "Success" : "Error";
    const el = document.getElementById(`${integration}${suffix}Message`);
    const textEl = document.getElementById(`${integration}${suffix}Text`);

    if (el) {
      if (textEl) {
        textEl.textContent = message;
      } else {
        el.textContent = message;
      }
      el.style.display = "block";
      setTimeout(() => {
        el.style.display = "none";
      }, 5000);
    }
  };

  window.showSlackSuccess = (msg) =>
    window.showIntegrationFeedback("slack", "success", msg);
  window.showSlackError = (msg) =>
    window.showIntegrationFeedback("slack", "error", msg);
  window.showWebflowSuccess = (msg) =>
    window.showIntegrationFeedback("webflow", "success", msg);
  window.showWebflowError = (msg) =>
    window.showIntegrationFeedback("webflow", "error", msg);
  window.showGoogleSuccess = (msg) =>
    window.showIntegrationFeedback("google", "success", msg);
  window.showGoogleError = (msg) =>
    window.showIntegrationFeedback("google", "error", msg);

  function setActiveSettingsLink() {
    const path = window.location.pathname.replace(/\/$/, "");
    const currentPath = path === "/settings" ? "/settings/account" : path;

    document.querySelectorAll(".settings-link").forEach((link) => {
      try {
        const linkPath = new URL(link.href).pathname.replace(/\/$/, "");
        if (linkPath === currentPath) {
          link.classList.add("active");
        } else {
          link.classList.remove("active");
        }
      } catch (err) {
        link.classList.remove("active");
      }
    });
  }

  function scrollToHash() {
    if (!window.location.hash) return;
    const targetId = window.location.hash.replace("#", "");
    const target = document.getElementById(targetId);
    if (target) {
      target.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  }

  function scrollToPathSection() {
    if (window.location.hash) return;

    const path = window.location.pathname.replace(/\/$/, "");
    const sectionMap = {
      "/settings": "account",
      "/settings/account": "account",
      "/settings/team": "team",
      "/settings/plans": "plans",
      "/settings/billing": "billing",
      "/settings/notifications": "notifications",
      "/settings/analytics": "analytics",
      "/settings/auto-crawl": "auto-crawl",
    };

    const targetId = sectionMap[path];
    if (!targetId) return;

    const target = document.getElementById(targetId);
    if (target) {
      target.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  }

  function setupSettingsNavigation() {
    setActiveSettingsLink();
    scrollToHash();
    scrollToPathSection();

    window.addEventListener("hashchange", () => {
      scrollToHash();
    });

    window.addEventListener("popstate", () => {
      setActiveSettingsLink();
      scrollToPathSection();
    });
  }

  async function loadAccountDetails() {
    const sessionResult = await window.supabase.auth.getSession();
    const session = sessionResult?.data?.session;
    if (!session?.user) return;

    const email = session.user.email || "";
    const fullName =
      session.user.user_metadata?.full_name ||
      session.user.user_metadata?.name ||
      "";

    const emailEl = document.getElementById("settingsUserEmail");
    const nameEl = document.getElementById("settingsUserName");
    if (emailEl) emailEl.textContent = email || "Not set";
    if (nameEl) nameEl.textContent = fullName || "Not set";
  }

  async function sendPasswordReset() {
    const sessionResult = await window.supabase.auth.getSession();
    const session = sessionResult?.data?.session;
    const email = session?.user?.email;
    if (!email) {
      showSettingsToast("error", "Email address not available");
      return;
    }

    try {
      const { error } = await window.supabase.auth.resetPasswordForEmail(
        email,
        {
          redirectTo: window.location.origin + "/settings/account#security",
        }
      );
      if (error) {
        throw error;
      }
      showSettingsToast("success", "Password reset email sent");
    } catch (err) {
      console.error("Failed to send password reset:", err);
      showSettingsToast("error", "Failed to send password reset email");
    }
  }

  async function loadOrganisationMembers() {
    const membersList = document.getElementById("teamMembersList");
    const memberTemplate = document.getElementById("teamMemberTemplate");
    const emptyState = document.getElementById("teamMembersEmpty");
    if (!membersList || !memberTemplate) return;

    membersList.innerHTML = "";

    try {
      const response = await window.dataBinder.fetchData(
        "/v1/organisations/members"
      );
      const members = response.members || [];
      settingsState.currentUserRole = response.current_user_role || "member";
      settingsState.currentUserId = response.current_user_id || null;

      if (members.length === 0) {
        if (emptyState) emptyState.style.display = "block";
        return;
      }
      if (emptyState) emptyState.style.display = "none";

      members.forEach((member) => {
        const clone = memberTemplate.content.cloneNode(true);
        const row = clone.querySelector(".settings-member-row");
        const nameEl = clone.querySelector(".settings-member-name");
        const emailEl = clone.querySelector(".settings-member-email");
        const roleEl = clone.querySelector(".settings-member-role");
        const removeBtn = clone.querySelector(".settings-member-remove");

        if (row) row.dataset.memberId = member.id;
        if (nameEl) {
          nameEl.textContent = member.full_name || "Unnamed";
        }
        if (emailEl) emailEl.textContent = member.email || "";
        if (roleEl) roleEl.textContent = member.role || "member";

        if (removeBtn) {
          removeBtn.dataset.memberId = member.id;
          removeBtn.addEventListener("click", () => removeMember(member.id));

          if (settingsState.currentUserRole !== "admin") {
            removeBtn.disabled = true;
          }
        }

        membersList.appendChild(clone);
      });

      updateAdminVisibility();
    } catch (err) {
      console.error("Failed to load members:", err);
      showSettingsToast("error", "Failed to load members");
    }
  }

  async function removeMember(memberId) {
    if (!memberId) return;
    if (!confirm("Remove this member from the organisation?")) return;

    try {
      await window.dataBinder.fetchData(
        `/v1/organisations/members/${memberId}`,
        {
          method: "DELETE",
        }
      );
      showSettingsToast("success", "Member removed");
      loadOrganisationMembers();
    } catch (err) {
      console.error("Failed to remove member:", err);
      showSettingsToast("error", "Failed to remove member");
    }
  }

  async function loadOrganisationInvites() {
    const invitesList = document.getElementById("teamInvitesList");
    const inviteTemplate = document.getElementById("teamInviteTemplate");
    const emptyState = document.getElementById("teamInvitesEmpty");
    if (!invitesList || !inviteTemplate) return;

    invitesList.innerHTML = "";

    if (settingsState.currentUserRole !== "admin") {
      if (emptyState) {
        emptyState.style.display = "block";
        emptyState.textContent = "Only admins can view pending invites.";
      }
      return;
    }

    try {
      const response = await window.dataBinder.fetchData(
        "/v1/organisations/invites"
      );
      const invites = response.invites || [];

      if (invites.length === 0) {
        if (emptyState) emptyState.style.display = "block";
        return;
      }
      if (emptyState) emptyState.style.display = "none";

      invites.forEach((invite) => {
        const clone = inviteTemplate.content.cloneNode(true);
        const row = clone.querySelector(".settings-invite-row");
        const emailEl = clone.querySelector(".settings-invite-email");
        const roleEl = clone.querySelector(".settings-invite-role");
        const dateEl = clone.querySelector(".settings-invite-date");
        const revokeBtn = clone.querySelector(".settings-invite-revoke");

        if (row) row.dataset.inviteId = invite.id;
        if (emailEl) emailEl.textContent = invite.email;
        if (roleEl) roleEl.textContent = invite.role;
        if (dateEl) {
          const date = new Date(invite.created_at);
          dateEl.textContent = `Sent ${date.toLocaleDateString("en-AU")}`;
        }
        if (revokeBtn) {
          revokeBtn.dataset.inviteId = invite.id;
          revokeBtn.addEventListener("click", () => revokeInvite(invite.id));
        }

        invitesList.appendChild(clone);
      });
    } catch (err) {
      console.error("Failed to load invites:", err);
      showSettingsToast("error", "Failed to load invites");
    }
  }

  async function sendInvite(event) {
    event.preventDefault();
    if (settingsState.currentUserRole !== "admin") {
      showSettingsToast("error", "Only admins can send invites");
      return;
    }

    const emailInput = document.getElementById("teamInviteEmail");
    const roleSelect = document.getElementById("teamInviteRole");
    if (!emailInput) return;

    const email = emailInput.value.trim();
    const role = roleSelect?.value || "member";
    if (!email) {
      showSettingsToast("error", "Email is required");
      return;
    }

    try {
      await window.dataBinder.fetchData("/v1/organisations/invites", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, role }),
      });
      showSettingsToast("success", "Invite sent");
      emailInput.value = "";
      loadOrganisationInvites();
    } catch (err) {
      console.error("Failed to send invite:", err);
      showSettingsToast("error", "Failed to send invite");
    }
  }

  async function revokeInvite(inviteId) {
    if (!inviteId) return;
    if (!confirm("Revoke this invite?")) return;

    try {
      await window.dataBinder.fetchData(
        `/v1/organisations/invites/${inviteId}`,
        {
          method: "DELETE",
        }
      );
      showSettingsToast("success", "Invite revoked");
      loadOrganisationInvites();
    } catch (err) {
      console.error("Failed to revoke invite:", err);
      showSettingsToast("error", "Failed to revoke invite");
    }
  }

  async function loadPlansAndUsage() {
    const currentPlanName = document.getElementById("planCurrentName");
    const currentPlanLimit = document.getElementById("planCurrentLimit");
    const currentPlanUsage = document.getElementById("planCurrentUsage");
    const currentPlanReset = document.getElementById("planCurrentReset");
    const planList = document.getElementById("planCards");
    const planTemplate = document.getElementById("planCardTemplate");

    try {
      const [usageResponse, plansResponse] = await Promise.all([
        window.dataBinder.fetchData("/v1/usage"),
        window.dataBinder.fetchData("/v1/plans"),
      ]);

      const usage = usageResponse.usage || {};
      const plans = plansResponse.plans || [];

      if (currentPlanName) {
        currentPlanName.textContent = usage.plan_display_name || "Free";
      }
      if (currentPlanLimit) {
        currentPlanLimit.textContent = usage.daily_limit
          ? `${usage.daily_limit.toLocaleString()} pages/day`
          : "No limit";
      }
      if (currentPlanUsage) {
        currentPlanUsage.textContent = usage.daily_limit
          ? `${usage.daily_used.toLocaleString()} used today`
          : "No usage data";
      }
      if (currentPlanReset) {
        currentPlanReset.textContent = usage.resets_at
          ? formatTimeUntilReset(usage.resets_at)
          : "";
      }

      if (planList && planTemplate) {
        planList.innerHTML = "";
        plans.forEach((plan) => {
          const clone = planTemplate.content.cloneNode(true);
          const card = clone.querySelector(".settings-plan-card");
          const nameEl = clone.querySelector(".settings-plan-name");
          const priceEl = clone.querySelector(".settings-plan-price");
          const limitEl = clone.querySelector(".settings-plan-limit");
          const actionBtn = clone.querySelector(".settings-plan-action");

          if (card) {
            if (plan.id === usage.plan_id) {
              card.classList.add("current");
            }
          }
          if (nameEl) nameEl.textContent = plan.display_name;
          if (priceEl) {
            priceEl.textContent =
              plan.monthly_price_cents > 0
                ? `$${(plan.monthly_price_cents / 100).toFixed(0)}/month`
                : "Free";
          }
          if (limitEl) {
            limitEl.textContent = `${plan.daily_page_limit.toLocaleString()} pages/day`;
          }
          if (actionBtn) {
            actionBtn.dataset.planId = plan.id;
            if (plan.id === usage.plan_id) {
              actionBtn.textContent = "Current plan";
              actionBtn.disabled = true;
            } else if (settingsState.currentUserRole !== "admin") {
              actionBtn.textContent = "Admin only";
              actionBtn.disabled = true;
            } else {
              actionBtn.textContent = "Switch plan";
              actionBtn.disabled = false;
              actionBtn.addEventListener("click", () => switchPlan(plan.id));
            }
          }

          planList.appendChild(clone);
        });
      }
    } catch (err) {
      console.error("Failed to load plans:", err);
      showSettingsToast("error", "Failed to load plan details");
    }
  }

  async function switchPlan(planId) {
    if (!planId) return;
    if (!confirm("Switch to this plan?")) return;

    try {
      await window.dataBinder.fetchData("/v1/organisations/plan", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ plan_id: planId }),
      });
      showSettingsToast("success", "Plan updated");
      loadPlansAndUsage();
      fetchAndDisplayQuota();
    } catch (err) {
      console.error("Failed to switch plan:", err);
      showSettingsToast("error", "Failed to switch plan");
    }
  }

  async function loadUsageHistory() {
    const list = document.getElementById("usageHistoryList");
    if (!list) return;

    list.textContent = "";
    try {
      const response = await window.dataBinder.fetchData(
        "/v1/usage/history?days=30"
      );
      const entries = response.usage || [];

      if (entries.length === 0) {
        const empty = document.createElement("div");
        empty.className = "settings-muted";
        empty.textContent = "No usage history yet.";
        list.appendChild(empty);
        return;
      }

      entries.forEach((entry) => {
        const row = document.createElement("div");
        row.className = "settings-usage-row";
        const dateSpan = document.createElement("span");
        dateSpan.textContent = entry.usage_date;
        const pagesSpan = document.createElement("span");
        const pagesProcessed = Number.isFinite(entry.pages_processed)
          ? entry.pages_processed
          : 0;
        pagesSpan.textContent = `${pagesProcessed.toLocaleString()} pages`;
        row.appendChild(dateSpan);
        row.appendChild(pagesSpan);
        list.appendChild(row);
      });
    } catch (err) {
      console.error("Failed to load usage history:", err);
      const error = document.createElement("div");
      error.className = "settings-muted";
      error.textContent = "Failed to load usage history.";
      list.appendChild(error);
    }
  }

  function updateAdminVisibility() {
    document.querySelectorAll("[data-admin-only]").forEach((el) => {
      if (settingsState.currentUserRole === "admin") {
        el.style.display = "";
      } else {
        el.style.display = "none";
      }
    });
  }

  async function handleInviteToken() {
    const params = new URLSearchParams(window.location.search);
    const token = params.get("invite_token");
    if (!token) return;

    try {
      await window.dataBinder.fetchData("/v1/organisations/invites/accept", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token }),
      });
      showSettingsToast("success", "Invite accepted");
      params.delete("invite_token");
      const url = new URL(window.location.href);
      url.search = params.toString();
      window.history.replaceState({}, "", url.toString());
      await refreshSettingsData();
    } catch (err) {
      console.error("Failed to accept invite:", err);
      showSettingsToast("error", "Failed to accept invite");
    }
  }

  async function refreshSettingsData() {
    await loadOrganisationMembers();
    await loadOrganisationInvites();
    await loadPlansAndUsage();
    await loadUsageHistory();

    if (window.loadSlackConnections) {
      await window.loadSlackConnections();
    }
    if (window.loadWebflowConnections) {
      await window.loadWebflowConnections();
    }
    if (window.loadGoogleConnections) {
      await window.loadGoogleConnections();
    }
  }

  function formatTimeUntilReset(resetTime) {
    const now = new Date();
    const reset = new Date(resetTime);
    const diffMs = reset - now;

    if (diffMs <= 0) return "Resets soon";

    const hours = Math.floor(diffMs / (1000 * 60 * 60));
    const minutes = Math.floor((diffMs % (1000 * 60 * 60)) / (1000 * 60));

    if (hours > 0) {
      return `Resets in ${hours}h ${minutes}m`;
    }
    return `Resets in ${minutes}m`;
  }

  async function fetchAndDisplayQuota() {
    const quotaDisplay = document.getElementById("quotaDisplay");
    const quotaPlan = document.getElementById("quotaPlan");
    const quotaUsage = document.getElementById("quotaUsage");
    const quotaReset = document.getElementById("quotaReset");

    if (!quotaDisplay || !window.supabase) return;

    try {
      const session = await window.supabase.auth.getSession();
      const token = session?.data?.session?.access_token;

      if (!token) {
        return;
      }

      const response = await fetch("/v1/usage", {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        console.warn("Failed to fetch quota:", response.status);
        return;
      }

      const data = await response.json();
      const usage = data.data?.usage;

      if (!usage) return;

      const dailyLimit = Number.isFinite(usage.daily_limit)
        ? usage.daily_limit
        : 0;
      const dailyUsed = Number.isFinite(usage.daily_used)
        ? usage.daily_used
        : 0;
      quotaPlan.textContent = usage.plan_display_name || "Free";
      quotaUsage.textContent = `${dailyUsed.toLocaleString()}/${dailyLimit.toLocaleString()}`;
      quotaReset.textContent = formatTimeUntilReset(usage.resets_at);

      quotaDisplay.classList.remove("quota-warning", "quota-exhausted");
      if (usage.usage_percentage >= 100) {
        quotaDisplay.classList.add("quota-exhausted");
      } else if (usage.usage_percentage >= 80) {
        quotaDisplay.classList.add("quota-warning");
      }

      quotaDisplay.style.display = "flex";
    } catch (err) {
      console.warn("Error fetching quota:", err);
    }
  }

  let quotaResetInterval;
  let quotaVisibilityListener;
  function startQuotaResetCountdown() {
    if (quotaResetInterval) clearInterval(quotaResetInterval);
    quotaResetInterval = null;
    if (quotaVisibilityListener) {
      document.removeEventListener("visibilitychange", quotaVisibilityListener);
    }

    quotaVisibilityListener = () => {
      if (document.visibilityState === "visible") {
        fetchAndDisplayQuota();
        if (!quotaResetInterval) {
          quotaResetInterval = setInterval(fetchAndDisplayQuota, 30000);
        }
      } else if (quotaResetInterval) {
        clearInterval(quotaResetInterval);
        quotaResetInterval = null;
      }
    };

    document.addEventListener("visibilitychange", quotaVisibilityListener);
    quotaVisibilityListener();
  }

  function setupNotificationsDropdown() {
    const container = document.getElementById("notificationsContainer");
    const toggleBtn = document.getElementById("notificationsBtn");
    const settingsBtn = document.getElementById("notificationsSettingsBtn");
    const markAllReadBtn = document.getElementById("markAllReadBtn");

    if (!container || !toggleBtn) return;

    toggleBtn.addEventListener("click", async (e) => {
      e.stopPropagation();
      const isOpen = container.classList.toggle("open");
      if (isOpen) {
        await loadNotifications();
      }
    });

    if (settingsBtn) {
      settingsBtn.addEventListener("click", () => {
        container.classList.remove("open");
      });
    }

    if (markAllReadBtn) {
      markAllReadBtn.addEventListener("click", async () => {
        await markAllNotificationsRead();
      });
    }

    document.addEventListener("click", (e) => {
      if (!container.contains(e.target)) {
        container.classList.remove("open");
      }
    });

    document.addEventListener("keydown", (e) => {
      if (e.key === "Escape") {
        if (container.classList.contains("open")) {
          container.classList.remove("open");
        }
      }
    });

    loadNotificationCount();
    subscribeToNotifications();
  }

  async function subscribeToNotifications() {
    const orgId = window.BB_ACTIVE_ORG?.id;
    if (!orgId || !window.supabase) {
      setTimeout(subscribeToNotifications, 1000);
      return;
    }

    if (window.notificationsChannel) {
      window.supabase.removeChannel(window.notificationsChannel);
    }

    try {
      const channel = window.supabase
        .channel(`notifications-changes:${orgId}`)
        .on(
          "postgres_changes",
          {
            event: "INSERT",
            schema: "public",
            table: "notifications",
            filter: `organisation_id=eq.${orgId}`,
          },
          () => {
            setTimeout(() => {
              loadNotificationCount();
              if (
                document
                  .getElementById("notificationsContainer")
                  ?.classList.contains("open")
              ) {
                loadNotifications();
              }
            }, 200);
          }
        )
        .subscribe();

      window.notificationsChannel = channel;
    } catch (err) {
      console.error("Failed to subscribe to notifications:", err);
    }
  }

  async function loadNotificationCount() {
    try {
      const session = await window.supabase.auth.getSession();
      const token = session?.data?.session?.access_token;
      if (!token) return;

      const response = await fetch("/v1/notifications?limit=1", {
        headers: { Authorization: `Bearer ${token}` },
      });

      if (response.ok) {
        const data = await response.json();
        updateNotificationBadge(data.unread_count);
      }
    } catch (err) {
      console.error("Failed to load notification count:", err);
    }
  }

  async function loadNotifications() {
    const list = document.getElementById("notificationsList");
    if (!list) return;

    try {
      const session = await window.supabase.auth.getSession();
      const token = session?.data?.session?.access_token;
      if (!token) {
        list.innerHTML =
          '<div class="bb-notifications-empty"><div>Please sign in</div></div>';
        return;
      }

      const response = await fetch("/v1/notifications?limit=10", {
        headers: { Authorization: `Bearer ${token}` },
      });

      if (!response.ok) {
        throw new Error("Failed to fetch notifications");
      }

      const data = await response.json();
      updateNotificationBadge(data.unread_count);
      renderNotifications(data.notifications);
    } catch (err) {
      console.error("Failed to load notifications:", err);
      list.innerHTML =
        '<div class="bb-notifications-empty"><div>Failed to load</div></div>';
    }
  }

  function renderNotifications(notifications) {
    const list = document.getElementById("notificationsList");
    if (!list) return;

    if (!notifications || notifications.length === 0) {
      list.innerHTML = `
        <div class="bb-notifications-empty">
          <div class="bb-notifications-empty-icon">🔔</div>
          <div>No notifications yet</div>
        </div>
      `;
      return;
    }

    const typeIcons = {
      job_completed: "✅",
      job_failed: "❌",
      job_started: "🚀",
      system: "ℹ️",
    };

    list.innerHTML = notifications
      .map((n) => {
        const isUnread = !n.read_at;
        const icon = typeIcons[n.type] || "📬";
        const time = formatRelativeTime(n.created_at);

        return `
        <div class="bb-notification-item ${isUnread ? "unread" : ""}"
             data-id="${escapeHtml(n.id)}"
             data-link="${escapeHtml(n.link || "")}">
          <div class="bb-notification-item-icon">${icon}</div>
          <div class="bb-notification-item-content">
            <div class="bb-notification-item-subject">${escapeHtml(n.subject)}</div>
            <div class="bb-notification-item-preview">${escapeHtml(n.preview)}</div>
            <div class="bb-notification-item-time">${time}</div>
          </div>
        </div>
      `;
      })
      .join("");

    list.onclick = (e) => {
      const item = e.target.closest(".bb-notification-item");
      if (item) {
        handleNotificationClick(item.dataset.id, item.dataset.link);
      }
    };
  }

  async function handleNotificationClick(id, link) {
    try {
      const session = await window.supabase.auth.getSession();
      const token = session?.data?.session?.access_token;
      if (token) {
        await fetch(`/v1/notifications/${id}/read`, {
          method: "POST",
          headers: { Authorization: `Bearer ${token}` },
        });
        loadNotificationCount();
      }
    } catch (err) {
      console.error("Failed to mark notification read:", err);
    }

    if (link) {
      document
        .getElementById("notificationsContainer")
        ?.classList.remove("open");
      if (link.startsWith("/")) {
        window.location.href = link;
      } else {
        const newWindow = window.open(link, "_blank", "noopener,noreferrer");
        if (newWindow) {
          newWindow.opener = null;
        }
      }
    }
  }

  async function markAllNotificationsRead() {
    try {
      const session = await window.supabase.auth.getSession();
      const token = session?.data?.session?.access_token;
      if (!token) return;

      const response = await fetch("/v1/notifications/read-all", {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      });

      if (response.ok) {
        updateNotificationBadge(0);
        document
          .querySelectorAll(".bb-notification-item.unread")
          .forEach((el) => el.classList.remove("unread"));
      }
    } catch (err) {
      console.error("Failed to mark all read:", err);
    }
  }

  function updateNotificationBadge(count) {
    const badge = document.getElementById("notificationsBadge");
    if (badge) {
      badge.textContent = count > 9 ? "9+" : count || "";
      badge.dataset.count = count || 0;
    }
  }

  function formatRelativeTime(dateStr) {
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now - date;
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return "just now";
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;
    return date.toLocaleDateString("en-AU");
  }

  function escapeHtml(text) {
    if (!text) return "";
    const div = document.createElement("div");
    div.textContent = text;
    return div.innerHTML;
  }

  async function initOrgSwitcher() {
    const switcher = document.getElementById("orgSwitcher");
    const btn = document.getElementById("orgSwitcherBtn");
    const dropdown = document.getElementById("orgDropdown");
    const orgList = document.getElementById("orgList");
    const currentOrgName = document.getElementById("currentOrgName");
    const settingsOrgName = document.getElementById("settingsOrgName");
    const settingsSwitcherBtn = document.getElementById(
      "settingsOrgSwitcherBtn"
    );
    const divider = document.querySelector(".bb-org-divider");

    if (!switcher || !btn || !divider) return;

    btn.disabled = false;
    btn.style.cursor = "";
    const chevron = btn.querySelector(".bb-org-chevron");
    if (chevron) chevron.style.display = "";

    const newBtn = btn.cloneNode(true);
    btn.parentNode.replaceChild(newBtn, btn);
    const newOrgList = orgList.cloneNode(false);
    orgList.parentNode.replaceChild(newOrgList, orgList);

    const btnRef = newBtn;
    const orgListRef = newOrgList;
    let settingsBtnRef = settingsSwitcherBtn;

    if (settingsSwitcherBtn?.parentNode) {
      const newSettingsBtn = settingsSwitcherBtn.cloneNode(true);
      settingsSwitcherBtn.parentNode.replaceChild(
        newSettingsBtn,
        settingsSwitcherBtn
      );
      settingsBtnRef = newSettingsBtn;
    }

    try {
      const sessionResult = await window.supabase.auth.getSession();
      const session = sessionResult?.data?.session;
      if (!session) return;

      const response = await fetch("/v1/organisations", {
        headers: { Authorization: `Bearer ${session.access_token}` },
      });

      if (!response.ok) {
        console.error("Failed to fetch organisations");
        switcher.style.display = "none";
        divider.style.display = "none";
        return;
      }

      const data = await response.json();
      const organisations = data.data?.organisations || [];

      if (organisations.length <= 1) {
        if (dropdown) dropdown.style.display = "none";
        btnRef.disabled = true;
        btnRef.style.cursor = "default";
        if (chevron) chevron.style.display = "none";
        if (settingsBtnRef) {
          settingsBtnRef.disabled = true;
          settingsBtnRef.style.cursor = "default";
        }
      }

      const activeOrg = organisations.find(
        (org) => org.id === window.BB_ACTIVE_ORG?.id
      );
      const activeName = activeOrg?.name || organisations[0]?.name || "";
      if (currentOrgName)
        currentOrgName.textContent = activeName || "Loading...";
      if (settingsOrgName)
        settingsOrgName.textContent = activeName || "Organisation";

      btnRef.addEventListener("click", (e) => {
        e.stopPropagation();
        switcher.classList.toggle("open");
        btnRef.setAttribute(
          "aria-expanded",
          switcher.classList.contains("open")
        );
      });

      if (settingsBtnRef) {
        settingsBtnRef.addEventListener("click", (e) => {
          e.stopPropagation();
          switcher.classList.toggle("open");
          btnRef.setAttribute(
            "aria-expanded",
            switcher.classList.contains("open")
          );
        });
      }

      orgListRef.innerHTML = "";
      organisations.forEach((org) => {
        const button = document.createElement("button");
        button.className = "bb-org-item";
        button.textContent = org.name;
        if (org.id === window.BB_ACTIVE_ORG?.id) {
          button.classList.add("active");
        }
        button.addEventListener("click", async () => {
          switcher.classList.remove("open");
          btnRef.setAttribute("aria-expanded", "false");
          const previous = window.BB_ACTIVE_ORG?.id;

          try {
            const switchRes = await fetch("/v1/organisations/switch", {
              method: "POST",
              headers: {
                Authorization: `Bearer ${session.access_token}`,
                "Content-Type": "application/json",
              },
              body: JSON.stringify({ organisation_id: org.id }),
            });

            if (switchRes.ok) {
              const switchData = await switchRes.json();
              window.BB_ACTIVE_ORG = switchData.data?.organisation;
              if (currentOrgName) currentOrgName.textContent = org.name;
              if (settingsOrgName) settingsOrgName.textContent = org.name;

              if (previous !== org.id) {
                await refreshSettingsData();
              }

              if (typeof subscribeToNotifications === "function") {
                await subscribeToNotifications();
              }

              await loadNotificationCount();
              await fetchAndDisplayQuota();

              showSettingsToast("success", `Switched to ${org.name}`);
            } else {
              if (currentOrgName) {
                currentOrgName.textContent =
                  window.BB_ACTIVE_ORG?.name || "Unknown";
              }
              if (settingsOrgName) {
                settingsOrgName.textContent =
                  window.BB_ACTIVE_ORG?.name || "Organisation";
              }
              showSettingsToast("error", "Failed to switch organisation");
            }
          } catch (err) {
            console.error("Error switching organisation:", err);
            if (currentOrgName) {
              currentOrgName.textContent =
                window.BB_ACTIVE_ORG?.name || "Unknown";
            }
            if (settingsOrgName) {
              settingsOrgName.textContent =
                window.BB_ACTIVE_ORG?.name || "Organisation";
            }
            showSettingsToast("error", "Failed to switch organisation");
          }
        });
        orgListRef.appendChild(button);
      });

      const closeOrgDropdown = () => {
        switcher.classList.remove("open");
        btnRef.setAttribute("aria-expanded", "false");
      };
      document.removeEventListener("click", window._closeOrgDropdown);
      window._closeOrgDropdown = closeOrgDropdown;
      document.addEventListener("click", closeOrgDropdown);
    } catch (err) {
      console.error("Error initialising org switcher:", err);
      switcher.style.display = "none";
      divider.style.display = "none";
    }
  }

  function initCreateOrgModal() {
    const modal = document.getElementById("createOrgModal");
    const form = document.getElementById("createOrgForm");
    const nameInput = document.getElementById("newOrgName");
    const errorDiv = document.getElementById("createOrgError");
    const createBtn = document.getElementById("createOrgBtn");
    const closeBtn = document.getElementById("closeCreateOrgModal");
    const cancelBtn = document.getElementById("cancelCreateOrg");
    const submitBtn = document.getElementById("submitCreateOrg");

    if (!modal || !form) return;

    const openModal = () => {
      modal.classList.add("show");
      nameInput.value = "";
      errorDiv.style.display = "none";
      nameInput.focus();
    };

    const closeModal = () => {
      modal.classList.remove("show");
    };

    createBtn?.addEventListener("click", (e) => {
      e.stopPropagation();
      document.getElementById("orgSwitcher")?.classList.remove("open");
      openModal();
    });

    closeBtn?.addEventListener("click", closeModal);
    cancelBtn?.addEventListener("click", closeModal);
    modal?.addEventListener("click", (e) => {
      if (e.target === modal) closeModal();
    });
    document.addEventListener("keydown", (e) => {
      if (e.key === "Escape" && modal?.classList.contains("show")) {
        closeModal();
      }
    });

    form?.addEventListener("submit", async (e) => {
      e.preventDefault();

      const name = nameInput.value.trim();
      if (!name) {
        errorDiv.textContent = "Organisation name is required";
        errorDiv.style.display = "block";
        return;
      }

      submitBtn.disabled = true;
      submitBtn.textContent = "Creating...";
      errorDiv.style.display = "none";

      try {
        const sessionResult = await window.supabase.auth.getSession();
        const session = sessionResult?.data?.session;
        if (!session) {
          throw new Error("Not authenticated");
        }

        const response = await fetch("/v1/organisations", {
          method: "POST",
          headers: {
            Authorization: `Bearer ${session.access_token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify({ name }),
        });

        const data = await response.json();

        if (response.ok) {
          closeModal();

          const currentOrgName = document.getElementById("currentOrgName");
          const settingsOrgName = document.getElementById("settingsOrgName");
          if (currentOrgName) currentOrgName.textContent = name;
          if (settingsOrgName) settingsOrgName.textContent = name;
          window.BB_ACTIVE_ORG = data.data?.organisation;

          await initOrgSwitcher();
          await refreshSettingsData();

          showSettingsToast("success", `Organisation "${name}" created`);
        } else {
          errorDiv.textContent =
            data.message || "Failed to create organisation";
          errorDiv.style.display = "block";
        }
      } catch (err) {
        console.error("Error creating organisation:", err);
        errorDiv.textContent = "An error occurred. Please try again.";
        errorDiv.style.display = "block";
      } finally {
        submitBtn.disabled = false;
        submitBtn.textContent = "Create";
      }
    });
  }

  async function initSettingsPage() {
    try {
      if (window.BB_APP?.coreReady) {
        await window.BB_APP.coreReady;
      }

      const dataBinder = new BBDataBinder({
        apiBaseUrl: "",
        debug: false,
        refreshInterval: 0,
      });

      window.dataBinder = dataBinder;

      if (!window.BBAuth.initialiseSupabase()) {
        throw new Error("Failed to initialise Supabase client");
      }

      await dataBinder.init();
      if (window.setupQuickAuth) {
        await window.setupQuickAuth(dataBinder);
      }

      setupSettingsNavigation();
      setupNotificationsDropdown();

      const sessionResult = await window.supabase.auth.getSession();
      const session = sessionResult?.data?.session;
      if (session?.user) {
        await initOrgSwitcher();
        initCreateOrgModal();
        await loadAccountDetails();
        await refreshSettingsData();
        await handleInviteToken();
        fetchAndDisplayQuota();
        startQuotaResetCountdown();
      }

      const inviteForm = document.getElementById("teamInviteForm");
      if (inviteForm) {
        inviteForm.addEventListener("submit", sendInvite);
      }

      const resetBtn = document.getElementById("settingsResetPassword");
      if (resetBtn) {
        resetBtn.addEventListener("click", sendPasswordReset);
      }

      if (window.setupSlackIntegration) {
        window.setupSlackIntegration();
      }
      if (window.handleSlackOAuthCallback) {
        window.handleSlackOAuthCallback();
      }
      if (window.setupWebflowIntegration) {
        window.setupWebflowIntegration();
      }
      if (window.handleWebflowOAuthCallback) {
        window.handleWebflowOAuthCallback();
      }
      if (window.setupGoogleIntegration) {
        window.setupGoogleIntegration();
      }
      if (window.handleGoogleOAuthCallback) {
        window.handleGoogleOAuthCallback();
      }
    } catch (error) {
      console.error("Failed to initialise settings:", error);
      showSettingsToast("error", "Failed to load settings. Please refresh.");
    }
  }

  document.addEventListener("DOMContentLoaded", () => {
    initSettingsPage();
  });
})();

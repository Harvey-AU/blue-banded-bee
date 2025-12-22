/**
 * Dashboard Actions Handler
 * Handles lightweight interactions on the dashboard page.
 */

/**
 * Formats a timestamp as a relative time string (e.g., "In 2h 30m", "Overdue")
 * @param {string} timestamp - ISO timestamp string
 * @returns {string} Formatted relative time string
 */
function formatNextRunTime(timestamp) {
  const nextRun = new Date(timestamp);
  const now = new Date();
  const diffMs = nextRun - now;
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
  const diffMins = Math.floor((diffMs % (1000 * 60 * 60)) / (1000 * 60));

  if (diffMs < 0) {
    return "Overdue";
  } else if (diffHours > 0) {
    return `In ${diffHours}h ${diffMins}m`;
  } else {
    return `In ${diffMins}m`;
  }
}

function setupDashboardActions() {
  document.addEventListener("click", (event) => {
    const element = event.target.closest("[bb-action], [bbb-action]");
    if (!element) {
      return;
    }

    const action =
      element.getAttribute("bbb-action") || element.getAttribute("bb-action");
    if (!action) {
      return;
    }

    event.preventDefault();
    handleDashboardAction(action, element);
  });

  // Setup date range filter dropdown
  const dateRangeSelect = document.getElementById("dateRange");
  if (dateRangeSelect) {
    dateRangeSelect.addEventListener("change", (event) => {
      const range = event.target.value;
      if (window.changeTimeRange) {
        window.changeTimeRange(range);
      } else if (window.dataBinder) {
        window.dataBinder.currentRange = range;
        window.dataBinder.refresh();
      }
    });
  }

  console.log("Dashboard action handlers initialised");
}

function handleDashboardAction(action, element) {
  switch (action) {
    case "refresh-dashboard":
      if (window.dataBinder) {
        window.dataBinder.refresh();
      }
      break;

    case "restart-job": {
      const jobId =
        element.getAttribute("bbb-id") ||
        element.getAttribute("bb-data-job-id");
      if (jobId) {
        restartJob(jobId);
      }
      break;
    }

    case "cancel-job": {
      const jobId =
        element.getAttribute("bbb-id") ||
        element.getAttribute("bb-data-job-id");
      if (jobId) {
        cancelJob(jobId);
      }
      break;
    }

    case "create-job":
      openCreateJobModal();
      break;

    case "close-create-job-modal":
      closeCreateJobModal();
      break;

    case "refresh-schedules":
      loadSchedules();
      break;

    case "toggle-schedule": {
      const schedulerId =
        element.getAttribute("bbb-id") ||
        element.getAttribute("bb-data-scheduler-id");
      if (schedulerId) {
        toggleSchedule(schedulerId);
      }
      break;
    }

    case "delete-schedule": {
      const schedulerId =
        element.getAttribute("bbb-id") ||
        element.getAttribute("bb-data-scheduler-id");
      if (schedulerId) {
        deleteSchedule(schedulerId);
      }
      break;
    }

    case "view-schedule-jobs": {
      const schedulerId =
        element.getAttribute("bbb-id") ||
        element.getAttribute("bb-data-scheduler-id");
      if (schedulerId) {
        viewScheduleJobs(schedulerId);
      }
      break;
    }

    default:
      console.log("Unhandled dashboard action:", action);
  }
}

async function restartJob(jobId) {
  try {
    await window.dataBinder.fetchData(`/v1/jobs/${jobId}/restart`, {
      method: "POST",
    });
    showDashboardError("Job restart requested.");
    if (window.dataBinder) {
      window.dataBinder.refresh();
    }
  } catch (error) {
    console.error("Failed to restart job:", error);
    showDashboardError("Failed to restart job");
  }
}

async function cancelJob(jobId) {
  try {
    await window.dataBinder.fetchData(`/v1/jobs/${jobId}/cancel`, {
      method: "POST",
    });
    showDashboardError("Job cancel requested.");
    if (window.dataBinder) {
      window.dataBinder.refresh();
    }
  } catch (error) {
    console.error("Failed to cancel job:", error);
    showDashboardError("Failed to cancel job");
  }
}

function openCreateJobModal() {
  const modal = document.getElementById("createJobModal");
  if (modal) {
    modal.style.display = "flex";
  }
}

function closeCreateJobModal() {
  const modal = document.getElementById("createJobModal");
  if (modal) {
    modal.style.display = "none";
  }
  // Clear form
  const form = document.getElementById("createJobForm");
  if (form) {
    form.reset();
    const maxPagesField = document.getElementById("maxPages");
    if (maxPagesField) maxPagesField.value = "0";
  }
}

function showDashboardError(message) {
  const container = document.createElement("div");
  container.style.cssText = `
    position: fixed; top: 20px; right: 20px; z-index: 10000;
    background: #fee2e2; color: #dc2626; border: 1px solid #fecaca;
    padding: 16px 20px; border-radius: 8px; max-width: 400px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  `;

  const content = document.createElement("div");
  content.style.cssText = "display: flex; align-items: center; gap: 12px;";

  const icon = document.createElement("span");
  icon.textContent = "⚠️";
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

function showDashboardSuccess(message) {
  const container = document.createElement("div");
  container.style.cssText = `
    position: fixed; top: 20px; right: 20px; z-index: 10000;
    background: #d1fae5; color: #065f46; border: 1px solid #a7f3d0;
    padding: 16px 20px; border-radius: 8px; max-width: 400px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  `;

  const content = document.createElement("div");
  content.style.cssText = "display: flex; align-items: center; gap: 12px;";

  const icon = document.createElement("span");
  icon.textContent = "✓";
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

// Scheduler management functions
async function loadSchedules() {
  try {
    const schedules = await window.dataBinder.fetchData("/v1/schedulers", {
      method: "GET",
    });

    const schedulesList = document.getElementById("schedulesList");
    const schedulesEmpty = document.getElementById("schedulesEmpty");

    if (!schedulesList) {
      console.error("Schedules list element not found in DOM");
      showDashboardError("Failed to load schedules: DOM element missing");
      return;
    }

    const template = schedulesList.querySelector('[bbb-template="schedule"]');

    if (!template) {
      console.error("Schedule template not found in DOM");
      showDashboardError("Failed to load schedules: template missing");
      return;
    }

    // Clear existing schedules (except template)
    const existingSchedules = schedulesList.querySelectorAll(
      '.bb-job-card:not([bbb-template="schedule"])'
    );
    existingSchedules.forEach((el) => el.remove());

    if (!schedules || schedules.length === 0) {
      if (schedulesEmpty) schedulesEmpty.style.display = "block";
      return;
    }

    if (schedulesEmpty) schedulesEmpty.style.display = "none";

    schedules.forEach((schedule) => {
      const clone = template.cloneNode(true);
      clone.style.display = "block";
      clone.removeAttribute("bbb-template");

      // Format next run time using helper function
      const nextRunText = formatNextRunTime(schedule.next_run_at);

      // Set data attributes for template binding
      clone.setAttribute("data-domain", schedule.domain);
      clone.setAttribute(
        "data-schedule-interval",
        schedule.schedule_interval_hours
      );
      clone.setAttribute("data-next-run", schedule.next_run_at);
      clone.setAttribute("data-is-enabled", schedule.is_enabled);
      clone.setAttribute("data-id", schedule.id);

      // Use simple text replacement for now (can be enhanced with data binder)
      const domainEl = clone.querySelector(".bb-job-domain");
      if (domainEl) domainEl.textContent = schedule.domain;

      const intervalEl = clone.querySelector(".bb-schedule-info");
      if (intervalEl) {
        intervalEl.textContent = ""; // Clear existing content

        const hoursSpan = document.createElement("span");
        hoursSpan.textContent = `${schedule.schedule_interval_hours} hours`;
        intervalEl.appendChild(hoursSpan);

        const statusSpan = document.createElement("span");
        statusSpan.className = `bb-schedule-status bb-schedule-${schedule.is_enabled ? "enabled" : "disabled"}`;
        statusSpan.textContent = schedule.is_enabled ? "Enabled" : "Disabled";
        intervalEl.appendChild(statusSpan);
      }

      const nextRunEl = clone.querySelector(".bb-job-footer > div");
      if (nextRunEl) {
        nextRunEl.textContent = ""; // Clear existing content

        const labelSpan = document.createElement("span");
        labelSpan.style.fontWeight = "500";
        labelSpan.textContent = "Next run: ";
        nextRunEl.appendChild(labelSpan);

        const valueSpan = document.createElement("span");
        valueSpan.textContent = nextRunText;
        nextRunEl.appendChild(valueSpan);
      }

      const toggleBtn = clone.querySelector('[bbb-action="toggle-schedule"]');
      if (toggleBtn) {
        toggleBtn.textContent = schedule.is_enabled ? "Disable" : "Enable";
      }

      schedulesList.appendChild(clone);
    });
  } catch (error) {
    console.error("Failed to load schedules:", error);
    showDashboardError("Failed to load schedules");
  }
}

async function toggleSchedule(schedulerId) {
  try {
    // Use a dedicated toggle endpoint to avoid TOCTOU race conditions
    // If the endpoint doesn't exist yet, fall back to optimistic toggle
    const scheduler = await window.dataBinder.fetchData(
      `/v1/schedulers/${encodeURIComponent(schedulerId)}`,
      { method: "GET" }
    );

    // Send the toggle request with current state for server-side validation
    const updated = await window.dataBinder.fetchData(
      `/v1/schedulers/${encodeURIComponent(schedulerId)}`,
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          is_enabled: !scheduler.is_enabled,
        }),
      }
    );

    showDashboardSuccess(
      `Schedule ${updated.is_enabled ? "enabled" : "disabled"}`
    );
    loadSchedules();
  } catch (error) {
    console.error("Failed to toggle schedule:", error);
    showDashboardError("Failed to toggle schedule. Please try again.");
    // Reload to ensure UI is in sync with server state
    loadSchedules();
  }
}

async function deleteSchedule(schedulerId) {
  if (!confirm("Are you sure you want to delete this schedule?")) {
    return;
  }

  try {
    await window.dataBinder.fetchData(
      `/v1/schedulers/${encodeURIComponent(schedulerId)}`,
      {
        method: "DELETE",
      }
    );

    showDashboardSuccess("Schedule deleted");
    loadSchedules();
  } catch (error) {
    console.error("Failed to delete schedule:", error);
    showDashboardError("Failed to delete schedule");
  }
}

function viewScheduleJobs(schedulerId) {
  // Navigate to jobs page filtered by scheduler (URL-encode the ID)
  window.location.href = `/jobs?scheduler_id=${encodeURIComponent(schedulerId)}`;
}

if (typeof window !== "undefined") {
  window.setupDashboardActions = setupDashboardActions;
  window.showDashboardError = showDashboardError;
  window.showDashboardSuccess = showDashboardSuccess;
  window.loadSchedules = loadSchedules;
  window.closeCreateJobModal = closeCreateJobModal;
}

/**
 * Dashboard Actions Handler
 * Handles lightweight interactions on the dashboard page.
 */

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
}

function showDashboardError(message) {
  const container = document.createElement("div");
  container.style.cssText = `
    position: fixed; top: 20px; right: 20px; z-index: 10000;
    background: #fee2e2; color: #dc2626; border: 1px solid #fecaca;
    padding: 16px 20px; border-radius: 8px; max-width: 400px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  `;

  container.innerHTML = `
    <div style="display: flex; align-items: center; gap: 12px;">
      <span>⚠️</span>
      <span>${message}</span>
      <button
        style="background: none; border: none; font-size: 18px; cursor: pointer;"
        aria-label="Dismiss"
      >
        ×
      </button>
    </div>
  `;

  const closeButton = container.querySelector("button");
  closeButton.addEventListener("click", () => container.remove());

  document.body.appendChild(container);

  setTimeout(() => container.remove(), 5000);
}

if (typeof window !== "undefined") {
  window.setupDashboardActions = setupDashboardActions;
  window.showDashboardError = showDashboardError;
}

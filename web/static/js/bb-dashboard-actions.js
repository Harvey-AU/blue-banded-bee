(function () {
  'use strict';

  /**
   * Dashboard Actions Handler
   * Restores functionality for dashboard button actions that was lost during refactoring
   */

  // Global state for modal management
  let currentJobId = null;
  let tasksCurrentPage = 0;
  let tasksSortColumn = "created_at";
  let tasksSortDirection = "desc";
  let tasksHasNext = false;

  /**
   * Setup action handlers for dashboard
   * This sets up event delegation for all bb-action attributes
   */
  function setupDashboardActions() {
    // Set up attribute-based event handling using event delegation
    document.addEventListener("click", (e) => {
      const element = e.target.closest("[bb-action]");
      if (element) {
        const action = element.getAttribute("bb-action");
        if (action) {
          e.preventDefault();
          handleDashboardAction(action, element);
        }
      }
    });

    console.log("Dashboard action handlers initialized");
  }

  /**
   * Handle dashboard actions
   */
  function handleDashboardAction(action, element) {
    switch (action) {
      case "refresh-dashboard":
        if (window.dataBinder) {
          window.dataBinder.refresh();
        }
        break;

      case "view-job-details":
        const jobId = element.getAttribute("bb-data-job-id");
        if (jobId) {
          viewJobDetails(jobId);
        }
        break;

      case "close-modal":
        closeModal();
        break;

      case "refresh-tasks":
        if (currentJobId) {
          loadJobTasks(currentJobId);
        }
        break;

      case "tasks-prev-page":
        if (tasksCurrentPage > 0) {
          tasksCurrentPage--;
          loadJobTasks(currentJobId);
        }
        break;

      case "tasks-next-page":
        if (tasksHasNext) {
          tasksCurrentPage++;
          loadJobTasks(currentJobId);
        }
        break;

      case "toggle-export-menu":
        const menu = document.getElementById("exportDropdownMenu");
        if (menu) {
          menu.style.display = menu.style.display === "block" ? "none" : "block";
        }
        break;

      case "export-job":
      case "export-broken-links":
      case "export-slow-pages":
        exportTasks(action.replace("export-", ""));
        break;

      case "restart-job":
      case "restart-job-modal":
        const restartJobId = element.getAttribute("bb-data-job-id") || currentJobId;
        if (restartJobId) {
          restartJob(restartJobId);
        }
        break;

      case "cancel-job":
      case "cancel-job-modal":
        const cancelJobId = element.getAttribute("bb-data-job-id") || currentJobId;
        if (cancelJobId) {
          cancelJob(cancelJobId);
        }
        break;

      case "create-job":
        openCreateJobModal();
        break;

      case "close-create-job-modal":
        closeCreateJobModal();
        break;

      case "refresh-slow-pages":
        refreshSlowPages();
        break;

      case "refresh-redirects":
        refreshExternalRedirects();
        break;

      default:
        console.log("Unhandled action:", action);
    }
  }

  /**
   * View job details in modal
   */
  async function viewJobDetails(jobId) {
    if (!jobId) {
      console.error("No job ID provided");
      return;
    }

    currentJobId = jobId;

    // Reset pagination state
    tasksCurrentPage = 0;
    tasksSortColumn = "created_at";
    tasksSortDirection = "desc";

    // Reset filter tabs to "All"
    const modalTabs = document.querySelectorAll("#modalTaskFilterTabs .filter-tab");
    modalTabs.forEach((tab) => {
      tab.classList.remove("active");
      if (tab.dataset.status === "") {
        tab.classList.add("active");
      }
    });

    try {
      // Load job details using dataBinder
      const job = await window.dataBinder.fetchData(`/v1/jobs/${jobId}`);

      // Update modal content
      document.getElementById("modal-job-id").textContent = jobId;
      document.getElementById("modal-domain").textContent = job.domain || "-";
      document.getElementById("modal-status").textContent = job.status || "-";
      document.getElementById("modal-progress").textContent = `${Math.round(job.progress || 0)}%`;
      document.getElementById("modal-total-tasks").textContent = job.total_tasks || 0;
      document.getElementById("modal-completed-tasks").textContent = job.completed_tasks || 0;
      document.getElementById("modal-failed-tasks").textContent = job.failed_tasks || 0;

      // Format time fields
      const startedAt = job.started_at ? new Date(job.started_at) : null;
      const completedAt = job.completed_at ? new Date(job.completed_at) : null;

      document.getElementById("modal-started-at").textContent = startedAt ? startedAt.toLocaleString() : "-";
      document.getElementById("modal-completed-at").textContent = completedAt ? completedAt.toLocaleString() : "-";

      // Calculate total time
      let totalTimeText = "-";
      let avgTimeText = "-";

      if (startedAt && completedAt) {
        const totalMs = completedAt - startedAt;
        const totalMinutes = Math.round(totalMs / 60000);
        const hours = Math.floor(totalMinutes / 60);
        const minutes = totalMinutes % 60;

        if (hours > 0) {
          totalTimeText = `${hours}h ${minutes}m`;
        } else {
          totalTimeText = `${minutes}m`;
        }

        // Calculate average time per completed task
        const completedTasks = job.completed_tasks || 0;
        if (completedTasks > 0) {
          const avgMs = totalMs / completedTasks;
          const avgSeconds = Math.round(avgMs / 1000);
          if (avgSeconds < 60) {
            avgTimeText = `${avgSeconds}s`;
          } else {
            const avgMin = Math.floor(avgSeconds / 60);
            const avgSec = avgSeconds % 60;
            avgTimeText = `${avgMin}m ${avgSec}s`;
          }
        }
      }

      document.getElementById("modal-total-time").textContent = totalTimeText;
      document.getElementById("modal-avg-time").textContent = avgTimeText;

      // Show/hide action buttons
      const restartBtn = document.getElementById("modal-restart-btn");
      const cancelBtn = document.getElementById("modal-cancel-btn");

      if (["completed", "failed", "cancelled"].includes(job.status)) {
        restartBtn.style.display = "inline-block";
        cancelBtn.style.display = "none";
      } else if (["running", "pending"].includes(job.status)) {
        restartBtn.style.display = "none";
        cancelBtn.style.display = "inline-block";
      } else {
        restartBtn.style.display = "none";
        cancelBtn.style.display = "none";
      }

      // Load tasks
      await loadJobTasks(jobId);

      // Show modal
      showModal();
    } catch (error) {
      console.error("Failed to load job details:", error);
      if (window.Sentry) {
        window.Sentry.captureException(error, {
          tags: { component: 'dashboard', action: 'view_job_details' }
        });
      }
      showDashboardError("Failed to load job details. Please check your connection and try again.");
    }
  }

  /**
   * Load job tasks for modal
   */
  async function loadJobTasks(jobId) {
    const tasksContent = document.getElementById("tasksContent");
    const pagination = document.getElementById("tasksPagination");
    const limitSelect = document.getElementById("tasksLimit");

    tasksContent.innerHTML = '<div class="bb-loading">Loading tasks...</div>';
    pagination.style.display = "none";

    try {
      const limit = limitSelect ? parseInt(limitSelect.value) : 50;
      const offset = tasksCurrentPage * limit;
      const sort = tasksSortDirection === "desc" ? "-" + tasksSortColumn : tasksSortColumn;

      // Get current status filter
      const activeTab = document.querySelector("#modalTaskFilterTabs .filter-tab.active");
      const statusFilter = activeTab ? activeTab.dataset.status : "";

      // Build URL with status filter
      let url = `/v1/jobs/${jobId}/tasks?limit=${limit}&offset=${offset}&sort=${sort}`;
      if (statusFilter) {
        url += `&status=${statusFilter}`;
      }

      const response = await window.dataBinder.fetchData(url);
      const tasks = response.tasks || [];

      if (tasks.length === 0 && tasksCurrentPage === 0) {
        tasksContent.innerHTML = '<div class="bb-loading">No tasks found</div>';
        return;
      }

      // Build sortable table header
      const getSortIcon = (column) => {
        if (tasksSortColumn === column) {
          return tasksSortDirection === "desc" ? " ↓" : " ↑";
        }
        return "";
      };

      // Build tasks table with sortable headers
      let tableHTML = `
    <table class="bb-tasks-table">
      <thead>
        <tr>
          <th style="cursor: pointer;" onclick="sortTasks('path')">Path${getSortIcon("path")}</th>
          <th style="cursor: pointer;" onclick="sortTasks('status')">Status${getSortIcon("status")}</th>
          <th style="cursor: pointer;" onclick="sortTasks('response_time')">Response Time${getSortIcon("response_time")}</th>
          <th style="cursor: pointer;" onclick="sortTasks('cache_status')">Cache Status${getSortIcon("cache_status")}</th>
          <th style="cursor: pointer;" onclick="sortTasks('second_response_time')">2nd Request${getSortIcon("second_response_time")}</th>
          <th style="cursor: pointer;" onclick="sortTasks('status_code')">Status Code${getSortIcon("status_code")}</th>
        </tr>
      </thead>
      <tbody>
    `;

      tasks.forEach((task) => {
        const statusClass = `bb-status-${task.status}`;

        // Format second request data (just response time, no cache status)
        const secondRequest = task.second_response_time ? `${task.second_response_time}ms` : "-";

        tableHTML += `
        <tr>
          <td><a href="${task.url}" target="_blank"><code class="bb-task-path">${task.path}</code></a></td>
          <td><span class="bb-task-status ${statusClass}" ${task.error ? `title="${task.error.substring(0, 50)}${task.error.length > 50 ? "..." : ""}"` : ""}>${task.status}</span></td>
          <td>${task.response_time ? `${task.response_time}ms` : "-"}</td>
          <td ${task.error ? `title="${task.error.substring(0, 50)}${task.error.length > 50 ? "..." : ""}"` : ""}>${task.cache_status || "-"}</td>
          <td>${secondRequest}</td>
          <td>${task.status_code || "-"}</td>
        </tr>
      `;
      });

      tableHTML += "</tbody></table>";
      tasksContent.innerHTML = tableHTML;

      // Update pagination
      if (response.pagination) {
        const { total, has_next, has_prev } = response.pagination;
        tasksHasNext = has_next;

        const startItem = offset + 1;
        const endItem = Math.min(offset + parseInt(limit), total);

        document.getElementById("tasksPageInfo").textContent = `${startItem}-${endItem} of ${total} tasks`;

        document.getElementById("prevTasksBtn").disabled = !has_prev;
        document.getElementById("nextTasksBtn").disabled = !has_next;

        pagination.style.display = total > limit ? "block" : "none";
      }
    } catch (error) {
      console.error("Failed to load tasks:", error);
      tasksContent.innerHTML = '<div class="bb-error">Failed to load tasks</div>';
    }
  }

  /**
   * Sort tasks table
   */
  function sortTasks(column) {
    if (tasksSortColumn === column) {
      tasksSortDirection = tasksSortDirection === "desc" ? "asc" : "desc";
    } else {
      tasksSortColumn = column;
      tasksSortDirection = "desc";
    }
    loadJobTasks(currentJobId);
  }

  /**
   * Show modal
   */
  function showModal() {
    const modal = document.getElementById("jobDetailsModal");
    if (modal) {
      modal.style.display = "flex";
    }
  }

  /**
   * Close modal
   */
  function closeModal() {
    const modal = document.getElementById("jobDetailsModal");
    if (modal) {
      modal.style.display = "none";
    }
    currentJobId = null;
  }

  /**
   * Restart job
   */
  async function restartJob(jobId) {
    if (!jobId) return;

    try {
      await window.dataBinder.fetchData(`/v1/jobs/${jobId}/restart`, { method: "POST" });

      // Close modal and refresh dashboard
      closeModal();
      if (window.dataBinder) {
        window.dataBinder.refresh();
      }
    } catch (error) {
      console.error("Failed to restart job:", error);
      showDashboardError("Failed to restart job");
    }
  }

  /**
   * Cancel job
   */
  async function cancelJob(jobId) {
    if (!jobId) return;

    try {
      await window.dataBinder.fetchData(`/v1/jobs/${jobId}/cancel`, { method: "POST" });

      // Close modal and refresh dashboard
      closeModal();
      if (window.dataBinder) {
        window.dataBinder.refresh();
      }
    } catch (error) {
      console.error("Failed to cancel job:", error);
      showDashboardError("Failed to cancel job");
    }
  }

  /**
   * Export tasks
   */
  async function exportTasks(type) {
    if (!currentJobId) return;

    try {
      let url = `/v1/jobs/${currentJobId}/export`;
      if (type !== "job") {
        url += `?type=${type}`;
      }

      const response = await window.dataBinder.fetchData(url);

      // Create download link
      const blob = new Blob([JSON.stringify(response, null, 2)], { type: "application/json" });
      const downloadUrl = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = downloadUrl;
      a.download = `${currentJobId}-${type}.json`;
      a.click();
      URL.revokeObjectURL(downloadUrl);
    } catch (error) {
      console.error("Failed to export tasks:", error);
      showDashboardError("Failed to export tasks");
    }
  }

  /**
   * Open create job modal
   */
  function openCreateJobModal() {
    const modal = document.getElementById("createJobModal");
    if (modal) {
      modal.style.display = "flex";
    }
  }

  /**
   * Close create job modal
   */
  function closeCreateJobModal() {
    const modal = document.getElementById("createJobModal");
    if (modal) {
      modal.style.display = "none";
    }
  }

  /**
   * Refresh slow pages section
   */
  async function refreshSlowPages() {
    if (window.dataBinder) {
      // Trigger refresh of slow pages data
      await window.dataBinder.refresh();
    }
  }

  /**
   * Refresh external redirects section
   */
  async function refreshExternalRedirects() {
    if (window.dataBinder) {
      // Trigger refresh of redirects data
      await window.dataBinder.refresh();
    }
  }

  /**
   * Show dashboard error
   */
  function showDashboardError(message) {
    const errorNotification = document.createElement("div");
    errorNotification.style.cssText = `
    position: fixed; top: 20px; right: 20px; z-index: 10000;
    background: #fee2e2; color: #dc2626; border: 1px solid #fecaca;
    padding: 16px 20px; border-radius: 8px; max-width: 400px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  `;
    errorNotification.innerHTML = `
    <div style="display: flex; align-items: center; gap: 12px;">
      <span>⚠️</span>
      <span>${message}</span>
      <button onclick="this.parentElement.parentElement.remove()" style="background: none; border: none; font-size: 18px; cursor: pointer;">×</button>
    </div>
  `;
    document.body.appendChild(errorNotification);

    // Auto-remove after 5 seconds
    setTimeout(() => errorNotification.remove(), 5000);
  }

  // Export functions for global use
  if (typeof window !== "undefined") {
    window.setupDashboardActions = setupDashboardActions;
    window.sortTasks = sortTasks;
    window.viewJobDetails = viewJobDetails;
    window.loadJobTasks = loadJobTasks;
  }

})();

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
 * Helper function to get nested value from object
 */
function getNestedValue(obj, path) {
  return path.split('.').reduce((current, key) => current?.[key], obj);
}

/**
 * Setup action handlers for dashboard
 * Supports both bb-action and bbb-action attributes
 */
function setupDashboardActions() {
  // Set up attribute-based event handling using event delegation
  // Support both old (bb-action) and new (bbb-action) formats
  document.addEventListener("click", (e) => {
    const element = e.target.closest("[bb-action], [bbb-action]");
    if (element) {
      const action = element.getAttribute("bbb-action") || element.getAttribute("bb-action");
      if (action) {
        e.preventDefault();
        handleDashboardAction(action, element);
      }
    }
  });

  // Set up filter tab click handlers for the modal
  document.addEventListener("click", (e) => {
    const filterTab = e.target.closest("#modalTaskFilterTabs .filter-tab");
    if (filterTab) {
      e.preventDefault();
      handleFilterTabClick(filterTab);
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
      // Support both old (bb-data-job-id) and new (bbb-id) formats
      const jobId = element.getAttribute("bbb-id") || element.getAttribute("bb-data-job-id");
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
      // Support both old (bb-data-job-id) and new (bbb-id) formats
      const restartJobId = element.getAttribute("bbb-id") || element.getAttribute("bb-data-job-id") || currentJobId;
      if (restartJobId) {
        restartJob(restartJobId);
      }
      break;

    case "cancel-job":
    case "cancel-job-modal":
      // Support both old (bb-data-job-id) and new (bbb-id) formats
      const cancelJobId = element.getAttribute("bbb-id") || element.getAttribute("bb-data-job-id") || currentJobId;
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

    // Add formatted fields for display
    job.id = jobId;
    job.progress_formatted = `${Math.round(job.progress || 0)}%`;

    // Format time fields
    const startedAt = job.started_at ? new Date(job.started_at) : null;
    const completedAt = job.completed_at ? new Date(job.completed_at) : null;

    job.started_at_formatted = startedAt ? startedAt.toLocaleString() : "-";
    job.completed_at_formatted = completedAt ? completedAt.toLocaleString() : "-";

    // Format total duration (from database duration_seconds)
    if (job.duration_seconds != null) {
      const totalSeconds = job.duration_seconds;
      const hours = Math.floor(totalSeconds / 3600);
      const minutes = Math.floor((totalSeconds % 3600) / 60);
      const seconds = totalSeconds % 60;

      if (hours > 0) {
        job.duration_formatted = `${hours}h ${minutes}m ${seconds}s`;
      } else if (minutes > 0) {
        job.duration_formatted = `${minutes}m ${seconds}s`;
      } else {
        job.duration_formatted = `${seconds}s`;
      }
    } else {
      job.duration_formatted = "-";
    }

    // Format average time per task (from database avg_time_per_task_seconds)
    if (job.avg_time_per_task_seconds != null) {
      const avgSeconds = parseFloat(job.avg_time_per_task_seconds);

      if (avgSeconds >= 60) {
        const avgMin = Math.floor(avgSeconds / 60);
        const avgSec = (avgSeconds % 60).toFixed(2);
        job.avg_time_formatted = `${avgMin}m ${avgSec}s`;
      } else {
        job.avg_time_formatted = `${avgSeconds.toFixed(2)}s`;
      }
    } else {
      job.avg_time_formatted = "-";
    }

    // Format stats fields if they exist
    if (job.stats) {
      if (job.stats.cache_stats && job.stats.cache_stats.hit_rate) {
        job.stats.cache_stats.hit_rate = `${job.stats.cache_stats.hit_rate}%`;
      }
      if (job.stats.cache_warming_effect && job.stats.cache_warming_effect.total_time_saved_seconds) {
        job.stats.cache_warming_effect.total_time_saved_seconds = `${job.stats.cache_warming_effect.total_time_saved_seconds}s`;
      }
    }

    // Update all data-bound elements in the modal automatically
    const modalContainer = document.getElementById("modal-job-data");
    if (modalContainer && window.dataBinder) {
      // Find all data binding elements (both old and new formats)
      const bindElements = modalContainer.querySelectorAll('[data-bb-bind], [bbb-text]');
      bindElements.forEach(el => {
        // Support both old (data-bb-bind) and new (bbb-text) formats
        const path = el.getAttribute('bbb-text') || el.getAttribute('data-bb-bind');
        const value = getNestedValue(job, path);
        if (value !== undefined && value !== null) {
          el.textContent = value;
        }
      });
    }

    // Display additional stats if available
    if (job.stats) {
      displayJobStats(job.stats);
    }

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
 * Display job statistics from database
 */
function displayJobStats(stats) {
  if (!stats) return;

  // Find or create stats container in modal
  let statsContainer = document.getElementById("modal-stats-container");
  if (!statsContainer) {
    // Create stats container after the job info grid
    const infoGrid = document.querySelector(".bb-job-info-grid");
    if (!infoGrid) return;

    statsContainer = document.createElement("div");
    statsContainer.id = "modal-stats-container";
    statsContainer.className = "bb-modal-section";

    // Insert after the parent of info grid
    infoGrid.parentElement.insertAdjacentElement("afterend", statsContainer);
  }

  let statsHTML = '<div class="bb-modal-section-title">Performance Statistics</div>';

  // Slow Page Buckets
  if (stats.slow_page_buckets) {
    const buckets = stats.slow_page_buckets;
    statsHTML += '<div style="margin-bottom: 24px;">';
    statsHTML += '<h4 style="font-size: 14px; font-weight: 600; margin-bottom: 12px;">Response Time Distribution</h4>';
    statsHTML += '<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 12px;">';

    if (buckets.over_10s > 0) {
      statsHTML += `<div class="bb-info-item" style="background: #fee2e2;">
        <div class="bb-info-label" bbb-help="response_time_over_10s">Over 10s</div>
        <div class="bb-info-value" style="color: #dc2626;">${buckets.over_10s}</div>
      </div>`;
    }
    if (buckets['5_to_10s'] > 0) {
      statsHTML += `<div class="bb-info-item" style="background: #fed7aa;">
        <div class="bb-info-label" bbb-help="response_time_5_to_10s">5-10s</div>
        <div class="bb-info-value" style="color: #ea580c;">${buckets['5_to_10s']}</div>
      </div>`;
    }
    if (buckets['3_to_5s'] > 0) {
      statsHTML += `<div class="bb-info-item" style="background: #fef3c7;">
        <div class="bb-info-label" bbb-help="response_time_3_to_5s">3-5s</div>
        <div class="bb-info-value" style="color: #d97706;">${buckets['3_to_5s']}</div>
      </div>`;
    }
    if (buckets['2_to_3s'] > 0) {
      statsHTML += `<div class="bb-info-item">
        <div class="bb-info-label" bbb-help="response_time_2_to_3s">2-3s</div>
        <div class="bb-info-value">${buckets['2_to_3s']}</div>
      </div>`;
    }
    if (buckets['1_5_to_2s'] > 0) {
      statsHTML += `<div class="bb-info-item">
        <div class="bb-info-label" bbb-help="response_time_1_5_to_2s">1.5-2s</div>
        <div class="bb-info-value">${buckets['1_5_to_2s']}</div>
      </div>`;
    }
    if (buckets['1_to_1_5s'] > 0) {
      statsHTML += `<div class="bb-info-item">
        <div class="bb-info-label" bbb-help="response_time_1_to_1_5s">1-1.5s</div>
        <div class="bb-info-value">${buckets['1_to_1_5s']}</div>
      </div>`;
    }
    statsHTML += `<div class="bb-info-item">
      <div class="bb-info-label" bbb-help="response_time_500ms_to_1s">500ms-1s</div>
      <div class="bb-info-value">${buckets['500ms_to_1s'] || 0}</div>
    </div>`;
    statsHTML += `<div class="bb-info-item" style="background: #dcfce7;">
      <div class="bb-info-label" bbb-help="response_time_under_500ms">Under 500ms</div>
      <div class="bb-info-value" style="color: #16a34a;">${buckets.under_500ms || 0}</div>
    </div>`;

    // Show total slow pages if any
    if (buckets.total_slow_over_3s > 0) {
      statsHTML += `<div class="bb-info-item" style="grid-column: span 2; background: #fee2e2;">
        <div class="bb-info-label" bbb-help="total_slow_over_3s">Total Slow (>3s)</div>
        <div class="bb-info-value" style="color: #dc2626; font-size: 20px;">${buckets.total_slow_over_3s}</div>
      </div>`;
    }

    statsHTML += '</div></div>';
  }

  // Cache Performance
  if (stats.cache_stats || stats.cache_warming_effect) {
    statsHTML += '<div style="margin-bottom: 24px;">';
    statsHTML += '<h4 style="font-size: 14px; font-weight: 600; margin-bottom: 12px;">Cache Performance</h4>';
    statsHTML += '<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px;">';

    if (stats.cache_stats) {
      statsHTML += `<div class="bb-info-item">
        <div class="bb-info-label" bbb-help="cache_hits">Cache Hits</div>
        <div class="bb-info-value">${stats.cache_stats.hits}</div>
      </div>`;
      statsHTML += `<div class="bb-info-item">
        <div class="bb-info-label" bbb-help="cache_misses">Cache Misses</div>
        <div class="bb-info-value">${stats.cache_stats.misses}</div>
      </div>`;
      if (stats.cache_stats.hit_rate) {
        statsHTML += `<div class="bb-info-item" style="background: ${stats.cache_stats.hit_rate > 80 ? '#dcfce7' : '#f9fafb'};">
          <div class="bb-info-label" bbb-help="hit_rate">Hit Rate</div>
          <div class="bb-info-value" style="color: ${stats.cache_stats.hit_rate > 80 ? '#16a34a' : '#1f2937'};">${stats.cache_stats.hit_rate}%</div>
        </div>`;
      }
    }

    if (stats.cache_warming_effect) {
      const effect = stats.cache_warming_effect;
      if (effect.total_time_saved_seconds > 0) {
        statsHTML += `<div class="bb-info-item" style="background: #dbeafe;">
          <div class="bb-info-label" bbb-help="time_saved">Time Saved</div>
          <div class="bb-info-value" style="color: #1d4ed8;">${effect.total_time_saved_seconds}s</div>
        </div>`;
      }
      if (effect.avg_time_saved_per_page_ms > 0) {
        statsHTML += `<div class="bb-info-item">
          <div class="bb-info-label" bbb-help="avg_saved_per_page">Avg Saved/Page</div>
          <div class="bb-info-value">${Math.round(effect.avg_time_saved_per_page_ms)}ms</div>
        </div>`;
      }
      if (effect.improvement_rate > 0) {
        statsHTML += `<div class="bb-info-item">
          <div class="bb-info-label" bbb-help="improvement_rate">Improvement Rate</div>
          <div class="bb-info-value">${effect.improvement_rate}%</div>
        </div>`;
      }
    }

    statsHTML += '</div></div>';
  }

  // Response Time Percentiles
  if (stats.response_times) {
    const times = stats.response_times;
    statsHTML += '<div style="margin-bottom: 24px;">';
    statsHTML += '<h4 style="font-size: 14px; font-weight: 600; margin-bottom: 12px;">Response Time Percentiles</h4>';
    statsHTML += '<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 12px;">';

    statsHTML += `<div class="bb-info-item">
      <div class="bb-info-label" bbb-help="p25">P25</div>
      <div class="bb-info-value">${Math.round(times.p25_ms)}ms</div>
    </div>`;
    statsHTML += `<div class="bb-info-item">
      <div class="bb-info-label" bbb-help="median">Median</div>
      <div class="bb-info-value">${Math.round(times.median_ms)}ms</div>
    </div>`;
    statsHTML += `<div class="bb-info-item">
      <div class="bb-info-label" bbb-help="p75">P75</div>
      <div class="bb-info-value">${Math.round(times.p75_ms)}ms</div>
    </div>`;
    statsHTML += `<div class="bb-info-item">
      <div class="bb-info-label" bbb-help="p90">P90</div>
      <div class="bb-info-value">${Math.round(times.p90_ms)}ms</div>
    </div>`;
    statsHTML += `<div class="bb-info-item">
      <div class="bb-info-label" bbb-help="p95">P95</div>
      <div class="bb-info-value">${Math.round(times.p95_ms)}ms</div>
    </div>`;
    statsHTML += `<div class="bb-info-item">
      <div class="bb-info-label" bbb-help="p99">P99</div>
      <div class="bb-info-value">${Math.round(times.p99_ms)}ms</div>
    </div>`;

    statsHTML += '</div></div>';
  }

  // Issues Found
  if (stats.total_broken_links > 0 || stats.total_404s > 0 || stats.redirect_stats) {
    statsHTML += '<div style="margin-bottom: 24px;">';
    statsHTML += '<h4 style="font-size: 14px; font-weight: 600; margin-bottom: 12px;">Issues Found</h4>';
    statsHTML += '<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 12px;">';

    if (stats.total_broken_links > 0) {
      statsHTML += `<div class="bb-info-item" style="background: #fee2e2;">
        <div class="bb-info-label">Broken Links</div>
        <div class="bb-info-value" style="color: #dc2626;">${stats.total_broken_links}</div>
      </div>`;
    }
    if (stats.total_404s > 0) {
      statsHTML += `<div class="bb-info-item" style="background: #fee2e2;">
        <div class="bb-info-label">404 Errors</div>
        <div class="bb-info-value" style="color: #dc2626;">${stats.total_404s}</div>
      </div>`;
    }
    if (stats.total_server_errors > 0) {
      statsHTML += `<div class="bb-info-item" style="background: #fee2e2;">
        <div class="bb-info-label">Server Errors</div>
        <div class="bb-info-value" style="color: #dc2626;">${stats.total_server_errors}</div>
      </div>`;
    }

    if (stats.redirect_stats && stats.redirect_stats.total > 0) {
      statsHTML += `<div class="bb-info-item">
        <div class="bb-info-label">Total Redirects</div>
        <div class="bb-info-value">${stats.redirect_stats.total}</div>
      </div>`;
      if (stats.redirect_stats['301_permanent'] > 0) {
        statsHTML += `<div class="bb-info-item">
          <div class="bb-info-label">301 Permanent</div>
          <div class="bb-info-value">${stats.redirect_stats['301_permanent']}</div>
        </div>`;
      }
      if (stats.redirect_stats['302_temporary'] > 0) {
        statsHTML += `<div class="bb-info-item">
          <div class="bb-info-label">302 Temporary</div>
          <div class="bb-info-value">${stats.redirect_stats['302_temporary']}</div>
        </div>`;
      }
    }

    statsHTML += '</div></div>';
  }

  // Discovery Sources
  if (stats.discovery_sources) {
    const sources = stats.discovery_sources;
    statsHTML += '<div style="margin-bottom: 24px;">';
    statsHTML += '<h4 style="font-size: 14px; font-weight: 600; margin-bottom: 12px;">URL Discovery</h4>';
    statsHTML += '<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 12px;">';

    if (sources.sitemap > 0) {
      statsHTML += `<div class="bb-info-item">
        <div class="bb-info-label">From Sitemap</div>
        <div class="bb-info-value">${sources.sitemap}</div>
      </div>`;
    }
    if (sources.discovered > 0) {
      statsHTML += `<div class="bb-info-item">
        <div class="bb-info-label">Discovered</div>
        <div class="bb-info-value">${sources.discovered}</div>
      </div>`;
    }
    if (sources.manual > 0) {
      statsHTML += `<div class="bb-info-item">
        <div class="bb-info-label">Manual</div>
        <div class="bb-info-value">${sources.manual}</div>
      </div>`;
    }

    statsHTML += '</div></div>';
  }

  statsContainer.innerHTML = statsHTML;

  // Refresh metadata tooltips for dynamically added elements
  if (window.metricsMetadata && window.metricsMetadata.isLoaded()) {
    window.metricsMetadata.refresh();
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

    // Build tasks table with sortable headers and data-binding support
    let tableHTML = `
    <table class="bb-tasks-table">
      <thead>
        <tr>
          <th style="cursor: pointer;" onclick="sortTasks('path')" bbb-help="task_path">Path${getSortIcon("path")}</th>
          <th style="cursor: pointer;" onclick="sortTasks('status')" bbb-help="task_status">Status${getSortIcon("status")}</th>
          <th style="cursor: pointer;" onclick="sortTasks('response_time')" bbb-help="task_response_time">Response Time${getSortIcon("response_time")}</th>
          <th style="cursor: pointer;" onclick="sortTasks('cache_status')" bbb-help="task_cache_status">Cache Status${getSortIcon("cache_status")}</th>
          <th style="cursor: pointer;" onclick="sortTasks('second_response_time')" bbb-help="task_second_request">2nd Request${getSortIcon("second_response_time")}</th>
          <th style="cursor: pointer;" onclick="sortTasks('status_code')" bbb-help="task_status_code">Status Code${getSortIcon("status_code")}</th>
        </tr>
      </thead>
      <tbody id="tasks-table-body">
    `;

    tasks.forEach((task, index) => {
      const statusClass = `bb-status-${task.status}`;

      // Format display values
      task.response_time_formatted = task.second_response_time ? `${task.second_response_time}ms` : (task.response_time ? `${task.response_time}ms` : "-");
      task.second_response_time_formatted = task.second_response_time ? `${task.second_response_time}ms` : "-";
      task.cache_status_display = task.cache_status || "-";
      task.status_code_display = task.status_code || "-";
      task.error_tooltip = task.error ? task.error.substring(0, 50) + (task.error.length > 50 ? "..." : "") : "";

      tableHTML += `
        <tr data-task-index="${index}">
          <td><a href="${task.url}" target="_blank"><code class="bb-task-path" bbb-text="tasks.${index}.path">${task.path}</code></a></td>
          <td><span class="bb-task-status ${statusClass}" bbb-text="tasks.${index}.status" ${task.error_tooltip ? `title="${task.error_tooltip}"` : ""}>${task.status}</span></td>
          <td bbb-text="tasks.${index}.response_time_formatted">${task.response_time_formatted}</td>
          <td bbb-text="tasks.${index}.cache_status_display" ${task.error_tooltip ? `title="${task.error_tooltip}"` : ""}>${task.cache_status_display}</td>
          <td bbb-text="tasks.${index}.second_response_time_formatted">${task.second_response_time_formatted}</td>
          <td bbb-text="tasks.${index}.status_code_display">${task.status_code_display}</td>
        </tr>
      `;
    });

    tableHTML += "</tbody></table>";
    tasksContent.innerHTML = tableHTML;

    // Refresh info icons for dynamically added table headers
    if (window.metricsMetadata) {
      window.metricsMetadata.refresh();
    }

    // Store tasks data for potential data-binding updates
    if (window.currentTasksData) {
      window.currentTasksData.tasks = tasks;
    } else {
      window.currentTasksData = { tasks };
    }

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
 * Handle filter tab clicks
 */
function handleFilterTabClick(tab) {
  // Remove active class from all tabs
  const allTabs = document.querySelectorAll("#modalTaskFilterTabs .filter-tab");
  allTabs.forEach(t => t.classList.remove("active"));

  // Add active class to clicked tab
  tab.classList.add("active");

  // Reset page to 0 when filter changes
  tasksCurrentPage = 0;

  // Reload tasks with new filter
  if (currentJobId) {
    loadJobTasks(currentJobId);
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

    // Convert JSON to CSV
    const csv = convertToCSV(response);

    // Create download link
    const blob = new Blob([csv], { type: "text/csv" });
    const downloadUrl = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = downloadUrl;
    a.download = `${currentJobId}-${type}.csv`;
    a.click();
    URL.revokeObjectURL(downloadUrl);
  } catch (error) {
    console.error("Failed to export tasks:", error);
    showDashboardError("Failed to export tasks");
  }
}

/**
 * Convert JSON data to CSV format
 */
function convertToCSV(data) {
  // Handle different response formats
  let tasks = [];

  // Check for standardised API response format
  if (data.data && data.data.tasks && Array.isArray(data.data.tasks)) {
    tasks = data.data.tasks;
  } else if (Array.isArray(data)) {
    tasks = data;
  } else if (data.tasks && Array.isArray(data.tasks)) {
    tasks = data.tasks;
  } else {
    throw new Error("Unexpected data format");
  }

  if (tasks.length === 0) {
    return "No data to export";
  }

  // Get all unique keys from all tasks
  const headers = new Set();
  tasks.forEach(task => {
    Object.keys(task).forEach(key => headers.add(key));
  });
  const headerArray = Array.from(headers);

  // Build CSV header row
  const csvRows = [];
  csvRows.push(headerArray.map(h => escapeCSVValue(h)).join(","));

  // Build CSV data rows
  tasks.forEach(task => {
    const row = headerArray.map(header => {
      const value = task[header];
      return escapeCSVValue(value);
    });
    csvRows.push(row.join(","));
  });

  return csvRows.join("\n");
}

/**
 * Escape CSV values properly
 */
function escapeCSVValue(value) {
  if (value === null || value === undefined) {
    return "";
  }

  const stringValue = String(value);

  // If value contains comma, quote, or newline, wrap in quotes and escape internal quotes
  if (stringValue.includes(",") || stringValue.includes('"') || stringValue.includes("\n")) {
    return '"' + stringValue.replace(/"/g, '""') + '"';
  }

  return stringValue;
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
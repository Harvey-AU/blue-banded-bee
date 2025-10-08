const DEFAULT_PAGE_SIZE = 50;
const PAGE_SIZE_OPTIONS = [25, 50, 100, 200];

const integerFormatter = new Intl.NumberFormat("en-AU", { maximumFractionDigits: 0 });
const decimalFormatter = new Intl.NumberFormat("en-AU", { minimumFractionDigits: 0, maximumFractionDigits: 2 });
const METRIC_GROUP_KEYS = ["cache", "warming", "performance", "distribution", "reliability", "discovery", "redirects"];

function hasNonNullValue(obj) {
  if (!obj) {
    return false;
  }
  return Object.values(obj).some((value) => value !== null && value !== undefined);
}

function escapeHTML(value) {
  return String(value ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function formatCount(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) {
    return "0";
  }
  return integerFormatter.format(numeric);
}

function formatDecimal(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) {
    return "0";
  }
  return decimalFormatter.format(numeric);
}

function formatMilliseconds(value, { empty = "0ms" } = {}) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) {
    return empty;
  }
  return `${decimalFormatter.format(numeric)}ms`;
}

function formatSeconds(value, { empty = "0s" } = {}) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) {
    return empty;
  }
  return `${decimalFormatter.format(numeric)}s`;
}

function formatPercentage(value, { empty = "0%" } = {}) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) {
    return empty;
  }
  return `${decimalFormatter.format(numeric)}%`;
}

function formatDateTime(value) {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "—";
  }
  return date.toLocaleString();
}

function formatDuration(totalSeconds) {
  if (totalSeconds == null || Number.isNaN(Number(totalSeconds))) {
    return "—";
  }
  const seconds = Math.max(0, Number(totalSeconds));
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const remaining = Math.floor(seconds % 60);

  if (hours > 0) {
    return `${hours}h ${minutes}m ${remaining}s`;
  }
  if (minutes > 0) {
    return `${minutes}m ${remaining}s`;
  }
  return `${remaining}s`;
}

function formatAverageSeconds(seconds) {
  if (seconds == null || Number.isNaN(Number(seconds))) {
    return "—";
  }
  const numeric = Number(seconds);
  if (numeric >= 60) {
    const mins = Math.floor(numeric / 60);
    const secs = numeric % 60;
    return `${mins}m ${decimalFormatter.format(secs)}s`;
  }
  return `${decimalFormatter.format(numeric)}s`;
}

function resolvePath(obj, path) {
  return path.split(".").reduce((current, key) => {
    if (!current) {
      return undefined;
    }
    return current[key] !== undefined ? current[key] : undefined;
  }, obj);
}

function applyMetricsVisibility(metrics) {
  const container = document.querySelector(".stats-container");
  if (!container) {
    return;
  }

  let anyVisible = false;

  METRIC_GROUP_KEYS.forEach((key) => {
    const groupEl = container.querySelector(`[data-metric-group="${key}"]`);
    if (!groupEl) {
      return;
    }

    const groupData = metrics[key];
    const isVisible = !!(groupData && groupData.visible);
    groupEl.style.display = isVisible ? "" : "none";

    if (!isVisible) {
      return;
    }

    anyVisible = true;

    groupEl.querySelectorAll("[data-metric-field]").forEach((row) => {
      const fieldPath = row.getAttribute("data-metric-field") || "";
      const path = fieldPath.startsWith("metrics.") ? fieldPath.slice("metrics.".length) : fieldPath;
      const shouldShow = resolvePath(metrics, path);
      row.style.display = shouldShow === false ? "none" : "";
    });
  });

  const emptyState = container.querySelector("[data-metrics-empty]");
  if (emptyState) {
    emptyState.style.display = anyVisible ? "none" : "flex";
  }
}

async function ensureMetadataLoaded() {
  if (!window.metricsMetadata) {
    return;
  }

  try {
    await window.metricsMetadata.load();
    if (typeof window.metricsMetadata.refresh === "function") {
      window.metricsMetadata.refresh();
    } else {
      window.metricsMetadata.initializeInfoIcons();
    }
  } catch (error) {
    console.warn("Failed to initialise metrics metadata tooltips:", error);
  }
}

function updateActionButtons(jobBinding) {
  const restartBtn = document.getElementById("restartJobBtn");
  const cancelBtn = document.getElementById("cancelJobBtn");

  if (restartBtn) {
    restartBtn.style.display = jobBinding.can_restart ? "inline-flex" : "none";
  }
  if (cancelBtn) {
    cancelBtn.style.display = jobBinding.can_cancel ? "inline-flex" : "none";
  }
}

function updatePageTitle(title) {
  if (!title) {
    return;
  }
  document.title = `${title} · Blue Banded Bee`;
}

function formatJobForBinding(job, jobId) {
  const statusRaw = (job.status || "unknown").toString().toLowerCase();
  const statusLabel = statusRaw.replace(/_/g, " ").toUpperCase();
  const totalTasks = Number(job.total_tasks ?? job.totalTasks ?? 0);
  const completedTasks = Number(job.completed_tasks ?? job.completedTasks ?? 0);
  const failedTasks = Number(job.failed_tasks ?? job.failedTasks ?? 0);
  const skippedTasks = Number(job.skipped_tasks ?? job.skippedTasks ?? 0);

  let progress = Number(job.progress);
  if (!Number.isFinite(progress)) {
    const denominator = totalTasks - skippedTasks;
    progress = denominator > 0 ? ((completedTasks + failedTasks) / denominator) * 100 : 0;
  }
  const progressDisplay = `${Math.round(Math.max(0, Math.min(100, progress)))}%`;

  const avgSeconds = job.avg_time_per_task_seconds ?? job.avgTimePerTaskSeconds;
  const domain = job.domain ?? job.domains?.name ?? job.domain_name ?? "—";

  return {
    id: job.id || jobId,
    domain,
    page_title: domain && domain !== "—" ? `Job · ${domain}` : "Job Details",
    status_label: statusLabel,
    status_class: statusRaw,
    progress_display: progressDisplay,
    total_tasks_display: formatCount(totalTasks),
    completed_tasks_display: formatCount(completedTasks),
    failed_tasks_display: formatCount(failedTasks),
    started_at_display: formatDateTime(job.started_at ?? job.startedAt),
    completed_at_display: formatDateTime(job.completed_at ?? job.completedAt),
    duration_display: formatDuration(job.duration_seconds ?? job.durationSeconds),
    avg_time_display: formatAverageSeconds(avgSeconds),
    can_restart: ["completed", "failed", "cancelled"].includes(statusRaw),
    can_cancel: ["running", "pending"].includes(statusRaw),
  };
}

function formatMetricsForBinding(statsRaw = {}) {
  const cacheStats = statsRaw.cache_stats || {};
  const cacheVisible = hasNonNullValue(cacheStats);

  const warmingStats = statsRaw.cache_warming_effect || {};
  const warmingVisible = hasNonNullValue(warmingStats);

  const responseTimes = statsRaw.response_times || {};
  const performanceVisible = hasNonNullValue(responseTimes);

  const slowBuckets = statsRaw.slow_page_buckets || {};
  const distributionVisible = hasNonNullValue(slowBuckets);

  const taskSummary = statsRaw.task_summary || {};
  const reliabilityVisible =
    statsRaw.total_failed_pages != null ||
    statsRaw.total_server_errors != null ||
    slowBuckets.total_slow_over_3s != null ||
    taskSummary.with_errors != null;

  const discoverySources = statsRaw.discovery_sources || {};
  const discoveryVisible = hasNonNullValue(discoverySources);

  const redirectStats = statsRaw.redirect_stats || {};
  const redirectVisible = hasNonNullValue(redirectStats);

  return {
    cache: {
      visible: cacheVisible,
      hits: formatCount(cacheStats.hits ?? 0),
      misses: formatCount(cacheStats.misses ?? 0),
      bypass: formatCount(cacheStats.bypass ?? 0),
      bypass_visible: cacheStats.bypass != null,
      hit_rate: formatPercentage(cacheStats.hit_rate, { empty: "0%" }),
    },
    warming: {
      visible: warmingVisible,
      time_saved: formatSeconds(warmingStats.total_time_saved_seconds, { empty: "0s" }),
      avg_saved_per_page: formatMilliseconds(warmingStats.avg_time_saved_per_page_ms, { empty: "0ms" }),
      avg_second_request: formatMilliseconds(warmingStats.avg_second_request_ms, { empty: "0ms" }),
      avg_second_request_visible: warmingStats.avg_second_request_ms != null,
      validated: formatCount(warmingStats.total_validated ?? 0),
      validated_visible: warmingStats.total_validated != null,
      improved: formatCount(warmingStats.total_improved ?? 0),
      improved_visible: warmingStats.total_improved != null,
      improvement_rate: formatPercentage(warmingStats.improvement_rate, { empty: "0%" }),
    },
    performance: {
      visible: performanceVisible,
      avg: formatMilliseconds(responseTimes.avg_ms, { empty: "0ms" }),
      median: formatMilliseconds(responseTimes.median_ms, { empty: "0ms" }),
      p90: formatMilliseconds(responseTimes.p90_ms, { empty: "0ms" }),
      p90_visible: responseTimes.p90_ms != null,
      p95: formatMilliseconds(responseTimes.p95_ms, { empty: "0ms" }),
      p99: formatMilliseconds(responseTimes.p99_ms, { empty: "0ms" }),
      p99_visible: responseTimes.p99_ms != null,
      min: formatMilliseconds(responseTimes.min_ms, { empty: "0ms" }),
      min_visible: responseTimes.min_ms != null,
      max: formatMilliseconds(responseTimes.max_ms, { empty: "0ms" }),
      max_visible: responseTimes.max_ms != null,
    },
    distribution: {
      visible: distributionVisible,
      under_500ms: formatCount(slowBuckets.under_500ms ?? 0),
      under_500ms_visible: slowBuckets.under_500ms != null,
      _500ms_to_1s: formatCount(slowBuckets["500ms_to_1s"] ?? 0),
      _500ms_to_1s_visible: slowBuckets["500ms_to_1s"] != null,
      _1_to_1_5s: formatCount(slowBuckets["1_to_1_5s"] ?? 0),
      _1_to_1_5s_visible: slowBuckets["1_to_1_5s"] != null,
      _1_5_to_2s: formatCount(slowBuckets["1_5_to_2s"] ?? 0),
      _1_5_to_2s_visible: slowBuckets["1_5_to_2s"] != null,
      _2_to_3s: formatCount(slowBuckets["2_to_3s"] ?? 0),
      _2_to_3s_visible: slowBuckets["2_to_3s"] != null,
      _3_to_5s: formatCount(slowBuckets["3_to_5s"] ?? 0),
      _3_to_5s_visible: slowBuckets["3_to_5s"] != null,
      _5_to_10s: formatCount(slowBuckets["5_to_10s"] ?? 0),
      _5_to_10s_visible: slowBuckets["5_to_10s"] != null,
      over_10s: formatCount(slowBuckets.over_10s ?? 0),
      over_10s_visible: slowBuckets.over_10s != null,
    },
    reliability: {
      visible: reliabilityVisible,
      failed_pages: formatCount(statsRaw.total_failed_pages ?? 0),
      failed_pages_visible: statsRaw.total_failed_pages != null,
      server_errors: formatCount(statsRaw.total_server_errors ?? 0),
      server_errors_visible: statsRaw.total_server_errors != null,
      slow_over_3s: formatCount(slowBuckets.total_slow_over_3s ?? 0),
      slow_over_3s_visible: slowBuckets.total_slow_over_3s != null,
      tasks_with_errors: formatCount(taskSummary.with_errors ?? 0),
      tasks_with_errors_visible: taskSummary.with_errors != null,
    },
    discovery: {
      visible: discoveryVisible,
      sitemap: formatCount(discoverySources.sitemap ?? 0),
      sitemap_visible: discoverySources.sitemap != null,
      discovered: formatCount(discoverySources.discovered ?? 0),
      discovered_visible: discoverySources.discovered != null,
      manual: formatCount(discoverySources.manual ?? 0),
      manual_visible: discoverySources.manual != null,
      unique_sources: formatCount(discoverySources.unique_sources ?? 0),
      unique_sources_visible: discoverySources.unique_sources != null,
    },
    redirects: {
      visible: redirectVisible,
      total: formatCount(redirectStats.total ?? 0),
      total_visible: redirectStats.total != null,
      permanent: formatCount(redirectStats["301_permanent"] ?? 0),
      permanent_visible: redirectStats["301_permanent"] != null,
      temporary: formatCount(redirectStats["302_temporary"] ?? 0),
      temporary_visible: redirectStats["302_temporary"] != null,
    },
  };
}

function buildTaskUrl(task, defaultDomain) {
  if (task.url) {
    return task.url;
  }

  const host = task.domain || defaultDomain || "";
  const path = task.path || "/";

  if (!host || host === "—") {
    return path;
  }

  const safePath = path.startsWith("/") ? path : `/${path}`;
  if (host.startsWith("http://") || host.startsWith("https://")) {
    return `${host}${safePath}`;
  }

  return `https://${host}${safePath}`;
}

function formatTasksForBinding(tasks, defaultDomain) {
  return tasks.map((task) => {
    const statusRaw = (task.status || "unknown").toString().toLowerCase();
    return {
      path: task.path || "/",
      url: buildTaskUrl(task, defaultDomain),
      status: statusRaw,
      status_label: statusRaw.replace(/_/g, " ").toUpperCase(),
      response_time: formatMilliseconds(task.response_time, { empty: "—" }),
      cache_status: task.cache_status || "—",
      second_response_time: formatMilliseconds(task.second_response_time, { empty: "—" }),
      status_code: task.status_code != null ? String(task.status_code) : "—",
    };
  });
}

function renderTasksTable(tasks) {
  const table = document.getElementById("tasksTable");
  const tbody = document.getElementById("tasksTableBody");
  const emptyEl = document.getElementById("tasksEmpty");

  if (!table || !tbody) {
    return;
  }

  if (!tasks.length) {
    table.style.display = "none";
    if (emptyEl) {
      emptyEl.style.display = "block";
    }
    tbody.innerHTML = "";
    return;
  }

  table.style.display = "table";
  if (emptyEl) {
    emptyEl.style.display = "none";
  }

  const rowsHtml = tasks
    .map(
      (task) => `
        <tr>
          <td>
            <a href="${escapeHTML(task.url)}" target="_blank" rel="noopener noreferrer">
              <code>${escapeHTML(task.path)}</code>
            </a>
          </td>
          <td><span class="status-pill ${escapeHTML(task.status)}">${escapeHTML(task.status_label)}</span></td>
          <td>${escapeHTML(task.response_time)}</td>
          <td>${escapeHTML(task.cache_status)}</td>
          <td>${escapeHTML(task.second_response_time)}</td>
          <td>${escapeHTML(task.status_code)}</td>
        </tr>
      `,
    )
    .join("");

  tbody.innerHTML = rowsHtml;
}

function renderTaskHeader(state) {
  const table = document.getElementById("tasksTable");
  if (!table) {
    return;
  }

  const thead = table.querySelector("thead");
  if (!thead) {
    return;
  }

  const headers = [
    { key: "path", label: "Path" },
    { key: "status", label: "Status" },
    { key: "response_time", label: "Response Time (ms)" },
    { key: "cache_status", label: "Cache Status" },
    { key: "second_response_time", label: "2nd Response (ms)" },
    { key: "status_code", label: "Status Code" },
  ];

  const headerHtml = headers
    .map((header) => {
      const isActive = state.sortColumn === header.key;
      const icon = isActive ? (state.sortDirection === "desc" ? " ↓" : " ↑") : "";
      return `<th data-column="${header.key}">${header.label}${icon}</th>`;
    })
    .join("");

  thead.innerHTML = `<tr>${headerHtml}</tr>`;

  thead.querySelectorAll("th[data-column]").forEach((th) => {
    th.addEventListener("click", () => {
      const column = th.dataset.column;
      if (state.sortColumn === column) {
        state.sortDirection = state.sortDirection === "desc" ? "asc" : "desc";
      } else {
        state.sortColumn = column;
        state.sortDirection = "desc";
      }
      state.page = 0;
      loadTasks(state).catch((error) => {
        console.error("Failed to resort tasks:", error);
        showToast("Failed to resort tasks.", true);
      });
    });
  });
}

function updateTasksTableVisibility(count) {
  const loadingEl = document.getElementById("tasksLoading");
  const emptyEl = document.getElementById("tasksEmpty");
  const table = document.getElementById("tasksTable");

  if (loadingEl) {
    loadingEl.style.display = "none";
  }

  if (count === 0) {
    if (emptyEl) {
      emptyEl.style.display = "block";
    }
    if (table) {
      table.style.display = "none";
    }
  } else {
    if (emptyEl) {
      emptyEl.style.display = "none";
    }
    if (table) {
      table.style.display = "table";
    }
  }
}

function updatePagination(pagination, state) {
  const total = Number(pagination?.total ?? state.totalTasks ?? 0);
  const offset = Number(pagination?.offset ?? state.page * state.limit);
  const paginationEl = document.getElementById("tasksPagination");
  const start = total === 0 ? 0 : offset + 1;
  const end = total === 0 ? 0 : Math.min(offset + state.limit, total);
  const summary = total === 0 ? "0 tasks" : `${start}-${end} of ${formatCount(total)} tasks`;

  if (state.binder) {
    state.binder.updateElements({ tasks: { pagination: { summary } } });
  }

  if (!pagination || total <= state.limit) {
    if (paginationEl) {
      paginationEl.style.display = "none";
    }
    state.hasPrev = false;
    state.hasNext = false;
    state.totalTasks = total;
    return;
  }

  if (paginationEl) {
    paginationEl.style.display = "flex";
  }

  const hasNext = Boolean(pagination.has_next ?? offset + state.limit < total);
  const hasPrev = Boolean(pagination.has_prev ?? offset > 0);
  const prevBtn = document.getElementById("prevTasksBtn");
  const nextBtn = document.getElementById("nextTasksBtn");
  if (prevBtn) {
    prevBtn.disabled = !hasPrev;
  }
  if (nextBtn) {
    nextBtn.disabled = !hasNext;
  }

  state.hasPrev = hasPrev;
  state.hasNext = hasNext;
  state.totalTasks = total;
  state.page = total === 0 ? 0 : Math.floor(offset / state.limit);
}

function setupInteractions(state) {
  const limitSelect = document.getElementById("tasksLimit");
  if (limitSelect) {
    limitSelect.innerHTML = PAGE_SIZE_OPTIONS.map((value) => `<option value="${value}">${value}</option>`).join("");
    limitSelect.value = String(state.limit);
    limitSelect.addEventListener("change", (event) => {
      state.limit = Number(event.target.value) || DEFAULT_PAGE_SIZE;
      state.page = 0;
      loadTasks(state).catch((error) => {
        console.error("Failed to change page size:", error);
        showToast("Failed to update page size.", true);
      });
    });
  }

  const filterTabs = document.getElementById("taskFilters");
  if (filterTabs) {
    filterTabs.addEventListener("click", (event) => {
      const button = event.target.closest("button[data-status]");
      if (!button) {
        return;
      }

      event.preventDefault();
      filterTabs.querySelectorAll("button").forEach((btn) => btn.classList.remove("active"));
      button.classList.add("active");
      state.statusFilter = button.dataset.status || "";
      state.page = 0;
      loadTasks(state).catch((error) => {
        console.error("Failed to apply filter:", error);
        showToast("Failed to apply filter.", true);
      });
    });
  }

  const shareBtn = document.getElementById("shareJobBtn");
  if (shareBtn) {
    shareBtn.addEventListener("click", async () => {
      try {
        await navigator.clipboard.writeText(window.location.href);
        showToast("Link copied to clipboard.");
      } catch (error) {
        console.error("Clipboard copy failed:", error);
        showToast("Failed to copy link.", true);
      }
    });
  }

  const refreshJobBtn = document.getElementById("refreshJobBtn");
  if (refreshJobBtn) {
    refreshJobBtn.addEventListener("click", async () => {
      await loadJob(state);
      await loadTasks(state);
      showToast("Job data refreshed.");
    });
  }

  const refreshTasksBtn = document.getElementById("refreshTasksBtn");
  if (refreshTasksBtn) {
    refreshTasksBtn.addEventListener("click", async () => {
      await loadTasks(state);
      showToast("Task list refreshed.");
    });
  }

  const restartBtn = document.getElementById("restartJobBtn");
  if (restartBtn) {
    restartBtn.addEventListener("click", async () => {
      try {
        await restartJobFromPage(state);
      } catch (error) {
        console.error("Failed to restart job:", error);
        showToast("Failed to restart job.", true);
      }
    });
  }

  const cancelBtn = document.getElementById("cancelJobBtn");
  if (cancelBtn) {
    cancelBtn.addEventListener("click", async () => {
      try {
        await cancelJobFromPage(state);
      } catch (error) {
        console.error("Failed to cancel job:", error);
        showToast("Failed to cancel job.", true);
      }
    });
  }

  const prevBtn = document.getElementById("prevTasksBtn");
  if (prevBtn) {
    prevBtn.addEventListener("click", () => {
      if (!state.hasPrev) {
        return;
      }
      state.page = Math.max(0, state.page - 1);
      loadTasks(state).catch((error) => {
        console.error("Failed to load previous page:", error);
        showToast("Failed to load previous page.", true);
      });
    });
  }

  const nextBtn = document.getElementById("nextTasksBtn");
  if (nextBtn) {
    nextBtn.addEventListener("click", () => {
      if (!state.hasNext) {
        return;
      }
      state.page += 1;
      loadTasks(state).catch((error) => {
        console.error("Failed to load next page:", error);
        showToast("Failed to load next page.", true);
      });
    });
  }

  const exportToggle = document.getElementById("exportMenuToggle");
  const exportMenu = document.getElementById("exportMenu");
  if (exportToggle && exportMenu) {
    exportToggle.addEventListener("click", (event) => {
      event.stopPropagation();
      exportMenu.style.display = exportMenu.style.display === "block" ? "none" : "block";
    });

    document.querySelectorAll("#exportMenu button[data-type]").forEach((button) => {
      button.addEventListener("click", async () => {
        const type = button.getAttribute("data-type");
        const format = button.getAttribute("data-format") || "csv";
        exportMenu.style.display = "none";
        try {
          await exportJobData(state, { type, format });
          showToast("Export ready.");
        } catch (error) {
          console.error("Failed to export job data:", error);
          showToast("Failed to export data.", true);
        }
      });
    });

    document.addEventListener("click", (event) => {
      if (!event.target.closest("#exportMenu") && !event.target.closest("#exportMenuToggle")) {
        exportMenu.style.display = "none";
      }
    });
  }
}

async function initialiseAuth(state) {
  if (typeof window.initializeSupabase === "function") {
    window.initializeSupabase();
  }

  const { data, error } = await window.supabase.auth.getSession();
  if (error) {
    throw error;
  }

  if (!data || !data.session) {
    window.location.href = "/dashboard";
    return;
  }

  state.token = data.session.access_token;
  state.session = data.session;

  if (state.binder) {
    state.binder.authManager = {
      session: data.session,
      isAuthenticated: true,
      user: data.session.user,
    };
    state.binder.updateAuthElements();
  }

  const user = data.session.user;
  const userEmailEl = document.getElementById("userEmail");
  if (userEmailEl) {
    userEmailEl.textContent = user?.email || "Signed in";
  }

  const logoutBtn = document.getElementById("logoutBtn");
  if (logoutBtn) {
    logoutBtn.addEventListener("click", async () => {
      await window.supabase.auth.signOut();
      window.location.href = "/dashboard";
    });
  }
}

async function loadJob(state) {
  const job = await state.binder.fetchData(`/v1/jobs/${state.jobId}`);
  if (!job) {
    throw new Error("Job not found.");
  }

  const jobBinding = formatJobForBinding(job, state.jobId);
  const metricsBinding = formatMetricsForBinding(job.stats || {});
  state.domain = jobBinding.domain;

  state.binder.updateElements({ job: jobBinding, metrics: metricsBinding });

  applyMetricsVisibility(metricsBinding);
  updateActionButtons(jobBinding);
  updatePageTitle(jobBinding.page_title);
  await ensureMetadataLoaded();

  return job;
}

async function loadTasks(state) {
  const params = new URLSearchParams();
  params.set("limit", state.limit);
  params.set("offset", state.page * state.limit);
  params.set("sort", state.sortDirection === "desc" ? `-${state.sortColumn}` : state.sortColumn);
  if (state.statusFilter) {
    params.set("status", state.statusFilter);
  }

  const loadingEl = document.getElementById("tasksLoading");
  if (loadingEl && state.page === 0 && state.totalTasks === 0) {
    loadingEl.style.display = "block";
  }

  const data = await state.binder.fetchData(`/v1/jobs/${state.jobId}/tasks?${params.toString()}`);
  const tasks = Array.isArray(data?.tasks) ? data.tasks : [];
  const pagination = data?.pagination || {};

  renderTaskHeader(state);

  const formattedTasks = formatTasksForBinding(tasks, state.domain);
  renderTasksTable(formattedTasks);

  updateTasksTableVisibility(formattedTasks.length);
  updatePagination(pagination, state);

  if (loadingEl) {
    loadingEl.style.display = "none";
  }
}

async function authorisedFetch(state, path, options = {}) {
  const headers = new Headers(options.headers || {});
  headers.set("Authorization", `Bearer ${state.token}`);
  if (!headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  return fetch(path, {
    ...options,
    headers,
  });
}

async function restartJobFromPage(state) {
  const response = await authorisedFetch(state, `/v1/jobs/${state.jobId}/restart`, { method: "POST" });
  if (!response.ok) {
    throw new Error(`Failed to restart job (${response.status})`);
  }
  showToast("Restart requested. Refreshing…");
  await loadJob(state);
  await loadTasks(state);
}

async function cancelJobFromPage(state) {
  const response = await authorisedFetch(state, `/v1/jobs/${state.jobId}/cancel`, { method: "POST" });
  if (!response.ok) {
    throw new Error(`Failed to cancel job (${response.status})`);
  }
  showToast("Cancel requested. Refreshing…");
  await loadJob(state);
  await loadTasks(state);
}

async function exportJobData(state, { type, format }) {
  let url = `/v1/jobs/${state.jobId}/export`;
  if (type && type !== "job") {
    url += `?type=${encodeURIComponent(type)}`;
  }

  const response = await authorisedFetch(state, url, { headers: { Accept: "application/json" } });
  if (!response.ok) {
    throw new Error(`Export failed (${response.status})`);
  }

  const exportPayload = await response.json();
  const { payload, tasks, columns } = normaliseExportPayload(exportPayload);

  const { headers, keys } = prepareExportColumns(columns, tasks);

  const formattedRows = tasks.map((task) => {
    const row = {};
    keys.forEach((key) => {
      row[key] = task[key] ?? "";
    });
    return row;
  });

  if (format === "json") {
    const jsonContent = JSON.stringify(
      {
        meta: {
          job_id: payload?.job_id || state.jobId,
          export_time: payload?.export_time || new Date().toISOString(),
          export_type: payload?.export_type || type,
        },
        columns: headers,
        tasks: formattedRows,
      },
      null,
      2,
    );
    const filename = `${sanitizeForFilename(payload?.domain || state.domain || "job")}-${formatCompletionTimestampForFilename(
      payload?.completed_at,
      payload?.export_time,
    )}.json`;
    triggerFileDownload(jsonContent, "application/json", filename);
    return;
  }

  const csvRows = [headers.join(",")];
  formattedRows.forEach((row) => {
    const values = keys.map((key) => escapeCSVValue(row[key]));
    csvRows.push(values.join(","));
  });
  const csvContent = csvRows.join("\n");
  const filename = `${sanitizeForFilename(payload?.domain || state.domain || "job")}-${formatCompletionTimestampForFilename(
    payload?.completed_at,
    payload?.export_time,
  )}.csv`;
  triggerFileDownload(csvContent, "text/csv", filename);
}

function normaliseExportPayload(data) {
  let payload = data;
  if (payload && payload.data) {
    payload = payload.data;
  }

  let tasks = [];
  if (Array.isArray(payload?.tasks)) {
    tasks = payload.tasks;
  } else if (Array.isArray(payload)) {
    tasks = payload;
  }

  const columns = Array.isArray(payload?.columns) ? payload.columns : null;

  return { payload, tasks, columns };
}

function prepareExportColumns(columns, tasks) {
  if (Array.isArray(columns) && columns.length > 0) {
    const keys = columns.map((col) => col.key);
    const headers = columns.map((col) => col.label || formatColumnLabel(col.key));
    return { keys, headers };
  }

  const keySet = new Set();
  tasks.forEach((task) => {
    if (!task) return;
    Object.keys(task).forEach((key) => keySet.add(key));
  });

  const keys = Array.from(keySet);
  const headers = keys.map((key) => formatColumnLabel(key));
  return { keys, headers };
}

function formatColumnLabel(key) {
  if (!key) {
    return "";
  }

  const overrides = {
    id: "Task ID",
    job_id: "Job ID",
    url: "URL",
  };

  if (overrides[key]) {
    return overrides[key];
  }

  return key
    .replace(/_/g, " ")
    .split(" ")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function escapeCSVValue(value) {
  if (value === null || value === undefined) {
    return "";
  }

  const stringValue = String(value);
  if (stringValue.includes(",") || stringValue.includes('"') || stringValue.includes("\n")) {
    return `"${stringValue.replace(/"/g, '""')}"`;
  }

  return stringValue;
}

function formatCompletionTimestampForFilename(completedAt, fallback) {
  const parse = (val) => {
    if (!val) return null;
    const date = new Date(val);
    return Number.isNaN(date.getTime()) ? null : date;
  };

  const date = parse(completedAt) || parse(fallback) || new Date();
  const pad = (num) => String(num).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}-${pad(date.getHours())}-${pad(
    date.getMinutes(),
  )}`;
}

function sanitizeForFilename(value) {
  return (value || "")
    .toString()
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "") || "data";
}

function triggerFileDownload(content, mimeType, filename) {
  const blob = new Blob([content], { type: mimeType });
  const downloadUrl = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = downloadUrl;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(downloadUrl);
}

function showToast(message, isError = false) {
  const toast = document.createElement("div");
  toast.className = "toast";
  toast.style.background = isError ? "#b91c1c" : "#111827";
  toast.textContent = message;
  document.body.appendChild(toast);
  setTimeout(() => toast.remove(), 4000);
}

document.addEventListener("DOMContentLoaded", async () => {
  const jobId = window.location.pathname.split("/").filter(Boolean).pop();
  if (!jobId) {
    showToast("No job ID provided.", true);
    return;
  }

  const binder = new BBDataBinder({ apiBaseUrl: "" });
  window.dataBinder = binder;
  binder.scanAndBind();

  const state = {
    jobId,
    binder,
    limit: DEFAULT_PAGE_SIZE,
    page: 0,
    sortColumn: "created_at",
    sortDirection: "desc",
    statusFilter: "",
    totalTasks: 0,
    hasPrev: false,
    hasNext: false,
    domain: null,
    token: null,
  };

  try {
    await initialiseAuth(state);
    await loadJob(state);
    await loadTasks(state);
    setupInteractions(state);
  } catch (error) {
    console.error("Failed to initialise job page:", error);
    showToast("Failed to load job details.", true);
  }
});

/**
 * Standalone Job Details Page
 * Fetches job information directly from the API using Supabase auth tokens.
 */

const DEFAULT_PAGE_SIZE = 50;
const PAGE_SIZE_OPTIONS = [25, 50, 100, 200];

document.addEventListener("DOMContentLoaded", async () => {
  const jobId = window.location.pathname.split("/").filter(Boolean).pop();
  if (!jobId) {
    showToast("No job ID provided.", true);
    return;
  }

  const state = {
    jobId,
    token: null,
    domain: null,
    limit: DEFAULT_PAGE_SIZE,
    page: 0,
    sortColumn: "created_at",
    sortDirection: "desc",
    statusFilter: "",
    totalTasks: 0,
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
  const response = await authFetch(`/v1/jobs/${state.jobId}`, state.token);
  if (!response.ok) {
    if (response.status === 404) {
      throw new Error("Job not found. It may have been deleted.");
    }
    if (response.status === 403) {
      throw new Error("You do not have permission to view this job.");
    }
    throw new Error(`Failed to load job (${response.status})`);
  }

  const payload = await response.json();
  const job = payload.data ?? payload;

  state.domain = job.domains?.name || job.domain || job.domain_name || "—";

  const formatted = formatJobData(job);

  document.getElementById("pageTitle").textContent = `Job · ${state.domain}`;
  document.getElementById("jobDomain").textContent = state.domain;
  document.getElementById("jobId").textContent = state.jobId;
  document.getElementById("jobStatusValue").textContent = formatted.status || "—";
  document.getElementById("jobProgress").textContent = formatted.progress_formatted;
  document.getElementById("jobTotalTasks").textContent = Number(formatted.total_tasks ?? 0);
  document.getElementById("jobCompletedTasks").textContent = Number(formatted.completed_tasks ?? 0);
  document.getElementById("jobFailedTasks").textContent = Number(formatted.failed_tasks ?? 0);
  document.getElementById("jobStartedAt").textContent = formatted.started_at_formatted;
  document.getElementById("jobCompletedAt").textContent = formatted.completed_at_formatted;
  document.getElementById("jobDuration").textContent = formatted.duration_formatted;
  document.getElementById("jobAvgTime").textContent = formatted.avg_time_formatted;
  document.getElementById("jobStatusValue").className = `status-pill ${formatted.status}`;

  const restartBtn = document.getElementById("restartJobBtn");
  const cancelBtn = document.getElementById("cancelJobBtn");

  if (["completed", "failed", "cancelled"].includes(formatted.status)) {
    restartBtn.style.display = "inline-flex";
    restartBtn.onclick = () => restartJobFromPage(state);
    cancelBtn.style.display = "none";
  } else if (["running", "pending"].includes(formatted.status)) {
    cancelBtn.style.display = "inline-flex";
    cancelBtn.onclick = () => cancelJobFromPage(state);
    restartBtn.style.display = "none";
  } else {
    restartBtn.style.display = "none";
    cancelBtn.style.display = "none";
  }

  renderStats(job.stats || {});
}

function renderStats(stats) {
  const container = document.getElementById("statsContainer");
  if (!container) return;

  if (!stats || Object.keys(stats).length === 0) {
    container.innerHTML = `<div class="stat-row" style="grid-column: 1 / -1;">No statistics available yet.</div>`;
    return;
  }

  const groups = [];

  if (stats.cache_stats || stats.cache_warming_effect) {
    const parts = [];
    if (stats.cache_stats) {
      parts.push(renderStatRow("Cache Hits", stats.cache_stats.hits ?? 0));
      parts.push(renderStatRow("Cache Misses", stats.cache_stats.misses ?? 0));
      if (stats.cache_stats.hit_rate != null) {
        parts.push(renderStatRow("Cache Hit Rate", `${stats.cache_stats.hit_rate}%`));
      }
    }
    if (stats.cache_warming_effect) {
      parts.push(renderStatRow("Total Time Saved (s)", stats.cache_warming_effect.total_time_saved_seconds ?? 0));
      if (stats.cache_warming_effect.avg_time_saved_per_page_ms != null) {
        parts.push(
          renderStatRow("Avg Time Saved / Page (ms)", stats.cache_warming_effect.avg_time_saved_per_page_ms ?? 0),
        );
      }
      if (stats.cache_warming_effect.improvement_rate != null) {
        parts.push(renderStatRow("Improvement Rate", `${stats.cache_warming_effect.improvement_rate}%`));
      }
    }
    groups.push(renderStatsGroup("Cache Performance", parts));
  }

  if (stats.response_times) {
    const rt = stats.response_times;
    const parts = [
      renderStatRow("Average (ms)", rt.avg_ms ?? 0),
      renderStatRow("Median (ms)", rt.median_ms ?? 0),
      renderStatRow("P95 (ms)", rt.p95_ms ?? 0),
      renderStatRow("Min (ms)", rt.min_ms ?? 0),
      renderStatRow("Max (ms)", rt.max_ms ?? 0),
    ];
    groups.push(renderStatsGroup("Response Times", parts));
  }

  if (stats.total_failed_pages != null || stats.total_server_errors != null) {
    const parts = [
      renderStatRow("Failed Pages", stats.total_failed_pages ?? 0),
      renderStatRow("Server Errors", stats.total_server_errors ?? 0),
    ];
    groups.push(renderStatsGroup("Issues", parts));
  }

  if (stats.discovery_sources) {
    const ds = stats.discovery_sources;
    const parts = [
      renderStatRow("From Sitemap", ds.sitemap ?? 0),
      renderStatRow("Discovered", ds.discovered ?? 0),
      renderStatRow("Manual", ds.manual ?? 0),
    ];
    groups.push(renderStatsGroup("Discovery Sources", parts));
  }

  if (groups.length === 0) {
    container.innerHTML = `<div class="stat-row" style="grid-column: 1 / -1;">No statistics available yet.</div>`;
  } else {
    container.innerHTML = groups.join("");
  }

  if (window.metricsMetadata && !window.metricsMetadata.isLoaded()) {
    window.metricsMetadata.load().catch(() => {});
  }
  if (window.metricsMetadata && window.metricsMetadata.isLoaded()) {
    window.metricsMetadata.initializeInfoIcons();
  }
}

function renderStatsGroup(title, rows) {
  return `
    <div class="stats-group">
      <h3>${title}</h3>
      ${rows.join("")}
    </div>
  `;
}

function renderStatRow(label, value) {
  return `
    <div class="stat-row">
      <span>${label}</span>
      <strong>${value ?? 0}</strong>
    </div>
  `;
}

async function loadTasks(state) {
  const params = new URLSearchParams();
  params.set("limit", state.limit);
  params.set("offset", state.page * state.limit);
  params.set("sort", state.sortDirection === "desc" ? `-${state.sortColumn}` : state.sortColumn);
  if (state.statusFilter) {
    params.set("status", state.statusFilter);
  }

  const url = `/v1/jobs/${state.jobId}/tasks?${params.toString()}`;
  const response = await authFetch(url, state.token);
  if (!response.ok) {
    if (response.status === 404) {
      throw new Error("No tasks found.");
    }
    throw new Error(`Failed to load tasks (${response.status})`);
  }

  const payload = await response.json();
  const data = payload.data ?? payload;
  const tasks = data.tasks ?? [];
  state.totalTasks = data.pagination?.total ?? tasks.length;

  renderTasksTable(tasks, state);
  updatePagination(data.pagination ?? {}, state);
}

function renderTasksTable(tasks, state) {
  const table = document.getElementById("tasksTable");
  const loading = document.getElementById("tasksLoading");
  const tbody = table.querySelector("tbody");
  const thead = table.querySelector("thead");

  if (loading) loading.style.display = "none";
  table.style.display = tasks.length ? "table" : "none";

  if (!tasks.length) {
    const container = document.getElementById("tasksContainer");
    container.querySelector(
      "#tasksPagination",
    ).style.display = "none";
    container.querySelector("#tasksPageInfo").textContent = "";
    if (loading) {
      loading.style.display = "block";
      loading.textContent = "No tasks found for this view.";
    }
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

  thead.innerHTML = `
    <tr>
      ${headers
        .map((header) => {
          const isActive = state.sortColumn === header.key;
          const icon = isActive ? (state.sortDirection === "desc" ? " ↓" : " ↑") : "";
          return `<th data-column="${header.key}">${header.label}${icon}</th>`;
        })
        .join("")}
    </tr>
  `;

  tbody.innerHTML = tasks
    .map((task) => {
      return `
        <tr>
          <td><a href="${task.url}" target="_blank"><code>${task.path}</code></a></td>
          <td><span class="status-pill ${task.status}">${task.status}</span></td>
          <td>${task.response_time ?? "—"}</td>
          <td>${task.cache_status ?? "—"}</td>
          <td>${task.second_response_time ?? "—"}</td>
          <td>${task.status_code ?? "—"}</td>
        </tr>
      `;
    })
    .join("");

  thead.querySelectorAll("th[data-column]").forEach((th) => {
    th.addEventListener("click", () => {
      const column = th.dataset.column;
      if (state.sortColumn === column) {
        state.sortDirection = state.sortDirection === "desc" ? "asc" : "desc";
      } else {
        state.sortColumn = column;
        state.sortDirection = "desc";
      }
      loadTasks(state).catch((error) => {
        console.error("Failed to resort tasks:", error);
        showToast("Failed to resort tasks.", true);
      });
    });
  });
}

function updatePagination(pagination, state) {
  const paginationEl = document.getElementById("tasksPagination");
  const infoEl = document.getElementById("tasksPageInfo");
  const prevBtn = document.getElementById("prevTasksBtn");
  const nextBtn = document.getElementById("nextTasksBtn");

  if (!pagination || state.totalTasks <= state.limit) {
    paginationEl.style.display = "none";
    return;
  }

  paginationEl.style.display = "flex";

  const { total = state.totalTasks, has_next = false, has_prev = false, offset = state.page * state.limit } = pagination;
  const start = offset + 1;
  const end = Math.min(offset + state.limit, total);

  infoEl.textContent = `${start}-${end} of ${total} tasks`;
  prevBtn.disabled = !has_prev && state.page === 0;
  nextBtn.disabled = !has_next;
}

function setupInteractions(state) {
  const limitSelect = document.getElementById("tasksLimit");
  if (limitSelect) {
    limitSelect.innerHTML = PAGE_SIZE_OPTIONS.map((value) => {
      const selected = value === DEFAULT_PAGE_SIZE ? "selected" : "";
      return `<option value="${value}" ${selected}>${value}</option>`;
    }).join("");
    limitSelect.value = String(state.limit);
  }

  document.getElementById("shareJobBtn")?.addEventListener("click", async () => {
    try {
      await navigator.clipboard.writeText(window.location.href);
      showToast("Link copied to clipboard.");
    } catch (error) {
      console.error("Clipboard copy failed:", error);
      showToast("Failed to copy link.", true);
    }
  });

  document.getElementById("refreshJobBtn")?.addEventListener("click", async () => {
    await loadJob(state);
    await loadTasks(state);
    showToast("Job data refreshed.");
  });

  document.getElementById("refreshTasksBtn")?.addEventListener("click", async () => {
    await loadTasks(state);
    showToast("Task list refreshed.");
  });

  limitSelect?.addEventListener("change", (event) => {
    state.limit = Number(event.target.value);
    state.page = 0;
    loadTasks(state).catch((error) => {
      console.error("Failed to change limit:", error);
      showToast("Failed to change limit.", true);
    });
  });

  document.getElementById("taskFilters")?.addEventListener("click", (event) => {
    const button = event.target.closest("button[data-status]");
    if (!button) return;

    event.preventDefault();

    document.querySelectorAll("#taskFilters button").forEach((btn) => btn.classList.remove("active"));
    button.classList.add("active");

    state.statusFilter = button.dataset.status;
    state.page = 0;
    loadTasks(state).catch((error) => {
      console.error("Failed to filter tasks:", error);
      showToast("Failed to apply filter.", true);
    });
  });

  document.getElementById("prevTasksBtn")?.addEventListener("click", () => {
    if (state.page > 0) {
      state.page -= 1;
      loadTasks(state).catch((error) => {
        console.error("Failed to paginate:", error);
        showToast("Failed to load more tasks.", true);
      });
    }
  });

  document.getElementById("nextTasksBtn")?.addEventListener("click", () => {
    state.page += 1;
    loadTasks(state).catch((error) => {
      console.error("Failed to paginate:", error);
      showToast("Failed to load more tasks.", true);
    });
  });

  document.getElementById("exportMenuToggle")?.addEventListener("click", (event) => {
    event.stopPropagation();
    const menu = document.getElementById("exportMenu");
    if (menu) {
      menu.style.display = menu.style.display === "block" ? "none" : "block";
    }
  });

  document.querySelectorAll("#exportMenu button[data-type]").forEach((button) => {
    button.addEventListener("click", async () => {
      const type = button.dataset.type || "job";
      const format = button.dataset.format || "csv";
      const menu = document.getElementById("exportMenu");
      if (menu) menu.style.display = "none";

      try {
        await exportJobData(state, { type, format });
        showToast(`Exported ${type} (${format.toUpperCase()}).`);
      } catch (error) {
        console.error("Export failed:", error);
        showToast("Failed to export data.", true);
      }
    });
  });

  document.addEventListener("click", (event) => {
    if (!event.target.closest("#exportMenu") && !event.target.closest("#exportMenuToggle")) {
      const menu = document.getElementById("exportMenu");
      if (menu) menu.style.display = "none";
    }
  });
}

async function exportJobData(state, { type, format }) {
  let url = `/v1/jobs/${state.jobId}/export`;
  if (type && type !== "job") {
    url += `?type=${type}`;
  }

  const response = await authFetch(url, state.token);
  if (!response.ok) {
    throw new Error(`Export failed (${response.status})`);
  }

  const payload = await response.json();
  const data = payload.data ?? payload;
  const { rows, columns, filenameBase } = buildExportPayload(data, state, type);

  if (format === "json") {
    const jsonPayload = {
      columns,
      rows,
      metadata: {
        job_id: data.job_id || state.jobId,
        domain: data.domain || state.domain,
        status: data.status || null,
        completed_at: data.completed_at || null,
        created_at: data.created_at || null,
        export_time: data.export_time || new Date().toISOString(),
        export_type: data.export_type || type,
        total_rows: rows.length,
      },
    };

    triggerFileDownload(JSON.stringify(jsonPayload, null, 2), "application/json", `${filenameBase}.json`);
    return;
  }

  const csv = convertRowsToCSV(rows, columns);
  triggerFileDownload(csv, "text/csv", `${filenameBase}.csv`);
}

function buildExportPayload(data, state, type) {
  const { payload, tasks, columns } = normaliseExportPayload(data);
  const { keys, headers } = prepareExportColumns(columns, tasks);

  const filteredRows = tasks.map((task) => {
    const row = {};
    keys.forEach((key, index) => {
      row[headers[index]] = task && Object.prototype.hasOwnProperty.call(task, key) ? task[key] : null;
    });
    return row;
  });

  const domainForFilename = sanitizeForFilename(payload?.domain || state.domain || "domain");
  const dateStamp = formatCompletionTimestampForFilename(payload?.completed_at, payload?.export_time);
  const typeForFilename = sanitizeForFilename(type);
  const filenameBase = `${typeForFilename}-${domainForFilename}-${dateStamp}`;

  return {
    rows: filteredRows,
    columns: (columns && columns.length
      ? columns
      : keys.map((key, idx) => ({ key, label: headers[idx] ?? key }))),
    filenameBase,
  };
}

function convertRowsToCSV(rows, columns) {
  if (!rows.length) {
    return "";
  }

  const headers = columns.map((column) => column.label || column.key);
  const keys = columns.map((column) => column.key);

  const csvRows = [headers.map(escapeCSVValue).join(",")];

  rows.forEach((row) => {
    const line = keys.map((key, idx) => {
      const label = columns[idx].label || key;
      return escapeCSVValue(row[label]);
    });
    csvRows.push(line.join(","));
  });

  return csvRows.join("\n");
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

function formatJobData(job) {
  const formatted = {
    ...job,
    status: job.status || "unknown",
    progress_formatted: `${Math.round(job.progress || 0)}%`,
  };

  const startedAt = job.started_at ? new Date(job.started_at) : null;
  const completedAt = job.completed_at ? new Date(job.completed_at) : null;

  formatted.started_at_formatted = startedAt ? startedAt.toLocaleString() : "—";
  formatted.completed_at_formatted = completedAt ? completedAt.toLocaleString() : "—";

  if (job.duration_seconds != null) {
    const hours = Math.floor(job.duration_seconds / 3600);
    const minutes = Math.floor((job.duration_seconds % 3600) / 60);
    const seconds = job.duration_seconds % 60;

    formatted.duration_formatted =
      hours > 0 ? `${hours}h ${minutes}m ${seconds}s` : minutes > 0 ? `${minutes}m ${seconds}s` : `${seconds}s`;
  } else {
    formatted.duration_formatted = "—";
  }

  if (job.avg_time_per_task_seconds != null) {
    const avgSeconds = Number(job.avg_time_per_task_seconds);
    formatted.avg_time_formatted =
      avgSeconds >= 60
        ? `${Math.floor(avgSeconds / 60)}m ${(avgSeconds % 60).toFixed(2)}s`
        : `${avgSeconds.toFixed(2)}s`;
  } else {
    formatted.avg_time_formatted = "—";
  }

  return formatted;
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
  if (!key) return "";

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

function formatCompletionTimestampForFilename(completedAt, fallback) {
  const parse = (value) => {
    if (!value) return null;
    const date = new Date(value);
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
  const a = document.createElement("a");
  a.href = downloadUrl;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(downloadUrl);
}

async function authFetch(path, token, options = {}) {
  const headers = new Headers(options.headers || {});
  headers.set("Authorization", `Bearer ${token}`);
  headers.set("Content-Type", "application/json");

  return fetch(path, {
    ...options,
    headers,
  });
}

async function restartJobFromPage(state) {
  const response = await authFetch(`/v1/jobs/${state.jobId}/restart`, state.token, { method: "POST" });
  if (!response.ok) {
    throw new Error(`Failed to restart job (${response.status})`);
  }
  showToast("Restart requested. Refreshing…");
  await loadJob(state);
  await loadTasks(state);
}

async function cancelJobFromPage(state) {
  const response = await authFetch(`/v1/jobs/${state.jobId}/cancel`, state.token, { method: "POST" });
  if (!response.ok) {
    throw new Error(`Failed to cancel job (${response.status})`);
  }
  showToast("Cancel requested. Refreshing…");
  await loadJob(state);
  await loadTasks(state);
}

function showToast(message, isError = false) {
  const toast = document.createElement("div");
  toast.className = "toast";
  toast.style.background = isError ? "#b91c1c" : "#111827";
  toast.textContent = message;
  document.body.appendChild(toast);
  setTimeout(() => toast.remove(), 4000);
}

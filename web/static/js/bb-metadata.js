/**
 * Blue Banded Bee - Metrics Metadata System
 *
 * Provides metadata (descriptions, help text, links) for all dashboard metrics.
 * Fetches from /v1/metadata/metrics and caches locally for performance.
 */

class MetricsMetadata {
  constructor() {
    this.metadata = null;
    this.loading = false;
    this.loadPromise = null;
  }

  /**
   * Load metadata from the API (cached)
   */
  async load() {
    // Return cached data if already loaded
    if (this.metadata) {
      return this.metadata;
    }

    // Return existing promise if already loading
    if (this.loadPromise) {
      return this.loadPromise;
    }

    // Start loading
    this.loading = true;
    this.loadPromise = this._fetchMetadata();

    try {
      this.metadata = await this.loadPromise;
      return this.metadata;
    } finally {
      this.loading = false;
      this.loadPromise = null;
    }
  }

  /**
   * Fetch metadata from API
   */
  async _fetchMetadata() {
    try {
      const response = await window.dataBinder.fetchData("/v1/metadata/metrics");

      // fetchData already unwraps the data field
      if (response) {
        return response;
      }

      console.warn("Metadata response is empty:", response);
      return {};
    } catch (error) {
      console.error("Failed to load metrics metadata:", error);
      return {}; // Return empty object on error
    }
  }

  /**
   * Get info HTML for a specific metric
   */
  getInfo(metricKey) {
    if (!this.metadata || !this.metadata[metricKey]) {
      return null;
    }
    return this.metadata[metricKey].info_html;
  }

  /**
   * Get full metadata for a specific metric
   */
  getMetric(metricKey) {
    if (!this.metadata) {
      return null;
    }
    return this.metadata[metricKey];
  }

  /**
   * Check if metadata is loaded
   */
  isLoaded() {
    return this.metadata !== null;
  }

  /**
   * Initialize info icons on the page
   * Scans for elements with bbb-help or data-bb-info attributes and adds info icons with tooltips
   */
  initializeInfoIcons() {
    if (!this.isLoaded()) {
      console.warn("Metadata not loaded yet. Call load() first.");
      return;
    }

    // Find all elements with info attributes (both old and new formats)
    const elements = document.querySelectorAll("[data-bb-info], [bbb-help]");

    elements.forEach((element) => {
      // Support both old (data-bb-info) and new (bbb-help) formats
      const metricKey = element.getAttribute("bbb-help") || element.getAttribute("data-bb-info");
      const info = this.getInfo(metricKey);

      if (!info) {
        console.warn(`No metadata found for metric: ${metricKey}`);
        return;
      }

      // Check if info icon already exists
      if (element.querySelector(".bb-info-icon")) {
        return;
      }

      // Create info icon
      const infoIcon = document.createElement("span");
      infoIcon.className = "bb-info-icon";
      infoIcon.setAttribute("data-bbb-tooltip", info);
      infoIcon.setAttribute("aria-label", "More information");

      // Add click handler for mobile
      infoIcon.addEventListener("click", (e) => {
        e.stopPropagation();
        this._showTooltip(infoIcon);
      });

      // Append to element
      element.appendChild(infoIcon);
    });
  }

  /**
   * Show tooltip (for mobile/click interaction)
   */
  _showTooltip(iconElement) {
    // Remove any existing tooltips
    document.querySelectorAll(".bb-tooltip-popup").forEach((t) => t.remove());

    const tooltipContent = iconElement.getAttribute("data-bbb-tooltip");
    if (!tooltipContent) return;

    // Create tooltip popup
    const tooltip = document.createElement("div");
    tooltip.className = "bb-tooltip-popup";
    tooltip.innerHTML = tooltipContent;

    // Add close button
    const closeBtn = document.createElement("button");
    closeBtn.className = "bb-tooltip-close";
    closeBtn.innerHTML = "×";
    closeBtn.setAttribute("aria-label", "Close");
    closeBtn.addEventListener("click", () => tooltip.remove());
    tooltip.appendChild(closeBtn);

    // Position tooltip
    document.body.appendChild(tooltip);

    const iconRect = iconElement.getBoundingClientRect();
    const tooltipRect = tooltip.getBoundingClientRect();

    // Position below icon by default
    let top = iconRect.bottom + 8;
    let left = iconRect.left - tooltipRect.width / 2 + iconRect.width / 2;

    // Adjust if off-screen
    if (left + tooltipRect.width > window.innerWidth - 16) {
      left = window.innerWidth - tooltipRect.width - 16;
    }
    if (left < 16) {
      left = 16;
    }
    if (top + tooltipRect.height > window.innerHeight - 16) {
      // Position above instead
      top = iconRect.top - tooltipRect.height - 8;
    }

    tooltip.style.top = `${top}px`;
    tooltip.style.left = `${left}px`;

    // Close on click outside
    const closeOnClickOutside = (e) => {
      if (!tooltip.contains(e.target) && e.target !== iconElement) {
        tooltip.remove();
        document.removeEventListener("click", closeOnClickOutside);
      }
    };
    setTimeout(() => {
      document.addEventListener("click", closeOnClickOutside);
    }, 0);
  }

  /**
   * Refresh info icons (useful after dynamic content updates)
   */
  refresh() {
    if (this.isLoaded()) {
      this.initializeInfoIcons();
    }
  }
}

// Create global instance
window.metricsMetadata = new MetricsMetadata();

// Metadata will be loaded by dashboard after authentication
// No auto-initialization - auth is required for /v1/metadata/metrics endpoint

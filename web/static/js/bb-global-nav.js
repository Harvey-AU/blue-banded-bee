(function () {
  const container = document.getElementById("globalNav");
  if (!container) return;

  if (window.location.pathname.startsWith("/shared/jobs/")) {
    return;
  }

  let resolveNavReady = null;
  if (!window.BB_NAV_READY) {
    window.BB_NAV_READY = new Promise((resolve) => {
      resolveNavReady = resolve;
    });
  }

  fetch("/web/partials/global-nav.html")
    .then((response) => {
      if (!response.ok) {
        throw new Error("Failed to load global nav");
      }
      return response.text();
    })
    .then((html) => {
      container.innerHTML = html;
      const titleEl = container.querySelector("#globalNavTitle");
      const separatorEl = container.querySelector("#globalNavSeparator");
      const path = window.location.pathname.replace(/\/$/, "");
      const navLinks = container.querySelectorAll(".nav-link");

      const titleMap = [
        { match: (p) => p === "/dashboard", title: "Dashboard" },
        { match: (p) => p.startsWith("/settings"), title: "Settings" },
        { match: (p) => p.startsWith("/jobs/"), title: "Job Details" },
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
        } catch (err) {
          link.classList.remove("active");
        }
      });

      if (resolveNavReady) {
        resolveNavReady();
      }
      document.dispatchEvent(new CustomEvent("bb:nav-ready"));
    })
    .catch((error) => {
      console.warn("Global nav load failed:", error);
    });
})();

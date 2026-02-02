(function () {
  const container = document.getElementById("globalNav");
  if (!container) return;

  if (window.location.pathname.startsWith("/shared/jobs/")) {
    return;
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
      const path = window.location.pathname.replace(/\/$/, "");
      const navLinks = container.querySelectorAll(".nav-link");
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
    })
    .catch((error) => {
      console.warn("Global nav load failed:", error);
    });
})();

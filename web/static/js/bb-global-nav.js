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
    })
    .catch((error) => {
      console.warn("Global nav load failed:", error);
    });
})();

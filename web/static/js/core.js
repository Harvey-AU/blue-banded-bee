(function () {
  const loadedScripts = new Map();
  window.BB_APP = window.BB_APP || {};

  function loadScript(src, attrs = {}) {
    if (loadedScripts.has(src)) {
      return loadedScripts.get(src);
    }

    const existing = document.querySelector(`script[src="${src}"]`);
    if (existing) {
      if (
        existing.dataset.bbReady === "true" ||
        existing.dataset.bbLoader === "true" ||
        existing.getAttribute("data-bb-ready") === "true" ||
        existing.readyState === "complete"
      ) {
        const promise = Promise.resolve();
        loadedScripts.set(src, promise);
        return promise;
      }
      const promise = new Promise((resolve, reject) => {
        const onLoad = () => {
          existing.removeEventListener("load", onLoad);
          existing.removeEventListener("error", onError);
          resolve();
        };
        const onError = (err) => {
          existing.removeEventListener("load", onLoad);
          existing.removeEventListener("error", onError);
          reject(err);
        };
        existing.addEventListener("load", onLoad);
        existing.addEventListener("error", onError);
      });
      loadedScripts.set(src, promise);
      return promise;
    }

    const promise = new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = src;
      script.dataset.bbLoader = "true";
      Object.entries(attrs).forEach(([key, value]) => {
        if (value === undefined || value === null) return;
        script.setAttribute(key, value);
      });
      script.onload = () => {
        script.dataset.bbReady = "true";
        resolve();
      };
      script.onerror = (error) => reject(error);
      document.head.appendChild(script);
    });

    loadedScripts.set(src, promise);
    return promise;
  }

  async function ensureConfig() {
    if (window.BBB_CONFIG) {
      return;
    }
    try {
      await loadScript("/config.js");
    } catch (error) {
      throw new Error("Failed to load /config.js: " + error.message);
    }
    if (!window.BBB_CONFIG) {
      throw new Error("BBB_CONFIG missing after loading /config.js");
    }
  }

  function ensureSupabase() {
    const overrideSrc = window.BB_APP?.scripts?.supabase;
    const src =
      overrideSrc ||
      "https://unpkg.com/@supabase/supabase-js@2.80.0/dist/umd/supabase.js";
    const attrs = overrideSrc
      ? {}
      : {
          integrity:
            "sha384-i0m00Vn1ERlKXxNWSa87g6OUB7eLxpmsQoNF68IHuQVtfJTebIca7XhFsYt9h/gN",
          crossorigin: "anonymous",
        };
    return loadScript(src, attrs);
  }

  function ensurePasswordStrength() {
    const overrideSrc = window.BB_APP?.scripts?.passwordStrength;
    const src =
      overrideSrc || "https://cdn.jsdelivr.net/npm/zxcvbn@4.4.2/dist/zxcvbn.js";
    const attrs = overrideSrc
      ? {}
      : {
          integrity:
            "sha384-LXuP8lknSGBOLVn4fwVOl+rWR+zOEtZx6CF9ZLaN6gKBgLByU4D79VWWjV4/gefq",
          crossorigin: "anonymous",
        };
    return loadScript(src, attrs);
  }

  function ensureTurnstile() {
    const config = window.BBB_CONFIG || {};
    const shouldLoadTurnstile =
      window.BB_APP?.enableTurnstile ?? config.enableTurnstile ?? false;
    if (!shouldLoadTurnstile) {
      return Promise.resolve();
    }
    const overrideSrc = window.BB_APP?.scripts?.turnstile;
    const src =
      overrideSrc || "https://challenges.cloudflare.com/turnstile/v0/api.js";
    const attrs = overrideSrc
      ? { async: true, defer: true }
      : {
          crossorigin: "anonymous",
          async: true,
          defer: true,
        };
    return loadScript(src, attrs);
  }

  function ensureAuthBundle() {
    return loadScript("/js/auth.js");
  }

  async function initialise() {
    await ensureConfig();
    await ensureSupabase();
    await Promise.all([ensurePasswordStrength(), ensureTurnstile()]);
    await ensureAuthBundle();

    // Initialise Supabase client after loading SDK and auth bundle
    if (typeof window.BBAuth?.initialiseSupabase === "function") {
      const initialised = window.BBAuth.initialiseSupabase();
      if (!initialised) {
        console.error("Failed to initialise Supabase client");
      }
    }

    if (typeof window.BBAuth?.resumeCliAuthFromStorage === "function") {
      window.BBAuth.resumeCliAuthFromStorage();
    }

    if (window.BB_APP?.cliAuth && window.BBAuth?.initCliAuthPage) {
      window.BBAuth.initCliAuthPage();
      return;
    }

    if (typeof window.BBAuth?.setupAuthHandlers === "function") {
      window.BBAuth.setupAuthHandlers();
    }
  }

  const coreReady = (async () => {
    try {
      await initialise();
      window.BB_APP = window.BB_APP || {};
      window.BB_APP.coreReadyState = "ready";
    } catch (error) {
      window.BB_APP = window.BB_APP || {};
      window.BB_APP.coreReadyState = "error";
      console.error("Failed to initialise Blue Banded Bee core scripts", error);
      throw error;
    }
  })();

  window.BB_APP = window.BB_APP || {};
  window.BB_APP.coreReady = coreReady;

  // ========================================
  // Unified Organisation Initialisation
  // ========================================
  // Single source of truth for active organisation.
  // All code should await BB_ORG_READY before accessing BB_ACTIVE_ORG.

  let orgReadyResolve = null;
  let orgReadyReject = null;
  let orgInitialised = false;

  window.BB_ORG_READY = new Promise((resolve, reject) => {
    orgReadyResolve = resolve;
    orgReadyReject = reject;
  });

  /**
   * Initialise the active organisation. Called once after auth is confirmed.
   * Sets window.BB_ACTIVE_ORG and resolves BB_ORG_READY.
   * @returns {Promise<Object|null>} The active organisation or null
   */
  window.BB_APP.initialiseOrg = async function () {
    // Return cached result if we have a valid org
    if (
      orgInitialised &&
      window.BB_ACTIVE_ORG?.id &&
      window.BB_ACTIVE_ORG?.name
    ) {
      return window.BB_ACTIVE_ORG;
    }

    try {
      if (!window.supabase?.auth) {
        throw new Error("Supabase not initialised");
      }

      const { data: sessionData } = await window.supabase.auth.getSession();
      const session = sessionData?.session;
      if (!session) {
        // No session - don't mark as fully initialised so we can retry later
        window.BB_ACTIVE_ORG = null;
        window.BB_ORGANISATIONS = [];
        if (!orgInitialised) {
          orgReadyResolve(null);
          orgInitialised = true;
        }
        return null;
      }

      // Fetch organisations from API
      const response = await fetch("/v1/organisations", {
        headers: { Authorization: `Bearer ${session.access_token}` },
      });

      if (!response.ok) {
        throw new Error(`Failed to fetch organisations: ${response.status}`);
      }

      const data = await response.json();
      const organisations = data.data?.organisations || [];

      if (organisations.length === 0) {
        orgInitialised = true;
        window.BB_ACTIVE_ORG = null;
        window.BB_ORGANISATIONS = [];
        orgReadyResolve(null);
        return null;
      }

      // Get active org ID from users table
      let activeOrgId = null;
      try {
        const { data: userData } = await window.supabase
          .from("users")
          .select("active_organisation_id")
          .eq("id", session.user.id)
          .single();
        activeOrgId = userData?.active_organisation_id;
      } catch (err) {
        console.warn("Failed to fetch active_organisation_id:", err);
      }

      // Find active org in list, fall back to first
      const activeOrg =
        organisations.find((org) => org.id === activeOrgId) || organisations[0];

      // Set globals
      window.BB_ACTIVE_ORG = activeOrg;
      window.BB_ORGANISATIONS = organisations;
      orgInitialised = true;

      orgReadyResolve(activeOrg);
      return activeOrg;
    } catch (err) {
      console.error("Failed to initialise organisation:", err);
      orgInitialised = true;
      window.BB_ACTIVE_ORG = null;
      orgReadyReject(err);
      throw err;
    }
  };

  /**
   * Switch to a different organisation. Updates DB, globals, and notifies listeners.
   * @param {string} orgId - The organisation ID to switch to
   * @returns {Promise<Object>} The new active organisation
   */
  // Listen for auth state changes to re-init org when user signs in
  window.BB_APP.coreReady.then(() => {
    if (window.supabase?.auth) {
      window.supabase.auth.onAuthStateChange((event, session) => {
        if (event === "SIGNED_IN" || event === "TOKEN_REFRESHED") {
          // Re-init org if we don't have one yet
          if (!window.BB_ACTIVE_ORG?.id) {
            window.BB_APP.initialiseOrg()
              .then((org) => {
                if (org) {
                  document.dispatchEvent(
                    new CustomEvent("bb:org-ready", {
                      detail: { organisation: org },
                    })
                  );
                }
              })
              .catch((err) => {
                console.warn("Failed to init org after auth change:", err);
              });
          }
        }
      });
    }
  });

  window.BB_APP.switchOrg = async function (orgId) {
    if (!window.supabase?.auth) {
      throw new Error("Supabase not initialised");
    }

    const { data: sessionData } = await window.supabase.auth.getSession();
    const session = sessionData?.session;
    if (!session) {
      throw new Error("Not authenticated");
    }

    const response = await fetch("/v1/organisations/switch", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${session.access_token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ organisation_id: orgId }),
    });

    if (!response.ok) {
      const err = await response.json().catch(() => ({}));
      throw new Error(err.message || "Failed to switch organisation");
    }

    const switchData = await response.json();
    const newOrg = switchData.data?.organisation;

    // Update global
    window.BB_ACTIVE_ORG = newOrg;

    // Dispatch event for listeners
    document.dispatchEvent(
      new CustomEvent("bb:org-switched", { detail: { organisation: newOrg } })
    );

    return newOrg;
  };

  /**
   * Builds the payload for restarting a job with the same configuration.
   * @param {Object} job - The job object to extract config from
   * @returns {Object} Payload for POST /v1/jobs
   */
  window.BB_APP.buildRestartJobPayload = function (job) {
    return {
      domain: job.domain ?? job.domains?.name ?? job.domain_name,
      use_sitemap: true,
      find_links: job.find_links ?? true,
      concurrency: job.concurrency ?? 20,
      max_pages: job.max_pages ?? 0,
    };
  };

  coreReady.catch((err) => {
    if (typeof window !== "undefined" && window.console) {
      window.console.debug("coreReady rejected", err);
    }
  });

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => {
      coreReady.catch((err) => {
        console.error("Core initialization failed after DOMContentLoaded", err);
      });
    });
  }
})();

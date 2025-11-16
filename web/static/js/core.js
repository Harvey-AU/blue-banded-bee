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
        existing.dataset.bbLoader === "true"
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

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => {
      coreReady.catch((err) => {
        console.error("Core initialization failed after DOMContentLoaded", err);
      });
    });
  }
})();

let clickCount = 0;

const pingButton = document.getElementById("pingButton");
const counterText = document.getElementById("counterText");
const statusText = document.getElementById("statusText");
const apiBaseUrlInput = document.getElementById(
  "apiBaseUrl"
) as HTMLInputElement;
const apiTokenInput = document.getElementById("apiToken") as HTMLInputElement;
const connectAuthButton = document.getElementById("connectAuthButton");
const checkApiButton = document.getElementById("checkApiButton");
const apiStatusText = document.getElementById("apiStatusText");
const apiDetailsText = document.getElementById("apiDetailsText");

const API_BASE_STORAGE_KEY = "bbb_extension_api_base";
const API_TOKEN_STORAGE_KEY = "bbb_extension_api_token_session";
const AUTH_POPUP_WIDTH = 520;
const AUTH_POPUP_HEIGHT = 760;
const DEFAULT_BBB_APP_ORIGIN = "https://app.bluebandedbee.co";

if (pingButton && counterText && statusText) {
  pingButton.addEventListener("click", () => {
    clickCount += 1;
    counterText.textContent = `Clicks: ${clickCount}`;
    statusText.textContent = `Interaction working (${new Date().toLocaleTimeString()}).`;
  });
}

hydrateApiInputs();

if (checkApiButton && apiStatusText && apiDetailsText) {
  checkApiButton.addEventListener("click", () => {
    void runApiCheck();
  });
}

if (connectAuthButton && apiStatusText && apiDetailsText) {
  connectAuthButton.addEventListener("click", () => {
    void connectAccount();
  });
}

function hydrateApiInputs() {
  const savedBaseUrl = window.localStorage.getItem(API_BASE_STORAGE_KEY);
  const savedToken = window.sessionStorage.getItem(API_TOKEN_STORAGE_KEY);

  if (apiBaseUrlInput) {
    apiBaseUrlInput.value = savedBaseUrl || "https://app.bluebandedbee.co";
  }
  if (apiTokenInput && savedToken) {
    apiTokenInput.value = savedToken;
  }
}

function getHeaders(token: string): HeadersInit {
  const headers: HeadersInit = {
    Accept: "application/json",
  };

  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  return headers;
}

async function runApiCheck() {
  if (
    !apiBaseUrlInput ||
    !apiTokenInput ||
    !checkApiButton ||
    !apiStatusText ||
    !apiDetailsText
  ) {
    return;
  }

  const baseUrl = apiBaseUrlInput.value.trim().replace(/\/$/, "");
  const token = apiTokenInput.value.trim();

  window.localStorage.setItem(API_BASE_STORAGE_KEY, baseUrl);
  if (token) {
    window.sessionStorage.setItem(API_TOKEN_STORAGE_KEY, token);
  } else {
    window.sessionStorage.removeItem(API_TOKEN_STORAGE_KEY);
  }

  if (!baseUrl) {
    apiStatusText.textContent = "Enter API base URL first.";
    apiDetailsText.textContent = "";
    return;
  }

  checkApiButton.setAttribute("disabled", "true");
  apiStatusText.textContent = "Checking API...";
  apiDetailsText.textContent = "";

  try {
    const healthResponse = await fetch(`${baseUrl}/health`, {
      method: "GET",
      headers: getHeaders(token),
    });

    if (!healthResponse.ok) {
      apiStatusText.textContent = `Health check failed (${healthResponse.status}).`;
      apiDetailsText.textContent = "Confirm API URL and CORS configuration.";
      return;
    }

    const integrationsResponse = await fetch(
      `${baseUrl}/v1/integrations/webflow`,
      {
        method: "GET",
        headers: getHeaders(token),
      }
    );

    if (
      integrationsResponse.status === 401 ||
      integrationsResponse.status === 403
    ) {
      apiStatusText.textContent =
        "API reachable, auth required for integrations.";
      apiDetailsText.textContent =
        "Paste a valid bearer token to read /v1/integrations/webflow.";
      return;
    }

    if (!integrationsResponse.ok) {
      apiStatusText.textContent = `Integrations read failed (${integrationsResponse.status}).`;
      apiDetailsText.textContent =
        "Health endpoint works, but integrations endpoint returned an error.";
      return;
    }

    const payload = (await integrationsResponse.json()) as {
      data?: Array<unknown>;
    };
    const count = Array.isArray(payload.data) ? payload.data.length : 0;

    apiStatusText.textContent = "API reachable and integrations fetched.";
    apiDetailsText.textContent = `Webflow connections: ${count}`;
  } catch (error) {
    const message = error instanceof Error ? error.message : "Unknown error";
    apiStatusText.textContent = "Request failed.";
    apiDetailsText.textContent = message;
  } finally {
    checkApiButton.removeAttribute("disabled");
  }
}

function createAuthState() {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

function getPopupPosition() {
  const left =
    window.screenX + Math.max(0, (window.outerWidth - AUTH_POPUP_WIDTH) / 2);
  const top =
    window.screenY + Math.max(0, (window.outerHeight - AUTH_POPUP_HEIGHT) / 2);
  return { left: Math.floor(left), top: Math.floor(top) };
}

async function connectAccount() {
  if (
    !apiBaseUrlInput ||
    !apiTokenInput ||
    !connectAuthButton ||
    !apiStatusText ||
    !apiDetailsText
  ) {
    return;
  }

  const baseUrl = apiBaseUrlInput.value.trim().replace(/\/$/, "");
  if (!baseUrl) {
    apiStatusText.textContent = "Enter API base URL first.";
    apiDetailsText.textContent = "";
    return;
  }

  let baseOrigin = "";
  try {
    baseOrigin = new URL(baseUrl).origin;
  } catch (_error) {
    apiStatusText.textContent = "Invalid API base URL.";
    apiDetailsText.textContent =
      "Use full URL like https://app.bluebandedbee.co";
    return;
  }

  const state = createAuthState();
  const authBaseOrigin =
    baseOrigin.includes("localhost") || baseOrigin.includes("127.0.0.1")
      ? baseOrigin
      : DEFAULT_BBB_APP_ORIGIN;
  const authUrl = new URL(`${authBaseOrigin}/extension-auth.html`);
  authUrl.searchParams.set("origin", window.location.origin);
  authUrl.searchParams.set("state", state);

  const popupPosition = getPopupPosition();
  const popupFeatures =
    `width=${AUTH_POPUP_WIDTH},height=${AUTH_POPUP_HEIGHT},` +
    `left=${popupPosition.left},top=${popupPosition.top},resizable=yes,scrollbars=yes`;

  connectAuthButton.setAttribute("disabled", "true");
  apiStatusText.textContent = "Opening sign-in window...";
  apiDetailsText.textContent = "";

  const popup = window.open(
    authUrl.toString(),
    "bbbExtensionAuth",
    popupFeatures
  );
  if (!popup) {
    connectAuthButton.removeAttribute("disabled");
    apiStatusText.textContent = "Popup blocked.";
    apiDetailsText.textContent =
      "Allow popups for Webflow Designer and try again.";
    return;
  }

  await new Promise<void>((resolve) => {
    let settled = false;

    const cleanup = () => {
      if (settled) return;
      settled = true;
      window.removeEventListener("message", onMessage);
      window.clearInterval(closedPoll);
      connectAuthButton.removeAttribute("disabled");
      resolve();
    };

    const onMessage = (event: MessageEvent) => {
      if (event.origin !== authBaseOrigin) {
        return;
      }

      const payload = (event.data || {}) as {
        source?: string;
        type?: string;
        state?: string;
        accessToken?: string;
        message?: string;
      };

      if (payload.source !== "bbb-extension-auth" || payload.state !== state) {
        return;
      }

      if (payload.type === "success" && payload.accessToken) {
        apiTokenInput.value = payload.accessToken;
        window.sessionStorage.setItem(
          API_TOKEN_STORAGE_KEY,
          payload.accessToken
        );
        apiStatusText.textContent =
          "Authenticated. Token stored for this session.";
        apiDetailsText.textContent =
          "Click Check API to verify backend access.";
      } else {
        apiStatusText.textContent = "Authentication failed.";
        apiDetailsText.textContent = payload.message || "Please try again.";
      }

      cleanup();
    };

    const closedPoll = window.setInterval(() => {
      if (popup.closed) {
        apiStatusText.textContent = "Sign-in window closed.";
        if (!apiDetailsText.textContent) {
          apiDetailsText.textContent = "If sign-in completed, try Check API.";
        }
        cleanup();
      }
    }, 500);

    window.addEventListener("message", onMessage);
  });
}

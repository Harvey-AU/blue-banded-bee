(function () {
  let authListenerAttached = false;

  function getInviteToken(paramName = "invite_token") {
    return new URLSearchParams(window.location.search).get(paramName);
  }

  function clearInviteTokenFromURL(paramName = "invite_token") {
    const params = new URLSearchParams(window.location.search);
    if (!params.has(paramName)) return;

    params.delete(paramName);
    const url = new URL(window.location.href);
    url.search = params.toString();
    window.history.replaceState({}, "", url.toString());
  }

  async function fetchInvitePreview(token) {
    const response = await fetch(
      `/v1/organisations/invites/preview?token=${encodeURIComponent(token)}`,
      {
        method: "GET",
        headers: { "Content-Type": "application/json" },
      }
    );

    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      const message = payload?.message || "Failed to load invite details";
      throw new Error(message);
    }

    return payload?.data?.invite || null;
  }

  async function acceptInvite(token) {
    if (window.dataBinder?.fetchData) {
      return window.dataBinder.fetchData("/v1/organisations/invites/accept", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token }),
      });
    }

    const sessionResult = await window.supabase.auth.getSession();
    const session = sessionResult?.data?.session;
    if (!session?.access_token) {
      throw new Error("Authentication is required");
    }

    const response = await fetch("/v1/organisations/invites/accept", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${session.access_token}`,
      },
      body: JSON.stringify({ token }),
    });

    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      const message = payload?.message || "Failed to accept invite";
      throw new Error(message);
    }

    return payload?.data || payload;
  }

  function ensureAuthModalAndReloadOnSignIn() {
    if (typeof window.showAuthModal === "function") {
      window.showAuthModal();
    }

    if (authListenerAttached || !window.supabase?.auth) {
      return;
    }

    authListenerAttached = true;
    window.supabase.auth.onAuthStateChange((event) => {
      if (event === "SIGNED_IN") {
        window.location.reload();
      }
    });
  }

  async function handleInviteTokenFlow(options = {}) {
    const {
      tokenParamName = "invite_token",
      clearTokenOnSuccess = true,
      redirectTo = "",
      onAuthRequired,
      onAccepted,
      onError,
    } = options;

    const token = getInviteToken(tokenParamName);
    if (!token) {
      return { status: "no_token" };
    }

    const sessionResult = await window.supabase.auth.getSession();
    const session = sessionResult?.data?.session;
    if (!session?.user) {
      ensureAuthModalAndReloadOnSignIn();
      if (typeof onAuthRequired === "function") {
        onAuthRequired(token);
      }
      return { status: "auth_required", token };
    }

    try {
      const result = await acceptInvite(token);

      if (clearTokenOnSuccess) {
        clearInviteTokenFromURL(tokenParamName);
      }

      if (typeof onAccepted === "function") {
        await onAccepted(result);
      }

      if (redirectTo) {
        window.location.assign(redirectTo);
      }

      return { status: "accepted", token, result };
    } catch (error) {
      if (typeof onError === "function") {
        onError(error);
      }
      return { status: "error", token, error };
    }
  }

  window.BBInviteFlow = {
    getInviteToken,
    clearInviteTokenFromURL,
    fetchInvitePreview,
    handleInviteTokenFlow,
  };
})();

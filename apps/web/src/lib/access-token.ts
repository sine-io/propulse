const accessTokenKey = "propulse.accessToken";
const accessTokenChangeEvent = "propulse:access-token-change";

export function getAccessToken(): string | undefined {
  if (typeof window === "undefined") {
    return undefined;
  }
  return window.sessionStorage.getItem(accessTokenKey)?.trim() || undefined;
}

export function setAccessToken(token: string): void {
  if (typeof window === "undefined") {
    return;
  }
  window.sessionStorage.setItem(accessTokenKey, token.trim());
  window.dispatchEvent(new Event(accessTokenChangeEvent));
}

export function clearAccessToken(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.sessionStorage.removeItem(accessTokenKey);
  window.dispatchEvent(new Event(accessTokenChangeEvent));
}

export function subscribeToAccessToken(listener: () => void): () => void {
  if (typeof window === "undefined") {
    return () => undefined;
  }
  window.addEventListener(accessTokenChangeEvent, listener);
  window.addEventListener("storage", listener);
  return () => {
    window.removeEventListener(accessTokenChangeEvent, listener);
    window.removeEventListener("storage", listener);
  };
}

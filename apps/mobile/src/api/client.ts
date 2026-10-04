import {
  ACCOUNT_HEADER,
  type AuthTokens,
  createStayKeyClient,
  type Middleware,
} from "@staykey/api-client";
import { useSession } from "@/lib/session";
import { clearTokens, expiresSoon, loadTokens, saveTokens } from "@/lib/tokens";

export const API_URL = process.env.EXPO_PUBLIC_API_URL ?? "http://localhost:8080";

/** Talks to the auth routes, which never carry a token. */
const bare = createStayKeyClient(API_URL);

/**
 * The client every screen uses. It adds the access token and the active account to each
 * request, refreshes the token shortly before it expires, and on a 401 refreshes once and
 * retries. When the server refuses the refresh token, the session ends and the app returns to
 * Welcome.
 */
export const api = createStayKeyClient(API_URL);

const isAuthRoute = (schemaPath: string) => schemaPath.startsWith("/v1/auth/");

let refreshing: Promise<AuthTokens | null> | null = null;

/**
 * Exchanges the refresh token, once at a time: concurrent callers share the same request, so a
 * burst of 401s never presents a rotated token twice.
 */
export function refreshTokens(): Promise<AuthTokens | null> {
  refreshing ??= (async () => {
    try {
      const current = await loadTokens();
      if (!current) return null;
      const { data, response } = await bare.POST("/v1/auth/refresh", {
        body: { refreshToken: current.refreshToken },
      });
      if (data) {
        await saveTokens(data);
        return data;
      }
      if (response.status === 401) await endSession();
      return null;
    } catch {
      // Offline or the API is down: keep the session and let the caller fail.
      return null;
    } finally {
      refreshing = null;
    }
  })();
  return refreshing;
}

/** Called when the server no longer accepts the session. */
async function endSession() {
  await clearTokens();
  useSession.getState().signedOut();
  onSessionEnded?.();
}

let onSessionEnded: (() => void) | undefined;

/** Registers cleanup (clearing cached queries) for when a session ends. */
export function setOnSessionEnded(fn: () => void) {
  onSessionEnded = fn;
}

// Requests waiting on their response, kept so a 401 can be retried with a fresh token.
const pending = new Map<string, Request>();

const auth: Middleware = {
  async onRequest({ request, schemaPath, id }) {
    if (isAuthRoute(schemaPath)) return request;
    let tokens = await loadTokens();
    if (tokens && expiresSoon(tokens)) tokens = (await refreshTokens()) ?? tokens;
    if (tokens) request.headers.set("Authorization", `Bearer ${tokens.accessToken}`);
    const { accountId } = useSession.getState();
    if (accountId) request.headers.set(ACCOUNT_HEADER, accountId);
    pending.set(id, request.clone());
    return request;
  },

  async onResponse({ response, schemaPath, id }) {
    const original = pending.get(id);
    pending.delete(id);
    if (response.status !== 401 || isAuthRoute(schemaPath) || !original) return response;

    const fresh = await refreshTokens();
    if (!fresh) return response;
    const retry = new Request(original);
    retry.headers.set("Authorization", `Bearer ${fresh.accessToken}`);
    return fetch(retry);
  },

  onError({ id }) {
    pending.delete(id);
  },
};

api.use(auth);

export { bare as authApi };

import { beforeEach, describe, expect, mock, test } from "bun:test";
import type { AuthTokens } from "@staykey/api-client";

// The client reads tokens and the active account from these modules; stand-ins keep the test
// away from the keychain and AsyncStorage.
let stored: AuthTokens | null = null;
const signedOut = mock(() => {});
mock.module("@/lib/tokens", () => ({
  loadTokens: async () => stored,
  saveTokens: async (t: AuthTokens) => {
    stored = t;
  },
  clearTokens: async () => {
    stored = null;
  },
  expiresSoon: (t: AuthTokens) => new Date(t.accessTokenExpiresAt).getTime() - Date.now() < 30_000,
}));
mock.module("@/lib/session", () => ({
  useSession: { getState: () => ({ accountId: "acc-1", signedOut }) },
}));

type Handler = (req: Request) => Response | Promise<Response>;
let handler: Handler = () => new Response(null, { status: 500 });
const seen: Request[] = [];
// openapi-fetch keeps the fetch it was created with, so install a dispatcher before importing.
globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
  const req = input instanceof Request ? input : new Request(input, init);
  seen.push(req.clone());
  return handler(req);
}) as typeof fetch;

const { api } = await import("../client");

const later = (mins: number) => new Date(Date.now() + mins * 60_000).toISOString();
const tokens = (access: string, refresh: string): AuthTokens => ({
  accessToken: access,
  accessTokenExpiresAt: later(15),
  refreshToken: refresh,
  refreshTokenExpiresAt: later(60 * 24),
});
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });
const me = { user: { id: "u1" }, accounts: [] };

beforeEach(() => {
  stored = tokens("access-1", "refresh-1");
  seen.length = 0;
  signedOut.mockClear();
});

describe("api client", () => {
  test("sends the access token and the active account", async () => {
    handler = () => json(me);
    const { data } = await api.GET("/v1/me");
    expect(data).toEqual(me as never);
    expect(seen[0]?.headers.get("Authorization")).toBe("Bearer access-1");
    expect(seen[0]?.headers.get("X-Account-Id")).toBe("acc-1");
  });

  test("leaves auth routes without a token", async () => {
    handler = () => json({ phone: "+94771234567", expiresAt: later(5), resendAfter: 30 });
    await api.POST("/v1/auth/otp", { body: { phone: "+94771234567" } });
    expect(seen[0]?.headers.get("Authorization")).toBeNull();
  });

  test("refreshes once on a 401 and retries with the new token", async () => {
    handler = async (req) => {
      if (req.url.endsWith("/v1/auth/refresh")) return json(tokens("access-2", "refresh-2"));
      return req.headers.get("Authorization") === "Bearer access-2"
        ? json(me)
        : json({ code: "unauthorized", message: "" }, 401);
    };
    const { data, response } = await api.GET("/v1/me");
    expect(response.status).toBe(200);
    expect(data).toEqual(me as never);
    expect(stored?.refreshToken).toBe("refresh-2");
  });

  test("shares one refresh between concurrent 401s", async () => {
    let refreshes = 0;
    handler = async (req) => {
      if (req.url.endsWith("/v1/auth/refresh")) {
        refreshes++;
        await new Promise((r) => setTimeout(r, 10));
        return json(tokens("access-2", "refresh-2"));
      }
      return req.headers.get("Authorization") === "Bearer access-2"
        ? json(me)
        : json({ code: "unauthorized", message: "" }, 401);
    };
    const results = await Promise.all([api.GET("/v1/me"), api.GET("/v1/me"), api.GET("/v1/me")]);
    expect(results.map((r) => r.response.status)).toEqual([200, 200, 200]);
    expect(refreshes).toBe(1);
  });

  test("ends the session when the refresh token is refused", async () => {
    handler = (req) =>
      req.url.endsWith("/v1/auth/refresh")
        ? json({ code: "invalid_refresh", message: "" }, 401)
        : json({ code: "unauthorized", message: "" }, 401);
    const { response } = await api.GET("/v1/me");
    expect(response.status).toBe(401);
    expect(stored).toBeNull();
    expect(signedOut).toHaveBeenCalledTimes(1);
  });

  test("keeps the session when the refresh can't reach the server", async () => {
    handler = (req) => {
      if (req.url.endsWith("/v1/auth/refresh")) throw new TypeError("Network request failed");
      return json({ code: "unauthorized", message: "" }, 401);
    };
    const { response } = await api.GET("/v1/me");
    expect(response.status).toBe(401);
    expect(stored?.refreshToken).toBe("refresh-1");
    expect(signedOut).not.toHaveBeenCalled();
  });

  test("refreshes ahead of time when the access token is about to expire", async () => {
    stored = { ...tokens("access-1", "refresh-1"), accessTokenExpiresAt: later(0.1) };
    handler = (req) =>
      req.url.endsWith("/v1/auth/refresh") ? json(tokens("access-2", "refresh-2")) : json(me);
    await api.GET("/v1/me");
    const call = seen.find((r) => r.url.endsWith("/v1/me"));
    expect(call?.headers.get("Authorization")).toBe("Bearer access-2");
  });
});

import { type NextRequest, NextResponse } from "next/server";
import { ACCESS_COOKIE, cookieOptions, REFRESH_COOKIE } from "@/lib/session";

type Tokens = {
  accessToken: string;
  accessTokenExpiresAt: string;
  refreshToken: string;
  refreshTokenExpiresAt: string;
};

/**
 * Keeps the billing page signed in. The access cookie lasts as long as the access token (15
 * minutes); once it is gone, the refresh token is exchanged here, before the page renders,
 * since pages can read cookies but only a proxy or a server action can set them.
 */
export async function proxy(request: NextRequest) {
  const refresh = request.cookies.get(REFRESH_COOKIE)?.value;
  if (request.cookies.has(ACCESS_COOKIE) || !refresh) return NextResponse.next();

  const api = process.env.STAYKEY_API_URL ?? "http://localhost:8080";
  const res = await fetch(`${api}/v1/auth/refresh`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refreshToken: refresh }),
  }).catch(() => undefined);

  if (!res?.ok) {
    // An expired or revoked session signs the owner out; a network failure leaves it for the
    // next request.
    if (res?.status !== 401) return NextResponse.next();
    request.cookies.delete(REFRESH_COOKIE);
    const out = NextResponse.next({ request: { headers: request.headers } });
    out.cookies.delete({ name: REFRESH_COOKIE, path: "/billing" });
    return out;
  }

  const tokens = (await res.json()) as Tokens;
  request.cookies.set(ACCESS_COOKIE, tokens.accessToken);
  request.cookies.set(REFRESH_COOKIE, tokens.refreshToken);
  const out = NextResponse.next({ request: { headers: request.headers } });
  out.cookies.set(
    ACCESS_COOKIE,
    tokens.accessToken,
    cookieOptions(new Date(tokens.accessTokenExpiresAt)),
  );
  out.cookies.set(
    REFRESH_COOKIE,
    tokens.refreshToken,
    cookieOptions(new Date(tokens.refreshTokenExpiresAt)),
  );
  return out;
}

export const config = {
  matcher: ["/billing", "/billing/:path*"],
};

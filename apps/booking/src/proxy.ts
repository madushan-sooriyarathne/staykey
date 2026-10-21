import { type NextRequest, NextResponse } from "next/server";
import { slugFromHost } from "@/lib/tenant";

/**
 * Maps property subdomains onto the /p/[slug] routes:
 *   kingfisher.staykey.direct/        ->  /p/kingfisher
 *   kingfisher.staykey.direct/embed   ->  /p/kingfisher/embed
 * Requests to the root domain, and the app's own routes on any host, pass through unchanged.
 */
export function proxy(request: NextRequest) {
  const slug = slugFromHost(request.headers.get("host"));
  const { pathname, search } = request.nextUrl;
  if (!slug || isAppRoute(pathname)) return NextResponse.next();

  const target = request.nextUrl.clone();
  target.pathname = `/p/${slug}${pathname === "/" ? "" : pathname}`;
  target.search = search;
  return NextResponse.rewrite(target);
}

// The Sentry tunnel, the health check and the ops test route answer on every host, since the
// browser posts errors to the subdomain it is on.
const APP_ROUTES = ["/monitoring", "/healthz", "/internal/test-error"];

function isAppRoute(pathname: string): boolean {
  return APP_ROUTES.includes(pathname);
}

export const config = {
  matcher: ["/((?!_next/|favicon.ico|.*\\..*).*)"],
};

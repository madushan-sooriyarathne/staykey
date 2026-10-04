import { type NextRequest, NextResponse } from "next/server";
import { slugFromHost } from "@/lib/tenant";

/**
 * Maps property subdomains onto the /p/[slug] routes:
 *   kingfisher.staykey.direct/        ->  /p/kingfisher
 *   kingfisher.staykey.direct/embed   ->  /p/kingfisher/embed
 * Requests to the root domain pass through unchanged.
 */
export function proxy(request: NextRequest) {
  const slug = slugFromHost(request.headers.get("host"));
  if (!slug) return NextResponse.next();

  const { pathname, search } = request.nextUrl;
  const target = request.nextUrl.clone();
  target.pathname = `/p/${slug}${pathname === "/" ? "" : pathname}`;
  target.search = search;
  return NextResponse.rewrite(target);
}

export const config = {
  matcher: ["/((?!_next/|favicon.ico|.*\\..*).*)"],
};

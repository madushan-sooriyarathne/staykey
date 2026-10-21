import { type NextRequest, NextResponse } from "next/server";
import { storeFor } from "@/lib/store-for";
import { storeLinks } from "@/lib/stores";

/**
 * One link to share: a phone goes to its store's listing, anything else (or a store with no
 * listing yet) to the download section of the home page. It sets no cookies.
 */
export function GET(request: NextRequest) {
  const store = storeFor(request.headers.get("user-agent"));
  const target = (store && storeLinks()[store]) || new URL("/#download", request.url).toString();
  return NextResponse.redirect(target, 307);
}

import { createHash, timingSafeEqual } from "node:crypto";

// Proves that a server error reaches Sentry after a deploy. Without the ops secret this route
// looks like any missing page.
export const dynamic = "force-dynamic";

export async function POST(request: Request) {
  if (!hasOpsSecret(request.headers.get("x-ops-secret"))) {
    return new Response("Not found", { status: 404 });
  }
  throw new Error("Test error from the booking app, sent on purpose");
}

function hasOpsSecret(given: string | null): boolean {
  const secret = process.env.STAYKEY_OPS_SECRET;
  if (!secret || secret.length < 24 || !given) return false;
  // Comparing digests keeps the comparison constant-time whatever the lengths.
  const digest = (value: string) => createHash("sha256").update(value).digest();
  return timingSafeEqual(digest(given), digest(secret));
}

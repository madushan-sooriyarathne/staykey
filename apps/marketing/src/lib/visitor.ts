import "server-only";
import { headers } from "next/headers";

/** The visitor's address as the host in front of this site reports it. */
export async function visitorAddress(): Promise<string | undefined> {
  const h = await headers();
  return h.get("x-forwarded-for")?.split(",")[0]?.trim() || h.get("x-real-ip") || undefined;
}

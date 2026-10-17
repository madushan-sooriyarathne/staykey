import "server-only";
import { headers } from "next/headers";
import { slugFromHost } from "./tenant";

/**
 * The path of a page on a property's booking site. On the property's own subdomain the proxy
 * maps "/book" onto /p/<slug>/book, so links stay short there; elsewhere they carry the prefix.
 */
export async function pagePath(slug: string, rest = ""): Promise<string> {
  const host = (await headers()).get("host");
  if (slugFromHost(host) === slug) return rest || "/";
  return `/p/${slug}${rest}`;
}

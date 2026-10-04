/**
 * Hosted booking pages live at <slug>.<root domain>, for example kingfisher.staykey.direct.
 * In development the root domain is localhost:3001, so pages open at kingfisher.localhost:3001.
 */
export const ROOT_DOMAIN = process.env.NEXT_PUBLIC_ROOT_DOMAIN ?? "localhost:3001";

const SLUG_PATTERN = /^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$/;

// Keep in sync with reservedSlugs in apps/api/internal/domain/slug.go.
const RESERVED = new Set([
  "www",
  "api",
  "app",
  "cdn",
  "book",
  "admin",
  "mail",
  "help",
  "support",
  "status",
  "docs",
  "dashboard",
  "static",
  "assets",
  "embed",
  "widget",
  "staykey",
  "blog",
  "dev",
  "staging",
]);

/** Returns the property slug for a request host, or null for the root domain and reserved names. */
export function slugFromHost(host: string | null): string | null {
  if (!host) return null;
  const normalized = host.toLowerCase();
  const suffix = `.${ROOT_DOMAIN}`;
  if (!normalized.endsWith(suffix)) return null;

  const slug = normalized.slice(0, -suffix.length);
  if (!SLUG_PATTERN.test(slug) || RESERVED.has(slug)) return null;
  return slug;
}

export function bookingPageUrl(slug: string): string {
  const scheme = ROOT_DOMAIN.startsWith("localhost") ? "http" : "https";
  return `${scheme}://${slug}.${ROOT_DOMAIN}`;
}

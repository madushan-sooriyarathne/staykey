export type Store = "ios" | "android";

/** Which store a phone's user agent belongs to, or null for anything else. */
export function storeFor(userAgent: string | null): Store | null {
  if (!userAgent) return null;
  if (/iPhone|iPad|iPod/i.test(userAgent)) return "ios";
  // iPadOS 13 and later reports itself as a Mac, which a server can't tell apart, so those
  // visitors get the download section instead.
  if (/Android/i.test(userAgent)) return "android";
  return null;
}

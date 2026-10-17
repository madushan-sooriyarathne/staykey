import { joinPath } from "@/lib/links";

/** Rewrites incoming links before the router sees them. It must never throw. */
export function redirectSystemPath({ path }: { path: string; initial: boolean }): string {
  try {
    return joinPath(path);
  } catch {
    return path;
  }
}

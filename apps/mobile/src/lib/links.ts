/**
 * Invite links arrive as https://staykey.direct/join/<token> once the app owns that address
 * (universal links on iOS, app links on Android). The join screen reads the token from the query,
 * so the path becomes /join?token=<token>. Anything else passes through unchanged.
 */
export function joinPath(path: string): string {
  const match = /^(?:https?:\/\/[^/]+)?\/?join\/([^/?#]+)/.exec(path);
  return match?.[1] ? `/join?token=${match[1]}` : path;
}

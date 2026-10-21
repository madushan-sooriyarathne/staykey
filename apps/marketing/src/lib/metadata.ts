import type { Metadata } from "next";
import { SITE_URL } from "./company";

/**
 * A public page's metadata: its title and description in search results and in link previews,
 * with a canonical URL. Next merges metadata shallowly, so each page passes its own Open Graph
 * and Twitter fields rather than inheriting the home page's.
 */
export function pageMetadata({
  title,
  description,
  path,
  index = true,
}: {
  title: string;
  description: string;
  path: string;
  index?: boolean;
}): Metadata {
  const full = `${title} | StayKey`;
  return {
    title,
    description,
    alternates: { canonical: path },
    openGraph: {
      title: full,
      description,
      url: `${SITE_URL}${path === "/" ? "" : path}`,
      siteName: "StayKey",
      locale: "en_LK",
      type: "website",
    },
    twitter: { card: "summary_large_image", title: full, description },
    ...(index ? {} : { robots: { index: false, follow: true } }),
  };
}

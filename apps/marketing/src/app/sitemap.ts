import type { MetadataRoute } from "next";
import { LEGAL, SITE_URL } from "@/lib/company";

// The date the marketing copy last changed; bump it with the copy.
const CONTENT_UPDATED = "2026-10-21";

export default function sitemap(): MetadataRoute.Sitemap {
  return [
    { url: SITE_URL, lastModified: CONTENT_UPDATED, priority: 1 },
    { url: `${SITE_URL}/pricing`, lastModified: CONTENT_UPDATED, priority: 0.8 },
    { url: `${SITE_URL}/support`, lastModified: CONTENT_UPDATED, priority: 0.6 },
    { url: `${SITE_URL}/delete-account`, lastModified: CONTENT_UPDATED, priority: 0.3 },
    { url: `${SITE_URL}/privacy`, lastModified: LEGAL.privacy.updated, priority: 0.3 },
    { url: `${SITE_URL}/terms`, lastModified: LEGAL.terms.updated, priority: 0.3 },
  ];
}

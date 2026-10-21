import type { MetadataRoute } from "next";
import { SITE_URL } from "@/lib/company";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: "*",
      allow: "/",
      disallow: ["/billing", "/join/", "/verify-email", "/download"],
    },
    sitemap: `${SITE_URL}/sitemap.xml`,
  };
}

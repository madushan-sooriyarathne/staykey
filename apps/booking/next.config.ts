import { withSentryConfig } from "@sentry/nextjs/config";
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  transpilePackages: ["@staykey/api-client", "@staykey/observe", "@staykey/tokens"],
  // Inlined at build time, so the browser knows whether this is production without a request.
  env: {
    NEXT_PUBLIC_STAYKEY_ENVIRONMENT: process.env.STAYKEY_ENVIRONMENT ?? "development",
    NEXT_PUBLIC_STAYKEY_RELEASE: process.env.STAYKEY_RELEASE ?? "dev",
  },
  experimental: {
    // Bank slips go through a server action. The API takes slips up to 10 MB, and the form's
    // multipart framing needs a little room on top.
    serverActions: { bodySizeLimit: "11mb" },
  },
};

export default withSentryConfig(nextConfig, {
  org: process.env.SENTRY_ORG,
  project: process.env.SENTRY_PROJECT,
  authToken: process.env.SENTRY_AUTH_TOKEN,
  release: { name: process.env.STAYKEY_RELEASE },
  // Source maps upload only from builds that carry the token, which is the production deploy.
  sourcemaps: { disable: !process.env.SENTRY_AUTH_TOKEN },
  silent: !process.env.CI,
  telemetry: false,
  tunnelRoute: "/monitoring",
  widenClientFileUpload: true,
  suppressOnRouterTransitionStartWarning: true,
});

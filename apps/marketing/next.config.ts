import path from "node:path";
import { withSentryConfig } from "@sentry/nextjs/config";
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // The Docker image runs the standalone server; tracing from the repo root takes in the
  // workspace packages.
  output: "standalone",
  outputFileTracingRoot: path.join(__dirname, "../.."),
  transpilePackages: ["@staykey/api-client", "@staykey/observe", "@staykey/tokens"],
  // Inlined at build time, so the browser knows whether this is production without a request.
  env: {
    NEXT_PUBLIC_STAYKEY_ENVIRONMENT: process.env.STAYKEY_ENVIRONMENT ?? "development",
    NEXT_PUBLIC_STAYKEY_RELEASE: process.env.STAYKEY_RELEASE ?? "dev",
  },
  async redirects() {
    return [
      {
        source: "/:path*",
        has: [{ type: "host", value: "www.staykey.direct" }],
        destination: "https://staykey.direct/:path*",
        permanent: true,
      },
    ];
  },
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
          { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
        ],
      },
    ];
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

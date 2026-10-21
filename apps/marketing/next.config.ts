import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  transpilePackages: ["@staykey/api-client", "@staykey/tokens"],
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

export default nextConfig;

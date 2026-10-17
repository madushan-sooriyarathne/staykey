import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  transpilePackages: ["@staykey/api-client", "@staykey/tokens"],
};

export default nextConfig;

import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  transpilePackages: ["@staykey/api-client", "@staykey/tokens"],
  experimental: {
    // Bank slips go through a server action. The API takes slips up to 10 MB, and the form's
    // multipart framing needs a little room on top.
    serverActions: { bodySizeLimit: "11mb" },
  },
};

export default nextConfig;

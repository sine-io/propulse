import { StableModuleIdsPlugin } from "./scripts/stable-module-ids.mjs";

/** @type {import('next').NextConfig} */
const nextConfig = {
  experimental: {
    cpus: 1,
  },
  generateBuildId: async () => "static",
  output: "export",
  reactStrictMode: true,
  webpack: (config) => {
    config.optimization.moduleIds = false;
    config.plugins.push(new StableModuleIdsPlugin());
    return config;
  },
};

export default nextConfig;

/** @type {import('next').NextConfig} */
const nextConfig = {
  experimental: {
    cpus: 1,
  },
  generateBuildId: async () => "static",
  output: "export",
  reactStrictMode: true,
};

export default nextConfig;

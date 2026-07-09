/** @type {import('next').NextConfig} */
const nextConfig = {
  generateBuildId: async () => "static",
  output: "export",
  reactStrictMode: true,
};

export default nextConfig;

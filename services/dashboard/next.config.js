/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  swcMinify: true,
  output: 'standalone',
  env: {
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL,
    NEXT_PUBLIC_PROXY_ENDPOINT: process.env.NEXT_PUBLIC_PROXY_ENDPOINT,
    NEXT_PUBLIC_PROXY_HTTP_PORT: process.env.NEXT_PUBLIC_PROXY_HTTP_PORT,
    NEXT_PUBLIC_PROXY_SOCKS_PORT: process.env.NEXT_PUBLIC_PROXY_SOCKS_PORT,
  },
  async rewrites() {
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8002';
    return [
      {
        source: '/api/:path*',
        destination: `${apiUrl}/:path*`,
      },
    ]
  },
}

module.exports = nextConfig
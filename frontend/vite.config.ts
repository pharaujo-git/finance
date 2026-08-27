/// <reference types="vitest/config" />
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { fileURLToPath } from 'node:url'
import { defineConfig, loadEnv } from 'vite'

/**
 * Local API origins. Overridable so the ports can move without editing this
 * file -- port 5000, the .NET default, is taken by AirPlay on macOS.
 */
const DOTNET_TARGET = process.env.DEV_PROXY_DOTNET ?? 'http://localhost:5001'
const GO_TARGET = process.env.DEV_PROXY_GO ?? 'http://localhost:8081'
const PYTHON_TARGET = process.env.DEV_PROXY_PYTHON ?? 'http://localhost:8082'
const NODE_TARGET = process.env.DEV_PROXY_NODE ?? 'http://localhost:8083'
const RAILS_TARGET = process.env.DEV_PROXY_RAILS ?? 'http://localhost:8084'

/** Strips the routing prefix: /dotnet-api/api/x -> /api/x. */
const stripPrefix = (prefix: string) => (path: string) =>
  path.replace(new RegExp(`^${prefix}`), '')

/**
 * The API origins a production bundle must have baked in.
 *
 * Vite compiles these into the bundle, so a build made while one of them is
 * unset ships a page that silently dials http://localhost for that backend --
 * from a browser with no such server. It fails as "Failed to fetch", with
 * nothing in the server log and nothing in the database, which is
 * indistinguishable from that backend being down. The build is the last place
 * the mistake is still cheap to catch.
 */
const REQUIRED_API_ORIGINS = [
  'VITE_API_URL_GO',
  'VITE_API_URL_PYTHON',
  'VITE_API_URL_NODE',
  'VITE_API_URL_RAILS',
]

function assertApiOrigins(mode: string): void {
  const env = loadEnv(mode, process.cwd(), 'VITE_')
  const missing: string[] = REQUIRED_API_ORIGINS.filter(
    (name) => !env[name]?.trim(),
  )
  // The .NET origin still accepts the legacy single-API variable.
  if (!env.VITE_API_URL_DOTNET?.trim() && !env.VITE_API_URL?.trim()) {
    missing.push('VITE_API_URL_DOTNET')
  }
  if (missing.length === 0) return

  const verb = missing.length === 1 ? 'is' : 'are'
  throw new Error(
    `Refusing to build: ${missing.join(', ')} ${verb} not set. Every backend ` +
      'the login page offers needs its origin baked in, or that backend is ' +
      'unreachable from the deployed app. Set them and build again.',
  )
}

export default defineConfig(({ command, mode }) => {
  if (command === 'build') assertApiOrigins(mode)

  return {
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      // Every API is served through the dev server so the browser only ever
      // talks to its own origin. That removes CORS, the extra ports, and the
      // IPv4/IPv6 split from the list of things that can break a local run.
      // Point VITE_API_URL_DOTNET / _GO / _PYTHON / _NODE / _RAILS at these
      // paths to use them.
      proxy: {
        '/dotnet-api': {
          target: DOTNET_TARGET,
          changeOrigin: true,
          rewrite: stripPrefix('/dotnet-api'),
        },
        '/go-api': {
          target: GO_TARGET,
          changeOrigin: true,
          rewrite: stripPrefix('/go-api'),
        },
        '/py-api': {
          target: PYTHON_TARGET,
          changeOrigin: true,
          rewrite: stripPrefix('/py-api'),
        },
        '/node-api': {
          target: NODE_TARGET,
          changeOrigin: true,
          rewrite: stripPrefix('/node-api'),
        },
        '/rb-api': {
          target: RAILS_TARGET,
          changeOrigin: true,
          rewrite: stripPrefix('/rb-api'),
        },
      },
    },
    test: {
      globals: true,
      environment: 'jsdom',
      setupFiles: ['./src/test/setup.ts'],
      css: false,
      coverage: {
        provider: 'v8',
        reporter: ['text', 'lcov'],
        reportsDirectory: './coverage',
        include: ['src/**/*.{ts,tsx}'],
        exclude: [
          'src/main.tsx',
          'src/vite-env.d.ts',
          'src/test/**',
          'src/**/*.test.{ts,tsx}',
          'src/types/**',
        ],
      },
    },
  }
})

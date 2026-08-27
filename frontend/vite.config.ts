/// <reference types="vitest/config" />
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'

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

export default defineConfig({
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
})

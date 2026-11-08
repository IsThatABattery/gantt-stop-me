/// <reference types="vitest/config" />
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// localhost:8080 is the API in the dev container, or nginx when the compose stack runs on the host.
const API = 'http://localhost:8080'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  // Bind IPv4 loopback: Node resolves localhost to ::1 only, which VS Code's dev container port forwarding can't reach.
  server: {
    host: '127.0.0.1',
    proxy: {
      '/api': {
        target: API,
        // The API refuses writes whose Origin isn't PUBLIC_URL (localhost:8080
        // by default). Vite serves the page from :5173, so present the API's own
        // origin instead. Production is one origin behind nginx and needs nothing.
        configure: (proxy) => {
          proxy.on('proxyReq', (req) => {
            if (req.getHeader('origin')) req.setHeader('origin', API)
          })
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
})

/// <reference types="vitest/config" />
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  // localhost:8080 is the API in the dev container, or nginx when the compose stack runs on the host.
  // Bind IPv4 loopback: Node resolves localhost to ::1 only, which VS Code's dev container port forwarding can't reach.
  server: { host: '127.0.0.1', proxy: { '/api': 'http://localhost:8080' } },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
})

/// <reference types="vitest/config" />
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
<<<<<<< HEAD
  plugins: [react(), tailwindcss()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
=======
  plugins: [react()],
  // localhost:8080 is the API in the dev container, or nginx when the compose stack runs on the host.
  server: { proxy: { '/api': 'http://localhost:8080' } },
>>>>>>> main
})
